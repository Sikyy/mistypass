import type { TFunction } from "i18next"

import type {
  EnterpriseHRISPullState,
  EnterpriseHRISWebhookDLQEntry,
  EnterpriseHRISWebhookExecution,
  EnterpriseHRISWebhookReceipt,
  EnterpriseSyncJob,
  EnterpriseSyncWorkerAlertItem,
  EnterpriseSyncWorkerAlertSummaryItem,
} from "@/lib/api"
import { classifyEnterpriseSyncWorkerAlertLevel } from "@/components/enterprise/enterprise-sync-worker-alert-guidance"
import { classifySyncJob, withRouteHints } from "@/components/enterprise/enterprise-alerts-workspace-utils"

export function buildSyncJobScopedLinks(
  item: EnterpriseSyncJob,
  opts: {
    directoryLink: string
    policiesLink: string
    walletLink: string
    t: TFunction
  }
) {
  const { directoryLink, policiesLink, walletLink, t } = opts
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

export function workerFilterHint(item?: EnterpriseSyncWorkerAlertSummaryItem | null) {
  if (!item) {
    return ""
  }
  const category = classifyEnterpriseSyncWorkerAlertLevel(item)
  return category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"
}

export function matchWorkerSummary(
  primaryKeyword: string,
  extraKeywords: string[],
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
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

export function buildWorkerAlertScopedLinks(
  item: EnterpriseSyncWorkerAlertSummaryItem,
  opts: {
    directoryLink: string
    formatDateTime: (value?: string) => string
    policiesLink: string
    selectedTenantName?: string
    syncLink: string
    t: TFunction
    walletLink: string
  }
) {
  const { directoryLink, formatDateTime, policiesLink, selectedTenantName, syncLink, t, walletLink } = opts
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
  const workerFilterHintValue = category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"

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
      worker_filter_hint: workerFilterHintValue,
      worker_query_hint: item.tenant_id,
    }),
    sync: withRouteHints(syncLink, {
      sync_focus_hint: "worker_alert",
      worker_filter_hint: workerFilterHintValue,
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

export function buildWorkerEventSyncLink(
  item: EnterpriseSyncWorkerAlertItem,
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[],
  syncLink: string
) {
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

export function buildWebhookReceiptSyncLink(
  item: EnterpriseHRISWebhookReceipt,
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[],
  syncLink: string
) {
  const summaryItem =
    matchWorkerSummary("receipt", [item.vendor || ""], workerAlerts) ||
    matchWorkerSummary("processing", [item.vendor || ""], workerAlerts) ||
    matchWorkerSummary("webhook", [item.vendor || ""], workerAlerts)
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

export function buildPullStateSyncLink(
  item: EnterpriseHRISPullState,
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[],
  syncLink: string
) {
  const summaryItem = matchWorkerSummary("pull", [item.vendor || ""], workerAlerts)
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

export function buildDLQSyncLink(
  item: EnterpriseHRISWebhookDLQEntry,
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[],
  syncLink: string
) {
  const summaryItem =
    matchWorkerSummary("dlq", [item.vendor || ""], workerAlerts) ||
    matchWorkerSummary("replay", [item.vendor || ""], workerAlerts) ||
    matchWorkerSummary("processing", [item.vendor || ""], workerAlerts)
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

export function buildWebhookExecutionSyncLink(
  item: EnterpriseHRISWebhookExecution,
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[],
  syncLink: string
) {
  const summaryItem =
    item.kind === "dlq_replay"
      ? matchWorkerSummary("dlq", [item.vendor || ""], workerAlerts) ||
        matchWorkerSummary("replay", [item.vendor || ""], workerAlerts) ||
        matchWorkerSummary("processing", [item.vendor || ""], workerAlerts)
      : matchWorkerSummary("receipt", [item.vendor || ""], workerAlerts) ||
        matchWorkerSummary("processing", [item.vendor || ""], workerAlerts) ||
        matchWorkerSummary("webhook", [item.vendor || ""], workerAlerts)
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
