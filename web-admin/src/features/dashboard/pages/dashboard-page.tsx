import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import type { TFunction } from "i18next"
import { useTranslation } from "react-i18next"
import {
  ActivityIcon,
  AlertTriangleIcon,
  Building2Icon,
  CheckCircle2Icon,
  DoorOpenIcon,
  MapPinIcon,
  RadarIcon,
  RadioTowerIcon,
  SirenIcon,
  SparklesIcon,
  TriangleAlertIcon,
  Users2Icon,
  WalletCardsIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  listAlarms,
  listGateways,
  listGatewayEventCheckpointSummary,
  listTenants,
  type Alarm,
  type CurrentUser,
} from "@/lib/api"
import { getViewerBuildingIDs, getViewerTenantID, isBuildingAdmin, isPlatformViewer } from "@/lib/viewer"

type DashboardPageProps = {
  token: string
  viewer: CurrentUser
}

type DashboardFocusTone = "critical" | "warning" | "positive"

type DashboardFocusItem = {
  id: string
  titleKey: string
  noteKey: string
  values?: Record<string, string | number>
  tone: DashboardFocusTone
}

type DashboardSummary = {
  tenantCount: number
  gatewayTotal: number
  gatewayOnline: number
  openAlarmCount: number
  criticalAlarmCount: number
  alarmHandlingRate: number
  criticalFreeRate: number
  recentAlerts: Alarm[]
  focusItems: DashboardFocusItem[]
}

const openingWindowKeys = [
  "dashboard.windows.items.morning",
  "dashboard.windows.items.noon",
  "dashboard.windows.items.evening",
]

const alarmSeverityRank: Record<Alarm["severity"], number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
}

function severityColor(severity: Alarm["severity"]) {
  switch (severity) {
    case "critical":
      return "destructive"
    case "high":
      return "secondary"
    default:
      return "outline"
  }
}

function severityLabel(severity: Alarm["severity"], t: TFunction) {
  switch (severity) {
    case "critical":
      return t("dashboard.severity.critical")
    case "high":
      return t("dashboard.severity.high")
    case "medium":
      return t("dashboard.severity.medium")
    case "low":
      return t("dashboard.severity.low")
    default:
      return severity
  }
}

function formatDateTime(value: string, locale: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }
  return parsed.toLocaleString(locale)
}

function focusIcon(item: DashboardFocusItem) {
  switch (item.tone) {
    case "critical":
      return TriangleAlertIcon
    case "warning":
      return RadarIcon
    default:
      return CheckCircle2Icon
  }
}

function focusIconClassName(item: DashboardFocusItem) {
  switch (item.tone) {
    case "critical":
      return "text-destructive"
    case "warning":
      return "text-amber-500"
    default:
      return "text-emerald-500"
  }
}

function MetricSparkline({ tone = "silver" }: { tone?: "silver" | "safe" | "warning" | "danger" }) {
  const strokeClassName =
    tone === "safe"
      ? "stroke-emerald-200"
      : tone === "warning"
        ? "stroke-amber-200"
        : tone === "danger"
          ? "stroke-red-200"
          : "stroke-white/80"

  return (
    <svg viewBox="0 0 180 42" className="mp-sparkline h-9 w-full" aria-hidden>
      <path
        d="M0 28 C14 29 18 26 28 27 C40 29 43 18 55 22 C68 27 73 26 83 25 C96 24 101 15 112 18 C122 20 119 31 132 31 C145 32 148 23 159 24 C169 24 172 17 180 16"
        className={`${strokeClassName} fill-none`}
        strokeLinecap="round"
        strokeWidth="2"
      />
      <path
        d="M0 36 C16 34 22 33 35 34 C55 36 62 31 82 31 C106 31 114 33 132 37 C150 40 164 32 180 28 L180 42 L0 42 Z"
        className="fill-white/5"
      />
    </svg>
  )
}

