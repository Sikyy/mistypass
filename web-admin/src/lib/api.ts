import { clearSession, getRefreshToken, getToken, saveSession } from "@/lib/auth"

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080"

export type Tenant = {
  id: string
  name: string
  type: "studio" | "company" | "government" | "factory" | "public_facility"
  hq_region?: string
  status: "active" | "suspended" | "inactive"
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

export type Place = Building

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
  kind: "office" | "turnstile" | "server-room" | "elevator" | "parking-gate" | "emergency-exit"
  status: "online" | "offline"
  created_at: string
}

export type Lock = Door

export type DoorGroup = {
  id: string
  tenant_id: string
  name: string
  door_ids?: string[]
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
  source: "mistypass_procured" | "legacy_integration"
  protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"
  rs485_config?: {
    baud_rate: number
    parity: "none" | "even" | "odd"
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
  status: "online" | "offline"
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

export type Integration = {
  id: string
  resource_type: "Integration"
  tenant_id: string
  type: "identity_provider" | "hris" | "webhook" | "mqtt" | "device_api" | string
  provider: string
  name: string
  description: string
  status: string
  configured: boolean
  sync_mode?: string
  source_id?: string
  last_sync_at?: string
  created_at: string
  updated_at: string
}

export type GatewaySerialInventoryProductType =
  | "gateway"
  | "reader"
  | "controller"
  | "relay"
  | "sensor"

export type GatewaySerialInventoryStatus = "available" | "consumed" | "frozen" | "scrapped"

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
  direction: "up" | "down" | "flat"
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
    direction: "up" | "down" | "flat"
    last_report_at?: string
  }
}

export type AccessPolicy = {
  id: string
  tenant_id: string
  name: string
  scope_type: "all" | "building" | "area" | "door"
  building_id?: string
  area_id?: string
  door_id?: string
  schedule: string
  members: number
  status: "active" | "inactive" | "draft"
  updated_at: string
}

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

export type UserGroup = {
  id: string
  tenant_id: string
  building_id?: string
  name: string
  description: string
  members?: string[]
  created_at: string
  updated_at: string
}

export type Team = {
  id: string
  resource_type: "Team"
  tenant_id: string
  name: string
  scope: "organization" | "place"
  place_id?: string
  description?: string
  source?: string
  created_at: string
  updated_at: string
}

export type TeamMembership = {
  id: string
  resource_type: "TeamMembership"
  tenant_id: string
  team_id: string
  member_type: "User" | "Guest"
  member_id: string
  member_email?: string
  member_name?: string
  source?: string
  created_at: string
  updated_at: string
}

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
  created_at: string
  updated_at: string
}

export type TemporaryAccess = {
  id: string
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
  authorized_by_id?: string
  authorized_by_email?: string
  authorized_by_role?: string
  authorized_at?: string
  created_at: string
}

export type Share = {
  id: string
  tenant_id: string
  email: string
  group_id?: string
  role_id: string
  place_id?: string
  lock_id?: string
  valid_from?: string
  valid_until: string
  status: string
  delivery_method: "wallet" | "email_qr"
  grantee_name?: string
  authorized_by_id?: string
  authorized_by_email?: string
  authorized_by_role?: string
  created_at: string
}

