import { useEffect, useMemo, useState } from "react"
import { Link, useLocation, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { EnterpriseAlertsWorkspace } from "@/components/enterprise/enterprise-alerts-workspace"
import { EnterpriseEmployeesWorkspace } from "@/components/enterprise/enterprise-employees-workspace"
import { EnterprisePageHeader } from "@/components/enterprise/enterprise-page-header"
import { EnterpriseIDPWorkspace } from "@/components/enterprise/enterprise-idp-workspace"
import { EnterpriseSCIMWorkspace } from "@/components/enterprise/enterprise-scim-workspace"
import { EnterprisePageOverview } from "@/components/enterprise/enterprise-page-overview"
import { EnterpriseSyncWorkspace } from "@/components/enterprise/enterprise-sync-workspace"
import { type EnterpriseSyncWorkerAlertSubscriptionSaveInput } from "@/components/enterprise/enterprise-sync-worker-alert-subscription-card"
import { EnterpriseWorkflowOverview, type EnterpriseWorkflowOverviewStep } from "@/components/enterprise/enterprise-workflow-overview"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  API_BASE_URL,
  autoRetryEnterpriseSyncWorkerAlertNotifications,
  createEnterpriseHRISConnector,
  dispatchEnterpriseSyncWorkerAlerts,
  exportEnterpriseSyncWorkerAlertNotificationsCSV,
  getEnterpriseHRISWebhookExecution,
  getEnterpriseSyncWorkerAlertSubscription,
  getEnterpriseIDPConfig,
  listAccessPolicies,
  listEnterpriseEmployees,
  listEnterpriseHRISConnectors,
  listEnterpriseHRISPullStates,
  listEnterpriseHRISSecrets,
  listEnterpriseHRISWebhookExecutions,
  listEnterpriseHRISWebhookReceipts,
  listEnterpriseHRISWebhookDLQ,
  listEnterpriseJITProvisionApprovals,
  listEnterpriseSyncRequests,
  listEnterpriseSyncJobs,
  listEnterpriseSyncWorkerAlerts,
  listEnterpriseSyncWorkerAlertNotifications,
  listEnterpriseSyncWorkerAlertSummary,
  listUserGroups,
  listTenants,
  listWalletPasses,
  processEnterpriseHRISWebhookReceipt,
  processBatchEnterpriseHRISWebhookReceipts,
  reconcilePendingEnterpriseSyncRequests,
  replayBatchEnterpriseHRISWebhookDLQ,
  replayEnterpriseHRISWebhookExecution,
  replayEnterpriseHRISWebhookDLQ,
  retryEnterpriseSyncWorkerAlertNotification,
  retryEnterpriseSyncWorkerAlertNotificationsBatch,
  restoreEnterpriseSyncWorkerAlertNotificationsBatch,
  suppressEnterpriseSyncWorkerAlertNotificationsBatch,
  reviewEnterpriseJITProvisionApproval,
  syncEnterpriseEmployees,
  updateEnterpriseJITProvisionApprovalExternalSync,
  updateEnterpriseHRISConnector,
  upsertEnterpriseSyncWorkerAlertSubscription,
  type CurrentUser,
  type AccessPolicy,
  type EmployeeSyncInput,
  type EnterpriseEmployee,
  type EnterpriseHRISConnector,
  type EnterpriseHRISWebhookExecution,
  type EnterpriseHRISWebhookExecutionStatusCounts,
  type EnterpriseHRISPullState,
  type EnterpriseHRISSecret,
  type EnterpriseHRISWebhookDLQBatchReplayResult,
  type EnterpriseHRISWebhookReceipt,
  type EnterpriseHRISWebhookReceiptBatchProcessResult,
  type EnterpriseHRISWebhookDLQEntry,
  type EnterpriseHRISWebhookRuntimeCounts,
  type EnterpriseIDPConfig,
  type EnterpriseJITProvisionApproval,
  type EnterpriseSyncRequestRecord,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertNotificationFilterCounts,
  type EnterpriseSyncWorkerAlertNotification,
  type EnterpriseSyncWorkerAlertNotificationStatusCounts,
  type EnterpriseSyncWorkerAlertSubscription,
  type EnterpriseSyncWorkerAlertItem,
  type EnterpriseSyncWorkerAlertSummaryItem,
  type Tenant,
  type UserGroup,
  type WalletPassInstance,
} from "@/lib/api"
import { canManageEnterprise, getViewerTenantID, isPlatformViewer } from "@/lib/viewer"
import { type TalentaConnectorSaveInput } from "@/components/enterprise/enterprise-hris-connector-panel"

type EnterprisePageProps = {
  token: string
  viewer: CurrentUser
}

type EnterpriseSection = "employees" | "sync" | "idp" | "scim" | "alerts"
type AlertLandingView = "overview" | "approval_backlog" | "directory_exceptions"
type AlertSegmentHint = "receipt_recovery"
type AlertSegmentStatusHint = "pending" | "attention" | "ready"
type SyncFocusHint = "worker_alert"
type WorkerReviewStatusHint = "handled"
type WorkerReviewStageHint = "alerts" | "directory" | "policies" | "issuance" | "sync"
type EnterpriseWorkflowState = "completed" | "pending" | "blocked"
type NotificationHistoryFilter = "all" | "failed" | "retryable" | "suppressed" | "due_now"
type HRISWebhookExecutionKindFilter = "all" | "receipt_process" | "dlq_replay"
type HRISWebhookExecutionStatusFilter = "all" | "queued" | "running" | "succeeded" | "failed"
type HRISWebhookExecutionReplayScopeFilter = "all" | "replayed" | "worker_required"
type HRISWebhookExecutionModeFilter = "all" | "inline" | "queued"
type HRISWebhookExecutionDispatchFilter = "all" | "worker_tick" | "worker_task_channel" | "goroutine_fallback"

type SyncSourceOption = {
  value: string
  label: string
  description: string
}

function buildSyncSourceOptions(t: TFunction): SyncSourceOption[] {
  return [
    {
      value: "hris",
      label: t("enterprisePage.syncSources.hris.label"),
      description: t("enterprisePage.syncSources.hris.description"),
    },
    {
      value: "scim",
      label: t("enterprisePage.syncSources.scim.label"),
      description: t("enterprisePage.syncSources.scim.description"),
    },
    {
      value: "csv_import",
      label: t("enterprisePage.syncSources.csvImport.label"),
      description: t("enterprisePage.syncSources.csvImport.description"),
    },
    {
      value: "manual_sync",
      label: t("enterprisePage.syncSources.manual.label"),
      description: t("enterprisePage.syncSources.manual.description"),
    },
  ]
}

const sampleSyncPayload = `[
  {
    "external_id": "emp-001",
    "email": "employee@company.local",
    "full_name": "Alex Chen",
    "department": "Finance",
    "job_title": "Finance Manager",
    "location": "HQ / 18F",
    "status": "active"
  }
]`

function buildSyncSourceCheckpointsBySource(t: TFunction): Record<string, string[]> {
  return {
    hris: [
      t("enterprisePage.syncCheckpoints.hris.fieldMapping"),
      t("enterprisePage.syncCheckpoints.hris.primaryKeyEmail"),
    ],
    scim: [
      t("enterprisePage.syncCheckpoints.scim.externalIdEmail"),
      t("enterprisePage.syncCheckpoints.scim.deactivationSemantics"),
    ],
    csv_import: [
      t("enterprisePage.syncCheckpoints.csvImport.headersAndRequired"),
      t("enterprisePage.syncCheckpoints.csvImport.duplicateHandling"),
    ],
    manual_sync: [
      t("enterprisePage.syncCheckpoints.manual.requiredProfileFields"),
      t("enterprisePage.syncCheckpoints.manual.backflowToStableGroups"),
    ],
  }
}

function formatDateTime(value?: string, locale?: string) {
  if (!value) {
    return "-"
  }
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return value
  }
  return timestamp.toLocaleString(locale || undefined)
}

function syncSourceLabel(source: string | undefined, syncSourceOptions: SyncSourceOption[]) {
  if (!source) {
    return "-"
  }
  return syncSourceOptions.find((item) => item.value === source)?.label || source
}

function buildEnterpriseFlowLink(
  path: string,
  tenantID: string,
  stage: "directory" | "policies" | "issuance",
  hints?: Record<string, string>
) {
  const [pathname, rawQuery = ""] = path.split("?")
  const params = new URLSearchParams(rawQuery)
  params.set("from", "enterprise")
  params.set("flow", "sync_to_access")
  params.set("stage", stage)
  if (tenantID.trim()) {
    params.set("tenant_id", tenantID.trim())
  }
  if (hints) {
    Object.entries(hints).forEach(([key, value]) => {
      if (!key.trim() || !value.trim()) {
        return
      }
      params.set(key.trim(), value.trim())
    })
  }
  const nextQuery = params.toString()
  return nextQuery ? `${pathname}?${nextQuery}` : pathname
}

function syncSourceFailureHint(t: TFunction, source: string, rejected: number) {
  if (source === "scim") {
    return t("enterprisePage.syncResult.failure.scim", { rejected })
  }
  if (source === "csv_import") {
    return t("enterprisePage.syncResult.failure.csvImport", { rejected })
  }
  if (source === "manual_sync") {
    return t("enterprisePage.syncResult.failure.manual", { rejected })
  }
  return t("enterprisePage.syncResult.failure.hris", { rejected })
}

function syncSourceSuccessHint(t: TFunction, source: string, deactivated: number) {
  if (source === "scim" && deactivated > 0) {
    return t("enterprisePage.syncResult.success.scimDeactivated", { deactivated })
  }
  if (source === "csv_import") {
    return t("enterprisePage.syncResult.success.csvImport")
  }
  if (source === "manual_sync") {
    return t("enterprisePage.syncResult.success.manual")
  }
  return t("enterprisePage.syncResult.success.default")
}

function defaultEnterpriseSyncWorkerAlertSubscription(tenantID: string): EnterpriseSyncWorkerAlertSubscription {
  return {
    tenant_id: tenantID.trim(),
    enabled: true,
    worker_alert_threshold: 3,
    window_seconds: 900,
    cooldown_seconds: 900,
    channels: {
      email: true,
      whatsapp: false,
    },
    receiver_groups: ["security"],
    updated_at: new Date().toISOString(),
  }
}

function statusBadgeVariant(status?: string) {
  switch (status) {
    case "active":
    case "approved":
    case "completed":
      return "outline"
    case "rejected":
    case "failed":
      return "destructive"
    default:
      return "secondary"
  }
}

function formatDispatchModeLabel(value?: string) {
  const normalized = value?.trim()
  if (!normalized) {
    return "-"
  }
  return normalized
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((token) => token.charAt(0).toUpperCase() + token.slice(1))
    .join(" ")
}

function normalizeWebhookQueueActionError(message: string, t: TFunction) {
  const normalized = message.trim().toLowerCase()
  if (!normalized) {
    return message
  }
  if (normalized.includes("require_worker=true requires enabled receipt worker")) {
    return t("enterprisePage.errors.webhookReceiptWorkerRequired")
  }
  if (normalized.includes("require_worker=true requires enabled dlq worker")) {
    return t("enterprisePage.errors.webhookDLQWorkerRequired")
  }
  return message
}

function workflowStateVariant(state: EnterpriseWorkflowState) {
  switch (state) {
    case "completed":
      return "outline"
    case "blocked":
      return "secondary"
    default:
      return "secondary"
  }
}

function workflowStateLabel(t: TFunction, state: EnterpriseWorkflowState) {
  switch (state) {
    case "completed":
      return t("enterprisePage.workflow.state.completed")
    case "blocked":
      return t("enterprisePage.workflow.state.blocked")
    default:
      return t("enterprisePage.workflow.state.pending")
  }
}

function getEnterpriseSectionsForViewer(viewer: CurrentUser): EnterpriseSection[] {
  if (isPlatformViewer(viewer)) {
    return ["sync", "alerts", "employees", "idp", "scim"]
  }
  return ["employees", "idp", "scim", "sync", "alerts"]
}

function resolveEnterpriseSection(value: string | undefined, fallback: EnterpriseSection): EnterpriseSection {
  switch (value) {
    case "employees":
    case "sync":
    case "idp":
    case "scim":
    case "alerts":
      return value
    default:
      return fallback
  }
}

function enterpriseSectionLabel(section: EnterpriseSection, t: TFunction) {
  switch (section) {
    case "employees":
      return t("enterprisePage.labels.employees")
    case "sync":
      return t("enterprisePage.labels.sync")
    case "idp":
      return t("enterprisePage.labels.idp")
    case "scim":
      return "SCIM"
    case "alerts":
      return t("enterprisePage.labels.alerts")
  }
}

type EnterprisePageData = {
  employees: EnterpriseEmployee[]
  hrisConnectors: EnterpriseHRISConnector[]
  hrisSecrets: EnterpriseHRISSecret[]
  syncJobs: EnterpriseSyncJob[]
  syncRequests: EnterpriseSyncRequestRecord[]
  workerAlertSubscription: EnterpriseSyncWorkerAlertSubscription
  approvals: EnterpriseJITProvisionApproval[]
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
  workerAlertEvents: EnterpriseSyncWorkerAlertItem[]
  hrisPullStates: EnterpriseHRISPullState[]
  idpConfig: EnterpriseIDPConfig | null
  userGroups: UserGroup[]
  policies: AccessPolicy[]
  issuedPasses: WalletPassInstance[]
}

const workerAlertNotificationPageSize = 20
const hrisWebhookReceiptPageSize = 12
const hrisWebhookDLQPageSize = 12
const hrisWebhookExecutionPageSize = 12

const emptyWorkerAlertNotificationFilterCounts: EnterpriseSyncWorkerAlertNotificationFilterCounts = {
  all: 0,
  failed: 0,
  retryable: 0,
  suppressed: 0,
  due_now: 0,
}

const emptyWorkerAlertNotificationStatusCounts: EnterpriseSyncWorkerAlertNotificationStatusCounts = {
  sent: 0,
  failed: 0,
  skipped: 0,
}

const emptyHRISWebhookRuntimeCounts: EnterpriseHRISWebhookRuntimeCounts = {
  all: 0,
  ready: 0,
  cooldown: 0,
  in_flight: 0,
  attempt_limit: 0,
  terminal: 0,
}

const emptyHRISWebhookExecutionStatusCounts: EnterpriseHRISWebhookExecutionStatusCounts = {
  all: 0,
  queued: 0,
  running: 0,
  succeeded: 0,
  failed: 0,
}

type HRISWebhookExecutionQueueStateFilter = "all" | "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"

function buildWorkerAlertNotificationListOptions(args: {
  tenantID: string
  filter: NotificationHistoryFilter
  query: string
  limit: number
  offset?: number
}) {
  const options: Parameters<typeof listEnterpriseSyncWorkerAlertNotifications>[1] = {
    tenant_id: args.tenantID,
    q: args.query.trim() || undefined,
    limit: args.limit,
    offset: args.offset ?? 0,
  }
  switch (args.filter) {
    case "failed":
      options.status = "failed"
      break
    case "retryable":
      options.retryable = true
      break
    case "suppressed":
      options.status = "skipped"
      options.reason = "manual_suppressed"
      break
    case "due_now":
      options.due_now = true
      break
  }
  return options
}

function buildHRISWebhookExecutionListOptions(args: {
  tenantID: string
  kind: HRISWebhookExecutionKindFilter
  status: HRISWebhookExecutionStatusFilter
  queueState: HRISWebhookExecutionQueueStateFilter
  replayScope: HRISWebhookExecutionReplayScopeFilter
  executionMode: HRISWebhookExecutionModeFilter
  dispatchMode: HRISWebhookExecutionDispatchFilter
  targetStatus: string
  query: string
  limit: number
  offset?: number
}) {
  const options: Parameters<typeof listEnterpriseHRISWebhookExecutions>[1] = {
    tenant_id: args.tenantID,
    q: args.query.trim() || undefined,
    limit: args.limit,
    offset: args.offset ?? 0,
  }
  if (args.kind !== "all") {
    options.kind = args.kind
  }
  if (args.status !== "all") {
    options.status = args.status
  }
  if (args.queueState !== "all") {
    options.queue_state = args.queueState
  }
  if (args.replayScope !== "all") {
    options.replay_scope = args.replayScope
  }
  if (args.executionMode !== "all") {
    options.execution_mode = args.executionMode
  }
  if (args.dispatchMode !== "all") {
    options.dispatch_mode = args.dispatchMode
  }
  if (args.targetStatus.trim()) {
    options.target_status = args.targetStatus.trim().toLowerCase()
  }
  return options
}

function normalizeHRISWebhookRuntimeCounts(value: unknown): EnterpriseHRISWebhookRuntimeCounts | null {
  if (!value || typeof value !== "object") {
    return null
  }
  const source = value as Record<string, unknown>
  let hasNumericCount = false
  const result = { ...emptyHRISWebhookRuntimeCounts }
  ;(Object.keys(result) as Array<keyof EnterpriseHRISWebhookRuntimeCounts>).forEach((key) => {
    const count = source[key]
    if (typeof count === "number" && Number.isFinite(count) && count >= 0) {
      result[key] = Math.floor(count)
      hasNumericCount = true
    }
  })
  return hasNumericCount ? result : null
}

