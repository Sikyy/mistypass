import { type ComponentProps } from "react"
import { useTranslation } from "react-i18next"

import { WalletAlertNotificationRecordsCard } from "@/components/wallet/wallet-alert-notification-records-card"
import { WalletAlertSubscriptionCard } from "@/components/wallet/wallet-alert-subscription-card"
import { WalletAlertTrendPanels } from "@/components/wallet/wallet-alert-trend-panels"
import { WalletDlqCleanupArchivesCard } from "@/components/wallet/wallet-dlq-cleanup-archives-card"
import { WalletRiskOverviewPanels } from "@/components/wallet/wallet-risk-overview-panels"
import { TabsContent } from "@/components/ui/tabs"

type WalletAdvancedWorkspaceProps = {
  alertSubscriptionCardProps: ComponentProps<typeof WalletAlertSubscriptionCard>
  riskOverviewPanelsProps: ComponentProps<typeof WalletRiskOverviewPanels>
  alertTrendPanelsProps: ComponentProps<typeof WalletAlertTrendPanels>
  alertNotificationRecordsCardProps: ComponentProps<typeof WalletAlertNotificationRecordsCard>
  dlqCleanupArchivesCardProps: ComponentProps<typeof WalletDlqCleanupArchivesCard>
}

export function WalletAdvancedWorkspace({
  alertSubscriptionCardProps,
  riskOverviewPanelsProps,
  alertTrendPanelsProps,
  alertNotificationRecordsCardProps,
  dlqCleanupArchivesCardProps,
}: WalletAdvancedWorkspaceProps) {
  const { t } = useTranslation()

  return (
    <TabsContent value="advanced" className="space-y-4">
      <div className="rounded-xl border bg-muted/15 px-4 py-3">
        <p className="text-sm font-medium">{t("walletPage.advanced.title")}</p>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("walletPage.advanced.description")}
        </p>
      </div>

      <WalletAlertSubscriptionCard {...alertSubscriptionCardProps} />

      <WalletRiskOverviewPanels {...riskOverviewPanelsProps} />

      <WalletAlertTrendPanels {...alertTrendPanelsProps} />

      <WalletAlertNotificationRecordsCard {...alertNotificationRecordsCardProps} />

      <WalletDlqCleanupArchivesCard {...dlqCleanupArchivesCardProps} />
    </TabsContent>
  )
}
