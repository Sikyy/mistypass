import { ShieldAlertIcon, WalletCardsIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type WalletJobMetrics } from "@/lib/api"

type WalletTenantAggregateRow = {
  tenantID: string
  tenantName: string
  total: number
  failed: number
  dlq: number
  retryableFailed: number
  alertCount: number
  updatedAt: string
}

type WalletRiskOverviewPanelsProps = {
  loading: boolean
  platformViewer: boolean
  effectiveError: string
  aggregateWarning: string
  metrics: WalletJobMetrics | null
  alertItems: WalletJobMetrics["alerts"] | undefined
  formatDurationSeconds: (seconds?: number) => string
  aggregateStats: {
    total: number
    failed: number
    dlq: number
    alertTenants: number
  }
  tenantAggregates: WalletTenantAggregateRow[]
  formatDateTime: (value?: string) => string
}

export function WalletRiskOverviewPanels({
  loading,
  platformViewer,
  effectiveError,
  aggregateWarning,
  metrics,
  alertItems,
  formatDurationSeconds,
  aggregateStats,
  tenantAggregates,
  formatDateTime,
}: WalletRiskOverviewPanelsProps) {
  const { t } = useTranslation()
  const safeAlertItems = alertItems ?? []

  return (
    <>
      {effectiveError ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {effectiveError}
        </div>
      ) : null}
      {platformViewer && aggregateWarning ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700">
          {aggregateWarning}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.pendingAnomalies")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (metrics?.summary.dlq ?? 0)}
              <ShieldAlertIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("walletPage.components.riskOverview.thresholdHits", {
              count: loading ? "--" : safeAlertItems.length,
            })}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.retryableAnomalies")}</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : (metrics?.summary.retryable_failed ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.retryableHint")}</CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.windowCreated")}</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : (metrics?.window.created ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("walletPage.components.riskOverview.windowCreatedHint", {
              duration: loading ? "--" : formatDurationSeconds(metrics?.window.window_seconds),
            })}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.statusUpdates")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (metrics?.window.updated ?? 0)}
              <WalletCardsIcon className="size-4 text-cyan-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("walletPage.components.riskOverview.statusUpdatesHint", {
              success: loading ? "--" : (metrics?.window.success ?? 0),
              failed: loading ? "--" : (metrics?.window.failed ?? 0),
              dlq: loading ? "--" : (metrics?.window.dlq ?? 0),
            })}
          </CardContent>
        </Card>
      </div>

      {platformViewer ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantTotal")}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.total}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantTotalHint")}</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantFailed")}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.failed}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantFailedHint")}</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantDlq")}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.dlq}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantDlqHint")}</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantAlertTenants")}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.alertTenants}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantAlertTenantsHint")}</CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("walletPage.components.riskOverview.rankingTitle")}</CardTitle>
              <CardDescription>{t("walletPage.components.riskOverview.rankingDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("walletPage.components.riskOverview.columns.tenant")}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.total")}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.failed")}</TableHead>
                    <TableHead>DLQ</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.retryableFailed")}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.alerts")}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.updatedAt")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-10 text-center text-muted-foreground">
                        {t("walletPage.components.riskOverview.loadingAggregates")}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading && tenantAggregates.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                        {t("walletPage.components.riskOverview.emptyAggregates")}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading &&
                    tenantAggregates.map((row) => (
                      <TableRow key={row.tenantID}>
                        <TableCell className="font-medium">
                          <TableCellText className="max-w-[14rem]">{row.tenantName}</TableCellText>
                        </TableCell>
                        <TableCell>{row.total}</TableCell>
                        <TableCell>{row.failed}</TableCell>
                        <TableCell>{row.dlq}</TableCell>
                        <TableCell>{row.retryableFailed}</TableCell>
                        <TableCell>
                          <Badge variant={row.alertCount > 0 ? "destructive" : "outline"}>{row.alertCount}</Badge>
                        </TableCell>
                        <TableCell className="mp-kpi-note">{formatDateTime(row.updatedAt)}</TableCell>
                      </TableRow>
                    ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      ) : null}
    </>
  )
}
