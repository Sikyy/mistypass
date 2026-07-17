import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useRolloutList } from "../hooks/use-rollouts"
import { rolloutStateBadgeVariant, targetSummary } from "../lib/rollout-utils"

export function RolloutListCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const query = useRolloutList(token, tenantID)
  const items = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.rollout.list.title")}</CardTitle>
        <CardDescription>{t("ota.rollout.list.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <p className="text-sm text-muted-foreground">{t("ota.rollout.list.loading")}</p>
        ) : query.error ? (
          <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.rollout.errors.generic")}</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("ota.rollout.list.empty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("ota.rollout.list.colFirmware")}</TableHead>
                <TableHead>{t("ota.rollout.list.colTarget")}</TableHead>
                <TableHead>{t("ota.rollout.list.colState")}</TableHead>
                <TableHead>{t("ota.rollout.list.colPhase")}</TableHead>
                <TableHead>{t("ota.rollout.list.colCreated")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((r) => (
                <TableRow key={r.id} className="cursor-pointer" onClick={() => navigate(`/ota/rollouts/${r.id}`)}>
                  <TableCell>{r.firmware_version}</TableCell>
                  <TableCell>{targetSummary(r.target, t)}</TableCell>
                  <TableCell><Badge variant={rolloutStateBadgeVariant(r.state)}>{t(`ota.rollout.state.${r.state}`)}</Badge></TableCell>
                  <TableCell>{r.current_phase + 1}/{r.phases.length}</TableCell>
                  <TableCell>{new Date(r.created_at).toLocaleString(i18n.language)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
