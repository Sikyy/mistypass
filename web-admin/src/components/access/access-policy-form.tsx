import { type FormEvent } from "react"

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

type AccessPolicyScopeType = "all" | "building" | "area" | "door"
type AccessPolicyStatus = "active" | "inactive" | "draft"
type AccessPolicyOption = {
  id: string
  name: string
}

type AccessPolicyFormProps = {
  areaID: string
  areaOptions: AccessPolicyOption[]
  buildingID: string
  buildingOptions: AccessPolicyOption[]
  doorID: string
  doorOptions: AccessPolicyOption[]
  isEditing: boolean
  members: string
  name: string
  onAreaIDChange: (value: string) => void
  onBuildingIDChange: (value: string) => void
  onDoorIDChange: (value: string) => void
  onMembersChange: (value: string) => void
  onNameChange: (value: string) => void
  onScheduleChange: (value: string) => void
  onScopeTypeChange: (value: AccessPolicyScopeType) => void
  onStatusChange: (value: AccessPolicyStatus) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  schedule: string
  scopeSummaryLabel: string
  scopeType: AccessPolicyScopeType
  status: AccessPolicyStatus
}

export function AccessPolicyForm({
  areaID,
  areaOptions,
  buildingID,
  buildingOptions,
  doorID,
  doorOptions,
  isEditing,
  members,
  name,
  onAreaIDChange,
  onBuildingIDChange,
  onDoorIDChange,
  onMembersChange,
  onNameChange,
  onScheduleChange,
  onScopeTypeChange,
  onStatusChange,
  onSubmit,
  schedule,
  scopeSummaryLabel,
  scopeType,
  status,
}: AccessPolicyFormProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{isEditing ? "编辑策略" : "新建策略"}</CardTitle>
        <CardDescription>将访问规则单独抽离出来，明确到楼宇、区域、门点，而不是继续混在用户组和授权里。</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-3" onSubmit={onSubmit}>
          <Input value={name} onChange={(event) => onNameChange(event.target.value)} placeholder="策略名称" />

          <div className="grid grid-cols-2 gap-2">
            <Select value={scopeType} onValueChange={onScopeTypeChange}>
              <SelectTrigger>
                <SelectValue placeholder="范围类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部区域</SelectItem>
                <SelectItem value="building">楼宇</SelectItem>
                <SelectItem value="area">区域</SelectItem>
                <SelectItem value="door">门点</SelectItem>
              </SelectContent>
            </Select>
            <Select value={status} onValueChange={onStatusChange}>
              <SelectTrigger>
                <SelectValue placeholder="状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="active">启用</SelectItem>
                <SelectItem value="inactive">停用</SelectItem>
                <SelectItem value="draft">草稿</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Select value={buildingID} onValueChange={onBuildingIDChange}>
            <SelectTrigger disabled={scopeType === "all"}>
              <SelectValue placeholder="楼宇（可选）" />
            </SelectTrigger>
            <SelectContent>
              {buildingOptions.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select value={areaID} onValueChange={onAreaIDChange}>
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

          <Select value={doorID} onValueChange={onDoorIDChange}>
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

          <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
            当前范围：{scopeSummaryLabel}
          </div>

          <Input value={schedule} onChange={(event) => onScheduleChange(event.target.value)} placeholder="时间计划（如 Mon-Fri 07:00-19:00）" />
          <Input value={members} onChange={(event) => onMembersChange(event.target.value)} placeholder="授权人数" />
          <Button type="submit" className="w-full">
            {isEditing ? "更新策略" : "创建策略"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
