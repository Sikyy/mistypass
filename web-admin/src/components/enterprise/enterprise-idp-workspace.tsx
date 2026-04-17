import { Link } from "react-router-dom"

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
  return (
    <TabsContent value="idp">
      <div className="grid gap-4 xl:grid-cols-[0.95fr_1.05fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">企业登录落地工作区</CardTitle>
            <CardDescription>把 SSO、目录来源和审批积压放在一起看，避免企业自身登录只是一个配置快照。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{idpReady ? "企业登录已具备基础条件" : "企业登录仍有前置缺口"}</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {idpReady
                  ? "当前可以围绕审批积压、目录同步和 JIT 开通继续做收口。"
                  : "建议先补 IdP 配置，再回到审批与异常处理自动开户、同步失败和 worker 告警。"}
              </p>
            </div>

            <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-1">
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">企业登录</p>
                <p className="mt-1 text-sm font-medium">{loading ? "--" : idpConfig?.status || "未配置"}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {idpConfig ? `${idpConfig.provider} / ${idpConfig.sync_mode}` : "建议先启用企业 SSO。"}
                </p>
                <Button size="sm" variant="outline" className="mt-3" onClick={() => goToSection("idp")}>
                  {idpReady ? "复核企业登录" : "去企业登录"}
                </Button>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">目录来源</p>
                <p className="mt-1 text-sm font-medium">
                  {loading ? "--" : activeEmployeeCount > 0 ? `${activeEmployeeCount} 名员工` : "尚未接通"}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {syncJobsCount > 0 ? `最近同步 ${syncJobsCount} 次` : "建议先接入 HRIS、SCIM、CSV 或手动同步。"}
                </p>
                <Button
                  size="sm"
                  variant="outline"
                  className="mt-3"
                  onClick={() => goToSection(activeEmployeeCount > 0 ? "employees" : "sync")}
                >
                  {activeEmployeeCount > 0 ? "查看员工目录" : "去导入与同步"}
                </Button>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">审批与异常</p>
                <p className="mt-1 text-sm font-medium">
                  {loading ? "--" : `${pendingApprovalCount} 条待审批 / ${failedSyncJobCount} 条待复核`}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {workerAlertCount > 0 ? `最近累计 ${workerAlertCount} 条 worker 告警。` : "当前未发现额外 worker 告警。"}
                </p>
                <Button size="sm" variant="outline" className="mt-3" onClick={() => goToSection("alerts")}>
                  去审批与异常
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">企业登录 / SSO 配置概览</CardTitle>
            <CardDescription>把“企业自身登录”单独抬成一个模块，不再和平台级租户管理叙事混在同一语境。</CardDescription>
          </CardHeader>
          <CardContent>
            {!idpConfig ? (
              <div className="rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground">
                当前组织还没有企业登录配置。建议先接入 IdP，再配合 JIT Provision 和员工同步。
              </div>
            ) : (
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">Provider</p>
                  <p className="mt-1 font-medium">{idpConfig.provider}</p>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">状态</p>
                  <p className="mt-1 font-medium">{idpConfig.status}</p>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">同步模式</p>
                  <p className="mt-1 font-medium">{idpConfig.sync_mode}</p>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <p className="mp-kpi-note">最近更新</p>
                  <p className="mt-1 font-medium">{formatDateTime(idpConfig.updated_at)}</p>
                </div>
              </div>
            )}

            {idpConfig ? (
              <div className="mt-4 grid gap-4 md:grid-cols-2">
                <div className="rounded-lg border bg-muted/15 p-3">
                  <p className="mp-kpi-note">Issuer URL</p>
                  <p className="mt-1 break-all text-sm">{idpConfig.issuer_url || "-"}</p>
                </div>
                <div className="rounded-lg border bg-muted/15 p-3">
                  <p className="mp-kpi-note">Scopes</p>
                  <p className="mt-1 text-sm">{idpConfig.scopes?.join(", ") || "-"}</p>
                </div>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card className="xl:col-span-2" data-testid="enterprise-idp-outcome">
          <CardHeader>
            <CardTitle className="text-base">企业登录完成后的下一步</CardTitle>
            <CardDescription>企业登录配置不是终点。配置完成后，应该继续把目录、审批、策略和发放主路径接起来。</CardDescription>
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
                  <Link to={directoryLink}>去员工与用户组</Link>
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={policiesLink}>去权限策略</Link>
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>
  )
}
