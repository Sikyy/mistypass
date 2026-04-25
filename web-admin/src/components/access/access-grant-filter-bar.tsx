import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

type DeliveryMethodFilter = "all" | "email_qr" | "wallet"
type GrantStatusFilter = "all" | "active" | "expiring_soon" | "expired"

type AccessGrantFilterBarProps = {
  dateFrom: string
  dateTo: string
  methodFilter: DeliveryMethodFilter
  passTypeFilter: string
  statusFilter: GrantStatusFilter
  passTypeOptions: string[]
  onDateFromChange: (value: string) => void
  onDateToChange: (value: string) => void
  onMethodChange: (value: DeliveryMethodFilter) => void
  onPassTypeChange: (value: string) => void
  onStatusChange: (value: GrantStatusFilter) => void
  onReset: () => void
}

export function AccessGrantFilterBar({
  dateFrom,
  dateTo,
  methodFilter,
  passTypeFilter,
  statusFilter,
  passTypeOptions,
  onDateFromChange,
  onDateToChange,
  onMethodChange,
  onPassTypeChange,
  onStatusChange,
  onReset,
}: AccessGrantFilterBarProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-[1fr_1fr_220px_220px_180px_auto]">
      <Input
        type="date"
        value={dateFrom}
        onChange={(event) => onDateFromChange(event.target.value)}
        placeholder={t("accessPage.components.grantFilterBar.startDate", { defaultValue: "Start date" })}
      />
      <Input
        type="date"
        value={dateTo}
        onChange={(event) => onDateToChange(event.target.value)}
        placeholder={t("accessPage.components.grantFilterBar.endDate", { defaultValue: "End date" })}
      />
      <Select value={methodFilter} onValueChange={onMethodChange}>
        <SelectTrigger>
          <SelectValue placeholder={t("accessPage.components.grantFilterBar.deliveryMethod", { defaultValue: "Delivery method" })} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("accessPage.components.grantFilterBar.deliveryAll", { defaultValue: "All methods" })}</SelectItem>
          <SelectItem value="wallet">
            {t("accessPage.components.grantFilterBar.deliveryWallet", { defaultValue: "MistyPass mobile pass" })}
          </SelectItem>
          <SelectItem value="email_qr">
            {t("accessPage.components.grantFilterBar.deliveryEmailQr", { defaultValue: "Email QR pass" })}
          </SelectItem>
        </SelectContent>
      </Select>
      <Select value={passTypeFilter} onValueChange={onPassTypeChange}>
        <SelectTrigger>
          <SelectValue placeholder={t("accessPage.components.grantFilterBar.subjectType", { defaultValue: "Subject type" })} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("accessPage.components.grantFilterBar.subjectAll", { defaultValue: "All subject types" })}</SelectItem>
          {passTypeOptions.map((item) => (
            <SelectItem key={item} value={item}>
              {item}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={statusFilter} onValueChange={onStatusChange}>
        <SelectTrigger>
          <SelectValue placeholder={t("accessPage.components.grantFilterBar.status", { defaultValue: "Grant status" })} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("accessPage.components.grantFilterBar.statusAll", { defaultValue: "All statuses" })}</SelectItem>
          <SelectItem value="active">{t("accessPage.components.grantFilterBar.statusActive", { defaultValue: "Active" })}</SelectItem>
          <SelectItem value="expiring_soon">
            {t("accessPage.components.grantFilterBar.statusExpiringSoon", { defaultValue: "Expiring within 24h" })}
          </SelectItem>
          <SelectItem value="expired">{t("accessPage.components.grantFilterBar.statusExpired", { defaultValue: "Expired" })}</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="outline" onClick={onReset}>
        {t("accessPage.components.grantFilterBar.clearFilters", { defaultValue: "Clear filters" })}
      </Button>
    </div>
  )
}
