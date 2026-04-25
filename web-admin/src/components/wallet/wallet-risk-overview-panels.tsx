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
            <CardDescription>{t("walletPage.components.riskOverview.pendingAnomalies", { defaultValue: "Pending anomalies" })}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (metrics?.summary.dlq ?? 0)}
              <ShieldAlertIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("walletPage.components.riskOverview.thresholdHits", {
              defaultValue: "Threshold hits: {{count}}",
              count: loading ? "--" : safeAlertItems.length,
            })}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.retryableAnomalies", { defaultValue: "Retryable anomalies" })}</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : (metrics?.summary.retryable_failed ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.retryableHint", { defaultValue: "`status=failed` and retryable automatically." })}</CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.windowCreated", { defaultValue: "Created in window" })}</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : (metrics?.window.created ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("walletPage.components.riskOverview.windowCreatedHint", {
              defaultValue: "Unified count for employee/visitor/temporary passes in latest {{duration}}.",
              duration: loading ? "--" : formatDurationSeconds(metrics?.window.window_seconds),
            })}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("walletPage.components.riskOverview.statusUpdates", { defaultValue: "Issuance status updates" })}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (metrics?.window.updated ?? 0)}
              <WalletCardsIcon className="size-4 text-cyan-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("walletPage.components.riskOverview.statusUpdatesHint", {
              defaultValue: "Success {{success}} / Failed {{failed}} / DLQ {{dlq}}",
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
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantTotal", { defaultValue: "Cross-tenant total jobs" })}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.total}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantTotalHint", { defaultValue: "Tenant-level aggregate for current query window." })}</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantFailed", { defaultValue: "Cross-tenant failed" })}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.failed}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantFailedHint", { defaultValue: "Aggregate across all tenants with `status=failed`." })}</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantDlq", { defaultValue: "Cross-tenant DLQ" })}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.dlq}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantDlqHint", { defaultValue: "Aggregate across all tenants with `status=dlq`." })}</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t("walletPage.components.riskOverview.crossTenantAlertTenants", { defaultValue: "Tenants hitting threshold" })}</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.alertTenants}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">{t("walletPage.components.riskOverview.crossTenantAlertTenantsHint", { defaultValue: "Count of tenants where `alerts.length` > 0." })}</CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("walletPage.components.riskOverview.rankingTitle", { defaultValue: "Cross-tenant issuance risk ranking" })}</CardTitle>
              <CardDescription>{t("walletPage.components.riskOverview.rankingDescription", { defaultValue: "Sort by anomaly backlog and failures descending to prioritize high-risk tenants." })}</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("walletPage.components.riskOverview.columns.tenant", { defaultValue: "Tenant" })}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.total", { defaultValue: "Total jobs" })}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.failed", { defaultValue: "Failed" })}</TableHead>
                    <TableHead>DLQ</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.retryableFailed", { defaultValue: "Retryable failed" })}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.alerts", { defaultValue: "Alerts" })}</TableHead>
                    <TableHead>{t("walletPage.components.riskOverview.columns.updatedAt", { defaultValue: "Updated at" })}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-10 text-center text-muted-foreground">
                        {t("walletPage.components.riskOverview.loadingAggregates", { defaultValue: "Loading cross-tenant aggregates..." })}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading && tenantAggregates.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                        {t("walletPage.components.riskOverview.emptyAggregates", { defaultValue: "No cross-tenant aggregate data." })}
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
