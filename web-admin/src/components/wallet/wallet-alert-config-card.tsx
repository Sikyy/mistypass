import { RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { type Tenant } from "@/lib/api"

type WalletAlertConfigCardProps = {
  platformViewer: boolean
  tenantID: string
  onTenantChange: (value: string) => void
  tenants: Tenant[]
  windowSeconds: string
  onWindowSecondsChange: (value: string) => void
  maxRetry: string
  onMaxRetryChange: (value: string) => void
  alertThreshold: string
  onAlertThresholdChange: (value: string) => void
  archiveLimit: string
  onArchiveLimitChange: (value: string) => void
  trendBucketCount: string
  onTrendBucketCountChange: (value: string) => void
  loading: boolean
  refreshing: boolean
  onApplyFilters: () => void
}

export function WalletAlertConfigCard({
  platformViewer,
  tenantID,
  onTenantChange,
  tenants,
  windowSeconds,
  onWindowSecondsChange,
  maxRetry,
  onMaxRetryChange,
  alertThreshold,
  onAlertThresholdChange,
  archiveLimit,
  onArchiveLimitChange,
  trendBucketCount,
  onTrendBucketCountChange,
  loading,
  refreshing,
  onApplyFilters,
}: WalletAlertConfigCardProps) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.alertConfig.title", { defaultValue: "Runtime window and alert parameters" })}</CardTitle>
        <CardDescription>
          {platformViewer
            ? t("walletPage.components.alertConfig.descriptionPlatform", {
                defaultValue:
                  "Supports tenant switching and overriding default window/threshold parameters. This config is for runtime window, not delivery method.",
              })
            : t("walletPage.components.alertConfig.descriptionTenant", {
                defaultValue:
                  "Runtime parameters and status window for current organization. This config is for runtime window, not delivery method.",
              })}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-7">
        {platformViewer ? (
          <Select value={tenantID} onValueChange={onTenantChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("walletPage.components.alertConfig.tenant", { defaultValue: "Tenant" })} />
            </SelectTrigger>
            <SelectContent>
              {tenants.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <Input value={tenantID} readOnly />
        )}

        <Input
          value={windowSeconds}
          onChange={(event) => onWindowSecondsChange(event.target.value)}
          placeholder="window_seconds"
        />
        <Input
          value={maxRetry}
          onChange={(event) => onMaxRetryChange(event.target.value)}
          placeholder={t("walletPage.components.alertConfig.maxRetry", { defaultValue: "max_retry (empty = default)" })}
        />
        <Input
          value={alertThreshold}
          onChange={(event) => onAlertThresholdChange(event.target.value)}
          placeholder={t("walletPage.components.alertConfig.dlqAlertThreshold", {
            defaultValue: "dlq_alert_threshold (empty = default)",
          })}
        />
        <Input
          value={archiveLimit}
          onChange={(event) => onArchiveLimitChange(event.target.value)}
          placeholder="archive limit"
        />
        <Input
          value={trendBucketCount}
          onChange={(event) => onTrendBucketCountChange(event.target.value)}
          placeholder="trend bucket_count"
        />

        <Button onClick={onApplyFilters} disabled={loading || refreshing}>
          <RefreshCwIcon className={`mr-1.5 size-4 ${refreshing ? "animate-spin" : ""}`} />
          {t("walletPage.components.alertConfig.refresh", { defaultValue: "Refresh" })}
        </Button>
      </CardContent>
    </Card>
  )
}
