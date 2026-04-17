import { type FormEvent } from "react"
import { Link } from "react-router-dom"

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
  onSubmitPolicy: (event: FormEvent<HTMLFormElement>) => void
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
  return (
    <div className="space-y-4">
      <AccessDomainBanner
        title="权限策略"
        description="先确认楼宇拓扑和用户组都已具备，再把规则落到楼宇、区域和门点。策略负责定义“谁能进哪里、在什么时间能进”。"
        actions={
          <>
            <Button asChild size="sm" variant="outline">
              <Link to={spacesLink}>去空间拓扑</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={grantsLink}>下一步去临时授权</Link>
            </Button>
          </>
        }
      />

      <Card>
        <CardHeader className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div>
            <CardTitle className="text-base">快速生成策略草稿</CardTitle>
            <CardDescription>直接从已有用户组套入建议范围和时间计划，把目录准备尽快推进到首批策略落地。</CardDescription>
          </div>
          {!topologyReady ? (
            <Button asChild size="sm" variant="outline">
              <Link to={spacesLink}>先补空间拓扑</Link>
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
            <CardTitle className="text-base">策略列表</CardTitle>
            <CardDescription>支持按同对象线索快速定位策略，再直接编辑。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-center">
              <Input
                value={ledgerQuery}
                onChange={(event) => onLedgerQueryChange(event.target.value)}
                placeholder="按策略名称、范围或成员数搜索"
              />
              {ledgerFilteredCount > 0 ? (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={onBatchSetDraft}
                  disabled={batchActionPending.length > 0}
                >
                  {batchActionPending === "draft" ? "批量更新中..." : `批量设为草稿（${ledgerFilteredCount}）`}
                </Button>
              ) : null}
              {ledgerFilteredCount > 0 ? (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={onBatchSetActive}
                  disabled={batchActionPending.length > 0}
                >
                  {batchActionPending === "active" ? "批量更新中..." : `批量设为启用（${ledgerFilteredCount}）`}
                </Button>
              ) : null}
              {hasLedgerQuery ? (
                <Button size="sm" variant="outline" onClick={onClearLedgerQuery}>
                  清空筛选
                </Button>
              ) : null}
            </div>
            <p className="mp-kpi-note">
              {hasLedgerQuery
                ? `当前筛选命中 ${ledgerFilteredCount} / ${ledgerTotalCount} 条策略。`
                : `当前共 ${ledgerTotalCount} 条策略。`}
            </p>
            {batchFlowHint.trim() ? (
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">{batchFlowHint}</p>
                <div className="mt-2">
                  <Button asChild size="sm" variant="outline">
                    <Link to={issuanceLink}>去凭证发放继续处理</Link>
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
