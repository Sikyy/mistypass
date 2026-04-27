import { useTranslation } from "react-i18next"

import { ScopeLockedField } from "@/components/scope-locked-field"
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
  const selectedTenantLabel =
    tenants.find((item) => item.id === selectedTenantID)?.name ||
    selectedTenantID ||
    t("accessPage.components.header.currentOrganization")

  return (
    <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("accessPage.components.header.eyebrow")}</p>
        <h1 className="mp-page-title">
          {platformViewer
            ? t("accessPage.components.header.titlePlatform")
            : t("accessPage.components.header.titleTenant")}
        </h1>
        <p className="mp-page-description">
          {platformViewer
            ? t("accessPage.components.header.descriptionPlatform")
            : t("accessPage.components.header.descriptionTenant")}
        </p>
      </div>

      {platformViewer ? (
        <div className="w-full lg:max-w-[340px]">
          <Select value={selectedTenantID} onValueChange={onTenantChange}>
            <SelectTrigger className="w-full min-w-0">
              <SelectValue placeholder={t("accessPage.components.header.selectTenant")} />
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
        <ScopeLockedField
          className="w-full lg:max-w-[340px]"
          label={t("accessPage.components.header.currentOrganization")}
          value={selectedTenantLabel}
          description={t("accessPage.components.header.scopeLockedDescription")}
        />
      )}
    </div>
  )
}
