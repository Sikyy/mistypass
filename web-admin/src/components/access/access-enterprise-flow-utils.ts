import { enterpriseFlowSegmentLabel, enterpriseFlowSegmentStatusLabel } from "@/components/access/access-page-utils"
import type { EnterpriseEmployee } from "@/lib/api"

import type { AccessSection } from "./access-sections-tabs"

export type EnterpriseFlowContext = {
  flow: string
  grantPassType: string
  groupDesc: string
  groupMemberEmail: string
  groupMemberID: string
  groupMemberName: string
  groupMemberStatus: string
  groupName: string
  policyGroup: string
  policyName: string
  remediationHint: string
  segmentHint: string
  segmentStatusHint: string
  syncJobID: string
  syncSource: string
  syncStatus: string
  stage: string
  tenantID: string
  targetEmail: string
  targetID: string
  targetName: string
  workerAlertFailed: string
  workerAlertLastSeen: string
  workerAlertLevel: string
  workerAlertTenantID: string
  workerAlertThreshold: string
  workerFilterHint: string
  workerQueryHint: string
  workerReviewStageHint: string
  workerReviewStatusHint: string
}

type EnterpriseFlowContextLabelFields = Pick<
  EnterpriseFlowContext,
  | "groupMemberEmail"
  | "groupMemberName"
  | "groupMemberStatus"
  | "remediationHint"
  | "syncJobID"
  | "syncSource"
  | "workerAlertFailed"
  | "workerAlertLevel"
  | "workerAlertTenantID"
  | "workerAlertThreshold"
>

function queryValue(query: URLSearchParams, key: string) {
  return query.get(key)?.trim() || ""
}

function setOrDelete(query: URLSearchParams, key: string, value: string) {
  if (value.trim()) {
    query.set(key, value.trim())
    return
  }
  query.delete(key)
}

function normalizeAccessHint(value: string) {
  return value.trim().toLowerCase()
}

export function findByNameHint<T extends { name: string }>(items: T[], hint: string) {
  const normalizedHint = normalizeAccessHint(hint)
  if (!normalizedHint) {
    return null
  }
  return items.find((item) => normalizeAccessHint(item.name) === normalizedHint) ?? null
}

export function findByGroupNameHint<T extends { groupName: string }>(items: T[], hint: string) {
  const normalizedHint = normalizeAccessHint(hint)
  if (!normalizedHint) {
    return null
  }
  return items.find((item) => normalizeAccessHint(item.groupName) === normalizedHint) ?? null
}

function normalizeWorkerFilterHint(context: EnterpriseFlowContext) {
  const explicitWorkerFilter = context.workerFilterHint.trim()
  if (
    explicitWorkerFilter === "all" ||
    explicitWorkerFilter === "alerting" ||
    explicitWorkerFilter === "hot" ||
    explicitWorkerFilter === "stable"
  ) {
    return explicitWorkerFilter
  }
  if (
    context.workerAlertLevel === "hot" ||
    context.workerAlertLevel === "alerting" ||
    context.workerAlertLevel === "stable"
  ) {
    return context.workerAlertLevel
  }
  return ""
}

function isDeactivatedRemediation(context: EnterpriseFlowContextLabelFields) {
  const memberStatusHint = context.groupMemberStatus.trim().toLowerCase()
  return context.remediationHint === "deactivated_cleanup" || (memberStatusHint.length > 0 && memberStatusHint !== "active")
}

