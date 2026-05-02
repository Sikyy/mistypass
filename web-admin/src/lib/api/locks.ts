import { request, requestItems, withTenantQuery, encodePathSegment } from "./core"
import type { SpaceActionResult } from "./spaces"

export type Door = {
  id: string
  tenant_id: string
  building_id: string
  floor_id: string
  area_id: string
  name: string
  gateway_id: string
  kind: "office" | "turnstile" | "server-room" | "elevator" | "parking-gate" | "emergency-exit"
  status: "online" | "offline"
  created_at: string
}

export type Lock = Door

export type Controller = {
  id: string
  resource_type: "Controller"
  tenant_id: string
  place_id: string
  name: string
  description?: string
  device_id: string
  token: string
  status: "online" | "offline"
  configured: boolean
  lock_ids?: string[]
  last_seen_at: string
  created_at: string
  updated_at: string
}

export type Reader = {
  id: string
  resource_type: "Reader"
  tenant_id: string
  place_id: string
  controller_id: string
  name: string
  description?: string
  device_id: string
  token: string
  model: string
  protocol: string
  status: "online" | "offline"
  configured: boolean
  lock_ids?: string[]
  last_seen_at: string
  created_at: string
  updated_at: string
}

export type Terminal = {
  id: string
  resource_type: "Terminal"
  tenant_id: string
  created_at: string
  updated_at: string
  name: string
  description: string
  place_id: string
  place?: {
    id: string
    resource_type: "Place"
    name: string
  }
  marketplace_installation_id?: string | null
  controller_id?: string
  reader_id?: string
  status?: "online" | "offline"
  last_seen_at?: string
}

export type GatewayCommandAck = {
  task_id: string
  gateway_id: string
  command: string
  status: string
  created_at: string
}

// --- Doors / Locks ---

export async function listDoors(token: string | undefined, tenantID?: string): Promise<Door[]> {
  return requestItems<Door>(withTenantQuery("/api/v1/locks", tenantID), token)
}

export async function listLocks(token: string | undefined, tenantID?: string, placeID?: string): Promise<Lock[]> {
  let path = withTenantQuery("/api/v1/locks", tenantID)
  const nextPlaceID = placeID?.trim()
  if (nextPlaceID) {
    const separator = path.includes("?") ? "&" : "?"
    path = `${path}${separator}place_id=${encodeURIComponent(nextPlaceID)}`
  }
  return requestItems<Lock>(path, token)
}

export async function createLock(
  token: string | undefined,
  payload: {
    tenant_id: string
    place_id?: string
    building_id?: string
    floor_id: string
    area_id: string
    name: string
    gateway_id?: string
    kind?: Door["kind"]
    status?: Door["status"]
  }
): Promise<Lock> {
  return request<Lock>(
    "/api/v1/locks",
    {
      method: "POST",
      body: JSON.stringify({ lock: payload }),
    },
    token
  )
}

export async function getLock(token: string | undefined, lockID: string, tenantID?: string): Promise<Lock> {
  return request<Lock>(
    withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function updateLock(
  token: string | undefined,
  lockID: string,
  payload: {
    tenant_id?: string
    place_id?: string
    building_id?: string
    floor_id?: string
    area_id?: string
    name?: string
    gateway_id?: string
    kind?: Door["kind"]
    status?: Door["status"]
  }
): Promise<Lock> {
  return request<Lock>(
    `/api/v1/locks/${encodePathSegment(lockID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ lock: payload }),
    },
    token
  )
}

export async function deleteLock(token: string | undefined, lockID: string, tenantID?: string): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

export async function unlockLock(token: string | undefined, lockID: string, tenantID?: string): Promise<SpaceActionResult> {
  return request<SpaceActionResult>(
    withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}/unlock`, tenantID),
    { method: "POST" },
    token
  )
}

export async function lockDownLock(token: string | undefined, lockID: string, tenantID?: string): Promise<SpaceActionResult> {
  return request<SpaceActionResult>(
    withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}/lock_down`, tenantID),
    { method: "POST" },
    token
  )
}

export async function cancelLockLockdown(token: string | undefined, lockID: string, tenantID?: string): Promise<SpaceActionResult> {
  return request<SpaceActionResult>(
    withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}/cancel_lockdown`, tenantID),
    { method: "POST" },
    token
  )
}

export async function createDoor(
  token: string | undefined,
  payload: {
    tenant_id: string
    building_id: string
    floor_id: string
    area_id: string
    name: string
    gateway_id?: string
    kind?: "office" | "turnstile" | "server-room" | "elevator" | "parking-gate" | "emergency-exit"
    status?: "online" | "offline"
  }
): Promise<Door> {
  return request<Door>(
    "/api/v1/locks",
    {
      method: "POST",
      body: JSON.stringify({ lock: payload }),
    },
    token
  )
}

export async function favoriteLock(token: string | undefined, lockID: string): Promise<{ lock_id: string; favorited: boolean }> {
  return request(`/api/v1/locks/${encodePathSegment(lockID)}/favorite`, { method: "POST" }, token)
}

export async function unfavoriteLock(token: string | undefined, lockID: string): Promise<{ lock_id: string; favorited: boolean }> {
  return request(`/api/v1/locks/${encodePathSegment(lockID)}/unfavorite`, { method: "POST" }, token)
}

