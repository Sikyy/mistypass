import { request, withTenantQuery, encodePathSegment } from "./core"

export type Elevator = {
  id: string
  tenant_id: string
  place_id: string
  name: string
  description?: string
  elevator_stops_count: number
  created_at: string
  updated_at: string
}

export type ElevatorStop = {
  id: string
  tenant_id: string
  elevator_id: string
  floor_id?: string
  name: string
  status: string
  created_at: string
  updated_at: string
}

export type GroupElevatorStop = { id: string; tenant_id: string; group_id: string; elevator_stop_id: string; created_at: string }

export async function listElevators(token: string | undefined, tenantID?: string): Promise<{ items: Elevator[] }> {
  return request(withTenantQuery("/api/v1/elevators", tenantID), {}, token)
}
export async function createElevator(token: string | undefined, payload: { tenant_id?: string; place_id: string; name: string; description?: string }): Promise<Elevator> {
  return request("/api/v1/elevators", { method: "POST", body: JSON.stringify(payload) }, token)
}
export async function getElevator(token: string | undefined, id: string, tenantID?: string): Promise<Elevator> {
  return request(withTenantQuery(`/api/v1/elevators/${encodePathSegment(id)}`, tenantID), {}, token)
}
export async function updateElevator(token: string | undefined, id: string, payload: { name?: string; description?: string }): Promise<Elevator> {
  return request(`/api/v1/elevators/${encodePathSegment(id)}`, { method: "PATCH", body: JSON.stringify(payload) }, token)
}
export async function deleteElevator(token: string | undefined, id: string, tenantID?: string): Promise<void> {
  return request(withTenantQuery(`/api/v1/elevators/${encodePathSegment(id)}`, tenantID), { method: "DELETE" }, token)
}

export async function listElevatorStops(token: string | undefined, tenantID?: string, elevatorID?: string): Promise<{ items: ElevatorStop[] }> {
  const q = new URLSearchParams(); if (tenantID) q.set("tenant_id", tenantID); if (elevatorID) q.set("elevator_id", elevatorID)
  return request(`/api/v1/elevator_stops?${q}`, {}, token)
}
export async function createElevatorStop(token: string | undefined, payload: { tenant_id?: string; elevator_id: string; floor_id?: string; name: string }): Promise<ElevatorStop> {
  return request("/api/v1/elevator_stops", { method: "POST", body: JSON.stringify(payload) }, token)
}
export async function deleteElevatorStop(token: string | undefined, id: string, tenantID?: string): Promise<void> {
  return request(withTenantQuery(`/api/v1/elevator_stops/${encodePathSegment(id)}`, tenantID), { method: "DELETE" }, token)
}
export async function lockDownElevatorStop(token: string | undefined, id: string, tenantID?: string): Promise<ElevatorStop> {
  return request(withTenantQuery(`/api/v1/elevator_stops/${encodePathSegment(id)}/lock_down`, tenantID), { method: "POST" }, token)
}
export async function cancelElevatorStopLockdown(token: string | undefined, id: string, tenantID?: string): Promise<ElevatorStop> {
  return request(withTenantQuery(`/api/v1/elevator_stops/${encodePathSegment(id)}/cancel_lockdown`, tenantID), { method: "POST" }, token)
}

export async function listGroupElevatorStops(token: string | undefined, tenantID?: string, groupID?: string): Promise<{ items: GroupElevatorStop[] }> {
  const q = new URLSearchParams(); if (tenantID) q.set("tenant_id", tenantID); if (groupID) q.set("group_id", groupID)
  return request(`/api/v1/group_elevator_stops?${q}`, {}, token)
}
export async function createGroupElevatorStop(token: string | undefined, payload: { tenant_id?: string; group_id: string; elevator_stop_id: string }): Promise<GroupElevatorStop> {
  return request("/api/v1/group_elevator_stops", { method: "POST", body: JSON.stringify(payload) }, token)
}
export async function deleteGroupElevatorStop(token: string | undefined, id: string, tenantID?: string): Promise<void> {
  return request(withTenantQuery(`/api/v1/group_elevator_stops/${encodePathSegment(id)}`, tenantID), { method: "DELETE" }, token)
}
