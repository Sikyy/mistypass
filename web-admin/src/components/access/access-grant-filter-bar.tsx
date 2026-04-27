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
    <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,220px)_minmax(0,220px)_minmax(0,180px)_auto]">
      <Input
        type="date"
        value={dateFrom}
        onChange={(event) => onDateFromChange(event.target.value)}
        placeholder={t("accessPage.components.grantFilterBar.startDate")}
      />
      <Input
        type="date"
        value={dateTo}
        onChange={(event) => onDateToChange(event.target.value)}
        placeholder={t("accessPage.components.grantFilterBar.endDate")}
      />
      <Select value={methodFilter} onValueChange={onMethodChange}>
        <SelectTrigger className="w-full min-w-0">
          <SelectValue placeholder={t("accessPage.components.grantFilterBar.deliveryMethod")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("accessPage.components.grantFilterBar.deliveryAll")}</SelectItem>
          <SelectItem value="wallet">
            {t("accessPage.components.grantFilterBar.deliveryWallet")}
          </SelectItem>
          <SelectItem value="email_qr">
            {t("accessPage.components.grantFilterBar.deliveryEmailQr")}
          </SelectItem>
        </SelectContent>
      </Select>
      <Select value={passTypeFilter} onValueChange={onPassTypeChange}>
        <SelectTrigger className="w-full min-w-0">
          <SelectValue placeholder={t("accessPage.components.grantFilterBar.subjectType")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("accessPage.components.grantFilterBar.subjectAll")}</SelectItem>
          {passTypeOptions.map((item) => (
            <SelectItem key={item} value={item}>
              {item}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={statusFilter} onValueChange={onStatusChange}>
        <SelectTrigger className="w-full min-w-0">
          <SelectValue placeholder={t("accessPage.components.grantFilterBar.status")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("accessPage.components.grantFilterBar.statusAll")}</SelectItem>
          <SelectItem value="active">{t("accessPage.components.grantFilterBar.statusActive")}</SelectItem>
          <SelectItem value="expiring_soon">
            {t("accessPage.components.grantFilterBar.statusExpiringSoon")}
          </SelectItem>
          <SelectItem value="expired">{t("accessPage.components.grantFilterBar.statusExpired")}</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="outline" className="w-full xl:w-auto" onClick={onReset}>
        {t("accessPage.components.grantFilterBar.clearFilters")}
      </Button>
    </div>
  )
}
