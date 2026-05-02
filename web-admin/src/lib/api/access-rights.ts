import { request, requestItems, withTenantQuery, encodePathSegment } from "./core"

export type Role = {
  id: string
  name: string
  applies_to: "Organization" | "Place" | "Group"
  description?: string
  permissions: Record<string, boolean>
  built_in: boolean
}

export type RoleAssignment = {
  id: string
  tenant_id: string
  role_id: string
  applies_to_type: "Organization" | "Place" | "Group"
  applies_to_id: string
  assignee_type: "User" | "Team" | "Guest"
  assignee_id: string
  assignee_email?: string
  valid_from?: string
  valid_until?: string
  reviewed_at?: string
  reviewed_by?: string
  created_at: string
  updated_at: string
}

export type Share = {
  id: string
  tenant_id: string
  email: string
  group_id?: string
  role_id: string
  place_id?: string
  area_id?: string
  lock_id?: string
  valid_from?: string
  valid_until: string
  status: string
  delivery_method: "wallet" | "email_qr"
  grantee_name?: string
  grantee_phone?: string
  mobile_model?: string
  pass_type?: string
  authorized_by_id?: string
  authorized_by_email?: string
  authorized_by_role?: string
  authorized_at?: string
  reviewed_at?: string
  reviewed_by?: string
  created_at: string
}

export type AccessRightImpactItem = {
  source_type: "role_assignment" | "share"
  id: string
  name: string
  target: string
  status: string
  needs_review: boolean
  reviewed_at?: string
  reviewed_by?: string
  affected_users: number
  affected_teams: number
  affected_groups: number
  affected_places: number
  affected_locks: number
  role_id?: string
  applies_to_type?: RoleAssignment["applies_to_type"]
  applies_to_id?: string
  assignee_type?: RoleAssignment["assignee_type"]
  assignee_id?: string
  provider_record_id?: string
}

export type AccessRightsImpactPreview = {
  tenant_id: string
  selected_count: number
  needs_review_count: number
  affected_users: number
  affected_teams: number
  affected_groups: number
  affected_places: number
  affected_locks: number
  items: AccessRightImpactItem[]
}

export type AccessRightsSelectionPayload = {
  tenant_id: string
  role_assignment_ids?: string[]
  share_ids?: string[]
  reviewed_by?: string
}

export type AccessRightsReviewResult = {
  tenant_id: string
  reviewed_at: string
  reviewed_by: string
  reviewed_count: number
  skipped_count: number
  reviewed_role_assignment_ids?: string[]
  reviewed_share_ids?: string[]
}

export type AccessRightsScheduleUpdateResult = {
  tenant_id: string
  valid_from?: string
  valid_until: string
  updated_count: number
  updated_role_assignment_ids?: string[]
  updated_share_ids?: string[]
}

export type TimeWindow = {
  start_time: string
  end_time: string
  day_of_week_set: string
  timezone?: string
}

export type HolidayEntry = {
  date: string
  name: string
  description?: string
}

export type HolidayCalendar = {
  id: string
  tenant_id: string
  name: string
  country?: string
  entries: HolidayEntry[]
  updated_at: string
}

export type ScheduleEvaluation = {
  is_active: boolean
  reason: string
  valid_from?: string
  valid_until?: string
  time_windows?: TimeWindow[]
  evaluated_at: string
}

export type AccessRightsScheduleTemplate = {
  id: string
  name: string
  description?: string
  valid_from?: string
  valid_until?: string
  duration_days?: number
  time_windows?: TimeWindow[]
  source_types?: Array<AccessRightImpactItem["source_type"]>
}

export type HolidayPresetCountry = {
  code: string
  name: string
}

export type HolidayPresetsResponse = {
  country: string
  country_name: string
  year: number
  entries: HolidayEntry[]
}

export type Schedule = {
  id: string
  tenant_id: string
  name: string
  description?: string
  valid_from?: string
  valid_until?: string
  time_windows?: TimeWindow[]
  exception_dates?: string[]
  holiday_calendar_id?: string
  created_at: string
  updated_at: string
}

// --- Roles ---

export async function listRoles(token: string | undefined, appliesTo?: Role["applies_to"]): Promise<Role[]> {
  const path = appliesTo ? `/api/v1/roles?applies_to=${encodeURIComponent(appliesTo)}` : "/api/v1/roles"
  return requestItems<Role>(path, token)
}

// --- Role Assignments ---

