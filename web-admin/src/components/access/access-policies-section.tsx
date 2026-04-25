import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { AccessDomainBanner } from "@/components/access/access-domain-banner"
import { AccessPolicyForm } from "@/components/access/access-policy-form"
import { AccessPolicyLedgerTable } from "@/components/access/access-policy-ledger-table"
import { AccessPolicyStarterPanel } from "@/components/access/access-policy-starter-panel"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { type AccessPolicy } from "@/lib/api"

type AccessPoliciesSectionOption = {
  id: string
  name: string
}

type AccessPoliciesStarterItem = {
  id: string
  groupName: string
  memberCount: number
  name: string
  description: string
  reviewNote: string
  schedule: string
}

type AccessPoliciesLedgerRow = {
  policy: AccessPolicy
  scopeLabel: string
  scheduleLabel: string
  membersLabel: string
  statusLabel: string
  statusVariant: "outline" | "secondary"
}

type AccessPoliciesSectionProps = {
  areaID: string
  areaOptions: AccessPoliciesSectionOption[]
  batchActionPending: "" | "active" | "draft"
  batchFlowHint: string
  buildingID: string
  buildingOptions: AccessPoliciesSectionOption[]
  doorID: string
  doorOptions: AccessPoliciesSectionOption[]
  emptyState: string
  hasGroups: boolean
  hasLedgerQuery: boolean
  isEditing: boolean
  ledgerQuery: string
  ledgerRows: AccessPoliciesLedgerRow[]
  ledgerFilteredCount: number
  ledgerTotalCount: number
  members: string
  name: string
  onApplyStarter: (starterID: string) => void
  onAreaIDChange: (value: string) => void
  onBatchSetActive: () => void
  onBatchSetDraft: () => void
  onBuildingIDChange: (value: string) => void
  onClearLedgerQuery: () => void
  onDoorIDChange: (value: string) => void
  onEditPolicy: (policy: AccessPolicy) => void
  onLedgerQueryChange: (value: string) => void
  onMembersChange: (value: string) => void
  onNameChange: (value: string) => void
  onScheduleChange: (value: string) => void
  onScopeTypeChange: (value: "all" | "building" | "area" | "door") => void
  onStatusChange: (value: "active" | "inactive" | "draft") => void
  onSubmitPolicy: (payload: {
    name: string
    scopeType: "all" | "building" | "area" | "door"
    status: "active" | "inactive" | "draft"
    buildingID: string
    areaID: string
    doorID: string
    schedule: string
    members: string
  }) => void
  schedule: string
  spacesLink: string
  scopeSummaryLabel: string
  scopeType: "all" | "building" | "area" | "door"
  starterItems: AccessPoliciesStarterItem[]
  status: "active" | "inactive" | "draft"
  topologyReady: boolean
  grantsLink: string
  issuanceLink: string
}

