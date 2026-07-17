import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useFirmwareSummary } from "../hooks/use-firmware"

export function FirmwareSummaryCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const query = useFirmwareSummary(token, tenantID)
  const summary = query.data
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.firmware.summary.title")}</CardTitle>
        <CardDescription>{t("ota.firmware.summary.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.summary.loading")}</p>
        ) : query.error ? (
          <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.firmware.errors.generic")}</p>
        ) : !summary || summary.total === 0 ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.summary.empty")}</p>
        ) : (
          <div className="space-y-3">
            <p className="text-sm">{t("ota.firmware.summary.fleet", { total: summary.total, reported: summary.reported })}</p>
            <div className="flex flex-wrap gap-2">
              {summary.versions.map((v) => (
                <Badge key={v.version} variant="outline">{v.version} · {v.count}</Badge>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