export async function listRoleAssignments(
  token: string | undefined,
  options?: {
    tenant_id?: string
    role_id?: string
    applies_to_type?: RoleAssignment["applies_to_type"]
    applies_to_id?: string
    assignee_type?: RoleAssignment["assignee_type"]
    assignee_id?: string
  }
): Promise<RoleAssignment[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.role_id?.trim()) query.set("role_id", options.role_id.trim())
  if (options?.applies_to_type?.trim()) query.set("applies_to_type", options.applies_to_type.trim())
  if (options?.applies_to_id?.trim()) query.set("applies_to_id", options.applies_to_id.trim())
  if (options?.assignee_type?.trim()) query.set("assignee_type", options.assignee_type.trim())
  if (options?.assignee_id?.trim()) query.set("assignee_id", options.assignee_id.trim())
  const suffix = query.toString()
  return requestItems<RoleAssignment>(suffix ? `/api/v1/role_assignments?${suffix}` : "/api/v1/role_assignments", token)
}

export async function getRoleAssignment(token: string | undefined, assignmentID: string, tenantID?: string): Promise<RoleAssignment> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<RoleAssignment>(
    suffix ? `/api/v1/role_assignments/${encodePathSegment(assignmentID)}?${suffix}` : `/api/v1/role_assignments/${encodePathSegment(assignmentID)}`,
    { method: "GET" },
    token
  )
}

export async function createRoleAssignment(
  token: string | undefined,
  payload: {
    tenant_id: string
    role_id: string
    applies_to_type: RoleAssignment["applies_to_type"]
    applies_to_id: string
    assignee_type: RoleAssignment["assignee_type"]
    assignee_id: string
    assignee_email?: string
    valid_from?: string
    valid_until?: string
  }
): Promise<RoleAssignment> {
  return request<RoleAssignment>(
    "/api/v1/role_assignments",
    {
      method: "POST",
      body: JSON.stringify({ role_assignment: payload }),
    },
    token
  )
}

export async function updateRoleAssignment(
  token: string | undefined,
  assignmentID: string,
  payload: {
    tenant_id?: string
    role_id: string
    applies_to_type: RoleAssignment["applies_to_type"]
    applies_to_id: string
    assignee_type: RoleAssignment["assignee_type"]
    assignee_id: string
    assignee_email?: string
    valid_from?: string
    valid_until?: string
  }
): Promise<RoleAssignment> {
  return request<RoleAssignment>(
    `/api/v1/role_assignments/${encodePathSegment(assignmentID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ role_assignment: payload }),
    },
    token
  )
}

export async function deleteRoleAssignment(token: string | undefined, assignmentID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/role_assignments/${encodePathSegment(assignmentID)}?${suffix}` : `/api/v1/role_assignments/${encodePathSegment(assignmentID)}`,
    { method: "DELETE" },
    token
  )
}

// --- Shares ---

export async function listShares(
  token: string | undefined,
  options?: {
    tenant_id?: string
    group_id?: string
    place_id?: string
    area_id?: string
    lock_id?: string
    role_id?: string
    email?: string
  }
): Promise<Share[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.group_id?.trim()) query.set("group_id", options.group_id.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.area_id?.trim()) query.set("area_id", options.area_id.trim())
  if (options?.lock_id?.trim()) query.set("lock_id", options.lock_id.trim())
  if (options?.role_id?.trim()) query.set("role_id", options.role_id.trim())
  if (options?.email?.trim()) query.set("email", options.email.trim())
  const suffix = query.toString()
  return requestItems<Share>(suffix ? `/api/v1/shares?${suffix}` : "/api/v1/shares", token)
}

export async function getShare(token: string | undefined, shareID: string, tenantID?: string): Promise<Share> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<Share>(
    suffix ? `/api/v1/shares/${encodePathSegment(shareID)}?${suffix}` : `/api/v1/shares/${encodePathSegment(shareID)}`,
    { method: "GET" },
    token
  )
}

export async function createShare(
  token: string | undefined,
  payload: {
    tenant_id: string
    email: string
    group_id?: string
    role_id?: string
    place_id?: string
    area_id?: string
    lock_id?: string
    valid_from?: string
    valid_until: string
    delivery_method?: Share["delivery_method"]
    grantee_name?: string
    grantee_phone?: string
    mobile_model?: string
    pass_type?: string
  }
): Promise<Share> {
  return request<Share>(
    "/api/v1/shares",
    {
      method: "POST",
      body: JSON.stringify({ share: payload }),
    },
    token
  )
}

export async function updateShare(
  token: string | undefined,
  shareID: string,
  payload: {
    tenant_id?: string
    email?: string
    group_id?: string
    role_id?: string
    place_id?: string
    area_id?: string
    lock_id?: string
    valid_from?: string
    valid_until?: string
    delivery_method?: Share["delivery_method"]
    grantee_name?: string
    grantee_phone?: string
    mobile_model?: string
    pass_type?: string
  }
): Promise<Share> {
  return request<Share>(
    `/api/v1/shares/${encodePathSegment(shareID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ share: payload }),
    },
    token
  )
}

export async function deleteShare(token: string | undefined, shareID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/shares/${encodePathSegment(shareID)}?${suffix}` : `/api/v1/shares/${encodePathSegment(shareID)}`,
    { method: "DELETE" },
    token
  )
}

