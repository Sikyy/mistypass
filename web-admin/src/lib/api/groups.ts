import { request, requestItems, withTenantQuery, encodePathSegment } from "./core"

export type UserGroup = {
  id: string
  resource_type?: "Group"
  tenant_id: string
  building_id?: string
  place_id?: string
  name: string
  description: string
  login_enabled?: boolean
  geofence_restriction_enabled?: boolean
  geofence_restriction_radius?: number
  primary_device_restriction_enabled?: boolean
  managed_device_restriction_enabled?: boolean
  reader_restriction_enabled?: boolean
  time_restriction_enabled?: boolean
  tap_to_access_restriction_enabled?: boolean
  time_restriction_time_zone?: string
  users_count?: number
  locks_count?: number
  elevator_stops_count?: number
  members?: string[]
  created_at: string
  updated_at: string
}

export type GroupLink = {
  id: string
  resource_type: "GroupLink"
  tenant_id: string
  group_id: string
  group_name?: string
  name: string
  email?: string
  phone?: string
  last_used_at?: string
  link_enabled: boolean
  quick_response_code_type?: "online" | "offline" | string
  valid_from?: string
  valid_until?: string
  created_by_type?: "User" | "MarketplaceInstallation" | string
  created_by_id?: string
  created_by_email?: string
  created_by_name?: string
  issued_by_id?: string
  secret?: string
  quick_response_code_token?: string
  quick_response_code_image?: string
  created_at: string
  updated_at: string
}

export type GroupLinkVerification = {
  valid: boolean
  status: string
  verified_at: string
  claimed_at?: string
  group_link: GroupLink
}

export type GroupLock = {
  id: string
  tenant_id: string
  group_id: string
  lock_id: string
  created_at: string
}

export type GroupZone = {
  id: string
  tenant_id: string
  group_id: string
  zone_id: string
  place_id: string
  floor_id: string
  name: string
  created_at: string
}

export type GroupTerminal = { id: string; tenant_id: string; group_id: string; terminal_id: string; created_at: string }
export type Presence = { id: string; tenant_id: string; place_id: string; user_id: string; user_name?: string; user_email?: string; entered_at: string; exited_at?: string }
export type CSVCardImport = { id: string; tenant_id: string; file_name: string; status: string; total_rows: number; imported: number; failed: number; created_at: string }

// --- User Groups ---

export async function listUserGroups(token: string | undefined): Promise<UserGroup[]> {
  return requestItems<UserGroup>("/api/v1/groups", token)
}

export async function listGroups(token: string | undefined, tenantID?: string, placeID?: string): Promise<UserGroup[]> {
  let path = withTenantQuery("/api/v1/groups", tenantID)
  const nextPlaceID = placeID?.trim()
  if (nextPlaceID) {
    const separator = path.includes("?") ? "&" : "?"
    path = `${path}${separator}place_id=${encodeURIComponent(nextPlaceID)}`
  }
  return requestItems<UserGroup>(path, token)
}

export async function getGroup(token: string | undefined, groupID: string, tenantID?: string): Promise<UserGroup> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<UserGroup>(
    suffix ? `/api/v1/groups/${encodePathSegment(groupID)}?${suffix}` : `/api/v1/groups/${encodePathSegment(groupID)}`,
    { method: "GET" },
    token
  )
}

export async function createGroup(
  token: string | undefined,
  payload: {
    tenant_id: string
    place_id?: string
    building_id?: string
    name: string
    description?: string
    login_enabled?: boolean
    geofence_restriction_enabled?: boolean
    geofence_restriction_radius?: number
    primary_device_restriction_enabled?: boolean
    managed_device_restriction_enabled?: boolean
    reader_restriction_enabled?: boolean
    time_restriction_enabled?: boolean
    tap_to_access_restriction_enabled?: boolean
    time_restriction_time_zone?: string
    member_ids?: string[]
    members?: string[]
  }
): Promise<UserGroup> {
  return request<UserGroup>(
    "/api/v1/groups",
    {
      method: "POST",
      body: JSON.stringify({ group: payload }),
    },
    token
  )
}

