import { type FormEvent } from "react"
import { Link } from "react-router-dom"

import { AccessDomainBanner } from "@/components/access/access-domain-banner"
import { AccessGrantDetailDialog } from "@/components/access/access-grant-detail-dialog"
import { AccessGrantFilterBar } from "@/components/access/access-grant-filter-bar"
import { AccessGrantForm } from "@/components/access/access-grant-form"
import { AccessGrantLedgerTable, type AccessGrantLedgerRow } from "@/components/access/access-grant-ledger-table"
import { AccessGrantOverviewCards } from "@/components/access/access-grant-overview-cards"
import { AccessGrantStarterCard } from "@/components/access/access-grant-starter-card"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { type Area, type Building, type Door, type TemporaryAccess } from "@/lib/api"

type DeliveryMethod = "email_qr" | "wallet"
type ScopeType = "all" | "building" | "area" | "door"

type AccessGrantsStarterItem = {
  id: string
  title: string
  deliveryLabel: string
  passType: string
  description: string
  reviewNote: string
  validUntilLabel: string
}

type AccessGrantsSectionProps = {
  activeCount: number
  activeGrant: TemporaryAccess | null
  areaID: string
  areaOptions: Area[]
  buildingID: string
  buildingOptions: Building[]
  dateFrom: string
  dateTo: string
  deliveryMethod: DeliveryMethod
  doorID: string
  doorOptions: Door[]
  emptyState: string
  expiredCount: number
  filtersActive: boolean
  filteredCount: number
  grantRows: AccessGrantLedgerRow[]
  granteeEmail: string
  granteeGender: string
  granteeName: string
  granteePhone: string
  methodFilter: "all" | DeliveryMethod
  mobileModel: string
  onActiveGrantChange: (grant: TemporaryAccess | null) => void
  onDateFromChange: (value: string) => void
  onDateToChange: (value: string) => void
  onBuildingChange: (value: string) => void
  onDeliveryMethodChange: (value: DeliveryMethod) => void
  onAreaChange: (value: string) => void
  onDoorChange: (value: string) => void
  onGranteeEmailChange: (value: string) => void
  onGranteeGenderChange: (value: string) => void
  onGranteeNameChange: (value: string) => void
  onGranteePhoneChange: (value: string) => void
  onMethodChange: (value: "all" | DeliveryMethod) => void
  onMobileModelChange: (value: string) => void
  onOpenGrant: (grant: TemporaryAccess) => void
  onPassTypeChange: (value: string) => void
  onPassTypeFilterChange: (value: string) => void
  onResetFilters: () => void
  onScopeTypeChange: (value: ScopeType) => void
  onStarterApply: (starterID: string) => void
  onStatusChange: (value: "all" | "active" | "expiring_soon" | "expired") => void
  onSubmitGrant: (event: FormEvent<HTMLFormElement>) => void
  onValidUntilChange: (value: string) => void
  passType: string
  passTypeFilter: string
  passTypeOptions: string[]
  platformViewer: boolean
  scopeSummaryLabel: string
  scopeType: ScopeType
  starters: AccessGrantsStarterItem[]
  statusFilter: "all" | "active" | "expiring_soon" | "expired"
  validUntil: string
  visitorCount: number
  expiringSoonCount: number
  walletLink: string
  walletFilteredLink: string
}

