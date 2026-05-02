import { request, requestItems, requestText, withTenantQuery, encodePathSegment, APIError } from "./core"

export type AccessUser = {
  id: string
  tenant_id: string
  building_id?: string
  name: string
  email: string
  role: string
  status: string
  group_ids?: string[]
  sync_source?: string
  sync_ref?: string
  created_at: string
}

export type UserInvitationDelivery = {
  id: string
  resource_type: "UserInvitationDelivery"
  tenant_id: string
  user_id: string
  email: string
  place_id?: string
  delivery_method: "email" | "email_qr"
  status: "queued" | "sent" | "failed"
  provider?: string
  provider_delivery_id?: string
  provider_error?: string
  retryable?: boolean
  queued_at: string
  delivered_at?: string
  updated_at: string
}

export type Guest = {
  id: string
  tenant_id: string
  building_id?: string
  name: string
  email?: string
  phone: string
  company?: string
  purpose?: string
  host_name: string
  host_email?: string
  host_phone?: string
  id_document_type?: "KTP" | "KITAS" | "ITAS"
  id_document_number?: string
  status: "expected" | "checked_in" | "checked_out" | "cancelled"
  checked_in_at?: string
  checked_out_at?: string
  expected_at?: string
  notify_host?: boolean
  host_notified_at?: string
  created_at: string
  updated_at: string
}

export type BatchUserStatusResult = {
  tenant_id: string
  status: string
  updated: number
  skipped: number
  not_found: number
  user_ids: string[]
}

export type BatchUserDeleteResult = {
  tenant_id: string
  deleted: number
  not_found: number
  user_ids: string[]
}

export type BatchUserInviteResult = {
  tenant_id: string
  queued: number
  skipped: number
  not_found: number
  user_ids: string[]
}

export type UserImportResult = {
  tenant_id: string
  created: number
  updated: number
  skipped: number
  errors: number
}

// --- Users ---

export async function listAccessUsers(token: string | undefined, tenantID?: string): Promise<AccessUser[]> {
  return requestItems<AccessUser>(withTenantQuery("/api/v1/users", tenantID), token)
}

