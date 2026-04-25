import { TrendingUpIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type WalletJobMetrics, type WalletJobMetricsTrend } from "@/lib/api"

type WalletAlertTrendPanelsProps = {
  loading: boolean
  metrics: WalletJobMetrics | null
  metricsTrend: WalletJobMetricsTrend | null
  trendPeakUpdated: number
  formatDurationSeconds: (seconds?: number) => string
  formatTimeLabel: (value?: string) => string
  alertItems: WalletJobMetrics["alerts"] | undefined
  windowErrorCodeRows: [string, number][]
  formatDateTime: (value?: string) => string
}

export function WalletAlertTrendPanels({
  loading,
  metrics,
  metricsTrend,
  trendPeakUpdated,
  formatDurationSeconds,
  formatTimeLabel,
  alertItems,
  windowErrorCodeRows,
  formatDateTime,
}: WalletAlertTrendPanelsProps) {
  const { t } = useTranslation()
  const safeAlertItems = alertItems ?? []

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="inline-flex items-center gap-2 text-base">
            <TrendingUpIcon className="size-4 text-cyan-500" />
            {t("walletPage.components.alertTrend.title", { defaultValue: "Issuance runtime trend" })}
          </CardTitle>
          <CardDescription>
            {t("walletPage.components.alertTrend.description", {
              defaultValue: "Window {{window}}, bucket {{bucket}}, total {{count}} buckets.",
              window: loading ? "--" : formatDurationSeconds(metricsTrend?.window_seconds),
              bucket: loading ? "--" : formatDurationSeconds(metricsTrend?.bucket_seconds),
              count: loading ? "--" : (metricsTrend?.bucket_count ?? 0),
            })}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {loading ? (
            <div className="py-8 text-center text-sm text-muted-foreground">
              {t("walletPage.components.alertTrend.loading", { defaultValue: "Loading trend data..." })}
            </div>
          ) : null}
          {!loading && (!metricsTrend || metricsTrend.buckets.length === 0) ? (
            <div className="py-8 text-center text-sm text-muted-foreground">
              {t("walletPage.components.alertTrend.empty", { defaultValue: "No trend data in current window." })}
            </div>
          ) : null}
          {!loading &&
            (metricsTrend?.buckets ?? []).map((item) => {
              const successWidth = item.success > 0 ? Math.max(6, Math.round((item.success / trendPeakUpdated) * 100)) : 0
              const failedWidth = item.failed > 0 ? Math.max(6, Math.round((item.failed / trendPeakUpdated) * 100)) : 0
              const dlqWidth = item.dlq > 0 ? Math.max(6, Math.round((item.dlq / trendPeakUpdated) * 100)) : 0
              return (
                <div key={`${item.index}-${item.start}`} className="rounded-lg border bg-muted/20 px-3 py-2">
                  <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
                    <span>
                      {formatTimeLabel(item.start)} - {formatTimeLabel(item.end)}
                    </span>
                    <span>
                      updated {item.updated} / created {item.created}
                    </span>
                  </div>
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2 text-xs">
                      <span className="w-10 text-muted-foreground">success</span>
                      <div className="h-2 flex-1 rounded bg-emerald-500/15">
                        <div className="h-2 rounded bg-emerald-500" style={{ width: `${successWidth}%` }} />
                      </div>
                      <span className="w-6 text-right">{item.success}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="w-10 text-muted-foreground">failed</span>
                      <div className="h-2 flex-1 rounded bg-amber-500/15">
                        <div className="h-2 rounded bg-amber-500" style={{ width: `${failedWidth}%` }} />
                      </div>
                      <span className="w-6 text-right">{item.failed}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="w-10 text-muted-foreground">dlq</span>
                      <div className="h-2 flex-1 rounded bg-red-500/15">
                        <div className="h-2 rounded bg-red-500" style={{ width: `${dlqWidth}%` }} />
                      </div>
                      <span className="w-6 text-right">{item.dlq}</span>
                    </div>
                  </div>
                </div>
              )
            })}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("walletPage.components.alertTrend.thresholdTitle", { defaultValue: "Issuance anomaly thresholds" })}</CardTitle>
            <CardDescription>
              {t("walletPage.components.alertTrend.thresholdDescription", {
                defaultValue: "Current threshold: {{threshold}} (`dlq_alert_threshold`)",
                threshold: loading ? "--" : (metrics?.dlq_alert_threshold ?? 0),
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("walletPage.components.alertTrend.columns.type", { defaultValue: "Type" })}</TableHead>
                  <TableHead>{t("walletPage.components.alertTrend.columns.errorCode", { defaultValue: "Error code" })}</TableHead>
                  <TableHead>{t("walletPage.components.alertTrend.columns.count", { defaultValue: "Count" })}</TableHead>
                  <TableHead>{t("walletPage.components.alertTrend.columns.threshold", { defaultValue: "Threshold" })}</TableHead>
                  <TableHead>{t("walletPage.components.alertTrend.columns.status", { defaultValue: "Status" })}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-10 text-center text-muted-foreground">
                      {t("walletPage.components.alertTrend.loadingAlerts", { defaultValue: "Loading alerts..." })}
                    </TableCell>
                  </TableRow>
                ) : null}
                {!loading && safeAlertItems.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                      {t("walletPage.components.alertTrend.emptyAlerts", { defaultValue: "No threshold-hit alerts." })}
                    </TableCell>
                  </TableRow>
                ) : null}
                {!loading &&
                  safeAlertItems.map((item, index) => (
                    <TableRow key={`${item.type}-${item.error_code ?? "unknown"}-${index}`}>
                      <TableCell>{item.type}</TableCell>
                      <TableCell>{item.error_code ?? "-"}</TableCell>
                      <TableCell>{item.count}</TableCell>
                      <TableCell>{item.threshold}</TableCell>
                      <TableCell>
                        <Badge variant={item.count >= item.threshold ? "destructive" : "outline"}>
                          {item.count >= item.threshold
                            ? t("walletPage.components.alertTrend.overThreshold", { defaultValue: "Over threshold" })
                            : t("walletPage.components.alertTrend.normal", { defaultValue: "Normal" })}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("walletPage.components.alertTrend.topErrorsTitle", { defaultValue: "Failure reason distribution (Top 5)" })}</CardTitle>
            <CardDescription>
              {t("walletPage.components.alertTrend.topErrorsDescription", {
                defaultValue: "Statistics range: {{since}} - {{until}}",
                since: loading ? "--" : formatDateTime(metrics?.window.since),
                until: loading ? "--" : formatDateTime(metrics?.window.until),
              })}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading ? (
              <div className="py-8 text-center text-sm text-muted-foreground">
                {t("walletPage.components.alertTrend.loadingErrorCodes", { defaultValue: "Loading error code distribution..." })}
              </div>
            ) : null}
            {!loading && windowErrorCodeRows.length === 0 ? (
              <div className="py-8 text-center text-sm text-muted-foreground">
                {t("walletPage.components.alertTrend.emptyErrorCodes", { defaultValue: "No error codes in current window." })}
              </div>
            ) : null}
            {!loading &&
              windowErrorCodeRows.map(([code, count]) => (
                <div
                  key={code}
                  className="flex items-center justify-between rounded-lg border bg-muted/25 px-3 py-2 text-sm"
                >
                  <span className="font-medium">{code}</span>
                  <Badge variant="outline">{count}</Badge>
                </div>
              ))}
          </CardContent>
        </Card>
      </div>
    </>
  )
}
