const ACCESS_TOKEN_KEY = "mistypass_admin_access_token"
const REFRESH_TOKEN_KEY = "mistypass_admin_refresh_token"

export const AUTH_SESSION_EVENT = "mistypass:auth-session-changed"

export type AuthSessionChangeReason = "login" | "refresh" | "logout"

export type AuthSessionChangeDetail = {
  accessToken: string | null
  refreshToken: string | null
  reason: AuthSessionChangeReason
}

export function getToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
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
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
  dispatchAuthSessionChange(reason)
}

export function clearSession(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
  dispatchAuthSessionChange("logout")
}

export function saveToken(token: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, token)
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
