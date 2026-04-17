import { clearSession, getRefreshToken, saveSession } from "@/lib/auth"

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080"

export type Tenant = {
  id: string
  name: string
  type: "studio" | "company" | "government" | "factory" | "public_facility" | string
  hq_region?: string
  status: "active" | "suspended" | "inactive" | string
  created_at: string
}

export type Building = {
  id: string
  tenant_id: string
  name: string
  address: string
  region?: string
  created_at: string
}

export type Floor = {
  id: string
  tenant_id: string
  building_id: string
  name: string
  created_at: string
}

export type Area = {
  id: string
  tenant_id: string
  building_id: string
  floor_id: string
  name: string
  created_at: string
}

export type Door = {
  id: string
  tenant_id: string
  building_id: string
  floor_id: string
  area_id: string
  name: string
  gateway_id: string
  kind: "office" | "turnstile" | "server-room" | "elevator" | "parking-gate" | "emergency-exit" | string
  status: "online" | "offline" | string
  created_at: string
}

export type TenantTopology = {
  tenant_id: string
  buildings: Building[]
  floors: Floor[]
  areas: Area[]
  doors: Door[]
}

export type GatewayDevice = {
  id: string
  gateway_id: string
  serial_number: string
  kind: string
  source: "mistypass_procured" | "legacy_integration" | string
  protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble" | string
  rs485_config?: {
    baud_rate: number
    parity: "none" | "even" | "odd" | string
    stop_bits: number
    device_address: number
    timeout_ms: number
  }
  rs485_health?: {
    retry_count: number
    timeout_count: number
    collision_count: number
    consecutive_timeouts: number
    last_error?: string
    last_report_at: string
  }
  status: "online" | "offline" | string
  last_seen_at: string
}

export type Gateway = {
  id: string
  tenant_id: string
  serial_number: string
  building_id: string
  device_capacity: number
  devices?: GatewayDevice[]
  status: string
  last_seen_at: string
  bound_door_ids?: string[]
}

export type GatewaySerialInventoryProductType =
  | "gateway"
  | "reader"
  | "controller"
  | "relay"
  | "sensor"
  | string

export type GatewaySerialInventoryStatus = "available" | "consumed" | "frozen" | "scrapped" | string

export type GatewaySerialInventoryItem = {
  id: string
  tenant_id: string
  serial_number: string
  product_type: GatewaySerialInventoryProductType
  status: GatewaySerialInventoryStatus
  batch_code?: string
  source?: string
  consumed_gateway_id?: string
  consumed_at?: string
  created_at: string
  updated_at: string
}

export type GatewayCommandAck = {
  task_id: string
  gateway_id: string
  command: string
  status: string
  created_at: string
}

export type GatewayCheckpointTrend = {
  report_total: number
  acked_delta: number
  direction: "up" | "down" | "flat" | string
  first_report_at?: string
  last_report_at?: string
}

export type GatewayCheckpointSummaryItem = {
  gateway_id: string
  tenant_id: string
  building_id: string
  queue: string
  checkpoint_id: string
  last_request_id: string
  acked_count: number
  event_total: number
  access_event_total: number
  device_event_total: number
  lag_count: number
  last_occurred_at?: string
  updated_at: string
  time_window_trend: GatewayCheckpointTrend
}

export type GatewayCheckpointSummaryResponse = {
  items: GatewayCheckpointSummaryItem[]
  totals: {
    queues: number
    event_total: number
    acked_total: number
    lag_total: number
  }
  time_window_trend: {
    window_minutes: number
    since: string
    until: string
    report_total: number
    gateway_total: number
    queue_total: number
    acked_delta_total: number
    direction: "up" | "down" | "flat" | string
    last_report_at?: string
  }
}

export type AccessPolicy = {
  id: string
  tenant_id: string
  name: string
  scope_type: "all" | "building" | "area" | "door" | string
  building_id?: string
  area_id?: string
  door_id?: string
  schedule: string
  members: number
  status: "active" | "inactive" | "draft" | string
  updated_at: string
}

export type UserGroup = {
  id: string
  tenant_id: string
  name: string
  description: string
  members?: string[]
  created_at: string
  updated_at: string
}

export type TemporaryAccess = {
  id: string
  tenant_id: string
  scope_type: "all" | "building" | "area" | "door" | string
  building_id?: string
  area_id?: string
  door_id?: string
  delivery_method: "wallet" | "email_qr" | string
  grantee_name: string
  grantee_gender?: string
  grantee_phone: string
  grantee_email: string
  mobile_model?: string
  pass_type?: string
  valid_until: string
  authorized_by_id?: string
  authorized_by_email?: string
  authorized_by_role?: string
  authorized_at?: string
  created_at: string
}

export type VisitorPass = {
  id: string
  tenant_id: string
  host: string
  visitor: string
  delivery_method: "wallet" | "email_qr" | string
  expires_at: string
  created_at: string
}

export type AccessEvent = {
  id: string
  tenant_id: string
  building_id: string
  area_id: string
  type: string
  actor: string
  door_id: string
  gateway_id: string
  result: "success" | "denied" | "warning" | string
  at: string
}

export type DeviceEvent = {
  id: string
  tenant_id: string
  building_id: string
  type: string
  gateway_id: string
  detail: string
  result: "success" | "denied" | "warning" | string
  at: string
}

