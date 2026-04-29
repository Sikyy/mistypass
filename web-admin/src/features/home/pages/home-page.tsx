import { useEffect, useMemo } from "react"
import i18next from "i18next"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Link, useLocation, useNavigate } from "react-router-dom"
import {
  AlertCircleIcon,
  Clock3Icon,
  CreditCardIcon,
  DoorOpenIcon,
  HistoryIcon,
  MapPinPlusIcon,
  MessageSquareIcon,
  PlusIcon,
  ShieldCheckIcon,
} from "lucide-react"

import {
  KpiCard,
  toneClassName,
  type ActivityTone,
} from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import { MistyisletAdminShell } from "@/features/mistyislet-shell/mistyislet-admin-shell"
import { formatMistyisletRoleLabel, parsePlaceRoute, placePath } from "@/features/mistyislet-shell/navigation"
import { MistyisletConsoleRoutes } from "@/features/mistyislet-shell/routes"
import {
  listAccessEvents,
  listAlarms,
  listGateways,
  type AccessEvent,
  type Alarm,
  type CurrentUser,
  type Gateway,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import {
  canAccessAlarmsPage,
  canAccessEventsPage,
  canAccessGatewaysPage,
  getViewerBuildingIDs,
  isBuildingAdmin,
  isPlatformViewer,
} from "@/lib/viewer"

type HomePageProps = {
  token: string
  viewer: CurrentUser
  onViewerChange: (viewer: CurrentUser) => void
  onLogout: () => void
}

type HomeActivity = {
  id: string
  at: string
  title: string
  meta: string
  tone: ActivityTone
  status: string
}

type HomeSummary = {
  gatewayTotal: number
  gatewayOnline: number
  todayAccessCount: number
  openAlarmCount: number
  pendingReviewCount: number
  activities: HomeActivity[]
  partial: boolean
}

const quickActions = [
  { labelKey: "kisi.home.addUser", to: "/users", icon: PlusIcon, primary: true },
  { labelKey: "kisi.home.issueCredential", to: "/credentials", icon: CreditCardIcon },
  { labelKey: "kisi.home.createPlace", to: "/places", icon: MapPinPlusIcon },
  { labelKey: "kisi.home.viewReports", to: "/reports", icon: AlertCircleIcon },
]

function isSameLocalDate(value: string, expected: Date) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return false
  }
  return (
    parsed.getFullYear() === expected.getFullYear() &&
    parsed.getMonth() === expected.getMonth() &&
    parsed.getDate() === expected.getDate()
  )
}

function shortID(value: string | undefined) {
  if (!value) {
    return "unknown"
  }
  return value.length > 8 ? value.slice(0, 8) : value
}

function formatPersonName(email: string) {
  const localPart = email.split("@")[0] || "operator"
  return localPart
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

function formatScopeName(viewer: CurrentUser) {
  if (isPlatformViewer(viewer)) {
    return "Mistyislet Platform"
  }
  if (isBuildingAdmin(viewer)) {
    const buildingCount = getViewerBuildingIDs(viewer).length
    return buildingCount > 1 ? `${buildingCount} assigned places` : "Assigned Place"
  }
  const domain = viewer.email.split("@")[1]
  if (domain) {
    return domain
  }
  return viewer.tenant_id ? `Organization ${shortID(viewer.tenant_id)}` : "Mistyislet Organization"
}

function formatActivityTime(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return "--:--"
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  }).format(parsed)
}

function scopedByBuilding<T extends { building_id: string }>(
  items: T[],
  buildingAdmin: boolean,
  buildingIDs: Set<string>
) {
  if (!buildingAdmin) {
    return items
  }
  return items.filter((item) => buildingIDs.has(item.building_id))
}

function accessActivity(event: AccessEvent): HomeActivity {
  const action =
    event.result === "success" ? i18next.t("kisi.home.accessGranted") : event.result === "denied" ? i18next.t("kisi.home.accessDenied") : i18next.t("kisi.home.accessWarning")

  return {
    id: `access-${event.id}`,
    at: event.at,
    title: `${event.actor || "Unknown user"} - ${action}`,
    meta: `Door ${shortID(event.door_id)} via gateway ${shortID(event.gateway_id)}`,
    tone: event.result === "success" ? "success" : event.result === "denied" ? "danger" : "warning",
    status: event.result,
  }
}

function alarmActivity(alarm: Alarm): HomeActivity {
  const severe = alarm.severity === "critical" || alarm.severity === "high"
  return {
    id: `alarm-${alarm.id}`,
    at: alarm.created_at,
    title: alarm.type,
    meta: alarm.location || `Door ${shortID(alarm.door_id)}`,
    tone: severe ? "danger" : "warning",
    status: alarm.status.replace(/_/g, " "),
  }
}

