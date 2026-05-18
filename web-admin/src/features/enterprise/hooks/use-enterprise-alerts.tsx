import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  autoRetryEnterpriseSyncWorkerAlertNotifications,
  dispatchEnterpriseSyncWorkerAlerts,
  exportEnterpriseSyncWorkerAlertNotificationsCSV,
  listEnterpriseSyncWorkerAlertNotifications,
  retryEnterpriseSyncWorkerAlertNotification,
  retryEnterpriseSyncWorkerAlertNotificationsBatch,
  restoreEnterpriseSyncWorkerAlertNotificationsBatch,
  suppressEnterpriseSyncWorkerAlertNotificationsBatch,
  reviewEnterpriseJITProvisionApproval,
  updateEnterpriseJITProvisionApprovalExternalSync,
  upsertEnterpriseSyncWorkerAlertSubscription,
  type CurrentUser,
  type EnterpriseJITProvisionApproval,
  type EnterpriseSyncWorkerAlertNotification,
  type EnterpriseSyncWorkerAlertNotificationFilterCounts,
  type EnterpriseSyncWorkerAlertNotificationStatusCounts,
  type EnterpriseSyncWorkerAlertSubscription,
  type EnterpriseSyncWorkerAlertItem,
  type EnterpriseSyncWorkerAlertSummaryItem,
} from "@/lib/api"
import { type EnterpriseSyncWorkerAlertSubscriptionSaveInput } from "@/components/enterprise/enterprise-sync-worker-alert-subscription-card"
import { canManageEnterprise } from "@/lib/viewer"

// ── Filter type ─────────────────────────────────────────────────────────────

export type NotificationHistoryFilter = "all" | "failed" | "retryable" | "suppressed" | "due_now"

// ── Constants ───────────────────────────────────────────────────────────────

const workerAlertNotificationPageSize = 20

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

// ── Helpers ─────────────────────────────────────────────────────────────────

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

// ── Enterprise data slice type ──────────────────────────────────────────────

type EnterpriseAlertsDataSlice = {
  workerAlertSubscription: EnterpriseSyncWorkerAlertSubscription
  approvals: EnterpriseJITProvisionApproval[]
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
  workerAlertEvents: EnterpriseSyncWorkerAlertItem[]
}

// ── Hook params ─────────────────────────────────────────────────────────────

type UseEnterpriseAlertsParams = {
  token: string
  viewer: CurrentUser
  selectedTenantID: string
  selectedTenantName: string | undefined
  enterpriseData: EnterpriseAlertsDataSlice | undefined
  reloadEnterpriseData: (tenantID: string) => Promise<void>
  setSyncSummary: (summary: string) => void
  setError: (error: string) => void
}

