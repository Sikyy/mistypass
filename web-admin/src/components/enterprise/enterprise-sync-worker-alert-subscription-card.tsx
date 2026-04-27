import { useEffect, useState } from "react"
import { MailIcon, MessageCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { type EnterpriseSyncWorkerAlertSubscription } from "@/lib/api"

export type EnterpriseSyncWorkerAlertSubscriptionSaveInput = {
  enabled: boolean
  worker_alert_threshold: number
  window_seconds: number
  cooldown_seconds: number
  channels: {
    email: boolean
    whatsapp: boolean
  }
  receiver_groups: string[]
}

type EnterpriseSyncWorkerAlertSubscriptionCardProps = {
  dispatching?: boolean
  formatDateTime: (value?: string) => string
  onDispatch?: () => Promise<void>
  onSave: (payload: EnterpriseSyncWorkerAlertSubscriptionSaveInput) => Promise<void>
  saving: boolean
  subscription: EnterpriseSyncWorkerAlertSubscription | null
  writable: boolean
}

function parsePositiveInt(raw: string): number | undefined {
  const value = Number.parseInt(raw.trim(), 10)
  if (!Number.isFinite(value) || value <= 0) {
    return undefined
  }
  return value
}

function parseNonNegativeInt(raw: string): number | undefined {
  const value = Number.parseInt(raw.trim(), 10)
  if (!Number.isFinite(value) || value < 0) {
    return undefined
  }
  return value
}

function parseReceiverGroups(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

function normalizeChannels(raw: unknown) {
  if (Array.isArray(raw)) {
    return {
      email: raw.includes("email"),
      whatsapp: raw.includes("whatsapp"),
    }
  }
  if (raw && typeof raw === "object") {
    const channels = raw as Partial<EnterpriseSyncWorkerAlertSubscription["channels"]>
    return {
      email: Boolean(channels.email),
      whatsapp: Boolean(channels.whatsapp),
    }
  }
  return {
    email: true,
    whatsapp: false,
  }
}

export function EnterpriseSyncWorkerAlertSubscriptionCard({
  dispatching,
  formatDateTime,
  onDispatch,
  onSave,
  saving,
  subscription,
  writable,
}: EnterpriseSyncWorkerAlertSubscriptionCardProps) {
  const { t } = useTranslation()
  const [enabled, setEnabled] = useState(true)
  const [emailEnabled, setEmailEnabled] = useState(true)
  const [whatsAppEnabled, setWhatsAppEnabled] = useState(false)
  const [threshold, setThreshold] = useState("")
  const [windowSeconds, setWindowSeconds] = useState("")
  const [cooldownSeconds, setCooldownSeconds] = useState("")
  const [receiverGroups, setReceiverGroups] = useState("security")
  const [draftError, setDraftError] = useState("")

  useEffect(() => {
    if (!subscription) {
      return
    }
    const channels = normalizeChannels(subscription.channels)
    setEnabled(typeof subscription.enabled === "boolean" ? subscription.enabled : true)
    setEmailEnabled(channels.email)
    setWhatsAppEnabled(channels.whatsapp)
    setThreshold(String(subscription.worker_alert_threshold ?? 3))
    setWindowSeconds(String(subscription.window_seconds ?? 900))
    setCooldownSeconds(String(subscription.cooldown_seconds ?? 900))
    setReceiverGroups((subscription.receiver_groups?.length ? subscription.receiver_groups : ["security"]).join(", "))
    setDraftError("")
  }, [subscription])

  async function handleSave() {
    const nextThreshold = parsePositiveInt(threshold)
    if (!nextThreshold) {
      setDraftError(t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.errors.invalidThreshold"))
      return
    }

    const nextWindowSeconds = parsePositiveInt(windowSeconds)
    if (!nextWindowSeconds) {
      setDraftError(t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.errors.invalidWindow"))
      return
    }

    const nextCooldownSeconds = parseNonNegativeInt(cooldownSeconds)
    if (nextCooldownSeconds === undefined) {
      setDraftError(t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.errors.invalidCooldown"))
      return
    }

    if (enabled && !emailEnabled && !whatsAppEnabled) {
      setDraftError(t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.errors.channelRequired"))
      return
    }

    setDraftError("")
    await onSave({
      enabled,
      worker_alert_threshold: nextThreshold,
      window_seconds: nextWindowSeconds,
      cooldown_seconds: nextCooldownSeconds,
      channels: {
        email: emailEnabled,
        whatsapp: whatsAppEnabled,
      },
      receiver_groups: parseReceiverGroups(receiverGroups),
    })
  }

  return (
    <Card data-testid="enterprise-alerts-worker-subscription-card">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">
          {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.title")}
        </CardTitle>
        <CardDescription>
          {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid gap-3 md:grid-cols-3">
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
            <div className="space-y-0.5">
              <p className="text-sm font-medium">
                {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.enabled")}
              </p>
              <p className="mp-kpi-note">
                {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.enabledHint")}
              </p>
            </div>
            <Switch
              checked={enabled}
              data-testid="enterprise-alerts-worker-subscription-enabled"
              disabled={!writable || saving}
              onCheckedChange={setEnabled}
            />
          </div>

          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
            <div className="space-y-0.5">
              <p className="inline-flex items-center gap-1 text-sm font-medium">
                <MailIcon className="size-3.5" />
                Email
              </p>
              <p className="mp-kpi-note">
                {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.emailHint")}
              </p>
            </div>
            <Switch
              checked={emailEnabled}
              data-testid="enterprise-alerts-worker-subscription-email"
              disabled={!writable || saving}
              onCheckedChange={setEmailEnabled}
            />
          </div>

          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
            <div className="space-y-0.5">
              <p className="inline-flex items-center gap-1 text-sm font-medium">
                <MessageCircleIcon className="size-3.5" />
                WhatsApp
              </p>
              <p className="mp-kpi-note">
                {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.whatsAppHint")}
              </p>
            </div>
            <Switch
              checked={whatsAppEnabled}
              data-testid="enterprise-alerts-worker-subscription-whatsapp"
              disabled={!writable || saving}
              onCheckedChange={setWhatsAppEnabled}
            />
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <Input
            className="h-9"
            data-testid="enterprise-alerts-worker-subscription-threshold"
            disabled={!writable || saving}
            onChange={(event) => setThreshold(event.target.value)}
            placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.thresholdPlaceholder")}
            value={threshold}
          />
          <Input
            className="h-9"
            data-testid="enterprise-alerts-worker-subscription-window"
            disabled={!writable || saving}
            onChange={(event) => setWindowSeconds(event.target.value)}
            placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.windowPlaceholder")}
            value={windowSeconds}
          />
          <Input
            className="h-9"
            data-testid="enterprise-alerts-worker-subscription-cooldown"
            disabled={!writable || saving}
            onChange={(event) => setCooldownSeconds(event.target.value)}
            placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.cooldownPlaceholder")}
            value={cooldownSeconds}
          />
          <Input
            className="h-9"
            data-testid="enterprise-alerts-worker-subscription-receiver-groups"
            disabled={!writable || saving}
            onChange={(event) => setReceiverGroups(event.target.value)}
            placeholder={t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.receiverGroupsPlaceholder")}
            value={receiverGroups}
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <p className="mp-kpi-note">
              {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.lastUpdated", {
                time: formatDateTime(subscription?.updated_at),
              })}
            </p>
            {!writable ? (
              <p className="mp-kpi-note">
                {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.readOnlyHint")}
              </p>
            ) : null}
            {writable ? (
              <p className="mp-kpi-note">
                {t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.dispatchHint")}
              </p>
            ) : null}
            {draftError ? (
              <p
                className="text-xs text-destructive"
                data-testid="enterprise-alerts-worker-subscription-validation-error"
              >
                {draftError}
              </p>
            ) : null}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              data-testid="enterprise-alerts-worker-subscription-dispatch"
              disabled={!subscription || !writable || saving || dispatching || !onDispatch}
              onClick={() => {
                void onDispatch?.()
              }}
              variant="outline"
            >
              {dispatching
                ? t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.dispatching")
                : t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.dispatch")}
            </Button>
            <Button
              data-testid="enterprise-alerts-worker-subscription-save"
              disabled={!subscription || !writable || saving || dispatching}
              onClick={() => {
                void handleSave()
              }}
            >
              {saving
                ? t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.saving")
                : t("enterpriseAlertsWorkspace.syncAndWorker.alertSubscription.save")}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
