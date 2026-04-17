import { useQuery } from "@tanstack/react-query"
import { useEffect, useMemo, useState } from "react"
import { CheckCircle2Icon, FilterIcon, ShieldAlertIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  listAccessEvents,
  listDeviceEvents,
  listTenants,
  type CurrentUser,
  type AccessEvent,
  type DeviceEvent,
  type Tenant,
} from "@/lib/api"
import { getViewerBuildingIDs, getViewerTenantID, isBuildingAdmin, isPlatformViewer } from "@/lib/viewer"

type EventsPageProps = {
  token: string
  viewer: CurrentUser
}

type EventType = "access_granted" | "access_denied" | "gateway_event"

type EventRow = {
  id: string
  tenantID: string
  buildingID: string
  areaID: string
  type: EventType
  actor: string
  door: string
  gateway: string
  result: "success" | "denied" | "warning" | string
  at: string
}

function toAccessEventRow(item: AccessEvent): EventRow {
  return {
    id: item.id,
    tenantID: item.tenant_id,
    buildingID: item.building_id,
    areaID: item.area_id,
    type: item.type as EventType,
    actor: item.actor,
    door: item.door_id,
    gateway: item.gateway_id,
    result: item.result,
    at: item.at,
  }
}

function toDeviceEventRow(item: DeviceEvent): EventRow {
  return {
    id: item.id,
    tenantID: item.tenant_id,
    buildingID: item.building_id,
    areaID: "-",
    type: "gateway_event",
    actor: "gateway_agent",
    door: "-",
    gateway: item.gateway_id,
    result: item.result,
    at: item.at,
  }
}

function resultVariant(result: EventRow["result"]) {
  switch (result) {
    case "success":
      return "outline"
    case "denied":
      return "destructive"
    default:
      return "secondary"
  }
}

function resultLabel(result: EventRow["result"]) {
  switch (result) {
    case "success":
      return "成功"
    case "denied":
      return "拒绝"
    case "warning":
      return "告警"
    default:
      return result
  }
}

function eventTypeLabel(type: EventType) {
  switch (type) {
    case "access_granted":
      return "放行"
    case "access_denied":
      return "拒绝"
    case "gateway_event":
      return "网关事件"
    default:
      return type
  }
}

type EventsPageData = {
  events: EventRow[]
  tenants: Tenant[]
}

async function loadEventsPageData(args: {
  token: string
  platformViewer: boolean
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerBuildingIDs: Set<string>
}): Promise<EventsPageData> {
  const [accessEvents, deviceEvents, tenantItems] = await Promise.all([
    listAccessEvents(args.token),
    listDeviceEvents(args.token),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
  ])

  const combined = [...accessEvents.map(toAccessEventRow), ...deviceEvents.map(toDeviceEventRow)]
    .filter((item) => {
      if (args.missingBuildingScope) {
        return false
      }
      if (!args.buildingAdmin) {
        return true
      }
      return args.viewerBuildingIDs.has(item.buildingID)
    })
    .sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime())
    .slice(0, 100)

  return {
    events: combined,
    tenants: tenantItems,
  }
}