export async function updateGroup(
  token: string | undefined,
  groupID: string,
  payload: {
    tenant_id?: string
    place_id?: string
    building_id?: string
    name?: string
    description?: string
    login_enabled?: boolean
    geofence_restriction_enabled?: boolean
    geofence_restriction_radius?: number
    primary_device_restriction_enabled?: boolean
    managed_device_restriction_enabled?: boolean
    reader_restriction_enabled?: boolean
    time_restriction_enabled?: boolean
    tap_to_access_restriction_enabled?: boolean
    time_restriction_time_zone?: string
    member_ids?: string[]
    members?: string[]
  }
): Promise<UserGroup> {
  return request<UserGroup>(
    `/api/v1/groups/${encodePathSegment(groupID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ group: payload }),
    },
    token
  )
}

export async function deleteGroup(token: string | undefined, groupID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/groups/${encodePathSegment(groupID)}?${suffix}` : `/api/v1/groups/${encodePathSegment(groupID)}`,
    { method: "DELETE" },
    token
  )
}

export async function createUserGroup(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    description?: string
    members?: string[]
  }
): Promise<UserGroup> {
  return request<UserGroup>(
    "/api/v1/groups",
    {
      method: "POST",
      body: JSON.stringify({ group: payload }),
    },
    token
  )
}

export async function updateUserGroup(
  token: string | undefined,
  groupID: string,
  payload: {
    name: string
    description?: string
    members?: string[]
  }
): Promise<UserGroup> {
  return request<UserGroup>(
    `/api/v1/groups/${encodePathSegment(groupID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ group: payload }),
    },
    token
  )
}

// --- Group Links ---

export async function listGroupLinks(
  token: string | undefined,
  options?: {
    tenant_id?: string
    group_id?: string
    ids?: string[]
    query?: string
    sort?: "name" | "-name" | "valid_until" | "-valid_until" | "group_name" | "-group_name"
  }
): Promise<GroupLink[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.group_id?.trim()) query.set("group_id", options.group_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<GroupLink>(suffix ? `/api/v1/group_links?${suffix}` : "/api/v1/group_links", token)
}

export async function createGroupLink(
  token: string | undefined,
  payload: {
    tenant_id: string
    group_id: string
    name: string
    email?: string
    phone?: string
    link_enabled?: boolean
    quick_response_code_type?: "online" | "offline" | ""
    valid_from?: string
    valid_until?: string
  }
): Promise<GroupLink> {
  return request<GroupLink>(
    "/api/v1/group_links",
    {
      method: "POST",
      body: JSON.stringify({ group_link: payload }),
    },
    token
  )
}

export async function getGroupLink(token: string | undefined, groupLinkID: string, tenantID?: string): Promise<GroupLink> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<GroupLink>(
    suffix ? `/api/v1/group_links/${encodePathSegment(groupLinkID)}?${suffix}` : `/api/v1/group_links/${encodePathSegment(groupLinkID)}`,
    { method: "GET" },
    token
  )
}

export async function verifyGroupLinkToken(
  token: string | undefined,
  payload: {
    tenant_id?: string
    token?: string
    secret?: string
    quick_response_code_token?: string
  }
): Promise<GroupLinkVerification> {
  return request<GroupLinkVerification>(
    "/api/v1/group_links/verify",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateGroupLink(
  token: string | undefined,
  groupLinkID: string,
  payload: {
    tenant_id?: string
    group_id?: string
    name?: string
    email?: string
    phone?: string
    link_enabled?: boolean
    quick_response_code_type?: "online" | "offline" | ""
    valid_from?: string
    valid_until?: string
  }
): Promise<GroupLink> {
  return request<GroupLink>(
    `/api/v1/group_links/${encodePathSegment(groupLinkID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ group_link: payload }),
    },
    token
  )
}

