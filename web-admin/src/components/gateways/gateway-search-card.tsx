import { SearchIcon } from "lucide-react"
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
import type { GatewayStatus } from "@/lib/api"

type GatewaySearchCardProps = {
  platformViewer: boolean
  query: string
  onQueryChange: (value: string) => void
  gatewayStatusFilter: "all" | GatewayStatus
  onGatewayStatusFilterChange: (value: "all" | GatewayStatus) => void
  onResetFilters: () => void
}

export function GatewaySearchCard({
  platformViewer,
  query,
  onQueryChange,
  gatewayStatusFilter,
  onGatewayStatusFilterChange,
  onResetFilters,
}: GatewaySearchCardProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("gateways.search.title")}</CardTitle>
        <CardDescription>
          {platformViewer ? t("gateways.search.descriptionPlatform") : t("gateways.search.descriptionDefault")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,220px)_auto]">
          <div className="relative">
            <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => onQueryChange(event.target.value)}
              className="pl-8"
              aria-label={t("gateways.search.queryPlaceholder")}
              placeholder={t("gateways.search.queryPlaceholder")}
            />
          </div>
          <Select
            value={gatewayStatusFilter}
            onValueChange={(value) => {
              onGatewayStatusFilterChange(value as "all" | GatewayStatus)
            }}
          >
            <SelectTrigger aria-label={t("gateways.search.statusAriaLabel")}>
              <SelectValue placeholder={t("gateways.search.statusPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("gateways.search.allStatuses")}</SelectItem>
              <SelectItem value="online">{t("gateways.status.online")}</SelectItem>
              <SelectItem value="offline">{t("gateways.status.offline")}</SelectItem>
              <SelectItem value="disabled">{t("gateways.status.disabled")}</SelectItem>
              <SelectItem value="revoked">{t("gateways.status.revoked")}</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" className="w-full lg:w-auto" onClick={onResetFilters}>
            {t("gateways.search.reset")}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
