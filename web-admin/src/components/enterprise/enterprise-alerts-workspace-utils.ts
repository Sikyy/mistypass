import type {
  EnterpriseSyncJob,
  EnterpriseSyncWorkerAlertNotification,
} from "@/lib/api"

export function normalizeStatus(value?: string) {
  return (value || "").trim().toLowerCase()
}

export function formatLifecycleToken(value?: string, emptyDash = "\u2014") {
  const normalized = normalizeStatus(value)
  if (!normalized) {
    return emptyDash
  }
  return normalized.replace(/_/g, " ")
}

export function classifyExternalSyncStatus(value?: string): "none" | "failed" | "pending" | "success" | "other" {
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

export function classifySyncJob(item: EnterpriseSyncJob): "attention" | "rejected" | "deactivated" | "healthy" {
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

export function isWorkerAlertNotificationDueNow(item: EnterpriseSyncWorkerAlertNotification) {
  if (item.status !== "failed" || !item.retryable || !item.next_retry_at) {
    return false
  }
  const retryAt = new Date(item.next_retry_at).getTime()
  if (Number.isNaN(retryAt)) {
    return false
  }
  return retryAt <= Date.now()
}

export function matchesWorkerAlertNotificationQuery(
  item: EnterpriseSyncWorkerAlertNotification,
  query: string
) {
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

export function isWorkerAlertNotificationConfirmationPending(item: EnterpriseSyncWorkerAlertNotification) {
  return normalizeStatus(item.status).replace(/[\s_-]+/g, "") === "confirmationpending"
}

export function hasWorkerAlertNotificationPendingAge(value?: number): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
}

export function buildNotificationHistoryCSVValue(value: string | number | boolean | undefined | null) {
  return `"${String(value ?? "").replace(/"/g, '""')}"`
}

export function withRouteHints(baseLink: string, hints: Record<string, string>) {
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

export function pullStateBadgeVariant(status?: string): "outline" | "secondary" | "destructive" {
  const normalized = normalizeStatus(status)
  if (normalized === "succeeded" || normalized === "success" || normalized === "completed") {
    return "outline"
  }
  if (normalized === "failed" || normalized === "error") {
    return "destructive"
  }
  return "secondary"
}

export function queueRuntimeBadgeVariant(status?: string): "outline" | "secondary" | "destructive" {
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

export function queueBudgetBadgeVariant(
  state?: string,
  remainingAttempts?: number
): "outline" | "secondary" | "destructive" {
  if ((remainingAttempts || 0) > 0) {
    return "secondary"
  }
  if (normalizeStatus(state) === "attempt_limit") {
    return "destructive"
  }
  return "outline"
}
