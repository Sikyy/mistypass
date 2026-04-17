import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useMemo, useState } from "react"
import { BellRingIcon, FilterIcon, MailIcon, MessageCircleIcon, SirenIcon, TriangleAlertIcon, XIcon } from "lucide-react"

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
  listAlarms,
  listTenants,
  listUserGroups,
  updateAlarmStatus,
  type CurrentUser,
  type Alarm,
  type Tenant,
  type UserGroup,
} from "@/lib/api"
import { getViewerBuildingIDs, isBuildingAdmin, isPlatformViewer } from "@/lib/viewer"

type AlarmsPageProps = {
  token: string
  viewer: CurrentUser
}

type AlarmWorkflowStatus =
  | "acknowledged"
  | "investigating"
  | "mitigated"
  | "escalated"
  | "false_positive"
  | "resolved"

function severityVariant(severity: Alarm["severity"]) {
  switch (severity) {
    case "critical":
    case "high":
      return "destructive"
    case "medium":
      return "secondary"
    default:
      return "outline"
  }
}

function statusVariant(status: Alarm["status"]) {
  switch (status) {
    case "open":
    case "escalated":
      return "destructive"
    case "acknowledged":
    case "investigating":
      return "secondary"
    default:
      return "outline"
  }
}

function severityLabel(severity: Alarm["severity"]) {
  switch (severity) {
    case "critical":
      return "严重"
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

function statusLabel(status: Alarm["status"]) {
  switch (status) {
    case "open":
      return "待处理"
    case "acknowledged":
      return "已确认"
    case "investigating":
      return "调查中"
    case "mitigated":
      return "已缓解"
    case "escalated":
      return "已升级"
    case "resolved":
      return "已关闭"
    case "false_positive":
      return "误报"
    default:
      return status
  }
}

const statusOptions: AlarmWorkflowStatus[] = [
  "acknowledged",
  "investigating",
  "mitigated",
  "escalated",
  "false_positive",
  "resolved",
]

type NotificationLog = {
  id: string
  alarm_id: string
  channel: "email" | "whatsapp"
  receiver_group: string
  status: "queued" | "sent"
  created_at: string
}

type AlarmsPageData = {
  alarms: Alarm[]
  tenants: Tenant[]
  userGroups: UserGroup[]
}

async function loadAlarmsPageData(args: {
  token: string
  platformViewer: boolean
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerBuildingIDs: Set<string>
}): Promise<AlarmsPageData> {
  const [alarmItems, groupItems, tenantItems] = await Promise.all([
    listAlarms(args.token),
    listUserGroups(args.token),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
  ])
  const scopedAlarms = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? alarmItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : alarmItems

  return {
    alarms: scopedAlarms,
    tenants: tenantItems,
    userGroups: groupItems,
  }
}

export function AlarmsPage({ token, viewer }: AlarmsPageProps) {
  const queryClient = useQueryClient()
  const platformViewer = isPlatformViewer(viewer)
  const buildingAdmin = isBuildingAdmin(viewer)
  const viewerBuildingIDs = useMemo(() => new Set(getViewerBuildingIDs(viewer)), [viewer])
  const viewerBuildingScopeKey = useMemo(
    () => Array.from(viewerBuildingIDs).sort((a, b) => a.localeCompare(b)).join(","),
    [viewerBuildingIDs]
  )
  const missingBuildingScope = buildingAdmin && viewerBuildingIDs.size === 0
  const [error, setError] = useState("")
  const [updatingAlarmID, setUpdatingAlarmID] = useState<string | null>(null)
  const [pendingStatus, setPendingStatus] = useState<Record<string, AlarmWorkflowStatus>>({})
  const [notifyGroups, setNotifyGroups] = useState<string[]>(["安全组"])
  const [notifyGroupDraft, setNotifyGroupDraft] = useState("安全组")
  const [notifyEmail, setNotifyEmail] = useState(true)
  const [notifyWhatsApp, setNotifyWhatsApp] = useState(true)
  const [notificationLogs, setNotificationLogs] = useState<NotificationLog[]>([])
  const [query, setQuery] = useState("")
  const [statusFilter, setStatusFilter] = useState<"all" | Alarm["status"]>("all")
  const [severityFilter, setSeverityFilter] = useState<"all" | Alarm["severity"]>("all")
  const alarmsQueryKey = useMemo(
    () =>
      [
        "alarms-page",
        token,
        viewer.id,
        platformViewer,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingScopeKey,
      ] as const,
    [token, viewer.id, platformViewer, buildingAdmin, missingBuildingScope, viewerBuildingScopeKey]
  )
  const alarmsQuery = useQuery({
    queryKey: alarmsQueryKey,
    queryFn: () =>
      loadAlarmsPageData({
        token,
        platformViewer,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingIDs,
      }),
    staleTime: 30 * 1000,
  })

  const alarms = alarmsQuery.data?.alarms ?? []
  const tenants = alarmsQuery.data?.tenants ?? []
  const userGroups = alarmsQuery.data?.userGroups ?? []
  const loading = alarmsQuery.isPending
  const queryError = alarmsQuery.isError && alarmsQuery.error instanceof Error ? alarmsQuery.error.message : ""
  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])
  const updateAlarmStatusMutation = useMutation({
    mutationFn: (payload: { alarmID: string; status: AlarmWorkflowStatus }) =>
      updateAlarmStatus(token, payload.alarmID, payload.status),
    onSuccess: (updated) => {
      queryClient.setQueryData<AlarmsPageData>(alarmsQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          alarms: current.alarms.map((item) => (item.id === updated.id ? updated : item)),
        }
      })
    },
  })

  const stats = useMemo(() => {
    const open = alarms.filter((item) => item.status === "open").length
    const critical = alarms.filter((item) => item.severity === "critical").length
    const investigating = alarms.filter((item) => item.status === "investigating").length
    return { open, critical, investigating }
  }, [alarms])
  const filteredAlarms = useMemo(() => {
    const q = query.trim().toLowerCase()
    return alarms.filter((alarm) => {
      if (statusFilter !== "all" && alarm.status !== statusFilter) {
        return false
      }
      if (severityFilter !== "all" && alarm.severity !== severityFilter) {
        return false
      }
      if (!q) {
        return true
      }
      const tenantName = tenantByID.get(alarm.tenant_id)?.name ?? alarm.tenant_id
      return (
        alarm.id.toLowerCase().includes(q) ||
        alarm.type.toLowerCase().includes(q) ||
        alarm.location.toLowerCase().includes(q) ||
        alarm.building_id.toLowerCase().includes(q) ||
        alarm.area_id.toLowerCase().includes(q) ||
        alarm.door_id.toLowerCase().includes(q) ||
        tenantName.toLowerCase().includes(q)
      )
    })
  }, [alarms, query, severityFilter, statusFilter, tenantByID])
  const hasActiveFilters = query.trim().length > 0 || statusFilter !== "all" || severityFilter !== "all"

  const notifyGroupOptions = useMemo(() => {
    const base = userGroups.map((item) => item.name).filter(Boolean)
    return Array.from(new Set(["安全组", ...base]))
  }, [userGroups])

  function addNotifyGroup(value: string) {
    const group = value.trim()
    if (!group) {
      return
    }
    setNotifyGroups((current) => {
      if (current.includes(group)) {
        return current
      }
      return [...current, group]
    })
  }

  function removeNotifyGroup(value: string) {
    setNotifyGroups((current) => {
      const next = current.filter((item) => item !== value)
      if (next.length === 0) {
        return ["安全组"]
      }
      return next
    })
  }

  function pushNotificationLogs(alarmID: string) {
    const records: NotificationLog[] = []
    const now = new Date().toISOString()
    const groups = notifyGroups.length > 0 ? notifyGroups : ["安全组"]
    if (notifyEmail) {
      for (const group of groups) {
        records.push({
          id: `nfy_${Math.random().toString(36).slice(2, 10)}`,
          alarm_id: alarmID,
          channel: "email",
          receiver_group: group,
          status: "sent",
          created_at: now,
        })
      }
    }
    if (notifyWhatsApp) {
      for (const group of groups) {
        records.push({
          id: `nfy_${Math.random().toString(36).slice(2, 10)}`,
          alarm_id: alarmID,
          channel: "whatsapp",
          receiver_group: group,
          status: "sent",
          created_at: now,
        })
      }
    }
    if (records.length > 0) {
      setNotificationLogs((current) => [...records, ...current].slice(0, 50))
    }
  }

  async function applyStatus(alarmID: string) {
    const status = pendingStatus[alarmID]
    if (!status) {
      return
    }
    setUpdatingAlarmID(alarmID)
    setError("")
    try {
      await updateAlarmStatusMutation.mutateAsync({ alarmID, status })
      pushNotificationLogs(alarmID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新告警状态失败"
      setError(message)
    } finally {
      setUpdatingAlarmID(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">告警处置</p>
        <h1 className="mp-page-title">
          {platformViewer ? "实时告警与事件处置" : buildingAdmin ? "楼宇告警与事件处置" : "组织告警与事件处置"}
        </h1>
        <p className="mp-page-description">
          {buildingAdmin
            ? "聚焦负责楼宇内的告警确认、调查、缓解、升级、误报和关闭流程。"
            : "支持确认、调查、缓解、升级、误报、关闭等多维处置流程。"}
        </p>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何告警记录，避免误处置非本楼宇事件。
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          当前仅处置负责楼宇范围内的告警；可先在事件页定位上下文，再回到这里完成确认、升级或关闭。
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>待处理告警</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : stats.open} <BellRingIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">等待首次响应。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>严重告警</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : stats.critical} <SirenIcon className="size-4 text-red-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">需要立即处置。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>调查中</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : stats.investigating} <TriangleAlertIcon className="size-4 text-sky-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">已进入处置流程。</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">告警筛选</CardTitle>
          <CardDescription>
            {platformViewer
              ? "支持按租户线索、状态、等级和关键字组合定位。"
              : buildingAdmin
                ? "支持按状态、等级和关键字快速定位负责楼宇内告警。"
                : "支持按状态、等级和关键字快速定位告警。"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 md:grid-cols-[1fr_220px_220px_auto]">
            <div className="relative">
              <FilterIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                className="pl-8"
                placeholder={platformViewer ? "按告警ID/租户/楼宇/区域/门点/类型搜索" : "按告警ID/楼宇/区域/门点/类型搜索"}
              />
            </div>
            <Select
              value={statusFilter}
              onValueChange={(value: "all" | Alarm["status"]) => {
                setStatusFilter(value)
              }}
            >
              <SelectTrigger aria-label="告警状态筛选">
                <SelectValue placeholder="筛选状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="open">待处理</SelectItem>
                <SelectItem value="acknowledged">已确认</SelectItem>
                <SelectItem value="investigating">调查中</SelectItem>
                <SelectItem value="mitigated">已缓解</SelectItem>
                <SelectItem value="escalated">已升级</SelectItem>
                <SelectItem value="resolved">已关闭</SelectItem>
                <SelectItem value="false_positive">误报</SelectItem>
              </SelectContent>
            </Select>
            <Select
              value={severityFilter}
              onValueChange={(value: "all" | Alarm["severity"]) => {
                setSeverityFilter(value)
              }}
            >
              <SelectTrigger aria-label="告警等级筛选">
                <SelectValue placeholder="筛选等级" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部等级</SelectItem>
                <SelectItem value="critical">严重</SelectItem>
                <SelectItem value="high">高</SelectItem>
                <SelectItem value="medium">中</SelectItem>
                <SelectItem value="low">低</SelectItem>
              </SelectContent>
            </Select>
            <Button
              variant="outline"
              onClick={() => {
                setQuery("")
                setStatusFilter("all")
                setSeverityFilter("all")
              }}
            >
              重置筛选
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">告警通知策略</CardTitle>
          <CardDescription>
            {buildingAdmin
              ? "默认接收组为安全组；楼宇管理员可叠加负责楼宇的值守组，并支持邮件与 WhatsApp 双通道。"
              : "默认接收组为安全组；可叠加其他组，并支持邮件与 WhatsApp 双通道。"}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-2 md:grid-cols-[1fr_auto_auto_auto]">
            <Select value={notifyGroupDraft} onValueChange={setNotifyGroupDraft}>
              <SelectTrigger>
                <SelectValue placeholder="选择可通知用户组" />
              </SelectTrigger>
              <SelectContent>
                {notifyGroupOptions.map((group) => (
                  <SelectItem key={group} value={group}>
                    {group}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                addNotifyGroup(notifyGroupDraft)
              }}
            >
              新增组
            </Button>
            <Button
              type="button"
              variant={notifyEmail ? "default" : "outline"}
              onClick={() => {
                setNotifyEmail((current) => !current)
              }}
            >
              <MailIcon className="mr-1.5 size-4" />
              邮件
            </Button>
            <Button
              type="button"
              variant={notifyWhatsApp ? "default" : "outline"}
              onClick={() => {
                setNotifyWhatsApp((current) => !current)
              }}
            >
              <MessageCircleIcon className="mr-1.5 size-4" />
              WhatsApp
            </Button>
          </div>
          <div className="flex flex-wrap gap-2">
            {notifyGroups.map((group) => (
              <Badge key={group} variant="secondary" className="inline-flex items-center gap-1">
                {group}
                <button
                  type="button"
                  className="rounded p-0.5 hover:bg-muted-foreground/10"
                  onClick={() => {
                    removeNotifyGroup(group)
                  }}
                >
                  <XIcon className="size-3" />
                </button>
              </Badge>
            ))}
          </div>
          <p className="mp-kpi-note">
            说明：当前为系统内通知队列（sent）验证，后续可对接真实邮件/WhatsApp Provider。
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">告警队列</CardTitle>
          <CardDescription>
            {platformViewer
              ? `按租户、区域、门点定位并执行状态流转（当前命中 ${filteredAlarms.length} 条）。`
              : `按区域和门点定位并执行状态流转（当前命中 ${filteredAlarms.length} 条）。`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error || queryError ? (
            <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error || queryError}
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>告警 ID</TableHead>
                {platformViewer ? <TableHead>租户</TableHead> : null}
                <TableHead>类型</TableHead>
                <TableHead>位置</TableHead>
                <TableHead>等级</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>发现时间</TableHead>
                <TableHead className="text-right">处置</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 8 : 7} className="py-10 text-center text-muted-foreground">
                    正在加载告警...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredAlarms.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 8 : 7} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? "当前楼宇管理员尚未分配楼宇范围。"
                      : hasActiveFilters
                        ? "当前筛选条件下没有匹配的告警。"
                      : buildingAdmin
                        ? "当前楼宇范围内暂无告警。"
                        : "当前范围内暂无告警。"}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                filteredAlarms.map((alarm) => (
                  <TableRow key={alarm.id}>
                    <TableCell className="font-medium">
                      <TableCellText className="max-w-[12rem]">{alarm.id}</TableCellText>
                    </TableCell>
                    {platformViewer ? (
                      <TableCell>
                        <TableCellText className="max-w-[13rem]">
                          {tenantByID.get(alarm.tenant_id)?.name ?? alarm.tenant_id}
                        </TableCellText>
                      </TableCell>
                    ) : null}
                    <TableCell>
                      <TableCellText className="max-w-[12rem]">{alarm.type}</TableCellText>
                    </TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[16rem]">{alarm.location}</TableCellText>
                    </TableCell>
                    <TableCell>
                      <Badge variant={severityVariant(alarm.severity)}>
                        {severityLabel(alarm.severity)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(alarm.status)}>
                        {statusLabel(alarm.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>{new Date(alarm.created_at).toLocaleString("zh-CN")}</TableCell>
                    <TableCell className="text-right">
                      <div className="inline-flex items-center gap-2">
                        <Select
                          value={pendingStatus[alarm.id] ?? alarm.status}
                          onValueChange={(value) => {
                            setPendingStatus((current) => ({
                              ...current,
                              [alarm.id]: value as AlarmWorkflowStatus,
                            }))
                          }}
                        >
                          <SelectTrigger className="w-[150px]">
                            <SelectValue placeholder="处置状态" />
                          </SelectTrigger>
                          <SelectContent>
                            {statusOptions.map((status) => (
                              <SelectItem key={status} value={status}>
                                {statusLabel(status)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={updatingAlarmID === alarm.id}
                          onClick={() => {
                            void applyStatus(alarm.id)
                          }}
                        >
                          更新
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">通知日志</CardTitle>
          <CardDescription>记录告警状态更新后的通知发送轨迹。</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间</TableHead>
                <TableHead>告警</TableHead>
                <TableHead>通道</TableHead>
                <TableHead>接收组</TableHead>
                <TableHead>状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {notificationLogs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-6 text-center text-muted-foreground">
                    暂无通知记录，更新任一告警状态后会自动写入。
                  </TableCell>
                </TableRow>
              ) : null}
              {notificationLogs.map((item) => (
                <TableRow key={item.id}>
                  <TableCell>{new Date(item.created_at).toLocaleString("zh-CN")}</TableCell>
                  <TableCell>{item.alarm_id}</TableCell>
                  <TableCell>
                    <div className="inline-flex items-center gap-1.5">
                      {item.channel === "email" ? <MailIcon className="size-3.5" /> : <MessageCircleIcon className="size-3.5" />}
                      {item.channel}
                    </div>
                  </TableCell>
                  <TableCell>{item.receiver_group}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{item.status}</Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
