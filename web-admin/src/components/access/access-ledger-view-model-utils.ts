import { policyStatusLabel, scopeSummary } from "@/components/access/access-page-utils"
import i18n from "@/lib/i18n"
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

function t(key: string, defaultValue: string, options?: Record<string, unknown>) {
  return i18n.t(key, {
    defaultValue,
    ...options,
  })
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
    ? t(
        "accessPage.components.ledgerViewModel.groupEmpty.withoutEmployees",
        "No user groups yet. Connect employee directory first, then create baseline groups by role or department."
      )
    : t(
        "accessPage.components.ledgerViewModel.groupEmpty.withEmployees",
        "No user groups yet. Create the first group with the form on the left for policy and issuance targets."
      )
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
    return t(
      "accessPage.components.ledgerViewModel.policyEmpty.filtered",
      "No matching policy under current filters. Adjust keywords and try again."
    )
  }
  if (!directoryReady) {
    return t(
      "accessPage.components.ledgerViewModel.policyEmpty.directoryNotReady",
      "No policy yet. Prepare employees and groups first, then come back to define access rules."
    )
  }
  if (!topologyReady) {
    return t(
      "accessPage.components.ledgerViewModel.policyEmpty.topologyNotReady",
      "No policy yet. Complete building/area/door topology before creating precise door-level rules."
    )
  }
  return t(
    "accessPage.components.ledgerViewModel.policyEmpty.default",
    "No policy yet. Create the first policy with the form on the left."
  )
}
