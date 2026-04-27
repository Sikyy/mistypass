import { type ComponentProps } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { type WalletPassDeliveryNotification, type WalletPassInstance, type WalletPassTemplate } from "@/lib/api"

type ReceiptRecoveryStatus = "pending" | "attention" | "ready"
type BadgeVariant = ComponentProps<typeof Badge>["variant"]

type WalletDeliveryReceiptsCardProps = {
  writable: boolean
  loading: boolean
  refreshing: boolean
  batchRetryingDelivery: boolean
  repairingRetryablePasses: boolean
  retryingDeliveryNotificationID: string
  receiptRecoveryFlowStatus: ReceiptRecoveryStatus
  receiptSplitStatus: ReceiptRecoveryStatus
  receiptRemediationStatus: ReceiptRecoveryStatus
  receiptReviewStatus: ReceiptRecoveryStatus
  failedDeliveryNotificationsCount: number
  retryableDeliveryNotificationsCount: number
  nonRetryableFailedDeliveryNotificationsCount: number
  batchRetryableDeliveryNotificationsCount: number
  repairableRetryableDeliveryPassesCount: number
  reissueTargetIDsByRetryableDeliveryCount: number
  recentDeliveryNotifications: WalletPassDeliveryNotification[]
  deliveryRetryQuery: string
  enterpriseReceiptRecoveryReviewLink: string
  enterpriseAlertsIssueLink: string
  hasWorkerAlertFlowHints: boolean
  enterpriseSyncWorkerReviewLink: string
  passByID: Map<string, WalletPassInstance>
  templateByID: Map<string, WalletPassTemplate>
  receiptRecoveryStatusVariant: (status: ReceiptRecoveryStatus) => BadgeVariant
  receiptRecoveryStatusLabel: (status: ReceiptRecoveryStatus) => string
  deliveryNotificationStatusVariant: (status: string) => BadgeVariant
  deliveryNotificationStatusLabel: (status: string) => string
  formatDateTime: (value?: string) => string
  onRetryBatch: () => void
  onRepairBatch: () => void
  onSeedBatchReissue: () => void
  onOpenPassQrDialog: (pass: WalletPassInstance) => unknown
  onCopySaveLink: (pass: WalletPassInstance) => unknown
  onRetryDeliveryNotification: (notificationID: string) => unknown
}

