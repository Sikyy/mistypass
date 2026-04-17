import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  ActivityIcon,
  AlertTriangleIcon,
  ArrowUpRightIcon,
  Building2Icon,
  CheckCircle2Icon,
  DoorOpenIcon,
  FingerprintIcon,
  RadarIcon,
  Users2Icon,
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
import { listGateways, listTenants, type CurrentUser } from "@/lib/api"
import { getViewerBuildingIDs, getViewerTenantID, isBuildingAdmin, isPlatformViewer } from "@/lib/viewer"

type DashboardPageProps = {
  token: string
  viewer: CurrentUser
}

type AlertRow = {
  id: string
  severity: "high" | "medium" | "low"
  type: string
  location: string
  createdAt: string
}

const recentAlerts: AlertRow[] = [
  {
    id: "alm_2401",
    severity: "high",
    type: "非法门常开",
    location: "Sudirman Hub / 8 层 / A-23 门",
    createdAt: "2026-04-10 21:10",
  },
  {
    id: "alm_2402",
    severity: "medium",
    type: "网关离线",
    location: "Kuningan Tower / 3 层 / 网关 MP-GW-112",
    createdAt: "2026-04-10 20:44",
  },
  {
    id: "alm_2403",
    severity: "low",
    type: "异常重试激增",
    location: "Menteng Workspace / 访客通道",
    createdAt: "2026-04-10 19:58",
  },
]

const openingWindows = [
  "07:00-09:00 早高峰通行",
  "12:00-13:30 午间访客高峰",
  "17:30-20:00 下班与加班通行高峰",
]

function severityColor(severity: AlertRow["severity"]) {
  switch (severity) {
    case "high":
      return "destructive"
    case "medium":
      return "secondary"
    default:
      return "outline"
  }
}

function severityLabel(severity: AlertRow["severity"]) {
  switch (severity) {
    case "high":
      return "高"
    case "medium":
      return "中"
    case "low":
      return "低"
    default:
      return severity
  }
}

type DashboardSummary = {
  tenantCount: number
  gatewayTotal: number
  gatewayOnline: number
}

async function loadDashboardSummary(args: {
  token: string
  platformViewer: boolean
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerTenantID: string
  viewerBuildingIDs: Set<string>
}): Promise<DashboardSummary> {
  const gateways = await listGateways(args.token)
  const scopedGateways = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? gateways.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : gateways

  let tenantCount = 0
  if (args.platformViewer) {
    const tenants = await listTenants(args.token)
    tenantCount = tenants.length
  } else if (args.buildingAdmin) {
    tenantCount = args.viewerBuildingIDs.size
  } else {
    tenantCount = args.viewerTenantID ? 1 : 0
  }

  return {
    tenantCount,
    gatewayTotal: scopedGateways.length,
    gatewayOnline: scopedGateways.filter((item) => item.status === "online").length,
  }
}