async function loadHomeSummary(token: string, viewer: CurrentUser): Promise<HomeSummary> {
  const buildingAdmin = isBuildingAdmin(viewer)
  const buildingIDs = new Set(getViewerBuildingIDs(viewer))
  const missingBuildingScope = buildingAdmin && buildingIDs.size === 0

  const [gatewaysResult, alarmsResult, accessEventsResult] = await Promise.allSettled([
    canAccessGatewaysPage(viewer) && !missingBuildingScope ? listGateways(token) : Promise.resolve<Gateway[]>([]),
    canAccessAlarmsPage(viewer) && !missingBuildingScope
      ? listAlarms(token, { page: 1, limit: 100 })
      : Promise.resolve<Alarm[]>([]),
    canAccessEventsPage(viewer) && !missingBuildingScope
      ? listAccessEvents(token, { page: 1, limit: 80 })
      : Promise.resolve<AccessEvent[]>([]),
  ])

  const gateways = gatewaysResult.status === "fulfilled" ? gatewaysResult.value : []
  const alarms = alarmsResult.status === "fulfilled" ? alarmsResult.value : []
  const accessEvents = accessEventsResult.status === "fulfilled" ? accessEventsResult.value : []
  const scopedGateways = scopedByBuilding(gateways, buildingAdmin, buildingIDs)
  const scopedAlarms = scopedByBuilding(alarms, buildingAdmin, buildingIDs)
  const scopedAccessEvents = scopedByBuilding(accessEvents, buildingAdmin, buildingIDs)
  const today = new Date()
  const todayAccessEvents = scopedAccessEvents.filter((event) => isSameLocalDate(event.at, today))
  const openAlarms = scopedAlarms.filter((alarm) => alarm.status === "open")
  const deniedToday = todayAccessEvents.filter((event) => event.result === "denied").length
  const urgentAlarms = openAlarms.filter((alarm) => alarm.severity === "critical" || alarm.severity === "high").length
  const activities = [...scopedAccessEvents.slice(0, 8).map(accessActivity), ...scopedAlarms.slice(0, 8).map(alarmActivity)]
    .sort((left, right) => new Date(right.at).getTime() - new Date(left.at).getTime())
    .slice(0, 7)

  return {
    gatewayTotal: scopedGateways.length,
    gatewayOnline: scopedGateways.filter((gateway) => gateway.status === "online").length,
    todayAccessCount: todayAccessEvents.length,
    openAlarmCount: openAlarms.length,
    pendingReviewCount: urgentAlarms + deniedToday,
    activities,
    partial:
      gatewaysResult.status === "rejected" ||
      alarmsResult.status === "rejected" ||
      accessEventsResult.status === "rejected",
  }
}

