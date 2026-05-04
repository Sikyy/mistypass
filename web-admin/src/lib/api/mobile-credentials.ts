import { request, withTenantQuery, encodePathSegment } from "./core"

export type MobileCredential = {
  id: string
  tenant_id: string
  user_id: string
  user_email: string
  device_id: string
  platform: "android" | "ios"
  device_model: string
  keystore_level: "strongbox" | "tee" | "software"
  public_key_pem: string
  status: "active" | "revoked" | "expired"
  issued_at: string
  expires_at: string
  revoked_at?: string
  last_used_at?: string
}

export async function listMobileCredentials(
  token: string | undefined,
  tenantID?: string,
  userID?: string,
): Promise<{ items: MobileCredential[]; total: number }> {
  let path = withTenantQuery("/api/v1/credentials/mobile", tenantID)
  if (userID) path += `${path.includes("?") ? "&" : "?"}user_id=${encodePathSegment(userID)}`
  return request<{ items: MobileCredential[]; total: number }>(path, { method: "GET" }, token)
}

export async function getMobileCredential(
  token: string | undefined,
  credentialID: string,
): Promise<{ credential: MobileCredential }> {
  return request<{ credential: MobileCredential }>(
    `/api/v1/credentials/mobile/${encodePathSegment(credentialID)}`,
    { method: "GET" },
    token,
  )
}

export async function revokeMobileCredential(
  token: string | undefined,
  credentialID: string,
): Promise<{ status: string; id: string }> {
  return request<{ status: string; id: string }>(
    `/api/v1/credentials/mobile/${encodePathSegment(credentialID)}/revoke`,
    { method: "POST" },
    token,
  )
}

export async function revokeAllUserCredentials(
  token: string | undefined,
  userID: string,
): Promise<{ revoked_count: number; user_id: string }> {
  return request<{ revoked_count: number; user_id: string }>(
    `/api/v1/credentials/mobile/revoke-user`,
    { method: "POST", body: JSON.stringify({ user_id: userID }) },
    token,
  )
}
