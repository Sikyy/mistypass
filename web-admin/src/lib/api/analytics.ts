import { request, resolveAuthToken, API_BASE_URL } from "./core"

export type AccessSummary = {
  period: { start: string; end: string }
  total_events: number
  by_result: Record<string, number>
  by_door: { door_id: string; door_name: string; count: number }[]
  by_day: { date: string; granted: number; denied: number }[]
  peak_hour: number
}

export type DoorActivity = {
  doors: { door_id: string; total_access: number; unique_users: number; hourly_distribution: number[] }[]
}

export type AlarmMetrics = {
  total: number
  by_severity: Record<string, number>
  by_status: Record<string, number>
  mean_resolution_minutes: number
}

export type NetworkTopology = {
  nodes: { id: string; type: string; label: string; status: string; ip?: string; last_seen?: string; protocol?: string }[]
  edges: { source: string; target: string; protocol: string }[]
  summary: { total_gateways: number; total_devices: number; online: number; offline: number }
}

export type ReportSchedule = {
  id: string
  tenant_id: string
  name: string
  report_type: string
  frequency: string
  recipients: string[]
  format: string
  day_of_week: number
  enabled: boolean
  last_sent_at?: string
  created_at: string
  updated_at: string
}

export async function getAccessSummary(token: string | undefined, tenantID: string, start: string, end: string, buildingID?: string): Promise<AccessSummary> {
  const params = new URLSearchParams({ tenant_id: tenantID, start, end })
  if (buildingID) params.set("building_id", buildingID)
  return request<AccessSummary>(`/api/v1/analytics/access-summary?${params}`, {}, token)
}

export async function getDoorActivity(token: string | undefined, tenantID: string, options?: { door_id?: string; building_id?: string; days?: number }): Promise<DoorActivity> {
  const params = new URLSearchParams({ tenant_id: tenantID })
  if (options?.door_id) params.set("door_id", options.door_id)
  if (options?.building_id) params.set("building_id", options.building_id)
  if (options?.days) params.set("days", String(options.days))
  return request<DoorActivity>(`/api/v1/analytics/door-activity?${params}`, {}, token)
}

export async function getAlarmMetrics(token: string | undefined, tenantID: string, start: string, end: string): Promise<AlarmMetrics> {
  const params = new URLSearchParams({ tenant_id: tenantID, start, end })
  return request<AlarmMetrics>(`/api/v1/analytics/alarm-metrics?${params}`, {}, token)
}

export async function exportAnalytics(token: string | undefined, tenantID: string, type: string, format: string, start: string, end: string): Promise<Blob> {
  const params = new URLSearchParams({ tenant_id: tenantID, type, format, start, end })
  const activeToken = resolveAuthToken(token)
  const res = await fetch(`${API_BASE_URL}/api/v1/analytics/export?${params}`, {
    headers: activeToken ? { Authorization: `Bearer ${activeToken}` } : {},
  })
  if (!res.ok) throw new Error(`Export failed: ${res.status}`)
  return res.blob()
}

export async function getNetworkTopology(token: string | undefined, tenantID: string): Promise<NetworkTopology> {
  return request(`/api/v1/network/topology?tenant_id=${tenantID}`, {}, token)
}

export async function listReportSchedules(token: string | undefined, tenantID: string): Promise<{ items: ReportSchedule[] }> {
  return request(`/api/v1/report-schedules?tenant_id=${tenantID}`, {}, token)
}

export async function getReportSchedule(token: string | undefined, scheduleID: string, tenantID: string): Promise<ReportSchedule> {
  return request(`/api/v1/report-schedules/${scheduleID}?tenant_id=${tenantID}`, {}, token)
}

export async function createReportSchedule(token: string | undefined, payload: Partial<ReportSchedule> & { tenant_id: string }): Promise<ReportSchedule> {
  return request("/api/v1/report-schedules", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function updateReportSchedule(token: string | undefined, scheduleID: string, payload: Partial<ReportSchedule> & { tenant_id: string }): Promise<ReportSchedule> {
  return request(`/api/v1/report-schedules/${scheduleID}`, { method: "PATCH", body: JSON.stringify(payload) }, token)
}

export async function deleteReportSchedule(token: string | undefined, scheduleID: string, tenantID: string): Promise<void> {
  return request(`/api/v1/report-schedules/${scheduleID}?tenant_id=${tenantID}`, { method: "DELETE" }, token)
}

export async function listCameras(token: string | undefined, tenantID: string): Promise<{ items: any[]; message: string }> {
  return request(`/api/v1/cameras?tenant_id=${tenantID}`, {}, token)
}
