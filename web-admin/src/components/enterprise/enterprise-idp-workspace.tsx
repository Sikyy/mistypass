import { useState } from "react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CheckCircleIcon, PlusIcon, XCircleIcon } from "lucide-react"

import { useAuth } from "@/context/auth-context"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { TabsContent } from "@/components/ui/tabs"
import {
  listEnterpriseDomainMappings,
  createEnterpriseDomainMapping,
  updateEnterpriseDomainMappingStatus,
  type EnterpriseIDPConfig,
  type EnterpriseDomainMapping,
} from "@/lib/api"

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

        <DomainMappingsSection />
      </div>
    </TabsContent>
  )
}

// ---------------------------------------------------------------------------
// Domain Mappings Section
// ---------------------------------------------------------------------------

function DomainMappingsSection() {
  const { token } = useAuth()
  const queryClient = useQueryClient()
  const [newDomain, setNewDomain] = useState("")
  const [showAddForm, setShowAddForm] = useState(false)

  const mappingsQuery = useQuery({
    queryKey: ["enterprise-domain-mappings"],
    queryFn: () => listEnterpriseDomainMappings(token ?? undefined),
    enabled: Boolean(token),
  })

  const createMutation = useMutation({
    mutationFn: (domain: string) =>
      createEnterpriseDomainMapping(token ?? undefined, { domain }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["enterprise-domain-mappings"] })
      setNewDomain("")
      setShowAddForm(false)
    },
  })

  const toggleStatusMutation = useMutation({
    mutationFn: ({ mappingId, nextStatus }: { mappingId: string; nextStatus: string }) =>
      updateEnterpriseDomainMappingStatus(token ?? undefined, mappingId, nextStatus),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["enterprise-domain-mappings"] })
    },
  })

  const mappings: EnterpriseDomainMapping[] = mappingsQuery.data ?? []

  return (
    <Card className="xl:col-span-2" data-testid="enterprise-idp-domain-mappings">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Domain Mappings</CardTitle>
            <CardDescription>
              Map email domains to this tenant for automatic identity routing.
            </CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowAddForm(!showAddForm)}
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Domain
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {showAddForm ? (
          <div className="rounded-lg border bg-muted/10 px-4 py-3">
            <div className="flex items-end gap-3">
              <div className="flex-1">
                <label className="mb-1 block text-sm font-medium">Domain</label>
                <Input
                  value={newDomain}
                  onChange={(e) => setNewDomain(e.target.value)}
                  placeholder="company.com"
                />
              </div>
              <Button
                onClick={() => createMutation.mutate(newDomain)}
                disabled={createMutation.isPending || !newDomain.trim()}
              >
                {createMutation.isPending ? "Adding..." : "Add"}
              </Button>
              <Button variant="outline" onClick={() => setShowAddForm(false)}>
                Cancel
              </Button>
            </div>
            {createMutation.isError ? (
              <div className="mt-2 flex items-center gap-2 text-sm text-red-800">
                <XCircleIcon className="size-4" />
                {(createMutation.error as Error).message || "Failed to add domain"}
              </div>
            ) : null}
          </div>
        ) : null}

        {mappingsQuery.isLoading ? (
          <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
            Loading domain mappings...
          </div>
        ) : mappings.length === 0 ? (
          <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
            No domain mappings configured. Add a domain to enable identity routing.
          </div>
        ) : (
          <div className="space-y-2">
            {mappings.map((mapping) => {
              const isActive = mapping.status === "active"
              return (
                <div
                  key={mapping.id}
                  className="flex items-center justify-between rounded-lg border bg-muted/10 px-4 py-3"
                >
                  <div className="flex items-center gap-3">
                    <div>
                      <p className="font-medium">{mapping.domain}</p>
                      <p className="text-xs text-muted-foreground">
                        Created {new Date(mapping.created_at).toLocaleDateString()}
                        {mapping.verified_at
                          ? ` / Verified ${new Date(mapping.verified_at).toLocaleDateString()}`
                          : ""}
                      </p>
                    </div>
                    <Badge variant={isActive ? "outline" : "secondary"}>
                      {mapping.status}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">
                      {isActive ? "Active" : "Disabled"}
                    </span>
                    <Switch
                      checked={isActive}
                      disabled={toggleStatusMutation.isPending}
                      onCheckedChange={(checked) =>
                        toggleStatusMutation.mutate({
                          mappingId: mapping.id,
                          nextStatus: checked ? "active" : "disabled",
                        })
                      }
                    />
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
