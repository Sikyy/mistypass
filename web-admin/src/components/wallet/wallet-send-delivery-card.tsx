import { type ComponentProps } from "react"
import { Controller, type UseFormRegisterReturn, type UseFormReturn } from "react-hook-form"
import { MailIcon, MessageCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { type WalletPassInstance, type WalletPassTemplate } from "@/lib/api"

type BadgeVariant = ComponentProps<typeof Badge>["variant"]

type WalletSendDeliveryCardProps = {
  writable: boolean
  loading: boolean
  refreshing: boolean
  dispatchingDelivery: boolean
  resolvingSaveLinkPassID: string
  readOnlyBoundaryHint: string
  deliveryPassID: string
  deliveryEmailEnabled: boolean
  deliveryWhatsAppEnabled: boolean
  deliverablePasses: WalletPassInstance[]
  selectedDeliveryPass: WalletPassInstance | null
  selectedDeliveryTemplate?: WalletPassTemplate
  templateByID: Map<string, WalletPassTemplate>
  passDeliveryForm: UseFormReturn<any>
  deliveryEmailRecipientsField: UseFormRegisterReturn
  deliveryWhatsAppRecipientsField: UseFormRegisterReturn
  passDeliveryFormError: string
  onDeliveryPassIDChange: (value: string) => void
  onDeliveryEmailEnabledChange: (value: boolean) => void
  onDeliveryWhatsAppEnabledChange: (value: boolean) => void
  onDeliveryEmailRecipientsChange: (value: string) => void
  onDeliveryWhatsAppRecipientsChange: (value: string) => void
  onSubmit: (values: any) => unknown
  onOpenPassQrDialog: (pass: WalletPassInstance) => unknown
  onCopySaveLink: (pass: WalletPassInstance) => unknown
  onRefreshPassSaveLink: (pass: WalletPassInstance) => unknown
  passStatusVariant: (status: string) => BadgeVariant
  passStatusLabel: (status: string) => string
  walletScenarioLabel: (pass: WalletPassInstance, template?: WalletPassTemplate) => string
}

export function WalletSendDeliveryCard({
  writable,
  loading,
  refreshing,
  dispatchingDelivery,
  resolvingSaveLinkPassID,
  readOnlyBoundaryHint,
  deliveryPassID,
  deliveryEmailEnabled,
  deliveryWhatsAppEnabled,
  deliverablePasses,
  selectedDeliveryPass,
  selectedDeliveryTemplate,
  templateByID,
  passDeliveryForm,
  deliveryEmailRecipientsField,
  deliveryWhatsAppRecipientsField,
  passDeliveryFormError,
  onDeliveryPassIDChange,
  onDeliveryEmailEnabledChange,
  onDeliveryWhatsAppEnabledChange,
  onDeliveryEmailRecipientsChange,
  onDeliveryWhatsAppRecipientsChange,
  onSubmit,
  onOpenPassQrDialog,
  onCopySaveLink,
  onRefreshPassSaveLink,
  passStatusVariant,
  passStatusLabel,
  walletScenarioLabel,
}: WalletSendDeliveryCardProps) {
  const { t } = useTranslation()
  const deliverySubmitDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : dispatchingDelivery
      ? t("walletPage.disabledReasons.deliveryInProgress")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : !deliveryPassID
          ? t("walletPage.disabledReasons.selectDeliveryPass")
          : ""
  const emailInputDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : !deliveryEmailEnabled
      ? t("walletPage.disabledReasons.emailChannelOff")
      : ""
  const whatsAppInputDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : !deliveryWhatsAppEnabled
      ? t("walletPage.disabledReasons.whatsAppChannelOff")
      : ""

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.cards.sendDelivery.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.cards.sendDelivery.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form className="space-y-4" onSubmit={passDeliveryForm.handleSubmit(onSubmit)}>
          <Controller
            control={passDeliveryForm.control}
            name="delivery_pass_id"
            render={({ field }) => (
              <Select
                value={field.value}
                onValueChange={(value) => {
                  field.onChange(value)
                  onDeliveryPassIDChange(value)
                }}
              >
                <SelectTrigger className="w-full min-w-0">
                  <SelectValue placeholder={t("walletPage.placeholders.deliveryPass")} />
                </SelectTrigger>
                <SelectContent>
                  {deliverablePasses.map((item) => {
                    const itemTemplate = templateByID.get(item.template_id)
                    return (
                      <SelectItem key={item.id} value={item.id}>
                        {item.target_id} · {itemTemplate?.name ?? item.template_id}
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
            )}
          />

          {selectedDeliveryPass ? (
            <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="min-w-0 break-words font-medium">{selectedDeliveryPass.target_id}</span>
                <Badge variant={passStatusVariant(selectedDeliveryPass.status)}>
                  {passStatusLabel(selectedDeliveryPass.status)}
                </Badge>
                <Badge variant="secondary">
                  {walletScenarioLabel(selectedDeliveryPass, selectedDeliveryTemplate)}
                </Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                {selectedDeliveryTemplate?.name ?? selectedDeliveryPass.template_id}
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                {selectedDeliveryPass.save_link
                  ? t("walletPage.cards.sendDelivery.passHasSaveLink")
                  : t("walletPage.cards.sendDelivery.passMissingSaveLink")}
              </p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                {selectedDeliveryPass.save_link ? (
                  <>
                    <Button size="sm" type="button" variant="outline" onClick={() => void onOpenPassQrDialog(selectedDeliveryPass)}>
                      {t("walletPage.actions.viewQrCode")}
                    </Button>
                    <Button size="sm" type="button" variant="outline" onClick={() => void onCopySaveLink(selectedDeliveryPass)}>
                      {t("walletPage.actions.copyLink")}
                    </Button>
                  </>
                ) : (
                  <Button
                    size="sm"
                    type="button"
                    variant="outline"
                    onClick={() => void onRefreshPassSaveLink(selectedDeliveryPass)}
                    disabled={resolvingSaveLinkPassID === selectedDeliveryPass.id}
                  >
                    {resolvingSaveLinkPassID === selectedDeliveryPass.id
                      ? t("walletPage.actions.refreshing")
                      : t("walletPage.actions.refreshLink")}
                  </Button>
                )}
              </div>
            </div>
          ) : (
            <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
              {deliverablePasses.length === 0
                ? t("walletPage.cards.sendDelivery.emptyNoSaveLinks")
                : t("walletPage.cards.sendDelivery.emptySelectPass")}
            </div>
          )}

          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-3 rounded-xl border bg-muted/10 p-4">
              <div className="flex items-center justify-between gap-3">
                <div className="space-y-0.5">
                  <p className="inline-flex items-center gap-1 text-sm font-medium">
                    <MailIcon className="size-3.5" />
                    Email
                  </p>
                  <p className="mp-kpi-note">{t("walletPage.cards.sendDelivery.emailHint")}</p>
                </div>
                <Controller
                  control={passDeliveryForm.control}
                  name="delivery_email_enabled"
                  render={({ field }) => (
                    <Switch
                      checked={field.value}
                      disabled={!writable}
                      title={!writable ? t("walletPage.disabledReasons.readOnly") : undefined}
                      onCheckedChange={(checked) => {
                        field.onChange(checked)
                        onDeliveryEmailEnabledChange(checked)
                      }}
                    />
                  )}
                />
              </div>
              <Textarea
                {...deliveryEmailRecipientsField}
                rows={4}
                disabled={!writable || !deliveryEmailEnabled}
                title={emailInputDisabledReason || undefined}
                onChange={(event) => {
                  deliveryEmailRecipientsField.onChange(event)
                  onDeliveryEmailRecipientsChange(event.target.value)
                }}
                placeholder={t("walletPage.placeholders.emailRecipients")}
              />
            </div>

            <div className="space-y-3 rounded-xl border bg-muted/10 p-4">
              <div className="flex items-center justify-between gap-3">
                <div className="space-y-0.5">
                  <p className="inline-flex items-center gap-1 text-sm font-medium">
                    <MessageCircleIcon className="size-3.5" />
                    WhatsApp
                  </p>
                  <p className="mp-kpi-note">{t("walletPage.cards.sendDelivery.whatsAppHint")}</p>
                </div>
                <Controller
                  control={passDeliveryForm.control}
                  name="delivery_whatsapp_enabled"
                  render={({ field }) => (
                    <Switch
                      checked={field.value}
                      disabled={!writable}
                      title={!writable ? t("walletPage.disabledReasons.readOnly") : undefined}
                      onCheckedChange={(checked) => {
                        field.onChange(checked)
                        onDeliveryWhatsAppEnabledChange(checked)
                      }}
                    />
                  )}
                />
              </div>
              <Textarea
                {...deliveryWhatsAppRecipientsField}
                rows={4}
                disabled={!writable || !deliveryWhatsAppEnabled}
                title={whatsAppInputDisabledReason || undefined}
                onChange={(event) => {
                  deliveryWhatsAppRecipientsField.onChange(event)
                  onDeliveryWhatsAppRecipientsChange(event.target.value)
                }}
                placeholder={t("walletPage.placeholders.whatsAppRecipients")}
              />
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="submit"
              disabled={
                !writable ||
                dispatchingDelivery ||
                loading ||
                refreshing ||
                !deliveryPassID ||
                passDeliveryForm.formState.isSubmitting
              }
              title={deliverySubmitDisabledReason || undefined}
            >
              {dispatchingDelivery ? t("walletPage.actions.sending") : t("walletPage.actions.sendDelivery")}
            </Button>
            {!writable ? (
              <span className="mp-kpi-note">
                {t("walletPage.hints.readOnlyDeliveryReceiptsOnly")}
                {readOnlyBoundaryHint}
              </span>
            ) : null}
            {writable && deliverySubmitDisabledReason ? (
              <span className="mp-kpi-note">{deliverySubmitDisabledReason}</span>
            ) : null}
          </div>

          {passDeliveryFormError ? <p className="text-sm text-destructive">{passDeliveryFormError}</p> : null}
        </form>
      </CardContent>
    </Card>
  )
}
