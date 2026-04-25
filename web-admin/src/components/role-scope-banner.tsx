import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { BriefcaseBusinessIcon, Building2Icon, CircleDotIcon, RadioTowerIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { listBuildings, type CurrentUser } from "@/lib/api"
import { getViewerBuildingIDs, getViewerTenantID, isBuildingAdmin, isPlatformViewer } from "@/lib/viewer"

type RoleScopeBannerProps = {
  token: string
  viewer: CurrentUser
}

function joinLimited(items: string[], limit = 2) {
  if (items.length <= limit) {
    return items.join(", ")
  }
  return `${items.slice(0, limit).join(", ")} +${items.length - limit}`
}

export function RoleScopeBanner({ token, viewer }: RoleScopeBannerProps) {
  const { t } = useTranslation()
  const tenantID = getViewerTenantID(viewer)
  const buildingIDs = useMemo(() => getViewerBuildingIDs(viewer), [viewer])
  const buildingScopeKey = useMemo(() => buildingIDs.slice().sort((a, b) => a.localeCompare(b)).join(","), [buildingIDs])
  const buildingQuery = useQuery({
    queryKey: ["role-scope-buildings", viewer.id, tenantID, buildingScopeKey],
    queryFn: () => listBuildings(token, tenantID || undefined),
    enabled: isBuildingAdmin(viewer) && buildingIDs.length > 0,
    staleTime: 5 * 60 * 1000,
  })

  if (isPlatformViewer(viewer)) {
    return (
      <div className="hidden max-w-[18rem] items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-xs text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] lg:flex">
        <Building2Icon className="size-3.5 text-sky-200" />
        <span className="font-medium text-foreground/85">{t("app.scope.platform.label")}</span>
        <span className="truncate">{t("app.scope.platform.value")}</span>
      </div>
    )
  }

  if (viewer.role === "tenant_admin") {
    const tenantValue = tenantID || t("app.scope.tenant.missing")
    return (
      <div
        className="hidden max-w-[18rem] items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-xs text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] lg:flex"
        title={tenantValue}
      >
        <BriefcaseBusinessIcon className="size-3.5 text-emerald-200" />
        <span className="font-medium text-foreground/85">{t("app.scope.tenant.label")}</span>
        <span className="truncate">{tenantValue}</span>
      </div>
    )
  }

  if (isBuildingAdmin(viewer)) {
    const buildingNames = (buildingQuery.data ?? [])
      .filter((item) => buildingIDs.includes(item.id))
      .map((item) => item.name || item.id)
    const scopeNames = buildingNames.length > 0 ? buildingNames : buildingIDs
    const scopeValue =
      scopeNames.length > 0
        ? joinLimited(scopeNames)
        : t("app.scope.building.missing")

    return (
      <div
        className="hidden max-w-[20rem] items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-xs text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] lg:flex"
        title={scopeNames.join(", ")}
      >
        <RadioTowerIcon className="size-3.5 text-amber-200" />
        <span className="font-medium text-foreground/85">{t("app.scope.building.label")}</span>
        <span className="truncate">
          {buildingQuery.isFetching && buildingIDs.length > 0 ? t("app.scope.building.loading") : scopeValue}
        </span>
      </div>
    )
  }

  return (
    <div className="hidden max-w-[18rem] items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-xs text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] lg:flex">
      <CircleDotIcon className="size-3.5 text-cyan-200" />
      <span className="font-medium text-foreground/85">{t("app.scope.operator.label")}</span>
      <span className="truncate">{t("app.scope.operator.value")}</span>
    </div>
  )
}