function RecentActivity({ items, loading }: { items: HomeActivity[]; loading: boolean }) {
  const { t } = useTranslation()
  return (
    <section className="rounded-[6px] border border-[#e1e3e8] bg-white">
      <div className="flex items-center justify-between border-b border-[#eceef2] px-6 py-5">
        <div>
          <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.home.recentActivity")}</h2>
          <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.home.recentDesc")}</p>
        </div>
        <Clock3Icon className="hidden size-5 text-[#9a9ca7] sm:block" />
      </div>

      <div className="divide-y divide-[#eceef2]">
        {loading ? (
          Array.from({ length: 5 }).map((_, index) => (
            <div key={index} className="grid grid-cols-[64px_1fr] gap-4 px-6 py-4 sm:grid-cols-[76px_1fr_120px]">
              <div className="h-4 w-12 animate-pulse rounded bg-[#eceef2]" />
              <div className="space-y-2">
                <div className="h-4 w-2/3 animate-pulse rounded bg-[#eceef2]" />
                <div className="h-3 w-1/2 animate-pulse rounded bg-[#f1f2f5]" />
              </div>
              <div className="hidden h-4 w-20 animate-pulse rounded bg-[#eceef2] sm:block" />
            </div>
          ))
        ) : items.length > 0 ? (
          items.map((item) => (
            <div
              key={item.id}
              className="grid grid-cols-[64px_1fr] gap-4 px-6 py-4 transition-colors hover:bg-[#fbfbfc] sm:grid-cols-[76px_1fr_120px]"
            >
              <time className="text-sm font-medium text-[#6f717c]">{formatActivityTime(item.at)}</time>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-[#17171c]">{item.title}</p>
                <p className="mt-1 truncate text-xs text-[#6f717c]">{item.meta}</p>
              </div>
              <div className="hidden items-center justify-end gap-2 text-xs font-medium capitalize text-[#6f717c] sm:flex">
                <span className={cn("size-2 rounded-full", toneClassName(item.tone))} />
                <span className="truncate">{item.status}</span>
              </div>
            </div>
          ))
        ) : (
          <div className="px-6 py-12 text-center">
            <MessageSquareIcon className="mx-auto size-8 text-[#9a9ca7]" />
            <p className="mt-3 text-sm font-semibold text-[#17171c]">{t("kisi.home.noActivity")}</p>
            <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.home.noActivity")}</p>
          </div>
        )}
      </div>
    </section>
  )
}

export function HomePage({ token, viewer, onViewerChange, onLogout }: HomePageProps) {
  const personName = formatPersonName(viewer.email)
  const scopeName = formatScopeName(viewer)
  const { t } = useTranslation()
  const summaryQuery = useQuery({
    queryKey: ["kisi-home-summary", viewer.id, viewer.role, viewer.tenant_id, viewer.building_ids?.join(",")],
    queryFn: () => loadHomeSummary(token, viewer),
    staleTime: 30 * 1000,
  })
  const summary = summaryQuery.data
  const loading = summaryQuery.isPending
  const gatewayTotal = summary?.gatewayTotal ?? 0
  const gatewayOnline = summary?.gatewayOnline ?? 0
  const openAlarmCount = summary?.openAlarmCount ?? 0
  const pendingReviewCount = summary?.pendingReviewCount ?? 0
  const todayAccessCount = summary?.todayAccessCount ?? 0
  const gatewayTone: ActivityTone = gatewayTotal === 0 || gatewayOnline === gatewayTotal ? "success" : "warning"
  const alarmTone: ActivityTone = openAlarmCount > 0 ? "danger" : "success"
  const pendingTone: ActivityTone = pendingReviewCount > 0 ? "warning" : "success"

  const kpis = useMemo(
    () => [
      {
        label: t("kisi.home.onlineGates"),
        value: `${gatewayOnline}/${gatewayTotal}`,
        note: gatewayTotal === 0 ? t("kisi.home.noHardware") : t("kisi.home.gatewayPoints"),
        icon: DoorOpenIcon,
        tone: gatewayTone,
      },
      {
        label: t("kisi.home.todayAccess"),
        value: todayAccessCount.toLocaleString(),
        note: t("kisi.home.gatewayPoints"),
        icon: HistoryIcon,
        tone: "info" as ActivityTone,
      },
      {
        label: t("kisi.home.openAlerts"),
        value: openAlarmCount.toString(),
        note: openAlarmCount > 0 ? t("kisi.home.followUp") : t("kisi.home.noAlertQueue"),
        icon: AlertCircleIcon,
        tone: alarmTone,
      },
      {
        label: t("kisi.home.pendingReview"),
        value: pendingReviewCount.toString(),
        note: pendingReviewCount > 0 ? t("kisi.home.deniedAlarms") : t("kisi.home.noReviewBacklog"),
        icon: ShieldCheckIcon,
        tone: pendingTone,
      },
    ],
    [alarmTone, gatewayOnline, gatewayTone, gatewayTotal, openAlarmCount, pendingReviewCount, pendingTone, todayAccessCount]
  )
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    const placeRoute = parsePlaceRoute(location.pathname)
    if (placeRoute?.placeId === "assigned") {
      navigate(placePath(placeRoute.section), { replace: true })
    }
  }, [location.pathname, navigate])

  const homeContent = (
    <>
      <div className="flex flex-col gap-6 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium text-[#4f55ff]">
            <Link to="/home" className="underline-offset-4 hover:underline">
              Home
            </Link>
          </div>
          <h1 className="mt-6 text-[34px] font-bold leading-[42px] text-[#17171c] sm:text-[40px] sm:leading-[48px]">
            {t("kisi.home.welcome", { name: personName })}
          </h1>
          <div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-[#6f717c]">
            <span>{scopeName}</span>
            <span className="size-1 rounded-full bg-[#c3c6d1]" />
            <span className="rounded-full border border-[#e1e3e8] bg-white px-2.5 py-1 text-xs font-semibold text-[#2f3037]">
              {formatMistyisletRoleLabel(viewer, location.pathname)}
            </span>
          </div>
        </div>

        {summaryQuery.isError || summary?.partial ? (
          <div className="rounded-[14px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
            Some live signals are unavailable.
          </div>
        ) : null}
      </div>

      <section className="mt-9 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {kpis.map((kpi) => (
          <KpiCard key={kpi.label} {...kpi} loading={loading} />
        ))}
      </section>

      <section className="mt-8 rounded-[6px] border border-[#e1e3e8] bg-white p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.home.quickActions")}</h2>
            <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.home.quickDesc")}</p>
          </div>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {quickActions.map((action) => {
              const Icon = action.icon
              return (
                <Button
                  key={action.labelKey}
                  asChild
                  variant={action.primary ? "default" : "outline"}
                  className={cn(
                    "h-10 rounded-[6px] px-4",
                    action.primary
                      ? "border-[#4f55ff] bg-[#4f55ff] text-white hover:bg-[#454bea]"
                      : "border-[#d9dbe3] bg-white text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                  )}
                >
                  <Link to={action.to}>
                    <Icon className="mr-1.5 size-4" />
                    {t(action.labelKey)}
                  </Link>
                </Button>
              )
            })}
          </div>
        </div>
      </section>

      <div className="mt-8">
        <RecentActivity items={summary?.activities ?? []} loading={loading} />
      </div>
    </>
  )

  return (
    <MistyisletAdminShell viewer={viewer} onLogout={onLogout}>
      <MistyisletConsoleRoutes
        homeContent={homeContent}
        token={token}
        viewer={viewer}
        onViewerChange={onViewerChange}
        onLogout={onLogout}
      />
    </MistyisletAdminShell>
  )
}