export function AccessPoliciesSection({
  areaID,
  areaOptions,
  batchActionPending,
  batchFlowHint,
  buildingID,
  buildingOptions,
  doorID,
  doorOptions,
  emptyState,
  hasGroups,
  isEditing,
  ledgerRows,
  members,
  name,
  onApplyStarter,
  onAreaIDChange,
  onBatchSetActive,
  onBatchSetDraft,
  onBuildingIDChange,
  onDoorIDChange,
  onEditPolicy,
  onMembersChange,
  onNameChange,
  onScheduleChange,
  onScopeTypeChange,
  onStatusChange,
  onSubmitPolicy,
  schedule,
  spacesLink,
  scopeSummaryLabel,
  scopeType,
  starterItems,
  status,
  topologyReady,
  grantsLink,
  issuanceLink,
  hasLedgerQuery,
  ledgerQuery,
  ledgerFilteredCount,
  ledgerTotalCount,
  onClearLedgerQuery,
  onLedgerQueryChange,
}: AccessPoliciesSectionProps) {
  const { t } = useTranslation()

  return (
    <div className="space-y-4">
      <AccessDomainBanner
        title={t("accessPage.components.policiesSection.bannerTitle", { defaultValue: "Access policies" })}
        description={t("accessPage.components.policiesSection.bannerDescription", {
          defaultValue:
            "Ensure topology and groups are ready first, then apply rules to building/area/door scopes to define who can access what and when.",
        })}
        actions={
          <>
            <Button asChild size="sm" variant="outline">
              <Link to={spacesLink}>{t("accessPage.components.policiesSection.goTopology", { defaultValue: "Go to topology" })}</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={grantsLink}>
                {t("accessPage.components.policiesSection.goGrantsNext", { defaultValue: "Next: temporary grants" })}
              </Link>
            </Button>
          </>
        }
      />

      <Card>
        <CardHeader className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div>
            <CardTitle className="text-base">
              {t("accessPage.components.policiesSection.quickDraftTitle", { defaultValue: "Quick-generate policy drafts" })}
            </CardTitle>
            <CardDescription>
              {t("accessPage.components.policiesSection.quickDraftDescription", {
                defaultValue:
                  "Reuse existing user groups with suggested scope/schedule to move from directory readiness to first policy rollout quickly.",
              })}
            </CardDescription>
          </div>
          {!topologyReady ? (
            <Button asChild size="sm" variant="outline">
              <Link to={spacesLink}>{t("accessPage.components.policiesSection.completeTopologyFirst", { defaultValue: "Complete topology first" })}</Link>
            </Button>
          ) : null}
        </CardHeader>
        <CardContent className="space-y-3">
          <AccessPolicyStarterPanel
            hasGroups={hasGroups}
            items={starterItems}
            topologyReady={topologyReady}
            onApply={onApplyStarter}
          />
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <AccessPolicyForm
          areaID={areaID}
          areaOptions={areaOptions}
          buildingID={buildingID}
          buildingOptions={buildingOptions}
          doorID={doorID}
          doorOptions={doorOptions}
          isEditing={isEditing}
          members={members}
          name={name}
          onAreaIDChange={onAreaIDChange}
          onBuildingIDChange={onBuildingIDChange}
          onDoorIDChange={onDoorIDChange}
          onMembersChange={onMembersChange}
          onNameChange={onNameChange}
          onScheduleChange={onScheduleChange}
          onScopeTypeChange={onScopeTypeChange}
          onStatusChange={onStatusChange}
          onSubmit={onSubmitPolicy}
          schedule={schedule}
          scopeSummaryLabel={scopeSummaryLabel}
          scopeType={scopeType}
          status={status}
        />

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("accessPage.components.policiesSection.policyListTitle", { defaultValue: "Policy list" })}</CardTitle>
            <CardDescription>
              {t("accessPage.components.policiesSection.policyListDescription", {
                defaultValue: "Locate policies quickly by shared target hints, then edit directly.",
              })}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-center">
              <Input
                value={ledgerQuery}
                onChange={(event) => onLedgerQueryChange(event.target.value)}
                placeholder={t("accessPage.components.policiesSection.searchPlaceholder", {
                  defaultValue: "Search by policy name, scope, or member count",
                })}
              />
              {ledgerFilteredCount > 0 ? (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={onBatchSetDraft}
                  disabled={batchActionPending.length > 0}
                >
                  {batchActionPending === "draft"
                    ? t("accessPage.components.policiesSection.batchUpdating", { defaultValue: "Batch updating..." })
                    : t("accessPage.components.policiesSection.batchSetDraft", {
                        defaultValue: "Batch set to draft ({{count}})",
                        count: ledgerFilteredCount,
                      })}
                </Button>
              ) : null}
              {ledgerFilteredCount > 0 ? (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={onBatchSetActive}
                  disabled={batchActionPending.length > 0}
                >
                  {batchActionPending === "active"
                    ? t("accessPage.components.policiesSection.batchUpdating", { defaultValue: "Batch updating..." })
                    : t("accessPage.components.policiesSection.batchSetActive", {
                        defaultValue: "Batch set to active ({{count}})",
                        count: ledgerFilteredCount,
                      })}
                </Button>
              ) : null}
              {hasLedgerQuery ? (
                <Button size="sm" variant="outline" onClick={onClearLedgerQuery}>
                  {t("accessPage.components.policiesSection.clearFilters", { defaultValue: "Clear filters" })}
                </Button>
              ) : null}
            </div>
            <p className="mp-kpi-note">
              {hasLedgerQuery
                ? t("accessPage.components.policiesSection.filteredCount", {
                    defaultValue: "Filtered {{filtered}} / {{total}} policies.",
                    filtered: ledgerFilteredCount,
                    total: ledgerTotalCount,
                  })
                : t("accessPage.components.policiesSection.totalCount", {
                    defaultValue: "Total {{total}} policies.",
                    total: ledgerTotalCount,
                  })}
            </p>
            {batchFlowHint.trim() ? (
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">{batchFlowHint}</p>
                <div className="mt-2">
                  <Button asChild size="sm" variant="outline">
                    <Link to={issuanceLink}>
                      {t("accessPage.components.policiesSection.goIssuanceContinue", { defaultValue: "Continue in pass issuance" })}
                    </Link>
                  </Button>
                </div>
              </div>
            ) : null}
            <AccessPolicyLedgerTable rows={ledgerRows} emptyState={emptyState} onEdit={onEditPolicy} />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
