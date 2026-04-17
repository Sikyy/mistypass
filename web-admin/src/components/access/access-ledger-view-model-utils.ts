import { policyStatusLabel, scopeSummary } from "@/components/access/access-page-utils"
import type { AccessPolicy, Area, Building, Door, EnterpriseEmployee, UserGroup } from "@/lib/api"

type GroupLedgerRow = {
  descriptionLabel: string
  group: UserGroup
  membersLabel: string
}

export type PolicyLedgerRow = {
  policy: AccessPolicy
  scopeLabel: string
  scheduleLabel: string
  membersLabel: string
  statusLabel: string
  statusVariant: "outline" | "secondary"
}

export function buildGroupLedgerRows({
  employeeByID,
  tenantGroups,
}: {
  employeeByID: Map<string, EnterpriseEmployee>
  tenantGroups: UserGroup[]
}): GroupLedgerRow[] {
  return tenantGroups.map((group) => ({
    descriptionLabel: group.description || "-",
    group,
    membersLabel:
      (group.members ?? [])
        .map((memberID) => employeeByID.get(memberID)?.full_name || memberID)
        .join(", ") || "-",
  }))
}

export function deriveGroupLedgerEmptyState(employeeCount: number) {
  return employeeCount === 0
    ? "还没有用户组。建议先接入员工目录，再建立面向岗位或部门的基础用户组。"
    : "还没有用户组。可直接用左侧表单创建首个用户组，作为策略与发放对象。"
}

export function buildPolicyLedgerRows({
  areaByID,
  buildingByID,
  doorByID,
  tenantPolicies,
}: {
  areaByID: Map<string, Area>
  buildingByID: Map<string, Building>
  doorByID: Map<string, Door>
  tenantPolicies: AccessPolicy[]
}): PolicyLedgerRow[] {
  return tenantPolicies.map((policy) => ({
    policy,
    scopeLabel: scopeSummary(
      policy.scope_type,
      buildingByID.get(policy.building_id || "")?.name ?? policy.building_id,
      areaByID.get(policy.area_id || "")?.name ?? policy.area_id,
      doorByID.get(policy.door_id || "")?.name ?? policy.door_id
    ),
    scheduleLabel: policy.schedule || "-",
    membersLabel: String(policy.members),
    statusLabel: policyStatusLabel(policy.status),
    statusVariant: policy.status === "active" ? "outline" : "secondary",
  }))
}

export function deriveSuggestedPolicyLedgerQuery({
  enterpriseFlowContext,
  hintedMemberGroupName,
}: {
  enterpriseFlowContext: {
    policyGroup: string
    workerQueryHint: string
    workerAlertTenantID: string
    syncSource: string
    syncJobID: string
    groupName: string
    groupMemberName: string
    groupMemberEmail: string
  } | null
  hintedMemberGroupName?: string
}) {
  if (!enterpriseFlowContext) {
    return ""
  }
  return (
    enterpriseFlowContext.policyGroup ||
    enterpriseFlowContext.workerQueryHint ||
    enterpriseFlowContext.workerAlertTenantID ||
    enterpriseFlowContext.syncSource ||
    enterpriseFlowContext.syncJobID ||
    hintedMemberGroupName ||
    enterpriseFlowContext.groupName ||
    enterpriseFlowContext.groupMemberName ||
    enterpriseFlowContext.groupMemberEmail
  )
}

export function filterPolicyLedgerRows({
  policyLedgerQuery,
  policyLedgerRows,
}: {
  policyLedgerQuery: string
  policyLedgerRows: PolicyLedgerRow[]
}): PolicyLedgerRow[] {
  const query = policyLedgerQuery.trim().toLowerCase()
  if (!query) {
    return policyLedgerRows
  }
  return policyLedgerRows.filter((row) => {
    return (
      row.policy.name.toLowerCase().includes(query) ||
      row.scopeLabel.toLowerCase().includes(query) ||
      row.scheduleLabel.toLowerCase().includes(query) ||
      row.membersLabel.toLowerCase().includes(query) ||
      row.statusLabel.toLowerCase().includes(query)
    )
  })
}

export function derivePolicyLedgerMatch({
  fallbackQuery,
  policyLedgerQuery,
  policyLedgerRows,
}: {
  fallbackQuery: string
  policyLedgerQuery: string
  policyLedgerRows: PolicyLedgerRow[]
}) {
  const effectiveQuery = policyLedgerQuery.trim() || fallbackQuery.trim()
  const matchedRows = filterPolicyLedgerRows({
    policyLedgerQuery: effectiveQuery,
    policyLedgerRows,
  })
  return {
    effectiveQuery,
    firstMatchedPolicy: matchedRows[0]?.policy ?? null,
    matchedRows,
  }
}

export function derivePolicyLedgerEmptyState({
  directoryReady,
  policyLedgerQueryActive,
  topologyReady,
}: {
  directoryReady: boolean
  policyLedgerQueryActive: boolean
  topologyReady: boolean
}) {
  if (policyLedgerQueryActive) {
    return "当前筛选条件下没有匹配策略，可调整关键词后重试。"
  }
  if (!directoryReady) {
    return "还没有策略。建议先整理员工与用户组，再回到这里建立访问规则。"
  }
  if (!topologyReady) {
    return "还没有策略。请先补齐楼宇、区域和门点，再创建精确到门点的访问规则。"
  }
  return "还没有策略。可直接用左侧表单创建首条策略。"
}