export function useEnterpriseAlerts({
  token,
  viewer,
  selectedTenantID,
  selectedTenantName,
  enterpriseData,
  reloadEnterpriseData,
  setSyncSummary,
  setError,
}: UseEnterpriseAlertsParams) {
  const { t } = useTranslation()
  const writable = canManageEnterprise(viewer)

  // ── Core alert state ──────────────────────────────────────────────────

  const [workerAlertSubscription, setWorkerAlertSubscription] = useState<EnterpriseSyncWorkerAlertSubscription | null>(null)
  const [approvals, setApprovals] = useState<EnterpriseJITProvisionApproval[]>([])
  const [workerAlerts, setWorkerAlerts] = useState<EnterpriseSyncWorkerAlertSummaryItem[]>([])
  const [workerAlertEvents, setWorkerAlertEvents] = useState<EnterpriseSyncWorkerAlertItem[]>([])

  // ── Notification history state ────────────────────────────────────────

  const [workerAlertNotifications, setWorkerAlertNotifications] = useState<EnterpriseSyncWorkerAlertNotification[]>([])
  const [workerAlertNotificationFilterCounts, setWorkerAlertNotificationFilterCounts] =
    useState<EnterpriseSyncWorkerAlertNotificationFilterCounts>(emptyWorkerAlertNotificationFilterCounts)
  const [workerAlertNotificationStatusCounts, setWorkerAlertNotificationStatusCounts] =
    useState<EnterpriseSyncWorkerAlertNotificationStatusCounts>(emptyWorkerAlertNotificationStatusCounts)
  const [workerAlertNotificationTotal, setWorkerAlertNotificationTotal] = useState(0)
  const [workerAlertNotificationHasMore, setWorkerAlertNotificationHasMore] = useState(false)
  const [workerAlertNotificationFilter, setWorkerAlertNotificationFilter] = useState<NotificationHistoryFilter>("all")
  const [workerAlertNotificationQuery, setWorkerAlertNotificationQuery] = useState("")
  const [workerAlertNotificationListLoading, setWorkerAlertNotificationListLoading] = useState(false)
  const [workerAlertNotificationListLoadingMore, setWorkerAlertNotificationListLoadingMore] = useState(false)
  const [workerAlertNotificationExporting, setWorkerAlertNotificationExporting] = useState(false)

  // ── Action ID state ───────────────────────────────────────────────────

  const [approvalActionID, setApprovalActionID] = useState<string | null>(null)
  const [workerAlertSubscriptionActionID, setWorkerAlertSubscriptionActionID] = useState<string | null>(null)
  const [workerAlertDispatchActionID, setWorkerAlertDispatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationActionID, setWorkerAlertNotificationActionID] = useState<string | null>(null)
  const [workerAlertNotificationBatchActionID, setWorkerAlertNotificationBatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationSuppressBatchActionID, setWorkerAlertNotificationSuppressBatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationRestoreBatchActionID, setWorkerAlertNotificationRestoreBatchActionID] = useState<string | null>(null)
  const [workerAlertNotificationAutoRetryActionID, setWorkerAlertNotificationAutoRetryActionID] = useState<string | null>(null)

  // ── Derived data ──────────────────────────────────────────────────────

  const pendingApprovalCount = approvals.filter((item) => item.status === "pending").length
  const workerAlertCount = workerAlerts.reduce((sum, item) => sum + item.count, 0)

  // ── Effects: load enterprise data slice ───────────────────────────────

  useEffect(() => {
    const effectiveTenantID = selectedTenantID.trim()
    if (!effectiveTenantID) {
      setWorkerAlertSubscription(null)
      setApprovals([])
      setWorkerAlerts([])
      setWorkerAlertEvents([])
      setWorkerAlertNotifications([])
      setWorkerAlertNotificationFilterCounts(emptyWorkerAlertNotificationFilterCounts)
      setWorkerAlertNotificationStatusCounts(emptyWorkerAlertNotificationStatusCounts)
      setWorkerAlertNotificationTotal(0)
      setWorkerAlertNotificationHasMore(false)
      return
    }

    if (!enterpriseData) {
      return
    }

    setWorkerAlertSubscription(enterpriseData.workerAlertSubscription)
    setApprovals(enterpriseData.approvals)
    setWorkerAlerts(enterpriseData.workerAlerts)
    setWorkerAlertEvents(enterpriseData.workerAlertEvents)
  }, [enterpriseData, selectedTenantID])

  // ── Effect: load notification history ─────────────────────────────────

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

  // ── Reload helpers ────────────────────────────────────────────────────

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

  // ── Load more & view change ───────────────────────────────────────────

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

  function onWorkerAlertNotificationHistoryViewChange(input: {
    filter: NotificationHistoryFilter
    query: string
  }) {
    setWorkerAlertNotificationFilter((current) => (current === input.filter ? current : input.filter))
    setWorkerAlertNotificationQuery((current) => (current === input.query ? current : input.query))
  }

  // ── Export ────────────────────────────────────────────────────────────

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
      const scope = (selectedTenantName || tenantID).trim().replace(/\s+/g, "-").toLowerCase()
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

  // ── Approval mutation handlers ────────────────────────────────────────

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

  // ── Worker alert subscription handler ─────────────────────────────────

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

  // ── Worker alert dispatch handler ─────────────────────────────────────

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

  // ── Notification retry/suppress/restore handlers ──────────────────────

  const isNotificationActionBusy =
    workerAlertNotificationActionID !== null ||
    workerAlertNotificationBatchActionID !== null ||
    workerAlertNotificationRestoreBatchActionID !== null ||
    workerAlertNotificationSuppressBatchActionID !== null ||
    workerAlertNotificationAutoRetryActionID !== null

  async function onRetryWorkerAlertNotification(notificationID: string) {
    const tenantID = selectedTenantID.trim()
    const nextNotificationID = notificationID.trim()
    if (!writable || !tenantID || !nextNotificationID || isNotificationActionBusy) {
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
    if (!writable || !tenantID || isNotificationActionBusy) {
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
    if (!writable || !tenantID || nextNotificationIDs.length === 0 || isNotificationActionBusy) {
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
    if (!writable || !tenantID || nextNotificationIDs.length === 0 || isNotificationActionBusy) {
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
    if (!writable || !tenantID || nextNotificationIDs.length === 0 || isNotificationActionBusy) {
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

  return {
    // Alert data
    workerAlertSubscription,
    approvals,
    workerAlerts,
    workerAlertEvents,
    pendingApprovalCount,
    workerAlertCount,

    // Notification history
    workerAlertNotifications,
    workerAlertNotificationTotal,
    workerAlertNotificationFilterCounts,
    workerAlertNotificationStatusCounts,
    workerAlertNotificationHasMore,
    workerAlertNotificationListLoading,
    workerAlertNotificationListLoadingMore,
    workerAlertNotificationExporting,

    // Action IDs
    approvalActionID,
    workerAlertSubscriptionActionID,
    workerAlertDispatchActionID,
    workerAlertNotificationActionID,
    workerAlertNotificationBatchActionID,
    workerAlertNotificationSuppressBatchActionID,
    workerAlertNotificationRestoreBatchActionID,
    workerAlertNotificationAutoRetryActionID,

    // Handlers
    onReviewApproval,
    onUpdateApprovalExternalSync,
    onBatchReviewApprovals,
    onBatchUpdateApprovalExternalSync,
    onSaveWorkerAlertSubscription,
    onDispatchWorkerAlerts,
    onRetryWorkerAlertNotification,
    onAutoRetryWorkerAlertNotifications,
    onBatchRetryWorkerAlertNotifications,
    onBatchSuppressWorkerAlertNotifications,
    onBatchRestoreWorkerAlertNotifications,
    onWorkerAlertNotificationHistoryViewChange,
    onLoadMoreWorkerAlertNotifications,
    onExportWorkerAlertNotifications,
  }
}
