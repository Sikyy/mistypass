import { Badge } from "@/components/ui/badge"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

type TenantOption = {
  id: string
  name: string
}

type AccessPageHeaderProps = {
  platformViewer: boolean
  selectedTenantID: string
  tenants: TenantOption[]
  onTenantChange: (nextTenantID: string) => void
}

export function AccessPageHeader({
  platformViewer,
  selectedTenantID,
  tenants,
  onTenantChange,
}: AccessPageHeaderProps) {
  return (
    <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">身份与权限</p>
        <h1 className="mp-page-title">
          {platformViewer ? "组织目录、策略与授权工作台" : "员工与用户组、权限策略、临时授权"}
        </h1>
        <p className="mp-page-description">
          {platformViewer
            ? "按租户查看目录准备度、策略配置和临时授权，下游发放统一收口到凭证发放页。"
            : "围绕当前组织先整理员工与用户组，再配置访问策略，最后处理访客和临时授权。"}
        </p>
      </div>

      {platformViewer ? (
        <div className="w-full lg:w-[340px]">
          <Select value={selectedTenantID} onValueChange={onTenantChange}>
            <SelectTrigger>
              <SelectValue placeholder="选择租户" />
            </SelectTrigger>
            <SelectContent>
              {tenants.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : (
        <Badge variant="outline" className="w-fit rounded-full px-3 py-1">
          {selectedTenantID || "当前组织"}
        </Badge>
      )}
    </div>
  )
}