export type Alarm = {
  id: string
  tenant_id: string
  building_id: string
  area_id: string
  door_id: string
  type: string
  severity: "critical" | "high" | "medium" | "low" | string
  location: string
  status:
    | "open"
    | "acknowledged"
    | "investigating"
    | "mitigated"
    | "escalated"
    | "resolved"
    | "false_positive"
    | string
  created_at: string
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

export type CurrentUser = {
  id: string
  email: string
  role: string
  tenant_id: string
  building_ids?: string[]
}

export type EnterpriseIDPConfig = {
  id: string
  tenant_id: string
  provider: string
  issuer_url: string
  client_id: string
  auth_url?: string
  token_url?: string
  jwks_url?: string
  user_info_url?: string
  saml_acs_url?: string
  saml_x509_cert?: string
  scopes?: string[]
  status: string
  sync_mode: string
  updated_by: string
  created_at: string
  updated_at: string
}

export type EnterpriseEmployee = {
  id: string
  tenant_id: string
  external_id: string
  email: string
  full_name: string
  department: string
  job_title: string
  location: string
  access_role: string
  building_id: string
  group_ids?: string[]
  status: string
  source: string
  last_synced_at: string
}

export type EmployeeSyncInput = {
  external_id: string
  email: string
  full_name: string
  department: string
  job_title: string
  location: string
  phone?: string
  manager_external_id?: string
  employment_status?: string
  status: string
}

export type EnterpriseJITProvisionApproval = {
  id: string
  tenant_id: string
  email: string
  external_id?: string
  provider?: string
  employment_status?: string
  status: string
  reason?: string
  external_sync_status?: string
  external_sync_ref?: string
  external_sync_attempt_count?: number
  external_sync_last_error?: string
  external_sync_updated_at?: string
  reviewed_by?: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export type EnterpriseSyncJob = {
  id: string
  tenant_id: string
  source: string
  status: string
  total: number
  created: number
  updated: number
  deactivated: number
  rejected: number
  actor: string
  started_at: string
  ended_at: string
}

export type EnterpriseSyncWorkerAlertSummaryItem = {
  tenant_id: string
  count: number
  first_seen_at: string
  last_seen_at: string
  last_failed: number
  last_threshold: number
  last_processed: number
  last_applied: number
  last_skipped_by_attempt_limit: number
  last_skipped_by_cooldown: number
}

export type WalletPassTemplate = {
  id: string
  tenant_id: string
  provider: string
  pass_type: "employee" | "visitor" | string
  class_id: string
  name: string
  style_config?: Record<string, string>
  status: "active" | "inactive" | string
  created_at: string
  updated_at: string
}

export type WalletPassInstance = {
  id: string
  tenant_id: string
  provider: string
  template_id: string
  target_type: "user" | "visitor" | string
  target_id: string
  object_id: string
  status: "issued" | "active" | "suspended" | "revoked" | string
  save_link: string
  expires_at?: string
  issued_at: string
  activated_at?: string
  revoked_at?: string
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

export type WalletPhysicalCardTask = {
  id: string
  tenant_id: string
  pass_id: string
  template_id: string
  target_type: "user" | "visitor" | string
  target_id: string
  task_type: "issue" | "reissue" | "loss_report" | string
  status: "queued" | "printing" | "ready" | "issued" | "reported_lost" | "cancelled" | string
  card_number?: string
  note?: string
  pass_status: "issued" | "active" | "suspended" | "revoked" | string
  completed_at?: string
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

export type WalletPassDeliveryNotification = {
  id: string
  tenant_id: string
  pass_id: string
  template_id: string
  target_type: "user" | "visitor" | string
  target_id: string
  channels?: string[]
  status: "sent" | "skipped" | "failed" | string
  reason?: string
  attempt?: number
  retryable: boolean
  provider?: string
  provider_error?: string
  channel_results?: Array<{
    channel: string
    status: "sent" | "skipped" | "failed" | string
    reason?: string
    provider?: string
    provider_error?: string
    retryable: boolean
    receivers?: string[]
  }>
  source_notification_id?: string
  triggered_at: string
}

export type WalletIssueJob = {
  id: string
  tenant_id: string
  provider: string
  batch_id: string
  template_id: string
  target_type: "user" | "visitor" | string
  target_id: string
  expires_at?: string
  pass_id?: string
  status: string
  retry_count: number
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
}

export type WalletJobSummary = {
  tenant_id: string
  max_retry: number
  total: number
  pending: number
  processing: number
  success: number
  failed: number
  dlq: number
  retryable_failed: number
  non_retryable_failed: number
  error_code_breakdown?: Record<string, number>
  updated_at: string
}

export type WalletJobMetricsWindow = {
  window_seconds: number
  since: string
  until: string
  created: number
  updated: number
  pending: number
  processing: number
  success: number
  failed: number
  dlq: number
  error_code_breakdown?: Record<string, number>
}

export type WalletJobMetricsAlert = {
  type: string
  error_code?: string
  count: number
  threshold: number
}

export type WalletJobMetrics = {
  tenant_id: string
  max_retry: number
  dlq_alert_threshold: number
  summary: WalletJobSummary
  window: WalletJobMetricsWindow
  alerts?: WalletJobMetricsAlert[]
  updated_at: string
}

export type WalletJobMetricsTrendBucket = {
  index: number
  start: string
  end: string
  created: number
  updated: number
  pending: number
  processing: number
  success: number
  failed: number
  dlq: number
  error_code_breakdown?: Record<string, number>
}

export type WalletJobMetricsTrend = {
  tenant_id: string
  max_retry: number
  dlq_alert_threshold: number
  window_seconds: number
  bucket_seconds: number
  bucket_count: number
  since: string
  until: string
  summary: WalletJobSummary
  alerts?: WalletJobMetricsAlert[]
  buckets: WalletJobMetricsTrendBucket[]
  updated_at: string
}

export type WalletDLQCleanupArchive = {
  id: string
  tenant_id: string
  limit: number
  error_code?: string
  older_than_seconds: number
  actor: string
  removed: number
  remaining_dlq: number
  processed_jobs?: string[]
  at: string
}

export type WalletJobAlertSubscription = {
  tenant_id: string
  enabled: boolean
  dlq_alert_threshold: number
  window_seconds: number
  cooldown_seconds: number
  channels: {
    email: boolean
    whatsapp: boolean
  }
  receiver_groups?: string[]
  updated_at: string
}

export type WalletJobAlertNotification = {
  id: string
  tenant_id: string
  type: string
  error_code?: string
  count: number
  threshold: number
  channels?: string[]
  receiver_groups?: string[]
  status: "sent" | "skipped" | "failed" | string
  reason?: string
  idempotency_key?: string
  attempt?: number
  retryable: boolean
  provider?: string
  provider_error?: string
  channel_results?: Array<{
    channel: string
    status: "sent" | "skipped" | "failed" | string
    reason?: string
    provider?: string
    provider_error?: string
    retryable: boolean
    receivers?: string[]
  }>
  source_notification_id?: string
  triggered_at: string
}

export type WalletJobAlertDispatchResult = {
  tenant_id: string
  window_seconds: number
  max_retry: number
  dlq_alert_threshold: number
  total_alerts: number
  dispatched: number
  skipped: number
  failed: number
  items?: WalletJobAlertNotification[]
  updated_at: string
}

export type LoginResponse = {
  access_token: string
  refresh_token: string
  expires_in: number
  user: CurrentUser
}

type RequestOptions = {
  skipAuthRecovery?: boolean
}

class APIError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "APIError"
    this.status = status
  }
}

let refreshInFlight: Promise<string | null> | null = null

async function parseErrorMessage(response: Response): Promise<string> {
  const fallback = `${response.status} ${response.statusText}`
  try {
    const payload = (await response.json()) as { error?: string }
    if (payload.error) {
      return payload.error
    }
  } catch {
    return fallback
  }
  return fallback
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) {
    return refreshInFlight
  }

  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    clearSession()
    return null
  }

  refreshInFlight = (async () => {
    try {
      const response = await request<LoginResponse>(
        "/api/v1/auth/refresh",
        {
          method: "POST",
          body: JSON.stringify({ refresh_token: refreshToken }),
        },
        undefined,
        { skipAuthRecovery: true }
      )
      if (!response.access_token || !response.refresh_token) {
        clearSession()
        return null
      }
      saveSession(response.access_token, response.refresh_token, "refresh")
      return response.access_token
    } catch {
      clearSession()
      return null
    } finally {
      refreshInFlight = null
    }
  })()

  return refreshInFlight
}

