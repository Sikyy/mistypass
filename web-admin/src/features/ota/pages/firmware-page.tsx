import { useTranslation } from "react-i18next"

import type { CurrentUser } from "@/lib/api"
import { FirmwareListCard } from "../components/firmware-list-card"
import { FirmwareSummaryCard } from "../components/firmware-summary-card"
import { FirmwareUploadCard } from "../components/firmware-upload-card"

const UPLOAD_ROLES: CurrentUser["role"][] = ["super_admin", "tenant_admin", "building_admin"]

type FirmwarePageProps = {
  token: string
  viewer: CurrentUser
}

export function FirmwarePage({ token, viewer }: FirmwarePageProps) {
  const { t } = useTranslation()
  const tenantID = viewer.role === "super_admin" ? undefined : viewer.tenant_id
  const canUpload = UPLOAD_ROLES.includes(viewer.role)
  return (
    <div className="space-y-6">
      <div className="mp-page-hero">
        <div className="relative z-10 flex flex-col gap-5">
          <div className="max-w-3xl space-y-2">
            <h1 className="mp-page-title">{t("ota.firmware.pageTitle")}</h1>
          </div>
        </div>
      </div>
      <div className="space-y-4">
        <FirmwareSummaryCard token={token} tenantID={tenantID} />
        {canUpload ? <FirmwareUploadCard token={token} tenantID={tenantID} /> : null}
        <FirmwareListCard token={token} tenantID={tenantID} />
      </div>
    </div>
  )
}
