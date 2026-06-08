import { useTranslation } from "react-i18next"
import type { CurrentUser } from "@/lib/api"
import { RolloutListCard } from "../components/rollout-list-card"
import { RolloutCreateCard } from "../components/rollout-create-card"

const WRITE_ROLES: CurrentUser["role"][] = ["super_admin", "tenant_admin", "building_admin"]

export function RolloutsPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const { t } = useTranslation()
  const tenantID = viewer.role === "super_admin" ? undefined : viewer.tenant_id
  const canWrite = WRITE_ROLES.includes(viewer.role)
  return (
    <div className="space-y-6">
      <div className="mp-page-hero">
        <div className="relative z-10 flex flex-col gap-5">
          <div className="max-w-3xl space-y-2"><h1 className="mp-page-title">{t("ota.rollout.pageTitle")}</h1></div>
        </div>
      </div>
      <div className="space-y-4">
        {canWrite ? <RolloutCreateCard token={token} tenantID={tenantID} /> : null}
        <RolloutListCard token={token} tenantID={tenantID} />
      </div>
    </div>
  )
}
