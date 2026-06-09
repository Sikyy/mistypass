import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { CurrentUser } from "@/lib/api"
import type { RolloutActionName } from "@/lib/api/ota"
import { useRolloutActionMutation, useRolloutDetail } from "../hooks/use-rollouts"
import { availableRolloutActions, formatSchedule, rolloutStateBadgeVariant, targetSummary } from "../lib/rollout-utils"

const WRITE_ROLES: CurrentUser["role"][] = ["super_admin", "tenant_admin", "building_admin"]

export function RolloutDetail({ token, tenantID, viewer, id }: { token: string | undefined; tenantID: string | undefined; viewer: CurrentUser; id: string }) {
  const { t, i18n } = useTranslation()
  const query = useRolloutDetail(token, tenantID, id)
  const action = useRolloutActionMutation(token, tenantID, id)
  const [confirmAbort, setConfirmAbort] = useState(false)
  const canWrite = WRITE_ROLES.includes(viewer.role)

  if (query.isPending) return <p className="text-sm text-muted-foreground">{t("ota.rollout.detail.loading")}</p>
  if (query.error || !query.data) return <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.rollout.errors.generic")}</p>
  const { rollout, gateways } = query.data
  const actions = canWrite ? availableRolloutActions(rollout.state) : []
  const run = (a: RolloutActionName) => action.mutate(a)

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader><CardTitle className="flex items-center gap-3">{t("ota.rollout.detail.title")} <Badge variant={rolloutStateBadgeVariant(rollout.state)}>{t(`ota.rollout.state.${rollout.state}`)}</Badge></CardTitle></CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>{t("ota.rollout.detail.firmware")}: {rollout.firmware_version}</p>
          <p>{t("ota.rollout.detail.target")}: {targetSummary(rollout.target, t)}</p>
          <p>{t("ota.rollout.detail.phase")}: {rollout.current_phase + 1}/{rollout.phases.length} ({rollout.phases.map((p) => `${p.percentage}%${p.requires_approval ? "*" : ""}`).join(" → ")})</p>
          <p>{t("ota.rollout.detail.threshold")}: {rollout.failure_threshold_pct}%</p>
          {rollout.schedule && <p>{t("ota.rollout.detail.schedule")}: {formatSchedule(rollout.schedule, i18n.language)}</p>}
          <div className="flex gap-2 pt-2">
            {actions.map((a) => a === "abort" ? (
              <Button key={a} variant="destructive" size="sm" disabled={action.isPending} onClick={() => setConfirmAbort(true)}>{t("ota.rollout.detail.actions.abort")}</Button>
            ) : (
              <Button key={a} variant="outline" size="sm" disabled={action.isPending} onClick={() => run(a)}>{t(`ota.rollout.detail.actions.${a}`)}</Button>
            ))}
          </div>
          {action.error && <p className="text-sm text-destructive">{action.error instanceof Error ? action.error.message : t("ota.rollout.errors.generic")}</p>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>{t("ota.rollout.detail.gatewaysTitle")}</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader><TableRow>
              <TableHead>{t("ota.rollout.detail.colGateway")}</TableHead>
              <TableHead>{t("ota.rollout.detail.colPhase")}</TableHead>
              <TableHead>{t("ota.rollout.detail.colStatus")}</TableHead>
              <TableHead>{t("ota.rollout.detail.colVersion")}</TableHead>
            </TableRow></TableHeader>
            <TableBody>
              {gateways.map((g) => (
                <TableRow key={g.gateway_id}>
                  <TableCell>{g.gateway_id}</TableCell>
                  <TableCell>{g.phase < 0 ? "—" : g.phase + 1}</TableCell>
                  <TableCell><Badge variant="outline">{g.ota_status}</Badge></TableCell>
                  <TableCell>{g.current_firmware_version ?? "—"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={confirmAbort} onOpenChange={setConfirmAbort}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t("ota.rollout.detail.confirmAbortTitle")}</DialogTitle><DialogDescription>{t("ota.rollout.detail.confirmAbortBody")}</DialogDescription></DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmAbort(false)}>{t("ota.rollout.detail.cancel")}</Button>
            <Button variant="destructive" onClick={() => { setConfirmAbort(false); run("abort") }}>{t("ota.rollout.detail.actions.abort")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <p className="text-xs text-muted-foreground">{new Date(rollout.updated_at).toLocaleString(i18n.language)}</p>
    </div>
  )
}