export function EventsPage({ token, viewer }: EventsPageProps) {
  const platformViewer = isPlatformViewer(viewer)
  const buildingAdmin = isBuildingAdmin(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const viewerBuildingIDs = useMemo(() => new Set(getViewerBuildingIDs(viewer)), [viewer])
  const viewerBuildingScopeKey = useMemo(
    () => Array.from(viewerBuildingIDs).sort((a, b) => a.localeCompare(b)).join(","),
    [viewerBuildingIDs]
  )
  const missingBuildingScope = buildingAdmin && viewerBuildingIDs.size === 0
  const [query, setQuery] = useState("")
  const [typeFilter, setTypeFilter] = useState<"all" | EventType>("all")
  const [tenantFilter, setTenantFilter] = useState<"all" | string>(platformViewer ? "all" : viewerTenantID || "all")
  const eventsQuery = useQuery({
    queryKey: [
      "events-page",
      token,
      viewer.id,
      platformViewer,
      buildingAdmin,
      missingBuildingScope,
      viewerBuildingScopeKey,
    ],
    queryFn: () =>
      loadEventsPageData({
        token,
        platformViewer,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingIDs,
      }),
    staleTime: 30 * 1000,
  })

  const events = eventsQuery.data?.events ?? []
  const tenants = eventsQuery.data?.tenants ?? []
  const loading = eventsQuery.isPending
  const queryError = eventsQuery.isError && eventsQuery.error instanceof Error ? eventsQuery.error.message : ""

  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])

  useEffect(() => {
    if (platformViewer) {
      return
    }
    setTenantFilter(viewerTenantID || "all")
  }, [platformViewer, viewerTenantID])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return events.filter((row) => {
      const typeMatched = typeFilter === "all" || row.type === typeFilter
      const tenantMatched = !platformViewer || tenantFilter === "all" || row.tenantID === tenantFilter
      if (!typeMatched || !tenantMatched) {
        return false
      }
      if (!q) {
        return true
      }

      const tenantName = tenantByID.get(row.tenantID)?.name ?? row.tenantID
      return (
        row.id.toLowerCase().includes(q) ||
        row.actor.toLowerCase().includes(q) ||
        row.door.toLowerCase().includes(q) ||
        row.gateway.toLowerCase().includes(q) ||
        row.buildingID.toLowerCase().includes(q) ||
        row.areaID.toLowerCase().includes(q) ||
        tenantName.toLowerCase().includes(q)
      )
    })
  }, [events, query, tenantByID, tenantFilter, typeFilter])
  const hasActiveFilters = query.trim().length > 0 || typeFilter !== "all" || (platformViewer && tenantFilter !== "all")

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">事件流</p>
        <h1 className="mp-page-title">
          {platformViewer ? "通行与设备事件" : buildingAdmin ? "楼宇通行与设备事件" : "组织通行与设备事件"}
        </h1>
        <p className="mp-page-description">
          {platformViewer
            ? "所有事件都带租户、楼宇、区域、门点上下文，便于跨租户检索与排障。"
            : buildingAdmin
              ? "仅展示负责楼宇的通行与设备事件，不暴露其他楼宇范围。"
              : "聚焦当前组织的通行与设备事件，不再暴露租户切换。"}
        </p>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何事件记录，避免误暴露非本楼宇数据。
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          当前仅查看负责楼宇范围内最近 100 条通行与设备事件，可继续去告警页处置异常，或去网关页核对边缘设备状态。
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
          <CardDescription>放行事件</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {events.filter((item) => item.type === "access_granted").length}{" "}
              <CheckCircle2Icon className="size-4 text-emerald-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin ? "负责楼宇内通过策略校验的放行记录。" : "通过策略校验的门点放行记录。"}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>拒绝事件</CardDescription>
            <CardTitle className="text-2xl">
              {events.filter((item) => item.type === "access_denied").length}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">策略不匹配或凭证过期。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>网关异常</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {events.filter((item) => item.type === "gateway_event").length}{" "}
              <ShieldAlertIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin ? "负责楼宇内的心跳抖动与命令重试事件。" : "心跳抖动与命令重试事件。"}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">事件查询</CardTitle>
          <CardDescription>
            {platformViewer
              ? "支持按租户、类型、关键字组合过滤。"
              : buildingAdmin
                ? "支持按类型和关键字快速定位负责楼宇内的事件。"
                : "支持按类型和关键字快速定位事件。"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className={`grid gap-3 ${platformViewer ? "md:grid-cols-[1fr_220px_220px_auto]" : "md:grid-cols-[1fr_220px_auto]"}`}>
            <div className="relative">
              <FilterIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                className="pl-8"
                placeholder={platformViewer ? "按事件ID/租户/楼宇/区域/网关/门点搜索" : "按事件ID/楼宇/区域/网关/门点搜索"}
              />
            </div>
            {platformViewer ? (
              <Select value={tenantFilter} onValueChange={(value) => setTenantFilter(value)}>
                <SelectTrigger>
                  <SelectValue placeholder="租户" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部租户</SelectItem>
                  {tenants.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : null}
            <Select
              value={typeFilter}
              onValueChange={(value: "all" | EventType) => {
                setTypeFilter(value)
              }}
            >
              <SelectTrigger aria-label="事件类型">
                <SelectValue placeholder="事件类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部类型</SelectItem>
                <SelectItem value="access_granted">放行（access_granted）</SelectItem>
                <SelectItem value="access_denied">拒绝（access_denied）</SelectItem>
                <SelectItem value="gateway_event">网关事件（gateway_event）</SelectItem>
              </SelectContent>
            </Select>
            <Button
              variant="outline"
              onClick={() => {
                setQuery("")
                setTypeFilter("all")
                if (platformViewer) {
                  setTenantFilter("all")
                }
              }}
            >
              重置筛选
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">最近事件</CardTitle>
          <CardDescription>匹配到 {filtered.length} 条记录。</CardDescription>
        </CardHeader>
        <CardContent>
          {queryError ? (
            <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {queryError}
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>事件 ID</TableHead>
                {platformViewer ? <TableHead>租户</TableHead> : null}
                <TableHead>类型</TableHead>
                <TableHead>执行体</TableHead>
                <TableHead>楼宇 / 区域 / 门点</TableHead>
                <TableHead>网关</TableHead>
                <TableHead>结果</TableHead>
                <TableHead>时间戳</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 8 : 7} className="py-10 text-center text-muted-foreground">
                    正在加载事件...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 8 : 7} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? "当前楼宇管理员尚未分配楼宇范围。"
                      : hasActiveFilters
                        ? "当前筛选条件下没有匹配的事件。"
                        : buildingAdmin
                          ? "当前楼宇范围内暂无事件。"
                          : "当前范围内暂无事件。"}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                filtered.map((event) => (
                  <TableRow key={event.id}>
                    <TableCell className="font-medium">
                      <TableCellText className="max-w-[12rem]">{event.id}</TableCellText>
                    </TableCell>
                    {platformViewer ? (
                      <TableCell>
                        <TableCellText className="max-w-[13rem]">
                          {tenantByID.get(event.tenantID)?.name ?? event.tenantID}
                        </TableCellText>
                      </TableCell>
                    ) : null}
                    <TableCell>{eventTypeLabel(event.type)}</TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[12rem]">{event.actor}</TableCellText>
                    </TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[16rem]">
                        {event.buildingID} / {event.areaID} / {event.door}
                      </TableCellText>
                    </TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[10rem]">{event.gateway}</TableCellText>
                    </TableCell>
                    <TableCell>
                      <Badge variant={resultVariant(event.result)}>{resultLabel(event.result)}</Badge>
                    </TableCell>
                    <TableCell>{new Date(event.at).toLocaleString("zh-CN")}</TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