export function WalletDeliveryReceiptsCard({
  writable,
  loading,
  refreshing,
  batchRetryingDelivery,
  repairingRetryablePasses,
  retryingDeliveryNotificationID,
  receiptRecoveryFlowStatus,
  receiptSplitStatus,
  receiptRemediationStatus,
  receiptReviewStatus,
  failedDeliveryNotificationsCount,
  retryableDeliveryNotificationsCount,
  nonRetryableFailedDeliveryNotificationsCount,
  batchRetryableDeliveryNotificationsCount,
  repairableRetryableDeliveryPassesCount,
  reissueTargetIDsByRetryableDeliveryCount,
  recentDeliveryNotifications,
  deliveryRetryQuery,
  enterpriseReceiptRecoveryReviewLink,
  enterpriseAlertsIssueLink,
  hasWorkerAlertFlowHints,
  enterpriseSyncWorkerReviewLink,
  passByID,
  templateByID,
  receiptRecoveryStatusVariant,
  receiptRecoveryStatusLabel,
  deliveryNotificationStatusVariant,
  deliveryNotificationStatusLabel,
  formatDateTime,
  onRetryBatch,
  onRepairBatch,
  onSeedBatchReissue,
  onOpenPassQrDialog,
  onCopySaveLink,
  onRetryDeliveryNotification,
}: WalletDeliveryReceiptsCardProps) {
  const { t } = useTranslation()
  const batchActionDisabled =
    !writable || batchRetryingDelivery || repairingRetryablePasses || loading || refreshing
  const batchActionDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : batchRetryingDelivery || repairingRetryablePasses
      ? t("walletPage.disabledReasons.busy")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : ""
  const retryBatchDisabledReason =
    batchActionDisabledReason ||
    (batchRetryableDeliveryNotificationsCount === 0 ? t("walletPage.disabledReasons.noRetryableDeliveries") : "")
  const repairBatchDisabledReason =
    batchActionDisabledReason ||
    (repairableRetryableDeliveryPassesCount === 0 ? t("walletPage.disabledReasons.noRepairablePasses") : "")
  const reissueDraftDisabledReason =
    batchActionDisabledReason ||
    (reissueTargetIDsByRetryableDeliveryCount === 0 ? t("walletPage.disabledReasons.noReissueTargets") : "")

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.cards.recentDeliveryReceipts.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.cards.recentDeliveryReceipts.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="rounded-xl border bg-muted/10 px-4 py-3 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium">{t("walletPage.cards.recentDeliveryReceipts.recoveryStatusTitle")}</p>
            <Badge variant={receiptRecoveryStatusVariant(receiptRecoveryFlowStatus)}>
              {receiptRecoveryStatusLabel(receiptRecoveryFlowStatus)}
            </Badge>
          </div>
          <div className="grid gap-2 md:grid-cols-3">
            <div className="rounded-lg border bg-background px-3 py-2">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-medium">{t("walletPage.cards.recentDeliveryReceipts.step1Title")}</p>
                <Badge variant={receiptRecoveryStatusVariant(receiptSplitStatus)}>
                  {receiptRecoveryStatusLabel(receiptSplitStatus)}
                </Badge>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("walletPage.cards.recentDeliveryReceipts.step1Description", {
                  failedCount: failedDeliveryNotificationsCount,
                  retryableCount: retryableDeliveryNotificationsCount,
                  nonRetryableCount: nonRetryableFailedDeliveryNotificationsCount,
                })}
              </p>
            </div>
            <div className="rounded-lg border bg-background px-3 py-2">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-medium">{t("walletPage.cards.recentDeliveryReceipts.step2Title")}</p>
                <Badge variant={receiptRecoveryStatusVariant(receiptRemediationStatus)}>
                  {receiptRecoveryStatusLabel(receiptRemediationStatus)}
                </Badge>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("walletPage.cards.recentDeliveryReceipts.step2Description", {
                  retryableCount: batchRetryableDeliveryNotificationsCount,
                  repairableCount: repairableRetryableDeliveryPassesCount,
                  reissueCount: reissueTargetIDsByRetryableDeliveryCount,
                })}
              </p>
            </div>
            <div className="rounded-lg border bg-background px-3 py-2">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-medium">{t("walletPage.cards.recentDeliveryReceipts.step3Title")}</p>
                <Badge variant={receiptRecoveryStatusVariant(receiptReviewStatus)}>
                  {receiptRecoveryStatusLabel(receiptReviewStatus)}
                </Badge>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("walletPage.cards.recentDeliveryReceipts.step3Description")}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={batchActionDisabled || batchRetryableDeliveryNotificationsCount === 0}
              title={retryBatchDisabledReason || undefined}
              onClick={onRetryBatch}
            >
              {batchRetryingDelivery
                ? t("walletPage.actions.batchRetrying")
                : t("walletPage.actions.batchRetryDelivery", { count: batchRetryableDeliveryNotificationsCount })}
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={batchActionDisabled || repairableRetryableDeliveryPassesCount === 0}
              title={repairBatchDisabledReason || undefined}
              onClick={onRepairBatch}
            >
              {repairingRetryablePasses
                ? t("walletPage.actions.repairingStatus")
                : t("walletPage.actions.batchRepairStatus", { count: repairableRetryableDeliveryPassesCount })}
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={enterpriseReceiptRecoveryReviewLink}>{t("walletPage.actions.backToEnterpriseReceiptReview")}</Link>
            </Button>
            {batchActionDisabledReason ? (
              <p className="w-full basis-full text-xs text-muted-foreground">{batchActionDisabledReason}</p>
            ) : null}
          </div>
        </div>

        <div className="rounded-xl border bg-muted/10 px-4 py-3">
          <p className="mp-kpi-note">
            {deliveryRetryQuery.trim()
              ? t("walletPage.cards.recentDeliveryReceipts.matchRetryableByHint", {
                  hint: deliveryRetryQuery.trim(),
                  count: retryableDeliveryNotificationsCount,
                })
              : t("walletPage.cards.recentDeliveryReceipts.retryableCount", {
                  count: retryableDeliveryNotificationsCount,
                })}
            {retryableDeliveryNotificationsCount > batchRetryableDeliveryNotificationsCount
              ? t("walletPage.cards.recentDeliveryReceipts.batchLimit", {
                  count: batchRetryableDeliveryNotificationsCount,
                })
              : ""}
            {t("walletPage.cards.recentDeliveryReceipts.repairableAndReissueCount", {
              repairableCount: repairableRetryableDeliveryPassesCount,
              reissueCount: reissueTargetIDsByRetryableDeliveryCount,
            })}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={batchActionDisabled || batchRetryableDeliveryNotificationsCount === 0}
              title={retryBatchDisabledReason || undefined}
              onClick={onRetryBatch}
            >
              {batchRetryingDelivery
                ? t("walletPage.actions.batchRetrying")
                : t("walletPage.actions.batchRetryDelivery", { count: batchRetryableDeliveryNotificationsCount })}
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={batchActionDisabled || repairableRetryableDeliveryPassesCount === 0}
              title={repairBatchDisabledReason || undefined}
              onClick={onRepairBatch}
            >
              {repairingRetryablePasses
                ? t("walletPage.actions.repairingStatus")
                : t("walletPage.actions.batchRepairStatus", { count: repairableRetryableDeliveryPassesCount })}
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={batchActionDisabled || reissueTargetIDsByRetryableDeliveryCount === 0}
              title={reissueDraftDisabledReason || undefined}
              onClick={onSeedBatchReissue}
            >
              {t("walletPage.actions.seedBatchReissueDraft", { count: reissueTargetIDsByRetryableDeliveryCount })}
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={enterpriseAlertsIssueLink}>{t("walletPage.actions.backToEnterpriseSyncIssue")}</Link>
            </Button>
            {hasWorkerAlertFlowHints ? (
              <Button asChild size="sm" variant="outline">
                <Link to={enterpriseSyncWorkerReviewLink}>{t("walletPage.actions.backToEnterpriseWorkerReview")}</Link>
              </Button>
            ) : null}
            {!batchActionDisabledReason && (retryBatchDisabledReason || repairBatchDisabledReason || reissueDraftDisabledReason) ? (
              <p className="w-full basis-full text-xs text-muted-foreground">
                {retryBatchDisabledReason || repairBatchDisabledReason || reissueDraftDisabledReason}
              </p>
            ) : null}
          </div>
        </div>

        {recentDeliveryNotifications.length === 0 ? (
          <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
            {t("walletPage.cards.recentDeliveryReceipts.empty")}
          </div>
        ) : (
          recentDeliveryNotifications.map((item) => {
            const itemPass = passByID.get(item.pass_id)
            const itemTemplate = templateByID.get(item.template_id)
            return (
              <div
                key={item.id}
                className="rounded-xl border bg-card/80 px-4 py-3"
              >
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-medium">{item.target_id}</p>
                      <Badge variant={deliveryNotificationStatusVariant(item.status)}>
                        {deliveryNotificationStatusLabel(item.status)}
                      </Badge>
                      <Badge variant="outline">attempt {item.attempt ?? 1}</Badge>
                      {item.reason ? <Badge variant="outline">{item.reason}</Badge> : null}
                    </div>
                    <p className="text-sm text-muted-foreground">{itemTemplate?.name ?? item.template_id}</p>
                    <p className="mp-kpi-note">
                      {formatDateTime(item.triggered_at)}
                      {item.source_notification_id
                        ? t("walletPage.cards.recentDeliveryReceipts.retryFromSource", {
                            sourceID: item.source_notification_id,
                          })
                        : ""}
                    </p>
                    {item.channel_results && item.channel_results.length > 0 ? (
                      <div className="flex flex-col gap-1 pt-1 text-xs text-muted-foreground">
                        {item.channel_results.map((result) => (
                          <p key={`${item.id}-${result.channel}`}>
                            {result.channel} · {result.status}
                            {result.reason ? ` (${result.reason})` : ""}
                            {result.receivers && result.receivers.length > 0 ? ` · ${result.receivers.join(", ")}` : ""}
                          </p>
                        ))}
                      </div>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {itemPass?.save_link ? (
                      <>
                        <Button size="sm" variant="outline" onClick={() => void onOpenPassQrDialog(itemPass)}>
                          {t("walletPage.actions.viewQrCode")}
                        </Button>
                        <Button size="sm" variant="outline" onClick={() => void onCopySaveLink(itemPass)}>
                          {t("walletPage.actions.copyLink")}
                        </Button>
                      </>
                    ) : null}
                    {item.retryable ? (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!writable || retryingDeliveryNotificationID === item.id}
                        title={!writable ? t("walletPage.disabledReasons.readOnly") : retryingDeliveryNotificationID === item.id ? t("walletPage.disabledReasons.busy") : undefined}
                        onClick={() => void onRetryDeliveryNotification(item.id)}
                      >
                        {retryingDeliveryNotificationID === item.id
                          ? t("walletPage.actions.retrying")
                          : t("walletPage.actions.retryFailedDeliveryChannel")}
                      </Button>
                    ) : null}
                    {!writable ? <span className="mp-kpi-note">{t("walletPage.hints.readOnlyBoundaryOnly")}</span> : null}
                  </div>
                </div>
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}
