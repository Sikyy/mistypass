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

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.alertSubscription.title", { defaultValue: "Issuance alert subscription" })}</CardTitle>
        <CardDescription>
          {t("walletPage.components.alertSubscription.description", {
            defaultValue:
              "Issuance alert subscription config for current organization. Platform admins can switch tenants; tenant admins can maintain local policy directly.",
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid gap-3 md:grid-cols-3">
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
            <div className="space-y-0.5">
              <p className="text-sm font-medium">{t("walletPage.components.alertSubscription.enabled", { defaultValue: "Enable subscription" })}</p>
              <p className="mp-kpi-note">{t("walletPage.components.alertSubscription.enabledHint", { defaultValue: "If disabled, only metrics dashboard remains." })}</p>
            </div>
            <Switch checked={subscriptionEnabled} disabled={!writable} onCheckedChange={onSubscriptionEnabledChange} />
          </div>
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
            <div className="space-y-0.5">
              <p className="inline-flex items-center gap-1 text-sm font-medium">
                <MailIcon className="size-3.5" />
                Email
              </p>
              <p className="mp-kpi-note">{t("walletPage.components.alertSubscription.emailHint", { defaultValue: "Email notification channel." })}</p>
            </div>
            <Switch
              checked={subscriptionEmailEnabled}
              disabled={!writable}
              onCheckedChange={onSubscriptionEmailEnabledChange}
            />
          </div>
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
            <div className="space-y-0.5">
              <p className="inline-flex items-center gap-1 text-sm font-medium">
                <MessageCircleIcon className="size-3.5" />
                WhatsApp
              </p>
              <p className="mp-kpi-note">{t("walletPage.components.alertSubscription.whatsappHint", { defaultValue: "Instant notification channel." })}</p>
            </div>
            <Switch
              checked={subscriptionWhatsAppEnabled}
              disabled={!writable}
              onCheckedChange={onSubscriptionWhatsAppEnabledChange}
            />
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <Input
            value={subscriptionThreshold}
            disabled={!writable}
            onChange={(event) => onSubscriptionThresholdChange(event.target.value)}
            placeholder="dlq_alert_threshold"
          />
          <Input
            value={subscriptionWindowSeconds}
            disabled={!writable}
            onChange={(event) => onSubscriptionWindowSecondsChange(event.target.value)}
            placeholder="window_seconds"
          />
          <Input
            value={subscriptionCooldownSeconds}
            disabled={!writable}
            onChange={(event) => onSubscriptionCooldownSecondsChange(event.target.value)}
            placeholder="cooldown_seconds"
          />
          <Input
            value={subscriptionReceiverGroups}
            disabled={!writable}
            onChange={(event) => onSubscriptionReceiverGroupsChange(event.target.value)}
            placeholder={t("walletPage.components.alertSubscription.receiverGroups", {
              defaultValue: "receiver_groups (comma-separated)",
            })}
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <p className="mp-kpi-note">
              {t("walletPage.components.alertSubscription.lastUpdated", {
                defaultValue: "Last updated: {{time}}",
                time: formatDateTime(subscription?.updated_at),
              })}
            </p>
            {!writable ? (
              <p className="mp-kpi-note">
                {t("walletPage.components.alertSubscription.readOnlyHint", {
                  defaultValue: "Current role is read-only. You can view issuance status but cannot modify subscription policy.",
                })}
                {readOnlyBoundaryHint}
              </p>
            ) : null}
            {dispatchSummary ? <p className="text-xs text-emerald-700">{dispatchSummary}</p> : null}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              onClick={onDispatchAlertsNow}
              disabled={loading || refreshing || dispatchingAlerts || !writable}
            >
              {dispatchingAlerts
                ? t("walletPage.components.alertSubscription.dispatching", { defaultValue: "Sending..." })
                : t("walletPage.components.alertSubscription.dispatchNow", { defaultValue: "Evaluate and notify now" })}
            </Button>
            <Button
              onClick={onSaveAlertSubscription}
              disabled={loading || refreshing || savingSubscription || !writable}
            >
              {savingSubscription
                ? t("walletPage.components.alertSubscription.saving", { defaultValue: "Saving..." })
                : t("walletPage.components.alertSubscription.savePolicy", { defaultValue: "Save subscription policy" })}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
