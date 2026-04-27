import type { TFunction } from "i18next"

import type { EnterpriseSyncWorkerAlertSummaryItem } from "@/lib/api"

export type EnterpriseSyncWorkerAlertLevel = "alerting" | "hot" | "stable"

type EnterpriseSyncWorkerAlertStage = "dlq" | "processing" | "pull" | "reconcile"
type EnterpriseSyncWorkerAlertSignal =
  | "attemptLimit"
  | "cooldown"
  | "dlqBacklog"
  | "mapping"
  | "pullFailure"
  | "reconcileFailure"

export type EnterpriseSyncWorkerAlertGuidance = {
  badgeLabel: string
  badgeVariant: "outline" | "secondary" | "destructive"
  suggestions: string[]
  summary: string
  title: string
}

function includesAny(value: string, fragments: string[]) {
  return fragments.some((fragment) => value.includes(fragment))
}

function normalizeWorkerAlertSignature(item: EnterpriseSyncWorkerAlertSummaryItem) {
  return [item.worker_action || "", item.worker_kind || "", item.worker_label || ""].join(" ").trim().toLowerCase()
}

function resolveWorkerAlertVendor(item: EnterpriseSyncWorkerAlertSummaryItem, t: TFunction) {
  const signature = normalizeWorkerAlertSignature(item)
  if (includesAny(signature, ["talenta", "mekari"])) {
    return "Talenta"
  }
  if (signature.includes("gadjian")) {
    return "Gadjian"
  }
  if (includesAny(signature, ["greatday", "great day"])) {
    return "GreatDay HR"
  }
  if (includesAny(signature, ["linovhr", "linov hr", "linov"])) {
    return "LinovHR"
  }
  if (signature.includes("sunfish")) {
    return "SunFish"
  }
  if (signature.includes("sync")) {
    return t("enterpriseSyncWorkspace.workerAlerts.guidance.vendor.genericSync")
  }
  return t("enterpriseSyncWorkspace.workerAlerts.guidance.vendor.genericHris")
}

function resolveWorkerAlertStage(item: EnterpriseSyncWorkerAlertSummaryItem): EnterpriseSyncWorkerAlertStage {
  const signature = normalizeWorkerAlertSignature(item)
  if (includesAny(signature, ["dlq", "replay"])) {
    return "dlq"
  }
  if (includesAny(signature, ["processing", "normalize", "merge"])) {
    return "processing"
  }
  if (signature.includes("pull")) {
    return "pull"
  }
  return "reconcile"
}

function buildWorkerAlertSuggestions(
  stage: EnterpriseSyncWorkerAlertStage,
  signal: EnterpriseSyncWorkerAlertSignal,
  t: TFunction
) {
  if (signal === "cooldown") {
    return [
      t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.waitCooldown"),
      t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.avoidManualRetryFlood"),
      t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkUpstreamAvailability"),
    ]
  }

  if (signal === "attemptLimit") {
    switch (stage) {
      case "processing":
        return [
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.reviewAttemptLimit"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkFieldMapping"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkMergeSemantics"),
        ]
      case "dlq":
        return [
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.reviewAttemptLimit"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.reviewDlqPayload"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.rerunPullReconcile"),
        ]
      case "pull":
        return [
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.reviewAttemptLimit"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkConnectorConfig"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.rerunPullReconcile"),
        ]
      case "reconcile":
      default:
        return [
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.reviewAttemptLimit"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkConnectorConfig"),
          t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkFieldMapping"),
        ]
    }
  }

  switch (stage) {
    case "processing":
      return [
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkFieldMapping"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkMergeSemantics"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.rerunPullReconcile"),
      ]
    case "dlq":
      return [
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.reviewDlqPayload"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkFieldMapping"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.rerunPullReconcile"),
      ]
    case "pull":
      return [
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkConnectorConfig"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkUpstreamAvailability"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.rerunPullReconcile"),
      ]
    case "reconcile":
    default:
      return [
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkConnectorConfig"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.checkFieldMapping"),
        t("enterpriseSyncWorkspace.workerAlerts.guidance.actions.rerunPullReconcile"),
      ]
  }
}

