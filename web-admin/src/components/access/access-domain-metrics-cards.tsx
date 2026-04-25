import { Clock4Icon, ShieldCheckIcon, TicketIcon, UsersRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type AccessDomainMetricsCardsProps = {
  loading: boolean
  policyCount: number
  activeEmployeeCount: number
  groupCount: number
  grantCount: number
  visitorGrantCount: number
  expiredGrantCount: number
}

export function AccessDomainMetricsCards({
  loading,
  policyCount,
  activeEmployeeCount,
  groupCount,
  grantCount,
  visitorGrantCount,
  expiredGrantCount,
}: AccessDomainMetricsCardsProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>{t("accessPage.components.metrics.policies", { defaultValue: "Policies" })}</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            {loading ? "--" : policyCount} <ShieldCheckIcon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">
          {t("accessPage.components.metrics.policiesNote", {
            defaultValue: "Manage building/area/door access rules independently.",
          })}
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>{t("accessPage.components.metrics.directory", { defaultValue: "Employees & groups" })}</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            {loading ? "--" : activeEmployeeCount} <UsersRoundIcon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">
          {loading
            ? t("accessPage.components.metrics.directoryLoading", { defaultValue: "Loading directory status..." })
            : t("accessPage.components.metrics.directoryNote", {
                defaultValue: "{{groupCount}} groups maintained from organization directory.",
                groupCount,
              })}
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>{t("accessPage.components.metrics.grants", { defaultValue: "Temporary & visitor grants" })}</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            {loading ? "--" : grantCount} <Clock4Icon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">
          {loading
            ? t("accessPage.components.metrics.grantsLoading", { defaultValue: "Counting grants..." })
            : t("accessPage.components.metrics.grantsNote", {
                defaultValue: "Visitors {{visitorGrantCount}}, expired {{expiredGrantCount}}.",
                visitorGrantCount,
                expiredGrantCount,
              })}
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>{t("accessPage.components.metrics.deliveryMethods", { defaultValue: "Delivery methods" })}</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            2 <TicketIcon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">
          {t("accessPage.components.metrics.deliveryMethodsNote", { defaultValue: "MistyPass mobile pass / email QR." })}
        </CardContent>
      </Card>
    </div>
  )
}
