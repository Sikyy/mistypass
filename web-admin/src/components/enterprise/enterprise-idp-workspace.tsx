import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { TabsContent } from "@/components/ui/tabs"
import { type EnterpriseIDPConfig } from "@/lib/api"

type EnterpriseSection = "employees" | "sync" | "idp" | "alerts"

type EnterpriseOutcomeAction = {
  description: string
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  title: string
  to?: string
}

type EnterpriseIDPWorkspaceProps = {
  activeEmployeeCount: number
  directoryLink: string
  failedSyncJobCount: number
  formatDateTime: (value?: string) => string
  goToSection: (section: EnterpriseSection) => void
  idpConfig: EnterpriseIDPConfig | null
  idpOutcomeAction: EnterpriseOutcomeAction
  idpReady: boolean
  loading: boolean
  pendingApprovalCount: number
  policiesLink: string
  syncJobsCount: number
  workerAlertCount: number
}

export function EnterpriseIDPWorkspace({
  activeEmployeeCount,
  directoryLink,
  failedSyncJobCount,
  formatDateTime,
  goToSection,
  idpConfig,
  idpOutcomeAction,
  idpReady,
  loading,
  pendingApprovalCount,
  policiesLink,
  syncJobsCount,
  workerAlertCount,
}: EnterpriseIDPWorkspaceProps) {
  const { t } = useTranslation()
  return (
    <TabsContent value="idp">
      <div className="grid gap-4 xl:grid-cols-[0.95fr_1.05fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseIdpWorkspace.title")}</CardTitle>
            <CardDescription>{t("enterpriseIdpWorkspace.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">
                {idpReady
                  ? t("enterpriseIdpWorkspace.readiness.readyTitle")
                  : t("enterpriseIdpWorkspace.readiness.notReadyTitle")}
              </p>
              <p className="mt-1 text-sm text-muted-foreground">
                {idpReady
                  ? t("enterpriseIdpWorkspace.readiness.readyDescription")
                  : t("enterpriseIdpWorkspace.readiness.notReadyDescription")}
              </p>
            </div>

            <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-1">
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.kpi.enterpriseLogin.title")}</p>
                <p className="mt-1 text-sm font-medium">
                  {loading ? "--" : idpConfig?.status || t("enterpriseIdpWorkspace.unconfigured")}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {idpConfig
                    ? `${idpConfig.provider} / ${idpConfig.sync_mode}`
                    : t("enterpriseIdpWorkspace.kpi.enterpriseLogin.hintNoConfig")}
                </p>
                <Button size="sm" variant="outline" className="mt-3" onClick={() => goToSection("idp")}>
                  {idpReady
                    ? t("enterpriseIdpWorkspace.kpi.enterpriseLogin.review")
                    : t("enterpriseIdpWorkspace.kpi.enterpriseLogin.go")}
                </Button>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.kpi.directorySource.title")}</p>
                <p className="mt-1 text-sm font-medium">
                  {loading
                    ? "--"
                    : activeEmployeeCount > 0
                      ? t("enterpriseIdpWorkspace.kpi.directorySource.employeeCount", { count: activeEmployeeCount })
                      : t("enterpriseIdpWorkspace.kpi.directorySource.notConnected")}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {syncJobsCount > 0
                    ? t("enterpriseIdpWorkspace.kpi.directorySource.syncCount", { count: syncJobsCount })
                    : t("enterpriseIdpWorkspace.kpi.directorySource.hintNoSync")}
                </p>
                <Button
                  size="sm"
                  variant="outline"
                  className="mt-3"
                  onClick={() => goToSection(activeEmployeeCount > 0 ? "employees" : "sync")}
                >
                  {activeEmployeeCount > 0
                    ? t("enterpriseIdpWorkspace.kpi.directorySource.viewEmployees")
                    : t("enterpriseIdpWorkspace.kpi.directorySource.goSync")}
                </Button>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.kpi.approvals.title")}</p>
                <p className="mt-1 text-sm font-medium">
                  {loading
                    ? "--"
                    : t("enterpriseIdpWorkspace.kpi.approvals.metric", {
                        pending: pendingApprovalCount,
                        failed: failedSyncJobCount,
                      })}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {workerAlertCount > 0
                    ? t("enterpriseIdpWorkspace.kpi.approvals.workerAlertCount", { count: workerAlertCount })
                    : t("enterpriseIdpWorkspace.kpi.approvals.noWorkerAlert")}
                </p>
                <Button size="sm" variant="outline" className="mt-3" onClick={() => goToSection("alerts")}>
                  {t("enterpriseIdpWorkspace.kpi.approvals.go")}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseIdpWorkspace.overview.title")}</CardTitle>
            <CardDescription>{t("enterpriseIdpWorkspace.overview.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            {!idpConfig ? (
              <div className="rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground">
                {t("enterpriseIdpWorkspace.overview.empty")}
              </div>
            ) : (
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.overview.provider")}</p>
                  <p className="mt-1 font-medium">{idpConfig.provider}</p>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.overview.status")}</p>
                  <p className="mt-1 font-medium">{idpConfig.status}</p>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.overview.syncMode")}</p>
                  <p className="mt-1 font-medium">{idpConfig.sync_mode}</p>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.overview.updatedAt")}</p>
                  <p className="mt-1 font-medium">{formatDateTime(idpConfig.updated_at)}</p>
                </div>
              </div>
            )}

            {idpConfig ? (
              <div className="mt-4 grid gap-4 md:grid-cols-2">
                <div className="rounded-lg border bg-muted/15 p-3">
                  <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.overview.issuerURL")}</p>
                  <p className="mt-1 break-all text-sm">{idpConfig.issuer_url || "-"}</p>
                </div>
                <div className="rounded-lg border bg-muted/15 p-3">
                  <p className="mp-kpi-note">{t("enterpriseIdpWorkspace.overview.scopes")}</p>
                  <p className="mt-1 text-sm">{idpConfig.scopes?.join(", ") || "-"}</p>
                </div>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card className="xl:col-span-2" data-testid="enterprise-idp-outcome">
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseIdpWorkspace.outcome.title")}</CardTitle>
            <CardDescription>{t("enterpriseIdpWorkspace.outcome.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{idpOutcomeAction.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{idpOutcomeAction.description}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                {idpOutcomeAction.kind === "section" ? (
                  <Button size="sm" data-testid="enterprise-idp-outcome-action" onClick={() => goToSection(idpOutcomeAction.section!)}>
                    {idpOutcomeAction.label}
                  </Button>
                ) : (
                  <Button asChild size="sm" data-testid="enterprise-idp-outcome-action">
                    <Link to={idpOutcomeAction.to!}>{idpOutcomeAction.label}</Link>
                  </Button>
                )}
                <Button asChild size="sm" variant="outline">
                  <Link to={directoryLink}>{t("enterpriseIdpWorkspace.outcome.goDirectory")}</Link>
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={policiesLink}>{t("enterpriseIdpWorkspace.outcome.goPolicies")}</Link>
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>
  )
}
