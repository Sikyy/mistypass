import { request, requestItems, requestText, withTenantQuery, encodePathSegment, normalizeOffsetListResponse, normalizeCount } from "./core"
import type { EnterpriseOffsetListResponse, OffsetPaginationPayload } from "./core"

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

export type EnterpriseHRISWebhookRuntimeCounts = {
  all: number
  ready: number
  cooldown: number
  in_flight: number
  attempt_limit: number
  terminal: number
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

// --- Normalizer helpers ---

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

// --- API functions ---

export async function getEnterpriseIDPConfig(token: string | undefined, tenantID?: string): Promise<EnterpriseIDPConfig> {
  return request<EnterpriseIDPConfig>(withTenantQuery("/api/v1/enterprise/idp-config", tenantID), { method: "GET" }, token)
}

export async function listEnterpriseEmployees(token: string | undefined, tenantID?: string): Promise<EnterpriseEmployee[]> {
  return requestItems<EnterpriseEmployee>(withTenantQuery("/api/v1/enterprise/employees", tenantID), token)
}

export async function listEnterpriseHRISConnectors(token: string | undefined, tenantID?: string): Promise<EnterpriseHRISConnector[]> {
  return requestItems<EnterpriseHRISConnector>(withTenantQuery("/api/v1/enterprise/hris-connectors", tenantID), token)
}

export async function listEnterpriseHRISSecrets(token: string | undefined, tenantID?: string): Promise<EnterpriseHRISSecret[]> {
  return requestItems<EnterpriseHRISSecret>(withTenantQuery("/api/v1/enterprise/hris-secrets", tenantID), token)
}

export async function upsertEnterpriseHRISSecret(token: string | undefined, payload: { tenant_id: string; name: string; kind?: string; value: string; updated_by?: string }): Promise<EnterpriseHRISSecret> {
  return request<{ item: EnterpriseHRISSecret }>("/api/v1/enterprise/hris-secrets", { method: "PUT", body: JSON.stringify(payload) }, token).then((response) => response.item)
}

export async function createEnterpriseHRISConnector(token: string | undefined, payload: { tenant_id: string; vendor: EnterpriseHRISConnector["vendor"]; status?: EnterpriseHRISConnector["status"]; sync_strategy?: EnterpriseHRISConnector["sync_strategy"]; credential_ref?: string; credential_value?: string; webhook_secret_ref?: string; webhook_secret_value?: string; updated_by?: string }): Promise<EnterpriseHRISConnector> {
  return request<EnterpriseHRISConnector>("/api/v1/enterprise/hris-connectors", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function updateEnterpriseHRISConnector(token: string | undefined, connectorID: string, payload: { tenant_id: string; status?: EnterpriseHRISConnector["status"]; sync_strategy?: EnterpriseHRISConnector["sync_strategy"]; credential_ref?: string; credential_value?: string; webhook_secret_ref?: string; webhook_secret_value?: string; updated_by?: string }): Promise<EnterpriseHRISConnector> {
  return request<EnterpriseHRISConnector>(`/api/v1/enterprise/hris-connectors/${encodePathSegment(connectorID)}`, { method: "PATCH", body: JSON.stringify(payload) }, token)
}

export async function listEnterpriseJITProvisionApprovals(token: string | undefined, options?: { tenant_id?: string; status?: string; limit?: number }): Promise<EnterpriseJITProvisionApproval[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  return requestItems<EnterpriseJITProvisionApproval>(suffix ? `/api/v1/enterprise/jit-provision-approvals?${suffix}` : "/api/v1/enterprise/jit-provision-approvals", token)
}

export async function reviewEnterpriseJITProvisionApproval(token: string | undefined, approvalID: string, payload: { tenant_id: string; decision: "approved" | "rejected"; reviewed_by?: string; reason?: string }): Promise<EnterpriseJITProvisionApproval> {
  return request<{ item: EnterpriseJITProvisionApproval }>(`/api/v1/enterprise/jit-provision-approvals/${encodePathSegment(approvalID)}/review`, { method: "POST", body: JSON.stringify(payload) }, token).then((r) => r.item)
}

export async function updateEnterpriseJITProvisionApprovalExternalSync(token: string | undefined, approvalID: string, payload: { tenant_id: string; status: "synced" | "failed"; external_sync_ref?: string; last_error?: string }): Promise<EnterpriseJITProvisionApproval> {
  return request<{ item: EnterpriseJITProvisionApproval }>(`/api/v1/enterprise/jit-provision-approvals/${encodePathSegment(approvalID)}/external-sync`, { method: "POST", body: JSON.stringify(payload) }, token).then((r) => r.item)
}

export async function syncEnterpriseEmployees(token: string | undefined, payload: { tenant_id: string; source: string; actor?: string; request_id?: string; connector_id?: string; raw_payload_ref?: string; employees: EmployeeSyncInput[] }): Promise<{ job: EnterpriseSyncJob; items: EnterpriseEmployee[]; access_sync: { created: number; updated: number; rejected: number } }> {
  return request("/api/v1/enterprise/employees/sync", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function listEnterpriseSyncJobs(token: string | undefined, tenantID?: string): Promise<EnterpriseSyncJob[]> {
  return requestItems<EnterpriseSyncJob>(withTenantQuery("/api/v1/enterprise/sync-jobs", tenantID), token)
}

export async function listEnterpriseSyncRequests(token: string | undefined, options?: { tenant_id?: string; limit?: number }): Promise<EnterpriseSyncRequestRecord[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  return requestItems<EnterpriseSyncRequestRecord>(suffix ? `/api/v1/enterprise/sync-requests?${suffix}` : "/api/v1/enterprise/sync-requests", token)
}

export async function reconcilePendingEnterpriseSyncRequests(token: string | undefined, payload: { tenant_id: string; limit?: number }): Promise<EnterpriseBatchPendingSyncReconcileResult> {
  return request<EnterpriseBatchPendingSyncReconcileResult>("/api/v1/enterprise/sync-requests/reconcile-pending", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function listEnterpriseSyncWorkerAlertSummary(token: string | undefined, options?: { tenant_id?: string; limit?: number }): Promise<EnterpriseSyncWorkerAlertSummaryItem[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  return requestItems<EnterpriseSyncWorkerAlertSummaryItem>(suffix ? `/api/v1/enterprise/sync-worker-alerts/summary?${suffix}` : "/api/v1/enterprise/sync-worker-alerts/summary", token)
}

export async function getEnterpriseSyncWorkerAlertSubscription(token: string | undefined, options: { tenant_id?: string }): Promise<EnterpriseSyncWorkerAlertSubscription> {
  return request<EnterpriseSyncWorkerAlertSubscription>(withTenantQuery("/api/v1/enterprise/sync-worker-alert-subscription", options.tenant_id), { method: "GET" }, token)
}

export async function upsertEnterpriseSyncWorkerAlertSubscription(token: string | undefined, payload: { tenant_id: string; enabled?: boolean; worker_alert_threshold?: number; window_seconds?: number; cooldown_seconds?: number; channels?: { email?: boolean; whatsapp?: boolean }; receiver_groups?: string[] }): Promise<EnterpriseSyncWorkerAlertSubscription> {
  return request<EnterpriseSyncWorkerAlertSubscription>("/api/v1/enterprise/sync-worker-alert-subscription", { method: "PUT", body: JSON.stringify(payload) }, token)
}

export async function dispatchEnterpriseSyncWorkerAlerts(token: string | undefined, payload: { tenant_id: string; actor?: string; worker_actions?: string[] }): Promise<EnterpriseSyncWorkerAlertDispatchResult> {
  return request<EnterpriseSyncWorkerAlertDispatchResult>("/api/v1/enterprise/sync-worker-alerts/dispatch", { method: "POST", body: JSON.stringify(payload) }, token)
}

export async function listEnterpriseSyncWorkerAlertNotifications(token: string | undefined, options?: { tenant_id?: string; status?: string; reason?: string; q?: string; retryable?: boolean; due_now?: boolean; offset?: number; limit?: number }): Promise<EnterpriseSyncWorkerAlertNotificationListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.reason?.trim()) query.set("reason", options.reason.trim())
  if (options?.q?.trim()) query.set("q", options.q.trim())
  if (typeof options?.retryable === "boolean") query.set("retryable", String(options.retryable))
  if (typeof options?.due_now === "boolean") query.set("due_now", String(options.due_now))
  if (typeof options?.offset === "number" && options.offset >= 0) query.set("offset", String(options.offset))
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/sync-worker-alerts/notifications?${suffix}` : "/api/v1/enterprise/sync-worker-alerts/notifications"
  const payload = await request<| (Partial<EnterpriseSyncWorkerAlertNotificationListResponse> & { items?: EnterpriseSyncWorkerAlertNotification[] | null }) | EnterpriseSyncWorkerAlertNotification[] | null>(path, { method: "GET" }, token)
  const normalizedList = normalizeOffsetListResponse<EnterpriseSyncWorkerAlertNotification>(payload)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  const filterCounts = normalizeWorkerAlertNotificationFilterCounts(payloadObject?.filter_counts, normalizedList.items, normalizedList.total)
  const statusCounts = normalizeWorkerAlertNotificationStatusCounts(payloadObject?.status_counts, normalizedList.items)
  return { ...(payloadObject ?? {}), ...normalizedList, filter_counts: filterCounts, status_counts: statusCounts }
}

export async function exportEnterpriseSyncWorkerAlertNotificationsCSV(token: string | undefined, options?: { tenant_id?: string; status?: string; reason?: string; q?: string; retryable?: boolean; due_now?: boolean }): Promise<string> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.reason?.trim()) query.set("reason", options.reason.trim())
  if (options?.q?.trim()) query.set("q", options.q.trim())
  if (typeof options?.retryable === "boolean") query.set("retryable", String(options.retryable))
  if (typeof options?.due_now === "boolean") query.set("due_now", String(options.due_now))
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/sync-worker-alerts/notifications/export-csv?${suffix}` : "/api/v1/enterprise/sync-worker-alerts/notifications/export-csv"
  return requestText(path, token)
}

export async function retryEnterpriseSyncWorkerAlertNotification(token: string | undefined, payload: { tenant_id: string; notification_id: string; actor?: string }): Promise<EnterpriseSyncWorkerAlertNotification> {
  const notificationID = payload.notification_id.trim()
  return request<EnterpriseSyncWorkerAlertNotification>(`/api/v1/enterprise/sync-worker-alerts/notifications/${encodeURIComponent(notificationID)}/retry`, { method: "POST", body: JSON.stringify({ tenant_id: payload.tenant_id, actor: payload.actor }) }, token)
}

export async function retryEnterpriseSyncWorkerAlertNotificationsBatch(token: string | undefined, payload: { tenant_id: string; notification_ids: string[]; actor?: string }): Promise<EnterpriseSyncWorkerAlertNotificationBatchRetryResult> {
  return request<EnterpriseSyncWorkerAlertNotificationBatchRetryResult>("/api/v1/enterprise/sync-worker-alerts/notifications/retry-batch", { method: "POST", body: JSON.stringify({ tenant_id: payload.tenant_id, notification_ids: payload.notification_ids, actor: payload.actor }) }, token)
}

export async function suppressEnterpriseSyncWorkerAlertNotificationsBatch(token: string | undefined, payload: { tenant_id: string; notification_ids: string[] }): Promise<EnterpriseSyncWorkerAlertNotificationBatchSuppressResult> {
  return request<EnterpriseSyncWorkerAlertNotificationBatchSuppressResult>("/api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch", { method: "POST", body: JSON.stringify({ tenant_id: payload.tenant_id, notification_ids: payload.notification_ids }) }, token)
}

export async function restoreEnterpriseSyncWorkerAlertNotificationsBatch(token: string | undefined, payload: { tenant_id: string; notification_ids: string[]; actor?: string }): Promise<EnterpriseSyncWorkerAlertNotificationBatchRestoreResult> {
  return request<EnterpriseSyncWorkerAlertNotificationBatchRestoreResult>("/api/v1/enterprise/sync-worker-alerts/notifications/restore-batch", { method: "POST", body: JSON.stringify({ tenant_id: payload.tenant_id, notification_ids: payload.notification_ids, actor: payload.actor }) }, token)
}

export async function autoRetryEnterpriseSyncWorkerAlertNotifications(token: string | undefined, payload: { tenant_id: string; actor?: string; limit?: number; max_attempts?: number; base_backoff_ms?: number; max_backoff_ms?: number }): Promise<EnterpriseSyncWorkerAlertNotificationAutoRetryResult> {
  return request<EnterpriseSyncWorkerAlertNotificationAutoRetryResult>("/api/v1/enterprise/sync-worker-alerts/notifications/auto-retry", { method: "POST", body: JSON.stringify({ tenant_id: payload.tenant_id, actor: payload.actor, limit: payload.limit, max_attempts: payload.max_attempts, base_backoff_ms: payload.base_backoff_ms, max_backoff_ms: payload.max_backoff_ms }) }, token)
}

export async function listEnterpriseSyncWorkerAlerts(token: string | undefined, options?: { tenant_id?: string; limit?: number }): Promise<EnterpriseSyncWorkerAlertItem[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  return requestItems<EnterpriseSyncWorkerAlertItem>(suffix ? `/api/v1/enterprise/sync-worker-alerts?${suffix}` : "/api/v1/enterprise/sync-worker-alerts", token)
}

export async function listEnterpriseHRISWebhookDLQ(token: string | undefined, options?: { tenant_id?: string; connector_id?: string; offset?: number; limit?: number }): Promise<EnterpriseHRISWebhookDLQListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.connector_id?.trim()) query.set("connector_id", options.connector_id.trim())
  if (typeof options?.offset === "number" && options.offset >= 0) query.set("offset", String(Math.floor(options.offset)))
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/hris-webhook-dlq?${suffix}` : "/api/v1/enterprise/hris-webhook-dlq"
  const payload = await request<EnterpriseHRISWebhookDLQListResponse | { items?: EnterpriseHRISWebhookDLQEntry[] | null } | EnterpriseHRISWebhookDLQEntry[] | null>(path, { method: "GET" }, token)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  return { ...(payloadObject ?? {}), ...normalizeOffsetListResponse(payload) }
}

export async function listEnterpriseHRISWebhookReceipts(token: string | undefined, options?: { tenant_id?: string; connector_id?: string; offset?: number; limit?: number }): Promise<EnterpriseHRISWebhookReceiptListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.connector_id?.trim()) query.set("connector_id", options.connector_id.trim())
  if (typeof options?.offset === "number" && options.offset >= 0) query.set("offset", String(Math.floor(options.offset)))
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/hris-webhook-receipts?${suffix}` : "/api/v1/enterprise/hris-webhook-receipts"
  const payload = await request<EnterpriseHRISWebhookReceiptListResponse | { items?: EnterpriseHRISWebhookReceipt[] | null } | EnterpriseHRISWebhookReceipt[] | null>(path, { method: "GET" }, token)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  return { ...(payloadObject ?? {}), ...normalizeOffsetListResponse(payload) }
}

export async function listEnterpriseHRISWebhookExecutions(token: string | undefined, options?: { tenant_id?: string; connector_id?: string; kind?: "receipt_process" | "dlq_replay"; status?: "queued" | "running" | "succeeded" | "failed"; queue_state?: "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"; replay_scope?: "replayed" | "worker_required"; execution_mode?: "inline" | "queued"; dispatch_mode?: "worker_tick" | "worker_task_channel" | "goroutine_fallback"; target_status?: string; target_id?: string; q?: string; offset?: number; limit?: number }): Promise<EnterpriseHRISWebhookExecutionListResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.connector_id?.trim()) query.set("connector_id", options.connector_id.trim())
  if (options?.kind?.trim()) query.set("kind", options.kind.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.queue_state?.trim()) query.set("queue_state", options.queue_state.trim())
  if (options?.replay_scope?.trim()) query.set("replay_scope", options.replay_scope.trim())
  if (options?.execution_mode?.trim()) query.set("execution_mode", options.execution_mode.trim())
  if (options?.dispatch_mode?.trim()) query.set("dispatch_mode", options.dispatch_mode.trim())
  if (options?.target_status?.trim()) query.set("target_status", options.target_status.trim())
  if (options?.target_id?.trim()) query.set("target_id", options.target_id.trim())
  if (options?.q?.trim()) query.set("q", options.q.trim())
  if (typeof options?.offset === "number" && options.offset >= 0) query.set("offset", String(Math.floor(options.offset)))
  if (typeof options?.limit === "number" && options.limit > 0) query.set("limit", String(options.limit))
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/hris-webhook-executions?${suffix}` : "/api/v1/enterprise/hris-webhook-executions"
  const payload = await request<EnterpriseHRISWebhookExecutionListResponse | { items?: EnterpriseHRISWebhookExecution[] | null } | EnterpriseHRISWebhookExecution[] | null>(path, { method: "GET" }, token)
  const payloadObject = payload && !Array.isArray(payload) ? payload : null
  const normalizedList = normalizeOffsetListResponse<EnterpriseHRISWebhookExecution>(payload)
  const statusCounts = normalizeHRISWebhookExecutionStatusCounts(payloadObject && "status_counts" in payloadObject ? payloadObject.status_counts : null, normalizedList.items, normalizedList.total)
  return { ...(payloadObject ?? {}), ...normalizedList, status_counts: statusCounts }
}

export async function getEnterpriseHRISWebhookExecution(token: string | undefined, executionID: string, options: { tenant_id: string }): Promise<EnterpriseHRISWebhookExecutionDetailResponse> {
  const nextExecutionID = executionID.trim()
  if (!nextExecutionID) throw new Error("execution_id is required")
  const query = new URLSearchParams()
  if (options.tenant_id.trim()) query.set("tenant_id", options.tenant_id.trim())
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/hris-webhook-executions/${encodeURIComponent(nextExecutionID)}?${suffix}` : `/api/v1/enterprise/hris-webhook-executions/${encodeURIComponent(nextExecutionID)}`
  return request<EnterpriseHRISWebhookExecutionDetailResponse>(path, { method: "GET" }, token)
}

export async function replayEnterpriseHRISWebhookExecution(token: string | undefined, input: { tenant_id: string; execution_id: string; execution_mode?: "inline" | "queued"; require_worker?: boolean }): Promise<EnterpriseHRISWebhookExecutionReplayResponse> {
  const executionID = input.execution_id.trim()
  if (!executionID) throw new Error("execution_id is required")
  return request<EnterpriseHRISWebhookExecutionReplayResponse>(`/api/v1/enterprise/hris-webhook-executions/${encodeURIComponent(executionID)}/replay`, { method: "POST", body: JSON.stringify({ tenant_id: input.tenant_id, execution_mode: input.execution_mode, require_worker: input.require_worker }) }, token)
}

export async function processBatchEnterpriseHRISWebhookReceipts(token: string | undefined, input: { tenant_id: string; receipt_ids: string[]; execution_mode?: "inline" | "queued"; require_worker?: boolean }): Promise<EnterpriseHRISWebhookReceiptBatchProcessResult> {
  return request<EnterpriseHRISWebhookReceiptBatchProcessResult>("/api/v1/enterprise/hris-webhook-receipts/process-batch", { method: "POST", body: JSON.stringify(input) }, token)
}

export async function processEnterpriseHRISWebhookReceipt(token: string | undefined, input: { tenant_id: string; receipt_id: string; execution_mode?: "inline" | "queued"; require_worker?: boolean }): Promise<EnterpriseHRISWebhookReceiptProcessResponse> {
  return request<EnterpriseHRISWebhookReceiptProcessResponse>(`/api/v1/enterprise/hris-webhook-receipts/${encodePathSegment(input.receipt_id)}/process`, { method: "POST", body: JSON.stringify({ tenant_id: input.tenant_id, execution_mode: input.execution_mode, require_worker: input.require_worker }) }, token)
}

export async function listEnterpriseHRISPullStates(token: string | undefined, tenantID?: string): Promise<EnterpriseHRISPullState[]> {
  return requestItems<EnterpriseHRISPullState>(withTenantQuery("/api/v1/enterprise/hris-pull-states", tenantID), token)
}

export async function replayEnterpriseHRISWebhookDLQ(token: string | undefined, entryID: string, input?: { execution_mode?: "inline" | "queued"; require_worker?: boolean }): Promise<EnterpriseHRISWebhookDLQReplayResponse> {
  const nextEntryID = entryID.trim()
  const query = new URLSearchParams()
  if (input?.execution_mode?.trim()) query.set("execution_mode", input.execution_mode.trim())
  if (typeof input?.require_worker === "boolean") query.set("require_worker", String(input.require_worker))
  const suffix = query.toString()
  const path = suffix ? `/api/v1/enterprise/hris-webhook-dlq/${encodePathSegment(nextEntryID)}/replay?${suffix}` : `/api/v1/enterprise/hris-webhook-dlq/${encodePathSegment(nextEntryID)}/replay`
  return request<EnterpriseHRISWebhookDLQReplayResponse>(path, { method: "POST" }, token)
}

export async function replayBatchEnterpriseHRISWebhookDLQ(token: string | undefined, input: { tenant_id: string; entry_ids: string[]; execution_mode?: "inline" | "queued"; require_worker?: boolean }): Promise<EnterpriseHRISWebhookDLQBatchReplayResult> {
  return request<EnterpriseHRISWebhookDLQBatchReplayResult>("/api/v1/enterprise/hris-webhook-dlq/replay-batch", { method: "POST", body: JSON.stringify(input) }, token)
}
