import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  dispatchWalletJobAlerts,
  getWalletJobAlertSubscription,
  getWalletJobMetrics,
  getWalletJobMetricsTrend,
  listWalletDLQCleanupArchives,
  listWalletJobAlertNotifications,
  retryWalletJobAlertNotification,
  upsertWalletJobAlertSubscription,
  type Tenant,
  type WalletDLQCleanupArchive,
  type WalletJobAlertDispatchResult,
  type WalletJobAlertNotification,
  type WalletJobAlertSubscription,
  type WalletJobMetrics,
  type WalletJobMetricsTrend,
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

  const [savingSubscription, setSavingSubscription] = useState(false)
  const [dispatchingAlerts, setDispatchingAlerts] = useState(false)
  const [retryingAlertNotificationID, setRetryingAlertNotificationID] = useState("")
  const [dispatchSummary, setDispatchSummary] = useState("")
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

    const [metricsData, trendData, archiveItems, notificationItems, subscriptionData] =
      await Promise.all([
        getWalletJobMetrics(token, {
          tenant_id: nextTenantID,
          ...metricsQuery,
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
      ])

    setMetrics(metricsData)
    setMetricsTrend(trendData)
    setArchives(archiveItems)
    setAlertNotifications(notificationItems)
    setSubscription(subscriptionData)
    syncSubscriptionDraft(subscriptionData)
  }

  function resetMetricsAndAlerts() {
    setMetrics(null)
    setMetricsTrend(null)
    setArchives([])
    setAlertNotifications([])
    setSubscription(null)
    setTenantAggregates([])
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
    savingSubscription,
    dispatchingAlerts,
    retryingAlertNotificationID,
    dispatchSummary,
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
  }
}
