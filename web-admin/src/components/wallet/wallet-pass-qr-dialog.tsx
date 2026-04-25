import { type ComponentProps } from "react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { type WalletPassInstance, type WalletPassTemplate } from "@/lib/api"

type BadgeVariant = ComponentProps<typeof Badge>["variant"]

type WalletPassQrDialogProps = {
  open: boolean
  pass: WalletPassInstance | null
  template?: WalletPassTemplate
  loading: boolean
  previewURL: string
  saveLink: string
  resolvingSaveLinkPassID: string
  onOpenChange: (open: boolean) => void
  onDownloadSvg: () => void
  onCopyLink: () => void
  onRefreshLink: () => void
  passStatusVariant: (status: string) => BadgeVariant
  passStatusLabel: (status: string) => string
  scenarioLabel: (pass: WalletPassInstance, template?: WalletPassTemplate) => string
  deliveryMethodLabel: (template?: WalletPassTemplate) => string
  accessMediumLabel: (template?: WalletPassTemplate) => string
}

export function WalletPassQrDialog({
  open,
  pass,
  template,
  loading,
  previewURL,
  saveLink,
  resolvingSaveLinkPassID,
  onOpenChange,
  onDownloadSvg,
  onCopyLink,
  onRefreshLink,
  passStatusVariant,
  passStatusLabel,
  scenarioLabel,
  deliveryMethodLabel,
  accessMediumLabel,
}: WalletPassQrDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("walletPage.dialogs.qrCode.title")}</DialogTitle>
          <DialogDescription>
            {pass ? `${pass.target_id} · ${scenarioLabel(pass, template)}` : t("walletPage.dialogs.qrCode.description")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {pass ? (
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={passStatusVariant(pass.status)}>{passStatusLabel(pass.status)}</Badge>
              <Badge variant="secondary">{scenarioLabel(pass, template)}</Badge>
              <Badge variant="outline">{deliveryMethodLabel(template)}</Badge>
              <Badge variant="outline">{accessMediumLabel(template)}</Badge>
            </div>
          ) : null}

          <div className="flex min-h-84 items-center justify-center rounded-xl border bg-muted/10 p-4">
            {loading ? (
              <p className="text-sm text-muted-foreground">{t("walletPage.dialogs.qrCode.generating")}</p>
            ) : previewURL ? (
              <div className="rounded-lg border bg-white p-3 shadow-sm">
                <img
                  alt={t("walletPage.dialogs.qrCode.title")}
                  className="h-[280px] w-[280px]"
                  src={previewURL}
                />
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("walletPage.dialogs.qrCode.empty")}</p>
            )}
          </div>

          <div className="rounded-lg border bg-muted/10 px-3 py-2 text-xs text-muted-foreground break-all">
            {saveLink || t("walletPage.dialogs.qrCode.noSaveLink")}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {saveLink ? (
              <>
                <Button size="sm" variant="outline" onClick={onDownloadSvg}>
                  {t("walletPage.actions.downloadSvg")}
                </Button>
                <Button size="sm" variant="outline" onClick={onCopyLink}>
                  {t("walletPage.actions.copyLink")}
                </Button>
                <Button asChild size="sm" variant="outline">
                  <a href={saveLink} rel="noreferrer" target="_blank">
                    {t("walletPage.actions.openLink")}
                  </a>
                </Button>
              </>
            ) : null}
            {pass ? (
              <Button
                size="sm"
                variant="outline"
                onClick={onRefreshLink}
                disabled={resolvingSaveLinkPassID === pass.id}
              >
                {resolvingSaveLinkPassID === pass.id
                  ? t("walletPage.actions.refreshing")
                  : t("walletPage.actions.refreshLink")}
              </Button>
            ) : null}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