// --- Access Rights Impact & Review ---

export async function previewAccessRightsImpact(
  token: string | undefined,
  payload: AccessRightsSelectionPayload
): Promise<AccessRightsImpactPreview> {
  return request<AccessRightsImpactPreview>(
    "/api/v1/access_rights/impact_preview",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function reviewAccessRights(
  token: string | undefined,
  payload: AccessRightsSelectionPayload
): Promise<AccessRightsReviewResult> {
  return request<AccessRightsReviewResult>(
    "/api/v1/access_rights/review",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateAccessRightsSchedule(
  token: string | undefined,
  payload: AccessRightsSelectionPayload & {
    valid_from?: string
    valid_until: string
  }
): Promise<AccessRightsScheduleUpdateResult> {
  return request<AccessRightsScheduleUpdateResult>(
    "/api/v1/access_rights/schedule",
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listAccessRightsScheduleTemplates(
  token: string | undefined,
  tenantID?: string
): Promise<AccessRightsScheduleTemplate[]> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return requestItems<AccessRightsScheduleTemplate>(
    suffix ? `/api/v1/access_rights/schedule_templates?${suffix}` : "/api/v1/access_rights/schedule_templates",
    token
  )
}

export async function evaluateAccessRightsSchedule(
  token: string | undefined,
  payload: {
    tenant_id?: string
    valid_from?: string
    valid_until?: string
    time_windows?: TimeWindow[]
    exception_dates?: string[]
    holiday_calendar_id?: string
    evaluate_at?: string
  }
): Promise<ScheduleEvaluation> {
  return request<ScheduleEvaluation>(
    "/api/v1/access_rights/schedule/evaluate",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

// --- Holiday Calendars ---

export async function listHolidayCalendars(
  token: string | undefined,
  tenantID?: string
): Promise<HolidayCalendar[]> {
  return requestItems<HolidayCalendar>(
    withTenantQuery("/api/v1/holiday_calendars", tenantID),
    token
  )
}

export async function createHolidayCalendar(
  token: string | undefined,
  payload: { tenant_id: string; name: string; country?: string; entries: HolidayEntry[] }
): Promise<HolidayCalendar> {
  return request<HolidayCalendar>(
    "/api/v1/holiday_calendars",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function updateHolidayCalendar(
  token: string | undefined,
  calendarID: string,
  payload: { name?: string; country?: string; entries?: HolidayEntry[] }
): Promise<HolidayCalendar> {
  return request<HolidayCalendar>(
    `/api/v1/holiday_calendars/${encodePathSegment(calendarID)}`,
    { method: "PATCH", body: JSON.stringify(payload) },
    token
  )
}

export async function deleteHolidayCalendar(
  token: string | undefined,
  calendarID: string,
  tenantID?: string
): Promise<void> {
  await request<void>(
    withTenantQuery(`/api/v1/holiday_calendars/${encodePathSegment(calendarID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

export async function listHolidayCalendarPresetCountries(
  token: string | undefined
): Promise<HolidayPresetCountry[]> {
  return requestItems<HolidayPresetCountry>(
    "/api/v1/holiday_calendars/preset_countries",
    token
  )
}

export async function listHolidayCalendarPresets(
  token: string | undefined,
  country: string,
  year?: number
): Promise<HolidayPresetsResponse> {
  let path = `/api/v1/holiday_calendars/presets?country=${encodeURIComponent(country)}`
  if (year) path += `&year=${year}`
  return request<HolidayPresetsResponse>(path, {}, token)
}

// --- Schedules ---

export async function listSchedules(
  token: string | undefined,
  tenantID?: string
): Promise<Schedule[]> {
  return requestItems<Schedule>(
    withTenantQuery("/api/v1/schedules", tenantID),
    token
  )
}

export async function getSchedule(
  token: string | undefined,
  scheduleID: string,
  tenantID?: string
): Promise<Schedule> {
  return request<Schedule>(
    withTenantQuery(`/api/v1/schedules/${encodePathSegment(scheduleID)}`, tenantID),
    {},
    token
  )
}

export async function createSchedule(
  token: string | undefined,
  payload: Partial<Schedule> & { tenant_id?: string }
): Promise<Schedule> {
  return request<Schedule>(
    "/api/v1/schedules",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function updateSchedule(
  token: string | undefined,
  scheduleID: string,
  payload: Partial<Schedule>,
  tenantID?: string
): Promise<Schedule> {
  return request<Schedule>(
    withTenantQuery(`/api/v1/schedules/${encodePathSegment(scheduleID)}`, tenantID),
    { method: "PATCH", body: JSON.stringify(payload) },
    token
  )
}

export async function deleteSchedule(
  token: string | undefined,
  scheduleID: string,
  tenantID?: string
): Promise<void> {
  await request<void>(
    withTenantQuery(`/api/v1/schedules/${encodePathSegment(scheduleID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}