async function loadDashboardSummary(args: {
  token: string
  platformViewer: boolean
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerTenantID: string
  viewerBuildingIDs: Set<string>
}): Promise<DashboardSummary> {
  const [gateways, alarms, tenants, checkpointSummary] = await Promise.all([
    listGateways(args.token),
    args.missingBuildingScope ? Promise.resolve([]) : listAlarms(args.token, { page: 1, limit: 200 }),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
    args.missingBuildingScope
      ? Promise.resolve(null)
      : listGatewayEventCheckpointSummary(args.token, {
          tenant_id: args.platformViewer ? undefined : args.viewerTenantID || undefined,
          limit: 20,
          trend_window_minutes: 60,
        }).catch(() => null),
  ])

  const scopedGateways = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? gateways.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : gateways
  const scopedAlarms = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? alarms.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : alarms

  let tenantCount = 0
  if (args.platformViewer) {
    tenantCount = tenants.length
  } else if (args.buildingAdmin) {
    tenantCount = args.viewerBuildingIDs.size
  } else {
    tenantCount = args.viewerTenantID ? 1 : 0
  }

  const gatewayOnline = scopedGateways.filter((item) => item.status === "online").length
  const openAlarmCount = scopedAlarms.filter((item) => item.status === "open").length
  const criticalAlarmCount = scopedAlarms.filter((item) => item.severity === "critical").length
  const handledAlarmCount = scopedAlarms.filter((item) => item.status !== "open").length
  const alarmHandlingRate =
    scopedAlarms.length === 0 ? 100 : Math.round((handledAlarmCount / scopedAlarms.length) * 100)
  const criticalFreeRate =
    scopedAlarms.length === 0
      ? 100
      : Math.round(((scopedAlarms.length - criticalAlarmCount) / scopedAlarms.length) * 100)
  const recentAlerts = scopedAlarms
    .slice()
    .sort((left, right) => {
      const severityDiff = alarmSeverityRank[left.severity] - alarmSeverityRank[right.severity]
      if (severityDiff !== 0) {
        return severityDiff
      }
      return new Date(right.created_at).getTime() - new Date(left.created_at).getTime()
    })
    .slice(0, 5)

  const offlineGatewayCount = Math.max(0, scopedGateways.length - gatewayOnline)
  const checkpointLagTotal = checkpointSummary?.totals.lag_total ?? 0
  const checkpointQueueTotal = checkpointSummary?.totals.queues ?? 0

  const focusItems: DashboardFocusItem[] = []
  if (criticalAlarmCount > 0) {
    focusItems.push({
      id: "critical-alarms",
      titleKey: "dashboard.focus.items.criticalAlarm.title",
      noteKey: "dashboard.focus.items.criticalAlarm.note",
      values: {
        count: criticalAlarmCount,
        location: recentAlerts.find((item) => item.severity === "critical")?.location ?? scopedAlarms[0]?.location ?? "--",
      },
      tone: "critical",
    })
  } else {
    focusItems.push({
      id: "no-critical-alarms",
      titleKey: "dashboard.focus.items.noCriticalAlarm.title",
      noteKey: "dashboard.focus.items.noCriticalAlarm.note",
      tone: "positive",
    })
  }

  if (offlineGatewayCount > 0) {
    focusItems.push({
      id: "offline-gateways",
      titleKey: "dashboard.focus.items.gatewayOffline.title",
      noteKey: "dashboard.focus.items.gatewayOffline.note",
      values: { count: offlineGatewayCount },
      tone: "warning",
    })
  } else {
    focusItems.push({
      id: "gateway-stable",
      titleKey: "dashboard.focus.items.gatewayStable.title",
      noteKey: "dashboard.focus.items.gatewayStable.note",
      values: {
        online: gatewayOnline,
        total: scopedGateways.length,
      },
      tone: "positive",
    })
  }

  if (checkpointLagTotal > 0) {
    focusItems.push({
      id: "checkpoint-lag",
      titleKey: "dashboard.focus.items.checkpointLag.title",
      noteKey: "dashboard.focus.items.checkpointLag.note",
      values: {
        lagCount: checkpointLagTotal,
        queueCount: checkpointQueueTotal,
      },
      tone: "warning",
    })
  } else {
    focusItems.push({
      id: "checkpoint-healthy",
      titleKey: "dashboard.focus.items.checkpointHealthy.title",
      noteKey: "dashboard.focus.items.checkpointHealthy.note",
      tone: "positive",
    })
  }

  return {
    tenantCount,
    gatewayTotal: scopedGateways.length,
    gatewayOnline,
    openAlarmCount,
    criticalAlarmCount,
    alarmHandlingRate,
    criticalFreeRate,
    recentAlerts,
    focusItems,
  }
}