export async function createAccessUser(
  token: string | undefined,
  payload: {
    tenant_id: string
    building_id?: string
    name: string
    email: string
    role?: string
    status?: string
    group_ids?: string[]
  }
): Promise<AccessUser> {
  return request<AccessUser>(
    "/api/v1/users",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function fetchAccessUser(
  token: string | undefined,
  userID: string,
  tenantID?: string
): Promise<AccessUser> {
  return request<AccessUser>(
    withTenantQuery(`/api/v1/users/${encodePathSegment(userID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function updateAccessUser(
  token: string | undefined,
  userID: string,
  payload: {
    tenant_id?: string
    building_id?: string
    name?: string
    email?: string
    role?: string
    status?: string
    group_ids?: string[]
  }
): Promise<AccessUser> {
  return request<AccessUser>(
    `/api/v1/users/${encodePathSegment(userID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function deleteAccessUser(
  token: string | undefined,
  userID: string,
  tenantID?: string
): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/users/${encodePathSegment(userID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

export async function sendAccessUserInvitation(
  token: string | undefined,
  userID: string,
  payload: {
    tenant_id?: string
    delivery_method?: UserInvitationDelivery["delivery_method"]
  } = {}
): Promise<UserInvitationDelivery> {
  return request<UserInvitationDelivery>(
    `/api/v1/users/${encodePathSegment(userID)}/invite`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listAccessUserInvitations(
  token: string | undefined,
  userID: string,
  tenantID?: string
): Promise<UserInvitationDelivery[]> {
  return requestItems<UserInvitationDelivery>(
    withTenantQuery(`/api/v1/users/${encodePathSegment(userID)}/invitations`, tenantID),
    token
  )
}

// Aliases
export const fetchUser = fetchAccessUser
export const createUser = createAccessUser
export const updateUser = updateAccessUser
export const deleteUser = deleteAccessUser
export const sendUserInvitation = sendAccessUserInvitation
export const listUserInvitations = listAccessUserInvitations

// --- Independent invitation resource ---

export async function listInvitations(
  token: string | undefined,
  options?: { tenant_id?: string; status?: string; limit?: number; offset?: number }
): Promise<UserInvitationDelivery[]> {
  let path = "/api/v1/invitations"
  const params = new URLSearchParams()
  if (options?.tenant_id) params.set("tenant_id", options.tenant_id)
  if (options?.status) params.set("status", options.status)
  if (options?.limit) params.set("limit", String(options.limit))
  if (options?.offset) params.set("offset", String(options.offset))
  const q = params.toString()
  if (q) path += "?" + q
  return requestItems<UserInvitationDelivery>(path, token)
}

export async function cancelInvitation(
  token: string | undefined,
  deliveryID: string,
  tenantID?: string
): Promise<UserInvitationDelivery> {
  return request<UserInvitationDelivery>(
    withTenantQuery(`/api/v1/invitations/${encodePathSegment(deliveryID)}/cancel`, tenantID),
    { method: "POST" },
    token
  )
}

export async function resendInvitation(
  token: string | undefined,
  deliveryID: string,
  tenantID?: string
): Promise<UserInvitationDelivery> {
  return request<UserInvitationDelivery>(
    withTenantQuery(`/api/v1/invitations/${encodePathSegment(deliveryID)}/resend`, tenantID),
    { method: "POST" },
    token
  )
}

// --- Batch ops ---

export async function batchUpdateUserStatus(
  token: string | undefined,
  payload: { tenant_id: string; user_ids: string[]; status: string }
): Promise<BatchUserStatusResult> {
  return request<BatchUserStatusResult>(
    "/api/v1/users/batch-status",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function batchDeleteUsers(
  token: string | undefined,
  payload: { tenant_id: string; user_ids: string[] }
): Promise<BatchUserDeleteResult> {
  return request<BatchUserDeleteResult>(
    "/api/v1/users/batch-delete",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function batchInviteUsers(
  token: string | undefined,
  payload: { tenant_id: string; user_ids: string[]; delivery_method?: string }
): Promise<BatchUserInviteResult> {
  return request<BatchUserInviteResult>(
    "/api/v1/users/batch-invite",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function exportUsersCSV(
  token: string | undefined,
  tenantID?: string
): Promise<string> {
  const url = withTenantQuery("/api/v1/users/export-csv", tenantID)
  const resp = await fetch(url, {
    method: "GET",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!resp.ok) throw new APIError(resp.status, await resp.text())
  return resp.text()
}

export async function importUsersCSV(
  token: string | undefined,
  payload: { tenant_id: string; csv_content: string }
): Promise<UserImportResult> {
  return request<UserImportResult>(
    "/api/v1/users/import-csv",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

// --- Guests ---

export async function listGuests(token: string | undefined, tenantID?: string): Promise<{ items: Guest[] }> {
  return request<{ items: Guest[] }>(withTenantQuery("/api/v1/guests", tenantID), {}, token)
}

export async function createGuest(token: string | undefined, payload: Partial<Guest> & { tenant_id?: string }): Promise<Guest> {
  return request<Guest>("/api/v1/guests", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function updateGuestStatus(token: string | undefined, guestID: string, tenantID: string, status: string): Promise<Guest> {
  return request<Guest>(`/api/v1/guests/${guestID}/status`, {
    method: "PATCH",
    body: JSON.stringify({ tenant_id: tenantID, status }),
  }, token)
}

export async function deleteGuest(token: string | undefined, guestID: string, tenantID: string): Promise<void> {
  return request<void>(`/api/v1/guests/${guestID}?tenant_id=${tenantID}`, { method: "DELETE" }, token)
}
