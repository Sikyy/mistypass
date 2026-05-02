import { clearSession, getRefreshToken, getToken, saveSession } from "@/lib/auth"

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080"

export type LoginResponse = {
  access_token: string
  refresh_token: string
  expires_in: number
  user: CurrentUser
}

export type CurrentUser = {
  id: string
  name?: string
  email: string
  role: UserRole
  tenant_id: string
  building_ids?: string[]
  language?: "en-US" | "id-ID" | "zh-CN" | string
}

export type UserRole = "super_admin" | "tenant_admin" | "operator" | "building_admin" | "resident"

export type ServerSentEvent = {
  event: string
  data: string
}

type RequestOptions = {
  skipAuthRecovery?: boolean
}

export class APIError extends Error {
  readonly status: number
  readonly code?: string
  readonly responseStatus?: string

  constructor(status: number, message: string, options: { code?: string; responseStatus?: string } = {}) {
    super(message)
    this.name = "APIError"
    this.status = status
    this.code = options.code
    this.responseStatus = options.responseStatus
  }
}

let refreshInFlight: Promise<string | null> | null = null
let authTokenProvider: (() => string | null) | null = null

export function setAuthTokenProvider(provider: (() => string | null) | null): void {
  authTokenProvider = provider
}

export function resolveAuthToken(token: string | undefined): string | undefined {
  if (token && token.trim() !== "") {
    return token
  }
  const providedToken = authTokenProvider?.()
  if (providedToken && providedToken.trim() !== "") {
    return providedToken
  }
  return getToken() ?? undefined
}

export async function parseAPIErrorDetails(response: Response): Promise<{
  message: string
  code?: string
  responseStatus?: string
}> {
  const fallback = `${response.status} ${response.statusText}`
  try {
    const payload = (await response.json()) as {
      error?: unknown
      error_description?: unknown
      message?: unknown
      code?: unknown
      status?: unknown
    }
    const error = typeof payload.error === "string" && payload.error.trim() ? payload.error.trim() : undefined
    const description =
      typeof payload.error_description === "string" && payload.error_description.trim()
        ? payload.error_description.trim()
        : undefined
    const message = typeof payload.message === "string" && payload.message.trim() ? payload.message.trim() : undefined
    const code = typeof payload.code === "string" && payload.code.trim() ? payload.code.trim() : undefined
    const responseStatus =
      typeof payload.status === "string" && payload.status.trim()
        ? payload.status.trim()
        : typeof payload.status === "number" && Number.isFinite(payload.status)
          ? String(payload.status)
          : undefined
    return {
      message: error ?? message ?? description ?? fallback,
      code,
      responseStatus,
    }
  } catch {
    return { message: fallback }
  }
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) {
    return refreshInFlight
  }

  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    clearSession()
    return null
  }

  refreshInFlight = (async () => {
    try {
      const response = await request<LoginResponse>(
        "/api/v1/auth/refresh",
        {
          method: "POST",
          body: JSON.stringify({ refresh_token: refreshToken }),
        },
        undefined,
        { skipAuthRecovery: true }
      )
      if (!response.access_token || !response.refresh_token) {
        clearSession()
        return null
      }
      saveSession(response.access_token, response.refresh_token, "refresh")
      return response.access_token
    } catch {
      clearSession()
      return null
    } finally {
      refreshInFlight = null
    }
  })()

  return refreshInFlight
}