export function AccessGrantsSection({
  activeCount,
  activeGrant,
  areaID,
  areaOptions,
  buildingID,
  buildingOptions,
  dateFrom,
  dateTo,
  deliveryMethod,
  doorID,
  doorOptions,
  emptyState,
  expiredCount,
  expiringSoonCount,
  filtersActive,
  filteredCount,
  grantRows,
  granteeEmail,
  granteeGender,
  granteeName,
  granteePhone,
  methodFilter,
  mobileModel,
  onActiveGrantChange,
  onDateFromChange,
  onDateToChange,
  onBuildingChange,
  onDeliveryMethodChange,
  onAreaChange,
  onDoorChange,
  onGranteeEmailChange,
  onGranteeGenderChange,
  onGranteeNameChange,
  onGranteePhoneChange,
  onMethodChange,
  onMobileModelChange,
  onOpenGrant,
  onPassTypeChange,
  onPassTypeFilterChange,
  onResetFilters,
  onScopeTypeChange,
  onStarterApply,
  onStatusChange,
  onSubmitGrant,
  onValidUntilChange,
  passType,
  passTypeFilter,
  passTypeOptions,
  platformViewer,
  scopeSummaryLabel,
  scopeType,
  starters,
  statusFilter,
  validUntil,
  visitorCount,
  walletLink,
  walletFilteredLink,
}: AccessGrantsSectionProps) {
  const hasFilteredWalletLink = walletFilteredLink.trim().length > 0

  return (
    <div className="space-y-4">
      <AccessDomainBanner
        title="临时与访客授权"
        description="这里专门处理短期授权、访客通行和邮件二维码。长期员工、批量补发和凭证状态维护统一放在“凭证发放”页，不再重复建设两套入口。"
        actions={
          <>
            <Button asChild size="sm" variant="outline">
              <Link to={walletLink}>去访客/临时发放</Link>
            </Button>
            {hasFilteredWalletLink ? (
              <Button asChild size="sm" variant="outline">
                <Link to={walletFilteredLink}>按当前筛选继续发放</Link>
              </Button>
            ) : null}
          </>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">快速授权场景</CardTitle>
          <CardDescription>直接套用常见场景的范围、方式、对象类型和失效时间，减少短期授权逐项手填。</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 lg:grid-cols-2">
          {starters.map((starter) => (
            <AccessGrantStarterCard
              key={starter.id}
              title={starter.title}
              deliveryLabel={starter.deliveryLabel}
              passType={starter.passType}
              description={starter.description}
              reviewNote={starter.reviewNote}
              validUntilLabel={starter.validUntilLabel}
              onApply={() => onStarterApply(starter.id)}
              showTopologyAction={starter.reviewNote.includes("拓扑还不完整")}
            />
          ))}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">创建临时授权</CardTitle>
            <CardDescription>单独处理访客、临时证和短期访问，不再和长期员工权限混在一起。</CardDescription>
          </CardHeader>
          <CardContent>
            <AccessGrantForm
              onSubmit={onSubmitGrant}
              scopeType={scopeType}
              onScopeTypeChange={onScopeTypeChange}
              deliveryMethod={deliveryMethod}
              onDeliveryMethodChange={onDeliveryMethodChange}
              buildingID={buildingID}
              onBuildingChange={onBuildingChange}
              areaID={areaID}
              onAreaChange={onAreaChange}
              doorID={doorID}
              onDoorChange={onDoorChange}
              buildings={buildingOptions}
              areaOptions={areaOptions}
              doorOptions={doorOptions}
              scopeSummaryLabel={scopeSummaryLabel}
              granteeName={granteeName}
              onGranteeNameChange={onGranteeNameChange}
              granteeGender={granteeGender}
              onGranteeGenderChange={onGranteeGenderChange}
              granteePhone={granteePhone}
              onGranteePhoneChange={onGranteePhoneChange}
              granteeEmail={granteeEmail}
              onGranteeEmailChange={onGranteeEmailChange}
              mobileModel={mobileModel}
              onMobileModelChange={onMobileModelChange}
              passType={passType}
              onPassTypeChange={onPassTypeChange}
              validUntil={validUntil}
              onValidUntilChange={onValidUntilChange}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">授权台账</CardTitle>
            <CardDescription>展示授权时效、授权人信息与实名详情；支持按日期、方式、对象类型和状态筛选。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <AccessGrantOverviewCards
              activeCount={activeCount}
              expiringSoonCount={expiringSoonCount}
              visitorCount={visitorCount}
              expiredCount={expiredCount}
              onShowActive={() => onStatusChange("active")}
              onShowExpiringSoon={() => onStatusChange("expiring_soon")}
              onShowVisitors={() => onPassTypeFilterChange("visitor")}
              onShowExpired={() => onStatusChange("expired")}
            />

            <AccessGrantFilterBar
              dateFrom={dateFrom}
              dateTo={dateTo}
              methodFilter={methodFilter}
              passTypeFilter={passTypeFilter}
              statusFilter={statusFilter}
              passTypeOptions={passTypeOptions}
              onDateFromChange={onDateFromChange}
              onDateToChange={onDateToChange}
              onMethodChange={onMethodChange}
              onPassTypeChange={onPassTypeFilterChange}
              onStatusChange={onStatusChange}
              onReset={onResetFilters}
            />

            {filtersActive ? (
              <div className="rounded-lg border bg-muted/10 px-3 py-2 text-sm text-muted-foreground">
                当前筛选命中 {filteredCount} 条授权记录。若要回到完整台账，可清空筛选。
              </div>
            ) : null}
            {filtersActive && hasFilteredWalletLink ? (
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">
                  可基于当前筛选结果直接回流到凭证发放，并预填首个命中对象线索，减少跨页重复检索。
                </p>
                <div className="mt-2">
                  <Button asChild size="sm" variant="outline">
                    <Link to={walletFilteredLink}>按当前筛选继续发放</Link>
                  </Button>
                </div>
              </div>
            ) : null}

            <AccessGrantLedgerTable
              rows={grantRows}
              platformViewer={platformViewer}
              emptyState={emptyState}
              onOpenGrant={onOpenGrant}
            />

            <AccessGrantDetailDialog
              grant={activeGrant}
              open={Boolean(activeGrant)}
              onOpenChange={(open) => {
                if (!open) {
                  onActiveGrantChange(null)
                }
              }}
            />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