async function request<T>(path: string, init: RequestInit, token?: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(init.headers ?? {})
  headers.set("Content-Type", "application/json")
  if (token) {
    headers.set("Authorization", `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers,
  })

  if (response.status === 401 && token && !options.skipAuthRecovery) {
    const refreshedToken = await refreshAccessToken()
    if (refreshedToken) {
      return request<T>(path, init, refreshedToken, { skipAuthRecovery: true })
    }
    throw new APIError(401, "会话已过期，请重新登录")
  }

  if (!response.ok) {
    const message = await parseErrorMessage(response)
    if (response.status === 401) {
      clearSession()
    }
    throw new APIError(response.status, message)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

async function requestItems<T>(path: string, token: string): Promise<T[]> {
  const payload = await request<{ items: T[] }>(path, { method: "GET" }, token)
  return payload.items
}

async function requestText(path: string, token: string): Promise<string> {
  const headers = new Headers()
  headers.set("Authorization", `Bearer ${token}`)
  headers.set("Accept", "text/csv, text/plain;q=0.9, */*;q=0.8")

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "GET",
    headers,
  })

  if (response.status === 401) {
    const refreshedToken = await refreshAccessToken()
    if (refreshedToken) {
      return requestText(path, refreshedToken)
    }
    throw new APIError(401, "会话已过期，请重新登录")
  }

  if (!response.ok) {
    const message = await parseErrorMessage(response)
    throw new APIError(response.status, message)
  }

  return response.text()
}

function withTenantQuery(path: string, tenantID?: string): string {
  const value = tenantID?.trim()
  if (!value) {
    return path
  }

  const separator = path.includes("?") ? "&" : "?"
  return `${path}${separator}tenant_id=${encodeURIComponent(value)}`
}

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

export async function getCurrentUser(token: string): Promise<CurrentUser> {
  return request<CurrentUser>("/api/v1/me", { method: "GET" }, token)
}

export async function listTenants(token: string): Promise<Tenant[]> {
  return requestItems<Tenant>("/api/v1/tenants", token)
}

export async function createTenant(
  token: string,
  payload: {
    name: string
    type?: "studio" | "company" | "government" | "factory" | "public_facility"
    hq_region?: string
  }
): Promise<Tenant> {
  return request<Tenant>(
    "/api/v1/tenants",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateTenantStatus(
  token: string,
  tenantID: string,
  status: "active" | "suspended" | "inactive"
): Promise<Tenant> {
  return request<Tenant>(
    `/api/v1/tenants/${tenantID}/status`,
    {
      method: "PATCH",
      body: JSON.stringify({ status }),
    },
    token
  )
}

export async function getTenantTopology(token: string, tenantID: string): Promise<TenantTopology> {
  return request<TenantTopology>(`/api/v1/tenants/${tenantID}/topology`, { method: "GET" }, token)
}

export async function listBuildings(token: string, tenantID?: string): Promise<Building[]> {
  return requestItems<Building>(withTenantQuery("/api/v1/buildings", tenantID), token)
}

export async function createBuilding(
  token: string,
  payload: {
    tenant_id: string
    name: string
    address?: string
    region?: string
  }
): Promise<Building> {
  return request<Building>(
    "/api/v1/buildings",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listFloors(token: string, tenantID?: string): Promise<Floor[]> {
  return requestItems<Floor>(withTenantQuery("/api/v1/floors", tenantID), token)
}

export async function createFloor(
  token: string,
  payload: {
    tenant_id: string
    building_id: string
    name: string
  }
): Promise<Floor> {
  return request<Floor>(
    "/api/v1/floors",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listAreas(token: string, tenantID?: string): Promise<Area[]> {
  return requestItems<Area>(withTenantQuery("/api/v1/areas", tenantID), token)
}

export async function createArea(
  token: string,
  payload: {
    tenant_id: string
    building_id: string
    floor_id: string
    name: string
  }
): Promise<Area> {
  return request<Area>(
    "/api/v1/areas",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listDoors(token: string, tenantID?: string): Promise<Door[]> {
  return requestItems<Door>(withTenantQuery("/api/v1/doors", tenantID), token)
}

export async function createDoor(
  token: string,
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
    "/api/v1/doors",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listGateways(token: string): Promise<Gateway[]> {
  return requestItems<Gateway>("/api/v1/gateways", token)
}

function buildGatewaySerialInventoryPath(
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): string {
  const query = new URLSearchParams()
  if (tenantID?.trim()) {
    query.set("tenant_id", tenantID.trim())
  }
  if (options?.product_type?.trim()) {
    query.set("product_type", options.product_type.trim())
  }
  if (options?.status?.trim()) {
    query.set("status", options.status.trim())
  }
  const suffix = query.toString()
  return suffix ? `/api/v1/gateways/serial-inventory?${suffix}` : "/api/v1/gateways/serial-inventory"
}

export async function listGatewaySerialInventory(
  token: string,
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): Promise<GatewaySerialInventoryItem[]> {
  return requestItems<GatewaySerialInventoryItem>(buildGatewaySerialInventoryPath(tenantID, options), token)
}

export async function importGatewaySerialInventory(
  token: string,
  payload: {
    tenant_id: string
    items: Array<{
      serial_number: string
      product_type: GatewaySerialInventoryProductType
      batch_code?: string
      source?: string
    }>
  }
): Promise<GatewaySerialInventoryItem[]> {
  const response = await request<{ items: GatewaySerialInventoryItem[] }>(
    "/api/v1/gateways/serial-inventory/import",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.items
}

export async function importGatewaySerialInventoryCSV(
  token: string,
  payload: {
    tenant_id: string
    csv_content: string
  }
): Promise<GatewaySerialInventoryItem[]> {
  const response = await request<{ items: GatewaySerialInventoryItem[] }>(
    "/api/v1/gateways/serial-inventory/import-csv",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.items
}

export async function batchUpdateGatewaySerialInventoryStatus(
  token: string,
  payload: {
    tenant_id: string
    status: GatewaySerialInventoryStatus
    serial_numbers: string[]
    consumed_gateway_id?: string
  }
): Promise<GatewaySerialInventoryItem[]> {
  const response = await request<{ items: GatewaySerialInventoryItem[] }>(
    "/api/v1/gateways/serial-inventory/batch-status",
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.items
}

export async function updateGatewaySerialInventoryStatus(
  token: string,
  serialNumber: string,
  payload: {
    tenant_id: string
    status: GatewaySerialInventoryStatus
    consumed_gateway_id?: string
  }
): Promise<GatewaySerialInventoryItem> {
  return request<GatewaySerialInventoryItem>(
    `/api/v1/gateways/serial-inventory/${encodeURIComponent(serialNumber)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function exportGatewaySerialInventoryCSV(
  token: string,
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): Promise<string> {
  const base = buildGatewaySerialInventoryPath(tenantID, options)
  const path = base.replace("/api/v1/gateways/serial-inventory", "/api/v1/gateways/serial-inventory/export-csv")
  return requestText(path, token)
}

export async function registerGateway(
  token: string,
  payload: {
    serial_number: string
    tenant_id: string
    building_id?: string
    device_capacity?: number
  }
): Promise<Gateway> {
  return request<Gateway>(
    "/api/v1/gateways/register",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function bindGatewayDoor(
  token: string,
  gatewayID: string,
  doorID: string
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${gatewayID}/bind-door`,
    {
      method: "POST",
      body: JSON.stringify({ door_id: doorID }),
    },
    token
  )
}

export async function unbindGatewayDoor(
  token: string,
  gatewayID: string,
  doorID: string
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${gatewayID}/unbind-door`,
    {
      method: "POST",
      body: JSON.stringify({ door_id: doorID }),
    },
    token
  )
}

export async function registerGatewayDevice(
  token: string,
  gatewayID: string,
  payload: {
    serial_number: string
    kind?: "reader" | "door_controller" | "relay" | "sensor" | "legacy_reader" | "legacy_controller" | string
    source?: "mistypass_procured" | "legacy_integration" | string
    protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble" | string
    rs485_config?: {
      baud_rate?: number
      parity?: "none" | "even" | "odd" | string
      stop_bits?: number
      device_address?: number
      timeout_ms?: number
    }
    status?: "online" | "offline" | string
  }
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${gatewayID}/devices`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function reportGatewayDeviceRS485Telemetry(
  token: string,
  gatewayID: string,
  deviceID: string,
  payload: {
    retries?: number
    timeouts?: number
    collisions?: number
    last_error?: string
    reset_consecutive_timeouts?: boolean
  }
): Promise<{
  gateway_id: string
  device_id: string
  device: GatewayDevice
  alerted: boolean
}> {
  return request<{
    gateway_id: string
    device_id: string
    device: GatewayDevice
    alerted: boolean
  }>(
    `/api/v1/gateways/${gatewayID}/devices/${deviceID}/rs485/telemetry`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function probeGatewayLegacyDevices(
  token: string,
  gatewayID: string
): Promise<string[]> {
  const payload = await request<{ items: string[] }>(
    `/api/v1/gateways/${gatewayID}/devices/probe-legacy`,
    { method: "POST" },
    token
  )
  return payload.items
}

export async function publishGatewayConfig(
  token: string,
  gatewayID: string,
  version: string
): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    `/api/v1/gateways/${gatewayID}/config/publish`,
    {
      method: "POST",
      body: JSON.stringify({ version }),
    },
    token
  )
}

export async function rebootGateway(token: string, gatewayID: string): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    `/api/v1/gateways/${gatewayID}/reboot`,
    {
      method: "POST",
    },
    token
  )
}

export async function listGatewayEventCheckpointSummary(
  token: string,
  options?: {
    tenant_id?: string
    gateway_id?: string
    queue?: string
    limit?: number
    trend_window_minutes?: number
  }
): Promise<GatewayCheckpointSummaryResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.gateway_id?.trim()) {
    query.set("gateway_id", options.gateway_id.trim())
  }
  if (options?.queue?.trim()) {
    query.set("queue", options.queue.trim())
  }
  if (typeof options?.limit === "number" && Number.isFinite(options.limit) && options.limit >= 0) {
    query.set("limit", String(Math.floor(options.limit)))
  }
  if (
    typeof options?.trend_window_minutes === "number" &&
    Number.isFinite(options.trend_window_minutes) &&
    options.trend_window_minutes > 0
  ) {
    query.set("trend_window_minutes", String(Math.floor(options.trend_window_minutes)))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/gateways/events/checkpoint/summary?${suffix}`
    : "/api/v1/gateways/events/checkpoint/summary"
  return request<GatewayCheckpointSummaryResponse>(path, { method: "GET" }, token)
}

export async function listAccessPolicies(token: string): Promise<AccessPolicy[]> {
  return requestItems<AccessPolicy>("/api/v1/access-policies", token)
}

export async function createAccessPolicy(
  token: string,
  payload: {
    tenant_id: string
    name: string
    scope_type: "all" | "building" | "area" | "door"
    building_id?: string
    area_id?: string
    door_id?: string
    schedule?: string
    members?: number
    status?: "active" | "inactive" | "draft"
  }
): Promise<AccessPolicy> {
  return request<AccessPolicy>(
    "/api/v1/access-policies",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateAccessPolicy(
  token: string,
  policyID: string,
  payload: {
    name: string
    scope_type: "all" | "building" | "area" | "door"
    building_id?: string
    area_id?: string
    door_id?: string
    schedule?: string
    members?: number
    status?: "active" | "inactive" | "draft"
  }
): Promise<AccessPolicy> {
  return request<AccessPolicy>(
    `/api/v1/access-policies/${policyID}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listUserGroups(token: string): Promise<UserGroup[]> {
  return requestItems<UserGroup>("/api/v1/user-groups", token)
}

export async function createUserGroup(
  token: string,
  payload: {
    tenant_id: string
    name: string
    description?: string
    members?: string[]
  }
): Promise<UserGroup> {
  return request<UserGroup>(
    "/api/v1/user-groups",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateUserGroup(
  token: string,
  groupID: string,
  payload: {
    name: string
    description?: string
    members?: string[]
  }
): Promise<UserGroup> {
  return request<UserGroup>(
    `/api/v1/user-groups/${groupID}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listTemporaryAccess(token: string): Promise<TemporaryAccess[]> {
  return requestItems<TemporaryAccess>("/api/v1/temporary-access", token)
}

export async function createTemporaryAccess(
  token: string,
  payload: {
    tenant_id: string
    scope_type: "all" | "building" | "area" | "door"
    building_id?: string
    area_id?: string
    door_id?: string
    delivery_method: "wallet" | "email_qr"
    grantee_name: string
    grantee_gender?: string
    grantee_phone: string
    grantee_email: string
    mobile_model?: string
    pass_type?: string
    valid_until: string
  }
): Promise<TemporaryAccess> {
  return request<TemporaryAccess>(
    "/api/v1/temporary-access",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listVisitorPasses(token: string): Promise<VisitorPass[]> {
  return requestItems<VisitorPass>("/api/v1/visitor-passes", token)
}

export async function createVisitorPass(
  token: string,
  payload: {
    tenant_id: string
    host: string
    visitor: string
    delivery_method: "wallet" | "email_qr"
    expires_at: string
  }
): Promise<VisitorPass> {
  return request<VisitorPass>(
    "/api/v1/visitor-passes",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listAccessEvents(token: string): Promise<AccessEvent[]> {
  return requestItems<AccessEvent>("/api/v1/events/access", token)
}

export async function listDeviceEvents(token: string): Promise<DeviceEvent[]> {
  return requestItems<DeviceEvent>("/api/v1/events/device", token)
}

export async function listAlarms(token: string): Promise<Alarm[]> {
  return requestItems<Alarm>("/api/v1/alarms", token)
}

export async function updateAlarmStatus(
  token: string,
  alarmID: string,
  status: "open" | "acknowledged" | "investigating" | "mitigated" | "escalated" | "resolved" | "false_positive"
): Promise<Alarm> {
  return request<Alarm>(
    `/api/v1/alarms/${alarmID}/status`,
    {
      method: "PATCH",
      body: JSON.stringify({ status }),
    },
    token
  )
}

export async function listAuditLogs(token: string): Promise<AuditLog[]> {
  return requestItems<AuditLog>("/api/v1/audit-logs", token)
}

export async function getEnterpriseIDPConfig(token: string, tenantID?: string): Promise<EnterpriseIDPConfig> {
  return request<EnterpriseIDPConfig>(withTenantQuery("/api/v1/enterprise/idp-config", tenantID), { method: "GET" }, token)
}

export async function listEnterpriseEmployees(token: string, tenantID?: string): Promise<EnterpriseEmployee[]> {
  return requestItems<EnterpriseEmployee>(withTenantQuery("/api/v1/enterprise/employees", tenantID), token)
}

export async function listEnterpriseJITProvisionApprovals(
  token: string,
  options?: {
    tenant_id?: string
    status?: string
    limit?: number
  }
): Promise<EnterpriseJITProvisionApproval[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.status?.trim()) {
    query.set("status", options.status.trim())
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/enterprise/jit-provision-approvals?${suffix}`
    : "/api/v1/enterprise/jit-provision-approvals"
  return requestItems<EnterpriseJITProvisionApproval>(path, token)
}

export async function reviewEnterpriseJITProvisionApproval(
  token: string,
  approvalID: string,
  payload: {
    tenant_id: string
    decision: "approved" | "rejected"
    reviewed_by?: string
    reason?: string
  }
): Promise<EnterpriseJITProvisionApproval> {
  const response = await request<{ item: EnterpriseJITProvisionApproval }>(
    `/api/v1/enterprise/jit-provision-approvals/${approvalID}/review`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.item
}

export async function updateEnterpriseJITProvisionApprovalExternalSync(
  token: string,
  approvalID: string,
  payload: {
    tenant_id: string
    status: "synced" | "failed"
    external_sync_ref?: string
    last_error?: string
  }
): Promise<EnterpriseJITProvisionApproval> {
  const response = await request<{ item: EnterpriseJITProvisionApproval }>(
    `/api/v1/enterprise/jit-provision-approvals/${approvalID}/external-sync`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.item
}

export async function syncEnterpriseEmployees(
  token: string,
  payload: {
    tenant_id: string
    source: string
    actor?: string
    request_id?: string
    employees: EmployeeSyncInput[]
  }
): Promise<{
  job: EnterpriseSyncJob
  items: EnterpriseEmployee[]
  access_sync: {
    created: number
    updated: number
    rejected: number
  }
}> {
  return request(
    "/api/v1/enterprise/employees/sync",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listEnterpriseSyncJobs(token: string, tenantID?: string): Promise<EnterpriseSyncJob[]> {
  return requestItems<EnterpriseSyncJob>(withTenantQuery("/api/v1/enterprise/sync-jobs", tenantID), token)
}

export async function listEnterpriseSyncWorkerAlertSummary(
  token: string,
  options?: {
    tenant_id?: string
    limit?: number
  }
): Promise<EnterpriseSyncWorkerAlertSummaryItem[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/enterprise/sync-worker-alerts/summary?${suffix}`
    : "/api/v1/enterprise/sync-worker-alerts/summary"
  return requestItems<EnterpriseSyncWorkerAlertSummaryItem>(path, token)
}

export async function listWalletTemplates(token: string, tenantID?: string): Promise<WalletPassTemplate[]> {
  return requestItems<WalletPassTemplate>(withTenantQuery("/api/v1/wallet/templates", tenantID), token)
}

export async function createWalletTemplate(
  token: string,
  payload: {
    tenant_id: string
    pass_type: "employee" | "visitor"
    class_id?: string
    name: string
    style_config?: Record<string, string>
    status?: "active" | "inactive"
    actor?: string
  }
): Promise<WalletPassTemplate> {
  return request<WalletPassTemplate>(
    "/api/v1/wallet/templates",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateWalletTemplateStatus(
  token: string,
  templateID: string,
  payload: {
    tenant_id: string
    status: "active" | "inactive"
    actor?: string
  }
): Promise<WalletPassTemplate> {
  return request<WalletPassTemplate>(
    withTenantQuery(`/api/v1/wallet/templates/${templateID}/status`, payload.tenant_id),
    {
      method: "PATCH",
      body: JSON.stringify({
        status: payload.status,
        actor: payload.actor,
      }),
    },
    token
  )
}

export async function issueWalletPass(
  token: string,
  payload: {
    tenant_id: string
    template_id: string
    target_type: "user" | "visitor"
    target_id: string
    expires_at?: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return request<WalletPassInstance>(
    "/api/v1/wallet/passes/issue",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function issueWalletPassBatch(
  token: string,
  payload: {
    tenant_id: string
    template_id: string
    target_type: "user" | "visitor"
    target_ids: string[]
    expires_at?: string
    actor?: string
    execution_mode?: "inline" | "queued"
  }
): Promise<{
  items: WalletIssueJob[]
  execution_mode: "inline" | "queued" | string
}> {
  return request(
    "/api/v1/wallet/passes/issue-batch",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listWalletPasses(token: string, tenantID?: string): Promise<WalletPassInstance[]> {
  return requestItems<WalletPassInstance>(withTenantQuery("/api/v1/wallet/passes", tenantID), token)
}

export async function getWalletPass(token: string, passID: string, tenantID?: string): Promise<WalletPassInstance> {
  return request<WalletPassInstance>(withTenantQuery(`/api/v1/wallet/passes/${passID}`, tenantID), { method: "GET" }, token)
}

export async function getWalletPassSaveLink(token: string, passID: string, tenantID?: string): Promise<string> {
  const payload = await request<{ save_link: string }>(
    withTenantQuery(`/api/v1/wallet/passes/${passID}/save-link`, tenantID),
    { method: "GET" },
    token
  )
  return payload.save_link
}

export async function listWalletPhysicalCardTasks(token: string, tenantID?: string): Promise<WalletPhysicalCardTask[]> {
  return requestItems<WalletPhysicalCardTask>(withTenantQuery("/api/v1/wallet/physical-card-tasks", tenantID), token)
}

export async function listWalletPassDeliveries(
  token: string,
  payload: {
    tenant_id: string
    pass_id?: string
  }
): Promise<WalletPassDeliveryNotification[]> {
  const params = new URLSearchParams()
  if (payload.tenant_id.trim()) {
    params.set("tenant_id", payload.tenant_id.trim())
  }
  if (payload.pass_id?.trim()) {
    params.set("pass_id", payload.pass_id.trim())
  }
  const suffix = params.toString()
  return requestItems<WalletPassDeliveryNotification>(
    suffix ? `/api/v1/wallet/deliveries?${suffix}` : "/api/v1/wallet/deliveries",
    token
  )
}

export async function dispatchWalletPassDelivery(
  token: string,
  payload: {
    tenant_id: string
    pass_id: string
    channels: string[]
    email_recipients?: string[]
    whatsapp_recipients?: string[]
    actor?: string
  }
): Promise<WalletPassDeliveryNotification> {
  return request<WalletPassDeliveryNotification>(
    "/api/v1/wallet/deliveries/dispatch",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function retryWalletPassDelivery(
  token: string,
  payload: {
    tenant_id: string
    notification_id: string
    actor?: string
  }
): Promise<WalletPassDeliveryNotification> {
  return request<WalletPassDeliveryNotification>(
    `/api/v1/wallet/deliveries/${encodeURIComponent(payload.notification_id)}/retry`,
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: payload.tenant_id,
        actor: payload.actor,
      }),
    },
    token
  )
}

export async function createWalletPhysicalCardTask(
  token: string,
  payload: {
    tenant_id: string
    pass_id: string
    task_type: "issue" | "reissue" | "loss_report"
    card_number?: string
    note?: string
    actor?: string
  }
): Promise<WalletPhysicalCardTask> {
  return request<WalletPhysicalCardTask>(
    "/api/v1/wallet/physical-card-tasks",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateWalletPhysicalCardTaskStatus(
  token: string,
  taskID: string,
  payload: {
    tenant_id: string
    status: string
    card_number?: string
    note?: string
    actor?: string
  }
): Promise<WalletPhysicalCardTask> {
  return request<WalletPhysicalCardTask>(
    withTenantQuery(`/api/v1/wallet/physical-card-tasks/${taskID}/status`, payload.tenant_id),
    {
      method: "PATCH",
      body: JSON.stringify({
        status: payload.status,
        card_number: payload.card_number,
        note: payload.note,
        actor: payload.actor,
      }),
    },
    token
  )
}

async function updateWalletPassStatus(
  token: string,
  passID: string,
  action: "suspend" | "activate" | "revoke",
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return request<WalletPassInstance>(
    withTenantQuery(`/api/v1/wallet/passes/${passID}/${action}`, payload.tenant_id),
    {
      method: "PATCH",
      body: JSON.stringify({
        actor: payload.actor,
      }),
    },
    token
  )
}

export async function suspendWalletPass(
  token: string,
  passID: string,
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return updateWalletPassStatus(token, passID, "suspend", payload)
}

export async function activateWalletPass(
  token: string,
  passID: string,
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return updateWalletPassStatus(token, passID, "activate", payload)
}

export async function revokeWalletPass(
  token: string,
  passID: string,
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return updateWalletPassStatus(token, passID, "revoke", payload)
}

function buildWalletJobMetricsPath(options: {
  tenant_id?: string
  window_seconds?: number
  max_retry?: number
  dlq_alert_threshold?: number
}): string {
  const query = new URLSearchParams()
  if (options.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options.window_seconds === "number" && options.window_seconds > 0) {
    query.set("window_seconds", String(options.window_seconds))
  }
  if (typeof options.max_retry === "number" && options.max_retry >= 0) {
    query.set("max_retry", String(options.max_retry))
  }
  if (typeof options.dlq_alert_threshold === "number" && options.dlq_alert_threshold > 0) {
    query.set("dlq_alert_threshold", String(options.dlq_alert_threshold))
  }
  const suffix = query.toString()
  return suffix ? `/api/v1/wallet/jobs/metrics?${suffix}` : "/api/v1/wallet/jobs/metrics"
}

function buildWalletDLQCleanupArchivesPath(options: {
  tenant_id?: string
  limit?: number
}): string {
  const query = new URLSearchParams()
  if (options.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  return suffix
    ? `/api/v1/wallet/jobs/dlq/cleanup/archives?${suffix}`
    : "/api/v1/wallet/jobs/dlq/cleanup/archives"
}

function buildWalletJobMetricsTrendPath(options: {
  tenant_id?: string
  window_seconds?: number
  bucket_count?: number
  max_retry?: number
  dlq_alert_threshold?: number
}): string {
  const query = new URLSearchParams()
  if (options.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options.window_seconds === "number" && options.window_seconds > 0) {
    query.set("window_seconds", String(options.window_seconds))
  }
  if (typeof options.bucket_count === "number" && options.bucket_count > 0) {
    query.set("bucket_count", String(options.bucket_count))
  }
  if (typeof options.max_retry === "number" && options.max_retry >= 0) {
    query.set("max_retry", String(options.max_retry))
  }
  if (typeof options.dlq_alert_threshold === "number" && options.dlq_alert_threshold > 0) {
    query.set("dlq_alert_threshold", String(options.dlq_alert_threshold))
  }
  const suffix = query.toString()
  return suffix
    ? `/api/v1/wallet/jobs/metrics/trend?${suffix}`
    : "/api/v1/wallet/jobs/metrics/trend"
}

function buildWalletJobAlertSubscriptionPath(options: {
  tenant_id?: string
}): string {
  const query = new URLSearchParams()
  if (options.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  const suffix = query.toString()
  return suffix
    ? `/api/v1/wallet/jobs/alert-subscription?${suffix}`
    : "/api/v1/wallet/jobs/alert-subscription"
}

function buildWalletJobAlertNotificationsPath(options: {
  tenant_id?: string
  limit?: number
}): string {
  const query = new URLSearchParams()
  if (options.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  return suffix
    ? `/api/v1/wallet/jobs/alert-notifications?${suffix}`
    : "/api/v1/wallet/jobs/alert-notifications"
}

export async function getWalletJobMetrics(
  token: string,
  options: {
    tenant_id?: string
    window_seconds?: number
    max_retry?: number
    dlq_alert_threshold?: number
  }
): Promise<WalletJobMetrics> {
  return request<WalletJobMetrics>(
    buildWalletJobMetricsPath(options),
    { method: "GET" },
    token
  )
}

export async function listWalletDLQCleanupArchives(
  token: string,
  options: {
    tenant_id?: string
    limit?: number
  }
): Promise<WalletDLQCleanupArchive[]> {
  const payload = await request<{ items: WalletDLQCleanupArchive[] }>(
    buildWalletDLQCleanupArchivesPath(options),
    { method: "GET" },
    token
  )
  return payload.items
}

export async function getWalletJobMetricsTrend(
  token: string,
  options: {
    tenant_id?: string
    window_seconds?: number
    bucket_count?: number
    max_retry?: number
    dlq_alert_threshold?: number
  }
): Promise<WalletJobMetricsTrend> {
  return request<WalletJobMetricsTrend>(
    buildWalletJobMetricsTrendPath(options),
    { method: "GET" },
    token
  )
}

export async function getWalletJobAlertSubscription(
  token: string,
  options: {
    tenant_id?: string
  }
): Promise<WalletJobAlertSubscription> {
  return request<WalletJobAlertSubscription>(
    buildWalletJobAlertSubscriptionPath(options),
    { method: "GET" },
    token
  )
}

export async function upsertWalletJobAlertSubscription(
  token: string,
  payload: {
    tenant_id: string
    enabled?: boolean
    dlq_alert_threshold?: number
    window_seconds?: number
    cooldown_seconds?: number
    channels?: {
      email?: boolean
      whatsapp?: boolean
    }
    receiver_groups?: string[]
    actor?: string
  }
): Promise<WalletJobAlertSubscription> {
  return request<WalletJobAlertSubscription>(
    "/api/v1/wallet/jobs/alert-subscription",
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listWalletJobAlertNotifications(
  token: string,
  options: {
    tenant_id?: string
    limit?: number
  }
): Promise<WalletJobAlertNotification[]> {
  const payload = await request<{ items: WalletJobAlertNotification[] }>(
    buildWalletJobAlertNotificationsPath(options),
    { method: "GET" },
    token
  )
  return payload.items
}

export async function dispatchWalletJobAlerts(
  token: string,
  payload: {
    tenant_id: string
    window_seconds?: number
    max_retry?: number
    dlq_alert_threshold?: number
    actor?: string
  }
): Promise<WalletJobAlertDispatchResult> {
  return request<WalletJobAlertDispatchResult>(
    "/api/v1/wallet/jobs/alerts/dispatch",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function retryWalletJobAlertNotification(
  token: string,
  payload: {
    tenant_id: string
    notification_id: string
    actor?: string
  }
): Promise<WalletJobAlertNotification> {
  const notificationID = payload.notification_id.trim()
  return request<WalletJobAlertNotification>(
    `/api/v1/wallet/jobs/alert-notifications/${encodeURIComponent(notificationID)}/retry`,
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: payload.tenant_id,
        actor: payload.actor,
      }),
    },
    token
  )
}
