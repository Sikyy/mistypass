import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type WalletJobAlertNotification } from "@/lib/api"

type WalletAlertNotificationRecordsCardProps = {
  loading: boolean
  refreshing: boolean
  writable: boolean
  alertNotifications: WalletJobAlertNotification[]
  retryingAlertNotificationID: string
  onRetryAlertNotification: (notificationID: string) => void
  formatDateTime: (value?: string) => string
}

export function WalletAlertNotificationRecordsCard({
  loading,
  refreshing,
  writable,
  alertNotifications,
  retryingAlertNotificationID,
  onRetryAlertNotification,
  formatDateTime,
}: WalletAlertNotificationRecordsCardProps) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.alertNotificationRecords.title", { defaultValue: "Alert notification records" })}</CardTitle>
        <CardDescription>
          {t("walletPage.components.alertNotificationRecords.description", {
            defaultValue: "Recent {{count}} delivery results (including cooldown skips).",
            count: alertNotifications.length,
          })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.time", { defaultValue: "Time" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.errorCode", { defaultValue: "Error code" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.countThreshold", { defaultValue: "Count / threshold" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.attempt", { defaultValue: "Attempt" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.channels", { defaultValue: "Channels" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.receiverGroups", { defaultValue: "Receiver groups" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.status", { defaultValue: "Status" })}</TableHead>
              <TableHead>{t("walletPage.components.alertNotificationRecords.columns.actions", { defaultValue: "Actions" })}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={8} className="py-10 text-center text-muted-foreground">
                  {t("walletPage.components.alertNotificationRecords.loading", { defaultValue: "Loading notification records..." })}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading && alertNotifications.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  {t("walletPage.components.alertNotificationRecords.empty", {
                    defaultValue: 'No records yet. Click "Evaluate and notify now" to create records.',
                  })}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading &&
              alertNotifications.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="mp-kpi-note">{formatDateTime(item.triggered_at)}</TableCell>
                  <TableCell>{item.error_code || "-"}</TableCell>
                  <TableCell>
                    {item.count} / {item.threshold}
                  </TableCell>
                  <TableCell>{item.attempt ?? "-"}</TableCell>
                  <TableCell className="mp-kpi-note">
                    {item.channels && item.channels.length > 0 ? item.channels.join(", ") : "-"}
                  </TableCell>
                  <TableCell className="mp-kpi-note">
                    {item.receiver_groups && item.receiver_groups.length > 0
                      ? item.receiver_groups.join(", ")
                      : "-"}
                  </TableCell>
                  <TableCell>
                    <div className="space-y-1">
                      <Badge variant={item.status === "sent" ? "secondary" : "outline"}>
                        {item.status}
                        {item.reason ? ` (${item.reason})` : ""}
                      </Badge>
                      {item.channel_results && item.channel_results.length > 0 ? (
                        <p className="mp-kpi-note">
                          {item.channel_results
                            .map((result) =>
                              result.reason
                                ? `${result.channel}:${result.status}(${result.reason})`
                                : `${result.channel}:${result.status}`
                            )
                            .join(" | ")}
                        </p>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    {item.status === "failed" && item.retryable ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => onRetryAlertNotification(item.id)}
                        disabled={retryingAlertNotificationID === item.id || loading || refreshing || !writable}
                      >
                        {retryingAlertNotificationID === item.id
                          ? t("walletPage.components.alertNotificationRecords.retrying", { defaultValue: "Retrying..." })
                          : t("walletPage.components.alertNotificationRecords.retry", { defaultValue: "Retry" })}
                      </Button>
                    ) : (
                      <span className="mp-kpi-note">-</span>
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