function extractHRISWebhookRuntimeCounts(...sources: unknown[]): EnterpriseHRISWebhookRuntimeCounts | null {
  for (const source of sources) {
    const counts = normalizeHRISWebhookRuntimeCounts(source)
    if (counts) {
      return counts
    }
  }
  return null
}

function downloadWorkerAlertNotificationCSV(fileName: string, csvContent: string) {
  const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8" })
  const url = window.URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = fileName
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.URL.revokeObjectURL(url)
}

async function loadEnterprisePageData(token: string, tenantID: string): Promise<EnterprisePageData> {
  const [
    employeeItems,
    connectorItems,
    secretItems,
    syncJobItems,
    syncRequestItems,
    workerAlertSubscription,
    approvalItems,
    workerAlertItems,
    workerAlertEventItems,
    hrisPullStateItems,
    groupItems,
    policyItems,
    passItems,
  ] = await Promise.all([
    listEnterpriseEmployees(token, tenantID),
    listEnterpriseHRISConnectors(token, tenantID),
    listEnterpriseHRISSecrets(token, tenantID),
    listEnterpriseSyncJobs(token, tenantID),
    listEnterpriseSyncRequests(token, {
      tenant_id: tenantID,
      limit: 50,
    }),
    getEnterpriseSyncWorkerAlertSubscription(token, {
      tenant_id: tenantID,
    }).catch(() => defaultEnterpriseSyncWorkerAlertSubscription(tenantID)),
    listEnterpriseJITProvisionApprovals(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseSyncWorkerAlertSummary(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseSyncWorkerAlerts(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseHRISPullStates(token, tenantID),
    listUserGroups(token),
    listAccessPolicies(token),
    listWalletPasses(token, tenantID),
  ])

  let nextIDPConfig: EnterpriseIDPConfig | null = null
  try {
    nextIDPConfig = await getEnterpriseIDPConfig(token, tenantID)
  } catch (err) {
    const message = err instanceof Error ? err.message : ""
    if (!message.includes("404") && !message.toLowerCase().includes("not found")) {
      throw err
    }
  }

  return {
    employees: employeeItems,
    hrisConnectors: connectorItems,
    hrisSecrets: secretItems,
    syncJobs: syncJobItems,
    syncRequests: syncRequestItems,
    workerAlertSubscription,
    approvals: approvalItems,
    workerAlerts: workerAlertItems,
    workerAlertEvents: workerAlertEventItems,
    hrisPullStates: hrisPullStateItems,
    idpConfig: nextIDPConfig,
    userGroups: groupItems,
    policies: policyItems,
    issuedPasses: passItems,
  }
}

export function EnterprisePage({ token, viewer }: EnterprisePageProps) {
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const platformViewer = isPlatformViewer(viewer)
  const writable = canManageEnterprise(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const visibleSections = useMemo(() => getEnterpriseSectionsForViewer(viewer), [viewer])
  const defaultSection = visibleSections[0] ?? "employees"
  const requestedSection = resolveEnterpriseSection(location.hash.replace(/^#/, ""), defaultSection)
  const activeSection = visibleSections.includes(requestedSection) ? requestedSection : defaultSection
  const syncSourceOptions = useMemo(() => buildSyncSourceOptions(t), [t])
  const syncSourceCheckpointsBySource = useMemo(() => buildSyncSourceCheckpointsBySource(t), [t])
  const formatDateTimeValue = (value?: string) => formatDateTime(value, i18n.language)
  const syncSourceLabelValue = (source?: string) => syncSourceLabel(source, syncSourceOptions)
  const enterpriseRouteHints = useMemo(() => {
    const query = new URLSearchParams(location.search)
    return {
      executionID: query.get("execution_id")?.trim() || "",
      tenantID: query.get("tenant_id")?.trim() || "",
    }
  }, [location.search])
  const alertsRouteHints = useMemo(() => {
    const query = new URLSearchParams(location.search)
    const rawLandingView = query.get("alerts_view_hint")?.trim() || ""
    const landingView: AlertLandingView | undefined =
      rawLandingView === "overview" || rawLandingView === "approval_backlog" || rawLandingView === "directory_exceptions"
        ? (rawLandingView as AlertLandingView)
        : undefined
    const rawSegmentHint = query.get("segment_hint")?.trim() || ""
    const segmentHint: AlertSegmentHint | undefined = rawSegmentHint === "receipt_recovery" ? "receipt_recovery" : undefined
    const rawSegmentStatus = query.get("segment_status_hint")?.trim() || ""
    const segmentStatus: AlertSegmentStatusHint | undefined = ["pending", "attention", "ready"].includes(rawSegmentStatus)
      ? (rawSegmentStatus as AlertSegmentStatusHint)
      : undefined
    const rawSyncStatus = query.get("sync_status_hint")?.trim() || ""
    const syncStatus:
      | "all"
      | "attention"
      | "rejected"
      | "deactivated"
      | "healthy"
      | undefined = ["all", "attention", "rejected", "deactivated", "healthy"].includes(rawSyncStatus)
      ? (rawSyncStatus as "all" | "attention" | "rejected" | "deactivated" | "healthy")
      : undefined
    const syncSource = query.get("sync_source_hint")?.trim() || ""
    const rawWorkerFilter = query.get("worker_filter_hint")?.trim() || ""
    const workerFilter: "all" | "alerting" | "hot" | "stable" | undefined = ["all", "alerting", "hot", "stable"].includes(
      rawWorkerFilter
    )
      ? (rawWorkerFilter as "all" | "alerting" | "hot" | "stable")
      : undefined
    const workerAction = query.get("worker_action")?.trim() || ""
    const workerLabel = query.get("worker_alert_label")?.trim() || ""
    const workerKind = query.get("worker_kind")?.trim() || ""
    const workerQueueState = query.get("worker_queue_state")?.trim() || ""
    const workerReplayState = query.get("worker_replay_state")?.trim() || ""
    const workerStatus = query.get("worker_status")?.trim() || ""
    const rawExecutionKind = query.get("execution_kind")?.trim() || ""
    const executionKind: HRISWebhookExecutionKindFilter | undefined = ["receipt_process", "dlq_replay"].includes(
      rawExecutionKind
    )
      ? (rawExecutionKind as HRISWebhookExecutionKindFilter)
      : undefined
    const rawExecutionStatus = query.get("execution_status")?.trim() || ""
    const executionStatus: HRISWebhookExecutionStatusFilter | undefined = ["queued", "running", "succeeded", "failed"].includes(
      rawExecutionStatus
    )
      ? (rawExecutionStatus as HRISWebhookExecutionStatusFilter)
      : undefined
    const rawExecutionQueueState = query.get("execution_queue_state")?.trim() || ""
    const executionQueueState: HRISWebhookExecutionQueueStateFilter | undefined = [
      "ready",
      "cooldown",
      "in_flight",
      "attempt_limit",
      "terminal",
    ].includes(rawExecutionQueueState)
      ? (rawExecutionQueueState as HRISWebhookExecutionQueueStateFilter)
      : undefined
    const rawExecutionReplayScope = query.get("execution_replay_scope")?.trim() || ""
    const executionReplayScope: HRISWebhookExecutionReplayScopeFilter | undefined = [
      "replayed",
      "worker_required",
    ].includes(rawExecutionReplayScope)
      ? (rawExecutionReplayScope as HRISWebhookExecutionReplayScopeFilter)
      : undefined
    const rawExecutionMode = query.get("execution_mode")?.trim() || ""
    const executionMode: HRISWebhookExecutionModeFilter | undefined = ["inline", "queued"].includes(rawExecutionMode)
      ? (rawExecutionMode as HRISWebhookExecutionModeFilter)
      : undefined
    const approvalQuery =
      query.get("approval_query_hint")?.trim() ||
      query.get("target_hint")?.trim() ||
      query.get("target_email")?.trim() ||
      query.get("target_id")?.trim() ||
      ""
    const directoryQuery = query.get("worker_query_hint")?.trim() || query.get("sync_query_hint")?.trim() || ""
    const hasHints =
      activeSection === "alerts" &&
      Boolean(
        landingView ||
          segmentHint ||
          segmentStatus ||
          syncStatus ||
          syncSource ||
          workerFilter ||
          workerAction ||
          workerLabel ||
          workerKind ||
          workerQueueState ||
          workerReplayState ||
          workerStatus ||
          executionKind ||
          executionStatus ||
          executionQueueState ||
          executionReplayScope ||
          executionMode ||
          approvalQuery ||
          directoryQuery
      )
    return {
      hasHints,
      contextKey: hasHints ? `${location.search}::${activeSection}` : "",
      approvalQuery: approvalQuery || undefined,
      directoryQuery: directoryQuery || undefined,
      landingView,
      segmentHint,
      segmentStatus,
      workerAction: workerAction || undefined,
      workerFilter,
      workerKind: workerKind || undefined,
      workerLabel: workerLabel || undefined,
      workerQueueState: workerQueueState || undefined,
      workerReplayState: workerReplayState || undefined,
      workerStatus: workerStatus || undefined,
      executionKind,
      executionMode,
      executionQueueState,
      executionReplayScope,
      executionStatus,
      syncSource: syncSource || undefined,
      syncStatus,
    }
  }, [activeSection, location.search])
  const syncRouteHints = useMemo(() => {
    const query = new URLSearchParams(location.search)
    const rawFocusHint = query.get("sync_focus_hint")?.trim() || ""
    const focusHint: SyncFocusHint | undefined = rawFocusHint === "worker_alert" ? "worker_alert" : undefined
    const rawWorkerReviewStatus = query.get("worker_review_status_hint")?.trim() || ""
    const workerReviewStatus: WorkerReviewStatusHint | undefined =
      rawWorkerReviewStatus === "handled" ? "handled" : undefined
    const rawWorkerReviewStage = query.get("worker_review_stage_hint")?.trim() || ""
    const workerReviewStage: WorkerReviewStageHint | undefined = [
      "alerts",
      "directory",
      "policies",
      "issuance",
      "sync",
    ].includes(rawWorkerReviewStage)
      ? (rawWorkerReviewStage as WorkerReviewStageHint)
      : undefined
    const rawWorkerFilter = query.get("worker_filter_hint")?.trim() || ""
    const workerFilter: "all" | "alerting" | "hot" | "stable" | undefined = ["all", "alerting", "hot", "stable"].includes(
      rawWorkerFilter
    )
      ? (rawWorkerFilter as "all" | "alerting" | "hot" | "stable")
      : undefined
    const workerAction = query.get("worker_action")?.trim() || ""
    const workerLabel = query.get("worker_alert_label")?.trim() || ""
    const workerKind = query.get("worker_kind")?.trim() || ""
    const workerConnectorID = query.get("worker_connector_id")?.trim() || ""
    const workerFailureStage = query.get("worker_failure_stage")?.trim() || ""
    const workerMode = query.get("worker_mode")?.trim() || ""
    const workerQueueState = query.get("worker_queue_state")?.trim() || ""
    const workerQuery = query.get("worker_query_hint")?.trim() || ""
    const workerReplayState = query.get("worker_replay_state")?.trim() || ""
    const workerRequestID = query.get("worker_request_id")?.trim() || ""
    const workerStatus = query.get("worker_status")?.trim() || ""
    const workerVendor = query.get("worker_vendor")?.trim() || ""
    const executionID = query.get("execution_id")?.trim() || ""
    const rawExecutionKind = query.get("execution_kind")?.trim() || ""
    const executionKind: HRISWebhookExecutionKindFilter | undefined = ["receipt_process", "dlq_replay"].includes(
      rawExecutionKind
    )
      ? (rawExecutionKind as HRISWebhookExecutionKindFilter)
      : undefined
    const rawExecutionMode = query.get("execution_mode")?.trim() || ""
    const executionMode: HRISWebhookExecutionModeFilter | undefined = ["inline", "queued"].includes(rawExecutionMode)
      ? (rawExecutionMode as HRISWebhookExecutionModeFilter)
      : undefined
    const rawExecutionQueueState = query.get("execution_queue_state")?.trim() || ""
    const executionQueueState: HRISWebhookExecutionQueueStateFilter | undefined = [
      "ready",
      "cooldown",
      "in_flight",
      "attempt_limit",
      "terminal",
    ].includes(rawExecutionQueueState)
      ? (rawExecutionQueueState as HRISWebhookExecutionQueueStateFilter)
      : undefined
    const rawExecutionReplayScope = query.get("execution_replay_scope")?.trim() || ""
    const executionReplayScope: HRISWebhookExecutionReplayScopeFilter | undefined = [
      "replayed",
      "worker_required",
    ].includes(rawExecutionReplayScope)
      ? (rawExecutionReplayScope as HRISWebhookExecutionReplayScopeFilter)
      : undefined
    const rawExecutionStatus = query.get("execution_status")?.trim() || ""
    const executionStatus: HRISWebhookExecutionStatusFilter | undefined = ["queued", "running", "succeeded", "failed"].includes(
      rawExecutionStatus
    )
      ? (rawExecutionStatus as HRISWebhookExecutionStatusFilter)
      : undefined
    const hasHints =
      activeSection === "sync" &&
      Boolean(
        focusHint ||
          workerFilter ||
          workerQuery ||
          workerAction ||
          workerLabel ||
          workerKind ||
          workerConnectorID ||
          workerFailureStage ||
          workerMode ||
          workerQueueState ||
          workerRequestID ||
          workerReplayState ||
          workerReviewStatus ||
          workerReviewStage ||
          workerStatus ||
          workerVendor ||
          executionID ||
          executionKind ||
          executionMode ||
          executionQueueState ||
          executionReplayScope ||
          executionStatus
      )
    return {
      hasHints,
      contextKey: hasHints ? `${location.search}::${activeSection}` : "",
      executionID: executionID || undefined,
      executionKind: executionKind || undefined,
      executionMode: executionMode || undefined,
      executionQueueState: executionQueueState || undefined,
      executionReplayScope: executionReplayScope || undefined,
      executionStatus: executionStatus || undefined,
      focusHint,
      workerAction: workerAction || undefined,
      workerConnectorID: workerConnectorID || undefined,
      workerFailureStage: workerFailureStage || undefined,
      workerFilter,
      workerKind: workerKind || undefined,
      workerLabel: workerLabel || undefined,
      workerMode: workerMode || undefined,
      workerQueueState: workerQueueState || undefined,
      workerRequestID: workerRequestID || undefined,
      workerReplayState: workerReplayState || undefined,
      workerReviewStage,
      workerReviewStatus,
      workerQuery: workerQuery || undefined,
      workerStatus: workerStatus || undefined,
      workerVendor: workerVendor || undefined,
    }
  }, [activeSection, location.search])

  const [tenants, setTenants] = useState<Tenant[]>([])
  const [selectedTenantID, setSelectedTenantID] = useState(viewerTenantID)
  const [employees, setEmployees] = useState<EnterpriseEmployee[]>([])
  const [hrisConnectors, setHRISConnectors] = useState<EnterpriseHRISConnector[]>([])
  const [hrisSecrets, setHRISSecrets] = useState<EnterpriseHRISSecret[]>([])
  const [syncJobs, setSyncJobs] = useState<EnterpriseSyncJob[]>([])
  const [syncRequests, setSyncRequests] = useState<EnterpriseSyncRequestRecord[]>([])
  const [workerAlertSubscription, setWorkerAlertSubscription] = useState<EnterpriseSyncWorkerAlertSubscription | null>(null)
  const [approvals, setApprovals] = useState<EnterpriseJITProvisionApproval[]>([])
  const [workerAlerts, setWorkerAlerts] = useState<EnterpriseSyncWorkerAlertSummaryItem[]>([])
  const [workerAlertEvents, setWorkerAlertEvents] = useState<EnterpriseSyncWorkerAlertItem[]>([])
  const [workerAlertNotifications, setWorkerAlertNotifications] = useState<EnterpriseSyncWorkerAlertNotification[]>([])
  const [workerAlertNotificationFilterCounts, setWorkerAlertNotificationFilterCounts] =
    useState<EnterpriseSyncWorkerAlertNotificationFilterCounts>(emptyWorkerAlertNotificationFilterCounts)
  const [workerAlertNotificationStatusCounts, setWorkerAlertNotificationStatusCounts] =
    useState<EnterpriseSyncWorkerAlertNotificationStatusCounts>(emptyWorkerAlertNotificationStatusCounts)
  const [workerAlertNotificationTotal, setWorkerAlertNotificationTotal] = useState(0)
  const [workerAlertNotificationHasMore, setWorkerAlertNotificationHasMore] = useState(false)
  const [workerAlertNotificationFilter, setWorkerAlertNotificationFilter] = useState<NotificationHistoryFilter>("all")
  const [workerAlertNotificationQuery, setWorkerAlertNotificationQuery] = useState("")
  const [hrisWebhookExecutions, setHRISWebhookExecutions] = useState<EnterpriseHRISWebhookExecution[]>([])
  const [hrisWebhookExecutionStatusCounts, setHRISWebhookExecutionStatusCounts] =
    useState<EnterpriseHRISWebhookExecutionStatusCounts>(emptyHRISWebhookExecutionStatusCounts)
  const [hrisWebhookExecutionQueueCounts, setHRISWebhookExecutionQueueCounts] =
    useState<EnterpriseHRISWebhookRuntimeCounts | null>(null)
  const [hrisWebhookExecutionTotal, setHRISWebhookExecutionTotal] = useState(0)
  const [hrisWebhookExecutionHasMore, setHRISWebhookExecutionHasMore] = useState(false)
  const [hrisWebhookExecutionNextOffset, setHRISWebhookExecutionNextOffset] = useState<number | null>(null)
  const [hrisWebhookExecutionKindFilter, setHRISWebhookExecutionKindFilter] =
    useState<HRISWebhookExecutionKindFilter>("all")
  const [hrisWebhookExecutionStatusFilter, setHRISWebhookExecutionStatusFilter] =
    useState<HRISWebhookExecutionStatusFilter>("all")
  const [hrisWebhookExecutionQueueStateFilter, setHRISWebhookExecutionQueueStateFilter] =
    useState<HRISWebhookExecutionQueueStateFilter>("all")
  const [hrisWebhookExecutionReplayScopeFilter, setHRISWebhookExecutionReplayScopeFilter] =
    useState<HRISWebhookExecutionReplayScopeFilter>("all")
  const [hrisWebhookExecutionModeFilter, setHRISWebhookExecutionModeFilter] =
    useState<HRISWebhookExecutionModeFilter>("all")
  const [hrisWebhookExecutionDispatchFilter, setHRISWebhookExecutionDispatchFilter] =
    useState<HRISWebhookExecutionDispatchFilter>("all")
  const [hrisWebhookExecutionTargetStatusFilter, setHRISWebhookExecutionTargetStatusFilter] = useState("")
  const [hrisWebhookExecutionQuery, setHRISWebhookExecutionQuery] = useState("")
  const [hrisWebhookReceipts, setHRISWebhookReceipts] = useState<EnterpriseHRISWebhookReceipt[]>([])
  const [hrisWebhookReceiptQueueCounts, setHRISWebhookReceiptQueueCounts] =
    useState<EnterpriseHRISWebhookRuntimeCounts | null>(null)
  const [hrisWebhookReceiptTotal, setHRISWebhookReceiptTotal] = useState(0)
  const [hrisWebhookReceiptHasMore, setHRISWebhookReceiptHasMore] = useState(false)
  const [hrisWebhookReceiptNextOffset, setHRISWebhookReceiptNextOffset] = useState<number | null>(null)
  const [hrisWebhookDLQEntries, setHRISWebhookDLQEntries] = useState<EnterpriseHRISWebhookDLQEntry[]>([])
  const [hrisWebhookDLQReplayCounts, setHRISWebhookDLQReplayCounts] =
    useState<EnterpriseHRISWebhookRuntimeCounts | null>(null)
  const [hrisWebhookDLQTotal, setHRISWebhookDLQTotal] = useState(0)
  const [hrisWebhookDLQHasMore, setHRISWebhookDLQHasMore] = useState(false)
  const [hrisWebhookDLQNextOffset, setHRISWebhookDLQNextOffset] = useState<number | null>(null)
  const [hrisPullStates, setHRISPullStates] = useState<EnterpriseHRISPullState[]>([])
  const [idpConfig, setIDPConfig] = useState<EnterpriseIDPConfig | null>(null)
  const [userGroups, setUserGroups] = useState<UserGroup[]>([])
  const [policies, setPolicies] = useState<AccessPolicy[]>([])
  const [issuedPasses, setIssuedPasses] = useState<WalletPassInstance[]>([])

  const [syncSource, setSyncSource] = useState("hris")
  const [syncRequestID, setSyncRequestID] = useState(() => `sync-${Date.now()}`)
  const [syncPayload, setSyncPayload] = useState(sampleSyncPayload)
  const [syncSummary, setSyncSummary] = useState("")
  const [approvalActionID, setApprovalActionID] = useState<string | null>(null)
  const [dlqActionID, setDLQActionID] = useState<string | null>(null)
  const [executionActionID, setExecutionActionID] = useState<string | null>(null)
  const [receiptActionID, setReceiptActionID] = useState<string | null>(null)
  const [latestWebhookReceiptBatchProcessResult, setLatestWebhookReceiptBatchProcessResult] =
    useState<EnterpriseHRISWebhookReceiptBatchProcessResult | null>(null)
  const [latestWebhookDLQBatchReplayResult, setLatestWebhookDLQBatchReplayResult] =
    useState<EnterpriseHRISWebhookDLQBatchReplayResult | null>(null)
  const [syncRequestActionID, setSyncRequestActionID] = useState<string | null>(null)
  const [workerAlertSubscriptionActionID, setWorkerAlertSubscriptionActionID] = useState<string | null>(null)
  const [workerAlertDispatchActionID, setWorkerAlertDispatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationActionID, setWorkerAlertNotificationActionID] = useState<string | null>(null)
  const [workerAlertNotificationBatchActionID, setWorkerAlertNotificationBatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationSuppressBatchActionID, setWorkerAlertNotificationSuppressBatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationRestoreBatchActionID, setWorkerAlertNotificationRestoreBatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationAutoRetryActionID, setWorkerAlertNotificationAutoRetryActionID] = useState<string | null>(null)
  const [workerAlertNotificationListLoading, setWorkerAlertNotificationListLoading] = useState(false)
  const [workerAlertNotificationListLoadingMore, setWorkerAlertNotificationListLoadingMore] = useState(false)
  const [workerAlertNotificationExporting, setWorkerAlertNotificationExporting] = useState(false)
  const [hrisWebhookExecutionListLoading, setHRISWebhookExecutionListLoading] = useState(false)
  const [hrisWebhookExecutionListLoadingMore, setHRISWebhookExecutionListLoadingMore] = useState(false)
  const [selectedHRISWebhookExecution, setSelectedHRISWebhookExecution] =
    useState<EnterpriseHRISWebhookExecution | null>(null)
  const [selectedHRISWebhookExecutionLoading, setSelectedHRISWebhookExecutionLoading] = useState(false)
  const [selectedHRISWebhookExecutionError, setSelectedHRISWebhookExecutionError] = useState("")
  const [hrisWebhookReceiptListLoading, setHRISWebhookReceiptListLoading] = useState(false)
  const [hrisWebhookReceiptListLoadingMore, setHRISWebhookReceiptListLoadingMore] = useState(false)
  const [hrisWebhookDLQListLoading, setHRISWebhookDLQListLoading] = useState(false)
  const [hrisWebhookDLQListLoadingMore, setHRISWebhookDLQListLoadingMore] = useState(false)

  const [syncing, setSyncing] = useState(false)
  const [error, setError] = useState("")

  const tenantsQuery = useQuery({
    queryKey: ["enterprise-tenants", platformViewer ? "platform" : "tenant"],
    queryFn: () => (platformViewer ? listTenants(token) : Promise.resolve([])),
    staleTime: 30 * 1000,
  })
  const enterpriseDataQuery = useQuery({
    queryKey: ["enterprise-data", selectedTenantID.trim()],
    queryFn: () => loadEnterprisePageData(token, selectedTenantID.trim()),
    enabled: selectedTenantID.trim().length > 0,
    staleTime: 30 * 1000,
  })
  const loading =
    (platformViewer && tenantsQuery.isPending) ||
    (selectedTenantID.trim().length > 0 && enterpriseDataQuery.isPending)
  const queryError =
    (tenantsQuery.error instanceof Error && tenantsQuery.error.message) ||
    (enterpriseDataQuery.error instanceof Error && enterpriseDataQuery.error.message) ||
    ""

  const fetchHRISWebhookReceiptsPage = (tenantID: string, offset = 0) =>
    listEnterpriseHRISWebhookReceipts(token, {
      tenant_id: tenantID,
      limit: hrisWebhookReceiptPageSize,
      offset,
    })

  const fetchHRISWebhookDLQPage = (tenantID: string, offset = 0) =>
    listEnterpriseHRISWebhookDLQ(token, {
      tenant_id: tenantID,
      limit: hrisWebhookDLQPageSize,
      offset,
    })

  const fetchHRISWebhookExecutionsPage = (tenantID: string, offset = 0) =>
    listEnterpriseHRISWebhookExecutions(
      token,
      buildHRISWebhookExecutionListOptions({
        tenantID,
        kind: hrisWebhookExecutionKindFilter,
        status: hrisWebhookExecutionStatusFilter,
        queueState: hrisWebhookExecutionQueueStateFilter,
        replayScope: hrisWebhookExecutionReplayScopeFilter,
        executionMode: hrisWebhookExecutionModeFilter,
        dispatchMode: hrisWebhookExecutionDispatchFilter,
        targetStatus: hrisWebhookExecutionTargetStatusFilter,
        query: hrisWebhookExecutionQuery,
        limit: hrisWebhookExecutionPageSize,
        offset,
      })
    )

  const applyHRISWebhookReceiptPage = (
    response: Awaited<ReturnType<typeof listEnterpriseHRISWebhookReceipts>>,
    mode: "replace" | "append" = "replace"
  ) => {
    setHRISWebhookReceipts((current) => (mode === "append" ? [...current, ...response.items] : response.items))
    setHRISWebhookReceiptQueueCounts(
      extractHRISWebhookRuntimeCounts(response.queue_counts, response.filter_counts, response.summary)
    )
    setHRISWebhookReceiptTotal(response.total)
    setHRISWebhookReceiptHasMore(response.has_more)
    setHRISWebhookReceiptNextOffset(response.next_offset ?? null)
  }

  const applyHRISWebhookExecutionPage = (
    response: Awaited<ReturnType<typeof listEnterpriseHRISWebhookExecutions>>,
    mode: "replace" | "append" = "replace"
  ) => {
    setHRISWebhookExecutions((current) => (mode === "append" ? [...current, ...response.items] : response.items))
    setHRISWebhookExecutionStatusCounts(response.status_counts ?? emptyHRISWebhookExecutionStatusCounts)
    setHRISWebhookExecutionQueueCounts(extractHRISWebhookRuntimeCounts(response.queue_counts))
    setHRISWebhookExecutionTotal(response.total)
    setHRISWebhookExecutionHasMore(response.has_more)
    setHRISWebhookExecutionNextOffset(response.next_offset ?? null)
  }

  const applyHRISWebhookDLQPage = (
    response: Awaited<ReturnType<typeof listEnterpriseHRISWebhookDLQ>>,
    mode: "replace" | "append" = "replace"
  ) => {
    setHRISWebhookDLQEntries((current) => (mode === "append" ? [...current, ...response.items] : response.items))
    setHRISWebhookDLQReplayCounts(
      extractHRISWebhookRuntimeCounts(response.replay_counts, response.filter_counts, response.summary)
    )
    setHRISWebhookDLQTotal(response.total)
    setHRISWebhookDLQHasMore(response.has_more)
    setHRISWebhookDLQNextOffset(response.next_offset ?? null)
  }

  const selectedTenant = useMemo(
    () => tenants.find((item) => item.id === selectedTenantID) ?? null,
    [selectedTenantID, tenants]
  )
  const selectedHRISWebhookExecutionID = enterpriseRouteHints.executionID || null
  const selectedHRISWebhookExecutionListItem = useMemo(
    () =>
      selectedHRISWebhookExecutionID
        ? hrisWebhookExecutions.find((item) => item.id === selectedHRISWebhookExecutionID) ?? null
        : null,
    [hrisWebhookExecutions, selectedHRISWebhookExecutionID]
  )
  const selectedHRISWebhookExecutionItem = useMemo(() => {
    if (selectedHRISWebhookExecution?.id === selectedHRISWebhookExecutionID) {
      return selectedHRISWebhookExecution
    }
    return selectedHRISWebhookExecutionListItem
  }, [selectedHRISWebhookExecution, selectedHRISWebhookExecutionID, selectedHRISWebhookExecutionListItem])
  const tenantGroups = useMemo(
    () => userGroups.filter((item) => item.tenant_id === selectedTenantID.trim()),
    [selectedTenantID, userGroups]
  )
  const tenantPolicies = useMemo(
    () => policies.filter((item) => item.tenant_id === selectedTenantID.trim()),
    [policies, selectedTenantID]
  )
  const activeEmployeeCount = useMemo(
    () => employees.filter((item) => item.status === "active").length,
    [employees]
  )
  const pendingApprovalCount = useMemo(
    () => approvals.filter((item) => item.status === "pending").length,
    [approvals]
  )
  const failedSyncJobCount = useMemo(
    () => syncJobs.filter((item) => item.status !== "completed" || item.rejected > 0).length,
    [syncJobs]
  )
  const workerAlertCount = useMemo(
    () => workerAlerts.reduce((sum, item) => sum + item.count, 0),
    [workerAlerts]
  )
  const idpReady = Boolean(idpConfig && idpConfig.status === "active")
  const issuedPassCount = issuedPasses.length
  const primaryEmployeeHint = useMemo(
    () =>
      employees.find((item) => item.status === "active" && item.email.trim()) ??
      employees.find((item) => item.status === "active") ??
      employees[0] ??
      null,
    [employees]
  )
  const primaryEmployeeTargetHint = useMemo(() => {
    if (!primaryEmployeeHint) {
      return ""
    }
    return primaryEmployeeHint.email.trim() || primaryEmployeeHint.external_id.trim() || primaryEmployeeHint.id.trim()
  }, [primaryEmployeeHint])
  const deactivatedEmployeeHint = useMemo(
    () =>
      employees.find((item) => item.status !== "active" && item.email.trim()) ??
      employees.find((item) => item.status !== "active") ??
      null,
    [employees]
  )
  const primaryGroupHint = useMemo(() => tenantGroups[0]?.name?.trim() || "Common Office Access", [tenantGroups])
  const primaryPolicyHint = useMemo(() => {
    if (tenantPolicies[0]?.name?.trim()) {
      return tenantPolicies[0].name.trim()
    }
    return primaryGroupHint
      ? t("enterprisePage.flowHints.officePolicyTemplate", { groupName: primaryGroupHint })
      : t("enterprisePage.flowHints.firstPolicy")
  }, [primaryGroupHint, t, tenantPolicies])
  const directoryFlowLink = buildEnterpriseFlowLink("/access/directory", selectedTenantID, "directory", {
    group_name: primaryGroupHint,
    group_desc: t("enterprisePage.flowHints.groupDescSyncDraft"),
    group_member_email: primaryEmployeeHint?.email || "",
    group_member_id: primaryEmployeeHint?.id || "",
    group_member_name: primaryEmployeeHint?.full_name || "",
  })
  const deactivatedDirectoryFlowLink = buildEnterpriseFlowLink("/access/directory", selectedTenantID, "directory", {
    group_name: primaryGroupHint,
    group_desc: t("enterprisePage.flowHints.groupDescDeactivatedCleanup"),
    group_member_email: deactivatedEmployeeHint?.email || "",
    group_member_id: deactivatedEmployeeHint?.id || "",
    group_member_name: deactivatedEmployeeHint?.full_name || "",
    group_member_status: deactivatedEmployeeHint?.status || "",
    remediation_hint: "deactivated_cleanup",
  })
  const policiesFlowLink = buildEnterpriseFlowLink("/access/policies", selectedTenantID, "policies", {
    policy_group: primaryGroupHint,
    policy_name: primaryPolicyHint,
  })
  const walletFlowLink = buildEnterpriseFlowLink("/wallet?scenario=employee_mobile", selectedTenantID, "issuance", {
    target_hint: "user",
    target_email: primaryEmployeeHint?.email || "",
    target_id: primaryEmployeeTargetHint,
    target_name: primaryEmployeeHint?.full_name || "",
    template_hint: "employee",
  })
  const enterpriseSyncFlowLink = `${buildEnterpriseFlowLink("/enterprise", selectedTenantID, "directory")}#sync`
  const enterpriseAlertsFlowLink = `${buildEnterpriseFlowLink("/enterprise", selectedTenantID, "directory")}#alerts`
  const sortedSyncJobs = useMemo(
    () =>
      [...syncJobs].sort((left, right) => {
        const leftTime = new Date(left.ended_at || left.started_at).getTime() || 0
        const rightTime = new Date(right.ended_at || right.started_at).getTime() || 0
        return rightTime - leftTime
      }),
    [syncJobs]
  )
  const latestSyncJob = sortedSyncJobs[0] ?? null
  const workflowSteps = useMemo(
    () => [
      {
        id: "sync" as const,
        title: t("enterprisePage.workflow.steps.sync.title"),
        metric:
          loading
            ? "--"
            : t("enterprisePage.workflow.steps.sync.metric", {
                activeEmployeeCount,
                syncCount: syncJobs.length,
              }),
        state: activeEmployeeCount > 0 ? ("completed" as const) : ("pending" as const),
        helper:
          activeEmployeeCount > 0
            ? t("enterprisePage.workflow.steps.sync.helperCompleted")
            : t("enterprisePage.workflow.steps.sync.helperPending"),
        actionLabel: activeEmployeeCount > 0 ? t("enterprisePage.actions.viewEmployees") : t("enterprisePage.actions.goToSync"),
      },
      {
        id: "directory" as const,
        title: t("enterprisePage.workflow.steps.directory.title"),
        metric: loading ? "--" : t("enterprisePage.workflow.steps.directory.metric", { count: tenantGroups.length }),
        state:
          activeEmployeeCount === 0
            ? ("blocked" as const)
            : tenantGroups.length > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          activeEmployeeCount === 0
            ? t("enterprisePage.workflow.steps.directory.helperBlocked")
            : tenantGroups.length > 0
              ? t("enterprisePage.workflow.steps.directory.helperCompleted")
              : t("enterprisePage.workflow.steps.directory.helperPending"),
        actionLabel: t("enterprisePage.actions.goToDirectory"),
      },
      {
        id: "policies" as const,
        title: t("enterprisePage.workflow.steps.policies.title"),
        metric: loading ? "--" : t("enterprisePage.workflow.steps.policies.metric", { count: tenantPolicies.length }),
        state:
          tenantGroups.length === 0
            ? ("blocked" as const)
            : tenantPolicies.length > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          tenantGroups.length === 0
            ? t("enterprisePage.workflow.steps.policies.helperBlocked")
            : tenantPolicies.length > 0
              ? t("enterprisePage.workflow.steps.policies.helperCompleted")
              : t("enterprisePage.workflow.steps.policies.helperPending"),
        actionLabel: t("enterprisePage.actions.goToPolicies"),
      },
      {
        id: "issuance" as const,
        title: t("enterprisePage.workflow.steps.issuance.title"),
        metric: loading ? "--" : t("enterprisePage.workflow.steps.issuance.metric", { count: issuedPassCount }),
        state:
          tenantPolicies.length === 0
            ? ("blocked" as const)
            : issuedPassCount > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          tenantPolicies.length === 0
            ? t("enterprisePage.workflow.steps.issuance.helperBlocked")
            : issuedPassCount > 0
              ? t("enterprisePage.workflow.steps.issuance.helperCompleted")
              : t("enterprisePage.workflow.steps.issuance.helperPending"),
        actionLabel: issuedPassCount > 0 ? t("enterprisePage.actions.viewIssuanceCenter") : t("enterprisePage.actions.goToWallet"),
      },
    ],
    [activeEmployeeCount, issuedPassCount, loading, syncJobs.length, t, tenantGroups.length, tenantPolicies.length]
  )
  const nextWorkflowAction = useMemo(() => {
    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.nextAction.connectEmployees.title"),
        description: t("enterprisePage.nextAction.connectEmployees.description"),
        kind: "section" as const,
        section: "sync" as const,
        label: t("enterprisePage.actions.goToSync"),
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.nextAction.groupEmployees.title"),
        description: t("enterprisePage.nextAction.groupEmployees.description", { activeEmployeeCount }),
        kind: "route" as const,
        to: directoryFlowLink,
        label: t("enterprisePage.actions.goToDirectory"),
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.nextAction.createPolicies.title"),
        description: t("enterprisePage.nextAction.createPolicies.description", { tenantGroupsCount: tenantGroups.length }),
        kind: "route" as const,
        to: policiesFlowLink,
        label: t("enterprisePage.actions.goToPolicies"),
      }
    }
    if (issuedPassCount === 0) {
      return {
        title: t("enterprisePage.nextAction.goIssuance.title"),
        description: t("enterprisePage.nextAction.goIssuance.description"),
        kind: "route" as const,
        to: walletFlowLink,
        label: t("enterprisePage.actions.goToWallet"),
      }
    }
    return {
      title: t("enterprisePage.nextAction.closedLoop.title"),
      description: t("enterprisePage.nextAction.closedLoop.description"),
      kind: "route" as const,
      to: walletFlowLink,
      label: t("enterprisePage.actions.viewIssuanceCenter"),
    }
  }, [activeEmployeeCount, directoryFlowLink, issuedPassCount, policiesFlowLink, t, tenantGroups.length, tenantPolicies.length, walletFlowLink])
  const workflowDirectAction = useMemo(() => {
    if (nextWorkflowAction.kind === "section") {
      return {
        kind: "section" as const,
        section: nextWorkflowAction.section,
        label: nextWorkflowAction.label,
      }
    }
    return {
      kind: "route" as const,
      to: nextWorkflowAction.to,
      label: nextWorkflowAction.label,
    }
  }, [nextWorkflowAction, t])
  const workflowStepsOverview = useMemo<EnterpriseWorkflowOverviewStep[]>(
    () =>
      workflowSteps.map((step) => ({
        id: step.id,
        title: step.title,
        metric: step.metric,
        helper: step.helper,
        statusLabel: workflowStateLabel(t, step.state),
        statusVariant: workflowStateVariant(step.state) as EnterpriseWorkflowOverviewStep["statusVariant"],
        action:
          step.id === "sync"
            ? ({
                kind: "section" as const,
                section: activeEmployeeCount > 0 ? ("employees" as const) : ("sync" as const),
                label: step.actionLabel,
              })
            : step.id === "directory"
              ? ({
                  kind: "route" as const,
                  to: directoryFlowLink,
                  label: step.actionLabel,
                })
              : step.id === "policies"
                ? ({
                    kind: "route" as const,
                    to: policiesFlowLink,
                    label: step.actionLabel,
                  })
                : ({
                    kind: "route" as const,
                    to: walletFlowLink,
                    label: step.actionLabel,
                  }),
      })),
    [activeEmployeeCount, directoryFlowLink, policiesFlowLink, t, walletFlowLink, workflowSteps]
  )
  const workflowReturnAction = useMemo(() => {
    if (nextWorkflowAction.kind === "section") {
      return {
        kind: "section" as const,
        section: nextWorkflowAction.section,
        label: t("enterprisePage.actions.afterAction", { action: nextWorkflowAction.label }),
      }
    }
    return {
      kind: "route" as const,
      to: nextWorkflowAction.to,
      label: t("enterprisePage.actions.afterAction", { action: nextWorkflowAction.label }),
    }
  }, [nextWorkflowAction])
  const alertLandingCards = useMemo(() => {
    const resolvePostSyncAction = () => {
      if (activeEmployeeCount === 0) {
        return {
          kind: "section" as const,
          section: "employees" as const,
          label: t("enterprisePage.actions.reviewEmployees"),
        }
      }
      if (tenantGroups.length === 0) {
        return {
          kind: "route" as const,
          to: directoryFlowLink,
          label: t("enterprisePage.actions.goToDirectory"),
        }
      }
      if (tenantPolicies.length === 0) {
        return {
          kind: "route" as const,
          to: policiesFlowLink,
          label: t("enterprisePage.actions.goToPolicies"),
        }
      }
      if (issuedPassCount === 0) {
        return {
          kind: "route" as const,
          to: walletFlowLink,
          label: t("enterprisePage.actions.goToWallet"),
        }
      }
      return {
        kind: "route" as const,
        to: walletFlowLink,
        label: t("enterprisePage.actions.viewIssuanceCenter"),
      }
    }

    const postSyncAction = resolvePostSyncAction()
    const postSyncReturnAction =
      postSyncAction.kind === "section"
        ? {
            kind: "section" as const,
            section: postSyncAction.section,
            label: t("enterprisePage.actions.afterAction", { action: postSyncAction.label }),
          }
        : {
            kind: "route" as const,
            to: postSyncAction.to,
            label: t("enterprisePage.actions.afterAction", { action: postSyncAction.label }),
          }

    const syncLandingCard = !latestSyncJob
      ? {
          title: t("enterprisePage.cards.syncOutcome.title"),
          statusLabel: t("enterprisePage.status.pendingImport"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.cards.syncOutcome.emptyDescription"),
          action: {
            kind: "section" as const,
            section: "sync" as const,
            label: t("enterprisePage.actions.goToSync"),
          },
          returnHint: t("enterprisePage.cards.syncOutcome.returnHintAfterImport"),
        }
      : latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0
        ? {
            title: t("enterprisePage.cards.syncOutcome.title"),
            statusLabel: t("enterprisePage.status.pendingReview"),
            statusVariant: "destructive" as const,
            description: t("enterprisePage.cards.syncOutcome.needsReview", {
              sourceLabel: syncSourceLabelValue(latestSyncJob.source),
              status: latestSyncJob.status,
              rejected: latestSyncJob.rejected,
            }),
            action: {
              kind: "section" as const,
              section: "alerts" as const,
              label: t("enterprisePage.actions.goToAlerts"),
            },
            returnAction: postSyncReturnAction,
            returnHint: t("enterprisePage.cards.syncOutcome.returnHintAfterAlert"),
          }
        : {
            title: t("enterprisePage.cards.syncOutcome.title"),
            statusLabel:
              activeEmployeeCount === 0
                ? t("enterprisePage.status.pendingConfirm")
                : tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                  ? t("enterprisePage.status.pendingBackflow")
                  : t("enterprisePage.status.closedLoop"),
            statusVariant:
              activeEmployeeCount === 0
                ? ("secondary" as const)
                : tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                  ? ("secondary" as const)
                  : ("outline" as const),
            description:
              activeEmployeeCount === 0
                ? t("enterprisePage.cards.syncOutcome.completedEmptyDirectory")
                : tenantGroups.length === 0
                  ? t("enterprisePage.cards.syncOutcome.groupEmployeesNext", { activeEmployeeCount })
                  : tenantPolicies.length === 0
                    ? t("enterprisePage.cards.syncOutcome.applyPoliciesNext", { tenantGroupsCount: tenantGroups.length })
                    : issuedPassCount === 0
                      ? t("enterprisePage.cards.syncOutcome.readyForIssuance")
                      : t("enterprisePage.cards.syncOutcome.connectedWithIssuedPasses", { issuedPassCount }),
            action: postSyncAction,
            returnHint:
              tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                ? t("enterprisePage.cards.syncOutcome.keepMoving")
                : t("enterprisePage.cards.syncOutcome.closedLoopHint"),
          }

    const approvalLandingCard =
      pendingApprovalCount > 0
        ? {
            title: t("enterprisePage.cards.approvalBacklog.title"),
            statusLabel: t("enterprisePage.cards.approvalBacklog.pendingCount", { count: pendingApprovalCount }),
            statusVariant: "secondary" as const,
            description: t("enterprisePage.cards.approvalBacklog.descriptionPending"),
            action: {
              kind: "section" as const,
              section: "alerts" as const,
              label: t("enterprisePage.actions.handleApprovals"),
            },
            returnAction: workflowReturnAction,
            returnHint: t("enterprisePage.cards.approvalBacklog.returnHint"),
          }
        : {
            title: t("enterprisePage.cards.approvalBacklog.title"),
            statusLabel: t("enterprisePage.status.cleared"),
            statusVariant: "outline" as const,
            description: t("enterprisePage.cards.approvalBacklog.descriptionCleared"),
            action: workflowDirectAction,
          }

    const directoryExceptionCard =
      latestSyncJob && latestSyncJob.deactivated > 0
        ? {
            title: t("enterprisePage.cards.directoryException.title"),
            statusLabel: t("enterprisePage.status.pendingCleanup"),
            statusVariant: "secondary" as const,
            description: t("enterprisePage.cards.directoryException.deactivatedEmployees", {
              deactivated: latestSyncJob.deactivated,
            }),
            action: {
              kind: "route" as const,
              to: directoryFlowLink,
              label: t("enterprisePage.actions.reviewDirectory"),
            },
            returnAction: workflowReturnAction,
            returnHint: t("enterprisePage.cards.directoryException.returnHint"),
          }
        : latestSyncJob && latestSyncJob.status === "completed" && activeEmployeeCount === 0
          ? {
              title: t("enterprisePage.cards.directoryException.title"),
              statusLabel: t("enterprisePage.status.needsReview"),
              statusVariant: "destructive" as const,
              description: t("enterprisePage.cards.directoryException.completedButEmpty"),
              action: {
                kind: "section" as const,
                section: "employees" as const,
                label: t("enterprisePage.actions.reviewEmployees"),
              },
              returnAction: {
                kind: "section" as const,
                section: "sync" as const,
                label: t("enterprisePage.actions.backToSyncAfterReview"),
              },
            }
          : {
              title: t("enterprisePage.cards.directoryException.title"),
              statusLabel: t("enterprisePage.status.directoryStable"),
              statusVariant: "outline" as const,
              description: t("enterprisePage.cards.directoryException.stableDescription"),
              action: {
                kind: "route" as const,
                to: directoryFlowLink,
                label: t("enterprisePage.actions.viewDirectory"),
              },
            }

    return [syncLandingCard, approvalLandingCard, directoryExceptionCard]
  }, [
    activeEmployeeCount,
    issuedPassCount,
    latestSyncJob,
    pendingApprovalCount,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    directoryFlowLink,
    policiesFlowLink,
    walletFlowLink,
    workflowDirectAction,
    workflowReturnAction,
  ])
  const attentionItems = useMemo(() => {
    const items: Array<{
      title: string
      description: string
      actionLabel: string
      onClick: () => void
    }> = []

    if (!idpReady) {
      items.push({
        title: t("enterprisePage.attention.idpNotReady.title"),
        description: t("enterprisePage.attention.idpNotReady.description"),
        actionLabel: t("enterprisePage.actions.goToIdp"),
        onClick: () => goToSection("idp"),
      })
    }
    if (failedSyncJobCount > 0) {
      items.push({
        title: t("enterprisePage.attention.syncNeedsReview.title"),
        description: t("enterprisePage.attention.syncNeedsReview.description", { failedSyncJobCount }),
        actionLabel: t("enterprisePage.actions.goToAlerts"),
        onClick: () => goToSection("alerts"),
      })
    }
    if (pendingApprovalCount > 0) {
      items.push({
        title: t("enterprisePage.attention.pendingApprovals.title"),
        description: t("enterprisePage.attention.pendingApprovals.description", { pendingApprovalCount }),
        actionLabel: t("enterprisePage.actions.handleApprovals"),
        onClick: () => goToSection("alerts"),
      })
    }
    if (workerAlertCount > 0) {
      items.push({
        title: t("enterprisePage.attention.workerAlert.title"),
        description: t("enterprisePage.attention.workerAlert.description", { workerAlertCount }),
        actionLabel: t("enterprisePage.actions.viewExceptions"),
        onClick: () => goToSection("alerts"),
      })
    }

    return items.slice(0, 4)
  }, [failedSyncJobCount, idpReady, pendingApprovalCount, t, workerAlertCount])
  const quickActions = useMemo(
    () => [
      {
        title: t("enterprisePage.quickActions.hris.title"),
        description: t("enterprisePage.quickActions.hris.description"),
        actionLabel: t("enterprisePage.actions.setAsSyncEntry"),
        onClick: () => {
          setSyncSource("hris")
          goToSection("sync")
        },
      },
      {
        title: t("enterprisePage.quickActions.csv.title"),
        description: t("enterprisePage.quickActions.csv.description"),
        actionLabel: t("enterprisePage.actions.switchToCsv"),
        onClick: () => {
          setSyncSource("csv_import")
          goToSection("sync")
        },
      },
      {
        title: t("enterprisePage.quickActions.idp.title"),
        description: t("enterprisePage.quickActions.idp.description"),
        actionLabel: t("enterprisePage.actions.viewEnterpriseLogin"),
        onClick: () => {
          goToSection("idp")
        },
      },
      {
        title: t("enterprisePage.labels.alerts"),
        description: t("enterprisePage.quickActions.alerts.description"),
        actionLabel: t("enterprisePage.actions.viewPendingItems"),
        onClick: () => {
          goToSection("alerts")
        },
      },
    ],
    [t]
  )
  const alertRecoveryAction = useMemo(() => {
    const blockerCount = failedSyncJobCount + pendingApprovalCount + workerAlertCount + (idpReady ? 0 : 1)
    return {
      blockerCount,
      title:
        blockerCount > 0
          ? t("enterprisePage.alertRecovery.withBlockersTitle")
          : t("enterprisePage.alertRecovery.noBlockersTitle"),
      description:
        blockerCount > 0
          ? t("enterprisePage.alertRecovery.withBlockersDescription", { blockerCount })
          : t("enterprisePage.alertRecovery.noBlockersDescription"),
      nextAction: nextWorkflowAction,
    }
  }, [failedSyncJobCount, idpReady, nextWorkflowAction, pendingApprovalCount, t, workerAlertCount])
  const idpOutcomeAction = useMemo(() => {
    if (!idpReady) {
      return {
        title: t("enterprisePage.idpOutcome.setupIdpFirst.title"),
        description: t("enterprisePage.idpOutcome.setupIdpFirst.description"),
        kind: "section" as const,
        section: "idp" as const,
        label: t("enterprisePage.actions.goToIdp"),
      }
    }
    if (pendingApprovalCount > 0 || failedSyncJobCount > 0 || workerAlertCount > 0) {
      return {
        title: t("enterprisePage.idpOutcome.clearAlertsFirst.title"),
        description: t("enterprisePage.idpOutcome.clearAlertsFirst.description", {
          pendingApprovalCount,
          failedSyncJobCount,
          workerAlertCount,
        }),
        kind: "section" as const,
        section: "alerts" as const,
        label: t("enterprisePage.actions.goToAlerts"),
      }
    }
    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.idpOutcome.confirmSyncSource.title"),
        description: t("enterprisePage.idpOutcome.confirmSyncSource.description"),
        kind: "section" as const,
        section: "sync" as const,
        label: t("enterprisePage.actions.goToSync"),
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.idpOutcome.groupEmployeesAfterIdp.title"),
        description: t("enterprisePage.idpOutcome.groupEmployeesAfterIdp.description"),
        kind: "route" as const,
        to: directoryFlowLink,
        label: t("enterprisePage.actions.goToDirectory"),
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.idpOutcome.goPolicyRules.title"),
        description: t("enterprisePage.idpOutcome.goPolicyRules.description"),
        kind: "route" as const,
        to: policiesFlowLink,
        label: t("enterprisePage.actions.goToPolicies"),
      }
    }
    return {
      title: t("enterprisePage.idpOutcome.idpClosedLoop.title"),
      description: t("enterprisePage.idpOutcome.idpClosedLoop.description"),
      kind: "route" as const,
      to: walletFlowLink,
      label: t("enterprisePage.actions.goToWallet"),
    }
  }, [
    activeEmployeeCount,
    directoryFlowLink,
    failedSyncJobCount,
    idpReady,
    pendingApprovalCount,
    policiesFlowLink,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
    workerAlertCount,
  ])
  const syncOutcomeAction = useMemo(() => {
    if (!latestSyncJob) {
      return {
        title: t("enterprisePage.syncOutcome.firstImport.title"),
        description: t("enterprisePage.syncOutcome.firstImport.description"),
        kind: "section" as const,
        section: "sync" as const,
        label: t("enterprisePage.actions.continueImport"),
      }
    }
    if (latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0) {
      return {
        title: t("enterprisePage.syncOutcome.reviewRejected.title"),
        description: t("enterprisePage.syncOutcome.reviewRejected.description", {
          sourceLabel: syncSourceLabelValue(latestSyncJob.source),
          rejected: latestSyncJob.rejected,
        }),
        kind: "section" as const,
        section: "alerts" as const,
        label: t("enterprisePage.actions.goToAlerts"),
      }
    }
    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.syncOutcome.backToEmployees.title"),
        description: t("enterprisePage.syncOutcome.backToEmployees.description"),
        kind: "section" as const,
        section: "employees" as const,
        label: t("enterprisePage.actions.viewEmployees"),
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.syncOutcome.groupNewEmployees.title"),
        description: t("enterprisePage.syncOutcome.groupNewEmployees.description", { activeEmployeeCount }),
        kind: "route" as const,
        to: directoryFlowLink,
        label: t("enterprisePage.actions.goToDirectory"),
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.syncOutcome.createPolicies.title"),
        description: t("enterprisePage.syncOutcome.createPolicies.description", { tenantGroupsCount: tenantGroups.length }),
        kind: "route" as const,
        to: policiesFlowLink,
        label: t("enterprisePage.actions.goToPolicies"),
      }
    }
    return {
      title: t("enterprisePage.syncOutcome.closedLoop.title"),
      description: t("enterprisePage.syncOutcome.closedLoop.description"),
      kind: "route" as const,
      to: walletFlowLink,
      label: t("enterprisePage.actions.goToWallet"),
    }
  }, [
    activeEmployeeCount,
    directoryFlowLink,
    latestSyncJob,
    policiesFlowLink,
    syncSourceLabelValue,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])
  const syncIssueCards = useMemo(() => {
    if (!latestSyncJob) {
      return []
    }

    const items: Array<{
      title: string
      description: string
      label: string
      kind: "section" | "route"
      section?: EnterpriseSection
      to?: string
    }> = []

    if (latestSyncJob.status !== "completed") {
      items.push({
        title: t("enterprisePage.cards.latestSyncIncomplete.title"),
        description: t("enterprisePage.cards.latestSyncIncomplete.description", {
          sourceLabel: syncSourceLabelValue(latestSyncJob.source),
          status: latestSyncJob.status,
        }),
        label: t("enterprisePage.actions.goToAlerts"),
        kind: "section",
        section: "alerts",
      })
    }
    if (latestSyncJob.rejected > 0) {
      items.push({
        title: t("enterprisePage.cards.rejectedRecords.title"),
        description: t("enterprisePage.cards.rejectedRecords.description", { rejected: latestSyncJob.rejected }),
        label: t("enterprisePage.actions.goToAlerts"),
        kind: "section",
        section: "alerts",
      })
    }
    if (latestSyncJob.deactivated > 0) {
      items.push({
        title: t("enterprisePage.cards.deactivatedEmployees.title"),
        description: t("enterprisePage.cards.deactivatedEmployees.description", { deactivated: latestSyncJob.deactivated }),
        label: t("enterprisePage.actions.goToDeactivatedCleanup"),
        kind: "route",
        to: deactivatedDirectoryFlowLink,
      })
    }
    if (latestSyncJob.status === "completed" && activeEmployeeCount === 0) {
      items.push({
        title: t("enterprisePage.cards.syncCompletedEmpty.title"),
        description: t("enterprisePage.cards.syncCompletedEmpty.description"),
        label: t("enterprisePage.actions.viewEmployees"),
        kind: "section",
        section: "employees",
      })
    }

    return items.slice(0, 4)
  }, [activeEmployeeCount, deactivatedDirectoryFlowLink, latestSyncJob, syncSourceLabelValue, t])
  const syncRemediationSteps = useMemo(() => {
    const steps: Array<{
      title: string
      description: string
      state: EnterpriseWorkflowState
      kind: "section" | "route"
      label: string
      section?: EnterpriseSection
      to?: string
    }> = []

    if (!latestSyncJob) {
      steps.push({
        title: t("enterprisePage.remediation.firstSync.title"),
        description: t("enterprisePage.remediation.firstSync.description"),
        state: "pending",
        kind: "section",
        label: t("enterprisePage.actions.goToSync"),
        section: "sync",
      })
      return steps
    }

    steps.push({
      title: t("enterprisePage.remediation.reviewLatest.title"),
      description: t("enterprisePage.remediation.reviewLatest.description", {
        sourceLabel: syncSourceLabelValue(latestSyncJob.source),
        status: latestSyncJob.status,
      }),
      state: "completed",
      kind: "section",
      label: t("enterprisePage.actions.viewSyncResult"),
      section: "sync",
    })

    steps.push({
      title: t("enterprisePage.remediation.handleRejected.title"),
      description:
        latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0
          ? t("enterprisePage.remediation.handleRejected.pending", { rejected: latestSyncJob.rejected })
          : t("enterprisePage.remediation.handleRejected.completed"),
      state: latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0 ? "pending" : "completed",
      kind: "section",
      label: t("enterprisePage.actions.goToAlerts"),
      section: "alerts",
    })

    steps.push({
      title: t("enterprisePage.remediation.reviewDeactivatedImpact.title"),
      description:
        latestSyncJob.deactivated > 0
          ? t("enterprisePage.remediation.reviewDeactivatedImpact.pending", { deactivated: latestSyncJob.deactivated })
          : t("enterprisePage.remediation.reviewDeactivatedImpact.completed"),
      state: latestSyncJob.deactivated > 0 ? "pending" : "completed",
      kind: "route",
      label: t("enterprisePage.actions.goToDeactivatedCleanup"),
      to: deactivatedDirectoryFlowLink,
    })

    steps.push({
      title: t("enterprisePage.remediation.pushToPolicyAndIssuance.title"),
      description:
        activeEmployeeCount === 0
          ? t("enterprisePage.remediation.pushToPolicyAndIssuance.noEmployees")
          : tenantGroups.length === 0
            ? t("enterprisePage.remediation.pushToPolicyAndIssuance.noGroups")
            : tenantPolicies.length === 0
              ? t("enterprisePage.remediation.pushToPolicyAndIssuance.noPolicies")
              : t("enterprisePage.remediation.pushToPolicyAndIssuance.readyForIssuance"),
      state:
        activeEmployeeCount === 0
          ? "blocked"
          : tenantGroups.length === 0 || tenantPolicies.length === 0
            ? "pending"
            : "completed",
      kind: activeEmployeeCount === 0 ? "section" : "route",
      label:
        activeEmployeeCount === 0
          ? t("enterprisePage.actions.viewEmployees")
          : tenantGroups.length === 0
            ? t("enterprisePage.actions.goToDirectory")
            : tenantPolicies.length === 0
              ? t("enterprisePage.actions.goToPolicies")
              : t("enterprisePage.actions.goToWallet"),
      section: activeEmployeeCount === 0 ? "employees" : undefined,
      to:
        activeEmployeeCount === 0
          ? undefined
          : tenantGroups.length === 0
            ? directoryFlowLink
            : tenantPolicies.length === 0
              ? policiesFlowLink
              : walletFlowLink,
    })

    return steps
  }, [
    activeEmployeeCount,
    deactivatedDirectoryFlowLink,
    directoryFlowLink,
    latestSyncJob,
    policiesFlowLink,
    syncSourceLabelValue,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])
  const syncSourceFeedback = useMemo(() => {
    const checkpoints = syncSourceCheckpointsBySource[syncSource] ?? []
    const latestJobOfSource = sortedSyncJobs.find((item) => item.source === syncSource) ?? null
    const sourceLabel = syncSourceLabelValue(syncSource)

    if (!latestJobOfSource) {
      return {
        title: t("enterprisePage.syncSourceFeedback.pendingFirstResult.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.pendingSubmit"),
        statusVariant: "secondary" as const,
        description: t("enterprisePage.syncSourceFeedback.pendingFirstResult.description"),
        checkpoints,
        action: {
          kind: "section" as const,
          section: "sync" as const,
          label: t("enterprisePage.actions.submitSyncJob"),
        },
      }
    }

    if (latestJobOfSource.status !== "completed" || latestJobOfSource.rejected > 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.pendingReview.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.needsReview"),
        statusVariant: "destructive" as const,
        description: t("enterprisePage.syncSourceFeedback.pendingReview.description", {
          sourceLabel,
          status: latestJobOfSource.status,
          rejected: latestJobOfSource.rejected,
        }),
        checkpoints,
        action: {
          kind: "section" as const,
          section: "alerts" as const,
          label: t("enterprisePage.actions.goToAlerts"),
        },
      }
    }

    if (syncSource === "scim" && latestJobOfSource.deactivated > 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.scimDeactivated.title"),
        statusLabel: t("enterprisePage.status.pendingBackflow"),
        statusVariant: "secondary" as const,
        description: t("enterprisePage.syncSourceFeedback.scimDeactivated.description", {
          deactivated: latestJobOfSource.deactivated,
        }),
        checkpoints,
        action: {
          kind: "route" as const,
          to: deactivatedDirectoryFlowLink,
          label: t("enterprisePage.actions.goToDeactivatedCleanup"),
        },
      }
    }

    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.pendingConfirm.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.pendingConfirm"),
        statusVariant: "secondary" as const,
        description: t("enterprisePage.syncSourceFeedback.pendingConfirm.description"),
        checkpoints,
        action: {
          kind: "section" as const,
          section: "employees" as const,
          label: t("enterprisePage.actions.viewEmployees"),
        },
      }
    }

    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.readyForGrouping.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.readyToProceed"),
        statusVariant: "outline" as const,
        description: t("enterprisePage.syncSourceFeedback.readyForGrouping.description", { activeEmployeeCount }),
        checkpoints,
        action: {
          kind: "route" as const,
          to: directoryFlowLink,
          label: t("enterprisePage.actions.goToDirectory"),
        },
      }
    }

    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.backflowToDirectory.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.readyToProceed"),
        statusVariant: "outline" as const,
        description: t("enterprisePage.syncSourceFeedback.backflowToDirectory.description", { tenantGroupsCount: tenantGroups.length }),
        checkpoints,
        action: {
          kind: "route" as const,
          to: policiesFlowLink,
          label: t("enterprisePage.actions.goToPolicies"),
        },
      }
    }

    return {
      title: t("enterprisePage.syncSourceFeedback.closedLoop.title", { sourceLabel }),
      statusLabel: t("enterprisePage.status.closedLoop"),
      statusVariant: "outline" as const,
      description: t("enterprisePage.syncSourceFeedback.closedLoop.description"),
      checkpoints,
      action: {
        kind: "route" as const,
        to: walletFlowLink,
        label: t("enterprisePage.actions.goToWallet"),
      },
    }
  }, [
    activeEmployeeCount,
    deactivatedDirectoryFlowLink,
    directoryFlowLink,
    policiesFlowLink,
    sortedSyncJobs,
    syncSource,
    syncSourceCheckpointsBySource,
    syncSourceLabelValue,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])
  const syncSourceStatusCards = useMemo(() => {
    return syncSourceOptions.map((item) => {
      const sourceLabel = item.label
      const latestJobOfSource = sortedSyncJobs.find((job) => job.source === item.value) ?? null
      const checkpoints = syncSourceCheckpointsBySource[item.value] ?? []
      const metrics = latestJobOfSource
        ? t("enterprisePage.syncSourceStatus.metrics", {
            created: latestJobOfSource.created,
            updated: latestJobOfSource.updated,
            deactivated: latestJobOfSource.deactivated,
            rejected: latestJobOfSource.rejected,
          })
        : t("enterprisePage.syncSourceStatus.noResult")
      const latestEndedAt = latestJobOfSource
        ? formatDateTimeValue(latestJobOfSource.ended_at || latestJobOfSource.started_at)
        : "-"

      if (!latestJobOfSource) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.pendingConnect"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.pendingConnect.description"),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "sync" as const,
            label: t("enterprisePage.actions.goToSync"),
          },
        }
      }

      if (latestJobOfSource.status !== "completed" || latestJobOfSource.rejected > 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.needsReview"),
          statusVariant: "destructive" as const,
          description: t("enterprisePage.syncSourceStatus.needsReview.description", {
            status: latestJobOfSource.status,
            rejected: latestJobOfSource.rejected,
          }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "alerts" as const,
            label: t("enterprisePage.actions.goToAlerts"),
          },
        }
      }

      if (item.value === "scim" && latestJobOfSource.deactivated > 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.deactivatedNeedsCleanup"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.deactivatedNeedsCleanup.description", {
            deactivated: latestJobOfSource.deactivated,
          }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: deactivatedDirectoryFlowLink,
            label: t("enterprisePage.actions.goToDeactivatedCleanup"),
          },
        }
      }

      if (activeEmployeeCount === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.pendingConfirm"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.pendingConfirm.description"),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "employees" as const,
            label: t("enterprisePage.actions.viewEmployees"),
          },
        }
      }

      if (tenantGroups.length === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.readyToProceed"),
          statusVariant: "outline" as const,
          description: t("enterprisePage.syncSourceStatus.readyForGrouping.description", { activeEmployeeCount }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: directoryFlowLink,
            label: t("enterprisePage.actions.goToDirectory"),
          },
        }
      }

      if (tenantPolicies.length === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.readyToProceed"),
          statusVariant: "outline" as const,
          description: t("enterprisePage.syncSourceStatus.readyForPolicies.description", { tenantGroupsCount: tenantGroups.length }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: policiesFlowLink,
            label: t("enterprisePage.actions.goToPolicies"),
          },
        }
      }

      if (issuedPassCount === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.pendingIssuance"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.pendingIssuance.description"),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: walletFlowLink,
            label: t("enterprisePage.actions.goToWallet"),
          },
        }
      }

      return {
        source: item.value,
        title: sourceLabel,
        statusLabel: t("enterprisePage.status.closedLoop"),
        statusVariant: "outline" as const,
        description: t("enterprisePage.syncSourceStatus.closedLoop.description", { issuedPassCount }),
        metrics,
        latestEndedAt,
        checkpoints,
        action: {
          kind: "route" as const,
          to: walletFlowLink,
          label: t("enterprisePage.actions.viewIssuanceCenter"),
        },
      }
    })
  }, [
    activeEmployeeCount,
    deactivatedDirectoryFlowLink,
    directoryFlowLink,
    formatDateTimeValue,
    issuedPassCount,
    policiesFlowLink,
    sortedSyncJobs,
    syncSourceCheckpointsBySource,
    syncSourceOptions,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])

  useEffect(() => {
    if (!platformViewer) {
      setSelectedTenantID(viewerTenantID)
      return
    }

    const items = tenantsQuery.data ?? []
    setTenants(items)
    setSelectedTenantID((current) => {
      if (enterpriseRouteHints.tenantID && items.some((item) => item.id === enterpriseRouteHints.tenantID)) {
        return enterpriseRouteHints.tenantID
      }
      return current || items[0]?.id || ""
    })
  }, [enterpriseRouteHints.tenantID, platformViewer, tenantsQuery.data, viewerTenantID])

  useEffect(() => {
    setLatestWebhookReceiptBatchProcessResult(null)
    setLatestWebhookDLQBatchReplayResult(null)
  }, [selectedTenantID])

  useEffect(() => {
    const effectiveTenantID = selectedTenantID.trim()
    if (!effectiveTenantID) {
      setEmployees([])
      setHRISConnectors([])
      setHRISSecrets([])
      setSyncJobs([])
      setSyncRequests([])
      setWorkerAlertSubscription(null)
      setApprovals([])
      setWorkerAlerts([])
      setWorkerAlertEvents([])
      setWorkerAlertNotifications([])
      setWorkerAlertNotificationFilterCounts(emptyWorkerAlertNotificationFilterCounts)
      setWorkerAlertNotificationStatusCounts(emptyWorkerAlertNotificationStatusCounts)
      setWorkerAlertNotificationTotal(0)
      setWorkerAlertNotificationHasMore(false)
      setHRISWebhookExecutions([])
      setHRISWebhookExecutionStatusCounts(emptyHRISWebhookExecutionStatusCounts)
      setHRISWebhookExecutionQueueCounts(null)
      setHRISWebhookExecutionTotal(0)
      setHRISWebhookExecutionHasMore(false)
      setHRISWebhookExecutionNextOffset(null)
      setSelectedHRISWebhookExecution(null)
      setSelectedHRISWebhookExecutionLoading(false)
      setSelectedHRISWebhookExecutionError("")
      setHRISWebhookReceipts([])
      setHRISWebhookReceiptQueueCounts(null)
      setHRISWebhookReceiptTotal(0)
      setHRISWebhookReceiptHasMore(false)
      setHRISWebhookReceiptNextOffset(null)
      setHRISWebhookDLQEntries([])
      setHRISWebhookDLQReplayCounts(null)
      setHRISWebhookDLQTotal(0)
      setHRISWebhookDLQHasMore(false)
      setHRISWebhookDLQNextOffset(null)
      setHRISPullStates([])
      setIDPConfig(null)
      setUserGroups([])
      setPolicies([])
      setIssuedPasses([])
      setHRISWebhookExecutionListLoading(false)
      setHRISWebhookExecutionListLoadingMore(false)
      setHRISWebhookReceiptListLoading(false)
      setHRISWebhookReceiptListLoadingMore(false)
      setHRISWebhookDLQListLoading(false)
      setHRISWebhookDLQListLoadingMore(false)
      return
    }

    if (!enterpriseDataQuery.data) {
      return
    }

    setEmployees(enterpriseDataQuery.data.employees)
    setHRISConnectors(enterpriseDataQuery.data.hrisConnectors)
    setHRISSecrets(enterpriseDataQuery.data.hrisSecrets)
    setSyncJobs(enterpriseDataQuery.data.syncJobs)
    setSyncRequests(enterpriseDataQuery.data.syncRequests)
    setWorkerAlertSubscription(enterpriseDataQuery.data.workerAlertSubscription)
    setApprovals(enterpriseDataQuery.data.approvals)
    setWorkerAlerts(enterpriseDataQuery.data.workerAlerts)
    setWorkerAlertEvents(enterpriseDataQuery.data.workerAlertEvents)
    setHRISPullStates(enterpriseDataQuery.data.hrisPullStates)
    setIDPConfig(enterpriseDataQuery.data.idpConfig)
    setUserGroups(enterpriseDataQuery.data.userGroups)
    setPolicies(enterpriseDataQuery.data.policies)
    setIssuedPasses(enterpriseDataQuery.data.issuedPasses)
  }, [enterpriseDataQuery.data, selectedTenantID])

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setWorkerAlertNotificationListLoading(true)
    setError("")
    void listEnterpriseSyncWorkerAlertNotifications(
      token,
      buildWorkerAlertNotificationListOptions({
        tenantID,
        filter: workerAlertNotificationFilter,
        query: workerAlertNotificationQuery,
        limit: workerAlertNotificationPageSize,
      })
    )
      .then((response) => {
        if (cancelled) {
          return
        }
        setWorkerAlertNotifications(response.items)
        setWorkerAlertNotificationFilterCounts(response.filter_counts)
        setWorkerAlertNotificationStatusCounts(response.status_counts)
        setWorkerAlertNotificationTotal(response.total)
        setWorkerAlertNotificationHasMore(response.has_more)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWorkerAlertNotificationsFailed")
        setError(message)
        setWorkerAlertNotifications([])
        setWorkerAlertNotificationFilterCounts(emptyWorkerAlertNotificationFilterCounts)
        setWorkerAlertNotificationStatusCounts(emptyWorkerAlertNotificationStatusCounts)
        setWorkerAlertNotificationTotal(0)
        setWorkerAlertNotificationHasMore(false)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setWorkerAlertNotificationListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [selectedTenantID, t, token, workerAlertNotificationFilter, workerAlertNotificationQuery])

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setHRISWebhookExecutionListLoading(true)
    setError("")
    void fetchHRISWebhookExecutionsPage(tenantID)
      .then((response) => {
        if (cancelled) {
          return
        }
        applyHRISWebhookExecutionPage(response)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookExecutionsFailed")
        setError(message)
        setHRISWebhookExecutions([])
        setHRISWebhookExecutionStatusCounts(emptyHRISWebhookExecutionStatusCounts)
        setHRISWebhookExecutionQueueCounts(null)
        setHRISWebhookExecutionTotal(0)
        setHRISWebhookExecutionHasMore(false)
        setHRISWebhookExecutionNextOffset(null)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setHRISWebhookExecutionListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [
    hrisWebhookExecutionDispatchFilter,
    hrisWebhookExecutionKindFilter,
    hrisWebhookExecutionModeFilter,
    hrisWebhookExecutionQuery,
    hrisWebhookExecutionQueueStateFilter,
    hrisWebhookExecutionReplayScopeFilter,
    hrisWebhookExecutionStatusFilter,
    hrisWebhookExecutionTargetStatusFilter,
    selectedTenantID,
    t,
    token,
  ])

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    const executionID = enterpriseRouteHints.executionID.trim()
    if (!tenantID || !executionID) {
      setSelectedHRISWebhookExecution(null)
      setSelectedHRISWebhookExecutionLoading(false)
      setSelectedHRISWebhookExecutionError("")
      return
    }
    let cancelled = false
    setSelectedHRISWebhookExecutionLoading(true)
    setSelectedHRISWebhookExecutionError("")
    void getEnterpriseHRISWebhookExecution(token, executionID, {
      tenant_id: tenantID,
    })
      .then((response) => {
        if (cancelled) {
          return
        }
        setSelectedHRISWebhookExecution(response.item)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookExecutionDetailFailed")
        setSelectedHRISWebhookExecution(null)
        setSelectedHRISWebhookExecutionError(message)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setSelectedHRISWebhookExecutionLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [enterpriseRouteHints.executionID, selectedTenantID, t, token])

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setHRISWebhookReceiptListLoading(true)
    setError("")
    void fetchHRISWebhookReceiptsPage(tenantID)
      .then((response) => {
        if (cancelled) {
          return
        }
        applyHRISWebhookReceiptPage(response)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookReceiptsFailed")
        setError(message)
        setHRISWebhookReceipts([])
        setHRISWebhookReceiptQueueCounts(null)
        setHRISWebhookReceiptTotal(0)
        setHRISWebhookReceiptHasMore(false)
        setHRISWebhookReceiptNextOffset(null)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setHRISWebhookReceiptListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [selectedTenantID, t, token])

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setHRISWebhookDLQListLoading(true)
    setError("")
    void fetchHRISWebhookDLQPage(tenantID)
      .then((response) => {
        if (cancelled) {
          return
        }
        applyHRISWebhookDLQPage(response)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookDLQFailed")
        setError(message)
        setHRISWebhookDLQEntries([])
        setHRISWebhookDLQReplayCounts(null)
        setHRISWebhookDLQTotal(0)
        setHRISWebhookDLQHasMore(false)
        setHRISWebhookDLQNextOffset(null)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setHRISWebhookDLQListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [selectedTenantID, t, token])

  async function reloadEnterpriseData(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    if (effectiveTenantID !== selectedTenantID.trim()) {
      setSelectedTenantID(effectiveTenantID)
      return
    }
    await enterpriseDataQuery.refetch()
  }

  async function reloadWorkerAlertNotificationHistory(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setWorkerAlertNotificationListLoading(true)
    try {
      const response = await listEnterpriseSyncWorkerAlertNotifications(
        token,
        buildWorkerAlertNotificationListOptions({
          tenantID: effectiveTenantID,
          filter: workerAlertNotificationFilter,
          query: workerAlertNotificationQuery,
          limit: workerAlertNotificationPageSize,
        })
      )
      setWorkerAlertNotifications(response.items)
      setWorkerAlertNotificationFilterCounts(response.filter_counts)
      setWorkerAlertNotificationStatusCounts(response.status_counts)
      setWorkerAlertNotificationTotal(response.total)
      setWorkerAlertNotificationHasMore(response.has_more)
    } finally {
      setWorkerAlertNotificationListLoading(false)
    }
  }

  async function reloadHRISWebhookExecutions(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setHRISWebhookExecutionListLoading(true)
    try {
      const response = await fetchHRISWebhookExecutionsPage(effectiveTenantID)
      applyHRISWebhookExecutionPage(response)
    } finally {
      setHRISWebhookExecutionListLoading(false)
    }
  }

  async function reloadHRISWebhookReceipts(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setHRISWebhookReceiptListLoading(true)
    try {
      const response = await fetchHRISWebhookReceiptsPage(effectiveTenantID)
      applyHRISWebhookReceiptPage(response)
    } finally {
      setHRISWebhookReceiptListLoading(false)
    }
  }

  async function reloadHRISWebhookDLQ(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setHRISWebhookDLQListLoading(true)
    try {
      const response = await fetchHRISWebhookDLQPage(effectiveTenantID)
      applyHRISWebhookDLQPage(response)
    } finally {
      setHRISWebhookDLQListLoading(false)
    }
  }

  async function onLoadMoreWorkerAlertNotifications() {
    const tenantID = selectedTenantID.trim()
    if (
      !tenantID ||
      workerAlertNotificationListLoading ||
      workerAlertNotificationListLoadingMore ||
      !workerAlertNotificationHasMore
    ) {
      return
    }
    setWorkerAlertNotificationListLoadingMore(true)
    setError("")
    try {
      const response = await listEnterpriseSyncWorkerAlertNotifications(
        token,
        buildWorkerAlertNotificationListOptions({
          tenantID,
          filter: workerAlertNotificationFilter,
          query: workerAlertNotificationQuery,
          limit: workerAlertNotificationPageSize,
          offset: workerAlertNotifications.length,
        })
      )
      setWorkerAlertNotifications((current) => [...current, ...response.items])
      setWorkerAlertNotificationFilterCounts(response.filter_counts)
      setWorkerAlertNotificationStatusCounts(response.status_counts)
      setWorkerAlertNotificationTotal(response.total)
      setWorkerAlertNotificationHasMore(response.has_more)
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWorkerAlertNotificationsFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationListLoadingMore(false)
    }
  }

  async function onLoadMoreHRISWebhookExecutions() {
    const tenantID = selectedTenantID.trim()
    const nextOffset = hrisWebhookExecutionNextOffset ?? hrisWebhookExecutions.length
    if (
      !tenantID ||
      hrisWebhookExecutionListLoading ||
      hrisWebhookExecutionListLoadingMore ||
      !hrisWebhookExecutionHasMore ||
      nextOffset < 0
    ) {
      return
    }
    setHRISWebhookExecutionListLoadingMore(true)
    setError("")
    try {
      const response = await fetchHRISWebhookExecutionsPage(tenantID, nextOffset)
      applyHRISWebhookExecutionPage(response, "append")
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWebhookExecutionsFailed")
      setError(message)
    } finally {
      setHRISWebhookExecutionListLoadingMore(false)
    }
  }

  async function onLoadMoreHRISWebhookReceipts() {
    const tenantID = selectedTenantID.trim()
    const nextOffset = hrisWebhookReceiptNextOffset ?? hrisWebhookReceipts.length
    if (
      !tenantID ||
      hrisWebhookReceiptListLoading ||
      hrisWebhookReceiptListLoadingMore ||
      !hrisWebhookReceiptHasMore ||
      nextOffset < 0
    ) {
      return
    }
    setHRISWebhookReceiptListLoadingMore(true)
    setError("")
    try {
      const response = await fetchHRISWebhookReceiptsPage(tenantID, nextOffset)
      applyHRISWebhookReceiptPage(response, "append")
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWebhookReceiptsFailed")
      setError(message)
    } finally {
      setHRISWebhookReceiptListLoadingMore(false)
    }
  }

  async function onLoadMoreHRISWebhookDLQ() {
    const tenantID = selectedTenantID.trim()
    const nextOffset = hrisWebhookDLQNextOffset ?? hrisWebhookDLQEntries.length
    if (
      !tenantID ||
      hrisWebhookDLQListLoading ||
      hrisWebhookDLQListLoadingMore ||
      !hrisWebhookDLQHasMore ||
      nextOffset < 0
    ) {
      return
    }
    setHRISWebhookDLQListLoadingMore(true)
    setError("")
    try {
      const response = await fetchHRISWebhookDLQPage(tenantID, nextOffset)
      applyHRISWebhookDLQPage(response, "append")
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWebhookDLQFailed")
      setError(message)
    } finally {
      setHRISWebhookDLQListLoadingMore(false)
    }
  }

  function onWorkerAlertNotificationHistoryViewChange(input: {
    filter: NotificationHistoryFilter
    query: string
  }) {
    setWorkerAlertNotificationFilter((current) => (current === input.filter ? current : input.filter))
    setWorkerAlertNotificationQuery((current) => (current === input.query ? current : input.query))
  }

  function onHRISWebhookExecutionHistoryViewChange(input: {
    kind: HRISWebhookExecutionKindFilter
    status: HRISWebhookExecutionStatusFilter
    queueState: HRISWebhookExecutionQueueStateFilter
    replayScope: HRISWebhookExecutionReplayScopeFilter
    executionMode: HRISWebhookExecutionModeFilter
    dispatchMode: HRISWebhookExecutionDispatchFilter
    targetStatus: string
    query: string
  }) {
    setHRISWebhookExecutionKindFilter((current) => (current === input.kind ? current : input.kind))
    setHRISWebhookExecutionStatusFilter((current) => (current === input.status ? current : input.status))
    setHRISWebhookExecutionQueueStateFilter((current) =>
      current === input.queueState ? current : input.queueState
    )
    setHRISWebhookExecutionReplayScopeFilter((current) =>
      current === input.replayScope ? current : input.replayScope
    )
    setHRISWebhookExecutionModeFilter((current) => (current === input.executionMode ? current : input.executionMode))
    setHRISWebhookExecutionDispatchFilter((current) =>
      current === input.dispatchMode ? current : input.dispatchMode
    )
    setHRISWebhookExecutionTargetStatusFilter((current) =>
      current === input.targetStatus ? current : input.targetStatus
    )
    setHRISWebhookExecutionQuery((current) => (current === input.query ? current : input.query))
  }

  function onSelectHRISWebhookExecution(executionID: string | null) {
    const nextExecutionID = executionID?.trim() || ""
    const query = new URLSearchParams(location.search)
    if (nextExecutionID) {
      query.set("execution_id", nextExecutionID)
      if (selectedTenantID.trim()) {
        query.set("tenant_id", selectedTenantID.trim())
      }
      const currentAlertsViewHint = query.get("alerts_view_hint")?.trim() || ""
      if (currentAlertsViewHint !== "overview" && currentAlertsViewHint !== "directory_exceptions") {
        query.set("alerts_view_hint", "directory_exceptions")
      }
    } else {
      query.delete("execution_id")
    }
    const nextSearch = query.toString()
    navigate({
      pathname: location.pathname,
      search: nextSearch ? `?${nextSearch}` : "",
      hash: "#alerts",
    })
  }

  async function onExportWorkerAlertNotifications(input: {
    filter: NotificationHistoryFilter
    query: string
  }) {
    const tenantID = selectedTenantID.trim()
    if (!tenantID || workerAlertNotificationExporting) {
      return
    }
    setWorkerAlertNotificationExporting(true)
    setError("")
    try {
      const csvContent = await exportEnterpriseSyncWorkerAlertNotificationsCSV(
        token,
        buildWorkerAlertNotificationListOptions({
          tenantID,
          filter: input.filter,
          query: input.query,
          limit: workerAlertNotificationPageSize,
        })
      )
      const stamp = new Date().toISOString().replace(/[:.]/g, "-")
      const scope = (selectedTenant?.name || tenantID).trim().replace(/\s+/g, "-").toLowerCase()
      downloadWorkerAlertNotificationCSV(
        `enterprise-sync-worker-alert-notifications-${scope || "tenant"}-${stamp}.csv`,
        csvContent
      )
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertNotificationExported", {
          count: workerAlertNotificationTotal,
        })
      )
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.exportWorkerAlertNotificationsFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationExporting(false)
    }
  }

  async function onSyncEmployees(payload: { source: string; requestID: string; payload: string }) {
    if (!writable || !selectedTenantID.trim()) {
      return
    }

    const source = payload.source.trim()
    const requestID = payload.requestID.trim()
    const rawPayload = payload.payload

    setSyncSource(source)
    setSyncRequestID(requestID)
    setSyncPayload(rawPayload)

    let employeesToSync: EmployeeSyncInput[]
    try {
      const parsed = JSON.parse(rawPayload) as EmployeeSyncInput[]
      employeesToSync = Array.isArray(parsed) ? parsed : []
    } catch {
      setError(t("enterprisePage.errors.invalidSyncPayload"))
      return
    }

    if (employeesToSync.length === 0) {
      setError(t("enterprisePage.errors.syncAtLeastOne"))
      return
    }

    setSyncing(true)
    setError("")
    setSyncSummary("")
    try {
      const result = await syncEnterpriseEmployees(token, {
        tenant_id: selectedTenantID.trim(),
        source,
        actor: viewer.email,
        request_id: requestID || undefined,
        employees: employeesToSync,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.submitted", {
          jobID: result.job.id,
          created: result.job.created,
          updated: result.job.updated,
          accessSyncCreated: result.access_sync.created,
        })
      )
      setSyncRequestID(`sync-${Date.now()}`)
      const [employeeItems, syncJobItems] = await Promise.all([
        listEnterpriseEmployees(token, selectedTenantID.trim()),
        listEnterpriseSyncJobs(token, selectedTenantID.trim()),
      ])
      setEmployees(employeeItems)
      setSyncJobs(syncJobItems)
      const followUp =
        result.job.rejected > 0
          ? syncSourceFailureHint(t, source, result.job.rejected)
          : employeeItems.length === 0
            ? t("enterprisePage.syncSummary.submittedButEmpty")
            : syncSourceSuccessHint(t, source, result.job.deactivated)
      setSyncSummary(
        t("enterprisePage.syncSummary.submittedWithFollowUp", {
          jobID: result.job.id,
          created: result.job.created,
          updated: result.job.updated,
          accessSyncCreated: result.access_sync.created,
          followUp,
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.submitSyncFailed")
      setError(message)
    } finally {
      setSyncing(false)
    }
  }

  async function onSaveTalentaConnector(input: TalentaConnectorSaveInput) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID) {
      return
    }

    setError("")
    setSyncSummary("")
    try {
      const existingConnector = hrisConnectors.find((item) => item.vendor === "talenta") ?? null
      if (existingConnector) {
        await updateEnterpriseHRISConnector(token, existingConnector.id, {
          tenant_id: tenantID,
          status: input.status,
          sync_strategy: input.sync_strategy,
          credential_ref: input.credential_ref,
          credential_value: input.credential_value,
          webhook_secret_ref: input.webhook_secret_ref,
          webhook_secret_value: input.webhook_secret_value,
          updated_by: viewer.email,
        })
      } else {
        await createEnterpriseHRISConnector(token, {
          tenant_id: tenantID,
          vendor: "talenta",
          status: input.status,
          sync_strategy: input.sync_strategy,
          credential_ref: input.credential_ref,
          credential_value: input.credential_value,
          webhook_secret_ref: input.webhook_secret_ref,
          webhook_secret_value: input.webhook_secret_value,
          updated_by: viewer.email,
        })
      }
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.talentaConnectorSaved", {
          action: existingConnector
            ? t("enterprisePage.syncSummary.talentaConnectorAction.updated")
            : t("enterprisePage.syncSummary.talentaConnectorAction.created"),
          syncStrategy: t(`enterpriseSyncWorkspace.hrisConnector.syncStrategy.${input.sync_strategy}`),
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.saveTalentaConnectorFailed")
      setError(message)
      throw err instanceof Error ? err : new Error(message)
    }
  }

  async function onReviewApproval(approvalID: string, decision: "approved" | "rejected") {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || approvalActionID) {
      return
    }

    setApprovalActionID(approvalID)
    setError("")
    try {
      await reviewEnterpriseJITProvisionApproval(token, approvalID, {
        tenant_id: tenantID,
        decision,
        reviewed_by: viewer.email,
      })
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.approvalReviewed", {
          approvalID,
          decisionLabel:
            decision === "approved"
              ? t("enterpriseAlertsWorkspace.jitApproval.actions.approve")
              : t("enterpriseAlertsWorkspace.jitApproval.actions.reject"),
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.updateApprovalFailed")
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onUpdateApprovalExternalSync(approvalID: string, status: "synced" | "failed") {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || approvalActionID) {
      return
    }

    setApprovalActionID(approvalID)
    setError("")
    try {
      await updateEnterpriseJITProvisionApprovalExternalSync(token, approvalID, {
        tenant_id: tenantID,
        status,
        external_sync_ref: `web-admin-${Date.now()}`,
        last_error: status === "failed" ? "manually marked as failed from web-admin" : "",
      })
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.externalSyncMarked", {
          approvalID,
          status: status === "synced" ? "synced" : "failed",
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.updateExternalSyncFailed")
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onBatchReviewApprovals(approvalIDs: string[], decision: "approved" | "rejected") {
    const tenantID = selectedTenantID.trim()
    const uniqueApprovalIDs = Array.from(new Set(approvalIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || approvalActionID || uniqueApprovalIDs.length === 0) {
      return
    }

    setApprovalActionID(`batch-review-${decision}-${Date.now()}`)
    setError("")
    let successCount = 0
    let failedCount = 0

    for (const approvalID of uniqueApprovalIDs) {
      try {
        await reviewEnterpriseJITProvisionApproval(token, approvalID, {
          tenant_id: tenantID,
          decision,
          reviewed_by: viewer.email,
        })
        successCount += 1
      } catch {
        failedCount += 1
      }
    }

    try {
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.batchReviewCompleted", {
          decisionLabel:
            decision === "approved"
              ? t("enterpriseAlertsWorkspace.jitApproval.actions.approve")
              : t("enterpriseAlertsWorkspace.jitApproval.actions.reject"),
          successCount,
          failedCount,
        })
      )
      if (failedCount > 0) {
        setError(t("enterprisePage.errors.batchApprovalPartialFailure"))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.batchRefreshFailed")
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onBatchUpdateApprovalExternalSync(approvalIDs: string[], status: "synced" | "failed") {
    const tenantID = selectedTenantID.trim()
    const uniqueApprovalIDs = Array.from(new Set(approvalIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || approvalActionID || uniqueApprovalIDs.length === 0) {
      return
    }

    setApprovalActionID(`batch-external-sync-${status}-${Date.now()}`)
    setError("")
    let successCount = 0
    let failedCount = 0

    for (let index = 0; index < uniqueApprovalIDs.length; index += 1) {
      const approvalID = uniqueApprovalIDs[index]
      try {
        await updateEnterpriseJITProvisionApprovalExternalSync(token, approvalID, {
          tenant_id: tenantID,
          status,
          external_sync_ref: `web-admin-batch-${Date.now()}-${index + 1}`,
          last_error: status === "failed" ? "manually marked as failed from web-admin batch action" : "",
        })
        successCount += 1
      } catch {
        failedCount += 1
      }
    }

    try {
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.batchExternalSyncCompleted", {
          status,
          successCount,
          failedCount,
        })
      )
      if (failedCount > 0) {
        setError(t("enterprisePage.errors.batchExternalSyncPartialFailure"))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.batchRefreshFailed")
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onReplayHRISWebhookDLQ(entryID: string) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || dlqActionID) {
      return
    }

    setDLQActionID(entryID)
    setError("")
    setSyncSummary("")
    try {
      const result = await replayEnterpriseHRISWebhookDLQ(token, entryID, {
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.dlqReplayQueued", {
              entryID: result.item.id,
              status: result.item.status,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.dlqReplayCompleted")
      )
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.replayDLQFailed")
      setError(message)
    } finally {
      setDLQActionID(null)
    }
  }

  async function onReplayHRISWebhookExecution(executionID: string) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || executionActionID) {
      return
    }

    setExecutionActionID(executionID)
    setError("")
    setSyncSummary("")
    try {
      const result = await replayEnterpriseHRISWebhookExecution(token, {
        tenant_id: tenantID,
        execution_id: executionID,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      if (result.execution_id?.trim()) {
        onSelectHRISWebhookExecution(result.execution_id.trim())
      }
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.webhookExecutionReplayQueued", {
              sourceExecutionID: result.source_execution_id,
              executionID: result.execution_id || "",
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.webhookExecutionReplayCompleted", {
              sourceExecutionID: result.source_execution_id,
            })
      )
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.replayWebhookExecutionFailed")
      setError(message)
    } finally {
      setExecutionActionID(null)
    }
  }

  async function onProcessHRISWebhookReceipt(receiptID: string) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || receiptActionID) {
      return
    }

    setReceiptActionID(receiptID)
    setError("")
    setSyncSummary("")
    try {
      const result = await processEnterpriseHRISWebhookReceipt(token, {
        tenant_id: tenantID,
        receipt_id: receiptID,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.webhookReceiptQueued", {
              receiptID: result.item.id,
              status: result.item.status,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.webhookReceiptProcessed", {
              receiptID: result.item.id,
              status: result.item.status,
            })
      )
      if (result.item.status !== "processed" && result.item.last_error?.trim()) {
        setError(result.item.last_error.trim())
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.processWebhookReceiptBatchFailed")
      setError(message)
    } finally {
      setReceiptActionID(null)
    }
  }

  async function onBatchReplayHRISWebhookDLQ(entryIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const normalizedEntryIDs = Array.from(new Set(entryIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || dlqActionID || normalizedEntryIDs.length === 0) {
      return
    }

    setDLQActionID("__batch__")
    setError("")
    setSyncSummary("")
    try {
      const result = await replayBatchEnterpriseHRISWebhookDLQ(token, {
        tenant_id: tenantID,
        entry_ids: normalizedEntryIDs,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setLatestWebhookDLQBatchReplayResult(result)
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.dlqBatchReplayQueued", {
              totalEntries: result.total_entries,
              queued: result.queued ?? 0,
              skipped: result.skipped,
              failed: result.failed,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.dlqBatchReplayCompleted", {
              totalEntries: result.total_entries,
              replayed: result.replayed,
              skipped: result.skipped,
              failed: result.failed,
            })
      )
      if (result.failed > 0) {
        setError(t("enterprisePage.errors.replayDLQBatchPartialFailure"))
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.replayDLQFailed")
      setError(message)
    } finally {
      setDLQActionID(null)
    }
  }

  async function onBatchProcessHRISWebhookReceipts(receiptIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const normalizedReceiptIDs = Array.from(new Set(receiptIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || receiptActionID || normalizedReceiptIDs.length === 0) {
      return
    }

    setReceiptActionID("__batch__")
    setError("")
    setSyncSummary("")
    try {
      const result = await processBatchEnterpriseHRISWebhookReceipts(token, {
        tenant_id: tenantID,
        receipt_ids: normalizedReceiptIDs,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setLatestWebhookReceiptBatchProcessResult(result)
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.webhookReceiptBatchQueued", {
              totalReceipts: result.total_receipts,
              queued: result.queued ?? 0,
              skipped: result.skipped,
              failed: result.failed,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.webhookReceiptBatchProcessCompleted", {
              totalReceipts: result.total_receipts,
              processed: result.processed,
              skipped: result.skipped,
              failed: result.failed,
              dlq: result.dlq,
            })
      )
      if (result.failed > 0) {
        setError(t("enterprisePage.errors.processWebhookReceiptBatchPartialFailure"))
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.processWebhookReceiptBatchFailed")
      setError(message)
    } finally {
      setReceiptActionID(null)
    }
  }

  async function onReconcilePendingSyncRequests() {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || syncRequestActionID) {
      return
    }

    setSyncRequestActionID("reconcile-pending")
    setError("")
    setSyncSummary("")
    try {
      const result = await reconcilePendingEnterpriseSyncRequests(token, {
        tenant_id: tenantID,
        limit: 20,
      })
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.reconcilePendingCompleted", {
          processed: result.processed,
          applied: result.applied,
          failed: result.failed,
          skippedByAttemptLimit: result.skipped_by_attempt_limit || 0,
          skippedByCooldown: result.skipped_by_cooldown || 0,
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.reconcilePendingFailed")
      setError(message)
    } finally {
      setSyncRequestActionID(null)
    }
  }

  async function onSaveWorkerAlertSubscription(payload: EnterpriseSyncWorkerAlertSubscriptionSaveInput) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || workerAlertSubscriptionActionID) {
      return
    }

    setWorkerAlertSubscriptionActionID("save-worker-alert-subscription")
    setError("")
    setSyncSummary("")
    try {
      const updated = await upsertEnterpriseSyncWorkerAlertSubscription(token, {
        tenant_id: tenantID,
        enabled: payload.enabled,
        worker_alert_threshold: payload.worker_alert_threshold,
        window_seconds: payload.window_seconds,
        cooldown_seconds: payload.cooldown_seconds,
        channels: payload.channels,
        receiver_groups: payload.receiver_groups,
      })
      setWorkerAlertSubscription(updated)
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertSubscriptionSaved", {
          threshold: updated.worker_alert_threshold,
          windowSeconds: updated.window_seconds,
          cooldownSeconds: updated.cooldown_seconds,
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.saveWorkerAlertSubscriptionFailed")
      setError(message)
    } finally {
      setWorkerAlertSubscriptionActionID(null)
    }
  }

  async function onDispatchWorkerAlerts() {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || workerAlertDispatchActionID) {
      return
    }

    setWorkerAlertDispatchActionID("dispatch-worker-alerts")
    setError("")
    setSyncSummary("")
    try {
      const result = await dispatchEnterpriseSyncWorkerAlerts(token, {
        tenant_id: tenantID,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertDispatchCompleted", {
          totalAlerts: result.total_alerts,
          dispatched: result.dispatched,
          skipped: result.skipped,
          failed: result.failed,
        })
      )
      await Promise.all([reloadEnterpriseData(tenantID), reloadWorkerAlertNotificationHistory(tenantID)])
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.dispatchWorkerAlertsFailed")
      setError(message)
    } finally {
      setWorkerAlertDispatchActionID(null)
    }
  }

  async function onRetryWorkerAlertNotification(notificationID: string) {
    const tenantID = selectedTenantID.trim()
    const nextNotificationID = notificationID.trim()
    if (
      !writable ||
      !tenantID ||
      !nextNotificationID ||
      workerAlertNotificationActionID ||
      workerAlertNotificationBatchActionID ||
      workerAlertNotificationRestoreBatchActionID ||
      workerAlertNotificationSuppressBatchActionID ||
      workerAlertNotificationAutoRetryActionID
    ) {
      return
    }

    setWorkerAlertNotificationActionID(nextNotificationID)
    setError("")
    setSyncSummary("")
    try {
      const retried = await retryEnterpriseSyncWorkerAlertNotification(token, {
        tenant_id: tenantID,
        notification_id: nextNotificationID,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertNotificationRetried", {
          status: retried.status,
          attempt: retried.attempt ?? 0,
          workerLabel: retried.worker_label || retried.worker_action || nextNotificationID,
        })
      )
      await Promise.all([reloadEnterpriseData(tenantID), reloadWorkerAlertNotificationHistory(tenantID)])
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.retryWorkerAlertNotificationFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationActionID(null)
    }
  }

  async function onAutoRetryWorkerAlertNotifications() {
    const tenantID = selectedTenantID.trim()
    if (
      !writable ||
      !tenantID ||
      workerAlertNotificationActionID ||
      workerAlertNotificationBatchActionID ||
      workerAlertNotificationRestoreBatchActionID ||
      workerAlertNotificationSuppressBatchActionID ||
      workerAlertNotificationAutoRetryActionID
    ) {
      return
    }

    setWorkerAlertNotificationAutoRetryActionID("auto-retry")
    setError("")
    setSyncSummary("")
    try {
      const result = await autoRetryEnterpriseSyncWorkerAlertNotifications(token, {
        tenant_id: tenantID,
        actor: viewer.email,
        limit: 20,
        max_attempts: 3,
        base_backoff_ms: 5 * 60 * 1000,
        max_backoff_ms: 60 * 60 * 1000,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertNotificationAutoRetried", {
          retried: result.retried,
          failed: result.failed,
          skipped: result.skipped,
          suppressed: result.suppressed,
        })
      )
      await Promise.all([reloadEnterpriseData(tenantID), reloadWorkerAlertNotificationHistory(tenantID)])
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.autoRetryWorkerAlertNotificationsFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationAutoRetryActionID(null)
    }
  }

  async function onBatchRetryWorkerAlertNotifications(notificationIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const nextNotificationIDs = Array.from(new Set(notificationIDs.map((item) => item.trim()).filter(Boolean)))
    if (
      !writable ||
      !tenantID ||
      nextNotificationIDs.length === 0 ||
      workerAlertNotificationActionID ||
      workerAlertNotificationBatchActionID ||
      workerAlertNotificationRestoreBatchActionID ||
      workerAlertNotificationSuppressBatchActionID ||
      workerAlertNotificationAutoRetryActionID
    ) {
      return
    }

    setWorkerAlertNotificationBatchActionID("retry-batch")
    setError("")
    setSyncSummary("")
    try {
      const result = await retryEnterpriseSyncWorkerAlertNotificationsBatch(token, {
        tenant_id: tenantID,
        notification_ids: nextNotificationIDs,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertNotificationBatchRetried", {
          retried: result.retried,
          failed: result.failed,
          skipped: result.skipped,
          suppressed: result.suppressed,
        })
      )
      await Promise.all([reloadEnterpriseData(tenantID), reloadWorkerAlertNotificationHistory(tenantID)])
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.retryWorkerAlertNotificationsBatchFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationBatchActionID(null)
    }
  }

  async function onBatchSuppressWorkerAlertNotifications(notificationIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const nextNotificationIDs = Array.from(new Set(notificationIDs.map((item) => item.trim()).filter(Boolean)))
    if (
      !writable ||
      !tenantID ||
      nextNotificationIDs.length === 0 ||
      workerAlertNotificationActionID ||
      workerAlertNotificationBatchActionID ||
      workerAlertNotificationRestoreBatchActionID ||
      workerAlertNotificationSuppressBatchActionID ||
      workerAlertNotificationAutoRetryActionID
    ) {
      return
    }

    setWorkerAlertNotificationSuppressBatchActionID("suppress-batch")
    setError("")
    setSyncSummary("")
    try {
      const result = await suppressEnterpriseSyncWorkerAlertNotificationsBatch(token, {
        tenant_id: tenantID,
        notification_ids: nextNotificationIDs,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertNotificationBatchSuppressed", {
          suppressed: result.suppressed,
          skipped: result.skipped,
        })
      )
      await Promise.all([reloadEnterpriseData(tenantID), reloadWorkerAlertNotificationHistory(tenantID)])
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.suppressWorkerAlertNotificationsBatchFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationSuppressBatchActionID(null)
    }
  }

  async function onBatchRestoreWorkerAlertNotifications(notificationIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const nextNotificationIDs = Array.from(new Set(notificationIDs.map((item) => item.trim()).filter(Boolean)))
    if (
      !writable ||
      !tenantID ||
      nextNotificationIDs.length === 0 ||
      workerAlertNotificationActionID ||
      workerAlertNotificationBatchActionID ||
      workerAlertNotificationRestoreBatchActionID ||
      workerAlertNotificationSuppressBatchActionID ||
      workerAlertNotificationAutoRetryActionID
    ) {
      return
    }

    setWorkerAlertNotificationRestoreBatchActionID("restore-batch")
    setError("")
    setSyncSummary("")
    try {
      const result = await restoreEnterpriseSyncWorkerAlertNotificationsBatch(token, {
        tenant_id: tenantID,
        notification_ids: nextNotificationIDs,
        actor: viewer.email,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.workerAlertNotificationBatchRestored", {
          restored: result.restored,
          skipped: result.skipped,
        })
      )
      await Promise.all([reloadEnterpriseData(tenantID), reloadWorkerAlertNotificationHistory(tenantID)])
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.restoreWorkerAlertNotificationsBatchFailed")
      setError(message)
    } finally {
      setWorkerAlertNotificationRestoreBatchActionID(null)
    }
  }

  function goToSection(section: EnterpriseSection) {
    navigate({
      pathname: location.pathname,
      search: location.search,
      hash: `#${section}`,
    })
  }
  const effectiveError = error || queryError

  return (
    <div className="space-y-6">
      <EnterprisePageHeader
        platformViewer={platformViewer}
        selectedTenantID={selectedTenantID}
        selectedTenantName={selectedTenant?.name}
        tenants={tenants}
        onTenantChange={setSelectedTenantID}
      />

      <EnterprisePageOverview
        loading={loading}
        activeEmployeeCount={activeEmployeeCount}
        syncJobsCount={syncJobs.length}
        idpStatusLabel={idpConfig?.status || t("enterprisePage.status.notConfigured")}
        pendingApprovalCount={pendingApprovalCount}
        effectiveError={effectiveError}
        syncSummary={syncSummary}
        attentionItems={attentionItems}
        quickActions={quickActions}
      />

      <EnterpriseWorkflowOverview
        loading={loading}
        tenantGroupsCount={tenantGroups.length}
        tenantPoliciesCount={tenantPolicies.length}
        issuedPassCount={issuedPassCount}
        workflowSteps={workflowStepsOverview}
        nextWorkflowAction={nextWorkflowAction}
        directoryFlowLink={directoryFlowLink}
        policiesFlowLink={policiesFlowLink}
        onGoToSection={goToSection}
      />

      <Tabs value={activeSection} onValueChange={(value) => goToSection(value as EnterpriseSection)} className="space-y-4">
        <TabsList
          className="grid w-full max-w-3xl"
          style={{ gridTemplateColumns: `repeat(${visibleSections.length}, minmax(0, 1fr))` }}
        >
          {visibleSections.map((section) => (
            <TabsTrigger key={section} value={section}>
              {enterpriseSectionLabel(section, t)}
            </TabsTrigger>
          ))}
        </TabsList>

        <EnterpriseEmployeesWorkspace
          directoryLink={directoryFlowLink}
          employees={employees}
          formatDateTime={formatDateTimeValue}
          loading={loading}
          statusBadgeVariant={statusBadgeVariant}
        />

        <EnterpriseSyncWorkspace
          activeEmployeeCount={activeEmployeeCount}
          apiBaseURL={API_BASE_URL}
          alertsLink={enterpriseAlertsFlowLink}
          directoryLink={directoryFlowLink}
          failedSyncJobCount={failedSyncJobCount}
          formatDateTime={formatDateTimeValue}
          goToSection={goToSection}
          hrisConnectors={hrisConnectors}
          hrisSecrets={hrisSecrets}
          initialFilterContextKey={syncRouteHints.hasHints ? syncRouteHints.contextKey : undefined}
          initialFocusHint={syncRouteHints.focusHint}
          initialWorkerFilter={syncRouteHints.workerFilter}
          initialWorkerAction={syncRouteHints.workerAction}
          initialWorkerConnectorID={syncRouteHints.workerConnectorID}
          initialWorkerFailureStage={syncRouteHints.workerFailureStage}
          initialWorkerKind={syncRouteHints.workerKind}
          initialWorkerLabel={syncRouteHints.workerLabel}
          initialWorkerMode={syncRouteHints.workerMode}
          initialWorkerQueueState={syncRouteHints.workerQueueState}
          initialWorkerRequestID={syncRouteHints.workerRequestID}
          initialWorkerReplayState={syncRouteHints.workerReplayState}
          initialWorkerReviewStage={syncRouteHints.workerReviewStage}
          initialWorkerReviewStatus={syncRouteHints.workerReviewStatus}
          initialWorkerQuery={syncRouteHints.workerQuery}
          initialWorkerStatus={syncRouteHints.workerStatus}
          initialWorkerVendor={syncRouteHints.workerVendor}
          initialExecutionID={syncRouteHints.executionID}
          initialExecutionKind={syncRouteHints.executionKind}
          initialExecutionMode={syncRouteHints.executionMode}
          initialExecutionQueueState={syncRouteHints.executionQueueState}
          initialExecutionReplayScope={syncRouteHints.executionReplayScope}
          initialExecutionStatus={syncRouteHints.executionStatus}
          latestSyncJob={latestSyncJob}
          loading={loading}
          onSaveTalentaConnector={onSaveTalentaConnector}
          onSyncEmployees={onSyncEmployees}
          onSyncPayloadChange={setSyncPayload}
          onSyncRequestIDChange={setSyncRequestID}
          onSyncSourceChange={setSyncSource}
          sampleSyncPayload={sampleSyncPayload}
          sortedSyncJobs={sortedSyncJobs}
          statusBadgeVariant={statusBadgeVariant}
          syncIssueCards={syncIssueCards}
          syncOutcomeAction={syncOutcomeAction}
          syncPayload={syncPayload}
          syncRemediationSteps={syncRemediationSteps}
          syncRequestID={syncRequestID}
          syncSource={syncSource}
          syncSourceFeedback={syncSourceFeedback}
          syncSourceStatusCards={syncSourceStatusCards}
          syncSourceLabel={syncSourceLabelValue}
          syncSourceOptions={syncSourceOptions}
          syncing={syncing}
          tenantGroupsCount={tenantGroups.length}
          tenantPoliciesCount={tenantPolicies.length}
          issuedPassCount={issuedPassCount}
          policiesLink={policiesFlowLink}
          selectedTenantName={selectedTenant?.name}
          syncLink={enterpriseSyncFlowLink}
          workflowStateLabel={(state) => workflowStateLabel(t, state)}
          workflowStateVariant={workflowStateVariant}
          workerAlerts={workerAlerts}
          walletLink={walletFlowLink}
          writable={writable}
        />

        <EnterpriseIDPWorkspace
          activeEmployeeCount={activeEmployeeCount}
          directoryLink={directoryFlowLink}
          failedSyncJobCount={failedSyncJobCount}
          formatDateTime={formatDateTimeValue}
          goToSection={goToSection}
          idpConfig={idpConfig}
          idpOutcomeAction={idpOutcomeAction}
          idpReady={idpReady}
          loading={loading}
          pendingApprovalCount={pendingApprovalCount}
          policiesLink={policiesFlowLink}
          syncJobsCount={syncJobs.length}
          workerAlertCount={workerAlertCount}
        />

        <EnterpriseSCIMWorkspace token={token} />

        <EnterpriseAlertsWorkspace
          alertRecoveryAction={alertRecoveryAction}
          approvals={approvals}
          approvalActionID={approvalActionID}
          approvalActionBusy={approvalActionID !== null}
          attentionItems={attentionItems}
          directoryLink={directoryFlowLink}
          dlqActionBusy={dlqActionID !== null}
          dlqActionID={dlqActionID}
          formatDateTime={formatDateTimeValue}
          goToSection={goToSection}
          landingCards={alertLandingCards}
          loading={loading}
          onProcessHRISWebhookReceipt={onProcessHRISWebhookReceipt}
          onBatchProcessHRISWebhookReceipts={onBatchProcessHRISWebhookReceipts}
          onBatchReviewApprovals={onBatchReviewApprovals}
          onBatchUpdateApprovalExternalSync={onBatchUpdateApprovalExternalSync}
          onBatchReplayHRISWebhookDLQ={onBatchReplayHRISWebhookDLQ}
          onReconcilePendingSyncRequests={onReconcilePendingSyncRequests}
          onReplayHRISWebhookDLQ={onReplayHRISWebhookDLQ}
          onReviewApproval={onReviewApproval}
          onUpdateApprovalExternalSync={onUpdateApprovalExternalSync}
          syncLink={enterpriseSyncFlowLink}
          initialFilterContextKey={alertsRouteHints.hasHints ? alertsRouteHints.contextKey : undefined}
          initialLandingView={alertsRouteHints.landingView}
          initialApprovalQuery={alertsRouteHints.approvalQuery}
          initialDirectoryQuery={alertsRouteHints.directoryQuery}
          initialSegmentHint={alertsRouteHints.segmentHint}
          initialSegmentStatus={alertsRouteHints.segmentStatus}
          initialWorkerAction={alertsRouteHints.workerAction}
          initialWorkerFilter={alertsRouteHints.workerFilter}
          initialWorkerKind={alertsRouteHints.workerKind}
          initialWorkerLabel={alertsRouteHints.workerLabel}
          initialWorkerQueueState={alertsRouteHints.workerQueueState}
          initialWorkerReplayState={alertsRouteHints.workerReplayState}
          initialExecutionKind={alertsRouteHints.executionKind}
          initialExecutionMode={alertsRouteHints.executionMode}
          initialExecutionQueueState={alertsRouteHints.executionQueueState}
          initialExecutionReplayScope={alertsRouteHints.executionReplayScope}
          initialExecutionStatus={alertsRouteHints.executionStatus}
          initialSyncSourceFilter={alertsRouteHints.syncSource}
          initialSyncStatusFilter={alertsRouteHints.syncStatus}
          initialWorkerStatus={alertsRouteHints.workerStatus}
          policiesLink={policiesFlowLink}
          latestWebhookDLQBatchReplayResult={latestWebhookDLQBatchReplayResult}
          latestWebhookReceiptBatchProcessResult={latestWebhookReceiptBatchProcessResult}
          receiptActionID={receiptActionID}
          receiptActionBusy={receiptActionID !== null}
          selectedTenantName={selectedTenant?.name}
          statusBadgeVariant={statusBadgeVariant}
          syncRequestActionBusy={syncRequestActionID !== null}
          syncRequests={syncRequests}
          syncJobs={syncJobs}
          writable={writable}
          workerAlertSubscription={workerAlertSubscription}
          workerAlertDispatching={workerAlertDispatchActionID !== null}
          workerAlertSubscriptionSaving={workerAlertSubscriptionActionID !== null}
          walletLink={walletFlowLink}
          workerAlerts={workerAlerts}
          workerAlertEvents={workerAlertEvents}
          workerAlertNotifications={workerAlertNotifications}
          workerAlertNotificationTotal={workerAlertNotificationTotal}
          workerAlertNotificationFilterCounts={workerAlertNotificationFilterCounts}
          workerAlertNotificationStatusCounts={workerAlertNotificationStatusCounts}
          workerAlertNotificationLoading={workerAlertNotificationListLoading}
          workerAlertNotificationLoadingMore={workerAlertNotificationListLoadingMore}
          workerAlertNotificationHasMore={workerAlertNotificationHasMore}
          exportingWorkerAlertNotifications={workerAlertNotificationExporting}
          onWorkerAlertNotificationHistoryViewChange={onWorkerAlertNotificationHistoryViewChange}
          onLoadMoreWorkerAlertNotifications={onLoadMoreWorkerAlertNotifications}
          onExportWorkerAlertNotifications={onExportWorkerAlertNotifications}
          hrisWebhookExecutions={hrisWebhookExecutions}
          hrisWebhookExecutionTotal={hrisWebhookExecutionTotal}
          hrisWebhookExecutionStatusCounts={hrisWebhookExecutionStatusCounts}
          hrisWebhookExecutionQueueCounts={hrisWebhookExecutionQueueCounts}
          hrisWebhookExecutionLoading={hrisWebhookExecutionListLoading}
          hrisWebhookExecutionLoadingMore={hrisWebhookExecutionListLoadingMore}
          hrisWebhookExecutionHasMore={hrisWebhookExecutionHasMore}
          selectedTenantID={selectedTenantID}
          selectedHRISWebhookExecutionID={selectedHRISWebhookExecutionID}
          selectedHRISWebhookExecution={selectedHRISWebhookExecutionItem}
          selectedHRISWebhookExecutionLoading={selectedHRISWebhookExecutionLoading}
          selectedHRISWebhookExecutionError={selectedHRISWebhookExecutionError}
          executionActionID={executionActionID}
          onHRISWebhookExecutionHistoryViewChange={onHRISWebhookExecutionHistoryViewChange}
          onLoadMoreHRISWebhookExecutions={onLoadMoreHRISWebhookExecutions}
          onReplayHRISWebhookExecution={onReplayHRISWebhookExecution}
          onSelectHRISWebhookExecution={onSelectHRISWebhookExecution}
          retryingWorkerAlertNotificationID={workerAlertNotificationActionID}
          retryingWorkerAlertNotificationBatch={workerAlertNotificationBatchActionID !== null}
          restoringWorkerAlertNotificationBatch={workerAlertNotificationRestoreBatchActionID !== null}
          suppressingWorkerAlertNotificationBatch={workerAlertNotificationSuppressBatchActionID !== null}
          autoRetryingWorkerAlertNotifications={workerAlertNotificationAutoRetryActionID !== null}
          hrisWebhookReceipts={hrisWebhookReceipts}
          hrisWebhookReceiptTotal={hrisWebhookReceiptTotal}
          hrisWebhookReceiptQueueCounts={hrisWebhookReceiptQueueCounts}
          hrisWebhookReceiptLoading={hrisWebhookReceiptListLoading}
          hrisWebhookReceiptLoadingMore={hrisWebhookReceiptListLoadingMore}
          hrisWebhookReceiptHasMore={hrisWebhookReceiptHasMore}
          onLoadMoreHRISWebhookReceipts={onLoadMoreHRISWebhookReceipts}
          hrisWebhookDLQEntries={hrisWebhookDLQEntries}
          hrisWebhookDLQTotal={hrisWebhookDLQTotal}
          hrisWebhookDLQReplayCounts={hrisWebhookDLQReplayCounts}
          hrisWebhookDLQLoading={hrisWebhookDLQListLoading}
          hrisWebhookDLQLoadingMore={hrisWebhookDLQListLoadingMore}
          hrisWebhookDLQHasMore={hrisWebhookDLQHasMore}
          onLoadMoreHRISWebhookDLQ={onLoadMoreHRISWebhookDLQ}
          hrisPullStates={hrisPullStates}
          onDispatchWorkerAlerts={onDispatchWorkerAlerts}
          onRetryWorkerAlertNotification={onRetryWorkerAlertNotification}
          onAutoRetryWorkerAlertNotifications={onAutoRetryWorkerAlertNotifications}
          onBatchRetryWorkerAlertNotifications={onBatchRetryWorkerAlertNotifications}
          onBatchRestoreWorkerAlertNotifications={onBatchRestoreWorkerAlertNotifications}
          onBatchSuppressWorkerAlertNotifications={onBatchSuppressWorkerAlertNotifications}
          onSaveWorkerAlertSubscription={onSaveWorkerAlertSubscription}
        />
      </Tabs>
    </div>
  )
}