function formatWorkerAlertFailureAge(seconds: number, t: TFunction) {
  const normalized = Math.max(0, Math.floor(seconds))
  if (normalized < 60) {
    return t("enterpriseSyncWorkspace.workerAlerts.guidance.duration.seconds", {
      count: normalized,
    })
  }
  const minutes = Math.floor(normalized / 60)
  if (minutes < 60) {
    return t("enterpriseSyncWorkspace.workerAlerts.guidance.duration.minutes", {
      count: minutes,
    })
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 48) {
    return t("enterpriseSyncWorkspace.workerAlerts.guidance.duration.hours", {
      count: hours,
    })
  }
  const days = Math.floor(hours / 24)
  return t("enterpriseSyncWorkspace.workerAlerts.guidance.duration.days", {
    count: days,
  })
}

export function classifyEnterpriseSyncWorkerAlertLevel(
  item: EnterpriseSyncWorkerAlertSummaryItem
): EnterpriseSyncWorkerAlertLevel {
  if (item.count === 0) {
    return "stable"
  }
  if (item.worker_kind === "hris_pull" && (item.last_consecutive_failures || 0) >= Math.max(1, item.last_threshold)) {
    return "hot"
  }
  if (item.last_threshold > 0 && item.last_failed >= item.last_threshold) {
    return "hot"
  }
  return "alerting"
}

export function describeEnterpriseSyncWorkerAlertGuidance(
  item: EnterpriseSyncWorkerAlertSummaryItem,
  t: TFunction
): EnterpriseSyncWorkerAlertGuidance | null {
  const level = classifyEnterpriseSyncWorkerAlertLevel(item)
  if (level === "stable") {
    return null
  }

  const stage = resolveWorkerAlertStage(item)
  const vendor = resolveWorkerAlertVendor(item, t)
  let signal: EnterpriseSyncWorkerAlertSignal
  let badgeLabel: string
  let badgeVariant: "outline" | "secondary" | "destructive"

  if (item.last_skipped_by_attempt_limit > 0) {
    signal = "attemptLimit"
    badgeLabel = t("enterpriseSyncWorkspace.workerAlerts.guidance.badges.review")
    badgeVariant = "destructive"
  } else if (item.last_skipped_by_cooldown > 0) {
    signal = "cooldown"
    badgeLabel = t("enterpriseSyncWorkspace.workerAlerts.guidance.badges.retryLater")
    badgeVariant = "secondary"
  } else if (stage === "processing") {
    signal = "mapping"
    badgeLabel = t("enterpriseSyncWorkspace.workerAlerts.guidance.badges.fixNow")
    badgeVariant = level === "hot" ? "destructive" : "secondary"
  } else if (stage === "dlq") {
    signal = "dlqBacklog"
    badgeLabel = t("enterpriseSyncWorkspace.workerAlerts.guidance.badges.fixNow")
    badgeVariant = level === "hot" ? "destructive" : "secondary"
  } else if (stage === "pull") {
    signal = "pullFailure"
    badgeLabel = t("enterpriseSyncWorkspace.workerAlerts.guidance.badges.fixNow")
    badgeVariant = level === "hot" ? "destructive" : "secondary"
  } else {
    signal = "reconcileFailure"
    badgeLabel = t("enterpriseSyncWorkspace.workerAlerts.guidance.badges.fixNow")
    badgeVariant = level === "hot" ? "destructive" : "secondary"
  }

  return {
    badgeLabel,
    badgeVariant,
    title: t("enterpriseSyncWorkspace.workerAlerts.guidance.title", {
      signal: t(`enterpriseSyncWorkspace.workerAlerts.guidance.signals.${signal}`),
      stage: t(`enterpriseSyncWorkspace.workerAlerts.guidance.stages.${stage}`),
      vendor,
    }),
    summary:
      stage === "pull" && (item.last_consecutive_failures || 0) > 0
        ? t("enterpriseSyncWorkspace.workerAlerts.guidance.summaryPullStateful", {
            applied: item.last_applied,
            attemptLimit: item.last_skipped_by_attempt_limit,
            consecutiveFailures: item.last_consecutive_failures,
            cooldown: item.last_skipped_by_cooldown,
            failed: item.last_failed,
            failureAge: formatWorkerAlertFailureAge(item.last_failure_age_seconds || 0, t),
            processed: item.last_processed,
            threshold: item.last_threshold,
            vendor,
          })
        : t("enterpriseSyncWorkspace.workerAlerts.guidance.summary", {
            applied: item.last_applied,
            attemptLimit: item.last_skipped_by_attempt_limit,
            cooldown: item.last_skipped_by_cooldown,
            failed: item.last_failed,
            processed: item.last_processed,
            threshold: item.last_threshold,
            vendor,
          }),
    suggestions: buildWorkerAlertSuggestions(stage, signal, t),
  }
}
