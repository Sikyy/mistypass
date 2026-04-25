import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { type Tenant } from "@/lib/api"

type EnterprisePageHeaderProps = {
  platformViewer: boolean
  selectedTenantID: string
  selectedTenantName?: string
  tenants: Tenant[]
  onTenantChange: (value: string) => void
}

export function EnterprisePageHeader({
  platformViewer,
  selectedTenantID,
  selectedTenantName,
  tenants,
  onTenantChange,
}: EnterprisePageHeaderProps) {
  const { t } = useTranslation()
  const headerMode = platformViewer ? "platform" : "tenant"

  return (
    <div className="mp-page-hero">
      <div className="relative z-10 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div className="max-w-3xl space-y-2">
          <p className="mp-page-eyebrow">{t(`enterprisePage.header.${headerMode}.eyebrow`)}</p>
          <h1 className="mp-page-title">{t(`enterprisePage.header.${headerMode}.title`)}</h1>
          <p className="mp-page-description">{t(`enterprisePage.header.${headerMode}.description`)}</p>
        </div>

        {platformViewer ? (
          <div className="w-full lg:w-[360px]">
            <Select value={selectedTenantID} onValueChange={onTenantChange}>
              <SelectTrigger data-testid="enterprise-tenant-select-trigger" className="border-white/10 bg-black/20">
                <SelectValue placeholder={t("enterprisePage.header.selectTenant")} />
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
          <Badge variant="outline" className="w-fit rounded-full border-white/15 bg-white/5 px-3 py-1">
            {selectedTenantName || selectedTenantID || t("enterprisePage.labels.currentOrganization")}
          </Badge>
        )}
      </div>
    </div>
  )
}