export type VisitorPass = {
  id: string
  tenant_id: string
  host: string
  visitor: string
  delivery_method: "wallet" | "email_qr"
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
  type: string
  actor_type?: string
  actor_id?: string
  actor_name?: string
  actor_email?: string
  object_type?: string
  object_id?: string
  object_name?: string
  place_id?: string
  lock_id?: string
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

export type AuditLog = {
  id: string
  actor: string
  role: string
  action: string
  target: string
  source: string
  at: string
}

export type UserRole = "super_admin" | "tenant_admin" | "operator" | "building_admin" | "resident"

export type CurrentUser = {
  id: string
  email: string
  role: UserRole
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

export type EnterpriseHRISConnector = {
  id: string
  tenant_id: string
  vendor: "talenta" | "gadjian" | "greatday" | "linovhr" | "sunfish"
  status: "active" | "inactive"
  sync_strategy: "hybrid" | "webhook" | "pull"
  credential_ref?: string
  webhook_secret_ref?: string
  last_sync_at?: string
  updated_by?: string
  created_at: string
  updated_at: string
}

export type EnterpriseHRISSecret = {
  ref: string
  tenant_id: string
  name: string
  kind: string
  updated_by?: string
  created_at: string
  updated_at: string
}

export type EnterpriseEmployee = {
  id: string
  tenant_id: string
  external_id: string
  employee_number?: string
  email: string
  full_name: string
  department: string
  job_title: string
  location: string
  phone?: string
  manager_external_id?: string
  employment_status?: string
  join_date?: string
  resign_date?: string
  shift_code?: string
  schedule_window?: string
  leave_status?: string
  cost_center?: string
  photo_url?: string
  access_role: string
  building_id: string
  group_ids?: string[]
  status: string
  source: string
  last_synced_at: string
}

export type EmployeeSyncInput = {
  external_id: string
  employee_number?: string
  email: string
  full_name: string
  department: string
  job_title: string
  location: string
  phone?: string
  manager_external_id?: string
  employment_status?: string
  join_date?: string
  resign_date?: string
  shift_code?: string
  schedule_window?: string
  leave_status?: string
  cost_center?: string
  photo_url?: string
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

export type EnterpriseSyncRequestRecord = {
  request_id: string
  tenant_id: string
  connector_id?: string
  raw_payload_ref?: string
  result: {
    job: EnterpriseSyncJob
    items: EnterpriseEmployee[]
  }
  access_applied: boolean
  access_created: number
  access_updated: number
  access_rejected: number
  access_attempt_count: number
  last_access_error?: string
  last_access_attempt_at?: string
  created_at: string
}

export type EnterprisePendingSyncReconcileResult = {
  request_id: string
  job_id: string
  access_applied: boolean
  access_created: number
  access_updated: number
  access_rejected: number
  access_attempt_count: number
  last_access_error?: string
  last_access_attempt_at?: string
}

export type EnterpriseBatchPendingSyncReconcileResult = {
  processed: number
  applied: number
  failed: number
  skipped_by_attempt_limit?: number
  skipped_by_cooldown?: number
  items: EnterprisePendingSyncReconcileResult[]
}

export type EnterpriseSyncWorkerAlertSubscription = {
  tenant_id: string
  enabled: boolean
  worker_alert_threshold: number
  window_seconds: number
  cooldown_seconds: number
  channels: {
    email: boolean
    whatsapp: boolean
  }
  receiver_groups?: string[]
  updated_at: string
}

export type EnterpriseSyncWorkerAlertSummaryItem = {
  tenant_id: string
  worker_action: string
  worker_kind: string
  worker_label: string
  count: number
  first_seen_at: string
  last_seen_at: string
  last_failed: number
  last_threshold: number
  last_processed: number
  last_applied: number
  last_consecutive_failures?: number
  last_failure_age_seconds?: number
  last_skipped_by_attempt_limit: number
  last_skipped_by_cooldown: number
}

export type EnterpriseSyncWorkerAlertItem = {
  id: string
  tenant_id: string
  actor: string
  role: string
  action: string
  worker_action: string
  worker_kind: string
  worker_label: string
  source: string
  at: string
  failed: number
  threshold: number
  processed: number
  applied: number
  consecutive_failures?: number
  failure_age_seconds?: number
  skipped_by_attempt_limit: number
  skipped_by_cooldown: number
  connector_id?: string
  vendor?: string
  event_type?: string
  request_id?: string
  failure_stage?: string
  mode?: string
  raw_target: string
}

export type EnterpriseSyncWorkerAlertNotification = {
  id: string
  tenant_id: string
  worker_action: string
  worker_kind: string
  worker_label: string
  fingerprint: string
  count: number
  threshold: number
  failed: number
  processed: number
  applied: number
  skipped_by_attempt_limit?: number
  skipped_by_cooldown?: number
  connector_id?: string
  vendor?: string
  event_type?: string
  request_id?: string
  failure_stage?: string
  mode?: string
  channels?: string[]
  receiver_groups?: string[]
  status: string
  reason?: string
  idempotency_key?: string
  attempt?: number
  confirm_attempts?: number
  last_confirm_attempt_at?: string
  last_confirm_result?: string
  retryable: boolean
  provider?: string
  provider_error?: string
  source_notification_id?: string
  restore_status?: string
  pending_age_seconds?: number
  next_retry_at?: string
  channel_results?: Array<{
    channel: string
    status: string
    reason?: string
    provider?: string
    provider_error?: string
    retryable: boolean
    receivers?: string[]
  }>
  triggered_at: string
}

export type EnterpriseSyncWorkerAlertNotificationFilterCounts = {
  all: number
  failed: number
  retryable: number
  suppressed: number
  due_now: number
}

export type EnterpriseSyncWorkerAlertNotificationStatusCounts = {
  sent: number
  failed: number
  skipped: number
}

export type EnterpriseSyncWorkerAlertNotificationListResponse = {
  items: EnterpriseSyncWorkerAlertNotification[]
  total: number
  offset: number
  limit: number
  next_offset?: number
  has_more: boolean
  filter_counts: EnterpriseSyncWorkerAlertNotificationFilterCounts
  status_counts: EnterpriseSyncWorkerAlertNotificationStatusCounts
}

export type EnterpriseOffsetListResponse<T> = {
  items: T[]
  total: number
  offset: number
  limit: number
  next_offset?: number
  has_more: boolean
}

export type EnterpriseHRISWebhookRuntimeCounts = {
  all: number
  ready: number
  cooldown: number
  in_flight: number
  attempt_limit: number
  terminal: number
}

export type EnterpriseSyncWorkerAlertDispatchResult = {
  tenant_id: string
  total_alerts: number
  dispatched: number
  skipped: number
  failed: number
  items?: EnterpriseSyncWorkerAlertNotification[]
  updated_at: string
}

export type EnterpriseSyncWorkerAlertNotificationBatchRetryResult = {
  tenant_id: string
  total_notifications: number
  retried: number
  skipped: number
  failed: number
  suppressed: number
  items?: EnterpriseSyncWorkerAlertNotification[]
  updated_at: string
}

export type EnterpriseSyncWorkerAlertNotificationBatchSuppressResult = {
  tenant_id: string
  total_notifications: number
  suppressed: number
  skipped: number
  items?: EnterpriseSyncWorkerAlertNotification[]
  updated_at: string
}

export type EnterpriseSyncWorkerAlertNotificationBatchRestoreResult = {
  tenant_id: string
  total_notifications: number
  restored: number
  skipped: number
  items?: EnterpriseSyncWorkerAlertNotification[]
  updated_at: string
}

export type EnterpriseSyncWorkerAlertNotificationAutoRetryResult = {
  tenant_id: string
  total_notifications: number
  retried: number
  skipped: number
  failed: number
  suppressed: number
  items?: EnterpriseSyncWorkerAlertNotification[]
  updated_at: string
}

export type EnterpriseHRISWebhookReceipt = {
  id: string
  tenant_id: string
  connector_id: string
  vendor: string
  event_type?: string
  request_id?: string
  content_type?: string
  headers?: Record<string, string>
  raw_payload?: string
  source_ip?: string
  status: string
  attempt_count?: number
  last_error?: string
  received_at: string
  last_attempt_at?: string
  processed_at?: string
  queue_state: string
  next_retry_at?: string
  processing_deadline_at?: string
  remaining_attempts: number
  cooldown_remaining_seconds: number
  stale_in_flight: boolean
}

export type EnterpriseHRISWebhookReceiptListResponse = EnterpriseOffsetListResponse<EnterpriseHRISWebhookReceipt> & {
  queue_counts?: EnterpriseHRISWebhookRuntimeCounts
  filter_counts?: EnterpriseHRISWebhookRuntimeCounts
  summary?: Record<string, unknown>
}

export type EnterpriseHRISWebhookReceiptBatchProcessItem = {
  receipt_id: string
  status: string
  reason?: string
  error?: string
  execution_id?: string
  item?: EnterpriseHRISWebhookReceipt
}

export type EnterpriseHRISWebhookReceiptBatchProcessResult = {
  tenant_id: string
  total_receipts: number
  queued?: number
  processed: number
  skipped: number
  failed: number
  dlq: number
  execution_mode?: "inline" | "queued"
  dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback" | string
  items?: EnterpriseHRISWebhookReceiptBatchProcessItem[]
  updated_at: string
}

export type EnterpriseHRISWebhookReceiptProcessResponse = {
  item: EnterpriseHRISWebhookReceipt
  execution_mode?: "inline" | "queued"
  dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback" | string
  execution_id?: string
}

export type EnterpriseHRISWebhookDLQEntry = {
  id: string
  tenant_id: string
  connector_id?: string
  vendor?: string
  receipt_id?: string
  request_id?: string
  event_type?: string
  failure_stage: string
  error: string
  raw_payload_ref?: string
  status: string
  replay_count?: number
  replay_state?: string
  next_retry_at?: string
  processing_deadline_at?: string
  remaining_attempts: number
  cooldown_remaining_seconds: number
  stale_in_flight: boolean
  last_replay_at?: string
  resolved_at?: string
  created_at: string
  updated_at: string
}

export type EnterpriseHRISWebhookDLQListResponse = EnterpriseOffsetListResponse<EnterpriseHRISWebhookDLQEntry> & {
  replay_counts?: EnterpriseHRISWebhookRuntimeCounts
  filter_counts?: EnterpriseHRISWebhookRuntimeCounts
  summary?: Record<string, unknown>
}

export type EnterpriseHRISWebhookDLQBatchReplayItem = {
  entry_id: string
  status: string
  reason?: string
  error?: string
  execution_id?: string
  item?: EnterpriseHRISWebhookDLQEntry
}

export type EnterpriseHRISWebhookDLQBatchReplayResult = {
  tenant_id: string
  total_entries: number
  queued?: number
  replayed: number
  skipped: number
  failed: number
  execution_mode?: "inline" | "queued"
  dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback" | string
  items?: EnterpriseHRISWebhookDLQBatchReplayItem[]
  updated_at: string
}

export type EnterpriseHRISWebhookDLQReplayResponse = {
  item: EnterpriseHRISWebhookDLQEntry
  execution_mode?: "inline" | "queued"
  dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback" | string
  execution_id?: string
}

export type EnterpriseHRISWebhookExecution = {
  id: string
  tenant_id: string
  kind: "receipt_process" | "dlq_replay" | string
  target_id: string
  receipt_id?: string
  connector_id?: string
  vendor?: string
  request_id?: string
  event_type?: string
  failure_stage?: string
  audit_source?: string
  execution_mode?: "inline" | "queued" | string
  dispatch_mode?: string
  status: "queued" | "running" | "succeeded" | "failed" | string
  target_status?: string
  requested_by?: string
  replay_source_execution_id?: string
  replay_require_worker?: boolean
  queue_state?: string
  next_retry_at?: string
  processing_deadline_at?: string
  cooldown_remaining_seconds?: number
  stale_in_flight?: boolean
  attempt_count?: number
  requeue_count?: number
  last_error?: string
  queued_at: string
  started_at?: string
  finished_at?: string
  updated_at: string
}

export type EnterpriseHRISWebhookExecutionStatusCounts = {
  all: number
  queued: number
  running: number
  succeeded: number
  failed: number
}

export type EnterpriseHRISWebhookExecutionListResponse = EnterpriseOffsetListResponse<EnterpriseHRISWebhookExecution> & {
  status_counts: EnterpriseHRISWebhookExecutionStatusCounts
  queue_counts?: EnterpriseHRISWebhookRuntimeCounts
}

export type EnterpriseHRISWebhookExecutionDetailResponse = {
  item: EnterpriseHRISWebhookExecution
}

export type EnterpriseHRISWebhookExecutionReplayResponse = {
  source_execution_id: string
  execution_mode: "inline" | "queued"
  dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback" | string
  execution_id?: string
  execution?: EnterpriseHRISWebhookExecution
  receipt_item?: EnterpriseHRISWebhookReceipt
  dlq_item?: EnterpriseHRISWebhookDLQEntry
}

export type EnterpriseHRISPullState = {
  tenant_id: string
  connector_id: string
  vendor: string
  status: string
  last_request_id?: string
  last_mode?: string
  last_started_at?: string
  last_success_at?: string
  last_full_success_at?: string
  last_failure_at?: string
  last_error?: string
  consecutive_failures?: number
  created_at: string
  updated_at: string
}

export type WalletPassTemplate = {
  id: string
  tenant_id: string
  provider: string
  pass_type: "employee" | "visitor"
  class_id: string
  name: string
  style_config?: Record<string, string>
  status: "active" | "inactive"
  created_at: string
  updated_at: string
}

export type WalletPassInstance = {
  id: string
  tenant_id: string
  provider: string
  template_id: string
  target_type: "user" | "visitor"
  target_id: string
  object_id: string
  status: "issued" | "active" | "suspended" | "revoked"
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

export type Card = {
  id: string
  resource_type: "Card"
  tenant_id: string
  status: "activated" | "deactivated" | "unassigned"
  token: string
  uid?: string
  card_number?: string
  provider: string
  template_id: string
  user_id?: string
  assignee_type?: "User" | "Guest"
  assignee_id?: string
  activation_token?: string
  last_used_at?: string
  issued_at: string
  expires_at?: string
  created_at: string
  updated_at: string
}

export type CardAssignment = {
  id: string
  resource_type: "CardAssignment"
  tenant_id: string
  status: "activated" | "deactivated"
  card_id: string
  card: Card
  assignee_type: "User" | "Guest"
  assignee_id: string
  user_id?: string
  created_at: string
  updated_at: string
}

export type WalletPhysicalCardTask = {
  id: string
  tenant_id: string
  pass_id: string
  template_id: string
  target_type: "user" | "visitor"
  target_id: string
  task_type: "issue" | "reissue" | "loss_report"
  status: "queued" | "printing" | "ready" | "issued" | "reported_lost" | "cancelled"
  card_number?: string
  note?: string
  pass_status: "issued" | "active" | "suspended" | "revoked"
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
  target_type: "user" | "visitor"
  target_id: string
  channels?: string[]
  status: "sent" | "skipped" | "failed"
  reason?: string
  attempt?: number
  retryable: boolean
  provider?: string
  provider_error?: string
  channel_results?: Array<{
    channel: string
    status: "sent" | "skipped" | "failed"
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
  target_type: "user" | "visitor"
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
  status: "sent" | "skipped" | "failed"
  reason?: string
  idempotency_key?: string
  attempt?: number
  retryable: boolean
  provider?: string
  provider_error?: string
  channel_results?: Array<{
    channel: string
    status: "sent" | "skipped" | "failed"
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

export type ServerSentEvent = {
  event: string
  data: string
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
let authTokenProvider: (() => string | null) | null = null

export function setAuthTokenProvider(provider: (() => string | null) | null): void {
  authTokenProvider = provider
}

function resolveAuthToken(token: string | undefined): string | undefined {
  if (token && token.trim() !== "") {
    return token
  }
  const providedToken = authTokenProvider?.()
  if (providedToken && providedToken.trim() !== "") {
    return providedToken
  }
  return getToken() ?? undefined
}

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

async function request<T>(path: string, init: RequestInit, token: string | undefined, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(init.headers ?? {})
  headers.set("Content-Type", "application/json")
  const activeToken = resolveAuthToken(token)
  if (activeToken) {
    headers.set("Authorization", `Bearer ${activeToken}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers,
  })

  if (response.status === 401 && activeToken && !options.skipAuthRecovery) {
    const refreshedToken = await refreshAccessToken()
    if (refreshedToken) {
      return request<T>(path, init, refreshedToken, { skipAuthRecovery: true })
    }
    throw new APIError(401, "Session expired, please sign in again")
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

async function requestItems<T>(path: string, token: string | undefined): Promise<T[]> {
  const payload = await request<{ items?: T[] | null } | T[] | null>(path, { method: "GET" }, token)
  if (Array.isArray(payload)) {
    return payload
  }
  return Array.isArray(payload?.items) ? payload.items : []
}

function normalizeOffsetListResponse<T>(
  payload:
    | EnterpriseOffsetListResponse<T>
    | {
        items?: T[] | null
        total?: number
        offset?: number
        limit?: number
        next_offset?: number
        has_more?: boolean
      }
    | T[]
    | null
): EnterpriseOffsetListResponse<T> {
  const normalizedPayload = payload && !Array.isArray(payload) ? payload : null
  const items = Array.isArray(payload) ? payload : Array.isArray(normalizedPayload?.items) ? normalizedPayload.items : []
  const total =
    typeof normalizedPayload?.total === "number" && Number.isFinite(normalizedPayload.total)
      ? Math.max(0, Math.floor(normalizedPayload.total))
      : items.length
  const offset =
    typeof normalizedPayload?.offset === "number" && Number.isFinite(normalizedPayload.offset) && normalizedPayload.offset >= 0
      ? Math.floor(normalizedPayload.offset)
      : 0
  const limit =
    typeof normalizedPayload?.limit === "number" && Number.isFinite(normalizedPayload.limit) && normalizedPayload.limit >= 0
      ? Math.floor(normalizedPayload.limit)
      : items.length
  const nextOffset =
    typeof normalizedPayload?.next_offset === "number" &&
    Number.isFinite(normalizedPayload.next_offset) &&
    normalizedPayload.next_offset >= 0
      ? Math.floor(normalizedPayload.next_offset)
      : undefined
  const hasMore =
    typeof normalizedPayload?.has_more === "boolean"
      ? normalizedPayload.has_more
      : typeof nextOffset === "number"
        ? nextOffset < total
        : offset + items.length < total

  return {
    items,
    total,
    offset,
    limit,
    next_offset: hasMore ? nextOffset ?? offset + items.length : undefined,
    has_more: hasMore,
  }
}

function normalizeCount(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.floor(value)) : fallback
}

function normalizeWorkerAlertNotificationFilterCounts(
  value: unknown,
  items: EnterpriseSyncWorkerAlertNotification[],
  total: number
): EnterpriseSyncWorkerAlertNotificationFilterCounts {
  const source =
    value && typeof value === "object"
      ? (value as Partial<Record<keyof EnterpriseSyncWorkerAlertNotificationFilterCounts, unknown>>)
      : null
  return {
    all: normalizeCount(source?.all, total),
    failed: normalizeCount(source?.failed, items.filter((item) => item.status === "failed").length),
    retryable: normalizeCount(source?.retryable, items.filter((item) => item.retryable).length),
    suppressed: normalizeCount(
      source?.suppressed,
      items.filter((item) => item.status === "skipped" && item.reason === "manual_suppressed").length
    ),
    due_now: normalizeCount(
      source?.due_now,
      items.filter((item) => {
        if (item.status !== "failed" || !item.retryable || !item.next_retry_at) {
          return false
        }
        const retryAt = new Date(item.next_retry_at).getTime()
        return Number.isFinite(retryAt) && retryAt <= Date.now()
      }).length
    ),
  }
}

function normalizeWorkerAlertNotificationStatusCounts(
  value: unknown,
  items: EnterpriseSyncWorkerAlertNotification[]
): EnterpriseSyncWorkerAlertNotificationStatusCounts {
  const source =
    value && typeof value === "object"
      ? (value as Partial<Record<keyof EnterpriseSyncWorkerAlertNotificationStatusCounts, unknown>>)
      : null
  return {
    sent: normalizeCount(source?.sent, items.filter((item) => item.status === "sent").length),
    failed: normalizeCount(source?.failed, items.filter((item) => item.status === "failed").length),
    skipped: normalizeCount(source?.skipped, items.filter((item) => item.status === "skipped").length),
  }
}

function normalizeHRISWebhookExecutionStatusCounts(
  value: unknown,
  items: EnterpriseHRISWebhookExecution[],
  total: number
): EnterpriseHRISWebhookExecutionStatusCounts {
  const source =
    value && typeof value === "object"
      ? (value as Partial<Record<keyof EnterpriseHRISWebhookExecutionStatusCounts, unknown>>)
      : null
  return {
    all: normalizeCount(source?.all, total),
    queued: normalizeCount(source?.queued, items.filter((item) => item.status === "queued").length),
    running: normalizeCount(source?.running, items.filter((item) => item.status === "running").length),
    succeeded: normalizeCount(source?.succeeded, items.filter((item) => item.status === "succeeded").length),
    failed: normalizeCount(source?.failed, items.filter((item) => item.status === "failed").length),
  }
}

async function requestText(path: string, token: string | undefined): Promise<string> {
  const activeToken = resolveAuthToken(token)
  const headers = new Headers()
  if (activeToken) {
    headers.set("Authorization", `Bearer ${activeToken}`)
  }
  headers.set("Accept", "text/csv, text/plain;q=0.9, */*;q=0.8")

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "GET",
    headers,
  })

  if (response.status === 401 && activeToken) {
    const refreshedToken = await refreshAccessToken()
    if (refreshedToken) {
      return requestText(path, refreshedToken)
    }
    throw new APIError(401, "Session expired, please sign in again")
  }

  if (!response.ok) {
    const message = await parseErrorMessage(response)
    throw new APIError(response.status, message)
  }

  return response.text()
}

export async function consumeServerSentEvents(args: {
  path: string
  token?: string
  signal: AbortSignal
  onEvent: (message: ServerSentEvent) => void
  onError?: (error: Error) => void
}): Promise<void> {
  try {
    const activeToken = resolveAuthToken(args.token)
    const headers = new Headers()
    if (activeToken) {
      headers.set("Authorization", `Bearer ${activeToken}`)
    }
    headers.set("Accept", "text/event-stream")

    const response = await fetch(`${API_BASE_URL}${args.path}`, {
      method: "GET",
      headers,
      signal: args.signal,
    })

    if (response.status === 401 && activeToken) {
      const refreshedToken = await refreshAccessToken()
      if (refreshedToken) {
        return consumeServerSentEvents({
          ...args,
          token: refreshedToken,
        })
      }
      throw new APIError(401, "Session expired, please sign in again")
    }

    if (!response.ok) {
      const message = await parseErrorMessage(response)
      throw new APIError(response.status, message)
    }

    const reader = response.body?.getReader()
    if (!reader) {
      throw new Error("SSE stream body is unavailable")
    }

    const decoder = new TextDecoder()
    let pending = ""

    const emitChunk = (chunk: string): void => {
      const lines = chunk.split("\n")
      let eventName = "message"
      const dataLines: string[] = []

      for (const rawLine of lines) {
        const line = rawLine.replace(/\r$/, "")
        if (line.startsWith("event:")) {
          eventName = line.slice("event:".length).trim() || "message"
          continue
        }
        if (line.startsWith("data:")) {
          dataLines.push(line.slice("data:".length).trimStart())
        }
      }

      if (dataLines.length > 0) {
        args.onEvent({
          event: eventName,
          data: dataLines.join("\n"),
        })
      }
    }

    while (!args.signal.aborted) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      pending += decoder.decode(value, { stream: true })
      const chunks = pending.split("\n\n")
      pending = chunks.pop() ?? ""
      for (const chunk of chunks) {
        if (chunk.trim() === "") {
          continue
        }
        emitChunk(chunk)
      }
    }
  } catch (error) {
    if (args.signal.aborted) {
      return
    }
    const nextError = error instanceof Error ? error : new Error(String(error))
    args.onError?.(nextError)
  }
}

function withTenantQuery(path: string, tenantID?: string): string {
  const value = tenantID?.trim()
  if (!value) {
    return path
  }

  const separator = path.includes("?") ? "&" : "?"
  return `${path}${separator}tenant_id=${encodeURIComponent(value)}`
}

function encodePathSegment(value: string): string {
  return encodeURIComponent(value.trim())
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

export async function getCurrentUser(token: string | undefined): Promise<CurrentUser> {
  return request<CurrentUser>("/api/v1/me", { method: "GET" }, token)
}

export async function listTenants(token: string | undefined): Promise<Tenant[]> {
  return requestItems<Tenant>("/api/v1/tenants", token)
}

export async function createTenant(
  token: string | undefined,
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
  token: string | undefined,
  tenantID: string,
  status: "active" | "suspended" | "inactive"
): Promise<Tenant> {
  return request<Tenant>(
    `/api/v1/tenants/${encodePathSegment(tenantID)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify({ status }),
    },
    token
  )
}

export async function getTenantTopology(token: string | undefined, tenantID: string): Promise<TenantTopology> {
  return request<TenantTopology>(`/api/v1/tenants/${encodePathSegment(tenantID)}/topology`, { method: "GET" }, token)
}

export async function listBuildings(token: string | undefined, tenantID?: string): Promise<Building[]> {
  return requestItems<Building>(withTenantQuery("/api/v1/buildings", tenantID), token)
}

export async function listPlaces(token: string | undefined, tenantID?: string): Promise<Place[]> {
  return requestItems<Place>(withTenantQuery("/api/v1/places", tenantID), token)
}

export async function createBuilding(
  token: string | undefined,
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

export async function listFloors(token: string | undefined, tenantID?: string): Promise<Floor[]> {
  return requestItems<Floor>(withTenantQuery("/api/v1/floors", tenantID), token)
}

export async function createFloor(
  token: string | undefined,
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

export async function listAreas(token: string | undefined, tenantID?: string): Promise<Area[]> {
  return requestItems<Area>(withTenantQuery("/api/v1/areas", tenantID), token)
}

export async function createArea(
  token: string | undefined,
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

export async function listDoors(token: string | undefined, tenantID?: string): Promise<Door[]> {
  return requestItems<Door>(withTenantQuery("/api/v1/doors", tenantID), token)
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
    "/api/v1/doors",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listGateways(token: string | undefined): Promise<Gateway[]> {
  return requestItems<Gateway>("/api/v1/gateways", token)
}

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
  token: string | undefined,
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): Promise<GatewaySerialInventoryItem[]> {
  return requestItems<GatewaySerialInventoryItem>(buildGatewaySerialInventoryPath(tenantID, options), token)
}

export async function importGatewaySerialInventory(
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
  gatewayID: string,
  doorID: string
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/bind-door`,
    {
      method: "POST",
      body: JSON.stringify({ door_id: doorID }),
    },
    token
  )
}

export async function unbindGatewayDoor(
  token: string | undefined,
  gatewayID: string,
  doorID: string
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/unbind-door`,
    {
      method: "POST",
      body: JSON.stringify({ door_id: doorID }),
    },
    token
  )
}

export async function registerGatewayDevice(
  token: string | undefined,
  gatewayID: string,
  payload: {
    serial_number: string
    kind?: "reader" | "door_controller" | "relay" | "sensor" | "legacy_reader" | "legacy_controller"
    source?: "mistypass_procured" | "legacy_integration"
    protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"
    rs485_config?: {
      baud_rate?: number
      parity?: "none" | "even" | "odd"
      stop_bits?: number
      device_address?: number
      timeout_ms?: number
    }
    status?: "online" | "offline"
  }
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/devices`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function reportGatewayDeviceRS485Telemetry(
  token: string | undefined,
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
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/devices/${encodePathSegment(deviceID)}/rs485/telemetry`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function probeGatewayLegacyDevices(
  token: string | undefined,
  gatewayID: string
): Promise<string[]> {
  const payload = await request<{ items: string[] }>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/devices/probe-legacy`,
    { method: "POST" },
    token
  )
  return payload.items
}

export async function publishGatewayConfig(
  token: string | undefined,
  gatewayID: string,
  version: string
): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/config/publish`,
    {
      method: "POST",
      body: JSON.stringify({ version }),
    },
    token
  )
}

export async function rebootGateway(token: string | undefined, gatewayID: string): Promise<GatewayCommandAck> {
  return request<GatewayCommandAck>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/reboot`,
    {
      method: "POST",
    },
    token
  )
}

export async function listGatewayEventCheckpointSummary(
  token: string | undefined,
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

export async function listAccessPolicies(token: string | undefined): Promise<AccessPolicy[]> {
  return requestItems<AccessPolicy>("/api/v1/access-policies", token)
}

export async function listAccessUsers(token: string | undefined, tenantID?: string): Promise<AccessUser[]> {
  return requestItems<AccessUser>(withTenantQuery("/api/v1/users", tenantID), token)
}

export async function createAccessPolicy(
  token: string | undefined,
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
  token: string | undefined,
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
    `/api/v1/access-policies/${encodePathSegment(policyID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listUserGroups(token: string | undefined): Promise<UserGroup[]> {
  return requestItems<UserGroup>("/api/v1/user-groups", token)
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

export async function listDoorGroups(token: string | undefined, tenantID?: string): Promise<DoorGroup[]> {
  return requestItems<DoorGroup>(withTenantQuery("/api/v1/door-groups", tenantID), token)
}

export async function listTeams(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    scope?: Team["scope"]
    place_id?: string
    sort?: "name" | "-name"
  }
): Promise<Team[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.scope?.trim()) query.set("scope", options.scope.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Team>(suffix ? `/api/v1/teams?${suffix}` : "/api/v1/teams", token)
}

export async function listTeamMemberships(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    team_id?: string
    member_type?: TeamMembership["member_type"]
    member_id?: string
  }
): Promise<TeamMembership[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.team_id?.trim()) query.set("team_id", options.team_id.trim())
  if (options?.member_type?.trim()) query.set("member_type", options.member_type.trim())
  if (options?.member_id?.trim()) query.set("member_id", options.member_id.trim())
  const suffix = query.toString()
  return requestItems<TeamMembership>(suffix ? `/api/v1/team_memberships?${suffix}` : "/api/v1/team_memberships", token)
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
    "/api/v1/user-groups",
    {
      method: "POST",
      body: JSON.stringify(payload),
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
    `/api/v1/user-groups/${encodePathSegment(groupID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listTemporaryAccess(token: string | undefined): Promise<TemporaryAccess[]> {
  return requestItems<TemporaryAccess>("/api/v1/temporary-access", token)
}

export async function listRoles(token: string | undefined, appliesTo?: Role["applies_to"]): Promise<Role[]> {
  const path = appliesTo ? `/api/v1/roles?applies_to=${encodeURIComponent(appliesTo)}` : "/api/v1/roles"
  return requestItems<Role>(path, token)
}

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

export async function listShares(
  token: string | undefined,
  options?: {
    tenant_id?: string
    place_id?: string
    lock_id?: string
    role_id?: string
    email?: string
  }
): Promise<Share[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.lock_id?.trim()) query.set("lock_id", options.lock_id.trim())
  if (options?.role_id?.trim()) query.set("role_id", options.role_id.trim())
  if (options?.email?.trim()) query.set("email", options.email.trim())
  const suffix = query.toString()
  return requestItems<Share>(suffix ? `/api/v1/shares?${suffix}` : "/api/v1/shares", token)
}

export async function createTemporaryAccess(
  token: string | undefined,
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

export async function listVisitorPasses(token: string | undefined): Promise<VisitorPass[]> {
  return requestItems<VisitorPass>("/api/v1/visitor-passes", token)
}

type ListPageOptions = {
  page?: number
  limit?: number
}

function withPageQuery(path: string, options?: ListPageOptions): string {
  const query = new URLSearchParams()
  if (typeof options?.page === "number" && Number.isFinite(options.page) && options.page > 0) {
    query.set("page", String(Math.floor(options.page)))
  }
  if (typeof options?.limit === "number" && Number.isFinite(options.limit) && options.limit > 0) {
    query.set("limit", String(Math.floor(options.limit)))
  }
  const suffix = query.toString()
  if (!suffix) {
    return path
  }
  const separator = path.includes("?") ? "&" : "?"
  return `${path}${separator}${suffix}`
}

export async function createVisitorPass(
  token: string | undefined,
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

export async function listAccessEvents(token: string | undefined, options?: ListPageOptions): Promise<AccessEvent[]> {
  return requestItems<AccessEvent>(withPageQuery("/api/v1/events/access", options), token)
}

export async function listDeviceEvents(token: string | undefined, options?: ListPageOptions): Promise<DeviceEvent[]> {
  return requestItems<DeviceEvent>(withPageQuery("/api/v1/events/device", options), token)
}

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

export async function listIntegrations(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    type?: Integration["type"]
    provider?: string
    status?: string
    sort?: "name" | "-name"
  }
): Promise<Integration[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.type?.trim()) query.set("type", options.type.trim())
  if (options?.provider?.trim()) query.set("provider", options.provider.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Integration>(suffix ? `/api/v1/integrations?${suffix}` : "/api/v1/integrations", token)
}

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

export async function listAuditLogs(token: string | undefined): Promise<AuditLog[]> {
  return requestItems<AuditLog>("/api/v1/audit-logs", token)
}

export async function getEnterpriseIDPConfig(token: string | undefined, tenantID?: string): Promise<EnterpriseIDPConfig> {
  return request<EnterpriseIDPConfig>(withTenantQuery("/api/v1/enterprise/idp-config", tenantID), { method: "GET" }, token)
}

export async function listEnterpriseEmployees(token: string | undefined, tenantID?: string): Promise<EnterpriseEmployee[]> {
  return requestItems<EnterpriseEmployee>(withTenantQuery("/api/v1/enterprise/employees", tenantID), token)
}

export async function listEnterpriseHRISConnectors(
  token: string | undefined,
  tenantID?: string
): Promise<EnterpriseHRISConnector[]> {
  return requestItems<EnterpriseHRISConnector>(withTenantQuery("/api/v1/enterprise/hris-connectors", tenantID), token)
}

export async function listEnterpriseHRISSecrets(
  token: string | undefined,
  tenantID?: string
): Promise<EnterpriseHRISSecret[]> {
  return requestItems<EnterpriseHRISSecret>(withTenantQuery("/api/v1/enterprise/hris-secrets", tenantID), token)
}

export async function upsertEnterpriseHRISSecret(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    kind?: string
    value: string
    updated_by?: string
  }
): Promise<EnterpriseHRISSecret> {
  return request<{ item: EnterpriseHRISSecret }>(
    "/api/v1/enterprise/hris-secrets",
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
    token
  ).then((response) => response.item)
}

export async function createEnterpriseHRISConnector(
  token: string | undefined,
  payload: {
    tenant_id: string
    vendor: EnterpriseHRISConnector["vendor"]
    status?: EnterpriseHRISConnector["status"]
    sync_strategy?: EnterpriseHRISConnector["sync_strategy"]
    credential_ref?: string
    credential_value?: string
    webhook_secret_ref?: string
    webhook_secret_value?: string
    updated_by?: string
  }
): Promise<EnterpriseHRISConnector> {
  return request<EnterpriseHRISConnector>(
    "/api/v1/enterprise/hris-connectors",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateEnterpriseHRISConnector(
  token: string | undefined,
  connectorID: string,
  payload: {
    tenant_id: string
    status?: EnterpriseHRISConnector["status"]
    sync_strategy?: EnterpriseHRISConnector["sync_strategy"]
    credential_ref?: string
    credential_value?: string
    webhook_secret_ref?: string
    webhook_secret_value?: string
    updated_by?: string
  }
): Promise<EnterpriseHRISConnector> {
  return request<EnterpriseHRISConnector>(
    `/api/v1/enterprise/hris-connectors/${encodePathSegment(connectorID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listEnterpriseJITProvisionApprovals(
  token: string | undefined,
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
  token: string | undefined,
  approvalID: string,
  payload: {
    tenant_id: string
    decision: "approved" | "rejected"
    reviewed_by?: string
    reason?: string
  }
): Promise<EnterpriseJITProvisionApproval> {
  const response = await request<{ item: EnterpriseJITProvisionApproval }>(
    `/api/v1/enterprise/jit-provision-approvals/${encodePathSegment(approvalID)}/review`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.item
}

export async function updateEnterpriseJITProvisionApprovalExternalSync(
  token: string | undefined,
  approvalID: string,
  payload: {
    tenant_id: string
    status: "synced" | "failed"
    external_sync_ref?: string
    last_error?: string
  }
): Promise<EnterpriseJITProvisionApproval> {
  const response = await request<{ item: EnterpriseJITProvisionApproval }>(
    `/api/v1/enterprise/jit-provision-approvals/${encodePathSegment(approvalID)}/external-sync`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.item
}

export async function syncEnterpriseEmployees(
  token: string | undefined,
  payload: {
    tenant_id: string
    source: string
    actor?: string
    request_id?: string
    connector_id?: string
    raw_payload_ref?: string
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

export async function listEnterpriseSyncJobs(token: string | undefined, tenantID?: string): Promise<EnterpriseSyncJob[]> {
  return requestItems<EnterpriseSyncJob>(withTenantQuery("/api/v1/enterprise/sync-jobs", tenantID), token)
}

export async function listEnterpriseSyncRequests(
  token: string | undefined,
  options?: {
    tenant_id?: string
    limit?: number
  }
): Promise<EnterpriseSyncRequestRecord[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/sync-requests?${suffix}` : "/api/v1/enterprise/sync-requests"
  return requestItems<EnterpriseSyncRequestRecord>(path, token)
}

export async function reconcilePendingEnterpriseSyncRequests(
  token: string | undefined,
  payload: {
    tenant_id: string
    limit?: number
  }
): Promise<EnterpriseBatchPendingSyncReconcileResult> {
  return request<EnterpriseBatchPendingSyncReconcileResult>(
    "/api/v1/enterprise/sync-requests/reconcile-pending",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listEnterpriseSyncWorkerAlertSummary(
  token: string | undefined,
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

export async function getEnterpriseSyncWorkerAlertSubscription(
  token: string | undefined,
  options: {
    tenant_id?: string
  }
): Promise<EnterpriseSyncWorkerAlertSubscription> {
  return request<EnterpriseSyncWorkerAlertSubscription>(
    withTenantQuery("/api/v1/enterprise/sync-worker-alert-subscription", options.tenant_id),
    { method: "GET" },
    token
  )
}

export async function upsertEnterpriseSyncWorkerAlertSubscription(
  token: string | undefined,
  payload: {
    tenant_id: string
    enabled?: boolean
    worker_alert_threshold?: number
    window_seconds?: number
    cooldown_seconds?: number
    channels?: {
      email?: boolean
      whatsapp?: boolean
    }
    receiver_groups?: string[]
  }
): Promise<EnterpriseSyncWorkerAlertSubscription> {
  return request<EnterpriseSyncWorkerAlertSubscription>(
    "/api/v1/enterprise/sync-worker-alert-subscription",
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function dispatchEnterpriseSyncWorkerAlerts(
  token: string | undefined,
  payload: {
    tenant_id: string
    actor?: string
    worker_actions?: string[]
  }
): Promise<EnterpriseSyncWorkerAlertDispatchResult> {
  return request<EnterpriseSyncWorkerAlertDispatchResult>(
    "/api/v1/enterprise/sync-worker-alerts/dispatch",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function listEnterpriseSyncWorkerAlertNotifications(
  token: string | undefined,
  options?: {
    tenant_id?: string
    status?: string
    reason?: string
    q?: string
    retryable?: boolean
    due_now?: boolean
    offset?: number
    limit?: number
  }
): Promise<EnterpriseSyncWorkerAlertNotificationListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.status?.trim()) {
    query.set("status", options.status.trim())
  }
  if (options?.reason?.trim()) {
    query.set("reason", options.reason.trim())
  }
  if (options?.q?.trim()) {
    query.set("q", options.q.trim())
  }
  if (typeof options?.retryable === "boolean") {
    query.set("retryable", String(options.retryable))
  }
  if (typeof options?.due_now === "boolean") {
    query.set("due_now", String(options.due_now))
  }
  if (typeof options?.offset === "number" && options.offset >= 0) {
    query.set("offset", String(options.offset))
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/enterprise/sync-worker-alerts/notifications?${suffix}`
    : "/api/v1/enterprise/sync-worker-alerts/notifications"
  const payload = await request<
    | (Partial<EnterpriseSyncWorkerAlertNotificationListResponse> & {
        items?: EnterpriseSyncWorkerAlertNotification[] | null
      })
    | EnterpriseSyncWorkerAlertNotification[]
    | null
  >(path, { method: "GET" }, token)
  const normalizedList = normalizeOffsetListResponse<EnterpriseSyncWorkerAlertNotification>(payload)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  const filterCounts = normalizeWorkerAlertNotificationFilterCounts(
    payloadObject?.filter_counts,
    normalizedList.items,
    normalizedList.total
  )
  const statusCounts = normalizeWorkerAlertNotificationStatusCounts(payloadObject?.status_counts, normalizedList.items)
  return {
    ...(payloadObject ?? {}),
    ...normalizedList,
    filter_counts: filterCounts,
    status_counts: statusCounts,
  }
}

export async function exportEnterpriseSyncWorkerAlertNotificationsCSV(
  token: string | undefined,
  options?: {
    tenant_id?: string
    status?: string
    reason?: string
    q?: string
    retryable?: boolean
    due_now?: boolean
  }
): Promise<string> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.status?.trim()) {
    query.set("status", options.status.trim())
  }
  if (options?.reason?.trim()) {
    query.set("reason", options.reason.trim())
  }
  if (options?.q?.trim()) {
    query.set("q", options.q.trim())
  }
  if (typeof options?.retryable === "boolean") {
    query.set("retryable", String(options.retryable))
  }
  if (typeof options?.due_now === "boolean") {
    query.set("due_now", String(options.due_now))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/enterprise/sync-worker-alerts/notifications/export-csv?${suffix}`
    : "/api/v1/enterprise/sync-worker-alerts/notifications/export-csv"
  return requestText(path, token)
}

export async function retryEnterpriseSyncWorkerAlertNotification(
  token: string | undefined,
  payload: {
    tenant_id: string
    notification_id: string
    actor?: string
  }
): Promise<EnterpriseSyncWorkerAlertNotification> {
  const notificationID = payload.notification_id.trim()
  return request<EnterpriseSyncWorkerAlertNotification>(
    `/api/v1/enterprise/sync-worker-alerts/notifications/${encodeURIComponent(notificationID)}/retry`,
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

export async function retryEnterpriseSyncWorkerAlertNotificationsBatch(
  token: string | undefined,
  payload: {
    tenant_id: string
    notification_ids: string[]
    actor?: string
  }
): Promise<EnterpriseSyncWorkerAlertNotificationBatchRetryResult> {
  return request<EnterpriseSyncWorkerAlertNotificationBatchRetryResult>(
    "/api/v1/enterprise/sync-worker-alerts/notifications/retry-batch",
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: payload.tenant_id,
        notification_ids: payload.notification_ids,
        actor: payload.actor,
      }),
    },
    token
  )
}

export async function suppressEnterpriseSyncWorkerAlertNotificationsBatch(
  token: string | undefined,
  payload: {
    tenant_id: string
    notification_ids: string[]
  }
): Promise<EnterpriseSyncWorkerAlertNotificationBatchSuppressResult> {
  return request<EnterpriseSyncWorkerAlertNotificationBatchSuppressResult>(
    "/api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch",
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: payload.tenant_id,
        notification_ids: payload.notification_ids,
      }),
    },
    token
  )
}

export async function restoreEnterpriseSyncWorkerAlertNotificationsBatch(
  token: string | undefined,
  payload: {
    tenant_id: string
    notification_ids: string[]
    actor?: string
  }
): Promise<EnterpriseSyncWorkerAlertNotificationBatchRestoreResult> {
  return request<EnterpriseSyncWorkerAlertNotificationBatchRestoreResult>(
    "/api/v1/enterprise/sync-worker-alerts/notifications/restore-batch",
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: payload.tenant_id,
        notification_ids: payload.notification_ids,
        actor: payload.actor,
      }),
    },
    token
  )
}

export async function autoRetryEnterpriseSyncWorkerAlertNotifications(
  token: string | undefined,
  payload: {
    tenant_id: string
    actor?: string
    limit?: number
    max_attempts?: number
    base_backoff_ms?: number
    max_backoff_ms?: number
  }
): Promise<EnterpriseSyncWorkerAlertNotificationAutoRetryResult> {
  return request<EnterpriseSyncWorkerAlertNotificationAutoRetryResult>(
    "/api/v1/enterprise/sync-worker-alerts/notifications/auto-retry",
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: payload.tenant_id,
        actor: payload.actor,
        limit: payload.limit,
        max_attempts: payload.max_attempts,
        base_backoff_ms: payload.base_backoff_ms,
        max_backoff_ms: payload.max_backoff_ms,
      }),
    },
    token
  )
}

export async function listEnterpriseSyncWorkerAlerts(
  token: string | undefined,
  options?: {
    tenant_id?: string
    limit?: number
  }
): Promise<EnterpriseSyncWorkerAlertItem[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/sync-worker-alerts?${suffix}` : "/api/v1/enterprise/sync-worker-alerts"
  return requestItems<EnterpriseSyncWorkerAlertItem>(path, token)
}

export async function listEnterpriseHRISWebhookDLQ(
  token: string | undefined,
  options?: {
    tenant_id?: string
    connector_id?: string
    offset?: number
    limit?: number
  }
): Promise<EnterpriseHRISWebhookDLQListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.connector_id?.trim()) {
    query.set("connector_id", options.connector_id.trim())
  }
  if (typeof options?.offset === "number" && options.offset >= 0) {
    query.set("offset", String(Math.floor(options.offset)))
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/hris-webhook-dlq?${suffix}` : "/api/v1/enterprise/hris-webhook-dlq"
  const payload = await request<
    | EnterpriseHRISWebhookDLQListResponse
    | { items?: EnterpriseHRISWebhookDLQEntry[] | null }
    | EnterpriseHRISWebhookDLQEntry[]
    | null
  >(path, { method: "GET" }, token)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  return {
    ...(payloadObject ?? {}),
    ...normalizeOffsetListResponse(payload),
  }
}

export async function listEnterpriseHRISWebhookReceipts(
  token: string | undefined,
  options?: {
    tenant_id?: string
    connector_id?: string
    offset?: number
    limit?: number
  }
): Promise<EnterpriseHRISWebhookReceiptListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.connector_id?.trim()) {
    query.set("connector_id", options.connector_id.trim())
  }
  if (typeof options?.offset === "number" && options.offset >= 0) {
    query.set("offset", String(Math.floor(options.offset)))
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path =
    suffix ? `/api/v1/enterprise/hris-webhook-receipts?${suffix}` : "/api/v1/enterprise/hris-webhook-receipts"
  const payload = await request<
    | EnterpriseHRISWebhookReceiptListResponse
    | { items?: EnterpriseHRISWebhookReceipt[] | null }
    | EnterpriseHRISWebhookReceipt[]
    | null
  >(path, { method: "GET" }, token)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  return {
    ...(payloadObject ?? {}),
    ...normalizeOffsetListResponse(payload),
  }
}

export async function listEnterpriseHRISWebhookExecutions(
  token: string | undefined,
  options?: {
    tenant_id?: string
    connector_id?: string
    kind?: "receipt_process" | "dlq_replay"
    status?: "queued" | "running" | "succeeded" | "failed"
    queue_state?: "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"
    replay_scope?: "replayed" | "worker_required"
    execution_mode?: "inline" | "queued"
    dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback"
    target_status?: string
    target_id?: string
    q?: string
    offset?: number
    limit?: number
  }
): Promise<EnterpriseHRISWebhookExecutionListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.connector_id?.trim()) {
    query.set("connector_id", options.connector_id.trim())
  }
  if (options?.kind?.trim()) {
    query.set("kind", options.kind.trim())
  }
  if (options?.status?.trim()) {
    query.set("status", options.status.trim())
  }
  if (options?.queue_state?.trim()) {
    query.set("queue_state", options.queue_state.trim())
  }
  if (options?.replay_scope?.trim()) {
    query.set("replay_scope", options.replay_scope.trim())
  }
  if (options?.execution_mode?.trim()) {
    query.set("execution_mode", options.execution_mode.trim())
  }
  if (options?.dispatch_mode?.trim()) {
    query.set("dispatch_mode", options.dispatch_mode.trim())
  }
  if (options?.target_status?.trim()) {
    query.set("target_status", options.target_status.trim())
  }
  if (options?.target_id?.trim()) {
    query.set("target_id", options.target_id.trim())
  }
  if (options?.q?.trim()) {
    query.set("q", options.q.trim())
  }
  if (typeof options?.offset === "number" && options.offset >= 0) {
    query.set("offset", String(Math.floor(options.offset)))
  }
  if (typeof options?.limit === "number" && options.limit > 0) {
    query.set("limit", String(options.limit))
  }
  const suffix = query.toString()
  const path =
    suffix ? `/api/v1/enterprise/hris-webhook-executions?${suffix}` : "/api/v1/enterprise/hris-webhook-executions"
  const payload = await request<
    | EnterpriseHRISWebhookExecutionListResponse
    | { items?: EnterpriseHRISWebhookExecution[] | null }
    | EnterpriseHRISWebhookExecution[]
    | null
  >(path, { method: "GET" }, token)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  const normalizedList = normalizeOffsetListResponse<EnterpriseHRISWebhookExecution>(payload)
  const statusCounts = normalizeHRISWebhookExecutionStatusCounts(
    payloadObject && "status_counts" in payloadObject ? payloadObject.status_counts : null,
    normalizedList.items,
    normalizedList.total
  )
  return {
    ...(payloadObject ?? {}),
    ...normalizedList,
    status_counts: statusCounts,
  }
}

export async function getEnterpriseHRISWebhookExecution(
  token: string | undefined,
  executionID: string,
  options: {
    tenant_id: string
  }
): Promise<EnterpriseHRISWebhookExecutionDetailResponse> {
  const nextExecutionID = executionID.trim()
  if (!nextExecutionID) {
    throw new Error("execution_id is required")
  }
  const query = new URLSearchParams()
  if (options.tenant_id.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/enterprise/hris-webhook-executions/${encodeURIComponent(nextExecutionID)}?${suffix}`
    : `/api/v1/enterprise/hris-webhook-executions/${encodeURIComponent(nextExecutionID)}`
  return request<EnterpriseHRISWebhookExecutionDetailResponse>(path, { method: "GET" }, token)
}

export async function replayEnterpriseHRISWebhookExecution(
  token: string | undefined,
  input: {
    tenant_id: string
    execution_id: string
    execution_mode?: "inline" | "queued"
    require_worker?: boolean
  }
): Promise<EnterpriseHRISWebhookExecutionReplayResponse> {
  const executionID = input.execution_id.trim()
  if (!executionID) {
    throw new Error("execution_id is required")
  }
  return request<EnterpriseHRISWebhookExecutionReplayResponse>(
    `/api/v1/enterprise/hris-webhook-executions/${encodeURIComponent(executionID)}/replay`,
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: input.tenant_id,
        execution_mode: input.execution_mode,
        require_worker: input.require_worker,
      }),
    },
    token
  )
}

export async function processBatchEnterpriseHRISWebhookReceipts(
  token: string | undefined,
  input: {
    tenant_id: string
    receipt_ids: string[]
    execution_mode?: "inline" | "queued"
    require_worker?: boolean
  }
): Promise<EnterpriseHRISWebhookReceiptBatchProcessResult> {
  return request<EnterpriseHRISWebhookReceiptBatchProcessResult>(
    "/api/v1/enterprise/hris-webhook-receipts/process-batch",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
    token
  )
}

export async function processEnterpriseHRISWebhookReceipt(
  token: string | undefined,
  input: {
    tenant_id: string
    receipt_id: string
    execution_mode?: "inline" | "queued"
    require_worker?: boolean
  }
): Promise<EnterpriseHRISWebhookReceiptProcessResponse> {
  return request<EnterpriseHRISWebhookReceiptProcessResponse>(
    `/api/v1/enterprise/hris-webhook-receipts/${encodePathSegment(input.receipt_id)}/process`,
    {
      method: "POST",
      body: JSON.stringify({
        tenant_id: input.tenant_id,
        execution_mode: input.execution_mode,
        require_worker: input.require_worker,
      }),
    },
    token
  )
}

export async function listEnterpriseHRISPullStates(
  token: string | undefined,
  tenantID?: string
): Promise<EnterpriseHRISPullState[]> {
  return requestItems<EnterpriseHRISPullState>(withTenantQuery("/api/v1/enterprise/hris-pull-states", tenantID), token)
}

export async function replayEnterpriseHRISWebhookDLQ(
  token: string | undefined,
  entryID: string,
  input?: {
    execution_mode?: "inline" | "queued"
    require_worker?: boolean
  }
): Promise<EnterpriseHRISWebhookDLQReplayResponse> {
  const nextEntryID = entryID.trim()
  const query = new URLSearchParams()
  if (input?.execution_mode?.trim()) {
    query.set("execution_mode", input.execution_mode.trim())
  }
  if (typeof input?.require_worker === "boolean") {
    query.set("require_worker", String(input.require_worker))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/enterprise/hris-webhook-dlq/${encodePathSegment(nextEntryID)}/replay?${suffix}`
    : `/api/v1/enterprise/hris-webhook-dlq/${encodePathSegment(nextEntryID)}/replay`
  return request<EnterpriseHRISWebhookDLQReplayResponse>(
    path,
    {
      method: "POST",
    },
    token
  )
}

export async function replayBatchEnterpriseHRISWebhookDLQ(
  token: string | undefined,
  input: {
    tenant_id: string
    entry_ids: string[]
    execution_mode?: "inline" | "queued"
    require_worker?: boolean
  }
): Promise<EnterpriseHRISWebhookDLQBatchReplayResult> {
  return request<EnterpriseHRISWebhookDLQBatchReplayResult>(
    "/api/v1/enterprise/hris-webhook-dlq/replay-batch",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
    token
  )
}

export async function listWalletTemplates(token: string | undefined, tenantID?: string): Promise<WalletPassTemplate[]> {
  return requestItems<WalletPassTemplate>(withTenantQuery("/api/v1/wallet/templates", tenantID), token)
}

export async function createWalletTemplate(
  token: string | undefined,
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
  token: string | undefined,
  templateID: string,
  payload: {
    tenant_id: string
    status: "active" | "inactive"
    actor?: string
  }
): Promise<WalletPassTemplate> {
  return request<WalletPassTemplate>(
    withTenantQuery(`/api/v1/wallet/templates/${encodePathSegment(templateID)}/status`, payload.tenant_id),
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
  token: string | undefined,
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
  token: string | undefined,
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
  execution_mode: "inline" | "queued"
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

export async function listWalletPasses(
  token: string | undefined,
  tenantID?: string,
  options?: ListPageOptions
): Promise<WalletPassInstance[]> {
  return requestItems<WalletPassInstance>(withPageQuery(withTenantQuery("/api/v1/wallet/passes", tenantID), options), token)
}

export async function getWalletPass(token: string | undefined, passID: string, tenantID?: string): Promise<WalletPassInstance> {
  return request<WalletPassInstance>(
    withTenantQuery(`/api/v1/wallet/passes/${encodePathSegment(passID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function getWalletPassSaveLink(token: string | undefined, passID: string, tenantID?: string): Promise<string> {
  const payload = await request<{ save_link: string }>(
    withTenantQuery(`/api/v1/wallet/passes/${encodePathSegment(passID)}/save-link`, tenantID),
    { method: "GET" },
    token
  )
  return payload.save_link
}

export async function listWalletPhysicalCardTasks(token: string | undefined, tenantID?: string): Promise<WalletPhysicalCardTask[]> {
  return requestItems<WalletPhysicalCardTask>(withTenantQuery("/api/v1/wallet/physical-card-tasks", tenantID), token)
}

export async function listWalletPassDeliveries(
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
    withTenantQuery(`/api/v1/wallet/physical-card-tasks/${encodePathSegment(taskID)}/status`, payload.tenant_id),
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
  token: string | undefined,
  passID: string,
  action: "suspend" | "activate" | "revoke",
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return request<WalletPassInstance>(
    withTenantQuery(`/api/v1/wallet/passes/${encodePathSegment(passID)}/${action}`, payload.tenant_id),
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
  token: string | undefined,
  passID: string,
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return updateWalletPassStatus(token, passID, "suspend", payload)
}

export async function activateWalletPass(
  token: string | undefined,
  passID: string,
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return updateWalletPassStatus(token, passID, "activate", payload)
}

export async function revokeWalletPass(
  token: string | undefined,
  passID: string,
  payload: {
    tenant_id: string
    actor?: string
  }
): Promise<WalletPassInstance> {
  return updateWalletPassStatus(token, passID, "revoke", payload)
}

export async function listCards(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    status?: Card["status"]
    user_id?: string
    token?: string
    uid?: string
    card_number?: string
    card_id?: string
  }
): Promise<Card[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.user_id?.trim()) query.set("user_id", options.user_id.trim())
  if (options?.token?.trim()) query.set("token", options.token.trim())
  if (options?.uid?.trim()) query.set("uid", options.uid.trim())
  if (options?.card_number?.trim()) query.set("card_number", options.card_number.trim())
  if (options?.card_id?.trim()) query.set("card_id", options.card_id.trim())
  const suffix = query.toString()
  return requestItems<Card>(suffix ? `/api/v1/cards?${suffix}` : "/api/v1/cards", token)
}

export async function listCardAssignments(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    status?: CardAssignment["status"]
    card_id?: string
    assignee_type?: CardAssignment["assignee_type"]
    assignee_id?: string
    user_id?: string
  }
): Promise<CardAssignment[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.card_id?.trim()) query.set("card_id", options.card_id.trim())
  if (options?.assignee_type?.trim()) query.set("assignee_type", options.assignee_type.trim())
  if (options?.assignee_id?.trim()) query.set("assignee_id", options.assignee_id.trim())
  if (options?.user_id?.trim()) query.set("user_id", options.user_id.trim())
  const suffix = query.toString()
  return requestItems<CardAssignment>(suffix ? `/api/v1/card_assignments?${suffix}` : "/api/v1/card_assignments", token)
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
  token: string | undefined,
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