export function DashboardPage({ token, viewer }: DashboardPageProps) {
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
      token,
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
  const loading = summaryQuery.isPending

  const scopedRecentAlerts = missingBuildingScope ? [] : recentAlerts

  const gatewayRate = useMemo(() => {
    if (gatewayTotal === 0) {
      return 0
    }
    return Math.round((gatewayOnline / gatewayTotal) * 100)
  }, [gatewayOnline, gatewayTotal])

  const kpis = [
    {
      title: platformViewer ? "活跃租户" : buildingAdmin ? "负责楼宇" : "当前组织",
      value: tenantCount.toString(),
      note: platformViewer
        ? "商业办公与联合办公站点"
        : buildingAdmin
          ? "按楼宇管理员负责范围统计"
          : "企业工作台范围",
      icon: Building2Icon,
    },
    {
      title: "在线网关",
      value: `${gatewayOnline}/${gatewayTotal}`,
      note: "60 秒内有心跳上报",
      icon: RadarIcon,
    },
    {
      title: "开门成功率",
      value: missingBuildingScope ? "--" : "98.4%",
      note: missingBuildingScope ? "待分配楼宇范围后统计" : "近 24 小时 BLE + 云协同",
      icon: DoorOpenIcon,
    },
    {
      title: "严重告警",
      value: missingBuildingScope ? "0" : "2",
      note: missingBuildingScope ? "当前无楼宇范围" : "需要值班人员处理",
      icon: AlertTriangleIcon,
    },
  ]

  return (
    <div className="space-y-6">
      <div className="rounded-2xl border bg-gradient-to-br from-teal-500/10 via-background to-cyan-500/5 p-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="space-y-1">
            <p className="mp-page-eyebrow">
              {platformViewer ? "平台工作台" : buildingAdmin ? "楼宇值守工作台" : "企业工作台"}
            </p>
            <h1 className="mp-page-title">MistyPass 控制中心</h1>
            <p className="mp-page-description">
              {platformViewer
                ? "提供租户、网关、通行事件与告警态势的实时可视化。"
                : buildingAdmin
                  ? "聚焦负责楼宇的空间状态、网关在线率和告警处置，不暴露无关组织范围。"
                  : "聚焦当前组织的空间状态、发放运行和安全处置。"}
            </p>
          </div>
          <Badge variant="outline" className="w-fit gap-1.5 rounded-full px-3 py-1 text-xs">
            <ActivityIcon className="size-3.5 text-emerald-500" />
            实时监控
          </Badge>
        </div>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          当前楼宇管理员尚未分配 `building_ids` 范围。仪表盘只保留空态指标，不展示任何楼宇级运行数据。
        </div>
      ) : summaryQuery.isError ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          仪表盘数据加载失败：{summaryQuery.error instanceof Error ? summaryQuery.error.message : "未知错误"}
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          当前仪表盘仅统计负责楼宇范围内的网关和处置任务；若要继续核对空间、事件或告警，请进入对应页面查看详细列表。
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {kpis.map((kpi) => (
          <Card key={kpi.title}>
            <CardHeader className="pb-2">
              <CardDescription className="flex items-center justify-between">
                {kpi.title}
                <kpi.icon className="size-4 text-muted-foreground" />
              </CardDescription>
              <CardTitle className="text-2xl">{loading ? "--" : kpi.value}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="mp-kpi-note">{kpi.note}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.3fr_0.7fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">运行健康度</CardTitle>
            <CardDescription>SLA 降级前的核心服务指标。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>网关在线率</span>
                <span>{gatewayRate}%</span>
              </div>
              <div className="h-2 rounded-full bg-muted">
                <div
                  className="h-2 rounded-full bg-emerald-500 transition-all"
                  style={{ width: `${gatewayRate}%` }}
                />
              </div>
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>MQTT 下发成功率</span>
                <span>99.2%</span>
              </div>
              <div className="h-2 rounded-full bg-muted">
                <div className="h-2 w-[99.2%] rounded-full bg-cyan-500" />
              </div>
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>策略同步完成率</span>
                <span>96.8%</span>
              </div>
              <div className="h-2 rounded-full bg-muted">
                <div className="h-2 w-[96.8%] rounded-full bg-violet-500" />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">通行时段</CardTitle>
            <CardDescription>{buildingAdmin ? "用于值守排班的楼宇通行高峰。" : "用于排班的预期流量高峰。"}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {openingWindows.map((window) => (
              <div key={window} className="rounded-lg border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                {window}
              </div>
            ))}
            <Separator />
            <div className="flex items-start gap-2 text-xs text-muted-foreground">
              <Users2Icon className="mt-0.5 size-3.5" />
              <p>
                建议{buildingAdmin ? "楼宇值守" : "值班"}交接时间：
                <span className="font-medium text-foreground"> 本地时间 17:00</span>。
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.3fr_0.7fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">最近告警</CardTitle>
            <CardDescription>按严重级别与影响范围优先排序。</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>告警</TableHead>
                  <TableHead>位置</TableHead>
                  <TableHead>等级</TableHead>
                  <TableHead>发现时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {scopedRecentAlerts.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                      {missingBuildingScope ? "当前楼宇管理员尚未分配楼宇范围。" : "当前暂无重点告警。"}
                    </TableCell>
                  </TableRow>
                ) : null}
                {scopedRecentAlerts.map((alert) => (
                  <TableRow key={alert.id}>
                    <TableCell className="font-medium">{alert.type}</TableCell>
                    <TableCell className="text-muted-foreground">{alert.location}</TableCell>
                    <TableCell>
                      <Badge variant={severityColor(alert.severity)} className="capitalize">
                        {severityLabel(alert.severity)}
                      </Badge>
                    </TableCell>
                    <TableCell>{alert.createdAt}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">当前重点</CardTitle>
            <CardDescription>高价值处置项。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {missingBuildingScope ? (
              <div className="rounded-lg border bg-muted/20 p-3 text-xs text-muted-foreground">
                当前没有楼宇范围，暂不生成值守重点。建议先补齐楼宇管理员负责范围，再查看网关和告警重点。
              </div>
            ) : (
              <>
                <div className="flex items-start gap-2 rounded-lg border bg-muted/20 p-3">
                  <CheckCircle2Icon className="mt-0.5 size-4 text-emerald-500" />
                  <div>
                    <p className="font-medium">网关 OTA 窗口</p>
                    <p className="mp-kpi-note">今晚 23:00-01:00，已有 8 台设备排队。</p>
                  </div>
                </div>
                <div className="flex items-start gap-2 rounded-lg border bg-muted/20 p-3">
                  <FingerprintIcon className="mt-0.5 size-4 text-sky-500" />
                  <div>
                    <p className="font-medium">访客流转审计</p>
                    <p className="mp-kpi-note">
                      需复核 Kuningan Tower 的 14 张过期访客通行证。
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-2 rounded-lg border bg-muted/20 p-3">
                  <ArrowUpRightIcon className="mt-0.5 size-4 text-violet-500" />
                  <div>
                    <p className="font-medium">策略发布</p>
                    <p className="mp-kpi-note">
                      财务楼层通行策略 v0.1.8 已可发布。
                    </p>
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
