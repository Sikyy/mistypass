import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  cleanupWalletDLQJobs,
  dispatchWalletJobAlerts,
  getWalletJobAlertSubscription,
  getWalletJobMetrics,
  getWalletJobMetricsTrend,
  getWalletJobSummary,
  listWalletDLQCleanupArchives,
  listWalletJobs,
  listWalletJobAlertNotifications,
  processWalletJobs,
  requeueWalletDLQJob,
  requeueWalletDLQJobs,
  retryWalletJobAlertNotification,
  upsertWalletJobAlertSubscription,
  type Tenant,
  type WalletDLQCleanupArchive,
  type WalletJobAlertDispatchResult,
  type WalletJobAlertNotification,
  type WalletJobAlertSubscription,
  type WalletIssueJob,
  type WalletJobMetrics,
  type WalletJobMetricsTrend,
  type WalletJobSummary,
} from "@/lib/api"
import {
  parseNonNegativeInt,
  parsePositiveInt,
  parseReceiverGroups,
} from "../pages/wallet-page-utils"

const defaultWindowSeconds = "900"
const defaultArchiveLimit = "20"
const defaultTrendBucketCount = "12"
const defaultSubscriptionCooldownSeconds = "900"
const defaultProcessLimit = "20"
const defaultProcessWorkerCount = "2"
const defaultDLQLimit = "20"
const defaultDLQOlderThanSeconds = "86400"

type WalletTenantAggregateRow = {
  tenantID: string
  tenantName: string
  total: number
  failed: number
  dlq: number
  retryableFailed: number
  alertCount: number
  updatedAt: string
}

type UseWalletAlertsParams = {
  token: string
  tenantID: string
}