export function DashboardPage({ token, viewer }: DashboardPageProps) {
  const { t, i18n } = useTranslation()
  const platformViewer = isPlatformViewer(viewer)
  const buildingAdmin = isBuildingAdmin(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const viewerBuildingIDs = useMemo(() => new Set(getViewerBuildingIDs(viewer)), [viewer])
  const viewerBuildingScopeKey = useMemo(
    () => Array.from(viewerBuildingIDs).sort((a, b) => a.localeCompare(b)).join(","),
    [viewerBuildingIDs]
  )
  const missingBuildingScope = buildingAdmin && viewerBuildingIDs.size === 0
  const summaryQuery = useQuery({
    queryKey: [
      "dashboard-summary",
      viewer.id,
      platformViewer,
      buildingAdmin,
      missingBuildingScope,
      viewerTenantID,
      viewerBuildingScopeKey,
    ],
    queryFn: () =>
      loadDashboardSummary({
        token,
        platformViewer,
        buildingAdmin,
        missingBuildingScope,
        viewerTenantID,
        viewerBuildingIDs,
      }),
    staleTime: 60 * 1000,
  })

  const tenantCount = summaryQuery.data?.tenantCount ?? 0
  const gatewayTotal = summaryQuery.data?.gatewayTotal ?? 0
  const gatewayOnline = summaryQuery.data?.gatewayOnline ?? 0
  const openAlarmCount = summaryQuery.data?.openAlarmCount ?? 0
  const criticalAlarmCount = summaryQuery.data?.criticalAlarmCount ?? 0
  const alarmHandlingRate = summaryQuery.data?.alarmHandlingRate ?? 100
  const criticalFreeRate = summaryQuery.data?.criticalFreeRate ?? 100
  const recentAlerts = summaryQuery.data?.recentAlerts ?? []
  const focusItems = summaryQuery.data?.focusItems ?? []
  const loading = summaryQuery.isPending
  const offlineGatewayCount = Math.max(0, gatewayTotal - gatewayOnline)

  const gatewayRate = useMemo(() => {
    if (gatewayTotal === 0) {
      return 0
    }
    return Math.round((gatewayOnline / gatewayTotal) * 100)
  }, [gatewayOnline, gatewayTotal])

  const kpis = [
    {
      title: platformViewer
        ? t("dashboard.kpi.tenantCount.titlePlatform")
        : buildingAdmin
          ? t("dashboard.kpi.tenantCount.titleBuildingAdmin")
          : t("dashboard.kpi.tenantCount.titleTenant"),
      value: tenantCount.toString(),
      note: platformViewer
        ? t("dashboard.kpi.tenantCount.notePlatform")
        : buildingAdmin
          ? t("dashboard.kpi.tenantCount.noteBuildingAdmin")
          : t("dashboard.kpi.tenantCount.noteTenant"),
      icon: Building2Icon,
    },
    {
      title: t("dashboard.kpi.gatewayOnline.title"),
      value: `${gatewayOnline}/${gatewayTotal}`,
      note: t("dashboard.kpi.gatewayOnline.note"),
      icon: RadarIcon,
    },
    {
      title: t("dashboard.kpi.openAlarm.title"),
      value: openAlarmCount.toString(),
      note: missingBuildingScope
        ? t("dashboard.kpi.openAlarm.noteMissingScope")
        : t("dashboard.kpi.openAlarm.noteDefault"),
      icon: SirenIcon,
    },
    {
      title: t("dashboard.kpi.criticalAlarm.title"),
      value: criticalAlarmCount.toString(),
      note: missingBuildingScope
        ? t("dashboard.kpi.criticalAlarm.noteMissingScope")
        : t("dashboard.kpi.criticalAlarm.noteDefault"),
      icon: AlertTriangleIcon,
    },
  ]

  return (
    <div className="space-y-6">
      <div className="mp-page-hero">
        <div className="relative z-10 grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
          <div className="space-y-5">
            <Badge variant="outline" className="w-fit gap-1.5 rounded-full border-white/15 bg-white/5 px-3 py-1 text-xs">
              <ActivityIcon className="size-3.5 text-emerald-300" />
              {t("dashboard.hero.monitoring")}
            </Badge>
            <div className="space-y-2">
              <p className="mp-page-eyebrow">
                {platformViewer
                  ? t("dashboard.hero.eyebrowPlatform")
                  : buildingAdmin
                    ? t("dashboard.hero.eyebrowBuildingAdmin")
                    : t("dashboard.hero.eyebrowTenant")}
              </p>
              <h1 className="mp-page-title">{t("dashboard.hero.title")}</h1>
              <p className="mp-page-description max-w-2xl">
                {platformViewer
                  ? t("dashboard.hero.descriptionPlatform")
                  : buildingAdmin
                    ? t("dashboard.hero.descriptionBuildingAdmin")
                    : t("dashboard.hero.descriptionTenant")}
              </p>
            </div>
            <div className="grid max-w-2xl gap-3 sm:grid-cols-3">
              <div className="rounded-2xl border border-white/10 bg-black/[0.18] p-3">
                <p className="mp-kpi-note">{t("dashboard.command.onlineEdge")}</p>
                <p className="mt-1 text-xl font-semibold">{gatewayRate}%</p>
              </div>
              <div className="rounded-2xl border border-white/10 bg-black/[0.18] p-3">
                <p className="mp-kpi-note">{t("dashboard.command.openAlarms")}</p>
                <p className="mt-1 text-xl font-semibold">{loading ? "--" : openAlarmCount}</p>
              </div>
              <div className="rounded-2xl border border-white/10 bg-black/[0.18] p-3">
                <p className="mp-kpi-note">{t("dashboard.command.criticalFree")}</p>
                <p className="mt-1 text-xl font-semibold">{criticalFreeRate}%</p>
              </div>
            </div>
          </div>

          <div className="relative min-h-[230px] overflow-hidden rounded-3xl border border-white/10 bg-black/25 p-4">
            <div className="absolute inset-x-0 bottom-0 h-32 bg-[radial-gradient(ellipse_at_center,rgba(255,255,255,0.18),transparent_58%)] blur-xl" />
            <div className="absolute bottom-8 left-1/2 h-24 w-48 -translate-x-1/2 rounded-[50%] border border-white/15 bg-white/5 shadow-[0_0_60px_rgba(255,255,255,0.18)]" />
            <div className="absolute bottom-[3.75rem] left-1/2 h-20 w-28 -translate-x-1/2 rounded-[48%_52%_42%_58%] bg-white/80 blur-[1px]" />
            <div className="absolute bottom-[5.5rem] left-1/2 h-24 w-[4.5rem] -translate-x-1/2 rounded-t-full bg-white/90" />
            <div className="relative z-10 flex items-center justify-between">
              <span className="mp-command-pill">
                <SparklesIcon className="size-3.5" />
                {t("dashboard.command.mistSignal")}
              </span>
              <span className="mp-command-pill">
                <RadioTowerIcon className="size-3.5 text-emerald-300" />
                {gatewayOnline}/{gatewayTotal}
              </span>
            </div>
          </div>
        </div>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          {t("dashboard.notice.missingScope")}
        </div>
      ) : summaryQuery.isError ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {t("dashboard.notice.loadFailed", {
            message:
              summaryQuery.error instanceof Error
                ? summaryQuery.error.message
                : t("dashboard.notice.unknownError"),
          })}
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          {t("dashboard.notice.buildingScopeHint")}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {kpis.map((kpi, index) => (
          <div key={kpi.title} className="mp-metric-card">
            <div className="relative z-10 space-y-3">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm text-muted-foreground">{kpi.title}</p>
                  <p className="mt-1 text-3xl font-semibold tracking-[-0.04em]">{loading ? "--" : kpi.value}</p>
                </div>
                <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
                  <kpi.icon className="size-4 text-white/75" />
                </div>
              </div>
              <MetricSparkline tone={index === 2 ? "warning" : index === 3 ? "danger" : "silver"} />
              <p className="mp-kpi-note">{kpi.note}</p>
            </div>
          </div>
        ))}
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.3fr_0.7fr]">
        <Card className="mp-fog-surface">
          <CardHeader>
            <CardTitle className="text-base">{t("dashboard.command.siteConstellation")}</CardTitle>
            <CardDescription>{t("dashboard.command.siteConstellationDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="mp-orbit-map">
              <div className="mp-site-node left-[17%] top-[28%]">
                <Building2Icon className="size-5" />
              </div>
              <div className="mp-site-node right-[20%] top-[22%]">
                <DoorOpenIcon className="size-5" />
              </div>
              <div className="mp-site-node left-[43%] top-[48%]">
                <WalletCardsIcon className="size-5" />
              </div>
              <div className="mp-site-node right-[15%] bottom-[20%]">
                <MapPinIcon className="size-5" />
              </div>
              <div className="absolute bottom-4 left-4 right-4 z-10 grid gap-2 sm:grid-cols-3">
                <div className="rounded-xl border border-white/10 bg-black/30 p-3 backdrop-blur">
                  <p className="mp-kpi-note">{t("dashboard.command.primaryHub")}</p>
                  <p className="text-sm font-medium">{t("dashboard.command.liveGateways", { count: gatewayOnline })}</p>
                </div>
                <div className="rounded-xl border border-white/10 bg-black/30 p-3 backdrop-blur">
                  <p className="mp-kpi-note">{t("dashboard.command.offlineEdge")}</p>
                  <p className="text-sm font-medium">{t("dashboard.command.needsReview", { count: offlineGatewayCount })}</p>
                </div>
                <div className="rounded-xl border border-white/10 bg-black/30 p-3 backdrop-blur">
                  <p className="mp-kpi-note">{t("dashboard.command.criticalAlarms")}</p>
                  <p className="text-sm font-medium">{t("dashboard.command.activeCount", { count: criticalAlarmCount })}</p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("dashboard.health.title")}</CardTitle>
            <CardDescription>{t("dashboard.health.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            {[
              { label: t("dashboard.health.gatewayRate"), value: gatewayRate, tone: "bg-emerald-300" },
              { label: t("dashboard.health.responseRate"), value: alarmHandlingRate, tone: "bg-white" },
              { label: t("dashboard.health.criticalFreeRate"), value: criticalFreeRate, tone: "bg-sky-200" },
            ].map((item) => (
              <div key={item.label} className="space-y-2">
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{item.label}</span>
                  <span>{item.value}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-white/8">
                  <div className={`h-2 rounded-full ${item.tone} transition-all`} style={{ width: `${item.value}%` }} />
                </div>
              </div>
            ))}
            <Separator />
            <div className="mp-heartline" />
            <div className="flex items-start gap-2 text-xs text-muted-foreground">
              <Users2Icon className="mt-0.5 size-3.5" />
              <p>
                {buildingAdmin
                  ? t("dashboard.windows.handoverPrefixBuildingAdmin")
                  : t("dashboard.windows.handoverPrefixDefault")}
                <span className="font-medium text-foreground">
                  {" "}
                  {t("dashboard.windows.handoverTime")}
                </span>
                。
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.3fr_0.7fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("dashboard.alerts.title")}</CardTitle>
            <CardDescription>{t("dashboard.alerts.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("dashboard.alerts.table.alert")}</TableHead>
                  <TableHead>{t("dashboard.alerts.table.location")}</TableHead>
                  <TableHead>{t("dashboard.alerts.table.severity")}</TableHead>
                  <TableHead>{t("dashboard.alerts.table.createdAt")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentAlerts.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                      {missingBuildingScope
                        ? t("dashboard.alerts.emptyMissingScope")
                        : t("dashboard.alerts.emptyDefault")}
                    </TableCell>
                  </TableRow>
                ) : null}
                {recentAlerts.map((alert) => (
                  <TableRow key={alert.id}>
                    <TableCell className="font-medium">{alert.type}</TableCell>
                    <TableCell className="text-muted-foreground">{alert.location}</TableCell>
                    <TableCell>
                      <Badge variant={severityColor(alert.severity)} className="capitalize">
                        {severityLabel(alert.severity, t)}
                      </Badge>
                    </TableCell>
                    <TableCell>{formatDateTime(alert.created_at, i18n.language)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("dashboard.windows.title")}</CardTitle>
            <CardDescription>
              {buildingAdmin
                ? t("dashboard.windows.descriptionBuildingAdmin")
                : t("dashboard.windows.descriptionDefault")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {openingWindowKeys.map((windowKey) => (
              <div key={windowKey} className="rounded-xl border border-white/10 bg-white/[0.035] px-3 py-2 text-xs text-muted-foreground">
                {t(windowKey)}
              </div>
            ))}
            <Separator />
            {missingBuildingScope ? (
              <div className="rounded-lg border bg-muted/20 p-3 text-xs text-muted-foreground">
                {t("dashboard.focus.emptyMissingScope")}
              </div>
            ) : (
              <>
                {focusItems.map((item) => {
                  const Icon = focusIcon(item)
                  return (
                    <div key={item.id} className="flex items-start gap-2 rounded-lg border bg-muted/20 p-3">
                      <Icon className={`mt-0.5 size-4 ${focusIconClassName(item)}`} />
                      <div>
                        <p className="font-medium">{t(item.titleKey, item.values ?? {})}</p>
                        <p className="mp-kpi-note">{t(item.noteKey, item.values ?? {})}</p>
                      </div>
                    </div>
                  )
                })}
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