export function parseEnterpriseFlowContext(search: string): EnterpriseFlowContext | null {
  const query = new URLSearchParams(search)
  if (queryValue(query, "from") !== "enterprise") {
    return null
  }
  return {
    flow: queryValue(query, "flow"),
    grantPassType: queryValue(query, "grant_pass_type"),
    groupDesc: queryValue(query, "group_desc"),
    groupMemberEmail: queryValue(query, "group_member_email"),
    groupMemberID: queryValue(query, "group_member_id"),
    groupMemberName: queryValue(query, "group_member_name"),
    groupMemberStatus: queryValue(query, "group_member_status"),
    groupName: queryValue(query, "group_name"),
    policyGroup: queryValue(query, "policy_group"),
    policyName: queryValue(query, "policy_name"),
    remediationHint: queryValue(query, "remediation_hint"),
    segmentHint: queryValue(query, "segment_hint"),
    segmentStatusHint: queryValue(query, "segment_status_hint"),
    syncJobID: queryValue(query, "sync_job_id"),
    syncSource: queryValue(query, "sync_source"),
    syncStatus: queryValue(query, "sync_status"),
    stage: queryValue(query, "stage"),
    tenantID: queryValue(query, "tenant_id"),
    targetEmail: queryValue(query, "target_email"),
    targetID: queryValue(query, "target_id"),
    targetName: queryValue(query, "target_name"),
    workerAlertFailed: queryValue(query, "worker_alert_failed"),
    workerAlertLastSeen: queryValue(query, "worker_alert_last_seen"),
    workerAlertLevel: queryValue(query, "worker_alert_level"),
    workerAlertTenantID: queryValue(query, "worker_alert_tenant_id"),
    workerAlertThreshold: queryValue(query, "worker_alert_threshold"),
    workerFilterHint: queryValue(query, "worker_filter_hint"),
    workerQueryHint: queryValue(query, "worker_query_hint"),
    workerReviewStageHint: queryValue(query, "worker_review_stage_hint"),
    workerReviewStatusHint: queryValue(query, "worker_review_status_hint"),
  }
}

export function buildEnterpriseSyncRecordLabel(context: EnterpriseFlowContextLabelFields) {
  if (!context.syncJobID) {
    return ""
  }
  return `${context.syncSource || "同步"} 任务 ${context.syncJobID}`
}

export function buildEnterpriseStagePresetKey({
  search,
  stage,
}: {
  search: string
  stage: string
}) {
  return `${search}::${stage}`
}

export function buildEnterpriseFlowSummary(message: string) {
  return `来源：企业页。${message}`
}

export function resolveEnterpriseAccessStageRoute(stage: string): { section: AccessSection; pathname: string } | null {
  if (stage === "directory") {
    return {
      section: "directory",
      pathname: "/access/directory",
    }
  }
  if (stage === "policies") {
    return {
      section: "policies",
      pathname: "/access/policies",
    }
  }
  if (stage === "issuance") {
    return {
      section: "grants",
      pathname: "/access/grants",
    }
  }
  return null
}

export function buildEnterpriseSyncGroupDraft({
  syncJobID,
  syncSource,
  syncStatus,
}: {
  syncJobID: string
  syncSource: string
  syncStatus: string
}) {
  return {
    description: syncJobID
      ? `来源同步任务 ${syncJobID}${syncStatus ? `（状态：${syncStatus}）` : ""}`
      : "来源企业页同步异常复核",
    name: `${syncSource.toUpperCase()} 同步复核`,
  }
}

export function buildEnterpriseWorkerGroupDraft({
  selectedTenantID,
  workerAlertLabel,
  workerAlertLastSeen,
  workerAlertTenantID,
}: {
  selectedTenantID: string
  workerAlertLabel: string
  workerAlertLastSeen: string
  workerAlertTenantID: string
}) {
  const workerTenantLabel = workerAlertTenantID || selectedTenantID || "当前租户"
  return {
    description: `来源 worker 告警${workerAlertLabel ? `（${workerAlertLabel}）` : ""}${
      workerAlertLastSeen ? `，last_seen ${workerAlertLastSeen}` : ""
    }`,
    name: `${workerTenantLabel} Worker 告警复核`,
  }
}

export function buildEnterpriseWorkerPolicyDraftName({
  selectedTenantID,
  workerAlertTenantID,
}: {
  selectedTenantID: string
  workerAlertTenantID: string
}) {
  const workerTenantLabel = workerAlertTenantID || selectedTenantID || "当前租户"
  return `${workerTenantLabel} Worker 告警策略复核`
}

export function buildEnterpriseWorkerAlertLabel({
  context,
  selectedTenantID,
}: {
  context: EnterpriseFlowContextLabelFields
  selectedTenantID: string
}) {
  if (!context.workerAlertLevel) {
    return ""
  }
  const tenantLabel = context.workerAlertTenantID || selectedTenantID || "当前租户"
  const thresholdLabel =
    context.workerAlertFailed && context.workerAlertThreshold
      ? `（failed ${context.workerAlertFailed} / threshold ${context.workerAlertThreshold}）`
      : ""
  return `${tenantLabel} worker ${context.workerAlertLevel} 告警${thresholdLabel}`
}

