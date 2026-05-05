import { type ComponentProps } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { type WalletPassInstance, type WalletPassTemplate } from "@/lib/api"

type BadgeVariant = ComponentProps<typeof Badge>["variant"]

type WalletDeliveryWorkspaceScenarioPreset = {
  id: string
  titleKey: string
}

type WalletDeliveryWorkspacePanelsProps = {
  scenarioPresets: WalletDeliveryWorkspaceScenarioPreset[]
  activeTemplateByScenario: Map<string, WalletPassTemplate>
  passScenarioCounts: Record<string, number>
  saveLinkScenarioCounts: Record<string, number>
  deliveryDeskPasses: WalletPassInstance[]
  templateByID: Map<string, WalletPassTemplate>
  resolvingSaveLinkPassID: string
  onFocusPassScenario: (scenarioID: string) => void
  onOpenPassQrDialog: (pass: WalletPassInstance) => unknown
  onCopySaveLink: (pass: WalletPassInstance) => unknown
  onRefreshPassSaveLink: (pass: WalletPassInstance) => unknown
  passStatusVariant: (status: string) => BadgeVariant
  passStatusLabel: (status: string) => string
  walletScenarioLabel: (pass: WalletPassInstance, template?: WalletPassTemplate) => string
  inferScenarioID: (pass: WalletPassInstance, template?: WalletPassTemplate) => string
  deliveryMethodLabel: (template?: WalletPassTemplate) => string
  accessMediumLabel: (template?: WalletPassTemplate) => string
  dispatchChannelLabels: (template?: WalletPassTemplate) => string
  deliveryHint: (pass: WalletPassInstance, template?: WalletPassTemplate) => string
}

export function WalletDeliveryWorkspacePanels({
  scenarioPresets,
  activeTemplateByScenario,
  passScenarioCounts,
  saveLinkScenarioCounts,
  deliveryDeskPasses,
  templateByID,
  resolvingSaveLinkPassID,
  onFocusPassScenario,
  onOpenPassQrDialog,
  onCopySaveLink,
  onRefreshPassSaveLink,
  passStatusVariant,
  passStatusLabel,
  walletScenarioLabel,
  inferScenarioID,
  deliveryMethodLabel,
  accessMediumLabel,
  dispatchChannelLabels,
  deliveryHint,
}: WalletDeliveryWorkspacePanelsProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 xl:grid-cols-[0.92fr_1.08fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("walletPage.cards.deliveryWorkspace.title")}</CardTitle>
          <CardDescription>
            {t("walletPage.cards.deliveryWorkspace.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {scenarioPresets.map((item) => {
            const activeTemplate = activeTemplateByScenario.get(item.id)
            return (
              <div
                key={item.id}
                className="flex flex-col gap-3 rounded-xl border bg-muted/10 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{t(item.titleKey)}</p>
                    <Badge variant="secondary">{t("walletPage.labels.passCount", { count: passScenarioCounts[item.id] ?? 0 })}</Badge>
                    <Badge variant="outline">{t("walletPage.labels.saveLinkCount", { count: saveLinkScenarioCounts[item.id] ?? 0 })}</Badge>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {activeTemplate
                      ? t("walletPage.cards.deliveryWorkspace.activeTemplate", {
                          templateName: activeTemplate.name,
                          deliveryMethod: deliveryMethodLabel(activeTemplate),
                          accessMedium: accessMediumLabel(activeTemplate),
                        })
                      : t("walletPage.cards.deliveryWorkspace.noTemplate")}
                  </p>
                  <p className="mp-kpi-note">
                    {t("walletPage.cards.deliveryWorkspace.dispatchChannels", {
                      channels: dispatchChannelLabels(activeTemplate),
                    })}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button size="sm" variant="outline" onClick={() => onFocusPassScenario(item.id)}>
                    {t("walletPage.actions.viewLedger")}
                  </Button>
                  {item.id === "visitor_temporary" ? (
                    <Button asChild size="sm" variant="outline">
                      <Link to="/access/grants">{t("walletPage.actions.goTemporaryAccess")}</Link>
                    </Button>
                  ) : null}
                </div>
              </div>
            )
          })}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("walletPage.cards.recentDelivery.title")}</CardTitle>
          <CardDescription>
            {t("walletPage.cards.recentDelivery.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {deliveryDeskPasses.length === 0 ? (
            <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              {t("walletPage.cards.recentDelivery.empty")}
            </div>
          ) : (
            deliveryDeskPasses.map((item) => {
              const itemTemplate = templateByID.get(item.template_id)
              const scenarioID = inferScenarioID(item, itemTemplate)
              return (
                <div
                  key={item.id}
                  className="flex flex-col gap-3 rounded-xl border bg-card/80 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
                >
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-medium">{item.target_id}</p>
                      <Badge variant={passStatusVariant(item.status)}>{passStatusLabel(item.status)}</Badge>
                      <Badge variant="secondary">{walletScenarioLabel(item, itemTemplate)}</Badge>
                    </div>
                    <p className="text-sm text-muted-foreground">{itemTemplate?.name ?? item.template_id}</p>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span>{deliveryMethodLabel(itemTemplate)}</span>
                      <span>{accessMediumLabel(itemTemplate)}</span>
                      <span>{dispatchChannelLabels(itemTemplate)}</span>
                    </div>
                    <p className="mp-kpi-note">{deliveryHint(item, itemTemplate)}</p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="outline" onClick={() => onFocusPassScenario(scenarioID)}>
                      {t("walletPage.actions.viewSimilarLedger")}
                    </Button>
                    {item.save_link ? (
                      <>
                        <Button size="sm" variant="outline" onClick={() => void onOpenPassQrDialog(item)}>
                          {t("walletPage.actions.viewQrCode")}
                        </Button>
                        <Button asChild size="sm" variant="outline">
                          <a href={item.save_link} rel="noreferrer" target="_blank">
                            {t("walletPage.actions.openLink")}
                          </a>
                        </Button>
                        <Button size="sm" variant="outline" onClick={() => void onCopySaveLink(item)}>
                          {t("walletPage.actions.copyLink")}
                        </Button>
                      </>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void onRefreshPassSaveLink(item)}
                        disabled={resolvingSaveLinkPassID === item.id}
                      >
                        {resolvingSaveLinkPassID === item.id
                          ? t("walletPage.actions.refreshing")
                          : t("walletPage.actions.refreshLink")}
                      </Button>
                    )}
                  </div>
                </div>
              )
            })
          )}
        </CardContent>
      </Card>
    </div>
  )
}
