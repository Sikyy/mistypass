import { type ComponentProps, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type BadgeVariant = ComponentProps<typeof Badge>["variant"]
type EnterpriseWorkflowOverviewSection = "employees" | "sync" | "idp" | "alerts"

export type EnterpriseWorkflowOverviewStep = {
  id: "sync" | "directory" | "policies" | "issuance"
  title: string
  metric: string
  helper: string
  statusLabel: string
  statusVariant: BadgeVariant
  action:
    | {
        kind: "section"
        section: EnterpriseWorkflowOverviewSection
        label: string
      }
    | {
        kind: "route"
        to: string
        label: string
      }
}

export type EnterpriseWorkflowOverviewNextAction =
  | {
      title: string
      description: string
      kind: "section"
      section: EnterpriseWorkflowOverviewSection
      label: string
    }
  | {
      title: string
      description: string
      kind: "route"
      to: string
      label: string
    }

type EnterpriseWorkflowOverviewProps = {
  loading: boolean
  tenantGroupsCount: number
  tenantPoliciesCount: number
  issuedPassCount: number
  workflowSteps: EnterpriseWorkflowOverviewStep[]
  nextWorkflowAction: EnterpriseWorkflowOverviewNextAction
  directoryFlowLink: string
  policiesFlowLink: string
  onGoToSection: (section: EnterpriseWorkflowOverviewSection) => void
}

export function EnterpriseWorkflowOverview({
  loading,
  tenantGroupsCount,
  tenantPoliciesCount,
  issuedPassCount,
  workflowSteps,
  nextWorkflowAction,
  directoryFlowLink,
  policiesFlowLink,
  onGoToSection,
}: EnterpriseWorkflowOverviewProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("enterprisePage.workflow.title")}</CardTitle>
          <CardDescription>{t("enterprisePage.workflow.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {workflowSteps.map((step) => {
            let action: ReactNode
            if (step.action.kind === "section") {
              const sectionAction = step.action
              action = (
                <Button size="sm" variant="outline" onClick={() => onGoToSection(sectionAction.section)}>
                  {sectionAction.label}
                </Button>
              )
            } else {
              const routeAction = step.action
              action = (
                <Button asChild size="sm" variant="outline">
                  <Link to={routeAction.to}>{routeAction.label}</Link>
                </Button>
              )
            }

            return (
              <div
                key={step.id}
                className="flex flex-col gap-3 rounded-xl border bg-muted/10 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{step.title}</p>
                    <Badge variant={step.statusVariant}>{step.statusLabel}</Badge>
                  </div>
                  <p className="text-sm">{step.metric}</p>
                  <p className="mp-kpi-note">{step.helper}</p>
                </div>
                {action}
              </div>
            )
          })}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("enterprisePage.nextAction.cardTitle")}</CardTitle>
          <CardDescription>{t("enterprisePage.nextAction.cardDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <p className="font-medium">{nextWorkflowAction.title}</p>
            <p className="mt-1 text-sm text-muted-foreground">{nextWorkflowAction.description}</p>
          </div>

          <div className="grid gap-3 sm:grid-cols-3 xl:grid-cols-1">
            <div className="rounded-lg border bg-muted/10 px-3 py-2">
              <p className="mp-kpi-note">{t("enterprisePage.labels.userGroups")}</p>
              <p className="mt-1 text-sm font-medium">
                {loading ? "--" : t("enterprisePage.nextAction.stats.groups", { count: tenantGroupsCount })}
              </p>
            </div>
            <div className="rounded-lg border bg-muted/10 px-3 py-2">
              <p className="mp-kpi-note">{t("enterprisePage.labels.policies")}</p>
              <p className="mt-1 text-sm font-medium">
                {loading ? "--" : t("enterprisePage.nextAction.stats.policies", { count: tenantPoliciesCount })}
              </p>
            </div>
            <div className="rounded-lg border bg-muted/10 px-3 py-2">
              <p className="mp-kpi-note">{t("enterprisePage.labels.issuedPasses")}</p>
              <p className="mt-1 text-sm font-medium">
                {loading ? "--" : t("enterprisePage.nextAction.stats.issuedPasses", { count: issuedPassCount })}
              </p>
            </div>
          </div>

          {nextWorkflowAction.kind === "section" ? (
            <Button className="w-full" onClick={() => onGoToSection(nextWorkflowAction.section)}>
              {nextWorkflowAction.label}
            </Button>
          ) : (
            <Button asChild className="w-full">
              <Link to={nextWorkflowAction.to}>{nextWorkflowAction.label}</Link>
            </Button>
          )}

          <div className="flex flex-wrap items-center gap-2">
            <Button asChild size="sm" variant="outline">
              <Link to={directoryFlowLink}>{t("enterprisePage.actions.goToDirectory")}</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={policiesFlowLink}>{t("enterprisePage.actions.goToPolicies")}</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