export function deriveEnterpriseRemediationLabel({
  context,
  deactivatedLabel,
  normalLabel,
}: {
  context: EnterpriseFlowContextLabelFields
  deactivatedLabel: string
  normalLabel: string
}) {
  return isDeactivatedRemediation(context) ? deactivatedLabel : normalLabel
}

export function deriveEnterpriseHintedMemberLabel({
  context,
  hintedGroupMember,
}: {
  context: EnterpriseFlowContextLabelFields
  hintedGroupMember: EnterpriseEmployee | null
}) {
  return hintedGroupMember?.full_name.trim() || context.groupMemberName || context.groupMemberEmail
}

export function buildEnterpriseSummaryTail({
  syncLabelPrefix = "同步记录：",
  syncRecordLabel,
  workerLabelPrefix = "worker 告警：",
  workerAlertLabel,
}: {
  syncLabelPrefix?: string
  syncRecordLabel: string
  workerLabelPrefix?: string
  workerAlertLabel: string
}) {
  const syncTail = syncRecordLabel ? `（${syncLabelPrefix}${syncRecordLabel}）` : ""
  const workerTail = workerAlertLabel ? `（${workerLabelPrefix}${workerAlertLabel}）` : ""
  return `${syncTail}${workerTail}`
}

export function findHintedGroupMember(context: EnterpriseFlowContext | null, employees: EnterpriseEmployee[]) {
  if (!context) {
    return null
  }

  const memberIDHint = normalizeAccessHint(context.groupMemberID)
  if (memberIDHint) {
    const byID = employees.find((item) => normalizeAccessHint(item.id) === memberIDHint)
    if (byID) {
      return byID
    }
  }

  const memberEmailHint = normalizeAccessHint(context.groupMemberEmail)
  if (memberEmailHint) {
    const byEmail = employees.find((item) => normalizeAccessHint(item.email) === memberEmailHint)
    if (byEmail) {
      return byEmail
    }
  }

  const memberNameHint = normalizeAccessHint(context.groupMemberName)
  if (memberNameHint) {
    const byExactName = employees.find((item) => normalizeAccessHint(item.full_name) === memberNameHint)
    if (byExactName) {
      return byExactName
    }
    const byNameContains = employees.find((item) => normalizeAccessHint(item.full_name).includes(memberNameHint))
    if (byNameContains) {
      return byNameContains
    }
  }

  return null
}

export function buildFlowSegmentDescriptor(context: EnterpriseFlowContext | null) {
  if (!context) {
    return ""
  }
  const segmentLabel = enterpriseFlowSegmentLabel(context.segmentHint)
  if (!segmentLabel) {
    return ""
  }
  const statusLabel = enterpriseFlowSegmentStatusLabel(context.segmentStatusHint)
  return statusLabel ? `${segmentLabel} / ${statusLabel}` : segmentLabel
}

export function buildEnterpriseStageSearch({
  baseSearch,
  context,
  selectedTenantID,
  stage,
  hints,
}: {
  baseSearch: string
  context: EnterpriseFlowContext | null
  selectedTenantID: string
  stage: "directory" | "policies" | "issuance"
  hints?: Record<string, string>
}) {
  const query = new URLSearchParams(baseSearch)
  if (!context) {
    const existing = query.toString()
    return existing ? `?${existing}` : ""
  }

  query.set("from", "enterprise")
  query.set("flow", context.flow || "sync_to_access")
  query.set("stage", stage)

  const effectiveTenantID = selectedTenantID.trim() || context.tenantID.trim()
  if (effectiveTenantID) {
    query.set("tenant_id", effectiveTenantID)
  }

  if (hints) {
    Object.entries(hints).forEach(([key, value]) => {
      const hintKey = key.trim()
      if (!hintKey) {
        return
      }
      if (!value.trim()) {
        query.delete(hintKey)
        return
      }
      query.set(hintKey, value.trim())
    })
  }

  const existing = query.toString()
  return existing ? `?${existing}` : ""
}

