import { Fragment, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import {
  EnterpriseSyncWorkerAlertSubscriptionCard,
  type EnterpriseSyncWorkerAlertSubscriptionSaveInput,
} from "@/components/enterprise/enterprise-sync-worker-alert-subscription-card"
import {
  classifyEnterpriseSyncWorkerAlertLevel,
  describeEnterpriseSyncWorkerAlertGuidance,
} from "@/components/enterprise/enterprise-sync-worker-alert-guidance"
import { EnterpriseHRISDLQ } from "@/components/enterprise/enterprise-hris-dlq"
import { EnterpriseHRISReceipts } from "@/components/enterprise/enterprise-hris-receipts"
import { EnterpriseJITApprovalInbox } from "@/components/enterprise/enterprise-jit-approval-inbox"
import { EnterpriseSyncExceptions } from "@/components/enterprise/enterprise-sync-exceptions"
import { Badge } from "@/components/ui/badge"
import { EnterpriseWorkerAlerts } from "@/components/enterprise/enterprise-worker-alerts"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { TabsContent } from "@/components/ui/tabs"
import {
  type EnterpriseHRISPullState,
  type EnterpriseHRISWebhookDLQBatchReplayResult,
  type EnterpriseHRISWebhookExecution,
  type EnterpriseHRISWebhookExecutionStatusCounts,
  type EnterpriseHRISWebhookReceipt,
  type EnterpriseHRISWebhookReceiptBatchProcessResult,
  type EnterpriseHRISWebhookDLQEntry,
  type EnterpriseHRISWebhookRuntimeCounts,
  type EnterpriseJITProvisionApproval,
  type EnterpriseSyncRequestRecord,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertNotificationFilterCounts,
  type EnterpriseSyncWorkerAlertNotification,
  type EnterpriseSyncWorkerAlertNotificationStatusCounts,
  type EnterpriseSyncWorkerAlertSubscription,
  type EnterpriseSyncWorkerAlertItem,
  type EnterpriseSyncWorkerAlertSummaryItem,
} from "@/lib/api"

type EnterpriseSection = "employees" | "sync" | "idp" | "alerts"
type AlertLandingView = "overview" | "approval_backlog" | "directory_exceptions"
type AlertSegmentHint = "receipt_recovery"
type AlertSegmentStatus = "pending" | "attention" | "ready"
type NotificationHistoryFilter = "all" | "failed" | "retryable" | "suppressed" | "due_now"
type HRISWebhookExecutionKindFilter = "all" | "receipt_process" | "dlq_replay"
type HRISWebhookExecutionStatusFilter = "all" | "queued" | "running" | "succeeded" | "failed"
type HRISWebhookExecutionQueueStateFilter = "all" | "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"
type HRISWebhookExecutionReplayScopeFilter = "all" | "replayed" | "worker_required"
type HRISWebhookExecutionModeFilter = "all" | "inline" | "queued"
type HRISWebhookExecutionDispatchFilter = "all" | "worker_tick" | "worker_task_channel" | "goroutine_fallback"

type EnterpriseAttentionItem = {
  actionLabel: string
  description: string
  onClick: () => void
  title: string
}

type EnterpriseLandingAction = {
  alertsView?: AlertLandingView
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  to?: string
}

type EnterpriseLandingCard = {
  action: EnterpriseLandingAction
  description: string
  returnAction?: EnterpriseLandingAction
  returnHint?: string
  statusLabel: string
  statusVariant: "outline" | "secondary" | "destructive"
  title: string
}

type EnterpriseRecoveryAction = {
  blockerCount: number
  description: string
  nextAction: {
    description: string
    kind: "section" | "route"
    label: string
    section?: EnterpriseSection
    title: string
    to?: string
  }
  title: string
}

type EnterpriseAlertsWorkspaceProps = {
  alertRecoveryAction: EnterpriseRecoveryAction
  approvals: EnterpriseJITProvisionApproval[]
  approvalActionID?: string | null
  approvalActionBusy?: boolean
  attentionItems: EnterpriseAttentionItem[]
  directoryLink: string
  dlqActionBusy?: boolean
  dlqActionID?: string | null
  formatDateTime: (value?: string) => string
  goToSection: (section: EnterpriseSection) => void
  landingCards: EnterpriseLandingCard[]
  loading: boolean
  onBatchProcessHRISWebhookReceipts?: (receiptIDs: string[]) => Promise<void>
  onBatchReviewApprovals?: (approvalIDs: string[], decision: "approved" | "rejected") => Promise<void>
  onBatchUpdateApprovalExternalSync?: (approvalIDs: string[], status: "synced" | "failed") => Promise<void>
  onProcessHRISWebhookReceipt?: (receiptID: string) => Promise<void>
  onBatchReplayHRISWebhookDLQ?: (entryIDs: string[]) => Promise<void>
  onDispatchWorkerAlerts?: () => Promise<void>
  onReconcilePendingSyncRequests?: () => Promise<void>
  onReplayHRISWebhookDLQ?: (entryID: string) => Promise<void>
  onAutoRetryWorkerAlertNotifications?: () => Promise<void>
  onBatchRetryWorkerAlertNotifications?: (notificationIDs: string[]) => Promise<void>
  onBatchRestoreWorkerAlertNotifications?: (notificationIDs: string[]) => Promise<void>
  onBatchSuppressWorkerAlertNotifications?: (notificationIDs: string[]) => Promise<void>
  onRetryWorkerAlertNotification?: (notificationID: string) => Promise<void>
  onReviewApproval?: (approvalID: string, decision: "approved" | "rejected") => Promise<void>
  onSaveWorkerAlertSubscription?: (payload: EnterpriseSyncWorkerAlertSubscriptionSaveInput) => Promise<void>
  onUpdateApprovalExternalSync?: (approvalID: string, status: "synced" | "failed") => Promise<void>
  syncLink: string
  latestWebhookReceiptBatchProcessResult?: EnterpriseHRISWebhookReceiptBatchProcessResult | null
  latestWebhookDLQBatchReplayResult?: EnterpriseHRISWebhookDLQBatchReplayResult | null
  initialFilterContextKey?: string
  initialLandingView?: AlertLandingView
  initialApprovalQuery?: string
  initialDirectoryQuery?: string
  initialSegmentHint?: AlertSegmentHint
  initialSegmentStatus?: AlertSegmentStatus
  initialWorkerAction?: string
  initialWorkerFilter?: "all" | "alerting" | "hot" | "stable"
  initialWorkerKind?: string
  initialWorkerLabel?: string
  initialWorkerQueueState?: string
  initialWorkerReplayState?: string
  initialWorkerStatus?: string
  initialExecutionKind?: HRISWebhookExecutionKindFilter
  initialExecutionStatus?: HRISWebhookExecutionStatusFilter
  initialExecutionQueueState?: HRISWebhookExecutionQueueStateFilter
  initialExecutionReplayScope?: HRISWebhookExecutionReplayScopeFilter
  initialExecutionMode?: HRISWebhookExecutionModeFilter
  initialSyncSourceFilter?: string
  initialSyncStatusFilter?: "all" | "attention" | "rejected" | "deactivated" | "healthy"
  policiesLink: string
  selectedTenantName?: string
  statusBadgeVariant: (status?: string) => "outline" | "secondary" | "destructive"
  syncRequestActionBusy?: boolean
  syncRequests: EnterpriseSyncRequestRecord[]
  syncJobs: EnterpriseSyncJob[]
  writable: boolean
  workerAlertSubscription: EnterpriseSyncWorkerAlertSubscription | null
  workerAlertDispatching?: boolean
  retryingWorkerAlertNotificationID?: string | null
  retryingWorkerAlertNotificationBatch?: boolean
  restoringWorkerAlertNotificationBatch?: boolean
  suppressingWorkerAlertNotificationBatch?: boolean
  autoRetryingWorkerAlertNotifications?: boolean
  workerAlertSubscriptionSaving?: boolean
  walletLink: string
  selectedTenantID?: string
  workerAlertEvents: EnterpriseSyncWorkerAlertItem[]
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
  workerAlertNotifications: EnterpriseSyncWorkerAlertNotification[]
  workerAlertNotificationTotal?: number
  workerAlertNotificationFilterCounts?: EnterpriseSyncWorkerAlertNotificationFilterCounts
  workerAlertNotificationStatusCounts?: EnterpriseSyncWorkerAlertNotificationStatusCounts
  workerAlertNotificationLoading?: boolean
  workerAlertNotificationLoadingMore?: boolean
  workerAlertNotificationHasMore?: boolean
  exportingWorkerAlertNotifications?: boolean
  onWorkerAlertNotificationHistoryViewChange?: (input: {
    filter: NotificationHistoryFilter
    query: string
  }) => void
  onLoadMoreWorkerAlertNotifications?: () => Promise<void>
  onExportWorkerAlertNotifications?: (input: {
    filter: NotificationHistoryFilter
    query: string
  }) => Promise<void>
  hrisWebhookExecutions: EnterpriseHRISWebhookExecution[]
  hrisWebhookExecutionTotal?: number
  hrisWebhookExecutionStatusCounts?: EnterpriseHRISWebhookExecutionStatusCounts
  hrisWebhookExecutionQueueCounts?: EnterpriseHRISWebhookRuntimeCounts | null
  hrisWebhookExecutionLoading?: boolean
  hrisWebhookExecutionLoadingMore?: boolean
  hrisWebhookExecutionHasMore?: boolean
  selectedHRISWebhookExecutionID?: string | null
  selectedHRISWebhookExecution?: EnterpriseHRISWebhookExecution | null
  selectedHRISWebhookExecutionLoading?: boolean
  selectedHRISWebhookExecutionError?: string
  executionActionID?: string | null
  onHRISWebhookExecutionHistoryViewChange?: (input: {
    kind: HRISWebhookExecutionKindFilter
    status: HRISWebhookExecutionStatusFilter
    queueState: HRISWebhookExecutionQueueStateFilter
    replayScope: HRISWebhookExecutionReplayScopeFilter
    executionMode: HRISWebhookExecutionModeFilter
    dispatchMode: HRISWebhookExecutionDispatchFilter
    targetStatus: string
    query: string
  }) => void
  onLoadMoreHRISWebhookExecutions?: () => Promise<void>
  onReplayHRISWebhookExecution?: (executionID: string) => Promise<void>
  onSelectHRISWebhookExecution?: (executionID: string | null) => void
  hrisWebhookReceiptTotal?: number
  hrisWebhookReceiptQueueCounts?: EnterpriseHRISWebhookRuntimeCounts | null
  hrisWebhookReceiptLoading?: boolean
  hrisWebhookReceiptLoadingMore?: boolean
  hrisWebhookReceiptHasMore?: boolean
  onLoadMoreHRISWebhookReceipts?: () => Promise<void>
  hrisWebhookDLQTotal?: number
  hrisWebhookDLQReplayCounts?: EnterpriseHRISWebhookRuntimeCounts | null
  hrisWebhookDLQLoading?: boolean
  hrisWebhookDLQLoadingMore?: boolean
  hrisWebhookDLQHasMore?: boolean
  onLoadMoreHRISWebhookDLQ?: () => Promise<void>
  receiptActionID?: string | null
  receiptActionBusy?: boolean
  hrisWebhookReceipts: EnterpriseHRISWebhookReceipt[]
  hrisWebhookDLQEntries: EnterpriseHRISWebhookDLQEntry[]
  hrisPullStates: EnterpriseHRISPullState[]
}

export function EnterpriseAlertsWorkspace({
  alertRecoveryAction,
  approvals: rawApprovals,
  approvalActionID,
  approvalActionBusy,
  attentionItems: rawAttentionItems,
  directoryLink,
  dlqActionBusy,
  dlqActionID,
  formatDateTime,
  goToSection,
  landingCards: rawLandingCards,
  loading,
  onBatchProcessHRISWebhookReceipts,
  onBatchReviewApprovals,
  onBatchUpdateApprovalExternalSync,
  onProcessHRISWebhookReceipt,
  onBatchReplayHRISWebhookDLQ,
  onDispatchWorkerAlerts,
  onReconcilePendingSyncRequests,
  onReplayHRISWebhookDLQ,
  onAutoRetryWorkerAlertNotifications,
  onBatchRetryWorkerAlertNotifications,
  onBatchRestoreWorkerAlertNotifications,
  onBatchSuppressWorkerAlertNotifications,
  onRetryWorkerAlertNotification,
  onReviewApproval,
  onSaveWorkerAlertSubscription,
  onUpdateApprovalExternalSync,
  syncLink,
  latestWebhookReceiptBatchProcessResult,
  latestWebhookDLQBatchReplayResult,
  initialFilterContextKey,
  initialLandingView,
  initialApprovalQuery,
  initialDirectoryQuery,
  initialSegmentHint,
  initialSegmentStatus,
  initialWorkerAction,
  initialWorkerFilter,
  initialWorkerKind,
  initialWorkerLabel,
  initialWorkerQueueState,
  initialWorkerReplayState,
  initialWorkerStatus,
  initialExecutionKind,
  initialExecutionStatus,
  initialExecutionQueueState,
  initialExecutionReplayScope,
  initialExecutionMode,
  initialSyncSourceFilter,
  initialSyncStatusFilter,
  policiesLink,
  selectedTenantName,
  statusBadgeVariant,
  syncRequestActionBusy,
  syncRequests: rawSyncRequests,
  syncJobs: rawSyncJobs,
  writable,
  workerAlertSubscription,
  workerAlertDispatching,
  retryingWorkerAlertNotificationID,
  retryingWorkerAlertNotificationBatch,
  restoringWorkerAlertNotificationBatch,
  suppressingWorkerAlertNotificationBatch,
  autoRetryingWorkerAlertNotifications,
  workerAlertSubscriptionSaving,
  walletLink,
  selectedTenantID,
  workerAlertEvents: rawWorkerAlertEvents,
  workerAlerts: rawWorkerAlerts,
  workerAlertNotifications: rawWorkerAlertNotifications,
  workerAlertNotificationTotal,
  workerAlertNotificationFilterCounts,
  workerAlertNotificationStatusCounts,
  workerAlertNotificationLoading,
  workerAlertNotificationLoadingMore,
  workerAlertNotificationHasMore,
  exportingWorkerAlertNotifications,
  onWorkerAlertNotificationHistoryViewChange,
  onLoadMoreWorkerAlertNotifications,
  onExportWorkerAlertNotifications,
  hrisWebhookExecutions: rawHRISWebhookExecutions,
  hrisWebhookExecutionTotal,
  hrisWebhookExecutionStatusCounts,
  hrisWebhookExecutionQueueCounts,
  hrisWebhookExecutionLoading,
  hrisWebhookExecutionLoadingMore,
  hrisWebhookExecutionHasMore,
  selectedHRISWebhookExecutionID,
  selectedHRISWebhookExecution,
  selectedHRISWebhookExecutionLoading,
  selectedHRISWebhookExecutionError,
  executionActionID,
  onHRISWebhookExecutionHistoryViewChange,
  onLoadMoreHRISWebhookExecutions,
  onReplayHRISWebhookExecution,
  onSelectHRISWebhookExecution,
  hrisWebhookReceiptTotal,
  hrisWebhookReceiptQueueCounts,
  hrisWebhookReceiptLoading,
  hrisWebhookReceiptLoadingMore,
  hrisWebhookReceiptHasMore,
  onLoadMoreHRISWebhookReceipts,
  hrisWebhookDLQTotal,
  hrisWebhookDLQReplayCounts,
  hrisWebhookDLQLoading,
  hrisWebhookDLQLoadingMore,
  hrisWebhookDLQHasMore,
  onLoadMoreHRISWebhookDLQ,
  receiptActionID,
  receiptActionBusy,
  hrisWebhookReceipts: rawHRISWebhookReceipts,
  hrisWebhookDLQEntries: rawHRISWebhookDLQEntries,
  hrisPullStates: rawHRISPullStates,
}: EnterpriseAlertsWorkspaceProps) {
  const { t } = useTranslation()
  const approvals = Array.isArray(rawApprovals) ? rawApprovals : []
  const attentionItems = Array.isArray(rawAttentionItems) ? rawAttentionItems : []
  const landingCards = Array.isArray(rawLandingCards) ? rawLandingCards : []
  const syncRequests = Array.isArray(rawSyncRequests) ? rawSyncRequests : []
  const syncJobs = Array.isArray(rawSyncJobs) ? rawSyncJobs : []
  const workerAlertEvents = Array.isArray(rawWorkerAlertEvents) ? rawWorkerAlertEvents : []
  const workerAlerts = Array.isArray(rawWorkerAlerts) ? rawWorkerAlerts : []
  const workerAlertNotifications = Array.isArray(rawWorkerAlertNotifications) ? rawWorkerAlertNotifications : []
  const hrisWebhookExecutions = Array.isArray(rawHRISWebhookExecutions) ? rawHRISWebhookExecutions : []
  const hrisWebhookReceipts = Array.isArray(rawHRISWebhookReceipts) ? rawHRISWebhookReceipts : []
  const hrisWebhookDLQEntries = Array.isArray(rawHRISWebhookDLQEntries) ? rawHRISWebhookDLQEntries : []
  const hrisPullStates = Array.isArray(rawHRISPullStates) ? rawHRISPullStates : []
  const [landingView, setLandingView] = useState<AlertLandingView>("overview")
  const [approvalStatusFilter, setApprovalStatusFilter] = useState("all")
  const [approvalSyncFilter, setApprovalSyncFilter] = useState<"all" | "failed" | "pending" | "success" | "none">("all")
  const [approvalQuery, setApprovalQuery] = useState("")
  const [syncStatusFilter, setSyncStatusFilter] = useState<"all" | "attention" | "rejected" | "deactivated" | "healthy">("all")
  const [syncSourceFilter, setSyncSourceFilter] = useState("all")
  const [workerActionScope, setWorkerActionScope] = useState("")
  const [workerFilter, setWorkerFilter] = useState<"all" | "alerting" | "hot" | "stable">("all")
  const [workerKindScope, setWorkerKindScope] = useState("")
  const [workerLabelScope, setWorkerLabelScope] = useState("")
  const [workerQueueStateScope, setWorkerQueueStateScope] = useState("")
  const [workerReplayStateScope, setWorkerReplayStateScope] = useState("")
  const [workerStatusScope, setWorkerStatusScope] = useState("")
  const [notificationHistoryFilter, setNotificationHistoryFilter] = useState<NotificationHistoryFilter>("all")
  const [notificationHistoryQuery, setNotificationHistoryQuery] = useState("")
  const [executionHistoryKindFilter, setExecutionHistoryKindFilter] =
    useState<HRISWebhookExecutionKindFilter>("all")
  const [executionHistoryStatusFilter, setExecutionHistoryStatusFilter] =
    useState<HRISWebhookExecutionStatusFilter>("all")
  const [executionHistoryQueueStateFilter, setExecutionHistoryQueueStateFilter] =
    useState<HRISWebhookExecutionQueueStateFilter>("all")
  const [executionHistoryReplayScopeFilter, setExecutionHistoryReplayScopeFilter] =
    useState<HRISWebhookExecutionReplayScopeFilter>("all")
  const [executionHistoryExecutionModeFilter, setExecutionHistoryExecutionModeFilter] =
    useState<HRISWebhookExecutionModeFilter>("all")
  const [executionHistoryDispatchModeFilter, setExecutionHistoryDispatchModeFilter] =
    useState<HRISWebhookExecutionDispatchFilter>("all")
  const [executionHistoryTargetStatusFilter, setExecutionHistoryTargetStatusFilter] = useState("")
  const [executionHistoryQuery, setExecutionHistoryQuery] = useState("")
  const [expandedWorkerAlertNotificationID, setExpandedWorkerAlertNotificationID] = useState<string | null>(null)
  const [directoryQuery, setDirectoryQuery] = useState("")
  const [appliedInitialFilterContextKey, setAppliedInitialFilterContextKey] = useState("")

  const normalizeStatus = (value?: string) => (value || "").trim().toLowerCase()

  const formatLifecycleToken = (value?: string) => {
    const normalized = normalizeStatus(value)
    if (!normalized) {
      return t("enterpriseAlertsWorkspace.common.emptyDash")
    }
    return normalized.replace(/_/g, " ")
  }

  const formatExecutionKindLabel = (value?: string) => {
    const normalized = normalizeStatus(value)
    switch (normalized) {
      case "receipt_process":
        return t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.kind.receiptProcess")
      case "dlq_replay":
        return t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.kind.dlqReplay")
      default:
        return formatLifecycleToken(normalized)
    }
  }

  const formatExecutionReplayWorkerRequirement = (value?: boolean) => {
    if (typeof value !== "boolean") {
      return t("enterpriseAlertsWorkspace.common.emptyDash")
    }
    return value
      ? t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replayWorkerRequired")
      : t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replayWorkerOptional")
  }

  const formatWorkerAlertNotificationRestoreStatus = (value?: string) => {
    const normalized = normalizeStatus(value)
    switch (normalized) {
      case "":
        return t("enterpriseAlertsWorkspace.common.emptyDash")
      case "ready":
        return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoreEligibility.ready")
      case "already_sent":
        return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoreEligibility.alreadySent")
      case "newer_history_exists":
        return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoreEligibility.newerHistoryExists")
      default:
        return normalized.replace(/_/g, " ")
    }
  }

  const classifyExternalSyncStatus = (value?: string): "none" | "failed" | "pending" | "success" | "other" => {
    const normalized = normalizeStatus(value)
    if (!normalized) {
      return "none"
    }
    if (/fail|error|reject/.test(normalized)) {
      return "failed"
    }
    if (/pending|queue|processing|running/.test(normalized)) {
      return "pending"
    }
    if (/success|synced|complete|ok/.test(normalized)) {
      return "success"
    }
    return "other"
  }

  const classifySyncJob = (item: EnterpriseSyncJob): "attention" | "rejected" | "deactivated" | "healthy" => {
    const normalizedStatus = normalizeStatus(item.status)
    if (normalizedStatus !== "completed") {
      return "attention"
    }
    if (item.rejected > 0) {
      return "rejected"
    }
    if (item.deactivated > 0) {
      return "deactivated"
    }
    return "healthy"
  }

  const isWorkerAlertNotificationDueNow = (item: EnterpriseSyncWorkerAlertNotification) => {
    if (item.status !== "failed" || !item.retryable || !item.next_retry_at) {
      return false
    }
    const retryAt = new Date(item.next_retry_at).getTime()
    if (Number.isNaN(retryAt)) {
      return false
    }
    return retryAt <= Date.now()
  }

  const matchesWorkerAlertNotificationQuery = (
    item: EnterpriseSyncWorkerAlertNotification,
    query: string
  ) => {
    const normalizedQuery = query.trim().toLowerCase()
    if (normalizedQuery.length === 0) {
      return true
    }
    const searchableValues = [
      item.id,
      item.worker_action,
      item.worker_kind,
      item.worker_label,
      item.fingerprint,
      item.connector_id || "",
      item.vendor || "",
      item.event_type || "",
      item.request_id || "",
      item.failure_stage || "",
      item.mode || "",
      item.status,
      item.reason || "",
      item.idempotency_key || "",
      item.provider || "",
      item.provider_error || "",
      item.source_notification_id || "",
      String(item.pending_age_seconds ?? ""),
      String(item.confirm_attempts ?? ""),
      item.last_confirm_attempt_at || "",
      item.last_confirm_result || "",
      item.next_retry_at || "",
      item.triggered_at,
      item.channels?.join(" ") || "",
      item.receiver_groups?.join(" ") || "",
    ]
      .map((value) => value.toLowerCase())
      .join(" ")
    if (searchableValues.includes(normalizedQuery)) {
      return true
    }
    return (item.channel_results || []).some((result) =>
      [
        result.channel,
        result.status,
        result.reason || "",
        result.provider || "",
        result.provider_error || "",
        result.receivers?.join(" ") || "",
      ]
        .join(" ")
        .toLowerCase()
        .includes(normalizedQuery)
    )
  }

  const isWorkerAlertNotificationConfirmationPending = (item: EnterpriseSyncWorkerAlertNotification) =>
    normalizeStatus(item.status).replace(/[\s_-]+/g, "") === "confirmationpending"

  const hasWorkerAlertNotificationPendingAge = (value?: number): value is number =>
    typeof value === "number" && Number.isFinite(value) && value >= 0

  const formatWorkerAlertNotificationPendingAge = (value?: number) => {
    if (!hasWorkerAlertNotificationPendingAge(value)) {
      return t("enterpriseAlertsWorkspace.common.emptyDash")
    }
    if (value < 60) {
      return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.pendingAge.seconds", {
        count: Math.max(1, Math.floor(value)),
      })
    }
    if (value < 3600) {
      return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.pendingAge.minutes", {
        count: Math.floor(value / 60),
      })
    }
    if (value < 86400) {
      return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.pendingAge.hours", {
        count: Math.floor(value / 3600),
      })
    }
    return t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.pendingAge.days", {
      count: Math.floor(value / 86400),
    })
  }

  const buildNotificationHistoryCSVValue = (value: string | number | boolean | undefined | null) =>
    `"${String(value ?? "").replace(/"/g, '""')}"`

  const exportVisibleWorkerAlertNotifications = () => {
    if (filteredWorkerAlertNotifications.length === 0) {
      return
    }
    const rows = [
      [
        "id",
        "triggered_at",
        "worker_action",
        "worker_kind",
        "worker_label",
        "fingerprint",
        "connector_id",
        "vendor",
        "event_type",
        "request_id",
        "failure_stage",
        "mode",
        "count",
        "threshold",
        "failed",
        "processed",
        "applied",
        "status",
        "reason",
        "attempt",
        "confirm_attempts",
        "last_confirm_result",
        "last_confirm_attempt_at",
        "retryable",
        "pending_age_seconds",
        "next_retry_at",
        "channels",
        "receiver_groups",
        "provider",
        "provider_error",
        "source_notification_id",
        "idempotency_key",
        "channel_results",
      ],
      ...filteredWorkerAlertNotifications.map((item) => [
        item.id,
        item.triggered_at,
        item.worker_action,
        item.worker_kind,
        item.worker_label,
        item.fingerprint,
        item.connector_id || "",
        item.vendor || "",
        item.event_type || "",
        item.request_id || "",
        item.failure_stage || "",
        item.mode || "",
        item.count,
        item.threshold,
        item.failed,
        item.processed,
        item.applied,
        item.status,
        item.reason || "",
        item.attempt ?? "",
        item.confirm_attempts ?? "",
        item.last_confirm_result || "",
        item.last_confirm_attempt_at || "",
        item.retryable,
        item.pending_age_seconds ?? "",
        item.next_retry_at || "",
        item.channels?.join("|") || "",
        item.receiver_groups?.join("|") || "",
        item.provider || "",
        item.provider_error || "",
        item.source_notification_id || "",
        item.idempotency_key || "",
        (item.channel_results || [])
          .map((result) =>
            [
              `${result.channel}:${result.status}`,
              result.reason ? `reason=${result.reason}` : "",
              result.provider ? `provider=${result.provider}` : "",
              result.provider_error ? `error=${result.provider_error}` : "",
              result.receivers && result.receivers.length > 0
                ? `receivers=${result.receivers.join("|")}`
                : "",
            ]
              .filter(Boolean)
              .join(" / ")
          )
          .join(" || "),
      ]),
    ]
      .map((row) => row.map((value) => buildNotificationHistoryCSVValue(value)).join(","))
      .join("\n")
    const stamp = new Date().toISOString().replace(/[:.]/g, "-")
    const scope =
      (selectedTenantName || "tenant")
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "") || "tenant"
    const fileName = `enterprise-sync-worker-alert-notifications-${scope}-${stamp}.csv`
    const blob = new Blob([rows], { type: "text/csv;charset=utf-8" })
    const url = window.URL.createObjectURL(blob)
    const anchor = document.createElement("a")
    anchor.href = url
    anchor.download = fileName
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    window.URL.revokeObjectURL(url)
  }

  const nextFlowAction = useMemo<EnterpriseLandingAction>(() => {
    if (alertRecoveryAction.nextAction.kind === "section") {
      return {
        kind: "section",
        section: alertRecoveryAction.nextAction.section,
        label: alertRecoveryAction.nextAction.label,
      }
    }
    return {
      kind: "route",
      to: alertRecoveryAction.nextAction.to,
      label: alertRecoveryAction.nextAction.label,
    }
  }, [alertRecoveryAction.nextAction])

  const approvalStatusOptions = useMemo(() => {
    const dynamicStatuses = Array.from(new Set(approvals.map((item) => normalizeStatus(item.status)).filter(Boolean)))
    const preferredOrder = ["pending", "approved", "rejected"]
    const extra = dynamicStatuses.filter((status) => !preferredOrder.includes(status)).sort((a, b) => a.localeCompare(b))
    return ["all", ...preferredOrder, ...extra]
  }, [approvals])

  const syncSourceOptions = useMemo(() => {
    const sources = Array.from(new Set(syncJobs.map((item) => item.source).filter(Boolean)))
    return ["all", ...sources]
  }, [syncJobs])

  useEffect(() => {
    if (!initialFilterContextKey || appliedInitialFilterContextKey === initialFilterContextKey) {
      return
    }
    if (initialLandingView) {
      setLandingView(initialLandingView)
    }
    if (initialSyncStatusFilter) {
      setSyncStatusFilter(initialSyncStatusFilter)
    }
    if (initialSyncSourceFilter && initialSyncSourceFilter !== "all") {
      const canApplySource = syncJobs.some((item) => item.source === initialSyncSourceFilter)
      setSyncSourceFilter(canApplySource ? initialSyncSourceFilter : "all")
    } else {
      setSyncSourceFilter("all")
    }
    if (initialWorkerFilter) {
      setWorkerFilter(initialWorkerFilter)
    }
    setWorkerActionScope(initialWorkerAction?.trim() || "")
    setWorkerKindScope(initialWorkerKind?.trim() || "")
    setWorkerLabelScope(initialWorkerLabel?.trim() || "")
    setWorkerQueueStateScope(initialWorkerQueueState?.trim() || "")
    setWorkerReplayStateScope(initialWorkerReplayState?.trim() || "")
    setWorkerStatusScope(initialWorkerStatus?.trim() || "")
    setExecutionHistoryKindFilter(initialExecutionKind || "all")
    setExecutionHistoryStatusFilter(initialExecutionStatus || "all")
    setExecutionHistoryQueueStateFilter(initialExecutionQueueState || "all")
    setExecutionHistoryReplayScopeFilter(initialExecutionReplayScope || "all")
    setExecutionHistoryExecutionModeFilter(initialExecutionMode || "all")
    setExecutionHistoryDispatchModeFilter("all")
    setExecutionHistoryTargetStatusFilter("")
    setExecutionHistoryQuery("")
    setNotificationHistoryQuery("")
    setApprovalQuery(initialApprovalQuery?.trim() || "")
    setDirectoryQuery(initialDirectoryQuery?.trim() || "")
    setAppliedInitialFilterContextKey(initialFilterContextKey)
  }, [
    appliedInitialFilterContextKey,
    initialApprovalQuery,
    initialDirectoryQuery,
    initialExecutionKind,
    initialExecutionMode,
    initialExecutionQueueState,
    initialExecutionReplayScope,
    initialExecutionStatus,
    initialFilterContextKey,
    initialLandingView,
    initialWorkerAction,
    initialWorkerFilter,
    initialWorkerKind,
    initialWorkerLabel,
    initialWorkerQueueState,
    initialWorkerReplayState,
    initialWorkerStatus,
    initialSyncSourceFilter,
    initialSyncStatusFilter,
    syncJobs,
  ])

  useEffect(() => {
    onWorkerAlertNotificationHistoryViewChange?.({
      filter: notificationHistoryFilter,
      query: notificationHistoryQuery,
    })
  }, [notificationHistoryFilter, notificationHistoryQuery, onWorkerAlertNotificationHistoryViewChange])

  useEffect(() => {
    onHRISWebhookExecutionHistoryViewChange?.({
      kind: executionHistoryKindFilter,
      status: executionHistoryStatusFilter,
      queueState: executionHistoryQueueStateFilter,
      replayScope: executionHistoryReplayScopeFilter,
      executionMode: executionHistoryExecutionModeFilter,
      dispatchMode: executionHistoryDispatchModeFilter,
      targetStatus: executionHistoryTargetStatusFilter,
      query: executionHistoryQuery,
    })
  }, [
    executionHistoryDispatchModeFilter,
    executionHistoryExecutionModeFilter,
    executionHistoryKindFilter,
    executionHistoryQuery,
    executionHistoryQueueStateFilter,
    executionHistoryReplayScopeFilter,
    executionHistoryStatusFilter,
    executionHistoryTargetStatusFilter,
    onHRISWebhookExecutionHistoryViewChange,
  ])

  const filteredApprovals = useMemo(() => {
    const normalizedQuery = approvalQuery.trim().toLowerCase()
    return approvals.filter((item) => {
      const normalizedStatus = normalizeStatus(item.status)
      const syncState = classifyExternalSyncStatus(item.external_sync_status)
      const statusPass = approvalStatusFilter === "all" || normalizedStatus === approvalStatusFilter
      const syncPass = approvalSyncFilter === "all" || syncState === approvalSyncFilter
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.id,
          item.email,
          item.external_id,
          item.provider,
          item.reason,
          item.external_sync_status,
          item.external_sync_ref,
          item.external_sync_last_error,
        ]
          .map((value) => (value || "").toString().toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return statusPass && syncPass && queryPass
    })
  }, [approvalQuery, approvalStatusFilter, approvalSyncFilter, approvals])
  const batchPendingApprovalIDs = useMemo(
    () => filteredApprovals.filter((item) => normalizeStatus(item.status) === "pending").map((item) => item.id),
    [filteredApprovals]
  )
  const batchSyncMarkableApprovalIDs = useMemo(
    () =>
      filteredApprovals
        .filter(
          (item) =>
            normalizeStatus(item.status) !== "pending" && classifyExternalSyncStatus(item.external_sync_status) !== "success"
        )
        .map((item) => item.id),
    [filteredApprovals]
  )
  const allPendingApprovalIDs = useMemo(
    () => approvals.filter((item) => normalizeStatus(item.status) === "pending").map((item) => item.id),
    [approvals]
  )
  const allFailedSyncApprovalIDs = useMemo(
    () =>
      approvals
        .filter(
          (item) =>
            normalizeStatus(item.status) !== "pending" && classifyExternalSyncStatus(item.external_sync_status) === "failed"
        )
        .map((item) => item.id),
    [approvals]
  )
  const allPendingSyncApprovalIDs = useMemo(
    () =>
      approvals
        .filter(
          (item) =>
            normalizeStatus(item.status) !== "pending" && classifyExternalSyncStatus(item.external_sync_status) === "pending"
        )
        .map((item) => item.id),
    [approvals]
  )
  const enterpriseReadOnlyDisabledReason = !writable
    ? t("enterpriseAlertsWorkspace.disabledReasons.readOnly")
    : ""
  const approvalActionDisabledReason =
    enterpriseReadOnlyDisabledReason ||
    (approvalActionBusy ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : "")
  const approvalBatchDisabledReason = (
    actionAvailable: boolean,
    targetCount: number,
    emptyReasonKey: "noPendingApprovals" | "noExternalSyncFailures" | "noExternalSyncPending" | "noVisiblePendingApprovals" | "noVisibleSyncMarkableApprovals"
  ) => {
    if (!actionAvailable) {
      return t("enterpriseAlertsWorkspace.disabledReasons.actionUnavailable")
    }
    return approvalActionDisabledReason || (targetCount === 0 ? t(`enterpriseAlertsWorkspace.disabledReasons.${emptyReasonKey}`) : "")
  }
  const jitBatchApprovalDisabledReason = approvalBatchDisabledReason(
    Boolean(onBatchReviewApprovals),
    batchPendingApprovalIDs.length,
    "noVisiblePendingApprovals"
  )
  const jitBatchExternalSyncDisabledReason = approvalBatchDisabledReason(
    Boolean(onBatchUpdateApprovalExternalSync),
    batchSyncMarkableApprovalIDs.length,
    "noVisibleSyncMarkableApprovals"
  )

  const filteredSyncJobs = useMemo(() => {
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return syncJobs.filter((item) => {
      const category = classifySyncJob(item)
      const statusPass = syncStatusFilter === "all" || category === syncStatusFilter
      const sourcePass = syncSourceFilter === "all" || item.source === syncSourceFilter
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.id,
          item.source,
          item.status,
          item.actor,
          String(item.total),
          String(item.created),
          String(item.updated),
          String(item.deactivated),
          String(item.rejected),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return statusPass && sourcePass && queryPass
    })
  }, [directoryQuery, syncJobs, syncSourceFilter, syncStatusFilter])

  const pendingSyncRequests = useMemo(
    () => syncRequests.filter((item) => !item.access_applied),
    [syncRequests]
  )

  const filteredPendingSyncRequests = useMemo(() => {
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return pendingSyncRequests.filter((item) => {
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          item.request_id,
          item.connector_id || "",
          item.raw_payload_ref || "",
          item.result.job.id,
          item.result.job.status,
          item.last_access_error || "",
          item.last_access_attempt_at || "",
          String(item.access_attempt_count),
          String(item.access_created),
          String(item.access_updated),
          String(item.access_rejected),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return queryPass
    })
  }, [directoryQuery, pendingSyncRequests])

  const filteredWorkerAlerts = useMemo(() => {
    const normalizedActionScope = workerActionScope.trim().toLowerCase()
    const normalizedKindScope = workerKindScope.trim().toLowerCase()
    const normalizedLabelScope = workerLabelScope.trim().toLowerCase()
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return workerAlerts.filter((item) => {
      const category = classifyEnterpriseSyncWorkerAlertLevel(item)
      const categoryPass = workerFilter === "all" || category === workerFilter
      const actionPass =
        normalizedActionScope.length === 0 || (item.worker_action || "").trim().toLowerCase() === normalizedActionScope
      const kindPass =
        normalizedKindScope.length === 0 || (item.worker_kind || "").trim().toLowerCase() === normalizedKindScope
      const labelPass =
        normalizedLabelScope.length === 0 || (item.worker_label || "").trim().toLowerCase() === normalizedLabelScope
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          item.worker_action || "",
          item.worker_kind || "",
          item.worker_label || "",
          String(item.count),
          String(item.last_failed),
          String(item.last_threshold),
          String(item.last_processed),
          String(item.last_applied),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return categoryPass && actionPass && kindPass && labelPass && queryPass
    })
  }, [directoryQuery, workerActionScope, workerAlerts, workerFilter, workerKindScope, workerLabelScope])

  const filteredWorkerAlertEvents = useMemo(() => {
    const normalizedActionScope = workerActionScope.trim().toLowerCase()
    const normalizedKindScope = workerKindScope.trim().toLowerCase()
    const normalizedLabelScope = workerLabelScope.trim().toLowerCase()
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return workerAlertEvents.filter((item) => {
      const actionPass =
        normalizedActionScope.length === 0 || (item.worker_action || "").trim().toLowerCase() === normalizedActionScope
      const kindPass =
        normalizedKindScope.length === 0 || (item.worker_kind || "").trim().toLowerCase() === normalizedKindScope
      const labelPass =
        normalizedLabelScope.length === 0 || (item.worker_label || "").trim().toLowerCase() === normalizedLabelScope
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          item.worker_action || "",
          item.worker_kind || "",
          item.worker_label || "",
          item.vendor || "",
          item.event_type || "",
          item.request_id || "",
          item.connector_id || "",
          item.failure_stage || "",
          item.raw_target || "",
          item.source || "",
          String(item.failed),
          String(item.threshold),
          String(item.processed),
          String(item.applied),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return actionPass && kindPass && labelPass && queryPass
    })
  }, [directoryQuery, workerActionScope, workerAlertEvents, workerKindScope, workerLabelScope])

  const derivedNotificationHistoryCounts = useMemo(
    () => ({
      all: workerAlertNotifications.length,
      failed: workerAlertNotifications.filter((item) => item.status === "failed").length,
      retryable: workerAlertNotifications.filter((item) => item.retryable).length,
      suppressed: workerAlertNotifications.filter((item) => item.status === "skipped" && item.reason === "manual_suppressed").length,
      due_now: workerAlertNotifications.filter((item) => isWorkerAlertNotificationDueNow(item)).length,
    }),
    [workerAlertNotifications]
  )

  const notificationHistoryBaseItems = useMemo(() => {
    if (notificationHistoryFilter === "failed") {
      return workerAlertNotifications.filter((item) => item.status === "failed")
    }
    if (notificationHistoryFilter === "retryable") {
      return workerAlertNotifications.filter((item) => item.retryable)
    }
    if (notificationHistoryFilter === "suppressed") {
      return workerAlertNotifications.filter((item) => item.status === "skipped" && item.reason === "manual_suppressed")
    }
    if (notificationHistoryFilter === "due_now") {
      return workerAlertNotifications.filter((item) => isWorkerAlertNotificationDueNow(item))
    }
    return workerAlertNotifications
  }, [notificationHistoryFilter, workerAlertNotifications])

  const filteredWorkerAlertNotifications = useMemo(
    () =>
      notificationHistoryBaseItems.filter((item) =>
        matchesWorkerAlertNotificationQuery(item, notificationHistoryQuery)
      ),
    [notificationHistoryBaseItems, notificationHistoryQuery]
  )

  const derivedNotificationHistoryStatusCounts = useMemo(
    () => ({
      sent: filteredWorkerAlertNotifications.filter((item) => item.status === "sent").length,
      failed: filteredWorkerAlertNotifications.filter((item) => item.status === "failed").length,
      skipped: filteredWorkerAlertNotifications.filter((item) => item.status === "skipped").length,
    }),
    [filteredWorkerAlertNotifications]
  )

  const notificationHistoryCounts = workerAlertNotificationFilterCounts ?? derivedNotificationHistoryCounts
  const notificationHistoryStatusCounts = workerAlertNotificationStatusCounts ?? derivedNotificationHistoryStatusCounts
  const notificationHistoryTotal = workerAlertNotificationTotal ?? filteredWorkerAlertNotifications.length
  const notificationHistoryLoadingState = loading || Boolean(workerAlertNotificationLoading)

  const visibleRetryableWorkerAlertNotificationIDs = useMemo(
    () =>
      filteredWorkerAlertNotifications
        .filter((item) => item.status === "failed" && item.retryable)
        .map((item) => item.id),
    [filteredWorkerAlertNotifications]
  )

  const visibleFailedWorkerAlertNotificationIDs = useMemo(
    () => filteredWorkerAlertNotifications.filter((item) => item.status === "failed").map((item) => item.id),
    [filteredWorkerAlertNotifications]
  )

  const visibleSuppressedWorkerAlertNotificationIDs = useMemo(
    () =>
      filteredWorkerAlertNotifications
        .filter(
          (item) =>
            item.status === "skipped" &&
            item.reason === "manual_suppressed" &&
            (!item.restore_status || item.restore_status === "ready")
        )
        .map((item) => item.id),
    [filteredWorkerAlertNotifications]
  )

  const dueWorkerAlertNotificationCount = useMemo(
    () => workerAlertNotifications.filter((item) => isWorkerAlertNotificationDueNow(item)).length,
    [workerAlertNotifications]
  )

  const derivedExecutionHistoryStatusCounts = useMemo(
    () => ({
      all: hrisWebhookExecutions.length,
      queued: hrisWebhookExecutions.filter((item) => item.status === "queued").length,
      running: hrisWebhookExecutions.filter((item) => item.status === "running").length,
      succeeded: hrisWebhookExecutions.filter((item) => item.status === "succeeded").length,
      failed: hrisWebhookExecutions.filter((item) => item.status === "failed").length,
    }),
    [hrisWebhookExecutions]
  )
  const derivedExecutionHistoryQueueCounts = useMemo(
    () => ({
      all: hrisWebhookExecutions.length,
      ready: hrisWebhookExecutions.filter((item) => item.queue_state === "ready").length,
      cooldown: hrisWebhookExecutions.filter((item) => item.queue_state === "cooldown").length,
      in_flight: hrisWebhookExecutions.filter((item) => item.queue_state === "in_flight").length,
      attempt_limit: hrisWebhookExecutions.filter((item) => item.queue_state === "attempt_limit").length,
      terminal: hrisWebhookExecutions.filter((item) => item.queue_state === "terminal").length,
    }),
    [hrisWebhookExecutions]
  )
  const executionHistoryCounts = hrisWebhookExecutionStatusCounts ?? derivedExecutionHistoryStatusCounts
  const executionHistoryQueueCounts = hrisWebhookExecutionQueueCounts ?? derivedExecutionHistoryQueueCounts
  const executionHistoryTotalCount = hrisWebhookExecutionTotal ?? hrisWebhookExecutions.length
  const executionHistoryLoadingState = loading || Boolean(hrisWebhookExecutionLoading)
  const selectedHRISWebhookExecutionInCurrentScope = useMemo(
    () =>
      Boolean(
        selectedHRISWebhookExecutionID &&
          hrisWebhookExecutions.some((item) => item.id === selectedHRISWebhookExecutionID)
      ),
    [hrisWebhookExecutions, selectedHRISWebhookExecutionID]
  )
  const selectedHRISWebhookExecutionURLHint = useMemo(() => {
    if (!selectedHRISWebhookExecutionID) {
      return ""
    }
    const query = new URLSearchParams()
    if (selectedTenantID?.trim()) {
      query.set("tenant_id", selectedTenantID.trim())
    }
    query.set("execution_id", selectedHRISWebhookExecutionID)
    const suffix = query.toString()
    return suffix ? `?${suffix}#alerts` : "#alerts"
  }, [selectedHRISWebhookExecutionID, selectedTenantID])
  const executionHistoryStatusFilterOptions = useMemo(
    () => [
      {
        value: "all" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.filters.all"),
        count: executionHistoryCounts.all,
      },
      {
        value: "queued" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.filters.queued"),
        count: executionHistoryCounts.queued,
      },
      {
        value: "running" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.filters.running"),
        count: executionHistoryCounts.running,
      },
      {
        value: "succeeded" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.filters.succeeded"),
        count: executionHistoryCounts.succeeded,
      },
      {
        value: "failed" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.filters.failed"),
        count: executionHistoryCounts.failed,
      },
    ],
    [executionHistoryCounts, t]
  )
  const executionHistoryQueueStateFilterOptions = useMemo(
    () => [
      {
        value: "all" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.queueStateFilter.all"),
        count: executionHistoryQueueCounts.all,
      },
      {
        value: "ready" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.queueStateFilter.ready"),
        count: executionHistoryQueueCounts.ready,
      },
      {
        value: "cooldown" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.queueStateFilter.cooldown"),
        count: executionHistoryQueueCounts.cooldown,
      },
      {
        value: "in_flight" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.queueStateFilter.inFlight"),
        count: executionHistoryQueueCounts.in_flight,
      },
      {
        value: "attempt_limit" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.queueStateFilter.attemptLimit"),
        count: executionHistoryQueueCounts.attempt_limit,
      },
      {
        value: "terminal" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.queueStateFilter.terminal"),
        count: executionHistoryQueueCounts.terminal,
      },
    ],
    [executionHistoryQueueCounts, t]
  )
  const executionHistoryReplayScopeFilterOptions = useMemo(
    () => [
      {
        value: "all" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replayScopeFilter.all"),
      },
      {
        value: "replayed" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replayScopeFilter.replayed"),
      },
      {
        value: "worker_required" as const,
        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replayScopeFilter.workerRequired"),
      },
    ],
    [t]
  )

  const filteredWebhookReceipts = useMemo(() => {
    const normalizedScope = [workerActionScope, workerKindScope, workerLabelScope].join(" ").trim().toLowerCase()
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    const normalizedQueueStateScope = workerQueueStateScope.trim().toLowerCase()
    const normalizedStatusScope = workerStatusScope.trim().toLowerCase()
    return hrisWebhookReceipts.filter((item) => {
      const scopePass =
        normalizedScope.length === 0 ||
        normalizedScope.includes("receipt") ||
        normalizedScope.includes("webhook") ||
        normalizedScope.includes("queue") ||
        normalizedScope.includes("processing") ||
        normalizedScope.includes("retry")
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          item.connector_id,
          item.vendor,
          item.event_type || "",
          item.request_id || "",
          item.status,
          item.queue_state,
          item.last_error || "",
          String(item.attempt_count || 0),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      const queueStatePass = normalizedQueueStateScope.length === 0 || (item.queue_state || "").toLowerCase() === normalizedQueueStateScope
      const statusPass = normalizedStatusScope.length === 0 || (item.status || "").toLowerCase() === normalizedStatusScope
      return scopePass && queryPass && queueStatePass && statusPass
    })
  }, [
    directoryQuery,
    hrisWebhookReceipts,
    workerActionScope,
    workerKindScope,
    workerLabelScope,
    workerQueueStateScope,
    workerStatusScope,
  ])

  const visibleProcessableWebhookReceiptIDs = useMemo(
    () => filteredWebhookReceipts.filter((item) => item.queue_state === "ready").map((item) => item.id),
    [filteredWebhookReceipts]
  )

  const filteredDLQEntries = useMemo(() => {
    const normalizedScope = [workerActionScope, workerKindScope, workerLabelScope].join(" ").trim().toLowerCase()
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    const normalizedReplayStateScope = workerReplayStateScope.trim().toLowerCase()
    const normalizedStatusScope = workerStatusScope.trim().toLowerCase()
    return hrisWebhookDLQEntries.filter((item) => {
      const scopePass =
        normalizedScope.length === 0 ||
        normalizedScope.includes("webhook") ||
        normalizedScope.includes("dlq") ||
        normalizedScope.includes("replay") ||
        normalizedScope.includes("processing")
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          item.connector_id || "",
          item.vendor || "",
          item.receipt_id || "",
          item.request_id || "",
          item.event_type || "",
          item.failure_stage || "",
          item.error || "",
          item.status || "",
          item.replay_state || "",
          item.next_retry_at || "",
          item.processing_deadline_at || "",
          item.raw_payload_ref || "",
          String(item.replay_count || 0),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      const replayStatePass =
        normalizedReplayStateScope.length === 0 || (item.replay_state || "").toLowerCase() === normalizedReplayStateScope
      const statusPass = normalizedStatusScope.length === 0 || (item.status || "").toLowerCase() === normalizedStatusScope
      return scopePass && queryPass && replayStatePass && statusPass
    })
  }, [
    directoryQuery,
    hrisWebhookDLQEntries,
    workerActionScope,
    workerKindScope,
    workerLabelScope,
    workerReplayStateScope,
    workerStatusScope,
  ])

  const visibleReplayableDLQEntryIDs = useMemo(
    () => filteredDLQEntries.filter((item) => item.replay_state === "ready").map((item) => item.id),
    [filteredDLQEntries]
  )
  const notificationActionBusy =
    Boolean(retryingWorkerAlertNotificationID) ||
    Boolean(retryingWorkerAlertNotificationBatch) ||
    Boolean(restoringWorkerAlertNotificationBatch) ||
    Boolean(suppressingWorkerAlertNotificationBatch) ||
    Boolean(autoRetryingWorkerAlertNotifications)
  const notificationActionDisabledReason =
    enterpriseReadOnlyDisabledReason ||
    (notificationHistoryLoadingState ? t("enterpriseAlertsWorkspace.disabledReasons.loading") : "") ||
    (notificationActionBusy ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : "")
  const notificationExportDisabledReason = notificationHistoryLoadingState
    ? t("enterpriseAlertsWorkspace.disabledReasons.loading")
    : exportingWorkerAlertNotifications
      ? t("enterpriseAlertsWorkspace.disabledReasons.exporting")
      : notificationHistoryTotal === 0
        ? t("enterpriseAlertsWorkspace.disabledReasons.noVisibleNotifications")
        : ""
  const autoRetryNotificationsDisabledReason =
    notificationActionDisabledReason ||
    (dueWorkerAlertNotificationCount === 0 ? t("enterpriseAlertsWorkspace.disabledReasons.noDueNotifications") : "")
  const suppressNotificationsDisabledReason =
    notificationActionDisabledReason ||
    (visibleFailedWorkerAlertNotificationIDs.length === 0 ? t("enterpriseAlertsWorkspace.disabledReasons.noFailedNotifications") : "")
  const restoreNotificationsDisabledReason =
    notificationActionDisabledReason ||
    (visibleSuppressedWorkerAlertNotificationIDs.length === 0 ? t("enterpriseAlertsWorkspace.disabledReasons.noSuppressedNotifications") : "")
  const retryNotificationsDisabledReason =
    notificationActionDisabledReason ||
    (visibleRetryableWorkerAlertNotificationIDs.length === 0 ? t("enterpriseAlertsWorkspace.disabledReasons.noRetryableNotifications") : "")

  const filteredPullStates = useMemo(() => {
    const normalizedScope = [workerActionScope, workerKindScope, workerLabelScope].join(" ").trim().toLowerCase()
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return hrisPullStates.filter((item) => {
      const scopePass = normalizedScope.length === 0 || normalizedScope.includes("pull")
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          item.connector_id,
          item.vendor,
          item.status,
          item.last_request_id || "",
          item.last_mode || "",
          item.last_error || "",
          String(item.consecutive_failures || 0),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return scopePass && queryPass
    })
  }, [directoryQuery, hrisPullStates, workerActionScope, workerKindScope, workerLabelScope])

  const approvalCounts = useMemo(() => {
    const statusCount = (status: string) => approvals.filter((item) => normalizeStatus(item.status) === status).length
    const syncCount = (syncType: "failed" | "pending" | "success" | "none") =>
      approvals.filter((item) => classifyExternalSyncStatus(item.external_sync_status) === syncType).length
    return {
      all: approvals.length,
      pending: statusCount("pending"),
      approved: statusCount("approved"),
      rejected: statusCount("rejected"),
      syncFailed: syncCount("failed"),
      syncPending: syncCount("pending"),
      syncSuccess: syncCount("success"),
      syncNone: syncCount("none"),
    }
  }, [approvals])

  const syncJobCounts = useMemo(() => {
    return {
      all: syncJobs.length,
      attention: syncJobs.filter((item) => classifySyncJob(item) === "attention").length,
      rejected: syncJobs.filter((item) => classifySyncJob(item) === "rejected").length,
      deactivated: syncJobs.filter((item) => classifySyncJob(item) === "deactivated").length,
      healthy: syncJobs.filter((item) => classifySyncJob(item) === "healthy").length,
    }
  }, [syncJobs])

  const workerCounts = useMemo(() => {
    return {
      all: workerAlerts.length,
      alerting: workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) === "alerting").length,
      hot: workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) === "hot").length,
      stable: workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) === "stable").length,
    }
  }, [workerAlerts])
  const derivedWebhookReceiptRuntimeCounts = useMemo(
    () => ({
      all: hrisWebhookReceipts.length,
      ready: hrisWebhookReceipts.filter((item) => item.queue_state === "ready").length,
      cooldown: hrisWebhookReceipts.filter((item) => item.queue_state === "cooldown").length,
      in_flight: hrisWebhookReceipts.filter((item) => item.queue_state === "in_flight").length,
      attempt_limit: hrisWebhookReceipts.filter((item) => item.queue_state === "attempt_limit").length,
      terminal: hrisWebhookReceipts.filter((item) => item.queue_state === "terminal").length,
    }),
    [hrisWebhookReceipts]
  )
  const webhookReceiptRuntimeCounts = hrisWebhookReceiptQueueCounts ?? derivedWebhookReceiptRuntimeCounts
  const webhookReceiptTotal = hrisWebhookReceiptTotal ?? hrisWebhookReceipts.length
  const webhookReceiptLoadingState = loading || Boolean(hrisWebhookReceiptLoading)
  const webhookReceiptRuntimeFilterOptions = useMemo(
    () => [
      {
        value: "all",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.filters.all"),
        count: webhookReceiptRuntimeCounts.all,
        queueState: "",
      },
      {
        value: "ready",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.filters.ready"),
        count: webhookReceiptRuntimeCounts.ready,
        queueState: "ready",
      },
      {
        value: "cooldown",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.filters.cooldown"),
        count: webhookReceiptRuntimeCounts.cooldown,
        queueState: "cooldown",
      },
      {
        value: "in_flight",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.filters.inFlight"),
        count: webhookReceiptRuntimeCounts.in_flight,
        queueState: "in_flight",
      },
      {
        value: "attempt_limit",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.filters.attemptLimit"),
        count: webhookReceiptRuntimeCounts.attempt_limit,
        queueState: "attempt_limit",
      },
      {
        value: "terminal",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.filters.terminal"),
        count: webhookReceiptRuntimeCounts.terminal,
        queueState: "terminal",
      },
    ],
    [t, webhookReceiptRuntimeCounts]
  )
  const derivedDLQRuntimeCounts = useMemo(
    () => ({
      all: hrisWebhookDLQEntries.length,
      ready: hrisWebhookDLQEntries.filter((item) => item.replay_state === "ready").length,
      cooldown: hrisWebhookDLQEntries.filter((item) => item.replay_state === "cooldown").length,
      in_flight: hrisWebhookDLQEntries.filter((item) => item.replay_state === "in_flight").length,
      attempt_limit: hrisWebhookDLQEntries.filter((item) => item.replay_state === "attempt_limit").length,
      terminal: hrisWebhookDLQEntries.filter((item) => item.replay_state === "terminal").length,
    }),
    [hrisWebhookDLQEntries]
  )
  const dlqRuntimeCounts = hrisWebhookDLQReplayCounts ?? derivedDLQRuntimeCounts
  const dlqTotal = hrisWebhookDLQTotal ?? hrisWebhookDLQEntries.length
  const dlqLoadingState = loading || Boolean(hrisWebhookDLQLoading)
  const receiptActionDisabledReason =
    enterpriseReadOnlyDisabledReason ||
    (receiptActionBusy ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : "")
  const receiptBatchDisabledReason =
    receiptActionDisabledReason ||
    (webhookReceiptLoadingState ? t("enterpriseAlertsWorkspace.disabledReasons.loading") : "") ||
    (visibleProcessableWebhookReceiptIDs.length === 0 ? t("enterpriseAlertsWorkspace.disabledReasons.noProcessableReceipts") : "")
  const dlqActionDisabledReason =
    enterpriseReadOnlyDisabledReason ||
    (dlqActionBusy ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : "")
  const dlqBatchDisabledReason =
    dlqActionDisabledReason ||
    (dlqLoadingState ? t("enterpriseAlertsWorkspace.disabledReasons.loading") : "") ||
    (visibleReplayableDLQEntryIDs.length === 0 ? t("enterpriseAlertsWorkspace.disabledReasons.noReplayableDLQ") : "")
  const dlqRuntimeFilterOptions = useMemo(
    () => [
      {
        value: "all",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.dlq.filters.all"),
        count: dlqRuntimeCounts.all,
        replayState: "",
      },
      {
        value: "ready",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.dlq.filters.ready"),
        count: dlqRuntimeCounts.ready,
        replayState: "ready",
      },
      {
        value: "cooldown",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.dlq.filters.cooldown"),
        count: dlqRuntimeCounts.cooldown,
        replayState: "cooldown",
      },
      {
        value: "in_flight",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.dlq.filters.inFlight"),
        count: dlqRuntimeCounts.in_flight,
        replayState: "in_flight",
      },
      {
        value: "attempt_limit",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.dlq.filters.attemptLimit"),
        count: dlqRuntimeCounts.attempt_limit,
        replayState: "attempt_limit",
      },
      {
        value: "terminal",
        label: t("enterpriseAlertsWorkspace.syncAndWorker.dlq.filters.terminal"),
        count: dlqRuntimeCounts.terminal,
        replayState: "terminal",
      },
    ],
    [dlqRuntimeCounts, t]
  )
  const approvalLandingIssueCount = approvalCounts.pending + approvalCounts.syncFailed + approvalCounts.syncPending
  const directoryLandingIssueCount =
    syncJobCounts.attention + syncJobCounts.rejected + syncJobCounts.deactivated + workerCounts.alerting + workerCounts.hot
  const receiptRecoveryFlowEnabled = initialSegmentHint === "receipt_recovery"
  const receiptRecoveryBlockerCount = approvalLandingIssueCount + directoryLandingIssueCount
  const receiptRecoverySegmentStatus: AlertSegmentStatus =
    initialSegmentStatus || (receiptRecoveryBlockerCount > 0 ? "attention" : "ready")
  const receiptRecoverySegmentStatusLabel =
    receiptRecoverySegmentStatus === "ready"
      ? t("enterpriseAlertsWorkspace.receiptRecovery.status.ready")
      : receiptRecoverySegmentStatus === "attention"
        ? t("enterpriseAlertsWorkspace.receiptRecovery.status.attention")
        : t("enterpriseAlertsWorkspace.receiptRecovery.status.pending")
  const receiptRecoverySegmentStatusVariant: "outline" | "secondary" | "destructive" =
    receiptRecoverySegmentStatus === "ready"
      ? "outline"
      : receiptRecoverySegmentStatus === "attention"
        ? "secondary"
        : "destructive"
  const receiptRecoveryBackflowStatus: AlertSegmentStatus = receiptRecoveryBlockerCount > 0 ? "attention" : "ready"
  const receiptRecoveryQueryHint = useMemo(() => {
    const first = approvalQuery.trim() || directoryQuery.trim()
    if (first) {
      return first
    }
    return (initialApprovalQuery || initialDirectoryQuery || "").trim()
  }, [approvalQuery, directoryQuery, initialApprovalQuery, initialDirectoryQuery])
  const receiptRecoveryBackflowLinks = useMemo(
    () => ({
      retry: withRouteHints(walletLink, {
        segment_hint: "receipt_recovery",
        segment_status_hint: receiptRecoveryBackflowStatus,
        receipt_recovery_action_hint: "retry_delivery",
        target_hint: receiptRecoveryQueryHint,
        target_id: receiptRecoveryQueryHint,
      }),
      repair: withRouteHints(walletLink, {
        segment_hint: "receipt_recovery",
        segment_status_hint: receiptRecoveryBackflowStatus,
        receipt_recovery_action_hint: "repair_pass_status",
        target_hint: receiptRecoveryQueryHint,
        target_id: receiptRecoveryQueryHint,
      }),
      closed: withRouteHints(walletLink, {
        segment_hint: "receipt_recovery",
        segment_status_hint: "ready",
        receipt_recovery_action_hint: "review_closed",
        target_hint: receiptRecoveryQueryHint,
        target_id: receiptRecoveryQueryHint,
      }),
    }),
    [receiptRecoveryBackflowStatus, receiptRecoveryQueryHint, walletLink]
  )
  const showOverviewCards = landingView === "overview"
  const showApprovalCards = landingView === "overview" || landingView === "approval_backlog"
  const showDirectoryCards = landingView === "overview" || landingView === "directory_exceptions"

  const resolveApprovalAction = (item: EnterpriseJITProvisionApproval): EnterpriseLandingAction => {
    const syncState = classifyExternalSyncStatus(item.external_sync_status)
    const normalizedStatus = normalizeStatus(item.status)

    if (syncState === "failed") {
      return {
        kind: "section",
        section: "idp",
        label: t("enterpriseAlertsWorkspace.actions.goIdpReview"),
      }
    }
    if (normalizedStatus === "approved") {
      return {
        kind: "route",
        to: directoryLink,
        label: t("enterpriseAlertsWorkspace.actions.backToDirectory"),
      }
    }
    if (normalizedStatus === "pending") {
      return {
        kind: "section",
        section: "idp",
        label: t("enterpriseAlertsWorkspace.actions.goIdpReview"),
      }
    }
    return nextFlowAction
  }

  const resolveSyncJobAction = (item: EnterpriseSyncJob): EnterpriseLandingAction => {
    const category = classifySyncJob(item)
    if (category === "attention") {
      return {
        kind: "section",
        section: "sync",
        label: t("enterpriseAlertsWorkspace.actions.goSync"),
      }
    }
    if (category === "rejected") {
      return {
        kind: "section",
        section: "alerts",
        alertsView: "directory_exceptions",
        label: t("enterpriseAlertsWorkspace.actions.goDirectoryExceptions"),
      }
    }
    if (category === "deactivated") {
      return {
        kind: "route",
        to: directoryLink,
        label: t("enterpriseAlertsWorkspace.actions.reviewDeactivated"),
      }
    }
    return nextFlowAction
  }

  const resolveWorkerAction = (item: EnterpriseSyncWorkerAlertSummaryItem): EnterpriseLandingAction => {
    const category = classifyEnterpriseSyncWorkerAlertLevel(item)
    if (category === "hot" || category === "alerting") {
      return {
        kind: "section",
        section: "sync",
        label: t("enterpriseAlertsWorkspace.actions.goSync"),
      }
    }
    return nextFlowAction
  }

  const pullStateBadgeVariant = (status?: string): "outline" | "secondary" | "destructive" => {
    const normalized = normalizeStatus(status)
    if (normalized === "succeeded" || normalized === "success" || normalized === "completed") {
      return "outline"
    }
    if (normalized === "failed" || normalized === "error") {
      return "destructive"
    }
    return "secondary"
  }

  const queueRuntimeBadgeVariant = (status?: string): "outline" | "secondary" | "destructive" => {
    const normalized = normalizeStatus(status)
    if (
      normalized === "ready" ||
      normalized === "processed" ||
      normalized === "skipped" ||
      normalized === "dlq" ||
      normalized === "terminal" ||
      normalized === "succeeded" ||
      normalized === "resolved"
    ) {
      return "outline"
    }
    if (normalized === "failed" || normalized === "error" || normalized === "attempt_limit") {
      return "destructive"
    }
    return "secondary"
  }

  const queueBudgetBadgeVariant = (
    state?: string,
    remainingAttempts?: number
  ): "outline" | "secondary" | "destructive" => {
    if ((remainingAttempts || 0) > 0) {
      return "secondary"
    }
    if (normalizeStatus(state) === "attempt_limit") {
      return "destructive"
    }
    return "outline"
  }

  const approvalClosureItems = useMemo(() => {
    const pendingCount = approvals.filter((item) => normalizeStatus(item.status) === "pending").length
    const syncFailedCount = approvals.filter((item) => classifyExternalSyncStatus(item.external_sync_status) === "failed").length
    const syncPendingCount = approvals.filter((item) => classifyExternalSyncStatus(item.external_sync_status) === "pending").length

    return [
      {
        key: "pending",
        title: t("enterpriseAlertsWorkspace.approvalClosure.pending.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: pendingCount }),
        statusVariant: pendingCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: pendingCount > 0
          ? t("enterpriseAlertsWorkspace.approvalClosure.pending.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.approvalClosure.pending.descriptionEmpty"),
        onLocate: () => {
          setLandingView("approval_backlog")
          setApprovalStatusFilter("pending")
          setApprovalSyncFilter("all")
        },
        locateLabel: t("enterpriseAlertsWorkspace.approvalClosure.pending.locate"),
        batchActions: [
          {
            key: "approve_pending",
            label: t("enterpriseAlertsWorkspace.approvalClosure.pending.batchApprove", { count: allPendingApprovalIDs.length }),
            disabled: Boolean(approvalBatchDisabledReason(Boolean(onBatchReviewApprovals), allPendingApprovalIDs.length, "noPendingApprovals")),
            disabledReason: approvalBatchDisabledReason(Boolean(onBatchReviewApprovals), allPendingApprovalIDs.length, "noPendingApprovals"),
            onClick: () => {
              if (!onBatchReviewApprovals || allPendingApprovalIDs.length === 0) {
                return
              }
              void onBatchReviewApprovals(allPendingApprovalIDs, "approved")
            },
          },
          {
            key: "reject_pending",
            label: t("enterpriseAlertsWorkspace.approvalClosure.pending.batchReject", { count: allPendingApprovalIDs.length }),
            disabled: Boolean(approvalBatchDisabledReason(Boolean(onBatchReviewApprovals), allPendingApprovalIDs.length, "noPendingApprovals")),
            disabledReason: approvalBatchDisabledReason(Boolean(onBatchReviewApprovals), allPendingApprovalIDs.length, "noPendingApprovals"),
            onClick: () => {
              if (!onBatchReviewApprovals || allPendingApprovalIDs.length === 0) {
                return
              }
              void onBatchReviewApprovals(allPendingApprovalIDs, "rejected")
            },
          },
        ],
      },
      {
        key: "sync_failed",
        title: t("enterpriseAlertsWorkspace.approvalClosure.syncFailed.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: syncFailedCount }),
        statusVariant: syncFailedCount > 0 ? ("destructive" as const) : ("outline" as const),
        description: syncFailedCount > 0
          ? t("enterpriseAlertsWorkspace.approvalClosure.syncFailed.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.approvalClosure.syncFailed.descriptionEmpty"),
        onLocate: () => {
          setLandingView("approval_backlog")
          setApprovalStatusFilter("all")
          setApprovalSyncFilter("failed")
        },
        locateLabel: t("enterpriseAlertsWorkspace.approvalClosure.syncFailed.locate"),
        batchActions: [
          {
            key: "mark_failed_synced",
            label: t("enterpriseAlertsWorkspace.approvalClosure.syncFailed.batchMarkSynced", { count: allFailedSyncApprovalIDs.length }),
            disabled: Boolean(approvalBatchDisabledReason(Boolean(onBatchUpdateApprovalExternalSync), allFailedSyncApprovalIDs.length, "noExternalSyncFailures")),
            disabledReason: approvalBatchDisabledReason(Boolean(onBatchUpdateApprovalExternalSync), allFailedSyncApprovalIDs.length, "noExternalSyncFailures"),
            onClick: () => {
              if (!onBatchUpdateApprovalExternalSync || allFailedSyncApprovalIDs.length === 0) {
                return
              }
              void onBatchUpdateApprovalExternalSync(allFailedSyncApprovalIDs, "synced")
            },
          },
        ],
      },
      {
        key: "sync_pending",
        title: t("enterpriseAlertsWorkspace.approvalClosure.syncPending.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: syncPendingCount }),
        statusVariant: syncPendingCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: syncPendingCount > 0
          ? t("enterpriseAlertsWorkspace.approvalClosure.syncPending.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.approvalClosure.syncPending.descriptionEmpty"),
        onLocate: () => {
          setLandingView("approval_backlog")
          setApprovalStatusFilter("all")
          setApprovalSyncFilter("pending")
        },
        locateLabel: t("enterpriseAlertsWorkspace.approvalClosure.syncPending.locate"),
        batchActions: [
          {
            key: "mark_pending_synced",
            label: t("enterpriseAlertsWorkspace.approvalClosure.syncPending.batchMarkSynced", { count: allPendingSyncApprovalIDs.length }),
            disabled: Boolean(approvalBatchDisabledReason(Boolean(onBatchUpdateApprovalExternalSync), allPendingSyncApprovalIDs.length, "noExternalSyncPending")),
            disabledReason: approvalBatchDisabledReason(Boolean(onBatchUpdateApprovalExternalSync), allPendingSyncApprovalIDs.length, "noExternalSyncPending"),
            onClick: () => {
              if (!onBatchUpdateApprovalExternalSync || allPendingSyncApprovalIDs.length === 0) {
                return
              }
              void onBatchUpdateApprovalExternalSync(allPendingSyncApprovalIDs, "synced")
            },
          },
        ],
      },
    ]
  }, [
    allFailedSyncApprovalIDs,
    allPendingApprovalIDs,
    allPendingSyncApprovalIDs,
    approvalActionBusy,
    approvalActionDisabledReason,
    approvals,
    onBatchReviewApprovals,
    onBatchUpdateApprovalExternalSync,
    writable,
    t,
  ])

  const directoryClosureItems = useMemo(() => {
    const unfinishedCount = syncJobs.filter((item) => classifySyncJob(item) === "attention").length
    const rejectedCount = syncJobs.filter((item) => classifySyncJob(item) === "rejected").length
    const deactivatedCount = syncJobs.filter((item) => classifySyncJob(item) === "deactivated").length
    const workerIssueCount = workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) !== "stable").length

    return [
      {
        key: "unfinished",
        title: t("enterpriseAlertsWorkspace.directoryClosure.unfinished.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: unfinishedCount }),
        statusVariant: unfinishedCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: unfinishedCount > 0
          ? t("enterpriseAlertsWorkspace.directoryClosure.unfinished.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.directoryClosure.unfinished.descriptionEmpty"),
        onLocate: () => {
          setLandingView("directory_exceptions")
          setSyncStatusFilter("attention")
          setSyncSourceFilter("all")
        },
        locateLabel: t("enterpriseAlertsWorkspace.directoryClosure.unfinished.locate"),
      },
      {
        key: "rejected",
        title: t("enterpriseAlertsWorkspace.directoryClosure.rejected.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: rejectedCount }),
        statusVariant: rejectedCount > 0 ? ("destructive" as const) : ("outline" as const),
        description: rejectedCount > 0
          ? t("enterpriseAlertsWorkspace.directoryClosure.rejected.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.directoryClosure.rejected.descriptionEmpty"),
        onLocate: () => {
          setLandingView("directory_exceptions")
          setSyncStatusFilter("rejected")
          setSyncSourceFilter("all")
        },
        locateLabel: t("enterpriseAlertsWorkspace.directoryClosure.rejected.locate"),
      },
      {
        key: "deactivated",
        title: t("enterpriseAlertsWorkspace.directoryClosure.deactivated.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: deactivatedCount }),
        statusVariant: deactivatedCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: deactivatedCount > 0
          ? t("enterpriseAlertsWorkspace.directoryClosure.deactivated.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.directoryClosure.deactivated.descriptionEmpty"),
        onLocate: () => {
          setLandingView("directory_exceptions")
          setSyncStatusFilter("deactivated")
          setSyncSourceFilter("all")
        },
        locateLabel: t("enterpriseAlertsWorkspace.directoryClosure.deactivated.locate"),
      },
      {
        key: "worker",
        title: t("enterpriseAlertsWorkspace.directoryClosure.worker.title"),
        statusLabel: t("enterpriseAlertsWorkspace.count", { count: workerIssueCount }),
        statusVariant: workerIssueCount > 0 ? ("destructive" as const) : ("outline" as const),
        description: workerIssueCount > 0
          ? t("enterpriseAlertsWorkspace.directoryClosure.worker.descriptionWithItems")
          : t("enterpriseAlertsWorkspace.directoryClosure.worker.descriptionEmpty"),
        onLocate: () => {
          setLandingView("directory_exceptions")
          setWorkerFilter("alerting")
        },
        locateLabel: t("enterpriseAlertsWorkspace.directoryClosure.worker.locate"),
      },
    ]
  }, [syncJobs, t, workerAlerts])

  function goToLandingView(view: AlertLandingView) {
    setLandingView(view)
    goToSection("alerts")
  }

  function withRouteHints(baseLink: string, hints: Record<string, string>) {
    const [pathPart, hashPart] = baseLink.split("#")
    const [pathname, rawQuery = ""] = pathPart.split("?")
    const query = new URLSearchParams(rawQuery)
    Object.entries(hints).forEach(([key, value]) => {
      const normalizedKey = key.trim()
      if (!normalizedKey) {
        return
      }
      const normalizedValue = (value || "").trim()
      if (!normalizedValue) {
        query.delete(normalizedKey)
        return
      }
      query.set(normalizedKey, normalizedValue)
    })
    const nextQuery = query.toString()
    const nextPath = nextQuery ? `${pathname}?${nextQuery}` : pathname
    return hashPart ? `${nextPath}#${hashPart}` : nextPath
  }

  function buildSyncJobScopedLinks(item: EnterpriseSyncJob) {
    const category = classifySyncJob(item)
    const sourceLabel = item.source.trim()
      ? item.source.trim().toUpperCase()
      : t("enterpriseAlertsWorkspace.hints.sync")
    const syncJobID = item.id.trim()
    const syncSource = item.source.trim()
    const syncStatus = item.status.trim()
    const remediationHint =
      category === "rejected" ? "sync_rejected_cleanup" : category === "deactivated" ? "deactivated_cleanup" : "sync_attention"
    const syncSummaryLabel = syncJobID
      ? t("enterpriseAlertsWorkspace.hints.syncTaskWithID", { source: sourceLabel, id: syncJobID })
      : t("enterpriseAlertsWorkspace.hints.syncTask", { source: sourceLabel })

    return {
      directory: withRouteHints(directoryLink, {
        group_desc: t("enterpriseAlertsWorkspace.hints.sourcePrefix", { summary: syncSummaryLabel }),
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: category === "deactivated" ? "deactivated" : "",
        group_name: t("enterpriseAlertsWorkspace.hints.syncReviewGroup", { source: sourceLabel }),
        remediation_hint: remediationHint,
        sync_category: category,
        sync_job_id: syncJobID,
        sync_source: syncSource,
        sync_status: syncStatus,
      }),
      policies: withRouteHints(policiesLink, {
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: category === "deactivated" ? "deactivated" : "",
        policy_group: t("enterpriseAlertsWorkspace.hints.syncReviewGroup", { source: sourceLabel }),
        policy_name: t("enterpriseAlertsWorkspace.hints.syncReviewPolicy", { source: sourceLabel }),
        remediation_hint: remediationHint,
        sync_category: category,
        sync_job_id: syncJobID,
        sync_source: syncSource,
        sync_status: syncStatus,
      }),
      wallet: withRouteHints(walletLink, {
        sync_category: category,
        sync_job_id: syncJobID,
        sync_source: syncSource,
        sync_status: syncStatus,
        sync_query_hint: syncJobID,
        target_email: "",
        target_id: "",
        target_ids: "",
        target_name: "",
        template_hint: "employee",
      }),
    }
  }

  function buildWorkerAlertScopedLinks(item: EnterpriseSyncWorkerAlertSummaryItem) {
    const category = classifyEnterpriseSyncWorkerAlertLevel(item)
    const remediationHint = category === "hot" ? "worker_hot_alert" : category === "alerting" ? "worker_alerting" : ""
    const workerScopeLabel = selectedTenantName || item.tenant_id
    const workerLabel = item.worker_label?.trim() || item.worker_action?.trim() || workerScopeLabel
    const workerSummaryLabel =
      remediationHint && item.last_seen_at
        ? t("enterpriseAlertsWorkspace.hints.workerAlertWithTime", {
            scope: `${workerScopeLabel} · ${workerLabel}`,
            at: formatDateTime(item.last_seen_at),
          })
        : t("enterpriseAlertsWorkspace.hints.workerAlert", { scope: `${workerScopeLabel} · ${workerLabel}` })
    const workerFilterHint = category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"

    return {
      directory: withRouteHints(directoryLink, {
        group_desc: t("enterpriseAlertsWorkspace.hints.sourcePrefix", { summary: workerSummaryLabel }),
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        group_name: t("enterpriseAlertsWorkspace.hints.workerReviewGroup", { scope: `${workerScopeLabel} · ${workerLabel}` }),
        remediation_hint: remediationHint,
        worker_action: item.worker_action || "",
        worker_alert_failed: String(item.last_failed),
        worker_alert_label: item.worker_label || "",
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
        worker_kind: item.worker_kind || "",
      }),
      policies: withRouteHints(policiesLink, {
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        policy_group: t("enterpriseAlertsWorkspace.hints.workerReviewGroup", { scope: `${workerScopeLabel} · ${workerLabel}` }),
        policy_name: t("enterpriseAlertsWorkspace.hints.workerReviewPolicy", { scope: `${workerScopeLabel} · ${workerLabel}` }),
        remediation_hint: remediationHint,
        worker_action: item.worker_action || "",
        worker_alert_failed: String(item.last_failed),
        worker_alert_label: item.worker_label || "",
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
        worker_kind: item.worker_kind || "",
      }),
      wallet: withRouteHints(walletLink, {
        template_hint: "employee",
        worker_action: item.worker_action || "",
        worker_alert_failed: String(item.last_failed),
        worker_alert_label: item.worker_label || "",
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
        worker_kind: item.worker_kind || "",
        worker_filter_hint: workerFilterHint,
        worker_query_hint: item.tenant_id,
      }),
      sync: withRouteHints(syncLink, {
        sync_focus_hint: "worker_alert",
        worker_filter_hint: workerFilterHint,
        worker_query_hint: item.tenant_id,
        worker_action: item.worker_action || "",
        worker_alert_failed: String(item.last_failed),
        worker_alert_label: item.worker_label || "",
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
        worker_kind: item.worker_kind || "",
      }),
    }
  }

  function workerFilterHint(item?: EnterpriseSyncWorkerAlertSummaryItem | null) {
    if (!item) {
      return ""
    }
    const category = classifyEnterpriseSyncWorkerAlertLevel(item)
    return category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"
  }

  function matchWorkerSummary(
    primaryKeyword: string,
    extraKeywords: string[] = []
  ): EnterpriseSyncWorkerAlertSummaryItem | null {
    const normalizedPrimary = primaryKeyword.trim().toLowerCase()
    const normalizedExtraKeywords = extraKeywords.map((value) => value.trim().toLowerCase()).filter(Boolean)
    if (!normalizedPrimary) {
      return null
    }
    const strictMatch = workerAlerts.find((item) => {
      const haystack = [item.worker_action || "", item.worker_kind || "", item.worker_label || ""].join(" ").toLowerCase()
      return haystack.includes(normalizedPrimary) && normalizedExtraKeywords.every((keyword) => haystack.includes(keyword))
    })
    if (strictMatch) {
      return strictMatch
    }
    return (
      workerAlerts.find((item) => {
        const haystack = [item.worker_action || "", item.worker_kind || "", item.worker_label || ""].join(" ").toLowerCase()
        return haystack.includes(normalizedPrimary)
      }) || null
    )
  }

  function buildWorkerEventSyncLink(item: EnterpriseSyncWorkerAlertItem) {
    const summaryItem =
      workerAlerts.find(
        (candidate) =>
          (candidate.worker_action || "").trim().toLowerCase() === (item.worker_action || "").trim().toLowerCase() &&
          (candidate.worker_kind || "").trim().toLowerCase() === (item.worker_kind || "").trim().toLowerCase()
      ) || null
    return withRouteHints(syncLink, {
      sync_focus_hint: "worker_alert",
      worker_action: item.worker_action || summaryItem?.worker_action || "",
      worker_alert_label: item.worker_label || summaryItem?.worker_label || "",
      worker_filter_hint: workerFilterHint(summaryItem),
      worker_kind: item.worker_kind || summaryItem?.worker_kind || "",
      worker_connector_id: item.connector_id || "",
      worker_failure_stage: item.failure_stage || "",
      worker_request_id: item.request_id || "",
      worker_vendor: item.vendor || "",
    })
  }

  function buildWebhookReceiptSyncLink(item: EnterpriseHRISWebhookReceipt) {
    const summaryItem =
      matchWorkerSummary("receipt", [item.vendor || ""]) ||
      matchWorkerSummary("processing", [item.vendor || ""]) ||
      matchWorkerSummary("webhook", [item.vendor || ""])
    return withRouteHints(syncLink, {
      sync_focus_hint: "worker_alert",
      worker_action: summaryItem?.worker_action || "",
      worker_alert_label: summaryItem?.worker_label || "",
      worker_filter_hint: workerFilterHint(summaryItem),
      worker_kind: summaryItem?.worker_kind || "",
      worker_connector_id: item.connector_id,
      worker_event_type: item.event_type || "",
      worker_queue_state: item.queue_state,
      worker_request_id: item.request_id || "",
      worker_status: item.status,
      worker_vendor: item.vendor || "",
    })
  }

  function buildPullStateSyncLink(item: EnterpriseHRISPullState) {
    const summaryItem = matchWorkerSummary("pull", [item.vendor || ""])
    return withRouteHints(syncLink, {
      sync_focus_hint: "worker_alert",
      worker_action: summaryItem?.worker_action || "",
      worker_alert_label: summaryItem?.worker_label || "",
      worker_filter_hint: workerFilterHint(summaryItem),
      worker_kind: summaryItem?.worker_kind || "",
      worker_connector_id: item.connector_id,
      worker_mode: item.last_mode || "",
      worker_request_id: item.last_request_id || "",
      worker_vendor: item.vendor || "",
    })
  }

  function buildDLQSyncLink(item: EnterpriseHRISWebhookDLQEntry) {
    const summaryItem =
      matchWorkerSummary("dlq", [item.vendor || ""]) ||
      matchWorkerSummary("replay", [item.vendor || ""]) ||
      matchWorkerSummary("processing", [item.vendor || ""])
    return withRouteHints(syncLink, {
      sync_focus_hint: "worker_alert",
      worker_action: summaryItem?.worker_action || "",
      worker_alert_label: summaryItem?.worker_label || "",
      worker_filter_hint: workerFilterHint(summaryItem),
      worker_kind: summaryItem?.worker_kind || "",
      worker_connector_id: item.connector_id || "",
      worker_failure_stage: item.failure_stage || "",
      worker_replay_state: item.replay_state || "",
      worker_request_id: item.request_id || "",
      worker_status: item.status || "",
      worker_vendor: item.vendor || "",
    })
  }

  function buildWebhookExecutionSyncLink(item: EnterpriseHRISWebhookExecution) {
    const summaryItem =
      item.kind === "dlq_replay"
        ? matchWorkerSummary("dlq", [item.vendor || ""]) ||
          matchWorkerSummary("replay", [item.vendor || ""]) ||
          matchWorkerSummary("processing", [item.vendor || ""])
        : matchWorkerSummary("receipt", [item.vendor || ""]) ||
          matchWorkerSummary("processing", [item.vendor || ""]) ||
          matchWorkerSummary("webhook", [item.vendor || ""])
    const executionReplayScope = item.replay_require_worker
      ? "worker_required"
      : item.replay_source_execution_id
        ? "replayed"
        : ""
    return withRouteHints(syncLink, {
      sync_focus_hint: "worker_alert",
      execution_id: item.id,
      execution_kind: item.kind || "",
      execution_mode: item.execution_mode || "",
      execution_queue_state: item.queue_state || "",
      execution_replay_scope: executionReplayScope,
      execution_status: item.status || "",
      worker_action: summaryItem?.worker_action || "",
      worker_alert_label: summaryItem?.worker_label || "",
      worker_filter_hint: workerFilterHint(summaryItem),
      worker_kind: summaryItem?.worker_kind || "",
      worker_connector_id: item.connector_id || "",
      worker_failure_stage: item.failure_stage || "",
      worker_mode: item.execution_mode || "",
      worker_request_id: item.request_id || "",
      worker_status: item.status || "",
      worker_vendor: item.vendor || "",
    })
  }

  const workerBatchScopeLabel = useMemo(() => {
    if (workerLabelScope.trim()) {
      return workerLabelScope.trim()
    }
    if (workerActionScope.trim()) {
      return workerActionScope.trim()
    }
    if (workerKindScope.trim()) {
      return workerKindScope.trim()
    }
    if (directoryQuery.trim()) {
      return directoryQuery.trim()
    }
    return selectedTenantName || t("enterpriseAlertsWorkspace.hints.workerAlert", { scope: "batch" })
  }, [directoryQuery, selectedTenantName, t, workerActionScope, workerKindScope, workerLabelScope])

  const workerBatchSummaryLabel = useMemo(() => {
    if (directoryQuery.trim()) {
      return t("enterpriseAlertsWorkspace.hints.workerAlert", {
        scope: `${workerBatchScopeLabel} · ${directoryQuery.trim()}`,
      })
    }
    return t("enterpriseAlertsWorkspace.hints.workerAlert", {
      scope: workerBatchScopeLabel,
    })
  }, [directoryQuery, t, workerBatchScopeLabel])

  const workerBatchDirectoryLink = withRouteHints(directoryLink, {
    group_desc: t("enterpriseAlertsWorkspace.hints.sourcePrefix", { summary: workerBatchSummaryLabel }),
    group_member_email: "",
    group_member_id: "",
    group_member_name: "",
    group_member_status: "",
    group_name: t("enterpriseAlertsWorkspace.hints.workerReviewGroup", { scope: workerBatchScopeLabel }),
    worker_action: workerActionScope,
    worker_alert_label: workerLabelScope,
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_kind: workerKindScope,
    worker_queue_state: workerQueueStateScope,
    worker_query_hint: directoryQuery,
    worker_replay_state: workerReplayStateScope,
    worker_status: workerStatusScope,
  })

  const workerBatchPoliciesLink = withRouteHints(policiesLink, {
    group_member_email: "",
    group_member_id: "",
    group_member_name: "",
    group_member_status: "",
    policy_group: t("enterpriseAlertsWorkspace.hints.workerReviewGroup", { scope: workerBatchScopeLabel }),
    policy_name: t("enterpriseAlertsWorkspace.hints.workerReviewPolicy", { scope: workerBatchScopeLabel }),
    worker_action: workerActionScope,
    worker_alert_label: workerLabelScope,
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_kind: workerKindScope,
    worker_queue_state: workerQueueStateScope,
    worker_query_hint: directoryQuery,
    worker_replay_state: workerReplayStateScope,
    worker_status: workerStatusScope,
  })

  const workerBatchSyncLink = withRouteHints(syncLink, {
    sync_focus_hint: "worker_alert",
    worker_action: workerActionScope,
    worker_alert_label: workerLabelScope,
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_kind: workerKindScope,
    worker_queue_state: workerQueueStateScope,
    worker_query_hint: directoryQuery,
    worker_replay_state: workerReplayStateScope,
    worker_status: workerStatusScope,
  })

  const renderActionButton = (action: EnterpriseLandingAction, variant: "default" | "outline" = "outline") => {
    if (action.kind === "section") {
      return (
        <Button
          size="sm"
          variant={variant}
          onClick={() => {
            if (action.section === "alerts" && action.alertsView) {
              setLandingView(action.alertsView)
            }
            goToSection(action.section!)
          }}
        >
          {action.label}
        </Button>
      )
    }
    return (
      <Button asChild size="sm" variant={variant}>
        <Link to={action.to!}>{action.label}</Link>
      </Button>
    )
  }

  return (
    <TabsContent value="alerts">
      <div className="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <Card className="xl:col-span-2">
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.header.title")}</CardTitle>
            <CardDescription>{t("enterpriseAlertsWorkspace.header.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <Button
                data-testid="enterprise-alerts-tab-overview"
                size="sm"
                variant={landingView === "overview" ? "default" : "outline"}
                onClick={() => goToLandingView("overview")}
              >
                {t("enterpriseAlertsWorkspace.header.tabs.overview")}
              </Button>
              <Button
                data-testid="enterprise-alerts-tab-approval-backlog"
                size="sm"
                variant={landingView === "approval_backlog" ? "default" : "outline"}
                onClick={() => goToLandingView("approval_backlog")}
              >
                {t("enterpriseAlertsWorkspace.header.tabs.approvalBacklog", { count: approvalLandingIssueCount })}
              </Button>
              <Button
                data-testid="enterprise-alerts-tab-directory-exceptions"
                size="sm"
                variant={landingView === "directory_exceptions" ? "default" : "outline"}
                onClick={() => goToLandingView("directory_exceptions")}
              >
                {t("enterpriseAlertsWorkspace.header.tabs.directoryExceptions", { count: directoryLandingIssueCount })}
              </Button>
            </div>
            <div className="rounded-lg border bg-muted/10 px-3 py-2 text-sm text-muted-foreground">
              {landingView === "overview"
                ? t("enterpriseAlertsWorkspace.header.hint.overview")
                : landingView === "approval_backlog"
                  ? t("enterpriseAlertsWorkspace.header.hint.approvalBacklog")
                  : t("enterpriseAlertsWorkspace.header.hint.directoryExceptions")}
            </div>
          </CardContent>
        </Card>

        {receiptRecoveryFlowEnabled ? (
          <Card className="xl:col-span-2" data-testid="enterprise-alerts-receipt-recovery">
            <CardHeader>
              <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.receiptRecovery.title")}</CardTitle>
              <CardDescription>{t("enterpriseAlertsWorkspace.receiptRecovery.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="rounded-xl border bg-muted/10 px-4 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="font-medium">{t("enterpriseAlertsWorkspace.receiptRecovery.currentStatus")}</p>
                  <Badge variant={receiptRecoverySegmentStatusVariant}>{receiptRecoverySegmentStatusLabel}</Badge>
                  <Badge variant={receiptRecoveryBlockerCount > 0 ? "secondary" : "outline"}>
                    {receiptRecoveryBlockerCount > 0
                      ? t("enterpriseAlertsWorkspace.receiptRecovery.pendingCount", { count: receiptRecoveryBlockerCount })
                      : t("enterpriseAlertsWorkspace.receiptRecovery.readyToBackflow")}
                  </Badge>
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  {receiptRecoveryBlockerCount > 0
                    ? t("enterpriseAlertsWorkspace.receiptRecovery.blockedHint")
                    : t("enterpriseAlertsWorkspace.receiptRecovery.readyHint")}
                </p>
              </div>
              {loading ? (
                <p className="mp-kpi-note">{t("enterpriseAlertsWorkspace.receiptRecovery.loading")}</p>
              ) : (
                <>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button asChild size="sm">
                      <Link to={receiptRecoveryBackflowLinks.retry} data-testid="enterprise-alerts-receipt-retry-link">
                        {t("enterpriseAlertsWorkspace.receiptRecovery.actions.retry")}
                      </Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={receiptRecoveryBackflowLinks.repair}>
                        {t("enterpriseAlertsWorkspace.receiptRecovery.actions.repair")}
                      </Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={receiptRecoveryBackflowLinks.closed}>
                        {t("enterpriseAlertsWorkspace.receiptRecovery.actions.closed")}
                      </Link>
                    </Button>
                  </div>
                  <p className="mp-kpi-note">
                    {receiptRecoveryQueryHint
                      ? t("enterpriseAlertsWorkspace.receiptRecovery.queryHintWithValue", { hint: receiptRecoveryQueryHint })
                      : t("enterpriseAlertsWorkspace.receiptRecovery.queryHintEmpty")}
                  </p>
                </>
              )}
            </CardContent>
          </Card>
        ) : null}

        {showOverviewCards ? (
          <Card className="xl:col-span-2">
            <CardHeader>
              <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.landingActions.title")}</CardTitle>
              <CardDescription>{t("enterpriseAlertsWorkspace.landingActions.description")}</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 md:grid-cols-3">
              {landingCards.map((item) => (
                <div key={item.title} className="rounded-xl border bg-muted/10 px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{item.title}</p>
                    <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {item.action.kind === "section" ? (
                      <Button size="sm" variant="outline" onClick={() => goToSection(item.action.section!)}>
                        {item.action.label}
                      </Button>
                    ) : (
                      <Button asChild size="sm" variant="outline">
                        <Link to={item.action.to!}>{item.action.label}</Link>
                      </Button>
                    )}
                    {item.returnAction ? (
                      item.returnAction.kind === "section" ? (
                        <Button size="sm" onClick={() => goToSection(item.returnAction!.section!)}>
                          {item.returnAction.label}
                        </Button>
                      ) : (
                        <Button asChild size="sm">
                          <Link to={item.returnAction.to!}>{item.returnAction.label}</Link>
                        </Button>
                      )
                    ) : null}
                  </div>
                  {item.returnHint ? <p className="mt-2 text-xs text-muted-foreground">{item.returnHint}</p> : null}
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}

        {showOverviewCards ? (
          <Card className="xl:col-span-2">
            <CardHeader>
              <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.blockers.title")}</CardTitle>
              <CardDescription>{t("enterpriseAlertsWorkspace.blockers.description")}</CardDescription>
            </CardHeader>
            <CardContent>
              {attentionItems.length === 0 ? (
                <div className="rounded-xl border bg-muted/10 px-4 py-3">
                  <p className="font-medium">{t("enterpriseAlertsWorkspace.blockers.emptyTitle")}</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t("enterpriseAlertsWorkspace.blockers.emptyDescription")}
                  </p>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <Button asChild size="sm" variant="outline">
                      <Link to={policiesLink}>{t("enterpriseAlertsWorkspace.blockers.goPolicies")}</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={walletLink}>{t("enterpriseAlertsWorkspace.blockers.goWallet")}</Link>
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="grid gap-3 md:grid-cols-2">
                  {attentionItems.map((item) => (
                    <div key={item.title} className="rounded-xl border bg-muted/10 px-4 py-3">
                      <p className="font-medium">{item.title}</p>
                      <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                      <Button size="sm" variant="outline" className="mt-3" onClick={item.onClick}>
                        {item.actionLabel}
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        ) : null}

        {showApprovalCards ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.approvalClosureCard.title")}</CardTitle>
              <CardDescription>{t("enterpriseAlertsWorkspace.approvalClosureCard.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {approvalClosureItems.map((item) => (
                <div key={item.key} className="rounded-lg border bg-muted/15 p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{item.title}</p>
                    <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{item.description}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="outline" onClick={item.onLocate}>
                      {item.locateLabel}
                    </Button>
                    {renderActionButton(nextFlowAction)}
                  </div>
                  {item.batchActions?.length ? (
                    <div className="mt-2 flex flex-wrap items-center gap-2">
                      {item.batchActions.map((action) => (
                        <Button
                          key={action.key}
                          size="sm"
                          variant="outline"
                          disabled={action.disabled}
                          title={action.disabledReason || undefined}
                          onClick={action.onClick}
                        >
                          {action.label}
                        </Button>
                      ))}
                      {item.batchActions.find((action) => action.disabledReason) ? (
                        <p className="w-full basis-full text-xs text-muted-foreground">
                          {item.batchActions.find((action) => action.disabledReason)?.disabledReason}
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}

        {showDirectoryCards ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.directoryClosureCard.title")}</CardTitle>
              <CardDescription>{t("enterpriseAlertsWorkspace.directoryClosureCard.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {directoryClosureItems.map((item) => (
                <div key={item.key} className="rounded-lg border bg-muted/15 p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{item.title}</p>
                    <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{item.description}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="outline" onClick={item.onLocate}>
                      {item.locateLabel}
                    </Button>
                    {renderActionButton(nextFlowAction)}
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}

        <Card className="xl:col-span-2">
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.recoveryActions.title")}</CardTitle>
            <CardDescription>{t("enterpriseAlertsWorkspace.recoveryActions.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">{alertRecoveryAction.title}</p>
                <Badge variant={alertRecoveryAction.blockerCount > 0 ? "secondary" : "outline"}>
                  {alertRecoveryAction.blockerCount > 0
                    ? t("enterpriseAlertsWorkspace.recoveryActions.blockerCount", { count: alertRecoveryAction.blockerCount })
                    : t("enterpriseAlertsWorkspace.recoveryActions.canBackflow")}
                </Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">{alertRecoveryAction.description}</p>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{alertRecoveryAction.nextAction.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{alertRecoveryAction.nextAction.description}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                {alertRecoveryAction.nextAction.kind === "section" ? (
                  <Button size="sm" onClick={() => goToSection(alertRecoveryAction.nextAction.section!)}>
                    {alertRecoveryAction.nextAction.label}
                  </Button>
                ) : (
                  <Button asChild size="sm">
                    <Link to={alertRecoveryAction.nextAction.to!}>{alertRecoveryAction.nextAction.label}</Link>
                  </Button>
                )}
                <Button asChild size="sm" variant="outline">
                  <Link to={directoryLink}>{t("enterpriseAlertsWorkspace.recoveryActions.goDirectory")}</Link>
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={policiesLink}>{t("enterpriseAlertsWorkspace.recoveryActions.goPolicies")}</Link>
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        {showApprovalCards ? (
          <EnterpriseJITApprovalInbox
            title={t("enterpriseAlertsWorkspace.jitApproval.title")}
            description={t("enterpriseAlertsWorkspace.jitApproval.description")}
          >
            <div className="rounded-lg border bg-muted/15 p-3">
              <p className="text-sm font-medium">{t("enterpriseAlertsWorkspace.jitApproval.statusFilterTitle")}</p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {approvalStatusOptions.map((status) => (
                  <Button
                    key={status}
                    size="sm"
                    variant={approvalStatusFilter === status ? "default" : "outline"}
                    onClick={() => setApprovalStatusFilter(status)}
                  >
                    {status === "all"
                      ? t("enterpriseAlertsWorkspace.jitApproval.statusOptions.all", { count: approvalCounts.all })
                      : status === "pending"
                        ? t("enterpriseAlertsWorkspace.jitApproval.statusOptions.pending", { count: approvalCounts.pending })
                        : status === "approved"
                          ? t("enterpriseAlertsWorkspace.jitApproval.statusOptions.approved", { count: approvalCounts.approved })
                          : status === "rejected"
                            ? t("enterpriseAlertsWorkspace.jitApproval.statusOptions.rejected", { count: approvalCounts.rejected })
                            : status}
                  </Button>
                ))}
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {[
                  { value: "all", label: t("enterpriseAlertsWorkspace.jitApproval.syncOptions.all", { count: approvalCounts.all }) },
                  { value: "failed", label: t("enterpriseAlertsWorkspace.jitApproval.syncOptions.failed", { count: approvalCounts.syncFailed }) },
                  { value: "pending", label: t("enterpriseAlertsWorkspace.jitApproval.syncOptions.pending", { count: approvalCounts.syncPending }) },
                  { value: "success", label: t("enterpriseAlertsWorkspace.jitApproval.syncOptions.success", { count: approvalCounts.syncSuccess }) },
                  { value: "none", label: t("enterpriseAlertsWorkspace.jitApproval.syncOptions.none", { count: approvalCounts.syncNone }) },
                ].map((item) => (
                  <Button
                    key={item.value}
                    size="sm"
                    variant={approvalSyncFilter === item.value ? "default" : "outline"}
                    onClick={() => setApprovalSyncFilter(item.value as "all" | "failed" | "pending" | "success" | "none")}
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
              <div className="mt-2 flex items-center gap-2">
                <Input
                  value={approvalQuery}
                  onChange={(event) => setApprovalQuery(event.target.value)}
                  placeholder={t("enterpriseAlertsWorkspace.jitApproval.queryPlaceholder")}
                  className="h-8"
                />
                {approvalQuery.trim() ? (
                  <Button size="sm" variant="outline" onClick={() => setApprovalQuery("")}>
                    {t("enterpriseAlertsWorkspace.common.clear")}
                  </Button>
                ) : null}
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={Boolean(jitBatchApprovalDisabledReason)}
                  title={jitBatchApprovalDisabledReason || undefined}
                  onClick={() => {
                    if (!onBatchReviewApprovals || batchPendingApprovalIDs.length === 0) {
                      return
                    }
                    void onBatchReviewApprovals(batchPendingApprovalIDs, "approved")
                  }}
                >
                  {t("enterpriseAlertsWorkspace.jitApproval.batchApprove", { count: batchPendingApprovalIDs.length })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={Boolean(jitBatchApprovalDisabledReason)}
                  title={jitBatchApprovalDisabledReason || undefined}
                  onClick={() => {
                    if (!onBatchReviewApprovals || batchPendingApprovalIDs.length === 0) {
                      return
                    }
                    void onBatchReviewApprovals(batchPendingApprovalIDs, "rejected")
                  }}
                >
                  {t("enterpriseAlertsWorkspace.jitApproval.batchReject", { count: batchPendingApprovalIDs.length })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={Boolean(jitBatchExternalSyncDisabledReason)}
                  title={jitBatchExternalSyncDisabledReason || undefined}
                  onClick={() => {
                    if (!onBatchUpdateApprovalExternalSync || batchSyncMarkableApprovalIDs.length === 0) {
                      return
                    }
                    void onBatchUpdateApprovalExternalSync(batchSyncMarkableApprovalIDs, "synced")
                  }}
                >
                  {t("enterpriseAlertsWorkspace.jitApproval.batchMarkSynced", { count: batchSyncMarkableApprovalIDs.length })}
                </Button>
                {jitBatchApprovalDisabledReason || jitBatchExternalSyncDisabledReason ? (
                  <p className="w-full basis-full text-xs text-muted-foreground">
                    {jitBatchApprovalDisabledReason || jitBatchExternalSyncDisabledReason}
                  </p>
                ) : null}
              </div>
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("enterpriseAlertsWorkspace.jitApproval.table.email")}</TableHead>
                  <TableHead>{t("enterpriseAlertsWorkspace.jitApproval.table.status")}</TableHead>
                  <TableHead>{t("enterpriseAlertsWorkspace.jitApproval.table.externalSync")}</TableHead>
                  <TableHead>{t("enterpriseAlertsWorkspace.jitApproval.table.updatedAt")}</TableHead>
                  <TableHead>{t("enterpriseAlertsWorkspace.jitApproval.table.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {!loading && filteredApprovals.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                      {t("enterpriseAlertsWorkspace.jitApproval.empty")}
                    </TableCell>
                  </TableRow>
                ) : null}
                {filteredApprovals.slice(0, 10).map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.email}</TableCell>
                    <TableCell>
                      <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                    </TableCell>
                    <TableCell>{item.external_sync_status || t("enterpriseAlertsWorkspace.common.emptyDash")}</TableCell>
                    <TableCell>{formatDateTime(item.updated_at)}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-2">
                        {normalizeStatus(item.status) === "pending" && onReviewApproval ? (
                          <>
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={Boolean(approvalActionDisabledReason) || approvalActionID === item.id}
                              title={
                                approvalActionDisabledReason ||
                                (approvalActionID === item.id ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : undefined)
                              }
                              onClick={() => {
                                void onReviewApproval(item.id, "approved")
                              }}
                            >
                              {approvalActionID === item.id
                                ? t("enterpriseAlertsWorkspace.common.processing")
                                : t("enterpriseAlertsWorkspace.jitApproval.actions.approve")}
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={Boolean(approvalActionDisabledReason) || approvalActionID === item.id}
                              title={
                                approvalActionDisabledReason ||
                                (approvalActionID === item.id ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : undefined)
                              }
                              onClick={() => {
                                void onReviewApproval(item.id, "rejected")
                              }}
                            >
                              {approvalActionID === item.id
                                ? t("enterpriseAlertsWorkspace.common.processing")
                                : t("enterpriseAlertsWorkspace.jitApproval.actions.reject")}
                            </Button>
                          </>
                        ) : null}
                        {normalizeStatus(item.status) !== "pending" &&
                        classifyExternalSyncStatus(item.external_sync_status) !== "success" &&
                        onUpdateApprovalExternalSync ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={Boolean(approvalActionDisabledReason) || approvalActionID === item.id}
                            title={
                              approvalActionDisabledReason ||
                              (approvalActionID === item.id ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : undefined)
                            }
                            onClick={() => {
                              void onUpdateApprovalExternalSync(item.id, "synced")
                            }}
                          >
                            {approvalActionID === item.id
                              ? t("enterpriseAlertsWorkspace.common.processing")
                              : t("enterpriseAlertsWorkspace.jitApproval.actions.markSynced")}
                          </Button>
                        ) : null}
                        {renderActionButton(resolveApprovalAction(item), "outline")}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </EnterpriseJITApprovalInbox>
        ) : null}

        {showDirectoryCards ? (
          <Card data-testid="enterprise-alerts-sync-worker-card">
            <CardHeader>
              <CardTitle className="text-base">{t("enterpriseAlertsWorkspace.syncAndWorker.title")}</CardTitle>
              <CardDescription>{t("enterpriseAlertsWorkspace.syncAndWorker.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
            {workerAlertSubscription ? (
              <EnterpriseSyncWorkerAlertSubscriptionCard
                dispatching={Boolean(workerAlertDispatching)}
                formatDateTime={formatDateTime}
                onDispatch={async () => {
                  if (!onDispatchWorkerAlerts) {
                    return
                  }
                  await onDispatchWorkerAlerts()
                }}
                onSave={async (payload) => {
                  if (!onSaveWorkerAlertSubscription) {
                    return
                  }
                  await onSaveWorkerAlertSubscription(payload)
                }}
                saving={Boolean(workerAlertSubscriptionSaving)}
                subscription={workerAlertSubscription}
                writable={writable}
              />
            ) : null}
            <EnterpriseSyncExceptions
              title={t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.title")}
              actions={
                <Badge variant="secondary">
                  {filteredSyncJobs.length} / {syncJobs.length}
                </Badge>
              }
            >
              <div className="flex flex-wrap items-center gap-2">
                {[
                  { value: "all", label: t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.filters.all", { count: syncJobCounts.all }) },
                  { value: "attention", label: t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.filters.attention", { count: syncJobCounts.attention }) },
                  { value: "rejected", label: t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.filters.rejected", { count: syncJobCounts.rejected }) },
                  { value: "deactivated", label: t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.filters.deactivated", { count: syncJobCounts.deactivated }) },
                  { value: "healthy", label: t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.filters.healthy", { count: syncJobCounts.healthy }) },
                ].map((item) => (
                  <Button
                    key={item.value}
                    size="sm"
                    variant={syncStatusFilter === item.value ? "default" : "outline"}
                    onClick={() =>
                      setSyncStatusFilter(item.value as "all" | "attention" | "rejected" | "deactivated" | "healthy")
                    }
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {syncSourceOptions.map((source) => (
                  <Button
                    key={source}
                    size="sm"
                    variant={syncSourceFilter === source ? "default" : "outline"}
                    onClick={() => setSyncSourceFilter(source)}
                  >
                    {source === "all"
                      ? t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.sourceAll")
                      : t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.sourceValue", { source })}
                  </Button>
                ))}
              </div>
              <div className="flex items-center gap-2">
                <Input
                  data-testid="enterprise-alerts-directory-query"
                  value={directoryQuery}
                  onChange={(event) => setDirectoryQuery(event.target.value)}
                  placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.queryPlaceholder")}
                  className="h-8"
                />
                {directoryQuery.trim() ? (
                  <Button size="sm" variant="outline" onClick={() => setDirectoryQuery("")}>
                    {t("enterpriseAlertsWorkspace.common.clear")}
                  </Button>
                ) : null}
              </div>
              <div className="rounded-md border bg-background/70 p-3">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <p className="text-sm font-medium">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.title")}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.description")}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {pendingSyncRequests.length > 0 && onReconcilePendingSyncRequests ? (
                      <Button
                        size="sm"
                        variant="outline"
                        data-testid="enterprise-alerts-sync-request-reconcile-pending"
                        disabled={syncRequestActionBusy}
                        onClick={() => {
                          void onReconcilePendingSyncRequests()
                        }}
                      >
                        {syncRequestActionBusy
                          ? t("enterpriseAlertsWorkspace.common.processing")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.batchReconcile")}
                      </Button>
                    ) : null}
                    <Badge variant="secondary">
                      {pendingSyncRequests.length} / {syncRequests.length}
                    </Badge>
                  </div>
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  {t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.batchHint")}
                </p>
                <div className="mt-3 space-y-2">
                  {filteredPendingSyncRequests.slice(0, 4).map((item) => (
                    <div
                      key={item.request_id}
                      className="rounded-md border bg-muted/10 px-3 py-2 text-sm"
                      data-testid="enterprise-alerts-sync-request-item"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="font-medium">{item.request_id}</p>
                        <div className="flex flex-wrap items-center gap-2">
                          {item.connector_id ? <Badge variant="secondary">{item.connector_id}</Badge> : null}
                          <Badge variant="outline">{t("enterpriseAlertsWorkspace.count", { count: item.access_attempt_count })}</Badge>
                        </div>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.meta", {
                          jobID: item.result.job.id || t("enterpriseAlertsWorkspace.common.emptyDash"),
                          connector: item.connector_id || t("enterpriseAlertsWorkspace.common.emptyDash"),
                          createdAt: formatDateTime(item.created_at),
                        })}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.attempts", {
                          attemptCount: item.access_attempt_count,
                          attemptedAt: formatDateTime(item.last_access_attempt_at),
                        })}
                      </p>
                      {item.last_access_error ? (
                        <p className="mt-1 line-clamp-3 text-xs text-muted-foreground">{item.last_access_error}</p>
                      ) : null}
                    </div>
                  ))}
                  {!loading && filteredPendingSyncRequests.length === 0 ? (
                    <p
                      className="text-sm text-muted-foreground"
                      data-testid="enterprise-alerts-sync-request-empty"
                    >
                      {t("enterpriseAlertsWorkspace.syncAndWorker.pendingSyncRequests.empty")}
                    </p>
                  ) : null}
                </div>
              </div>
              <div className="mt-3 space-y-2">
                {filteredSyncJobs.slice(0, 6).map((item) => {
                  const category = classifySyncJob(item)
                  const action = resolveSyncJobAction(item)
                  const scopedLinks = buildSyncJobScopedLinks(item)
                  return (
                    <div key={item.id} className="rounded-md border bg-background px-3 py-2 text-sm">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="font-medium">{item.source}</p>
                          <p className="mp-kpi-note">{item.id}</p>
                        </div>
                        <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        created {item.created} / updated {item.updated} / deactivated {item.deactivated} / rejected {item.rejected}
                      </p>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        {renderActionButton(action, "outline")}
                        {category !== "healthy" ? (
                          <>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.directory}>{t("enterpriseAlertsWorkspace.syncAndWorker.actions.jobToDirectory")}</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.policies}>{t("enterpriseAlertsWorkspace.syncAndWorker.actions.jobToPolicies")}</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.wallet}>{t("enterpriseAlertsWorkspace.syncAndWorker.actions.jobToWallet")}</Link>
                            </Button>
                          </>
                        ) : null}
                        {category !== "healthy" ? renderActionButton(nextFlowAction) : null}
                      </div>
                    </div>
                  )
                })}
                {!loading && filteredSyncJobs.length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t("enterpriseAlertsWorkspace.syncAndWorker.syncJobs.empty")}</p>
                ) : null}
              </div>
            </EnterpriseSyncExceptions>

            <EnterpriseWorkerAlerts
              title={t("enterpriseAlertsWorkspace.syncAndWorker.workerAlerts.title")}
              actions={
                <div className="flex flex-wrap items-center gap-2">
                  {filteredWorkerAlerts.length > 0 ? (
                    <>
                      <Button asChild size="sm" variant="outline">
                        <Link to={workerBatchDirectoryLink} data-testid="enterprise-alerts-worker-batch-to-directory">
                          {t("enterpriseAlertsWorkspace.recoveryActions.goDirectory")}
                        </Link>
                      </Button>
                      <Button asChild size="sm" variant="outline">
                        <Link to={workerBatchPoliciesLink} data-testid="enterprise-alerts-worker-batch-to-policies">
                          {t("enterpriseAlertsWorkspace.recoveryActions.goPolicies")}
                        </Link>
                      </Button>
                      <Button asChild size="sm" variant="outline">
                        <Link to={workerBatchSyncLink} data-testid="enterprise-alerts-worker-batch-to-sync">
                          {t("enterpriseAlertsWorkspace.actions.goSync")}
                        </Link>
                      </Button>
                    </>
                  ) : null}
                  <Badge variant="secondary">
                    {filteredWorkerAlerts.length} / {workerAlerts.length}
                  </Badge>
                </div>
              }
            >
              <div className="flex flex-wrap items-center gap-2">
                {[
                  { value: "all", label: t("enterpriseAlertsWorkspace.syncAndWorker.workerAlerts.filters.all", { count: workerCounts.all }) },
                  { value: "alerting", label: t("enterpriseAlertsWorkspace.syncAndWorker.workerAlerts.filters.alerting", { count: workerCounts.alerting }) },
                  { value: "hot", label: t("enterpriseAlertsWorkspace.syncAndWorker.workerAlerts.filters.hot", { count: workerCounts.hot }) },
                  { value: "stable", label: t("enterpriseAlertsWorkspace.syncAndWorker.workerAlerts.filters.stable", { count: workerCounts.stable }) },
                ].map((item) => (
                  <Button
                    key={item.value}
                    data-testid={`enterprise-alerts-worker-filter-${item.value}`}
                    size="sm"
                    variant={workerFilter === item.value ? "default" : "outline"}
                    onClick={() => setWorkerFilter(item.value as "all" | "alerting" | "hot" | "stable")}
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
              {workerActionScope ||
              workerLabelScope ||
              workerKindScope ||
              workerQueueStateScope ||
              workerReplayStateScope ||
              workerStatusScope ? (
                <div className="flex flex-wrap items-center gap-2" data-testid="enterprise-alerts-worker-alert-scope">
                  {workerLabelScope ? <Badge variant="secondary">{workerLabelScope}</Badge> : null}
                  {workerActionScope ? <Badge variant="secondary">{workerActionScope}</Badge> : null}
                  {workerKindScope ? <Badge variant="outline">{workerKindScope}</Badge> : null}
                  {workerStatusScope ? <Badge variant="outline">{workerStatusScope}</Badge> : null}
                  {workerQueueStateScope ? <Badge variant="outline">{workerQueueStateScope}</Badge> : null}
                  {workerReplayStateScope ? <Badge variant="outline">{workerReplayStateScope}</Badge> : null}
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setWorkerActionScope("")
                      setWorkerKindScope("")
                      setWorkerLabelScope("")
                      setWorkerQueueStateScope("")
                      setWorkerReplayStateScope("")
                      setWorkerStatusScope("")
                    }}
                  >
                    {t("enterpriseAlertsWorkspace.common.clear")}
                  </Button>
                </div>
              ) : null}
              <div className="mt-3 space-y-2">
                {filteredWorkerAlerts.slice(0, 6).map((item) => {
                  const action = resolveWorkerAction(item)
                  const category = classifyEnterpriseSyncWorkerAlertLevel(item)
                  const guidance = describeEnterpriseSyncWorkerAlertGuidance(item, t)
                  const scopedLinks = buildWorkerAlertScopedLinks(item)
                  const workerTitle = item.worker_label?.trim() || item.worker_action?.trim() || "Worker Alert"
                  return (
                    <div
                      key={`${item.tenant_id}-${item.worker_action || "worker"}-${item.last_seen_at}`}
                      className="rounded-md border bg-background px-3 py-2 text-sm"
                      data-testid="enterprise-alerts-worker-alert-item"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="font-medium">{workerTitle}</p>
                          <p className="text-xs text-muted-foreground">{selectedTenantName ?? item.tenant_id}</p>
                        </div>
                        <Badge variant={item.count > 0 ? "destructive" : "outline"}>{item.count}</Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        last failed {item.last_failed} / threshold {item.last_threshold} / {formatDateTime(item.last_seen_at)}
                      </p>
                      {guidance ? (
                        <div
                          className="mt-2 rounded-md border border-muted-foreground/15 bg-muted/10 px-3 py-2"
                          data-testid="enterprise-alerts-worker-alert-guidance"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <p
                              className="text-xs font-medium"
                              data-testid="enterprise-alerts-worker-alert-guidance-title"
                            >
                              {guidance.title}
                            </p>
                            <Badge
                              variant={guidance.badgeVariant}
                              data-testid="enterprise-alerts-worker-alert-guidance-badge"
                            >
                              {guidance.badgeLabel}
                            </Badge>
                          </div>
                          <p
                            className="mt-1 text-xs text-muted-foreground"
                            data-testid="enterprise-alerts-worker-alert-guidance-summary"
                          >
                            {guidance.summary}
                          </p>
                        </div>
                      ) : null}
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        {renderActionButton(action, "outline")}
                        {category !== "stable" ? (
                          <>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.directory}>{t("enterpriseAlertsWorkspace.syncAndWorker.actions.alertToDirectory")}</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.policies}>{t("enterpriseAlertsWorkspace.syncAndWorker.actions.alertToPolicies")}</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.wallet}>{t("enterpriseAlertsWorkspace.syncAndWorker.actions.alertToWallet")}</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.sync} data-testid="enterprise-alerts-worker-alert-to-sync">
                                {t("enterpriseAlertsWorkspace.syncAndWorker.actions.alertToSync")}
                              </Link>
                            </Button>
                          </>
                        ) : null}
                        {category !== "stable" ? renderActionButton(nextFlowAction) : null}
                      </div>
                    </div>
                  )
                })}
                {!loading && filteredWorkerAlerts.length === 0 ? (
                  <p
                    className="text-sm text-muted-foreground"
                    data-testid="enterprise-alerts-worker-alert-empty"
                  >
                    {t("enterpriseAlertsWorkspace.syncAndWorker.workerAlerts.empty")}
                  </p>
                ) : null}
              </div>
              <div className="rounded-md border bg-background/70 p-3" data-testid="enterprise-alerts-worker-notification-history">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="text-sm font-medium">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.title")}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.description", {
                        count: notificationHistoryTotal,
                      })}
                    </p>
                  </div>
                  <Badge variant="secondary">
                    {filteredWorkerAlertNotifications.length}
                    {notificationHistoryTotal !== filteredWorkerAlertNotifications.length
                      ? ` / ${notificationHistoryTotal}`
                      : ""}
                  </Badge>
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  {(
                    [
                      {
                        value: "all",
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.filters.all"),
                      },
                      {
                        value: "failed",
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.filters.failed"),
                      },
                      {
                        value: "retryable",
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.filters.retryable"),
                      },
                      {
                        value: "suppressed",
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.filters.suppressed"),
                      },
                      {
                        value: "due_now",
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.filters.dueNow"),
                      },
                    ] as const
                  ).map((option) => (
                    <Button
                      key={option.value}
                      size="sm"
                      type="button"
                      variant={notificationHistoryFilter === option.value ? "secondary" : "outline"}
                      onClick={() => {
                        setNotificationHistoryFilter(option.value)
                      }}
                      data-testid={`enterprise-alerts-worker-notification-filter-${option.value}`}
                    >
                      {option.label} ({notificationHistoryCounts[option.value]})
                    </Button>
                  ))}
                </div>
                <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
                  <Input
                    value={notificationHistoryQuery}
                    onChange={(event) => {
                      setNotificationHistoryQuery(event.target.value)
                    }}
                    className="h-9 w-full max-w-md"
                    placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.searchPlaceholder")}
                    data-testid="enterprise-alerts-worker-notification-query"
                  />
                  <Button
                    size="sm"
                    type="button"
                    variant="outline"
                    disabled={Boolean(notificationExportDisabledReason)}
                    title={notificationExportDisabledReason || undefined}
                    data-testid="enterprise-alerts-worker-notification-export-visible"
                    onClick={() => {
                      if (onExportWorkerAlertNotifications) {
                        void onExportWorkerAlertNotifications({
                          filter: notificationHistoryFilter,
                          query: notificationHistoryQuery,
                        })
                        return
                      }
                      exportVisibleWorkerAlertNotifications()
                    }}
                  >
                    {exportingWorkerAlertNotifications
                      ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.exporting")
                      : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.exportVisible", {
                          count: notificationHistoryTotal,
                        })}
                  </Button>
                </div>
                <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.statusSummary.sent", {
                        count: notificationHistoryStatusCounts.sent,
                      })}
                    </Badge>
                    <Badge variant="destructive">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.statusSummary.failed", {
                        count: notificationHistoryStatusCounts.failed,
                      })}
                    </Badge>
                    <Badge variant="secondary">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.statusSummary.skipped", {
                        count: notificationHistoryStatusCounts.skipped,
                      })}
                    </Badge>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {onAutoRetryWorkerAlertNotifications ? (
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={Boolean(autoRetryNotificationsDisabledReason)}
                        title={autoRetryNotificationsDisabledReason || undefined}
                        data-testid="enterprise-alerts-worker-notification-auto-retry-due"
                        onClick={() => {
                          void onAutoRetryWorkerAlertNotifications()
                        }}
                      >
                        {autoRetryingWorkerAlertNotifications
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.autoRetryDueBusy")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.autoRetryDue", {
                              count: dueWorkerAlertNotificationCount,
                            })}
                      </Button>
                    ) : null}
                    {onBatchSuppressWorkerAlertNotifications ? (
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={Boolean(suppressNotificationsDisabledReason)}
                        title={suppressNotificationsDisabledReason || undefined}
                        data-testid="enterprise-alerts-worker-notification-suppress-visible"
                        onClick={() => {
                          void onBatchSuppressWorkerAlertNotifications(visibleFailedWorkerAlertNotificationIDs)
                        }}
                      >
                        {suppressingWorkerAlertNotificationBatch
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.suppressVisibleBusy")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.suppressVisible", {
                              count: visibleFailedWorkerAlertNotificationIDs.length,
                            })}
                      </Button>
                    ) : null}
                    {onBatchRestoreWorkerAlertNotifications ? (
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={Boolean(restoreNotificationsDisabledReason)}
                        title={restoreNotificationsDisabledReason || undefined}
                        data-testid="enterprise-alerts-worker-notification-restore-visible"
                        onClick={() => {
                          void onBatchRestoreWorkerAlertNotifications(visibleSuppressedWorkerAlertNotificationIDs)
                        }}
                      >
                        {restoringWorkerAlertNotificationBatch
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoreVisibleBusy")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoreVisible", {
                              count: visibleSuppressedWorkerAlertNotificationIDs.length,
                            })}
                      </Button>
                    ) : null}
                    {onBatchRetryWorkerAlertNotifications ? (
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={Boolean(retryNotificationsDisabledReason)}
                        title={retryNotificationsDisabledReason || undefined}
                        data-testid="enterprise-alerts-worker-notification-retry-visible"
                        onClick={() => {
                          void onBatchRetryWorkerAlertNotifications(visibleRetryableWorkerAlertNotificationIDs)
                        }}
                      >
                        {retryingWorkerAlertNotificationBatch
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.retryVisibleBusy")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.retryVisible", {
                              count: visibleRetryableWorkerAlertNotificationIDs.length,
                            })}
                      </Button>
                    ) : null}
                  </div>
                  {autoRetryNotificationsDisabledReason ||
                  suppressNotificationsDisabledReason ||
                  restoreNotificationsDisabledReason ||
                  retryNotificationsDisabledReason ? (
                    <p className="w-full basis-full text-xs text-muted-foreground">
                      {autoRetryNotificationsDisabledReason ||
                        suppressNotificationsDisabledReason ||
                        restoreNotificationsDisabledReason ||
                        retryNotificationsDisabledReason}
                    </p>
                  ) : null}
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.batchHint")}
                </p>
                <div className="mt-3 overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.time")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.worker")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.countThreshold")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.attempt")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.channels")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.receivers")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.status")}</TableHead>
                        <TableHead>{t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.columns.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {notificationHistoryLoadingState ? (
                        <TableRow>
                          <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                            {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.loading")}
                          </TableCell>
                        </TableRow>
                      ) : null}
                      {!notificationHistoryLoadingState && workerAlertNotifications.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                            {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.empty")}
                          </TableCell>
                        </TableRow>
                      ) : null}
                      {!notificationHistoryLoadingState &&
                      workerAlertNotifications.length > 0 &&
                      filteredWorkerAlertNotifications.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                            {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.emptyFiltered")}
                          </TableCell>
                        </TableRow>
                      ) : null}
                      {!notificationHistoryLoadingState &&
                        filteredWorkerAlertNotifications.map((item) => {
                          const detailsExpanded = expandedWorkerAlertNotificationID === item.id
                          return (
                            <Fragment key={item.id}>
                              <TableRow data-testid="enterprise-alerts-worker-notification-row">
                                <TableCell className="mp-kpi-note">{formatDateTime(item.triggered_at)}</TableCell>
                                <TableCell>
                                  <div className="space-y-1">
                                    <p className="font-medium">{item.worker_label || item.worker_action}</p>
                                    <p className="mp-kpi-note">{item.worker_action || item.fingerprint}</p>
                                    {[
                                      item.connector_id,
                                      item.vendor,
                                      item.failure_stage,
                                      item.mode,
                                      item.event_type,
                                    ]
                                      .filter(Boolean)
                                      .length > 0 ? (
                                      <p className="mp-kpi-note">
                                        {[
                                          item.connector_id,
                                          item.vendor,
                                          item.failure_stage,
                                          item.mode,
                                          item.event_type,
                                        ]
                                          .filter(Boolean)
                                          .join(" / ")}
                                      </p>
                                    ) : null}
                                    {item.source_notification_id ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.retrySource", {
                                          id: item.source_notification_id,
                                        })}
                                      </p>
                                    ) : null}
                                  </div>
                                </TableCell>
                                <TableCell>
                                  {item.count} / {item.threshold}
                                </TableCell>
                                <TableCell>{item.attempt ?? t("enterpriseAlertsWorkspace.common.emptyDash")}</TableCell>
                                <TableCell className="mp-kpi-note">
                                  {item.channels && item.channels.length > 0
                                    ? item.channels.join(", ")
                                    : t("enterpriseAlertsWorkspace.common.emptyDash")}
                                </TableCell>
                                <TableCell className="mp-kpi-note">
                                  {item.receiver_groups && item.receiver_groups.length > 0
                                    ? item.receiver_groups.join(", ")
                                    : t("enterpriseAlertsWorkspace.common.emptyDash")}
                                </TableCell>
                                <TableCell>
                                  <div className="space-y-1">
                                    <Badge variant={statusBadgeVariant(item.status)}>
                                      {item.status}
                                      {item.reason ? ` (${item.reason})` : ""}
                                    </Badge>
                                    {isWorkerAlertNotificationConfirmationPending(item) &&
                                    hasWorkerAlertNotificationPendingAge(item.pending_age_seconds) ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.pendingAge.status", {
                                          age: formatWorkerAlertNotificationPendingAge(item.pending_age_seconds),
                                        })}
                                      </p>
                                    ) : null}
                                    {item.next_retry_at ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.nextRetryAt", {
                                          at: formatDateTime(item.next_retry_at),
                                        })}
                                      </p>
                                    ) : null}
                                    {typeof item.confirm_attempts === "number" ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.confirmAttempts", {
                                          count: item.confirm_attempts,
                                        })}
                                      </p>
                                    ) : null}
                                    {item.last_confirm_result ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.lastConfirmResult", {
                                          result: item.last_confirm_result,
                                        })}
                                      </p>
                                    ) : null}
                                    {item.last_confirm_attempt_at ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.lastConfirmAttemptAt", {
                                          at: formatDateTime(item.last_confirm_attempt_at),
                                        })}
                                      </p>
                                    ) : null}
                                    {item.channel_results && item.channel_results.length > 0 ? (
                                      <p className="mp-kpi-note">
                                        {item.channel_results
                                          .map((result) =>
                                            result.reason
                                              ? `${result.channel}:${result.status}(${result.reason})`
                                              : `${result.channel}:${result.status}`
                                          )
                                          .join(" | ")}
                                      </p>
                                    ) : null}
                                    {item.provider || item.provider_error ? (
                                      <p className="mp-kpi-note">
                                        {[item.provider, item.provider_error].filter(Boolean).join(" / ")}
                                      </p>
                                    ) : null}
                                    {item.status === "skipped" &&
                                    item.reason === "manual_suppressed" &&
                                    item.restore_status &&
                                    item.restore_status !== "ready" ? (
                                      <p className="mp-kpi-note">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoreStatus", {
                                          status: formatWorkerAlertNotificationRestoreStatus(item.restore_status),
                                        })}
                                      </p>
                                    ) : null}
                                  </div>
                                </TableCell>
                                <TableCell>
                                  <div className="flex flex-wrap items-center gap-2">
                                    {item.status === "failed" && item.retryable && onRetryWorkerAlertNotification ? (
                                      <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={Boolean(notificationActionDisabledReason) || retryingWorkerAlertNotificationID === item.id}
                                        title={
                                          notificationActionDisabledReason ||
                                          (retryingWorkerAlertNotificationID === item.id
                                            ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy")
                                            : undefined)
                                        }
                                        onClick={() => {
                                          void onRetryWorkerAlertNotification(item.id)
                                        }}
                                      >
                                        {retryingWorkerAlertNotificationID === item.id
                                          ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.retrying")
                                          : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.retry")}
                                      </Button>
                                    ) : null}
                                    {item.status === "failed" && onBatchSuppressWorkerAlertNotifications ? (
                                      <Button
                                        size="sm"
                                        type="button"
                                        variant="outline"
                                        data-testid={`enterprise-alerts-worker-notification-suppress-${item.id}`}
                                        disabled={Boolean(notificationActionDisabledReason)}
                                        title={notificationActionDisabledReason || undefined}
                                        onClick={() => {
                                          void onBatchSuppressWorkerAlertNotifications([item.id])
                                        }}
                                      >
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.suppress")}
                                      </Button>
                                    ) : null}
                                    {item.status === "skipped" &&
                                    item.reason === "manual_suppressed" &&
                                    (!item.restore_status || item.restore_status === "ready") &&
                                    onBatchRestoreWorkerAlertNotifications ? (
                                      <Button
                                        size="sm"
                                        type="button"
                                        variant="outline"
                                        data-testid={`enterprise-alerts-worker-notification-restore-${item.id}`}
                                        disabled={Boolean(notificationActionDisabledReason)}
                                        title={notificationActionDisabledReason || undefined}
                                        onClick={() => {
                                          void onBatchRestoreWorkerAlertNotifications([item.id])
                                        }}
                                      >
                                        {restoringWorkerAlertNotificationBatch
                                          ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restoring")
                                          : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.restore")}
                                      </Button>
                                    ) : null}
                                    <Button
                                      size="sm"
                                      type="button"
                                      variant={detailsExpanded ? "secondary" : "outline"}
                                      data-testid={`enterprise-alerts-worker-notification-details-toggle-${item.id}`}
                                      onClick={() => {
                                        setExpandedWorkerAlertNotificationID((current) => (current === item.id ? null : item.id))
                                      }}
                                    >
                                      {detailsExpanded
                                        ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.hideDetails")
                                        : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.details")}
                                    </Button>
                                  </div>
                                </TableCell>
                              </TableRow>
                              {detailsExpanded ? (
                                <TableRow data-testid={`enterprise-alerts-worker-notification-details-row-${item.id}`}>
                                  <TableCell colSpan={8} className="bg-muted/10">
                                    <div
                                      className="grid gap-3 py-1 text-sm md:grid-cols-2"
                                      data-testid={`enterprise-alerts-worker-notification-details-panel-${item.id}`}
                                    >
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsFingerprint")}
                                        </p>
                                        <p className="break-all">{item.fingerprint || t("enterpriseAlertsWorkspace.common.emptyDash")}</p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsIdempotencyKey")}
                                        </p>
                                        <p className="break-all">
                                          {item.idempotency_key || t("enterpriseAlertsWorkspace.common.emptyDash")}
                                        </p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsRequestID")}
                                        </p>
                                        <p className="break-all">{item.request_id || t("enterpriseAlertsWorkspace.common.emptyDash")}</p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsSourceNotificationID")}
                                        </p>
                                        <p className="break-all">
                                          {item.source_notification_id || t("enterpriseAlertsWorkspace.common.emptyDash")}
                                        </p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsRestoreStatus")}
                                        </p>
                                        <p className="break-all">
                                          {formatWorkerAlertNotificationRestoreStatus(item.restore_status)}
                                        </p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsPendingAge")}
                                        </p>
                                        <p>{formatWorkerAlertNotificationPendingAge(item.pending_age_seconds)}</p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsConfirmAttempts")}
                                        </p>
                                        <p>{item.confirm_attempts ?? t("enterpriseAlertsWorkspace.common.emptyDash")}</p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsNextRetryAt")}
                                        </p>
                                        <p>{item.next_retry_at ? formatDateTime(item.next_retry_at) : t("enterpriseAlertsWorkspace.common.emptyDash")}</p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsLastConfirmResult")}
                                        </p>
                                        <p className="break-all">
                                          {item.last_confirm_result || t("enterpriseAlertsWorkspace.common.emptyDash")}
                                        </p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsLastConfirmAttemptAt")}
                                        </p>
                                        <p>
                                          {item.last_confirm_attempt_at
                                            ? formatDateTime(item.last_confirm_attempt_at)
                                            : t("enterpriseAlertsWorkspace.common.emptyDash")}
                                        </p>
                                      </div>
                                      <div className="space-y-1">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsProviderError")}
                                        </p>
                                        <p className="break-all">
                                          {item.provider_error || t("enterpriseAlertsWorkspace.common.emptyDash")}
                                        </p>
                                      </div>
                                      <div className="space-y-1 md:col-span-2">
                                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                          {t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.detailsChannelResults")}
                                        </p>
                                        {item.channel_results && item.channel_results.length > 0 ? (
                                          <div className="space-y-1">
                                            {item.channel_results.map((result, index) => (
                                              <p
                                                key={`${item.id}-channel-result-${index}`}
                                                className="break-all rounded-md border bg-background/70 px-2 py-1 text-xs"
                                              >
                                                {[
                                                  `${result.channel}:${result.status}`,
                                                  result.reason ? `reason=${result.reason}` : "",
                                                  result.provider ? `provider=${result.provider}` : "",
                                                  result.provider_error ? `error=${result.provider_error}` : "",
                                                  result.receivers && result.receivers.length > 0
                                                    ? `receivers=${result.receivers.join(", ")}`
                                                    : "",
                                                ]
                                                  .filter(Boolean)
                                                  .join(" / ")}
                                              </p>
                                            ))}
                                          </div>
                                        ) : (
                                          <p>{t("enterpriseAlertsWorkspace.common.emptyDash")}</p>
                                        )}
                                      </div>
                                    </div>
                                  </TableCell>
                                </TableRow>
                              ) : null}
                            </Fragment>
                          )
                        })}
                    </TableBody>
                  </Table>
                </div>
                {workerAlertNotificationHasMore && onLoadMoreWorkerAlertNotifications ? (
                  <div className="mt-3 flex justify-center">
                    <Button
                      size="sm"
                      type="button"
                      variant="outline"
                      disabled={notificationHistoryLoadingState || workerAlertNotificationLoadingMore}
                      data-testid="enterprise-alerts-worker-notification-load-more"
                      onClick={() => {
                        void onLoadMoreWorkerAlertNotifications()
                      }}
                    >
                      {workerAlertNotificationLoadingMore
                        ? t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.loadingMore")
                        : t("enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.loadMore")}
                    </Button>
                  </div>
                ) : null}
              </div>
              <div className="grid gap-3 lg:grid-cols-3">
                <div className="rounded-md border bg-background/70 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm font-medium">{t("enterpriseAlertsWorkspace.syncAndWorker.workerEvents.title")}</p>
                    <Badge variant="secondary">
                      {filteredWorkerAlertEvents.length} / {workerAlertEvents.length}
                    </Badge>
                  </div>
                  <div className="mt-3 space-y-2">
                    {filteredWorkerAlertEvents.slice(0, 4).map((item) => (
                      <div
                        key={item.id}
                        className="rounded-md border bg-muted/10 px-3 py-2 text-sm"
                        data-testid="enterprise-alerts-worker-event-item"
                      >
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <p className="font-medium">{item.worker_label || item.worker_action}</p>
                          <div className="flex flex-wrap items-center gap-2">
                            {item.vendor ? <Badge variant="secondary">{item.vendor}</Badge> : null}
                            {item.failure_stage ? <Badge variant="outline">{item.failure_stage}</Badge> : null}
                            {item.mode ? <Badge variant="outline">{item.mode}</Badge> : null}
                          </div>
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.workerEvents.metrics", {
                            applied: item.applied,
                            failed: item.failed,
                            processed: item.processed,
                            threshold: item.threshold,
                          })}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.workerEvents.meta", {
                            at: formatDateTime(item.at),
                            connector: item.connector_id || "-",
                            request: item.request_id || "-",
                          })}
                        </p>
                        {item.event_type ? (
                          <p className="mt-1 break-all text-xs text-muted-foreground">{item.event_type}</p>
                        ) : null}
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <Button asChild size="sm" variant="outline">
                            <Link to={buildWorkerEventSyncLink(item)} data-testid="enterprise-alerts-worker-event-to-sync">
                              {t("enterprisePage.actions.goToSync")}
                            </Link>
                          </Button>
                        </div>
                      </div>
                    ))}
                    {!loading && filteredWorkerAlertEvents.length === 0 ? (
                      <p
                        className="text-sm text-muted-foreground"
                        data-testid="enterprise-alerts-worker-event-empty"
                      >
                        {t("enterpriseAlertsWorkspace.syncAndWorker.workerEvents.empty")}
                      </p>
                    ) : null}
                  </div>
                </div>

                <div className="rounded-md border bg-background/70 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <p className="text-sm font-medium">
                        {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.title")}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.description")}
                      </p>
                    </div>
                    <Badge variant="secondary">
                      {hrisWebhookExecutions.length}
                      {executionHistoryTotalCount !== hrisWebhookExecutions.length ? ` / ${executionHistoryTotalCount}` : ""}
                    </Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {[
                      {
                        value: "all" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.kindFilter.all"),
                      },
                      {
                        value: "receipt_process" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.kindFilter.receiptProcess"),
                      },
                      {
                        value: "dlq_replay" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.kindFilter.dlqReplay"),
                      },
                    ].map((item) => (
                      <Button
                        key={item.value}
                        size="sm"
                        type="button"
                        variant={executionHistoryKindFilter === item.value ? "secondary" : "outline"}
                        onClick={() => {
                          setExecutionHistoryKindFilter(item.value)
                        }}
                      >
                        {item.label}
                      </Button>
                    ))}
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {executionHistoryStatusFilterOptions.map((item) => (
                      <Button
                        key={item.value}
                        size="sm"
                        type="button"
                        variant={executionHistoryStatusFilter === item.value ? "secondary" : "outline"}
                        onClick={() => {
                          setExecutionHistoryStatusFilter(item.value)
                        }}
                      >
                        {item.label} ({item.count})
                      </Button>
                    ))}
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {executionHistoryQueueStateFilterOptions.map((item) => (
                      <Button
                        key={item.value}
                        size="sm"
                        type="button"
                        variant={executionHistoryQueueStateFilter === item.value ? "secondary" : "outline"}
                        onClick={() => {
                          setExecutionHistoryQueueStateFilter(item.value)
                        }}
                      >
                        {item.label} ({item.count})
                      </Button>
                    ))}
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {executionHistoryReplayScopeFilterOptions.map((item) => (
                      <Button
                        key={item.value}
                        size="sm"
                        type="button"
                        variant={executionHistoryReplayScopeFilter === item.value ? "secondary" : "outline"}
                        onClick={() => {
                          setExecutionHistoryReplayScopeFilter(item.value)
                        }}
                      >
                        {item.label}
                      </Button>
                    ))}
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {[
                      {
                        value: "all" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.executionModeFilter.all"),
                      },
                      {
                        value: "queued" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.executionModeFilter.queued"),
                      },
                      {
                        value: "inline" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.executionModeFilter.inline"),
                      },
                    ].map((item) => (
                      <Button
                        key={item.value}
                        size="sm"
                        type="button"
                        variant={executionHistoryExecutionModeFilter === item.value ? "secondary" : "outline"}
                        onClick={() => {
                          setExecutionHistoryExecutionModeFilter(item.value)
                        }}
                      >
                        {item.label}
                      </Button>
                    ))}
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {[
                      {
                        value: "all" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.dispatchModeFilter.all"),
                      },
                      {
                        value: "worker_tick" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.dispatchModeFilter.workerTick"),
                      },
                      {
                        value: "worker_task_channel" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.dispatchModeFilter.workerTaskChannel"),
                      },
                      {
                        value: "goroutine_fallback" as const,
                        label: t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.dispatchModeFilter.goroutineFallback"),
                      },
                    ].map((item) => (
                      <Button
                        key={item.value}
                        size="sm"
                        type="button"
                        variant={executionHistoryDispatchModeFilter === item.value ? "secondary" : "outline"}
                        onClick={() => {
                          setExecutionHistoryDispatchModeFilter(item.value)
                        }}
                      >
                        {item.label}
                      </Button>
                    ))}
                  </div>
                  <div className="mt-3 grid gap-2 md:grid-cols-2">
                    <Input
                      value={executionHistoryQuery}
                      onChange={(event) => {
                        setExecutionHistoryQuery(event.target.value)
                      }}
                      placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.searchPlaceholder")}
                    />
                    <Input
                      value={executionHistoryTargetStatusFilter}
                      onChange={(event) => {
                        setExecutionHistoryTargetStatusFilter(event.target.value)
                      }}
                      placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.targetStatusPlaceholder")}
                    />
                  </div>
                  {selectedHRISWebhookExecutionID ? (
                    <div
                      className="mt-3 rounded-lg border bg-muted/10 px-3 py-3"
                      data-testid="enterprise-alerts-webhook-execution-detail"
                    >
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div className="space-y-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <p className="text-sm font-medium">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.title")}
                            </p>
                            <Badge variant="secondary">
                              {selectedHRISWebhookExecution?.id || selectedHRISWebhookExecutionID}
                            </Badge>
                            {selectedHRISWebhookExecution?.status ? (
                              <Badge variant={queueRuntimeBadgeVariant(selectedHRISWebhookExecution.status)}>
                                {formatLifecycleToken(selectedHRISWebhookExecution.status)}
                              </Badge>
                            ) : null}
                            {selectedHRISWebhookExecution?.queue_state ? (
                              <Badge variant={queueRuntimeBadgeVariant(selectedHRISWebhookExecution.queue_state)}>
                                {formatLifecycleToken(selectedHRISWebhookExecution.queue_state)}
                              </Badge>
                            ) : null}
                            {selectedHRISWebhookExecution?.target_status ? (
                              <Badge variant="outline">
                                {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.targetStatus", {
                                  status: formatLifecycleToken(selectedHRISWebhookExecution.target_status),
                                })}
                              </Badge>
                            ) : null}
                          </div>
                          <p className="text-xs text-muted-foreground">
                            {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.description")}
                          </p>
                          {selectedHRISWebhookExecutionURLHint ? (
                            <p
                              className="break-all rounded-md border bg-background/70 px-2 py-1 font-mono text-xs text-muted-foreground"
                              data-testid="enterprise-alerts-webhook-execution-url-hint"
                            >
                              {selectedHRISWebhookExecutionURLHint}
                            </p>
                          ) : null}
                          {!selectedHRISWebhookExecutionInCurrentScope && !executionHistoryLoadingState ? (
                            <p className="text-xs text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.filteredHint")}
                            </p>
                          ) : null}
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          {selectedHRISWebhookExecution &&
                          selectedHRISWebhookExecution.status === "failed" &&
                          onReplayHRISWebhookExecution ? (
                            <Button
                              size="sm"
                              type="button"
                              variant="outline"
                              data-testid="enterprise-alerts-webhook-execution-detail-replay"
                              disabled={!writable || Boolean(executionActionID)}
                              onClick={() => {
                                void onReplayHRISWebhookExecution(selectedHRISWebhookExecution.id)
                              }}
                            >
                              {executionActionID === selectedHRISWebhookExecution.id
                                ? t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replaying")
                                : t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replay")}
                            </Button>
                          ) : null}
                          {selectedHRISWebhookExecution ? (
                            <Button asChild size="sm" variant="outline">
                              <Link
                                to={buildWebhookExecutionSyncLink(selectedHRISWebhookExecution)}
                                data-testid="enterprise-alerts-webhook-execution-detail-to-sync"
                              >
                                {t("enterprisePage.actions.goToSync")}
                              </Link>
                            </Button>
                          ) : null}
                          {selectedHRISWebhookExecution?.replay_source_execution_id &&
                          selectedHRISWebhookExecution.replay_source_execution_id !==
                            selectedHRISWebhookExecution.id &&
                          onSelectHRISWebhookExecution ? (
                            <Button
                              size="sm"
                              type="button"
                              variant="outline"
                              data-testid="enterprise-alerts-webhook-execution-detail-open-source"
                              onClick={() => {
                                onSelectHRISWebhookExecution(
                                  selectedHRISWebhookExecution.replay_source_execution_id || null
                                )
                              }}
                            >
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.openSourceExecution")}
                            </Button>
                          ) : null}
                          {onSelectHRISWebhookExecution ? (
                            <Button
                              size="sm"
                              type="button"
                              variant="outline"
                              data-testid="enterprise-alerts-webhook-execution-detail-close"
                              onClick={() => {
                                onSelectHRISWebhookExecution(null)
                              }}
                            >
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.hideDetails")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                      {selectedHRISWebhookExecutionLoading ? (
                        <p className="mt-3 text-sm text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.loading")}
                        </p>
                      ) : null}
                      {selectedHRISWebhookExecutionError ? (
                        <div className="mt-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                          {selectedHRISWebhookExecutionError}
                        </div>
                      ) : null}
                      {selectedHRISWebhookExecution ? (
                        <div
                          className="mt-3 grid gap-3 text-sm md:grid-cols-2"
                          data-testid="enterprise-alerts-webhook-execution-detail-grid"
                        >
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.kind")}
                            </p>
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge variant="outline">
                                {formatExecutionKindLabel(selectedHRISWebhookExecution.kind)}
                              </Badge>
                              {selectedHRISWebhookExecution.vendor ? (
                                <Badge variant="secondary">{selectedHRISWebhookExecution.vendor}</Badge>
                              ) : null}
                            </div>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.timeline")}
                            </p>
                            <p>
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.timelineQueued", {
                                value: formatDateTime(selectedHRISWebhookExecution.queued_at),
                              })}
                            </p>
                            <p className="text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.startedAt", {
                                value: formatDateTime(selectedHRISWebhookExecution.started_at),
                              })}
                            </p>
                            <p className="text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.finishedAt", {
                                value: formatDateTime(selectedHRISWebhookExecution.finished_at),
                              })}
                            </p>
                            <p className="text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.timelineUpdated", {
                                value: formatDateTime(selectedHRISWebhookExecution.updated_at),
                              })}
                            </p>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.target")}
                            </p>
                            <p className="break-all">
                              {selectedHRISWebhookExecution.target_id || t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                            <p className="break-all text-muted-foreground">
                              {selectedHRISWebhookExecution.receipt_id || t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.connector")}
                            </p>
                            <p className="break-all">
                              {selectedHRISWebhookExecution.connector_id || t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                            <p className="text-muted-foreground">
                              {selectedHRISWebhookExecution.vendor || t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.request")}
                            </p>
                            <p className="break-all">
                              {selectedHRISWebhookExecution.request_id || t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                            <p className="break-all text-muted-foreground">
                              {selectedHRISWebhookExecution.event_type || t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.dispatch")}
                            </p>
                            <p>
                              {selectedHRISWebhookExecution.execution_mode
                                ? formatLifecycleToken(selectedHRISWebhookExecution.execution_mode)
                                : t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                            <p className="text-muted-foreground">
                              {selectedHRISWebhookExecution.dispatch_mode
                                ? formatLifecycleToken(selectedHRISWebhookExecution.dispatch_mode)
                                : t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                            <p className="break-all text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.dispatchMeta", {
                                requestedBy:
                                  selectedHRISWebhookExecution.requested_by ||
                                  t("enterpriseAlertsWorkspace.common.emptyDash"),
                                auditSource:
                                  selectedHRISWebhookExecution.audit_source ||
                                  t("enterpriseAlertsWorkspace.common.emptyDash"),
                              })}
                            </p>
                          </div>
                          {selectedHRISWebhookExecution.replay_source_execution_id ? (
                            <div className="space-y-1">
                              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.replay")}
                              </p>
                              <p className="break-all">
                                {selectedHRISWebhookExecution.replay_source_execution_id}
                              </p>
                              <p className="text-muted-foreground">
                                {t(
                                  "enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.replayWorkerRequirement",
                                  {
                                    value: formatExecutionReplayWorkerRequirement(
                                      selectedHRISWebhookExecution.replay_require_worker
                                    ),
                                  }
                                )}
                              </p>
                            </div>
                          ) : null}
                          {selectedHRISWebhookExecution.queue_state ? (
                            <div className="space-y-1">
                              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.runtime")}
                              </p>
                              <p>
                                {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.runtimeMeta", {
                                  queueState: formatLifecycleToken(selectedHRISWebhookExecution.queue_state),
                                  nextRetryAt: formatDateTime(selectedHRISWebhookExecution.next_retry_at),
                                  processingDeadlineAt: formatDateTime(
                                    selectedHRISWebhookExecution.processing_deadline_at
                                  ),
                                })}
                              </p>
                              <div className="flex flex-wrap items-center gap-2">
                                {typeof selectedHRISWebhookExecution.cooldown_remaining_seconds === "number" &&
                                selectedHRISWebhookExecution.cooldown_remaining_seconds > 0 ? (
                                  <Badge variant="outline">
                                    {t(
                                      "enterpriseAlertsWorkspace.syncAndWorker.executionHistory.runtimeBadges.cooldownRemaining",
                                      {
                                        seconds: selectedHRISWebhookExecution.cooldown_remaining_seconds,
                                      }
                                    )}
                                  </Badge>
                                ) : null}
                                {selectedHRISWebhookExecution.stale_in_flight ? (
                                  <Badge variant="destructive">
                                    {t(
                                      "enterpriseAlertsWorkspace.syncAndWorker.executionHistory.runtimeBadges.staleInFlight"
                                    )}
                                  </Badge>
                                ) : null}
                              </div>
                            </div>
                          ) : null}
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.attempts")}
                            </p>
                            <p>
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.attemptCount", {
                                count: selectedHRISWebhookExecution.attempt_count ?? 0,
                              })}
                            </p>
                            <p className="text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.requeueCount", {
                                count: selectedHRISWebhookExecution.requeue_count ?? 0,
                              })}
                            </p>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.failureStage")}
                            </p>
                            <p>
                              {selectedHRISWebhookExecution.failure_stage ||
                                t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                          </div>
                          <div className="space-y-1 md:col-span-2">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.lastError")}
                            </p>
                            <p className="break-all rounded-md border bg-background/70 px-3 py-2">
                              {selectedHRISWebhookExecution.last_error ||
                                t("enterpriseAlertsWorkspace.common.emptyDash")}
                            </p>
                          </div>
                        </div>
                      ) : !selectedHRISWebhookExecutionLoading && !selectedHRISWebhookExecutionError ? (
                        <p className="mt-3 text-sm text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.detail.empty")}
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                  {executionHistoryTotalCount > hrisWebhookExecutions.length ? (
                    <p className="mt-2 text-xs text-muted-foreground">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.paginationSummary", {
                        loaded: hrisWebhookExecutions.length,
                        total: executionHistoryTotalCount,
                      })}
                    </p>
                  ) : null}
                  <div className="mt-3 rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>
                            {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.columns.queuedAt")}
                          </TableHead>
                          <TableHead>
                            {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.columns.kind")}
                          </TableHead>
                          <TableHead>
                            {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.columns.dispatch")}
                          </TableHead>
                          <TableHead>
                            {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.columns.status")}
                          </TableHead>
                          <TableHead>
                            {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.columns.actions")}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {executionHistoryLoadingState ? (
                          <TableRow>
                            <TableCell colSpan={5} className="py-6 text-center text-sm text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.loading")}
                            </TableCell>
                          </TableRow>
                        ) : null}
                        {!executionHistoryLoadingState && hrisWebhookExecutions.length === 0 ? (
                          <TableRow>
                            <TableCell colSpan={5} className="py-6 text-center text-sm text-muted-foreground">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.empty")}
                            </TableCell>
                          </TableRow>
                        ) : null}
                        {!executionHistoryLoadingState
                          ? hrisWebhookExecutions.map((item) => (
                              <TableRow
                                key={item.id}
                                className={selectedHRISWebhookExecutionID === item.id ? "bg-muted/10" : undefined}
                                data-testid="enterprise-alerts-webhook-execution-item"
                              >
                                <TableCell className="align-top">
                                  <p className="text-sm">{formatDateTime(item.queued_at)}</p>
                                  <p className="mt-1 text-xs text-muted-foreground">
                                    {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.startedAt", {
                                      value: formatDateTime(item.started_at),
                                    })}
                                  </p>
                                  <p className="text-xs text-muted-foreground">
                                    {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.finishedAt", {
                                      value: formatDateTime(item.finished_at),
                                    })}
                                  </p>
                                </TableCell>
                                <TableCell className="align-top">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <Badge variant="outline">{formatExecutionKindLabel(item.kind)}</Badge>
                                    {item.vendor ? <Badge variant="secondary">{item.vendor}</Badge> : null}
                                  </div>
                                  <p className="mt-1 break-all text-xs text-muted-foreground">
                                    {item.connector_id || item.target_id || item.id}
                                  </p>
                                  <p className="mt-1 break-all text-xs text-muted-foreground">
                                    {item.event_type || item.request_id || item.receipt_id || item.id}
                                  </p>
                                  {item.failure_stage ? (
                                    <p className="mt-1 text-xs text-muted-foreground">{item.failure_stage}</p>
                                  ) : null}
                                </TableCell>
                                <TableCell className="align-top">
                                  <div className="flex flex-wrap items-center gap-2">
                                    {item.execution_mode ? (
                                      <Badge variant="outline">{formatLifecycleToken(item.execution_mode)}</Badge>
                                    ) : null}
                                    {item.dispatch_mode ? (
                                      <Badge variant="outline">{formatLifecycleToken(item.dispatch_mode)}</Badge>
                                    ) : null}
                                    {item.queue_state ? (
                                      <Badge variant={queueRuntimeBadgeVariant(item.queue_state)}>
                                        {formatLifecycleToken(item.queue_state)}
                                      </Badge>
                                    ) : null}
                                  </div>
                                  <p className="mt-1 text-xs text-muted-foreground">
                                    {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.dispatchMeta", {
                                      requestedBy: item.requested_by || t("enterpriseAlertsWorkspace.common.emptyDash"),
                                      auditSource: item.audit_source || t("enterpriseAlertsWorkspace.common.emptyDash"),
                                    })}
                                  </p>
                                  {item.replay_source_execution_id ? (
                                    <p className="mt-1 break-all text-xs text-muted-foreground">
                                      {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.replayMeta", {
                                        sourceExecutionID: item.replay_source_execution_id,
                                        workerRequirement: formatExecutionReplayWorkerRequirement(
                                          item.replay_require_worker
                                        ),
                                      })}
                                    </p>
                                  ) : null}
                                  {item.queue_state ? (
                                    <p className="mt-1 text-xs text-muted-foreground">
                                      {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.runtimeMeta", {
                                        queueState: formatLifecycleToken(item.queue_state),
                                        nextRetryAt: formatDateTime(item.next_retry_at),
                                        processingDeadlineAt: formatDateTime(item.processing_deadline_at),
                                      })}
                                    </p>
                                  ) : null}
                                  {item.queue_state &&
                                  ((item.cooldown_remaining_seconds || 0) > 0 || item.stale_in_flight) ? (
                                    <div className="mt-1 flex flex-wrap items-center gap-2">
                                      {(item.cooldown_remaining_seconds || 0) > 0 ? (
                                        <Badge variant="outline">
                                          {t(
                                            "enterpriseAlertsWorkspace.syncAndWorker.executionHistory.runtimeBadges.cooldownRemaining",
                                            {
                                              seconds: item.cooldown_remaining_seconds || 0,
                                            }
                                          )}
                                        </Badge>
                                      ) : null}
                                      {item.stale_in_flight ? (
                                        <Badge variant="destructive">
                                          {t(
                                            "enterpriseAlertsWorkspace.syncAndWorker.executionHistory.runtimeBadges.staleInFlight"
                                          )}
                                        </Badge>
                                      ) : null}
                                    </div>
                                  ) : null}
                                  <p className="mt-1 text-xs text-muted-foreground">
                                    {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.auditCounts", {
                                      attempts: item.attempt_count ?? 0,
                                      requeues: item.requeue_count ?? 0,
                                    })}
                                  </p>
                                </TableCell>
                                <TableCell className="align-top">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <Badge variant={queueRuntimeBadgeVariant(item.status)}>
                                      {formatLifecycleToken(item.status)}
                                    </Badge>
                                    {item.target_status ? (
                                      <Badge variant="outline">
                                        {t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.targetStatus", {
                                          status: formatLifecycleToken(item.target_status),
                                        })}
                                      </Badge>
                                    ) : null}
                                  </div>
                                  {item.last_error ? (
                                    <p className="mt-1 line-clamp-3 text-xs text-muted-foreground">{item.last_error}</p>
                                  ) : null}
                                </TableCell>
                                <TableCell className="align-top">
                                  <div className="flex flex-wrap items-center gap-2">
                                    {onSelectHRISWebhookExecution ? (
                                      <Button
                                        size="sm"
                                        type="button"
                                        variant={selectedHRISWebhookExecutionID === item.id ? "secondary" : "outline"}
                                        data-testid={`enterprise-alerts-webhook-execution-details-toggle-${item.id}`}
                                        onClick={() => {
                                          onSelectHRISWebhookExecution(
                                            selectedHRISWebhookExecutionID === item.id ? null : item.id
                                          )
                                        }}
                                      >
                                        {selectedHRISWebhookExecutionID === item.id
                                          ? t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.hideDetails")
                                          : t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.details")}
                                      </Button>
                                    ) : null}
                                    <Button asChild size="sm" variant="outline">
                                      <Link
                                        to={buildWebhookExecutionSyncLink(item)}
                                        data-testid="enterprise-alerts-webhook-execution-to-sync"
                                      >
                                        {t("enterprisePage.actions.goToSync")}
                                      </Link>
                                    </Button>
                                  </div>
                                </TableCell>
                              </TableRow>
                            ))
                          : null}
                      </TableBody>
                    </Table>
                  </div>
                  {hrisWebhookExecutionHasMore && onLoadMoreHRISWebhookExecutions ? (
                    <div className="mt-3 flex justify-center">
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={executionHistoryLoadingState || Boolean(hrisWebhookExecutionLoadingMore)}
                        data-testid="enterprise-alerts-webhook-execution-load-more"
                        onClick={() => {
                          void onLoadMoreHRISWebhookExecutions()
                        }}
                      >
                        {hrisWebhookExecutionLoadingMore
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.loadingMore")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.executionHistory.loadMore")}
                      </Button>
                    </div>
                  ) : null}
                </div>

                <EnterpriseHRISReceipts
                  title={t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.title")}
                  actions={
                    <div className="flex flex-wrap items-center gap-2">
                      {onBatchProcessHRISWebhookReceipts ? (
                        <Button
                          size="sm"
                          type="button"
                          variant="outline"
                          disabled={Boolean(receiptBatchDisabledReason)}
                          title={receiptBatchDisabledReason || undefined}
                          data-testid="enterprise-alerts-webhook-receipt-process-visible"
                          onClick={() => {
                            void onBatchProcessHRISWebhookReceipts(visibleProcessableWebhookReceiptIDs)
                          }}
                        >
                          {receiptActionBusy
                            ? t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.actions.processingVisible")
                            : t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.actions.processVisible", {
                                count: visibleProcessableWebhookReceiptIDs.length,
                              })}
                        </Button>
                      ) : null}
                      <Badge variant="secondary">
                        {filteredWebhookReceipts.length} / {hrisWebhookReceipts.length}
                      </Badge>
                      {receiptBatchDisabledReason ? (
                        <p className="w-full basis-full text-xs text-muted-foreground">{receiptBatchDisabledReason}</p>
                      ) : null}
                    </div>
                  }
                >
                  <div
                    className="mt-3 flex flex-wrap items-center gap-2"
                    data-testid="enterprise-alerts-webhook-receipt-filters"
                  >
                    {webhookReceiptRuntimeFilterOptions.map((item) => {
                      const active = item.queueState
                        ? workerQueueStateScope === item.queueState
                        : workerQueueStateScope.trim().length === 0
                      return (
                        <Button
                          key={item.value}
                          size="sm"
                          variant={active ? "default" : "outline"}
                          data-testid={`enterprise-alerts-webhook-receipt-filter-${item.value}`}
                          onClick={() => {
                            setWorkerQueueStateScope(item.queueState)
                            setWorkerReplayStateScope("")
                            setWorkerStatusScope("")
                          }}
                        >
                          {item.label} ({item.count})
                        </Button>
                      )
                    })}
                  </div>
                  {visibleProcessableWebhookReceiptIDs.length > 0 ? (
                    <p
                      className="mt-2 text-xs text-muted-foreground"
                      data-testid="enterprise-alerts-webhook-receipt-batch-hint"
                    >
                      {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchHint", {
                        count: visibleProcessableWebhookReceiptIDs.length,
                      })}
                    </p>
                  ) : null}
                  {webhookReceiptTotal > hrisWebhookReceipts.length ? (
                    <p
                      className="mt-2 text-xs text-muted-foreground"
                      data-testid="enterprise-alerts-webhook-receipt-pagination-summary"
                    >
                      {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.paginationSummary", {
                        loaded: hrisWebhookReceipts.length,
                        total: webhookReceiptTotal,
                      })}
                    </p>
                  ) : null}
                  {latestWebhookReceiptBatchProcessResult ? (
                    <div
                      className="mt-3 rounded-md border bg-muted/10 p-3"
                      data-testid="enterprise-alerts-webhook-receipt-batch-result"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="text-xs font-medium">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchResult.title")}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {formatDateTime(latestWebhookReceiptBatchProcessResult.updated_at)}
                        </p>
                      </div>
                      <p
                        className="mt-1 text-xs text-muted-foreground"
                        data-testid="enterprise-alerts-webhook-receipt-batch-result-summary"
                      >
                        {latestWebhookReceiptBatchProcessResult.execution_mode === "queued"
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchResult.summaryQueued", {
                              total: latestWebhookReceiptBatchProcessResult.total_receipts,
                              queued: latestWebhookReceiptBatchProcessResult.queued ?? 0,
                              skipped: latestWebhookReceiptBatchProcessResult.skipped,
                              failed: latestWebhookReceiptBatchProcessResult.failed,
                            })
                          : t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchResult.summary", {
                              total: latestWebhookReceiptBatchProcessResult.total_receipts,
                              processed: latestWebhookReceiptBatchProcessResult.processed,
                              skipped: latestWebhookReceiptBatchProcessResult.skipped,
                              failed: latestWebhookReceiptBatchProcessResult.failed,
                              dlq: latestWebhookReceiptBatchProcessResult.dlq,
                            })}
                      </p>
                      {latestWebhookReceiptBatchProcessResult.execution_mode === "queued" &&
                      latestWebhookReceiptBatchProcessResult.dispatch_mode ? (
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchResult.dispatch", {
                            dispatchMode: formatLifecycleToken(latestWebhookReceiptBatchProcessResult.dispatch_mode),
                          })}
                        </p>
                      ) : null}
                      {latestWebhookReceiptBatchProcessResult.items?.length ? (
                        <div className="mt-2 space-y-2">
                          {latestWebhookReceiptBatchProcessResult.items.slice(0, 4).map((item) => (
                            <div
                              key={item.receipt_id}
                              className="rounded-md border bg-background/70 px-3 py-2 text-xs"
                              data-testid="enterprise-alerts-webhook-receipt-batch-result-item"
                            >
                              <div className="flex flex-wrap items-center justify-between gap-2">
                                <p className="font-medium">
                                  {item.item?.event_type || item.item?.request_id || item.receipt_id}
                                </p>
                                <div className="flex flex-wrap items-center gap-2">
                                  <Badge variant={queueRuntimeBadgeVariant(item.status)}>
                                    {formatLifecycleToken(item.status)}
                                  </Badge>
                                  {item.reason ? (
                                    <Badge variant={queueRuntimeBadgeVariant(item.reason)}>
                                      {formatLifecycleToken(item.reason)}
                                    </Badge>
                                  ) : null}
                                </div>
                              </div>
                              <p className="mt-1 text-muted-foreground">
                                {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchResult.itemMeta", {
                                  receiptID: item.receipt_id,
                                  connector: item.item?.connector_id || "-",
                                  detail: item.error || item.item?.last_error || item.reason || "-",
                                })}
                              </p>
                              {item.execution_id && onSelectHRISWebhookExecution ? (
                                <div className="mt-2 flex flex-wrap items-center gap-2">
                                  <Button
                                    size="sm"
                                    type="button"
                                    variant={selectedHRISWebhookExecutionID === item.execution_id ? "secondary" : "outline"}
                                    data-testid={`enterprise-alerts-webhook-receipt-batch-result-execution-${item.execution_id}`}
                                    onClick={() => {
                                      onSelectHRISWebhookExecution(item.execution_id || null)
                                    }}
                                  >
                                    {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.batchResult.openExecution")}
                                  </Button>
                                  <Badge variant="outline">{item.execution_id}</Badge>
                                </div>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                  <div className="mt-3 space-y-2">
                    {filteredWebhookReceipts.map((item) => (
                      <div
                        key={item.id}
                        className="rounded-md border bg-muted/10 px-3 py-2 text-sm"
                        data-testid="enterprise-alerts-webhook-receipt-item"
                      >
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <p className="font-medium">{item.connector_id || item.id}</p>
                          <div className="flex flex-wrap items-center gap-2">
                            <Badge variant="secondary">{item.vendor}</Badge>
                            <Badge variant={queueRuntimeBadgeVariant(item.status)}>{item.status}</Badge>
                            <Badge variant={queueRuntimeBadgeVariant(item.queue_state)}>{item.queue_state}</Badge>
                          </div>
                        </div>
                        <p className="mt-1 break-all text-xs text-muted-foreground">
                          {item.event_type || item.request_id || item.id}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.meta", {
                            attempts: item.attempt_count || 0,
                            attemptAt: formatDateTime(item.last_attempt_at),
                            receivedAt: formatDateTime(item.received_at),
                          })}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.runtime", {
                            queueState: item.queue_state,
                            nextRetryAt: formatDateTime(item.next_retry_at),
                            processingDeadlineAt: formatDateTime(item.processing_deadline_at),
                          })}
                        </p>
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <Badge
                            variant={queueBudgetBadgeVariant(item.queue_state, item.remaining_attempts)}
                            data-testid="enterprise-alerts-webhook-receipt-runtime-budget"
                          >
                            {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.runtimeBadges.remainingAttempts", {
                              count: item.remaining_attempts || 0,
                            })}
                          </Badge>
                          {item.cooldown_remaining_seconds > 0 ? (
                            <Badge variant="outline" data-testid="enterprise-alerts-webhook-receipt-runtime-cooldown">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.runtimeBadges.cooldownRemaining", {
                                seconds: item.cooldown_remaining_seconds,
                              })}
                            </Badge>
                          ) : null}
                          {item.stale_in_flight ? (
                            <Badge variant="destructive" data-testid="enterprise-alerts-webhook-receipt-runtime-stale">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.runtimeBadges.staleInFlight")}
                            </Badge>
                          ) : null}
                        </div>
                        {item.last_error ? (
                          <p className="mt-1 line-clamp-3 text-xs text-muted-foreground">{item.last_error}</p>
                        ) : null}
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <Button asChild size="sm" variant="outline">
                            <Link
                              to={buildWebhookReceiptSyncLink(item)}
                              data-testid="enterprise-alerts-webhook-receipt-to-sync"
                            >
                              {t("enterprisePage.actions.goToSync")}
                            </Link>
                          </Button>
                          {item.queue_state === "ready" && onProcessHRISWebhookReceipt ? (
                            <Button
                              size="sm"
                              variant="outline"
                              data-testid="enterprise-alerts-webhook-receipt-process"
                              disabled={Boolean(receiptActionDisabledReason) || receiptActionID === item.id}
                              title={
                                receiptActionDisabledReason ||
                                (receiptActionID === item.id ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : undefined)
                              }
                              onClick={() => {
                                void onProcessHRISWebhookReceipt(item.id)
                              }}
                            >
                              {receiptActionID === item.id
                                ? t("enterpriseAlertsWorkspace.common.processing")
                                : t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.actions.process")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                    ))}
                    {!loading && filteredWebhookReceipts.length === 0 ? (
                      <p
                        className="text-sm text-muted-foreground"
                        data-testid="enterprise-alerts-webhook-receipt-empty"
                      >
                        {t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.empty")}
                      </p>
                    ) : null}
                  </div>
                  {hrisWebhookReceiptHasMore && onLoadMoreHRISWebhookReceipts ? (
                    <div className="mt-3 flex justify-center">
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={webhookReceiptLoadingState || Boolean(hrisWebhookReceiptLoadingMore)}
                        data-testid="enterprise-alerts-webhook-receipt-load-more"
                        onClick={() => {
                          void onLoadMoreHRISWebhookReceipts()
                        }}
                      >
                        {hrisWebhookReceiptLoadingMore
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.loadingMore")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.loadMore")}
                      </Button>
                    </div>
                  ) : null}
                </EnterpriseHRISReceipts>

                <div className="rounded-md border bg-background/70 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm font-medium">{t("enterpriseAlertsWorkspace.syncAndWorker.pullStates.title")}</p>
                    <Badge variant="secondary">
                      {filteredPullStates.length} / {hrisPullStates.length}
                    </Badge>
                  </div>
                  <div className="mt-3 space-y-2">
                    {filteredPullStates.slice(0, 4).map((item) => (
                      <div
                        key={`${item.connector_id}-${item.updated_at}`}
                        className="rounded-md border bg-muted/10 px-3 py-2 text-sm"
                        data-testid="enterprise-alerts-pull-state-item"
                      >
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <p className="font-medium">{item.connector_id}</p>
                          <div className="flex flex-wrap items-center gap-2">
                            <Badge variant="secondary">{item.vendor}</Badge>
                            <Badge variant={pullStateBadgeVariant(item.status)}>{item.status}</Badge>
                          </div>
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.pullStates.meta", {
                            failures: item.consecutive_failures || 0,
                            mode: item.last_mode || "-",
                            request: item.last_request_id || "-",
                          })}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.pullStates.timestamps", {
                            failureAt: formatDateTime(item.last_failure_at),
                            successAt: formatDateTime(item.last_success_at),
                          })}
                        </p>
                        {item.last_error ? (
                          <p className="mt-1 line-clamp-3 text-xs text-muted-foreground">{item.last_error}</p>
                        ) : null}
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <Button asChild size="sm" variant="outline">
                            <Link to={buildPullStateSyncLink(item)} data-testid="enterprise-alerts-pull-state-to-sync">
                              {t("enterprisePage.actions.goToSync")}
                            </Link>
                          </Button>
                        </div>
                      </div>
                    ))}
                    {!loading && filteredPullStates.length === 0 ? (
                      <p
                        className="text-sm text-muted-foreground"
                        data-testid="enterprise-alerts-pull-state-empty"
                      >
                        {t("enterpriseAlertsWorkspace.syncAndWorker.pullStates.empty")}
                      </p>
                    ) : null}
                  </div>
                </div>

                <EnterpriseHRISDLQ
                  title={t("enterpriseAlertsWorkspace.syncAndWorker.dlq.title")}
                  actions={
                    <div className="flex flex-wrap items-center gap-2">
                      {onBatchReplayHRISWebhookDLQ ? (
                        <Button
                          size="sm"
                          type="button"
                          variant="outline"
                          disabled={Boolean(dlqBatchDisabledReason)}
                          title={dlqBatchDisabledReason || undefined}
                          data-testid="enterprise-alerts-hris-dlq-replay-visible"
                          onClick={() => {
                            void onBatchReplayHRISWebhookDLQ(visibleReplayableDLQEntryIDs)
                          }}
                        >
                          {dlqActionBusy
                            ? t("enterpriseAlertsWorkspace.syncAndWorker.dlq.actions.replayingVisible")
                            : t("enterpriseAlertsWorkspace.syncAndWorker.dlq.actions.replayVisible", {
                                count: visibleReplayableDLQEntryIDs.length,
                              })}
                        </Button>
                      ) : null}
                      <Badge variant="secondary">
                        {filteredDLQEntries.length} / {hrisWebhookDLQEntries.length}
                      </Badge>
                      {dlqBatchDisabledReason ? (
                        <p className="w-full basis-full text-xs text-muted-foreground">{dlqBatchDisabledReason}</p>
                      ) : null}
                    </div>
                  }
                >
                  <div className="mt-3 flex flex-wrap items-center gap-2" data-testid="enterprise-alerts-hris-dlq-filters">
                    {dlqRuntimeFilterOptions.map((item) => {
                      const active = item.replayState
                        ? workerReplayStateScope === item.replayState
                        : workerReplayStateScope.trim().length === 0
                      return (
                        <Button
                          key={item.value}
                          size="sm"
                          variant={active ? "default" : "outline"}
                          data-testid={`enterprise-alerts-hris-dlq-filter-${item.value}`}
                          onClick={() => {
                            setWorkerReplayStateScope(item.replayState)
                            setWorkerQueueStateScope("")
                            setWorkerStatusScope("")
                          }}
                        >
                          {item.label} ({item.count})
                        </Button>
                      )
                    })}
                  </div>
                  {visibleReplayableDLQEntryIDs.length > 0 ? (
                    <p className="mt-2 text-xs text-muted-foreground" data-testid="enterprise-alerts-hris-dlq-batch-hint">
                      {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchHint", {
                        count: visibleReplayableDLQEntryIDs.length,
                      })}
                    </p>
                  ) : null}
                  {dlqTotal > hrisWebhookDLQEntries.length ? (
                    <p
                      className="mt-2 text-xs text-muted-foreground"
                      data-testid="enterprise-alerts-hris-dlq-pagination-summary"
                    >
                      {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.paginationSummary", {
                        loaded: hrisWebhookDLQEntries.length,
                        total: dlqTotal,
                      })}
                    </p>
                  ) : null}
                  {latestWebhookDLQBatchReplayResult ? (
                    <div
                      className="mt-3 rounded-md border bg-muted/10 p-3"
                      data-testid="enterprise-alerts-hris-dlq-batch-result"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="text-xs font-medium">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchResult.title")}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {formatDateTime(latestWebhookDLQBatchReplayResult.updated_at)}
                        </p>
                      </div>
                      <p
                        className="mt-1 text-xs text-muted-foreground"
                        data-testid="enterprise-alerts-hris-dlq-batch-result-summary"
                      >
                        {latestWebhookDLQBatchReplayResult.execution_mode === "queued"
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchResult.summaryQueued", {
                              total: latestWebhookDLQBatchReplayResult.total_entries,
                              queued: latestWebhookDLQBatchReplayResult.queued ?? 0,
                              skipped: latestWebhookDLQBatchReplayResult.skipped,
                              failed: latestWebhookDLQBatchReplayResult.failed,
                            })
                          : t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchResult.summary", {
                              total: latestWebhookDLQBatchReplayResult.total_entries,
                              replayed: latestWebhookDLQBatchReplayResult.replayed,
                              skipped: latestWebhookDLQBatchReplayResult.skipped,
                              failed: latestWebhookDLQBatchReplayResult.failed,
                            })}
                      </p>
                      {latestWebhookDLQBatchReplayResult.execution_mode === "queued" &&
                      latestWebhookDLQBatchReplayResult.dispatch_mode ? (
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchResult.dispatch", {
                            dispatchMode: formatLifecycleToken(latestWebhookDLQBatchReplayResult.dispatch_mode),
                          })}
                        </p>
                      ) : null}
                      {latestWebhookDLQBatchReplayResult.items?.length ? (
                        <div className="mt-2 space-y-2">
                          {latestWebhookDLQBatchReplayResult.items.slice(0, 4).map((item) => (
                            <div
                              key={item.entry_id}
                              className="rounded-md border bg-background/70 px-3 py-2 text-xs"
                              data-testid="enterprise-alerts-hris-dlq-batch-result-item"
                            >
                              <div className="flex flex-wrap items-center justify-between gap-2">
                                <p className="font-medium">
                                  {item.item?.event_type || item.item?.request_id || item.entry_id}
                                </p>
                                <div className="flex flex-wrap items-center gap-2">
                                  <Badge variant={queueRuntimeBadgeVariant(item.status)}>
                                    {formatLifecycleToken(item.status)}
                                  </Badge>
                                  {item.reason ? (
                                    <Badge variant={queueRuntimeBadgeVariant(item.reason)}>
                                      {formatLifecycleToken(item.reason)}
                                    </Badge>
                                  ) : null}
                                </div>
                              </div>
                              <p className="mt-1 text-muted-foreground">
                                {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchResult.itemMeta", {
                                  entryID: item.entry_id,
                                  stage: item.item?.failure_stage || "-",
                                  detail: item.error || item.item?.error || item.reason || "-",
                                })}
                              </p>
                              {item.execution_id && onSelectHRISWebhookExecution ? (
                                <div className="mt-2 flex flex-wrap items-center gap-2">
                                  <Button
                                    size="sm"
                                    type="button"
                                    variant={selectedHRISWebhookExecutionID === item.execution_id ? "secondary" : "outline"}
                                    data-testid={`enterprise-alerts-hris-dlq-batch-result-execution-${item.execution_id}`}
                                    onClick={() => {
                                      onSelectHRISWebhookExecution(item.execution_id || null)
                                    }}
                                  >
                                    {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.batchResult.openExecution")}
                                  </Button>
                                  <Badge variant="outline">{item.execution_id}</Badge>
                                </div>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                  <div className="mt-3 space-y-2">
                    {filteredDLQEntries.map((item) => (
                      <div
                        key={item.id}
                        className="rounded-md border bg-muted/10 px-3 py-2 text-sm"
                        data-testid="enterprise-alerts-hris-dlq-item"
                      >
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <p className="font-medium">{item.vendor || item.connector_id || item.id}</p>
                          <div className="flex flex-wrap items-center gap-2">
                            <Badge variant="outline">{item.failure_stage}</Badge>
                            <Badge variant={queueRuntimeBadgeVariant(item.status)}>{item.status}</Badge>
                            {item.replay_state ? (
                              <Badge variant={queueRuntimeBadgeVariant(item.replay_state)}>{item.replay_state}</Badge>
                            ) : null}
                          </div>
                        </div>
                        <p className="mt-1 break-all text-xs text-muted-foreground">
                          {item.event_type || item.request_id || item.receipt_id || item.id}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.meta", {
                            replayCount: item.replay_count || 0,
                            replayedAt: formatDateTime(item.last_replay_at),
                            updatedAt: formatDateTime(item.updated_at),
                          })}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.runtime", {
                            replayState: item.replay_state || "-",
                            nextRetryAt: formatDateTime(item.next_retry_at),
                            processingDeadlineAt: formatDateTime(item.processing_deadline_at),
                          })}
                        </p>
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <Badge
                            variant={queueBudgetBadgeVariant(item.replay_state, item.remaining_attempts)}
                            data-testid="enterprise-alerts-hris-dlq-runtime-budget"
                          >
                            {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.runtimeBadges.remainingAttempts", {
                              count: item.remaining_attempts || 0,
                            })}
                          </Badge>
                          {item.cooldown_remaining_seconds > 0 ? (
                            <Badge variant="outline" data-testid="enterprise-alerts-hris-dlq-runtime-cooldown">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.runtimeBadges.cooldownRemaining", {
                                seconds: item.cooldown_remaining_seconds,
                              })}
                            </Badge>
                          ) : null}
                          {item.stale_in_flight ? (
                            <Badge variant="destructive" data-testid="enterprise-alerts-hris-dlq-runtime-stale">
                              {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.runtimeBadges.staleInFlight")}
                            </Badge>
                          ) : null}
                        </div>
                        <p className="mt-1 line-clamp-3 text-xs text-muted-foreground">{item.error}</p>
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <Button asChild size="sm" variant="outline">
                            <Link to={buildDLQSyncLink(item)} data-testid="enterprise-alerts-hris-dlq-to-sync">
                              {t("enterprisePage.actions.goToSync")}
                            </Link>
                          </Button>
                          {item.replay_state === "ready" && onReplayHRISWebhookDLQ ? (
                            <Button
                              size="sm"
                              variant="outline"
                              data-testid="enterprise-alerts-hris-dlq-replay"
                              disabled={Boolean(dlqActionDisabledReason) || dlqActionID === item.id}
                              title={
                                dlqActionDisabledReason ||
                                (dlqActionID === item.id ? t("enterpriseAlertsWorkspace.disabledReasons.actionBusy") : undefined)
                              }
                              onClick={() => {
                                void onReplayHRISWebhookDLQ(item.id)
                              }}
                            >
                              {dlqActionID === item.id
                                ? t("enterpriseAlertsWorkspace.common.processing")
                                : t("enterpriseAlertsWorkspace.syncAndWorker.dlq.actions.replay")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                    ))}
                    {!loading && filteredDLQEntries.length === 0 ? (
                      <p
                        className="text-sm text-muted-foreground"
                        data-testid="enterprise-alerts-hris-dlq-empty"
                      >
                        {t("enterpriseAlertsWorkspace.syncAndWorker.dlq.empty")}
                      </p>
                    ) : null}
                  </div>
                  {hrisWebhookDLQHasMore && onLoadMoreHRISWebhookDLQ ? (
                    <div className="mt-3 flex justify-center">
                      <Button
                        size="sm"
                        type="button"
                        variant="outline"
                        disabled={dlqLoadingState || Boolean(hrisWebhookDLQLoadingMore)}
                        data-testid="enterprise-alerts-hris-dlq-load-more"
                        onClick={() => {
                          void onLoadMoreHRISWebhookDLQ()
                        }}
                      >
                        {hrisWebhookDLQLoadingMore
                          ? t("enterpriseAlertsWorkspace.syncAndWorker.dlq.loadingMore")
                          : t("enterpriseAlertsWorkspace.syncAndWorker.dlq.loadMore")}
                      </Button>
                    </div>
                  ) : null}
                </EnterpriseHRISDLQ>
              </div>
            </EnterpriseWorkerAlerts>
            </CardContent>
          </Card>
        ) : null}
      </div>
    </TabsContent>
  )
}