export async function firstToArriveLock(token: string | undefined, lockID: string, tenantID?: string): Promise<any> {
  return request(withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}/first_to_arrive`, tenantID), { method: "POST" }, token)
}

export async function lastToLeaveLock(token: string | undefined, lockID: string, tenantID?: string): Promise<any> {
  return request(withTenantQuery(`/api/v1/locks/${encodePathSegment(lockID)}/last_to_leave`, tenantID), { method: "POST" }, token)
}

// --- Controllers ---

export async function listControllers(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    place_id?: string
    lock_id?: string
    status?: Controller["status"]
    sort?: "name" | "-name"
  }
): Promise<Controller[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.lock_id?.trim()) query.set("lock_id", options.lock_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Controller>(suffix ? `/api/v1/controllers?${suffix}` : "/api/v1/controllers", token)
}

export async function listReaders(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    model?: string
    place_id?: string
    lock_id?: string
    status?: Reader["status"]
    sort?: "name" | "-name"
  }
): Promise<Reader[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.model?.trim()) query.set("model", options.model.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.lock_id?.trim()) query.set("lock_id", options.lock_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Reader>(suffix ? `/api/v1/readers?${suffix}` : "/api/v1/readers", token)
}

export async function listTerminals(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    place_id?: string
    status?: Terminal["status"]
    sort?: "name" | "-name"
  }
): Promise<Terminal[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Terminal>(suffix ? `/api/v1/terminals?${suffix}` : "/api/v1/terminals", token)
}

export async function fetchTerminal(
  token: string | undefined,
  terminalID: string,
  tenantID?: string
): Promise<Terminal> {
  return request<Terminal>(
    withTenantQuery(`/api/v1/terminals/${encodePathSegment(terminalID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function assignController(
  token: string | undefined,
  controllerToken: string,
  payload: {
    tenant_id: string
    place_id?: string
    building_id?: string
    device_capacity?: number
  }
): Promise<Controller> {
  return request<Controller>(
    `/api/v1/controllers/${encodePathSegment(controllerToken)}/assign`,
    {
      method: "POST",
      body: JSON.stringify({ controller: payload }),
    },
    token
  )
}

export async function assignReader(
  token: string | undefined,
  readerToken: string,
  payload: {
    tenant_id: string
    controller_id: string
    kind?: "reader" | "door_controller" | "relay" | "sensor" | "legacy_reader" | "legacy_controller"
    source?: "mistypass_procured" | "legacy_integration"
    protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"
    status?: "online" | "offline"
  }
): Promise<Reader> {
  return request<Reader>(
    `/api/v1/readers/${encodePathSegment(readerToken)}/assign`,
    {
      method: "POST",
      body: JSON.stringify({ reader: payload }),
    },
    token
  )
}

export async function deassignController(
  token: string | undefined,
  controllerID: string,
  tenantID?: string
): Promise<Controller> {
  return request<Controller>(
    withTenantQuery(`/api/v1/controllers/${encodePathSegment(controllerID)}/deassign`, tenantID),
    { method: "POST" },
    token
  )
}

export async function deassignReader(
  token: string | undefined,
  readerID: string,
  tenantID?: string
): Promise<Reader> {
  return request<Reader>(
    withTenantQuery(`/api/v1/readers/${encodePathSegment(readerID)}/deassign`, tenantID),
    { method: "POST" },
    token
  )
}

export async function bindControllerLock(
  token: string | undefined,
  controllerID: string,
  lockID: string,
  tenantID?: string
): Promise<Controller> {
  return request<Controller>(
    withTenantQuery(`/api/v1/controllers/${encodePathSegment(controllerID)}/locks`, tenantID),
    {
      method: "POST",
      body: JSON.stringify({ tenant_id: tenantID, lock_id: lockID }),
    },
    token
  )
}

export async function unbindControllerLock(
  token: string | undefined,
  controllerID: string,
  lockID: string,
  tenantID?: string
): Promise<void> {
  return request<void>(
    withTenantQuery(
      `/api/v1/controllers/${encodePathSegment(controllerID)}/locks/${encodePathSegment(lockID)}`,
      tenantID
    ),
    { method: "DELETE" },
    token
  )
}

export async function publishControllerConfig(
  token: string | undefined,
  controllerID: string,
  version: string,
  tenantID?: string
): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    withTenantQuery(`/api/v1/controllers/${encodePathSegment(controllerID)}/config/publish`, tenantID),
    {
      method: "POST",
      body: JSON.stringify({ controller: { tenant_id: tenantID, version } }),
    },
    token
  )
}

export async function rebootController(
  token: string | undefined,
  controllerID: string,
  tenantID?: string
): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    withTenantQuery(`/api/v1/controllers/${encodePathSegment(controllerID)}/reboot`, tenantID),
    { method: "POST" },
    token
  )
}

export async function rebootReader(
  token: string | undefined,
  readerID: string,
  tenantID?: string
): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    withTenantQuery(`/api/v1/readers/${encodePathSegment(readerID)}/reboot`, tenantID),
    { method: "POST" },
    token
  )
}

export async function rebootTerminal(
  token: string | undefined,
  terminalID: string,
  tenantID?: string
): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    withTenantQuery(`/api/v1/terminals/${encodePathSegment(terminalID)}/reboot`, tenantID),
    { method: "POST" },
    token
  )
}

export async function triggerTerminal(
  token: string | undefined,
  terminalID: string,
  tenantID?: string
): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/terminals/${encodePathSegment(terminalID)}/trigger`, tenantID),
    { method: "POST" },
    token
  )
}

export async function resetTamperReader(token: string | undefined, readerID: string, tenantID?: string): Promise<{ status: string }> {
  return request(withTenantQuery(`/api/v1/readers/${encodePathSegment(readerID)}/reset_tamper`, tenantID), { method: "POST" }, token)
}
