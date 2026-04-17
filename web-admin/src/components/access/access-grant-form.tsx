import { FormEventHandler } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { type Area, type Building, type Door } from "@/lib/api"

type ScopeType = "all" | "building" | "area" | "door"
type DeliveryMethod = "email_qr" | "wallet"

type AccessGrantFormProps = {
  onSubmit: FormEventHandler<HTMLFormElement>
  scopeType: ScopeType
  onScopeTypeChange: (value: ScopeType) => void
  deliveryMethod: DeliveryMethod
  onDeliveryMethodChange: (value: DeliveryMethod) => void
  buildingID: string
  onBuildingChange: (value: string) => void
  areaID: string
  onAreaChange: (value: string) => void
  doorID: string
  onDoorChange: (value: string) => void
  buildings: Building[]
  areaOptions: Area[]
  doorOptions: Door[]
  scopeSummaryLabel: string
  granteeName: string
  onGranteeNameChange: (value: string) => void
  granteeGender: string
  onGranteeGenderChange: (value: string) => void
  granteePhone: string
  onGranteePhoneChange: (value: string) => void
  granteeEmail: string
  onGranteeEmailChange: (value: string) => void
  mobileModel: string
  onMobileModelChange: (value: string) => void
  passType: string
  onPassTypeChange: (value: string) => void
  validUntil: string
  onValidUntilChange: (value: string) => void
}

export function AccessGrantForm({
  onSubmit,
  scopeType,
  onScopeTypeChange,
  deliveryMethod,
  onDeliveryMethodChange,
  buildingID,
  onBuildingChange,
  areaID,
  onAreaChange,
  doorID,
  onDoorChange,
  buildings,
  areaOptions,
  doorOptions,
  scopeSummaryLabel,
  granteeName,
  onGranteeNameChange,
  granteeGender,
  onGranteeGenderChange,
  granteePhone,
  onGranteePhoneChange,
  granteeEmail,
  onGranteeEmailChange,
  mobileModel,
  onMobileModelChange,
  passType,
  onPassTypeChange,
  validUntil,
  onValidUntilChange,
}: AccessGrantFormProps) {
  return (
    <form className="space-y-3" onSubmit={onSubmit}>
      <div className="grid grid-cols-2 gap-2">
        <Select value={scopeType} onValueChange={(value: ScopeType) => onScopeTypeChange(value)}>
          <SelectTrigger>
            <SelectValue placeholder="授权范围" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部区域</SelectItem>
            <SelectItem value="building">楼宇</SelectItem>
            <SelectItem value="area">区域</SelectItem>
            <SelectItem value="door">门点</SelectItem>
          </SelectContent>
        </Select>
        <Select value={deliveryMethod} onValueChange={(value: DeliveryMethod) => onDeliveryMethodChange(value)}>
          <SelectTrigger>
            <SelectValue placeholder="下发方式" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="wallet">MistyPass 移动凭证</SelectItem>
            <SelectItem value="email_qr">邮件二维码凭证</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <Select value={buildingID} onValueChange={onBuildingChange}>
        <SelectTrigger disabled={scopeType === "all"}>
          <SelectValue placeholder="楼宇（可选）" />
        </SelectTrigger>
        <SelectContent>
          {buildings.map((item) => (
            <SelectItem key={item.id} value={item.id}>
              {item.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={areaID} onValueChange={onAreaChange}>
        <SelectTrigger disabled={scopeType === "all" || scopeType === "building" || !buildingID}>
          <SelectValue placeholder="区域（可选）" />
        </SelectTrigger>
        <SelectContent>
          {areaOptions.map((item) => (
            <SelectItem key={item.id} value={item.id}>
              {item.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={doorID} onValueChange={onDoorChange}>
        <SelectTrigger disabled={scopeType !== "door" || !areaID}>
          <SelectValue placeholder="门点（可选）" />
        </SelectTrigger>
        <SelectContent>
          {doorOptions.map((item) => (
            <SelectItem key={item.id} value={item.id}>
              {item.name} ({item.id})
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">当前范围：{scopeSummaryLabel}</div>

      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1.5">
          <Label>姓名</Label>
          <Input value={granteeName} onChange={(event) => onGranteeNameChange(event.target.value)} placeholder="被授权人姓名" />
        </div>
        <div className="space-y-1.5">
          <Label>性别</Label>
          <Input value={granteeGender} onChange={(event) => onGranteeGenderChange(event.target.value)} placeholder="male/female/other" />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1.5">
          <Label>手机号</Label>
          <Input value={granteePhone} onChange={(event) => onGranteePhoneChange(event.target.value)} placeholder="+62-xxx-xxxx-xxxx" />
        </div>
        <div className="space-y-1.5">
          <Label>邮箱</Label>
          <Input value={granteeEmail} onChange={(event) => onGranteeEmailChange(event.target.value)} placeholder="name@company.com" />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1.5">
          <Label>手机型号</Label>
          <Input value={mobileModel} onChange={(event) => onMobileModelChange(event.target.value)} placeholder="Pixel 8 / iPhone 16" />
        </div>
        <div className="space-y-1.5">
          <Label>授权对象类型</Label>
          <Input value={passType} onChange={(event) => onPassTypeChange(event.target.value)} placeholder="employee / visitor / customer" />
        </div>
      </div>
      <Input
        type="datetime-local"
        value={validUntil}
        onChange={(event) => onValidUntilChange(event.target.value)}
        placeholder="有效截止（如 2026-04-11 20:00）"
      />
      <Button type="submit" className="w-full">
        创建授权
      </Button>
    </form>
  )
}
