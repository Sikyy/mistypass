import { ArrowUpRightIcon, ShieldCheckIcon, UsersRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export type EnterpriseOverviewActionItem = {
  title: string
  description: string
  actionLabel: string
  onClick: () => void
}

type EnterprisePageOverviewProps = {
  loading: boolean
  activeEmployeeCount: number
  syncJobsCount: number
  idpStatusLabel: string
  pendingApprovalCount: number
  effectiveError: string
  syncSummary: string
  attentionItems: EnterpriseOverviewActionItem[]
  quickActions: EnterpriseOverviewActionItem[]
}

export function EnterprisePageOverview({
  loading,
  activeEmployeeCount,
  syncJobsCount,
  idpStatusLabel,
  pendingApprovalCount,
  effectiveError,
  syncSummary,
  attentionItems,
  quickActions,
}: EnterprisePageOverviewProps) {
  const { t } = useTranslation()

  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[
          {
            label: t("enterprisePage.labels.employees"),
            value: loading ? "--" : activeEmployeeCount,
            note: t("enterprisePage.kpi.employeesHint"),
            icon: UsersRoundIcon,
          },
          {
            label: t("enterprisePage.labels.recentSyncJobs"),
            value: loading ? "--" : syncJobsCount,
            note: t("enterprisePage.kpi.syncJobsHint"),
            icon: ArrowUpRightIcon,
          },
          {
            label: t("enterprisePage.labels.ssoStatus"),
            value: loading ? "--" : idpStatusLabel,
            note: t("enterprisePage.kpi.ssoHint"),
            icon: ShieldCheckIcon,
          },
          {
            label: t("enterprisePage.labels.pendingApprovals"),
            value: loading ? "--" : pendingApprovalCount,
            note: t("enterprisePage.kpi.approvalsHint"),
            icon: ShieldCheckIcon,
          },
        ].map((item) => (
          <div key={item.label} className="mp-metric-card">
            <div className="relative z-10 flex items-start justify-between gap-3">
              <div>
                <p className="text-sm text-muted-foreground">{item.label}</p>
                <p className="mt-1 text-3xl font-semibold tracking-[-0.04em]">{item.value}</p>
              </div>
              <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
                <item.icon className="size-4 text-white/75" />
              </div>
            </div>
            <p className="relative z-10 mt-4 mp-kpi-note">{item.note}</p>
          </div>
        ))}
      </div>

      {effectiveError ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {effectiveError}
        </div>
      ) : null}

      {syncSummary ? (
        <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
          {syncSummary}
        </div>
      ) : null}

      {attentionItems.length > 0 ? (
        <div className="grid gap-4 xl:grid-cols-2">
          {attentionItems.map((item) => (
            <Card key={item.title} className="border-amber-500/30 bg-amber-500/5">
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{item.title}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground">{item.description}</p>
                <Button size="sm" variant="outline" onClick={item.onClick}>
                  {item.actionLabel}
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-4">
        {quickActions.map((item) => (
          <Card key={item.title}>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">{item.title}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="text-sm text-muted-foreground">{item.description}</p>
              <Button variant="outline" size="sm" onClick={item.onClick}>
                {item.actionLabel}
                <ArrowUpRightIcon className="ml-1.5 size-4" />
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>
    </>
  )
}
