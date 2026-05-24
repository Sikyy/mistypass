import { type ComponentProps, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDownIcon, ChevronRightIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { WalletAlertNotificationRecordsCard } from "@/components/wallet/wallet-alert-notification-records-card"
import { WalletAlertSubscriptionCard } from "@/components/wallet/wallet-alert-subscription-card"
import { WalletAlertTrendPanels } from "@/components/wallet/wallet-alert-trend-panels"
import { WalletDlqCleanupArchivesCard } from "@/components/wallet/wallet-dlq-cleanup-archives-card"
import { WalletGoogleConfigCard } from "@/components/wallet/wallet-google-config-card"
import { WalletPhysicalCardTasksSection } from "@/components/wallet/wallet-physical-card-tasks-section"
import { WalletRiskOverviewPanels } from "@/components/wallet/wallet-risk-overview-panels"

type WalletAdvancedWorkspaceProps = {
  googleConfigCardProps: ComponentProps<typeof WalletGoogleConfigCard>
  physicalCardTasksSectionProps: ComponentProps<typeof WalletPhysicalCardTasksSection>
  alertSubscriptionCardProps: ComponentProps<typeof WalletAlertSubscriptionCard>
  riskOverviewPanelsProps: ComponentProps<typeof WalletRiskOverviewPanels>
  alertTrendPanelsProps: ComponentProps<typeof WalletAlertTrendPanels>
  alertNotificationRecordsCardProps: ComponentProps<typeof WalletAlertNotificationRecordsCard>
  dlqCleanupArchivesCardProps: ComponentProps<typeof WalletDlqCleanupArchivesCard>
}

export function WalletAdvancedWorkspace({
  googleConfigCardProps,
  physicalCardTasksSectionProps,
  alertSubscriptionCardProps,
  riskOverviewPanelsProps,
  alertTrendPanelsProps,
  alertNotificationRecordsCardProps,
  dlqCleanupArchivesCardProps,
}: WalletAdvancedWorkspaceProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const subscriptionStatus = alertSubscriptionCardProps.subscription?.enabled
    ? t("walletPage.advanced.summary.subscriptionEnabled")
    : t("walletPage.advanced.summary.subscriptionDisabled")
  const recentPhysicalCardTasks = physicalCardTasksSectionProps.recentPhysicalCardTasks ?? []
  const alertItems = riskOverviewPanelsProps.alertItems ?? []
  const alertNotifications = alertNotificationRecordsCardProps.alertNotifications ?? []
  const dlqArchives = dlqCleanupArchivesCardProps.archives ?? []
  const googleConfigStatus = googleConfigCardProps.config
    ? t("walletPage.advanced.summary.googleConfigured")
    : t("walletPage.advanced.summary.googleNotConfigured")
  const summaryItems = [
    {
      label: t("walletPage.advanced.summary.googleConfig"),
      value: googleConfigStatus,
    },
    {
      label: t("walletPage.advanced.summary.physicalTasks"),
      value: recentPhysicalCardTasks.length,
    },
    {
      label: t("walletPage.advanced.summary.alerts"),
      value: alertItems.length,
    },
    {
      label: t("walletPage.advanced.summary.notifications"),
      value: alertNotifications.length,
    },
    {
      label: t("walletPage.advanced.summary.dlqArchives"),
      value: dlqArchives.length,
    },
  ]
  const ToggleIcon = open ? ChevronDownIcon : ChevronRightIcon

  return (
    <section className="rounded-xl border bg-muted/15" data-testid="wallet-advanced-workspace">
      <div className="flex flex-col gap-3 px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-medium">{t("walletPage.advanced.title")}</p>
          <p className="mt-1 text-sm text-muted-foreground">{t("walletPage.advanced.description")}</p>
          <div className="mt-3 flex flex-wrap items-center gap-2" data-testid="wallet-advanced-summary">
            {summaryItems.map((item) => (
              <Badge key={item.label} variant="outline">
                {item.label}: {item.value}
              </Badge>
            ))}
            <Badge variant={alertSubscriptionCardProps.subscription?.enabled ? "secondary" : "outline"}>
              {subscriptionStatus}
            </Badge>
          </div>
        </div>
        <Button
          type="button"
          variant="outline"
          className="w-full justify-center gap-2 sm:w-auto"
          aria-expanded={open}
          data-testid="wallet-advanced-toggle"
          onClick={() => setOpen((current) => !current)}
        >
          <ToggleIcon className="size-4" />
          {open ? t("walletPage.advanced.collapse") : t("walletPage.advanced.expand")}
        </Button>
      </div>

      {open ? (
        <div className="space-y-4 border-t px-4 py-4" data-testid="wallet-advanced-content">
          <WalletGoogleConfigCard {...googleConfigCardProps} />

          <WalletPhysicalCardTasksSection {...physicalCardTasksSectionProps} />

          <WalletAlertSubscriptionCard {...alertSubscriptionCardProps} />

          <WalletRiskOverviewPanels {...riskOverviewPanelsProps} />

          <WalletAlertTrendPanels {...alertTrendPanelsProps} />

          <WalletAlertNotificationRecordsCard {...alertNotificationRecordsCardProps} />

          <WalletDlqCleanupArchivesCard {...dlqCleanupArchivesCardProps} />
        </div>
      ) : null}
    </section>
  )
}
