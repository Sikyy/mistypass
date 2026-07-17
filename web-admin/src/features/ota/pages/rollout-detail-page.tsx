import { useParams } from "react-router"
import { useTranslation } from "react-i18next"
import type { CurrentUser } from "@/lib/api"
import { RolloutDetail } from "../components/rollout-detail"

export function RolloutDetailPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const { t } = useTranslation()
  const { rolloutID } = useParams<{ rolloutID: string }>()
  const tenantID = viewer.role === "super_admin" ? undefined : viewer.tenant_id
  if (!rolloutID) return <p className="text-sm text-destructive">{t("ota.rollout.errors.generic")}</p>
  return (
    <div className="space-y-6">
      <div className="mp-page-hero"><div className="relative z-10 flex flex-col gap-5"><div className="max-w-3xl space-y-2"><h1 className="mp-page-title">{t("ota.rollout.detail.pageTitle")}</h1></div></div></div>
      <RolloutDetail token={token} tenantID={tenantID} viewer={viewer} id={rolloutID} />
    </div>
  )
}
