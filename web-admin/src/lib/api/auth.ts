import { request, requestItems } from "./core"
import type { LoginResponse, CurrentUser } from "./core"

export type MFAStatus = {
  user_id: string
  enabled: boolean
  pending: boolean
  updated_at?: string
}

export type MFAEnrollment = {
  user_id: string
  secret: string
  otpauth_url: string
}

export type LoginSession = {
  session_id: string
  user_id: string
  ip_address: string
  user_agent: string
  login_method: string
  created_at: string
  expires_at: string
}

export type WebAuthnCredential = {
  id: string
  user_id: string
  public_key: string
  attestation_type: string
  aaguid: string
  sign_count: number
  display_name: string
  created_at: string
}

export type WebAuthnLoginFinishResponse =
  | (LoginResponse & { mfa_required?: undefined })
  | { mfa_required: true; webauthn_token: string }

// --- Login ---

export async function login(email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>(
    "/api/v1/auth/login",
    {
      method: "POST",
      body: JSON.stringify({ email, password }),
    },
    undefined
  )
}

export async function getCurrentUser(token: string | undefined): Promise<CurrentUser> {
  return request<CurrentUser>("/api/v1/me", { method: "GET" }, token)
}

export async function getCurrentUserProfile(token: string | undefined): Promise<CurrentUser> {
  return request<CurrentUser>("/api/v1/user", { method: "GET" }, token)
}

export async function updateCurrentUser(
  token: string | undefined,
  payload: {
    name?: string
    language?: string
  }
): Promise<CurrentUser> {
  return request<CurrentUser>(
    "/api/v1/user",
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

// --- MFA ---

export async function getUserMFAStatus(token: string | undefined): Promise<MFAStatus> {
  return request<MFAStatus>("/api/v1/auth/mfa/user/status", {}, token)
}

export async function setupUserMFA(
  token: string | undefined,
  issuer?: string
): Promise<MFAEnrollment> {
  return request<MFAEnrollment>(
    "/api/v1/auth/mfa/user/setup",
    { method: "POST", body: JSON.stringify({ issuer: issuer || "Mistyislet" }) },
    token
  )
}

export async function enableUserMFA(
  token: string | undefined,
  code: string
): Promise<MFAStatus> {
  return request<MFAStatus>(
    "/api/v1/auth/mfa/user/enable",
    { method: "POST", body: JSON.stringify({ code }) },
    token
  )
}

export async function disableUserMFA(token: string | undefined): Promise<MFAStatus> {
  return request<MFAStatus>(
    "/api/v1/auth/mfa/user/disable",
    { method: "POST" },
    token
  )
}

// --- Password Reset ---

export async function requestPasswordReset(email: string): Promise<{ status: string; reset_token?: string }> {
  return request<{ status: string; reset_token?: string }>("/api/v1/auth/password-reset/request", {
    method: "POST",
    body: JSON.stringify({ email }),
  })
}

export async function confirmPasswordReset(token: string, newPassword: string): Promise<{ status: string }> {
  return request<{ status: string }>("/api/v1/auth/password-reset/confirm", {
    method: "POST",
    body: JSON.stringify({ token, new_password: newPassword }),
  })
}

// --- Login Sessions ---

export async function listLoginSessions(token: string | undefined): Promise<LoginSession[]> {
  return request<LoginSession[]>("/api/v1/auth/sessions", {}, token)
}

export async function revokeLoginSession(token: string | undefined, sessionID: string): Promise<void> {
  return request<void>("/api/v1/auth/sessions/revoke", {
    method: "POST",
    body: JSON.stringify({ session_id: sessionID }),
  }, token)
}

export async function revokeAllLoginSessions(token: string | undefined): Promise<{ revoked: number }> {
  return request<{ revoked: number }>("/api/v1/auth/sessions/revoke-all", {
    method: "POST",
  }, token)
}

// --- WebAuthn / Passkeys ---

export async function webAuthnRegisterBegin(token: string | undefined): Promise<CredentialCreationOptions> {
  return request<CredentialCreationOptions>("/api/v1/auth/webauthn/register/begin", { method: "POST" }, token)
}

export async function webAuthnRegisterFinish(
  token: string | undefined,
  credential: Credential,
  displayName?: string
): Promise<WebAuthnCredential> {
  const url = displayName
    ? `/api/v1/auth/webauthn/register/finish?display_name=${encodeURIComponent(displayName)}`
    : "/api/v1/auth/webauthn/register/finish"
  return request<WebAuthnCredential>(url, {
    method: "POST",
    body: JSON.stringify(credential),
  }, token)
}

export async function webAuthnLoginBegin(email: string): Promise<CredentialRequestOptions> {
  return request<CredentialRequestOptions>("/api/v1/auth/webauthn/login/begin", {
    method: "POST",
    body: JSON.stringify({ email }),
  })
}

export async function webAuthnLoginFinish(
  email: string,
  credential: any,
): Promise<WebAuthnLoginFinishResponse> {
  const params = new URLSearchParams({ email })
  return request<WebAuthnLoginFinishResponse>(`/api/v1/auth/webauthn/login/finish?${params}`, {
    method: "POST",
    body: JSON.stringify(credential),
  })
}

export async function webAuthnLoginFinishMFA(
  webauthnToken: string,
  mfaCode: string
): Promise<LoginResponse> {
  return request<LoginResponse>("/api/v1/auth/webauthn/login/finish", {
    method: "POST",
    body: JSON.stringify({ webauthn_token: webauthnToken, mfa_code: mfaCode }),
  })
}

export async function listWebAuthnCredentials(token: string | undefined): Promise<WebAuthnCredential[]> {
  return request<WebAuthnCredential[]>("/api/v1/auth/webauthn/credentials", {}, token)
}

export async function deleteWebAuthnCredential(token: string | undefined, credentialID: string): Promise<void> {
  return request<void>(`/api/v1/auth/webauthn/credentials/${encodeURIComponent(credentialID)}`, {
    method: "DELETE",
  }, token)
}

// --- Signup / Password Change ---

export async function userSignUp(payload: { name: string; email: string; password: string }): Promise<any> {
  return request("/api/v1/users/sign_up", { method: "POST", body: JSON.stringify(payload) })
}

export async function changeUserPassword(token: string | undefined, currentPassword: string, newPassword: string): Promise<{ status: string }> {
  return request("/api/v1/users/password", { method: "POST", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) }, token)
}
