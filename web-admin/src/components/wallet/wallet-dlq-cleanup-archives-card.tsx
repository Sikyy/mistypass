import { AlertTriangleIcon, Trash2Icon } from "lucide-react"
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
import { type WalletDLQCleanupArchive } from "@/lib/api"

type WalletDlqCleanupArchivesCardProps = {
  loading: boolean
  archives: WalletDLQCleanupArchive[]
  formatDateTime: (value?: string) => string
  formatDurationSeconds: (seconds?: number) => string
}

export function WalletDlqCleanupArchivesCard({
  loading,
  archives,
  formatDateTime,
  formatDurationSeconds,
}: WalletDlqCleanupArchivesCardProps) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.dlqArchives.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.components.dlqArchives.description", {
            count: archives.length,
          })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("walletPage.components.dlqArchives.columns.time")}</TableHead>
              <TableHead>{t("walletPage.components.dlqArchives.columns.actor")}</TableHead>
              <TableHead>{t("walletPage.components.dlqArchives.columns.filters")}</TableHead>
              <TableHead>{t("walletPage.components.dlqArchives.columns.result")}</TableHead>
              <TableHead>{t("walletPage.components.dlqArchives.columns.jobs")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={5} className="py-10 text-center text-muted-foreground">
                  {t("walletPage.components.dlqArchives.loading")}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading && archives.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  {t("walletPage.components.dlqArchives.empty")}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading &&
              archives.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="mp-kpi-note">{formatDateTime(item.at)}</TableCell>
                  <TableCell>{item.actor || "-"}</TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1 text-xs">
                      <span>error_code: {item.error_code || "*"}</span>
                      <span>older_than: {formatDurationSeconds(item.older_than_seconds)}</span>
                      <span>limit: {item.limit}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Badge variant={item.removed > 0 ? "secondary" : "outline"}>
                        <Trash2Icon className="mr-1 size-3" />
                        removed {item.removed}
                      </Badge>
                      <Badge variant="outline">remaining {item.remaining_dlq}</Badge>
                    </div>
                  </TableCell>
                  <TableCell className="mp-kpi-note">
                    {item.processed_jobs && item.processed_jobs.length > 0 ? (
                      item.processed_jobs.slice(0, 3).join(", ")
                    ) : (
                      <span className="inline-flex items-center gap-1">
                        <AlertTriangleIcon className="size-3" />
                        {t("walletPage.components.dlqArchives.none")}
                      </span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