export async function request<T>(path: string, init: RequestInit, token?: string | undefined, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(init.headers ?? {})
  headers.set("Content-Type", "application/json")
  const activeToken = resolveAuthToken(token)
  if (activeToken) {
    headers.set("Authorization", `Bearer ${activeToken}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers,
  })

  if (response.status === 401 && activeToken && !options.skipAuthRecovery) {
    const refreshedToken = await refreshAccessToken()
    if (refreshedToken) {
      return request<T>(path, init, refreshedToken, { skipAuthRecovery: true })
    }
    throw new APIError(401, "Session expired, please sign in again")
  }

  if (!response.ok) {
    const errorDetails = await parseAPIErrorDetails(response)
    if (response.status === 401) {
      clearSession()
    }
    throw new APIError(response.status, errorDetails.message, {
      code: errorDetails.code,
      responseStatus: errorDetails.responseStatus,
    })
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export async function requestItems<T>(path: string, token: string | undefined): Promise<T[]> {
  const payload = await request<{ items?: T[] | null } | T[] | null>(path, { method: "GET" }, token)
  if (Array.isArray(payload)) {
    return payload
  }
  return Array.isArray(payload?.items) ? payload.items : []
}

export type OffsetPaginationPayload = {
  total?: number
  offset?: number
  limit?: number
  next_offset?: number
  has_more?: boolean
}

export type EnterpriseOffsetListResponse<T> = {
  items: T[]
  total: number
  offset: number
  limit: number
  next_offset?: number
  has_more: boolean
  pagination?: OffsetPaginationPayload | null
}

function normalizePaginationNumber(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 ? Math.floor(value) : fallback
}

export function normalizeOffsetListResponse<T>(
  payload:
    | EnterpriseOffsetListResponse<T>
    | {
        items?: T[] | null
        total?: number
        offset?: number
        limit?: number
        next_offset?: number
        has_more?: boolean
        pagination?: OffsetPaginationPayload | null
      }
    | T[]
    | null
): EnterpriseOffsetListResponse<T> {
  const normalizedPayload = payload && !Array.isArray(payload) ? payload : null
  const pagination = normalizedPayload?.pagination ?? null
  const items = Array.isArray(payload) ? payload : Array.isArray(normalizedPayload?.items) ? normalizedPayload.items : []
  const total = normalizePaginationNumber(normalizedPayload?.total ?? pagination?.total, items.length)
  const offset = normalizePaginationNumber(normalizedPayload?.offset ?? pagination?.offset, 0)
  const limit = normalizePaginationNumber(normalizedPayload?.limit ?? pagination?.limit, items.length)
  const nextOffset =
    typeof normalizedPayload?.next_offset === "number" &&
    Number.isFinite(normalizedPayload.next_offset) &&
    normalizedPayload.next_offset >= 0
      ? Math.floor(normalizedPayload.next_offset)
      : typeof pagination?.next_offset === "number" &&
          Number.isFinite(pagination.next_offset) &&
          pagination.next_offset >= 0
        ? Math.floor(pagination.next_offset)
      : undefined
  const hasMore =
    typeof normalizedPayload?.has_more === "boolean"
      ? normalizedPayload.has_more
      : typeof pagination?.has_more === "boolean"
        ? pagination.has_more
      : typeof nextOffset === "number"
        ? nextOffset < total
        : offset + items.length < total

  return {
    items,
    total,
    offset,
    limit,
    next_offset: hasMore ? nextOffset ?? offset + items.length : undefined,
    has_more: hasMore,
  }
}

export function normalizeCount(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.floor(value)) : fallback
}

export async function requestText(path: string, token: string | undefined): Promise<string> {
  const activeToken = resolveAuthToken(token)
  const headers = new Headers()
  if (activeToken) {
    headers.set("Authorization", `Bearer ${activeToken}`)
  }
  headers.set("Accept", "text/csv, text/plain;q=0.9, */*;q=0.8")

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "GET",
    headers,
  })

  if (response.status === 401 && activeToken) {
    const refreshedToken = await refreshAccessToken()
    if (refreshedToken) {
      return requestText(path, refreshedToken)
    }
    throw new APIError(401, "Session expired, please sign in again")
  }

  if (!response.ok) {
    const errorDetails = await parseAPIErrorDetails(response)
    throw new APIError(response.status, errorDetails.message, {
      code: errorDetails.code,
      responseStatus: errorDetails.responseStatus,
    })
  }

  return response.text()
}

export async function consumeServerSentEvents(args: {
  path: string
  token?: string
  signal: AbortSignal
  onEvent: (message: ServerSentEvent) => void
  onError?: (error: Error) => void
}): Promise<void> {
  try {
    const activeToken = resolveAuthToken(args.token)
    const headers = new Headers()
    if (activeToken) {
      headers.set("Authorization", `Bearer ${activeToken}`)
    }
    headers.set("Accept", "text/event-stream")

    const response = await fetch(`${API_BASE_URL}${args.path}`, {
      method: "GET",
      headers,
      signal: args.signal,
    })

    if (response.status === 401 && activeToken) {
      const refreshedToken = await refreshAccessToken()
      if (refreshedToken) {
        return consumeServerSentEvents({
          ...args,
          token: refreshedToken,
        })
      }
      throw new APIError(401, "Session expired, please sign in again")
    }

    if (!response.ok) {
      const errorDetails = await parseAPIErrorDetails(response)
      throw new APIError(response.status, errorDetails.message, {
        code: errorDetails.code,
        responseStatus: errorDetails.responseStatus,
      })
    }

    const reader = response.body?.getReader()
    if (!reader) {
      throw new Error("SSE stream body is unavailable")
    }

    const decoder = new TextDecoder()
    let pending = ""

    const emitChunk = (chunk: string): void => {
      const lines = chunk.split("\n")
      let eventName = "message"
      const dataLines: string[] = []

      for (const rawLine of lines) {
        const line = rawLine.replace(/\r$/, "")
        if (line.startsWith("event:")) {
          eventName = line.slice("event:".length).trim() || "message"
          continue
        }
        if (line.startsWith("data:")) {
          dataLines.push(line.slice("data:".length).trimStart())
        }
      }

      if (dataLines.length > 0) {
        args.onEvent({
          event: eventName,
          data: dataLines.join("\n"),
        })
      }
    }

    while (!args.signal.aborted) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      pending += decoder.decode(value, { stream: true })
      const chunks = pending.split("\n\n")
      pending = chunks.pop() ?? ""
      for (const chunk of chunks) {
        if (chunk.trim() === "") {
          continue
        }
        emitChunk(chunk)
      }
    }
  } catch (error) {
    if (args.signal.aborted) {
      return
    }
    const nextError = error instanceof Error ? error : new Error(String(error))
    args.onError?.(nextError)
  }
}

export function withTenantQuery(path: string, tenantID?: string): string {
  const value = tenantID?.trim()
  if (!value) {
    return path
  }

  const separator = path.includes("?") ? "&" : "?"
  return `${path}${separator}tenant_id=${encodeURIComponent(value)}`
}

export function encodePathSegment(value: string): string {
  return encodeURIComponent(value.trim())
}

export type ListPageOptions = {
  page?: number
  limit?: number
}

export function withPageQuery(path: string, options?: ListPageOptions): string {
  const query = new URLSearchParams()
  if (typeof options?.page === "number" && Number.isFinite(options.page) && options.page > 0) {
    query.set("page", String(Math.floor(options.page)))
  }
  if (typeof options?.limit === "number" && Number.isFinite(options.limit) && options.limit > 0) {
    query.set("limit", String(Math.floor(options.limit)))
  }
  const suffix = query.toString()
  if (!suffix) {
    return path
  }
  const separator = path.includes("?") ? "&" : "?"
  return `${path}${separator}${suffix}`
}