export function hasWorkerAlertFlowHints(context: EnterpriseFlowContext | null) {
  if (!context) {
    return false
  }
  const hint = context.workerAlertLevel || context.workerAlertTenantID || context.workerFilterHint || context.workerQueryHint
  return hint.trim().length > 0
}

export function buildEnterpriseSyncWorkerReviewLink({
  activeSection,
  baseSearch,
  context,
  selectedTenantID,
}: {
  activeSection: AccessSection
  baseSearch: string
  context: EnterpriseFlowContext | null
  selectedTenantID: string
}) {
  const query = new URLSearchParams(baseSearch)
  if (!context) {
    const nextQuery = query.toString()
    return nextQuery ? `/enterprise?${nextQuery}#sync` : "/enterprise#sync"
  }

  query.set("from", "enterprise")
  query.set("flow", context.flow || "sync_to_access")
  query.set("sync_focus_hint", "worker_alert")
  query.set("worker_review_status_hint", "handled")
  const reviewStageHint = activeSection === "directory" ? "directory" : activeSection === "policies" ? "policies" : "issuance"
  query.set("worker_review_stage_hint", reviewStageHint)

  const effectiveTenantID = selectedTenantID.trim() || context.tenantID.trim()
  if (effectiveTenantID) {
    query.set("tenant_id", effectiveTenantID)
  }

  setOrDelete(query, "worker_filter_hint", normalizeWorkerFilterHint(context))
  setOrDelete(query, "worker_query_hint", context.workerQueryHint.trim() || context.workerAlertTenantID.trim())
  setOrDelete(query, "worker_alert_level", context.workerAlertLevel)
  setOrDelete(query, "worker_alert_tenant_id", context.workerAlertTenantID)
  setOrDelete(query, "worker_alert_last_seen", context.workerAlertLastSeen)
  setOrDelete(query, "worker_alert_failed", context.workerAlertFailed)
  setOrDelete(query, "worker_alert_threshold", context.workerAlertThreshold)

  const nextQuery = query.toString()
  return nextQuery ? `/enterprise?${nextQuery}#sync` : "/enterprise#sync"
}

export function applyEnterpriseWalletContext({
  context,
  flowMemberEmailHint,
  flowMemberIDHint,
  flowMemberNameHint,
  query,
  selectedTenantID,
  targetHint,
}: {
  context: EnterpriseFlowContext | null
  flowMemberEmailHint: string
  flowMemberIDHint: string
  flowMemberNameHint: string
  query: URLSearchParams
  selectedTenantID: string
  targetHint: "user" | "visitor"
}) {
  if (!context) {
    return
  }

  query.set("from", "enterprise")
  query.set("flow", context.flow || "sync_to_access")
  query.set("stage", "issuance")
  const effectiveTenantID = selectedTenantID.trim() || context.tenantID.trim()
  if (effectiveTenantID) {
    query.set("tenant_id", effectiveTenantID)
  }
  query.set("target_hint", targetHint)
  if (flowMemberEmailHint) {
    query.set("target_email", flowMemberEmailHint)
  }
  if (flowMemberIDHint) {
    query.set("target_id", flowMemberIDHint)
  }
  if (flowMemberNameHint) {
    query.set("target_name", flowMemberNameHint)
  }
  if (context.workerAlertLevel) {
    query.set("worker_alert_level", context.workerAlertLevel)
  }
  if (context.workerAlertTenantID) {
    query.set("worker_alert_tenant_id", context.workerAlertTenantID)
  }
  if (context.workerAlertLastSeen) {
    query.set("worker_alert_last_seen", context.workerAlertLastSeen)
  }
  if (context.workerAlertFailed) {
    query.set("worker_alert_failed", context.workerAlertFailed)
  }
  if (context.workerAlertThreshold) {
    query.set("worker_alert_threshold", context.workerAlertThreshold)
  }
  if (context.workerFilterHint) {
    query.set("worker_filter_hint", context.workerFilterHint)
  } else if (context.workerAlertLevel === "hot" || context.workerAlertLevel === "alerting") {
    query.set("worker_filter_hint", context.workerAlertLevel)
  }
  if (context.workerQueryHint) {
    query.set("worker_query_hint", context.workerQueryHint)
  } else if (context.workerAlertTenantID) {
    query.set("worker_query_hint", context.workerAlertTenantID)
  }
}
