import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { useTranslation } from "react-i18next"
import { ActivityIcon, GaugeIcon, RadioTowerIcon, RouteIcon } from "lucide-react"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type GatewayCheckpointSummaryResponse } from "@/lib/api"

function checkpointTrendDirectionLabel(direction: string, t: (key: string) => string) {
  switch (direction) {
    case "up":
      return t("gateways.checkpoint.direction.up")
    case "down":
      return t("gateways.checkpoint.direction.down")
    case "flat":
      return t("gateways.checkpoint.direction.flat")
    default:
      return direction
  }
}

function checkpointTrendDirectionVariant(direction: string) {
  switch (direction) {
    case "up":
      return "outline"
    case "down":
      return "destructive"
    default:
      return "secondary"
  }
}

type CheckpointMonitorProps = {
  checkpointTrendWindowMinutes: "15" | "60" | "240"
  onWindowMinutesChange: (value: "15" | "60" | "240") => void
  selectedGatewayRecord: { id: string } | null
  checkpointSummaryLoading: boolean
  checkpointSummaryError: string
  checkpointSummary: GatewayCheckpointSummaryResponse | null
}

export function CheckpointMonitor({
  checkpointTrendWindowMinutes,
  onWindowMinutesChange,
  selectedGatewayRecord,
  checkpointSummaryLoading,
  checkpointSummaryError,
  checkpointSummary,
}: CheckpointMonitorProps) {
  const { t } = useTranslation()
  const trend = checkpointSummary?.time_window_trend
  const lagTotal = checkpointSummary?.totals.lag_total ?? 0

  return (
    <Card className="mp-fog-surface">
      <CardHeader className="gap-3 md:flex md:flex-row md:items-end md:justify-between">
        <div>
          <CardTitle className="flex items-center gap-2 text-base">
            <RadioTowerIcon className="size-4 text-white/70" />
            {t("gateways.checkpoint.title")}
          </CardTitle>
          <CardDescription>{t("gateways.checkpoint.description")}</CardDescription>
        </div>
        <div className="w-full max-w-[220px]">
          <Label className="mb-1 block text-xs text-muted-foreground">{t("gateways.checkpoint.windowLabel")}</Label>
          <Select value={checkpointTrendWindowMinutes} onValueChange={onWindowMinutesChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("gateways.checkpoint.windowPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="15">{t("gateways.checkpoint.window.15m")}</SelectItem>
              <SelectItem value="60">{t("gateways.checkpoint.window.60m")}</SelectItem>
              <SelectItem value="240">{t("gateways.checkpoint.window.240m")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {!selectedGatewayRecord ? (
          <p className="text-sm text-muted-foreground">{t("gateways.checkpoint.selectGatewayHint")}</p>
        ) : checkpointSummaryLoading ? (
          <p className="text-sm text-muted-foreground">{t("gateways.checkpoint.loading")}</p>
        ) : checkpointSummaryError ? (
          <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {checkpointSummaryError}
          </div>
        ) : (
          <>
            <div className="grid gap-3 lg:grid-cols-[1.2fr_0.8fr]">
              <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-black/25 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="mp-page-eyebrow">{t("gateways.checkpoint.heartbeatTitle")}</p>
                    <p className="mt-2 text-2xl font-semibold tracking-[-0.03em]">
                      {t("gateways.checkpoint.reportUnit", { count: trend?.report_total ?? 0 })}
                    </p>
                    <p className="mp-kpi-note mt-1">
                      {selectedGatewayRecord.id}
                    </p>
                  </div>
                  <Badge variant={checkpointTrendDirectionVariant(trend?.direction ?? "flat")} className="capitalize">
                    {checkpointTrendDirectionLabel(trend?.direction ?? "flat", t)}
                  </Badge>
                </div>
                <div className="mt-5 grid gap-3 sm:grid-cols-3">
                  <div className="rounded-xl border border-white/10 bg-white/[0.035] p-3">
                    <p className="mp-kpi-note">{t("gateways.checkpoint.kpi.ackedDelta")}</p>
                    <p className="mt-1 text-lg font-semibold">{trend?.acked_delta_total ?? 0}</p>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.035] p-3">
                    <p className="mp-kpi-note">{t("gateways.checkpoint.kpi.queueTotal")}</p>
                    <p className="mt-1 text-lg font-semibold">{trend?.queue_total ?? 0}</p>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.035] p-3">
                    <p className="mp-kpi-note">{t("gateways.checkpoint.lag")}</p>
                    <p className="mt-1 text-lg font-semibold">{lagTotal}</p>
                  </div>
                </div>
                <div className="mt-4 mp-heartline" />
              </div>

              <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
                <div className="mp-metric-card">
                  <div className="relative z-10 flex items-center justify-between gap-3">
                    <div>
                      <p className="mp-kpi-note">{t("gateways.checkpoint.kpi.reportTotal")}</p>
                      <p className="text-2xl font-semibold">{trend?.report_total ?? 0}</p>
                    </div>
                    <ActivityIcon className="size-5 text-emerald-300" />
                  </div>
                </div>
                <div className="mp-metric-card">
                  <div className="relative z-10 flex items-center justify-between gap-3">
                    <div>
                      <p className="mp-kpi-note">{t("gateways.checkpoint.kpi.ackedDelta")}</p>
                      <p className="text-2xl font-semibold">{trend?.acked_delta_total ?? 0}</p>
                    </div>
                    <GaugeIcon className="size-5 text-white/70" />
                  </div>
                </div>
                <div className="mp-metric-card">
                  <div className="relative z-10 flex items-center justify-between gap-3">
                    <div>
                      <p className="mp-kpi-note">{t("gateways.checkpoint.kpi.queueTotal")}</p>
                      <p className="text-2xl font-semibold">{trend?.queue_total ?? 0}</p>
                    </div>
                    <RouteIcon className="size-5 text-white/70" />
                  </div>
                </div>
              </div>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("gateways.checkpoint.table.queue")}</TableHead>
                  <TableHead>{t("gateways.checkpoint.lag")}</TableHead>
                  <TableHead>{t("gateways.checkpoint.table.acked")}</TableHead>
                  <TableHead>{t("gateways.checkpoint.table.windowReport")}</TableHead>
                  <TableHead>{t("gateways.checkpoint.table.windowDirection")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {checkpointSummary?.items.length ? null : (
                  <TableRow>
                    <TableCell colSpan={5} className="py-4 text-center text-muted-foreground">
                      {t("gateways.checkpoint.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {(checkpointSummary?.items ?? []).slice(0, 5).map((item) => (
                  <TableRow key={`${item.gateway_id}-${item.queue}`}>
                    <TableCell className="font-medium">
                      <TableCellText className="max-w-[12rem]">{item.queue}</TableCellText>
                    </TableCell>
                    <TableCell>{item.lag_count}</TableCell>
                    <TableCell>{item.acked_count}</TableCell>
                    <TableCell>{item.time_window_trend.report_total}</TableCell>
                    <TableCell>
                      <Badge variant={checkpointTrendDirectionVariant(item.time_window_trend.direction)}>
                        {checkpointTrendDirectionLabel(item.time_window_trend.direction, t)}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </>
        )}
      </CardContent>
    </Card>
  )
}
