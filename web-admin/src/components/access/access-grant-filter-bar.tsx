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
  return (
    <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-[1fr_1fr_220px_220px_180px_auto]">
      <Input type="date" value={dateFrom} onChange={(event) => onDateFromChange(event.target.value)} placeholder="开始日期" />
      <Input type="date" value={dateTo} onChange={(event) => onDateToChange(event.target.value)} placeholder="结束日期" />
      <Select value={methodFilter} onValueChange={onMethodChange}>
        <SelectTrigger>
          <SelectValue placeholder="下发方式" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">全部方式</SelectItem>
          <SelectItem value="wallet">MistyPass 移动凭证</SelectItem>
          <SelectItem value="email_qr">邮件二维码凭证</SelectItem>
        </SelectContent>
      </Select>
      <Select value={passTypeFilter} onValueChange={onPassTypeChange}>
        <SelectTrigger>
          <SelectValue placeholder="对象类型" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">全部对象类型</SelectItem>
          {passTypeOptions.map((item) => (
            <SelectItem key={item} value={item}>
              {item}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={statusFilter} onValueChange={onStatusChange}>
        <SelectTrigger>
          <SelectValue placeholder="授权状态" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">全部状态</SelectItem>
          <SelectItem value="active">当前有效</SelectItem>
          <SelectItem value="expiring_soon">24 小时内到期</SelectItem>
          <SelectItem value="expired">已到期</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="outline" onClick={onReset}>
        清空筛选
      </Button>
    </div>
  )
}
