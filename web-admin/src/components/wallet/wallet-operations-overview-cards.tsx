import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type WalletOperationsScenarioPreset = {
  id: string
  titleKey: string
  descriptionKey: string
  passType: "employee" | "visitor"
}

type WalletOperationsOverviewCardsProps = {
  scenarioPresets: WalletOperationsScenarioPreset[]
  templateScenarioCounts: Record<string, number>
  passScenarioCounts: Record<string, number>
  activeTemplateNameByScenario: Map<string, string>
  onApplyScenarioPreset: (scenarioID: string) => void
  activeEmployeeTemplateName: string
  employeePassCount: number
  onUseEmployeeTemplate: () => void
  showDirectorySyncLink: boolean
  showAccessGrantLinks: boolean
  activeVisitorTemplateName: string
  visitorPassCount: number
  onUseVisitorTemplate: () => void
  suspendedPassCount: number
  revocablePassCount: number
  onViewSuspended: () => void
  onViewAllStatuses: () => void
}

export function WalletOperationsOverviewCards({
  scenarioPresets,
  templateScenarioCounts,
  passScenarioCounts,
  activeTemplateNameByScenario,
  onApplyScenarioPreset,
  activeEmployeeTemplateName,
  employeePassCount,
  onUseEmployeeTemplate,
  showDirectorySyncLink,
  showAccessGrantLinks,
  activeVisitorTemplateName,
  visitorPassCount,
  onUseVisitorTemplate,
  suspendedPassCount,
  revocablePassCount,
  onViewSuspended,
  onViewAllStatuses,
}: WalletOperationsOverviewCardsProps) {
  const { t } = useTranslation()

  return (
    <>
      <div className="grid gap-4 xl:grid-cols-4">
        {scenarioPresets.map((item) => {
          const activeTemplateName = activeTemplateNameByScenario.get(item.id) || ""
          return (
            <Card key={item.id}>
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="space-y-1">
                    <CardTitle className="text-base">{t(item.titleKey)}</CardTitle>
                    <CardDescription>{t(item.descriptionKey)}</CardDescription>
                  </div>
                  <Badge variant="outline">
                    {item.passType === "employee" ? t("walletPage.labels.passType.employee") : t("walletPage.labels.passType.visitor")}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="rounded-lg border bg-muted/10 px-3 py-2 text-xs text-muted-foreground">
                  {t("walletPage.operations.scenarioStats", {
                    templateCount: templateScenarioCounts[item.id] ?? 0,
                    issuedCount: passScenarioCounts[item.id] ?? 0,
                  })}
                </div>
                <p className="text-sm text-muted-foreground">
                  {activeTemplateName
                    ? t("walletPage.operations.scenarioTemplateReady", { templateName: activeTemplateName })
                    : t("walletPage.operations.scenarioTemplateMissing")}
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Button size="sm" variant="outline" onClick={() => onApplyScenarioPreset(item.id)}>
                    {activeTemplateName ? t("walletPage.actions.switchToScenario") : t("walletPage.actions.applyPreset")}
                  </Button>
                  {showAccessGrantLinks && item.id === "visitor_temporary" ? (
                    <Button asChild size="sm" variant="outline">
                      <Link to="/access/grants">{t("walletPage.actions.goTemporaryAccessLedger")}</Link>
                    </Button>
                  ) : null}
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t("walletPage.cards.employeeIssuance.title")}</CardTitle>
            <CardDescription>{t("walletPage.cards.employeeIssuance.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {activeEmployeeTemplateName
                ? t("walletPage.cards.employeeIssuance.templateReady", {
                    templateName: activeEmployeeTemplateName,
                    count: employeePassCount,
                  })
                : t("walletPage.cards.employeeIssuance.templateMissing")}
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" variant="outline" disabled={!activeEmployeeTemplateName} onClick={onUseEmployeeTemplate}>
                {activeEmployeeTemplateName ? t("walletPage.actions.useEmployeeTemplate") : t("walletPage.actions.createEmployeeTemplateFirst")}
              </Button>
              {showDirectorySyncLink ? (
                <Button asChild size="sm" variant="outline">
                  <Link to="/enterprise#sync">{t("walletPage.actions.goSyncEmployeeDirectory")}</Link>
                </Button>
              ) : null}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t("walletPage.cards.visitorIssuance.title")}</CardTitle>
            <CardDescription>{t("walletPage.cards.visitorIssuance.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {activeVisitorTemplateName
                ? t("walletPage.cards.visitorIssuance.templateReady", {
                    templateName: activeVisitorTemplateName,
                    count: visitorPassCount,
                  })
                : t("walletPage.cards.visitorIssuance.templateMissing")}
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" variant="outline" disabled={!activeVisitorTemplateName} onClick={onUseVisitorTemplate}>
                {activeVisitorTemplateName ? t("walletPage.actions.useVisitorTemplate") : t("walletPage.actions.createVisitorTemplateFirst")}
              </Button>
              {showAccessGrantLinks ? (
                <Button asChild size="sm" variant="outline">
                  <Link to="/access/grants">{t("walletPage.actions.goTemporaryAccess")}</Link>
                </Button>
              ) : null}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t("walletPage.cards.statusMaintenance.title")}</CardTitle>
            <CardDescription>{t("walletPage.cards.statusMaintenance.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {t("walletPage.cards.statusMaintenance.stats", { suspendedCount: suspendedPassCount, revocableCount: revocablePassCount })}
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" variant="outline" onClick={onViewSuspended}>
                {t("walletPage.actions.viewSuspended")}
              </Button>
              <Button size="sm" variant="outline" onClick={onViewAllStatuses}>
                {t("walletPage.actions.viewAllStatuses")}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </>
  )
}