export async function deleteGroupLink(token: string | undefined, groupLinkID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/group_links/${encodePathSegment(groupLinkID)}?${suffix}` : `/api/v1/group_links/${encodePathSegment(groupLinkID)}`,
    { method: "DELETE" },
    token
  )
}

// --- Group Locks ---

export async function listGroupLocks(
  token: string | undefined,
  options?: {
    tenant_id?: string
    group_id?: string
    lock_id?: string
    place_id?: string
  }
): Promise<GroupLock[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.group_id?.trim()) query.set("group_id", options.group_id.trim())
  if (options?.lock_id?.trim()) query.set("lock_id", options.lock_id.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  const suffix = query.toString()
  return requestItems<GroupLock>(suffix ? `/api/v1/group_locks?${suffix}` : "/api/v1/group_locks", token)
}

export async function listGroupZones(
  token: string | undefined,
  options?: {
    tenant_id?: string
    group_id?: string
    zone_id?: string
    area_id?: string
    place_id?: string
  }
): Promise<GroupZone[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.group_id?.trim()) query.set("group_id", options.group_id.trim())
  if (options?.zone_id?.trim()) query.set("zone_id", options.zone_id.trim())
  if (options?.area_id?.trim()) query.set("area_id", options.area_id.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  const suffix = query.toString()
  return requestItems<GroupZone>(suffix ? `/api/v1/group_zones?${suffix}` : "/api/v1/group_zones", token)
}

export async function createGroupLock(
  token: string | undefined,
  payload: {
    tenant_id: string
    group_id: string
    lock_id: string
    door_id?: string
    place_id?: string
  }
): Promise<GroupLock> {
  return request<GroupLock>(
    "/api/v1/group_locks",
    {
      method: "POST",
      body: JSON.stringify({ group_lock: payload }),
    },
    token
  )
}

export async function deleteGroupLock(token: string | undefined, groupLockID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/group_locks/${encodePathSegment(groupLockID)}?${suffix}` : `/api/v1/group_locks/${encodePathSegment(groupLockID)}`,
    { method: "DELETE" },
    token
  )
}

// --- Group Terminals ---

export async function listGroupTerminals(token: string | undefined, tenantID?: string, groupID?: string): Promise<{ items: GroupTerminal[] }> {
  const q = new URLSearchParams(); if (tenantID) q.set("tenant_id", tenantID); if (groupID) q.set("group_id", groupID)
  return request(`/api/v1/group_terminals?${q}`, {}, token)
}
export async function createGroupTerminal(token: string | undefined, payload: { tenant_id?: string; group_id: string; terminal_id: string }): Promise<GroupTerminal> {
  return request("/api/v1/group_terminals", { method: "POST", body: JSON.stringify(payload) }, token)
}
export async function deleteGroupTerminal(token: string | undefined, id: string, tenantID?: string): Promise<void> {
  return request(withTenantQuery(`/api/v1/group_terminals/${encodePathSegment(id)}`, tenantID), { method: "DELETE" }, token)
}

// --- Presences ---

export async function listPresences(token: string | undefined, tenantID?: string, placeID?: string): Promise<{ items: Presence[] }> {
  const q = new URLSearchParams(); if (tenantID) q.set("tenant_id", tenantID); if (placeID) q.set("place_id", placeID)
  return request(`/api/v1/presences?${q}`, {}, token)
}

// --- CSV Card Imports ---

export async function listCSVCardImports(token: string | undefined, tenantID?: string): Promise<{ items: CSVCardImport[] }> {
  return request(withTenantQuery("/api/v1/csv_card_imports", tenantID), {}, token)
}
export async function createCSVCardImport(token: string | undefined, payload: { tenant_id?: string; file_name: string }): Promise<CSVCardImport> {
  return request("/api/v1/csv_card_imports", { method: "POST", body: JSON.stringify(payload) }, token)
}
export async function getCSVCardImport(token: string | undefined, id: string, tenantID?: string): Promise<CSVCardImport> {
  return request(withTenantQuery(`/api/v1/csv_card_imports/${encodePathSegment(id)}`, tenantID), {}, token)
}