export function useWalletAlerts({ token, tenantID }: UseWalletAlertsParams) {
  const { t } = useTranslation()

  const [windowSeconds, setWindowSeconds] = useState(defaultWindowSeconds)
  const [maxRetry, setMaxRetry] = useState("")
  const [alertThreshold, setAlertThreshold] = useState("")
  const [archiveLimit, setArchiveLimit] = useState(defaultArchiveLimit)
  const [trendBucketCount, setTrendBucketCount] = useState(defaultTrendBucketCount)

  const [metrics, setMetrics] = useState<WalletJobMetrics | null>(null)
  const [jobSummary, setJobSummary] = useState<WalletJobSummary | null>(null)
  const [jobs, setJobs] = useState<WalletIssueJob[]>([])
  const [metricsTrend, setMetricsTrend] = useState<WalletJobMetricsTrend | null>(null)
  const [archives, setArchives] = useState<WalletDLQCleanupArchive[]>([])
  const [alertNotifications, setAlertNotifications] = useState<WalletJobAlertNotification[]>([])
  const [subscription, setSubscription] = useState<WalletJobAlertSubscription | null>(null)
  const [tenantAggregates, setTenantAggregates] = useState<WalletTenantAggregateRow[]>([])

  const [subscriptionEnabled, setSubscriptionEnabled] = useState(true)
  const [subscriptionEmailEnabled, setSubscriptionEmailEnabled] = useState(true)
  const [subscriptionWhatsAppEnabled, setSubscriptionWhatsAppEnabled] = useState(false)
  const [subscriptionThreshold, setSubscriptionThreshold] = useState("")
  const [subscriptionWindowSeconds, setSubscriptionWindowSeconds] = useState(defaultWindowSeconds)
  const [subscriptionCooldownSeconds, setSubscriptionCooldownSeconds] = useState(defaultSubscriptionCooldownSeconds)
  const [subscriptionReceiverGroups, setSubscriptionReceiverGroups] = useState("security")

  const [processLimit, setProcessLimit] = useState(defaultProcessLimit)
  const [processWorkerCount, setProcessWorkerCount] = useState(defaultProcessWorkerCount)
  const [processMaxRetry, setProcessMaxRetry] = useState("")
  const [dlqLimit, setDLQLimit] = useState(defaultDLQLimit)
  const [dlqErrorCode, setDLQErrorCode] = useState("")
  const [dlqTargetIDOverride, setDLQTargetIDOverride] = useState("")
  const [dlqOlderThanSeconds, setDLQOlderThanSeconds] = useState(defaultDLQOlderThanSeconds)

  const [savingSubscription, setSavingSubscription] = useState(false)
  const [dispatchingAlerts, setDispatchingAlerts] = useState(false)
  const [processingJobs, setProcessingJobs] = useState(false)
  const [requeueingDLQ, setRequeueingDLQ] = useState(false)
  const [requeueingDLQJobID, setRequeueingDLQJobID] = useState("")
  const [cleaningDLQ, setCleaningDLQ] = useState(false)
  const [retryingAlertNotificationID, setRetryingAlertNotificationID] = useState("")
  const [dispatchSummary, setDispatchSummary] = useState("")
  const [governanceSummary, setGovernanceSummary] = useState("")
  const [aggregateWarning, setAggregateWarning] = useState("")
  const [error, setError] = useState("")

  function buildMetricsQueryOptions() {
    return {
      window_seconds: parsePositiveInt(windowSeconds),
      max_retry: parseNonNegativeInt(maxRetry),
      dlq_alert_threshold: parsePositiveInt(alertThreshold),
    }
  }

  function buildMetricsTrendQueryOptions() {
    return {
      window_seconds: parsePositiveInt(windowSeconds),
      bucket_count: parsePositiveInt(trendBucketCount),
      max_retry: parseNonNegativeInt(maxRetry),
      dlq_alert_threshold: parsePositiveInt(alertThreshold),
    }
  }

  function syncSubscriptionDraft(next: WalletJobAlertSubscription) {
    setSubscriptionEnabled(next.enabled)
    setSubscriptionEmailEnabled(next.channels.email)
    setSubscriptionWhatsAppEnabled(next.channels.whatsapp)
    setSubscriptionThreshold(String(next.dlq_alert_threshold))
    setSubscriptionWindowSeconds(String(next.window_seconds))
    setSubscriptionCooldownSeconds(String(next.cooldown_seconds))
    setSubscriptionReceiverGroups((next.receiver_groups ?? ["security"]).join(", "))
  }

  const windowErrorCodeRows = useMemo(() => {
    const items = Object.entries(metrics?.window.error_code_breakdown ?? {})
    return items.sort((a, b) => b[1] - a[1]).slice(0, 5)
  }, [metrics?.window.error_code_breakdown])

  const dlqErrorCodeRows = useMemo(() => {
    const items = Object.entries(jobSummary?.error_code_breakdown ?? metrics?.summary.error_code_breakdown ?? {})
    return items
      .sort((a, b) => b[1] - a[1])
      .slice(0, 8)
      .map(([errorCode, count]) => ({ errorCode, count }))
  }, [jobSummary?.error_code_breakdown, metrics?.summary.error_code_breakdown])

  const dlqJobs = useMemo(() => {
    return jobs
      .filter((job) => job.status === "dlq")
      .sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at))
  }, [jobs])

  const filteredDLQJobs = useMemo(() => {
    const selectedErrorCode = dlqErrorCode.trim()
    const items = selectedErrorCode
      ? dlqJobs.filter((job) => (job.error_code ?? "") === selectedErrorCode)
      : dlqJobs
    return items.slice(0, 10)
  }, [dlqErrorCode, dlqJobs])

  const trendPeakUpdated = useMemo(() => {
    const peak = Math.max(...(metricsTrend?.buckets.map((item) => item.updated) ?? [0]))
    return peak > 0 ? peak : 1
  }, [metricsTrend?.buckets])

  const aggregateStats = useMemo(() => {
    const totals = tenantAggregates.reduce(
      (acc, row) => {
        acc.total += row.total
        acc.failed += row.failed
        acc.dlq += row.dlq
        acc.alertTenants += row.alertCount > 0 ? 1 : 0
        return acc
      },
      { total: 0, failed: 0, dlq: 0, alertTenants: 0 }
    )
    return totals
  }, [tenantAggregates])

  const alertItems = metrics?.alerts ?? []

  async function loadMetricsAndAlerts(nextTenantID: string) {
    const metricsQuery = buildMetricsQueryOptions()
    const trendQuery = buildMetricsTrendQueryOptions()
    const nextArchiveLimit = parsePositiveInt(archiveLimit) ?? 20

    const [metricsData, jobSummaryData, trendData, archiveItems, notificationItems, subscriptionData, jobItems] =
      await Promise.all([
        getWalletJobMetrics(token, {
          tenant_id: nextTenantID,
          ...metricsQuery,
        }),
        getWalletJobSummary(token, {
          tenant_id: nextTenantID,
          max_retry: metricsQuery.max_retry,
        }),
        getWalletJobMetricsTrend(token, {
          tenant_id: nextTenantID,
          ...trendQuery,
        }),
        listWalletDLQCleanupArchives(token, {
          tenant_id: nextTenantID,
          limit: nextArchiveLimit,
        }),
        listWalletJobAlertNotifications(token, {
          tenant_id: nextTenantID,
          limit: 30,
        }),
        getWalletJobAlertSubscription(token, {
          tenant_id: nextTenantID,
        }),
        listWalletJobs(token, nextTenantID),
      ])

    setMetrics(metricsData)
    setJobSummary(jobSummaryData)
    setMetricsTrend(trendData)
    setArchives(archiveItems)
    setAlertNotifications(notificationItems)
    setSubscription(subscriptionData)
    setJobs(jobItems)
    syncSubscriptionDraft(subscriptionData)
  }

  function resetMetricsAndAlerts() {
    setMetrics(null)
    setJobSummary(null)
    setJobs([])
    setMetricsTrend(null)
    setArchives([])
    setAlertNotifications([])
    setSubscription(null)
    setTenantAggregates([])
    setGovernanceSummary("")
  }

  function focusDLQErrorCode(errorCode: string) {
    setDLQErrorCode(errorCode)
  }

  function clearDLQErrorCode() {
    setDLQErrorCode("")
  }

  async function loadWalletTenantAggregates(tenantItems: Tenant[]) {
    if (tenantItems.length === 0) {
      setTenantAggregates([])
      setAggregateWarning("")
      return
    }

    const metricsQuery = buildMetricsQueryOptions()
    const settled = await Promise.allSettled(
      tenantItems.map(async (tenant) => {
        const data = await getWalletJobMetrics(token, {
          tenant_id: tenant.id,
          ...metricsQuery,
        })
        return {
          tenantID: tenant.id,
          tenantName: tenant.name,
          total: data.summary.total,
          failed: data.summary.failed,
          dlq: data.summary.dlq,
          retryableFailed: data.summary.retryable_failed,
          alertCount: data.alerts?.length ?? 0,
          updatedAt: data.updated_at,
        } satisfies WalletTenantAggregateRow
      })
    )

    const rows: WalletTenantAggregateRow[] = []
    let failedCount = 0
    for (const item of settled) {
      if (item.status === "fulfilled") {
        rows.push(item.value)
        continue
      }
      failedCount++
    }
    rows.sort((a, b) => {
      if (a.dlq !== b.dlq) {
        return b.dlq - a.dlq
      }
      if (a.failed !== b.failed) {
        return b.failed - a.failed
      }
      return a.tenantName.localeCompare(b.tenantName)
    })
    setTenantAggregates(rows)
    if (failedCount > 0) {
      setAggregateWarning(t("walletPage.warnings.aggregateTenantLoadFailed", { failedCount }))
    } else {
      setAggregateWarning("")
    }
  }

  async function saveAlertSubscription() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setSavingSubscription(true)
    setError("")
    try {
      const updated = await upsertWalletJobAlertSubscription(token, {
        tenant_id: nextTenantID,
        enabled: subscriptionEnabled,
        dlq_alert_threshold: parsePositiveInt(subscriptionThreshold) ?? subscription?.dlq_alert_threshold ?? 20,
        window_seconds: parsePositiveInt(subscriptionWindowSeconds) ?? subscription?.window_seconds ?? 900,
        cooldown_seconds: parseNonNegativeInt(subscriptionCooldownSeconds) ?? subscription?.cooldown_seconds ?? 900,
        channels: {
          email: subscriptionEmailEnabled,
          whatsapp: subscriptionWhatsAppEnabled,
        },
        receiver_groups: parseReceiverGroups(subscriptionReceiverGroups),
        actor: "web_admin.wallet",
      })
      setSubscription(updated)
      syncSubscriptionDraft(updated)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.saveAlertSubscriptionFailed")
      setError(message)
    } finally {
      setSavingSubscription(false)
    }
  }

  async function dispatchAlertsNow() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setDispatchingAlerts(true)
    setDispatchSummary("")
    setError("")
    try {
      const result: WalletJobAlertDispatchResult = await dispatchWalletJobAlerts(token, {
        tenant_id: nextTenantID,
        window_seconds: parsePositiveInt(windowSeconds),
        max_retry: parseNonNegativeInt(maxRetry),
        dlq_alert_threshold: parsePositiveInt(alertThreshold),
        actor: "web_admin.wallet.dispatch",
      })
      setDispatchSummary(
        t("walletPage.summaries.alertDispatchEvaluated", {
          totalAlerts: result.total_alerts,
          dispatched: result.dispatched,
          skipped: result.skipped,
          failed: result.failed,
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.dispatchAlertsFailed")
      setError(message)
    } finally {
      setDispatchingAlerts(false)
    }
  }

  async function retryAlertNotification(notificationID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setRetryingAlertNotificationID(notificationID)
    setError("")
    try {
      const result = await retryWalletJobAlertNotification(token, {
        tenant_id: nextTenantID,
        notification_id: notificationID,
        actor: "web_admin.wallet.dispatch.retry",
      })
      setDispatchSummary(
        t("walletPage.summaries.alertRetryResult", {
          status: result.status,
          reasonSuffix: result.reason ? ` (${result.reason})` : "",
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.retryAlertNotificationFailed")
      setError(message)
    } finally {
      setRetryingAlertNotificationID("")
    }
  }

  async function processPendingJobs() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setProcessingJobs(true)
    setGovernanceSummary("")
    setError("")
    try {
      const result = await processWalletJobs(token, {
        tenant_id: nextTenantID,
        limit: parsePositiveInt(processLimit),
        worker_count: parsePositiveInt(processWorkerCount),
        max_retry: parseNonNegativeInt(processMaxRetry),
        actor: "web_admin.wallet.queue.process",
      })
      setGovernanceSummary(
        t("walletPage.summaries.walletJobsProcessed", {
          claimed: result.claimed,
          succeeded: result.succeeded,
          failed: result.failed,
          dlq: result.dlq,
          pendingAfter: result.pending_after,
        })
      )
      await loadMetricsAndAlerts(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.processWalletJobsFailed")
      setError(message)
    } finally {
      setProcessingJobs(false)
    }
  }

  async function requeueDLQBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setRequeueingDLQ(true)
    setGovernanceSummary("")
    setError("")
    try {
      const result = await requeueWalletDLQJobs(token, {
        tenant_id: nextTenantID,
        limit: parsePositiveInt(dlqLimit),
        error_code: dlqErrorCode.trim() || undefined,
        target_id_override: dlqTargetIDOverride.trim() || undefined,
        actor: "web_admin.wallet.dlq.requeue",
      })
      setGovernanceSummary(
        t("walletPage.summaries.walletDLQRequeued", {
          requeued: result.requeued,
          skipped: result.skipped,
          remaining: result.remaining_dlq,
        })
      )
      await loadMetricsAndAlerts(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.requeueWalletDLQFailed")
      setError(message)
    } finally {
      setRequeueingDLQ(false)
    }
  }

  async function requeueDLQJob(jobID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setRequeueingDLQJobID(jobID)
    setGovernanceSummary("")
    setError("")
    try {
      const result = await requeueWalletDLQJob(token, jobID, {
        tenant_id: nextTenantID,
        target_id: dlqTargetIDOverride.trim() || undefined,
        actor: "web_admin.wallet.dlq.requeue_single",
      })
      setGovernanceSummary(
        t("walletPage.summaries.walletDLQJobRequeued", {
          jobID: result.id,
          status: result.status,
        })
      )
      await loadMetricsAndAlerts(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.requeueWalletDLQJobFailed")
      setError(message)
    } finally {
      setRequeueingDLQJobID("")
    }
  }

  async function cleanupDLQBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setCleaningDLQ(true)
    setGovernanceSummary("")
    setError("")
    try {
      const result = await cleanupWalletDLQJobs(token, {
        tenant_id: nextTenantID,
        limit: parsePositiveInt(dlqLimit),
        error_code: dlqErrorCode.trim() || undefined,
        older_than_seconds: parsePositiveInt(dlqOlderThanSeconds),
        actor: "web_admin.wallet.dlq.cleanup",
      })
      setGovernanceSummary(
        t("walletPage.summaries.walletDLQCleaned", {
          removed: result.removed,
          remaining: result.remaining_dlq,
        })
      )
      await loadMetricsAndAlerts(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.cleanupWalletDLQFailed")
      setError(message)
    } finally {
      setCleaningDLQ(false)
    }
  }

  return {
    windowSeconds,
    setWindowSeconds,
    maxRetry,
    setMaxRetry,
    alertThreshold,
    setAlertThreshold,
    archiveLimit,
    setArchiveLimit,
    trendBucketCount,
    setTrendBucketCount,
    metrics,
    jobSummary,
    dlqErrorCodeRows,
    filteredDLQJobs,
    metricsTrend,
    archives,
    alertNotifications,
    subscription,
    tenantAggregates,
    subscriptionEnabled,
    setSubscriptionEnabled,
    subscriptionEmailEnabled,
    setSubscriptionEmailEnabled,
    subscriptionWhatsAppEnabled,
    setSubscriptionWhatsAppEnabled,
    subscriptionThreshold,
    setSubscriptionThreshold,
    subscriptionWindowSeconds,
    setSubscriptionWindowSeconds,
    subscriptionCooldownSeconds,
    setSubscriptionCooldownSeconds,
    subscriptionReceiverGroups,
    setSubscriptionReceiverGroups,
    processLimit,
    setProcessLimit,
    processWorkerCount,
    setProcessWorkerCount,
    processMaxRetry,
    setProcessMaxRetry,
    dlqLimit,
    setDLQLimit,
    dlqErrorCode,
    setDLQErrorCode,
    dlqTargetIDOverride,
    setDLQTargetIDOverride,
    dlqOlderThanSeconds,
    setDLQOlderThanSeconds,
    savingSubscription,
    dispatchingAlerts,
    processingJobs,
    requeueingDLQ,
    requeueingDLQJobID,
    cleaningDLQ,
    retryingAlertNotificationID,
    dispatchSummary,
    governanceSummary,
    aggregateWarning,
    error,
    setError,
    alertItems,
    windowErrorCodeRows,
    trendPeakUpdated,
    aggregateStats,
    loadMetricsAndAlerts,
    resetMetricsAndAlerts,
    loadWalletTenantAggregates,
    saveAlertSubscription,
    dispatchAlertsNow,
    retryAlertNotification,
    processPendingJobs,
    requeueDLQBatch,
    requeueDLQJob,
    focusDLQErrorCode,
    clearDLQErrorCode,
    cleanupDLQBatch,
  }
}
