import { request, requestItems, withPageQuery, encodePathSegment } from "./core"
import type { ListPageOptions } from "./core"

export type AccessEvent = {
  id: string
  tenant_id: string
  building_id: string
  area_id: string
  type: string
  actor: string
  door_id: string
  gateway_id: string
  result: "success" | "denied" | "warning"
  at: string
}

export type DeviceEvent = {
  id: string
  tenant_id: string
  building_id: string
  type: string
  gateway_id: string
  detail: string
  result: "success" | "denied" | "warning"
  at: string
}

export type EventSetEvent = {
  uuid: string
  tenant_id?: string
  type: string
  actor_type?: string
  actor_id?: string
  actor_name?: string
  actor_email?: string
  object_type?: string
  object_id?: string
  object_name?: string
  place_id?: string
  area_id?: string
  lock_id?: string
  gateway_id?: string
  success: boolean
  result: string
  detail?: string
  created_at: string
}

export type EventSet = {
  id: string
  created_at: string
  status: "in_progress" | "finished" | "failed"
  interval?: string
  event_place_id?: string
  place_id?: string
  event_type?: string
  event_uuid?: string
  event_success?: boolean
  event_object_id?: string
  event_object_type?: string
  events: EventSetEvent[]
  cursor?: string
}

export type AuditLog = {
  id: string
  actor: string
  role: string
  action: string
  target: string
  source: string
  at: string
}

// --- Access Events ---

function accessEventFromEventSetEvent(item: EventSetEvent): AccessEvent {
  const result = item.result === "success" || item.result === "denied" || item.result === "warning" ? item.result : item.success ? "success" : "warning"
  return {
    id: item.uuid,
    tenant_id: item.tenant_id ?? "",
    building_id: item.place_id ?? "",
    area_id: item.area_id ?? "",
    type: item.type,
    actor: item.actor_name || item.actor_email || item.actor_id || "system",
    door_id: item.lock_id || item.object_id || "",
    gateway_id: item.gateway_id ?? "",
    result,
    at: item.created_at,
  }
}

export async function listAccessEvents(token: string | undefined, options?: ListPageOptions): Promise<AccessEvent[]> {
  const eventSet = await createEventSet(token, { event_object_type: "Lock" })
  const items = eventSet.events.map(accessEventFromEventSetEvent)
  const page = typeof options?.page === "number" && Number.isFinite(options.page) && options.page > 0 ? Math.floor(options.page) : 1
  const limit = typeof options?.limit === "number" && Number.isFinite(options.limit) && options.limit > 0 ? Math.floor(options.limit) : items.length
  return items.slice((page - 1) * limit, page * limit)
}

export async function listDeviceEvents(token: string | undefined, options?: ListPageOptions): Promise<DeviceEvent[]> {
  return requestItems<DeviceEvent>(withPageQuery("/api/v1/events/device", options), token)
}

// --- Event Sets ---

export async function createEventSet(
  token: string | undefined,
  payload?: {
    tenant_id?: string
    interval?: string
    place_id?: string
    event_place_id?: string
    event_type?: string
    event_uuid?: string
    event_success?: boolean
    event_object_id?: string
    event_object_type?: string
  }
): Promise<EventSet> {
  const query = new URLSearchParams()
  if (payload?.tenant_id?.trim()) query.set("tenant_id", payload.tenant_id.trim())
  const suffix = query.toString()
  return request<EventSet>(
    suffix ? `/api/v1/event_sets?${suffix}` : "/api/v1/event_sets",
    {
      method: "POST",
      body: JSON.stringify({
        event_set: {
          interval: payload?.interval,
          place_id: payload?.place_id,
          event_place_id: payload?.event_place_id,
          event_type: payload?.event_type,
          event_uuid: payload?.event_uuid,
          event_success: payload?.event_success,
          event_object_id: payload?.event_object_id,
          event_object_type: payload?.event_object_type,
        },
      }),
    },
    token
  )
}

export async function getEventSet(
  token: string | undefined,
  eventSetID: string,
  options?: {
    tenant_id?: string
    place_id?: string
    event_type?: string
    event_uuid?: string
    interval?: string
  }
): Promise<EventSet> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.event_type?.trim()) query.set("event_type", options.event_type.trim())
  if (options?.event_uuid?.trim()) query.set("event_uuid", options.event_uuid.trim())
  if (options?.interval?.trim()) query.set("interval", options.interval.trim())
  const suffix = query.toString()
  return request<EventSet>(
    suffix ? `/api/v1/event_sets/${encodePathSegment(eventSetID)}?${suffix}` : `/api/v1/event_sets/${encodePathSegment(eventSetID)}`,
    { method: "GET" },
    token
  )
}

export async function getEventMetadata(token: string | undefined): Promise<{ object_type_to_action: Record<string, string[]> }> {
  return request<{ object_type_to_action: Record<string, string[]> }>("/api/v1/events/meta", { method: "GET" }, token)
}

export async function listEventTypes(token: string | undefined): Promise<string[]> {
  return request<string[]>("/api/v1/events/types", { method: "GET" }, token)
}

// --- Audit Logs ---

export async function listAuditLogs(token: string | undefined): Promise<AuditLog[]> {
  return requestItems<AuditLog>("/api/v1/audit-logs", token)
}
