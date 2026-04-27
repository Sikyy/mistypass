import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

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
  onSubmitGrant: (payload: {
    scopeType: ScopeType
    deliveryMethod: DeliveryMethod
    buildingID: string
    areaID: string
    doorID: string
    granteeName: string
    granteeGender: string
    granteePhone: string
    granteeEmail: string
    mobileModel: string
    passType: string
    validUntil: string
  }) => void
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
  const { t } = useTranslation()
  const hasFilteredWalletLink = walletFilteredLink.trim().length > 0

  return (
    <div className="space-y-4">
      <AccessDomainBanner
        title={t("accessPage.components.grantsSection.bannerTitle", { defaultValue: "Temporary & visitor grants" })}
        description={t("accessPage.components.grantsSection.bannerDescription", {
          defaultValue:
            "This section handles short-term grants, visitor access, and email QR only. Long-term employee issuance and status operations stay in pass issuance.",
        })}
        actions={
          <>
            <Button asChild size="sm" variant="outline">
              <Link to={walletLink}>
                {t("accessPage.components.grantsSection.goVisitorTemporaryIssuance", { defaultValue: "Go to visitor/temporary issuance" })}
              </Link>
            </Button>
            {hasFilteredWalletLink ? (
              <Button asChild size="sm" variant="outline">
                <Link to={walletFilteredLink}>
                  {t("accessPage.components.grantsSection.continueIssuanceByFilter", { defaultValue: "Continue issuance by current filters" })}
                </Link>
              </Button>
            ) : null}
          </>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("accessPage.components.grantsSection.quickScenariosTitle", { defaultValue: "Quick grant scenarios" })}</CardTitle>
          <CardDescription>
            {t("accessPage.components.grantsSection.quickScenariosDescription", {
              defaultValue: "Apply common scope/method/subject/expiry presets to reduce repetitive manual input.",
            })}
          </CardDescription>
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
              showTopologyAction={starter.reviewNote.toLowerCase().includes("topology") && starter.reviewNote.toLowerCase().includes("incomplete")}
            />
          ))}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              {t("accessPage.components.grantsSection.createGrantTitle", { defaultValue: "Create temporary grant" })}
            </CardTitle>
            <CardDescription>
              {t("accessPage.components.grantsSection.createGrantDescription", {
                defaultValue: "Handle visitor/temporary/short-term access separately from long-term employee permissions.",
              })}
            </CardDescription>
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
            <CardTitle className="text-base">{t("accessPage.components.grantsSection.ledgerTitle", { defaultValue: "Grant ledger" })}</CardTitle>
            <CardDescription>
              {t("accessPage.components.grantsSection.ledgerDescription", {
                defaultValue:
                  "Show validity, authorizer info, and identity details; supports filtering by date, method, subject type, and status.",
              })}
            </CardDescription>
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
                {t("accessPage.components.grantsSection.filteredHint", {
                  defaultValue: "Current filters matched {{filteredCount}} grant records. Clear filters to return to full ledger.",
                  filteredCount,
                })}
              </div>
            ) : null}
            {filtersActive && hasFilteredWalletLink ? (
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">
                  {t("accessPage.components.grantsSection.filteredIssuanceHint", {
                    defaultValue:
                      "You can jump to pass issuance with current filters and prefill the first matched target hint to reduce cross-page re-search.",
                  })}
                </p>
                <div className="mt-2">
                  <Button asChild size="sm" variant="outline">
                    <Link to={walletFilteredLink}>
                      {t("accessPage.components.grantsSection.continueIssuanceByFilter", { defaultValue: "Continue issuance by current filters" })}
                    </Link>
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
