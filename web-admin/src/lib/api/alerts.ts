import { request, requestItems, withPageQuery, encodePathSegment } from "./core"
import type { ListPageOptions } from "./core"

export type AlertPolicy = {
  id: string
  resource_type: "AlertPolicy"
  tenant_id: string
  name: string
  description: string
  category: "enterprise_sync_worker" | "wallet_jobs" | string
  trigger: string
  severity: string
  condition_expression?: string
  status: "active" | "inactive" | string
  enabled: boolean
  threshold: number
  window_seconds: number
  cooldown_seconds: number
  channels: {
    email: boolean
    whatsapp: boolean
  }
  receiver_groups?: string[]
  updated_at: string
}

export type AlertPolicyConditionPreview = {
  policy_id?: string
  condition_expression: string
  matched: boolean
  event: Record<string, unknown>
}

export type AlertPolicyEventEvaluation = {
  tenant_id: string
  event: Record<string, unknown>
  evaluated_count: number
  matched_count: number
  matches: Array<{
    policy_id: string
    name: string
    trigger: string
    severity: string
    condition_expression?: string
    channels: AlertPolicy["channels"]
    receiver_groups: string[]
    window_seconds: number
    cooldown_seconds: number
    notification_summary: string
  }>
  errors?: Array<{
    policy_id: string
    error: string
  }>
}

export type Alarm = {
  id: string
  tenant_id: string
  building_id: string
  area_id: string
  door_id: string
  type: string
  severity: "critical" | "high" | "medium" | "low"
  location: string
  status:
    | "open"
    | "acknowledged"
    | "investigating"
    | "mitigated"
    | "escalated"
    | "resolved"
    | "false_positive"
  created_at: string
}

export type AlarmSchedule = {
  id: string
  tenant_id: string
  name: string
  description?: string
  days_of_week: number[]
  start_time: string
  end_time: string
  timezone: string
  alarm_types: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export type AlarmScheduleCalendarEntry = {
  schedule_id: string
  name: string
  day_of_week: number
  start_time: string
  end_time: string
  alarm_types: string[]
}

// --- Alert Policies ---

export async function listAlertPolicies(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    category?: string
    trigger?: string
    status?: string
    query?: string
    sort?: "name" | "-name"
  }
): Promise<AlertPolicy[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.category?.trim()) query.set("category", options.category.trim())
  if (options?.trigger?.trim()) query.set("trigger", options.trigger.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<AlertPolicy>(suffix ? `/api/v1/alert_policies?${suffix}` : "/api/v1/alert_policies", token)
}

export async function getAlertPolicy(token: string | undefined, policyID: string): Promise<AlertPolicy> {
  return request<AlertPolicy>(
    `/api/v1/alert_policies/${encodePathSegment(policyID)}`,
    { method: "GET" },
    token
  )
}

export async function createAlertPolicy(
  token: string | undefined,
  payload: {
    tenant_id: string
    category: "enterprise_sync_worker" | "wallet_jobs" | string
    name?: string
    description?: string
    trigger?: string
    severity?: string
    condition_expression?: string
    status?: string
    enabled?: boolean
    threshold?: number
    window_seconds?: number
    cooldown_seconds?: number
    channels?: {
      email?: boolean
      whatsapp?: boolean
    }
    receiver_groups?: string[]
    actor?: string
  }
): Promise<AlertPolicy> {
  return request<AlertPolicy>(
    "/api/v1/alert_policies",
    {
      method: "POST",
      body: JSON.stringify({ alert_policy: payload }),
    },
    token
  )
}

export async function updateAlertPolicy(
  token: string | undefined,
  policyID: string,
  payload: {
    tenant_id?: string
    name?: string
    description?: string
    trigger?: string
    severity?: string
    condition_expression?: string
    status?: string
    enabled?: boolean
    threshold?: number
    window_seconds?: number
    cooldown_seconds?: number
    channels?: {
      email?: boolean
      whatsapp?: boolean
    }
    receiver_groups?: string[]
    actor?: string
  }
): Promise<AlertPolicy> {
  return request<AlertPolicy>(
    `/api/v1/alert_policies/${encodePathSegment(policyID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ alert_policy: payload }),
    },
    token
  )
}

export async function deleteAlertPolicy(token: string | undefined, policyID: string): Promise<void> {
  return request<void>(
    `/api/v1/alert_policies/${encodePathSegment(policyID)}`,
    { method: "DELETE" },
    token
  )
}

export async function previewAlertPolicyCondition(
  token: string | undefined,
  payload: {
    tenant_id: string
    policy_id?: string
    condition_expression?: string
    event?: Record<string, unknown>
  }
): Promise<AlertPolicyConditionPreview> {
  return request<AlertPolicyConditionPreview>(
    "/api/v1/alert_policies/condition_preview",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function evaluateAlertPoliciesForEvent(
  token: string | undefined,
  payload: {
    tenant_id: string
    policy_ids?: string[]
    event: Record<string, unknown>
  }
): Promise<AlertPolicyEventEvaluation> {
  return request<AlertPolicyEventEvaluation>(
    "/api/v1/alert_policies/evaluate",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

// --- Alarms ---

export async function listAlarms(token: string | undefined, options?: ListPageOptions): Promise<Alarm[]> {
  return requestItems<Alarm>(withPageQuery("/api/v1/alarms", options), token)
}

export async function updateAlarmStatus(
  token: string | undefined,
  alarmID: string,
  status: "open" | "acknowledged" | "investigating" | "mitigated" | "escalated" | "resolved" | "false_positive"
): Promise<Alarm> {
  return request<Alarm>(
    `/api/v1/alarms/${encodePathSegment(alarmID)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify({ status }),
    },
    token
  )
}

// --- Alarm Schedules ---

export async function listAlarmSchedules(token: string | undefined, tenantID: string): Promise<{ items: AlarmSchedule[] }> {
  return request(`/api/v1/alarm-schedules?tenant_id=${tenantID}`, {}, token)
}

export async function getAlarmSchedule(token: string | undefined, scheduleID: string, tenantID: string): Promise<AlarmSchedule> {
  return request(`/api/v1/alarm-schedules/${scheduleID}?tenant_id=${tenantID}`, {}, token)
}

export async function createAlarmSchedule(token: string | undefined, payload: Partial<AlarmSchedule> & { tenant_id: string }): Promise<AlarmSchedule> {
  return request("/api/v1/alarm-schedules", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function updateAlarmSchedule(token: string | undefined, scheduleID: string, payload: Partial<AlarmSchedule> & { tenant_id: string }): Promise<AlarmSchedule> {
  return request(`/api/v1/alarm-schedules/${scheduleID}`, { method: "PATCH", body: JSON.stringify(payload) }, token)
}

export async function deleteAlarmSchedule(token: string | undefined, scheduleID: string, tenantID: string): Promise<void> {
  return request(`/api/v1/alarm-schedules/${scheduleID}?tenant_id=${tenantID}`, { method: "DELETE" }, token)
}

export async function getAlarmScheduleCalendar(token: string | undefined, tenantID: string): Promise<{ entries: AlarmScheduleCalendarEntry[] }> {
  return request(`/api/v1/alarm-schedules/calendar?tenant_id=${tenantID}`, {}, token)
}
