import { MailIcon, MessageCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { type WalletJobAlertSubscription } from "@/lib/api"

type WalletAlertSubscriptionCardProps = {
  writable: boolean
  readOnlyBoundaryHint: string
  subscriptionEnabled: boolean
  onSubscriptionEnabledChange: (value: boolean) => void
  subscriptionEmailEnabled: boolean
  onSubscriptionEmailEnabledChange: (value: boolean) => void
  subscriptionWhatsAppEnabled: boolean
  onSubscriptionWhatsAppEnabledChange: (value: boolean) => void
  subscriptionThreshold: string
  onSubscriptionThresholdChange: (value: string) => void
  subscriptionWindowSeconds: string
  onSubscriptionWindowSecondsChange: (value: string) => void
  subscriptionCooldownSeconds: string
  onSubscriptionCooldownSecondsChange: (value: string) => void
  subscriptionReceiverGroups: string
  onSubscriptionReceiverGroupsChange: (value: string) => void
  subscription: WalletJobAlertSubscription | null
  formatDateTime: (value?: string) => string
  dispatchSummary: string
  loading: boolean
  refreshing: boolean
  dispatchingAlerts: boolean
  savingSubscription: boolean
  onDispatchAlertsNow: () => void
  onSaveAlertSubscription: () => void
}

export function WalletAlertSubscriptionCard({
  writable,
  readOnlyBoundaryHint,
  subscriptionEnabled,
  onSubscriptionEnabledChange,
  subscriptionEmailEnabled,
  onSubscriptionEmailEnabledChange,
  subscriptionWhatsAppEnabled,
  onSubscriptionWhatsAppEnabledChange,
  subscriptionThreshold,
  onSubscriptionThresholdChange,
  subscriptionWindowSeconds,
  onSubscriptionWindowSecondsChange,
  subscriptionCooldownSeconds,
  onSubscriptionCooldownSecondsChange,
  subscriptionReceiverGroups,
  onSubscriptionReceiverGroupsChange,
  subscription,
  formatDateTime,
  dispatchSummary,
  loading,
  refreshing,
  dispatchingAlerts,
  savingSubscription,
  onDispatchAlertsNow,
  onSaveAlertSubscription,
}: WalletAlertSubscriptionCardProps) {
  const { t } = useTranslation()
  const readOnlyDisabledReason = !writable ? t("walletPage.disabledReasons.readOnly") : undefined
  const dispatchDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : dispatchingAlerts
      ? t("walletPage.disabledReasons.busy")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : undefined
  const saveDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : savingSubscription
      ? t("walletPage.disabledReasons.busy")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : undefined

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.alertSubscription.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.components.alertSubscription.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid gap-3 md:grid-cols-3">
          <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2">
            <div className="min-w-0 space-y-0.5">
              <p className="text-sm font-medium">{t("walletPage.components.alertSubscription.enabled")}</p>
              <p className="mp-kpi-note">{t("walletPage.components.alertSubscription.enabledHint")}</p>
            </div>
            <Switch
              checked={subscriptionEnabled}
              disabled={!writable}
              title={readOnlyDisabledReason}
              onCheckedChange={onSubscriptionEnabledChange}
            />
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2">
            <div className="min-w-0 space-y-0.5">
              <p className="inline-flex items-center gap-1 text-sm font-medium">
                <MailIcon className="size-3.5" />
                Email
              </p>
              <p className="mp-kpi-note">{t("walletPage.components.alertSubscription.emailHint")}</p>
            </div>
            <Switch
              checked={subscriptionEmailEnabled}
              disabled={!writable}
              title={readOnlyDisabledReason}
              onCheckedChange={onSubscriptionEmailEnabledChange}
            />
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2">
            <div className="min-w-0 space-y-0.5">
              <p className="inline-flex items-center gap-1 text-sm font-medium">
                <MessageCircleIcon className="size-3.5" />
                WhatsApp
              </p>
              <p className="mp-kpi-note">{t("walletPage.components.alertSubscription.whatsappHint")}</p>
            </div>
            <Switch
              checked={subscriptionWhatsAppEnabled}
              disabled={!writable}
              title={readOnlyDisabledReason}
              onCheckedChange={onSubscriptionWhatsAppEnabledChange}
            />
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <Input
            value={subscriptionThreshold}
            disabled={!writable}
            title={readOnlyDisabledReason}
            onChange={(event) => onSubscriptionThresholdChange(event.target.value)}
            placeholder={t("walletPage.components.alertSubscription.thresholdPlaceholder")}
          />
          <Input
            value={subscriptionWindowSeconds}
            disabled={!writable}
            title={readOnlyDisabledReason}
            onChange={(event) => onSubscriptionWindowSecondsChange(event.target.value)}
            placeholder={t("walletPage.components.alertSubscription.windowPlaceholder")}
          />
          <Input
            value={subscriptionCooldownSeconds}
            disabled={!writable}
            title={readOnlyDisabledReason}
            onChange={(event) => onSubscriptionCooldownSecondsChange(event.target.value)}
            placeholder={t("walletPage.components.alertSubscription.cooldownPlaceholder")}
          />
          <Input
            value={subscriptionReceiverGroups}
            disabled={!writable}
            title={readOnlyDisabledReason}
            onChange={(event) => onSubscriptionReceiverGroupsChange(event.target.value)}
            placeholder={t("walletPage.components.alertSubscription.receiverGroups")}
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <p className="mp-kpi-note">
              {t("walletPage.components.alertSubscription.lastUpdated", {
                time: formatDateTime(subscription?.updated_at),
              })}
            </p>
            {!writable ? (
              <p className="mp-kpi-note">
                {t("walletPage.components.alertSubscription.readOnlyHint")}
                {readOnlyBoundaryHint}
              </p>
            ) : null}
            {dispatchSummary ? <p className="text-xs text-emerald-700">{dispatchSummary}</p> : null}
          </div>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            <Button
              variant="outline"
              className="w-full sm:w-auto"
              onClick={onDispatchAlertsNow}
              disabled={loading || refreshing || dispatchingAlerts || !writable}
              title={dispatchDisabledReason}
            >
              {dispatchingAlerts
                ? t("walletPage.components.alertSubscription.dispatching")
                : t("walletPage.components.alertSubscription.dispatchNow")}
            </Button>
            <Button
              className="w-full sm:w-auto"
              onClick={onSaveAlertSubscription}
              disabled={loading || refreshing || savingSubscription || !writable}
              title={saveDisabledReason}
            >
              {savingSubscription
                ? t("walletPage.components.alertSubscription.saving")
                : t("walletPage.components.alertSubscription.savePolicy")}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
