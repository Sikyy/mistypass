import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()

  return (
    <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("accessPage.components.header.eyebrow", { defaultValue: "Identity & access" })}</p>
        <h1 className="mp-page-title">
          {platformViewer
            ? t("accessPage.components.header.titlePlatform", { defaultValue: "Directory, policy, and grant workspace" })
            : t("accessPage.components.header.titleTenant", { defaultValue: "Employees, groups, policies, and temporary grants" })}
        </h1>
        <p className="mp-page-description">
          {platformViewer
            ? t("accessPage.components.header.descriptionPlatform", {
                defaultValue:
                  "Review readiness, policy setup, and temporary grants by tenant; long-term issuance is centralized in pass issuance.",
              })
            : t("accessPage.components.header.descriptionTenant", {
                defaultValue:
                  "For current organization, prepare employees/groups first, then policies, then visitor and temporary grants.",
              })}
        </p>
      </div>

      {platformViewer ? (
        <div className="w-full lg:w-[340px]">
          <Select value={selectedTenantID} onValueChange={onTenantChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("accessPage.components.header.selectTenant", { defaultValue: "Select tenant" })} />
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
          {selectedTenantID || t("accessPage.components.header.currentOrganization", { defaultValue: "Current organization" })}
        </Badge>
      )}
    </div>
  )
}
