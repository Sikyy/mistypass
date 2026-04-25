const ACCESS_TOKEN_KEY = "mistypass_admin_access_token"
const REFRESH_TOKEN_KEY = "mistypass_admin_refresh_token"
const LEGACY_ACCESS_TOKEN_KEY = ACCESS_TOKEN_KEY
const LEGACY_REFRESH_TOKEN_KEY = REFRESH_TOKEN_KEY

export const AUTH_SESSION_EVENT = "mistypass:auth-session-changed"

export type AuthSessionChangeReason = "login" | "refresh" | "logout"

export type AuthSessionChangeDetail = {
  accessToken: string | null
  refreshToken: string | null
  reason: AuthSessionChangeReason
}

let inMemoryAccessToken: string | null = null
let inMemoryRefreshToken: string | null = null
let accessTokenHydrated = false
let refreshTokenHydrated = false

function readSessionStorage(key: string): string | null {
  if (typeof window === "undefined") {
    return null
  }
  try {
    return window.sessionStorage.getItem(key)
  } catch {
    return null
  }
}

function writeSessionStorage(key: string, value: string): void {
  if (typeof window === "undefined") {
    return
  }
  try {
    window.sessionStorage.setItem(key, value)
  } catch {
    // Ignore storage failures and keep memory-only fallback.
  }
}

function removeSessionStorage(key: string): void {
  if (typeof window === "undefined") {
    return
  }
  try {
    window.sessionStorage.removeItem(key)
  } catch {
    // Best effort cleanup.
  }
}

function readLegacyLocalStorage(key: string): string | null {
  if (typeof window === "undefined") {
    return null
  }
  try {
    return window.localStorage.getItem(key)
  } catch {
    return null
  }
}

function removeLegacyLocalStorage(key: string): void {
  if (typeof window === "undefined") {
    return
  }
  try {
    window.localStorage.removeItem(key)
  } catch {
    // Best effort cleanup.
  }
}

function migrateLegacyToken(currentKey: string, legacyKey: string): string | null {
  const sessionValue = readSessionStorage(currentKey)
  if (sessionValue && sessionValue.trim() !== "") {
    return sessionValue
  }

  const legacyValue = readLegacyLocalStorage(legacyKey)
  if (legacyValue && legacyValue.trim() !== "") {
    writeSessionStorage(currentKey, legacyValue)
    removeLegacyLocalStorage(legacyKey)
    return legacyValue
  }

  return null
}

export function getToken(): string | null {
  if (!accessTokenHydrated) {
    inMemoryAccessToken = migrateLegacyToken(ACCESS_TOKEN_KEY, LEGACY_ACCESS_TOKEN_KEY)
    accessTokenHydrated = true
  }
  return inMemoryAccessToken
}

export function getRefreshToken(): string | null {
  if (!refreshTokenHydrated) {
    inMemoryRefreshToken = migrateLegacyToken(REFRESH_TOKEN_KEY, LEGACY_REFRESH_TOKEN_KEY)
    refreshTokenHydrated = true
  }
  return inMemoryRefreshToken
}

function dispatchAuthSessionChange(reason: AuthSessionChangeReason): void {
  if (typeof window === "undefined") {
    return
  }
  window.dispatchEvent(
    new CustomEvent<AuthSessionChangeDetail>(AUTH_SESSION_EVENT, {
      detail: {
        accessToken: getToken(),
        refreshToken: getRefreshToken(),
        reason,
      },
    })
  )
}

export function saveSession(
  accessToken: string,
  refreshToken: string,
  reason: Exclude<AuthSessionChangeReason, "logout"> = "login"
): void {
  inMemoryAccessToken = accessToken
  inMemoryRefreshToken = refreshToken
  accessTokenHydrated = true
  refreshTokenHydrated = true
  writeSessionStorage(ACCESS_TOKEN_KEY, accessToken)
  writeSessionStorage(REFRESH_TOKEN_KEY, refreshToken)
  removeLegacyLocalStorage(LEGACY_ACCESS_TOKEN_KEY)
  removeLegacyLocalStorage(LEGACY_REFRESH_TOKEN_KEY)
  dispatchAuthSessionChange(reason)
}

export function clearSession(): void {
  inMemoryAccessToken = null
  inMemoryRefreshToken = null
  accessTokenHydrated = true
  refreshTokenHydrated = true
  removeSessionStorage(ACCESS_TOKEN_KEY)
  removeSessionStorage(REFRESH_TOKEN_KEY)
  removeLegacyLocalStorage(LEGACY_ACCESS_TOKEN_KEY)
  removeLegacyLocalStorage(LEGACY_REFRESH_TOKEN_KEY)
  dispatchAuthSessionChange("logout")
}

export function saveToken(token: string): void {
  inMemoryAccessToken = token
  accessTokenHydrated = true
  writeSessionStorage(ACCESS_TOKEN_KEY, token)
  removeLegacyLocalStorage(LEGACY_ACCESS_TOKEN_KEY)
}

export function clearToken(): void {
  clearSession()
}

export function subscribeAuthSessionChange(
  listener: (detail: AuthSessionChangeDetail) => void
): () => void {
  if (typeof window === "undefined") {
    return () => undefined
  }
  const handler = (event: Event) => {
    const customEvent = event as CustomEvent<AuthSessionChangeDetail>
    listener(customEvent.detail)
  }
  window.addEventListener(AUTH_SESSION_EVENT, handler)
  return () => {
    window.removeEventListener(AUTH_SESSION_EVENT, handler)
  }
}
