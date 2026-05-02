import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ArrowUpDownIcon, BellRingIcon, CheckCircle2Icon, ClockIcon, FilterIcon, MailIcon, MessageCircleIcon, ShieldAlertIcon, SirenIcon, SlidersHorizontalIcon, TriangleAlertIcon, XIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ListPagination } from "@/components/ui/list-pagination"
import { StatusDot } from "@/components/mistyislet/primitives"
import {
  consumeServerSentEvents,
  getAlertPolicy,
  listAlarms,
  listAlertPolicies,
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

const DEFAULT_NOTIFY_GROUP = "__default_security_group__"

type AlarmQueueGroupID = "open" | "investigating" | "mitigated" | "closed"

const alarmQueueGroups: Array<{
  id: AlarmQueueGroupID
  statusSet: Array<Alarm["status"]>
}> = [
  { id: "open", statusSet: ["open", "acknowledged"] },
  { id: "investigating", statusSet: ["investigating", "escalated"] },
  { id: "mitigated", statusSet: ["mitigated"] },
  { id: "closed", statusSet: ["resolved", "false_positive"] },
]

const severityOrder: Record<Alarm["severity"], number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
}

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

function severityLabel(severity: Alarm["severity"], t: (key: string) => string) {
  switch (severity) {
    case "critical":
      return t("alarms.severity.critical")
    case "high":
      return t("alarms.severity.high")
    case "medium":
      return t("alarms.severity.medium")
    case "low":
      return t("alarms.severity.low")
    default:
      return severity
  }
}

function statusLabel(status: Alarm["status"], t: (key: string) => string) {
  switch (status) {
    case "open":
      return t("alarms.status.open")
    case "acknowledged":
      return t("alarms.status.acknowledged")
    case "investigating":
      return t("alarms.status.investigating")
    case "mitigated":
      return t("alarms.status.mitigated")
    case "escalated":
      return t("alarms.status.escalated")
    case "resolved":
      return t("alarms.status.resolved")
    case "false_positive":
      return t("alarms.status.falsePositive")
    default:
      return status
  }
}

function nextAlarmAction(status: Alarm["status"]): AlarmWorkflowStatus | null {
  switch (status) {
    case "open":
      return "acknowledged"
    case "acknowledged":
      return "investigating"
    case "investigating":
    case "escalated":
      return "mitigated"
    case "mitigated":
      return "resolved"
    default:
      return null
  }
}

function nextAlarmActionLabel(status: AlarmWorkflowStatus, t: (key: string) => string) {
  switch (status) {
    case "acknowledged":
      return t("alarms.queue.nextAction.acknowledge")
    case "investigating":
      return t("alarms.queue.nextAction.investigate")
    case "mitigated":
      return t("alarms.queue.nextAction.mitigate")
    case "resolved":
      return t("alarms.queue.nextAction.close")
    default:
      return statusLabel(status, t)
  }
}

function sortAlarmsForQueue(a: Alarm, b: Alarm) {
  const severityDelta = severityOrder[a.severity] - severityOrder[b.severity]
  if (severityDelta !== 0) {
    return severityDelta
  }
  return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
}

function formatPolicySeconds(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${Math.round(seconds / 3600)}h`
}

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
  hasNextPage: boolean
}

async function loadAlarmsPageData(args: {
  token: string
  platformViewer: boolean
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerBuildingIDs: Set<string>
  page: number
  limit: number
}): Promise<AlarmsPageData> {
  const [alarmItems, groupItems, tenantItems] = await Promise.all([
    listAlarms(args.token, { page: args.page, limit: args.limit }),
    listUserGroups(args.token),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
  ])
  const hasNextPage = alarmItems.length === args.limit
  const scopedAlarms = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? alarmItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : alarmItems

  return {
    alarms: scopedAlarms,
    tenants: tenantItems,
    userGroups: groupItems,
    hasNextPage,
  }
}

export function AlarmsPage({ token, viewer }: AlarmsPageProps) {
  const { t, i18n } = useTranslation()
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
  const [notifyGroups, setNotifyGroups] = useState<string[]>([DEFAULT_NOTIFY_GROUP])
  const [notifyGroupDraft, setNotifyGroupDraft] = useState(DEFAULT_NOTIFY_GROUP)
  const [notifyEmail, setNotifyEmail] = useState(true)
  const [notifyWhatsApp, setNotifyWhatsApp] = useState(true)
  const [notificationLogs, setNotificationLogs] = useState<NotificationLog[]>([])
  const [query, setQuery] = useState("")
  const [statusFilter, setStatusFilter] = useState<"all" | Alarm["status"]>("all")
  const [severityFilter, setSeverityFilter] = useState<"all" | Alarm["severity"]>("all")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)
  const [notificationPage, setNotificationPage] = useState(1)
  const [notificationPageSize, setNotificationPageSize] = useState(25)
  const [notificationSorting, setNotificationSorting] = useState<SortingState>([])
  const [notificationColumnVisibility, setNotificationColumnVisibility] = useState<VisibilityState>({})
  const alarmsQueryKey = useMemo(
    () =>
      [
        "alarms-page",
        viewer.id,
        platformViewer,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingScopeKey,
        page,
        pageSize,
      ] as const,
    [viewer.id, platformViewer, buildingAdmin, missingBuildingScope, viewerBuildingScopeKey, page, pageSize]
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
        page,
        limit: pageSize,
      }),
    staleTime: 30 * 1000,
  })

  const alarms = alarmsQuery.data?.alarms ?? []
  const tenants = alarmsQuery.data?.tenants ?? []
  const userGroups = alarmsQuery.data?.userGroups ?? []
  const hasNextPage = alarmsQuery.data?.hasNextPage ?? false
  const loading = alarmsQuery.isPending
  const queryError = alarmsQuery.isError && alarmsQuery.error instanceof Error ? alarmsQuery.error.message : ""
  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])

  // --- Alert Policies ---
  const [selectedPolicyID, setSelectedPolicyID] = useState<string | null>(null)
  const alertPoliciesQuery = useQuery({
    queryKey: ["reference-alert-policies", viewer.id, viewer.role],
    queryFn: () => listAlertPolicies(token, { sort: "name" }),
    staleTime: 60 * 1000,
  })
  const alertPolicies = alertPoliciesQuery.data ?? []
  const alertPolicyDetailQuery = useQuery({
    queryKey: ["alert-policy-detail", selectedPolicyID],
    queryFn: () => getAlertPolicy(token, selectedPolicyID!),
    enabled: Boolean(selectedPolicyID),
    staleTime: 30 * 1000,
  })
  const policyDetail = alertPolicyDetailQuery.data ?? null

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
  const renderNotifyGroupLabel = (group: string) =>
    group === DEFAULT_NOTIFY_GROUP ? t("alarms.notificationPolicy.defaultGroupLabel") : group
  const groupedAlarmQueues = useMemo(
    () =>
      alarmQueueGroups.map((group) => ({
        ...group,
        alarms: filteredAlarms
          .filter((alarm) => group.statusSet.includes(alarm.status))
          .slice()
          .sort(sortAlarmsForQueue),
      })),
    [filteredAlarms]
  )
  const notificationMaxPage = Math.max(1, Math.ceil(notificationLogs.length / notificationPageSize))
  const notificationColumns = useMemo<ColumnDef<NotificationLog>[]>(
    () => [
      {
        id: "created_at",
        accessorKey: "created_at",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("alarms.notificationLogs.table.time")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => new Date(row.original.created_at).toLocaleString(i18n.language),
      },
      {
        id: "alarm_id",
        accessorKey: "alarm_id",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("alarms.notificationLogs.table.alarm")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => row.original.alarm_id,
      },
      {
        id: "channel",
        accessorKey: "channel",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("alarms.notificationLogs.table.channel")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <div className="inline-flex items-center gap-1.5">
            {row.original.channel === "email" ? <MailIcon className="size-3.5" /> : <MessageCircleIcon className="size-3.5" />}
            {row.original.channel === "email"
              ? t("alarms.notificationPolicy.channel.email")
              : t("alarms.notificationPolicy.channel.whatsApp")}
          </div>
        ),
      },
      {
        id: "receiver_group",
        accessorKey: "receiver_group",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("alarms.notificationLogs.table.receiverGroup")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => renderNotifyGroupLabel(row.original.receiver_group),
      },
      {
        id: "status",
        accessorKey: "status",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("alarms.notificationLogs.table.status")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <Badge variant="outline">
            {row.original.status === "sent"
              ? t("alarms.notificationPolicy.deliveryStatus.sent")
              : t("alarms.notificationPolicy.deliveryStatus.queued")}
          </Badge>
        ),
      },
    ],
    [t, i18n.language]
  )
  const notificationTable = useReactTable({
    columns: notificationColumns,
    data: notificationLogs,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility: notificationColumnVisibility,
      pagination: {
        pageIndex: Math.max(0, notificationPage - 1),
        pageSize: notificationPageSize,
      },
      sorting: notificationSorting,
    },
    onColumnVisibilityChange: setNotificationColumnVisibility,
    onSortingChange: setNotificationSorting,
  })
  const visibleNotificationColumnCount = notificationTable.getVisibleLeafColumns().length
  const notificationToggleableColumns = notificationTable.getAllLeafColumns().filter((column) => column.getCanHide())
  const notificationColumnLabels: Record<string, string> = {
    alarm_id: t("alarms.notificationLogs.table.alarm"),
    channel: t("alarms.notificationLogs.table.channel"),
    created_at: t("alarms.notificationLogs.table.time"),
    receiver_group: t("alarms.notificationLogs.table.receiverGroup"),
    status: t("alarms.notificationLogs.table.status"),
  }
  const hasActiveFilters = query.trim().length > 0 || statusFilter !== "all" || severityFilter !== "all"

  useEffect(() => {
    setPage(1)
  }, [pageSize, query, severityFilter, statusFilter])

  useEffect(() => {
    setNotificationPage(1)
  }, [notificationLogs, notificationPageSize])

  useEffect(() => {
    if (notificationPage > notificationMaxPage) {
      setNotificationPage(notificationMaxPage)
    }
  }, [notificationMaxPage, notificationPage])

  useEffect(() => {
    const controller = new AbortController()
    void consumeServerSentEvents({
      path: "/api/v1/alarms/stream",
      token,
      signal: controller.signal,
      onEvent: (message) => {
        if (message.event !== "update") {
          return
        }
        void queryClient.invalidateQueries({ queryKey: ["alarms-page"] })
      },
    })
    return () => {
      controller.abort()
    }
  }, [queryClient, token])

  const notifyGroupOptions = useMemo(() => {
    const base = userGroups.map((item) => item.name).filter(Boolean)
    return Array.from(new Set([DEFAULT_NOTIFY_GROUP, ...base]))
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
        return [DEFAULT_NOTIFY_GROUP]
      }
      return next
    })
  }

  function pushNotificationLogs(alarmID: string) {
    const records: NotificationLog[] = []
    const now = new Date().toISOString()
    const groups = notifyGroups.length > 0 ? notifyGroups : [DEFAULT_NOTIFY_GROUP]
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

  async function applyStatus(alarmID: string, status: AlarmWorkflowStatus) {
    setUpdatingAlarmID(alarmID)
    setError("")
    try {
      await updateAlarmStatusMutation.mutateAsync({ alarmID, status })
      pushNotificationLogs(alarmID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("alarms.error.updateStatusFailed")
      setError(message)
    } finally {
      setUpdatingAlarmID(null)
    }
  }

  function renderAlarmCard(alarm: Alarm) {
    const nextStatus = nextAlarmAction(alarm.status)
    const tenantLabel = tenantByID.get(alarm.tenant_id)?.name ?? alarm.tenant_id

    return (
      <Card key={alarm.id} data-testid="alarm-card">
        <CardContent className="p-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="min-w-0 space-y-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="break-all font-medium">{alarm.id}</span>
                <Badge variant={severityVariant(alarm.severity)}>
                  {severityLabel(alarm.severity, t)}
                </Badge>
                <Badge variant={statusVariant(alarm.status)}>
                  {statusLabel(alarm.status, t)}
                </Badge>
              </div>
              <div className="min-w-0">
                <p className="break-words text-sm font-medium">{alarm.type}</p>
                <p className="mt-1 break-words text-sm text-muted-foreground">{alarm.location}</p>
              </div>
              <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2 xl:grid-cols-4">
                {platformViewer ? (
                  <div className="min-w-0">
                    <p className="mp-kpi-note">{t("alarms.queue.fields.tenant")}</p>
                    <p className="mt-1 break-all text-foreground">{tenantLabel}</p>
                  </div>
                ) : null}
                <div className="min-w-0">
                  <p className="mp-kpi-note">{t("alarms.queue.fields.building")}</p>
                  <p className="mt-1 break-all text-foreground">{alarm.building_id}</p>
                </div>
                <div className="min-w-0">
                  <p className="mp-kpi-note">{t("alarms.queue.fields.area")}</p>
                  <p className="mt-1 break-all text-foreground">{alarm.area_id}</p>
                </div>
                <div className="min-w-0">
                  <p className="mp-kpi-note">{t("alarms.queue.fields.door")}</p>
                  <p className="mt-1 break-all text-foreground">{alarm.door_id}</p>
                </div>
                <div className="min-w-0">
                  <p className="mp-kpi-note">{t("alarms.queue.fields.detectedAt")}</p>
                  <p className="mt-1 break-words text-foreground">
                    {new Date(alarm.created_at).toLocaleString(i18n.language)}
                  </p>
                </div>
              </div>
            </div>
            <div className="flex shrink-0 flex-col gap-2 sm:flex-row lg:flex-col">
              {nextStatus ? (
                <Button
                  type="button"
                  disabled={updatingAlarmID === alarm.id}
                  onClick={() => {
                    void applyStatus(alarm.id, nextStatus)
                  }}
                >
                  <CheckCircle2Icon className="mr-1.5 size-4" />
                  {nextAlarmActionLabel(nextStatus, t)}
                </Button>
              ) : (
                <Button type="button" variant="outline" disabled>
                  <CheckCircle2Icon className="mr-1.5 size-4" />
                  {t("alarms.queue.nextAction.done")}
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("alarms.header.eyebrow")}</p>
        <h1 className="mp-page-title">
          {platformViewer
            ? t("alarms.header.titlePlatform")
            : buildingAdmin
              ? t("alarms.header.titleBuildingAdmin")
              : t("alarms.header.titleTenant")}
        </h1>
        <p className="mp-page-description">
          {buildingAdmin
            ? t("alarms.header.descriptionBuildingAdmin")
            : t("alarms.header.descriptionDefault")}
        </p>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          {t("alarms.notice.missingScope")}
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          {t("alarms.notice.buildingScopeHint")}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("alarms.kpi.open.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : stats.open} <BellRingIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("alarms.kpi.open.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("alarms.kpi.critical.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : stats.critical} <SirenIcon className="size-4 text-red-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("alarms.kpi.critical.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("alarms.kpi.investigating.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : stats.investigating} <TriangleAlertIcon className="size-4 text-sky-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("alarms.kpi.investigating.note")}</CardContent>
        </Card>
      </div>

      <Tabs defaultValue="queue" className="space-y-4">
        <TabsList className="grid h-auto w-full grid-cols-3 bg-transparent sm:w-fit">
          <TabsTrigger value="queue" className="py-2.5">
            {t("alarms.tabs.queue")}
          </TabsTrigger>
          <TabsTrigger value="policies" className="py-2.5">
            {t("alarms.tabs.policies", "Alert Policies")}
          </TabsTrigger>
          <TabsTrigger value="notifications" className="py-2.5">
            {t("alarms.tabs.notifications")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="queue" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("alarms.filter.title")}</CardTitle>
              <CardDescription>
                {platformViewer
                  ? t("alarms.filter.descriptionPlatform")
                  : buildingAdmin
                    ? t("alarms.filter.descriptionBuildingAdmin")
                    : t("alarms.filter.descriptionDefault")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,220px)_minmax(0,220px)_auto]">
                <div className="relative">
                  <FilterIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                  <Input
                    value={query}
                    onChange={(event) => {
                      setQuery(event.target.value)
                      setPage(1)
                    }}
                    className="pl-8"
                    aria-label={platformViewer ? t("alarms.filter.searchPlaceholderPlatform") : t("alarms.filter.searchPlaceholderDefault")}
                    placeholder={platformViewer ? t("alarms.filter.searchPlaceholderPlatform") : t("alarms.filter.searchPlaceholderDefault")}
                  />
                </div>
                <Select
                  value={statusFilter}
                  onValueChange={(value) => {
                    setStatusFilter(value as "all" | Alarm["status"])
                    setPage(1)
                  }}
                >
                  <SelectTrigger aria-label={t("alarms.filter.statusAriaLabel")}>
                    <SelectValue placeholder={t("alarms.filter.statusPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("alarms.filter.allStatuses")}</SelectItem>
                    <SelectItem value="open">{statusLabel("open", t)}</SelectItem>
                    <SelectItem value="acknowledged">{statusLabel("acknowledged", t)}</SelectItem>
                    <SelectItem value="investigating">{statusLabel("investigating", t)}</SelectItem>
                    <SelectItem value="mitigated">{statusLabel("mitigated", t)}</SelectItem>
                    <SelectItem value="escalated">{statusLabel("escalated", t)}</SelectItem>
                    <SelectItem value="resolved">{statusLabel("resolved", t)}</SelectItem>
                    <SelectItem value="false_positive">{statusLabel("false_positive", t)}</SelectItem>
                  </SelectContent>
                </Select>
                <Select
                  value={severityFilter}
                  onValueChange={(value) => {
                    setSeverityFilter(value as "all" | Alarm["severity"])
                    setPage(1)
                  }}
                >
                  <SelectTrigger aria-label={t("alarms.filter.severityAriaLabel")}>
                    <SelectValue placeholder={t("alarms.filter.severityPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("alarms.filter.allSeverities")}</SelectItem>
                    <SelectItem value="critical">{severityLabel("critical", t)}</SelectItem>
                    <SelectItem value="high">{severityLabel("high", t)}</SelectItem>
                    <SelectItem value="medium">{severityLabel("medium", t)}</SelectItem>
                    <SelectItem value="low">{severityLabel("low", t)}</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  variant="outline"
                  className="w-full lg:w-auto"
                  onClick={() => {
                    setQuery("")
                    setStatusFilter("all")
                    setSeverityFilter("all")
                    setPage(1)
                  }}
                >
                  {t("alarms.filter.reset")}
                </Button>
              </div>
            </CardContent>
          </Card>

          <div className="space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <div className="space-y-1">
                <h2 className="text-base font-medium">{t("alarms.list.title")}</h2>
                <p className="text-sm text-muted-foreground">
                  {platformViewer
                    ? t("alarms.list.matchedPlatform", { count: filteredAlarms.length })
                    : t("alarms.list.matchedDefault", { count: filteredAlarms.length })}
                </p>
              </div>
              <ListPagination
                page={page}
                onPageChange={setPage}
                pageSize={pageSize}
                onPageSizeChange={setPageSize}
                hasNextPage={hasNextPage}
                disabled={loading}
              />
            </div>

            {error || queryError ? (
              <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error || queryError}
              </div>
            ) : null}

            {loading ? (
              <div className="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
                {t("alarms.list.loading")}
              </div>
            ) : null}

            {!loading && filteredAlarms.length === 0 ? (
              <div className="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
                {missingBuildingScope
                  ? t("alarms.list.empty.missingScope")
                  : hasActiveFilters
                    ? t("alarms.list.empty.filtered")
                    : buildingAdmin
                      ? t("alarms.list.empty.buildingScope")
                      : t("alarms.list.empty.default")}
              </div>
            ) : null}

            {!loading && filteredAlarms.length > 0
              ? groupedAlarmQueues.map((group) => (
                  <section key={group.id} className="space-y-3">
                    <div className="flex flex-col gap-2 border-b pb-2 sm:flex-row sm:items-start sm:justify-between">
                      <div className="min-w-0">
                        <h3 className="text-sm font-medium">{t(`alarms.queue.groups.${group.id}.title`)}</h3>
                        <p className="mt-1 text-sm text-muted-foreground">
                          {t(`alarms.queue.groups.${group.id}.description`)}
                        </p>
                      </div>
                      <Badge variant="outline">{t("alarms.queue.groupCount", { count: group.alarms.length })}</Badge>
                    </div>
                    {group.alarms.length > 0 ? (
                      <div className="grid gap-3">
                        {group.alarms.map((alarm) => renderAlarmCard(alarm))}
                      </div>
                    ) : (
                      <div className="rounded-lg border border-dashed px-4 py-6 text-sm text-muted-foreground">
                        {t(`alarms.queue.groups.${group.id}.empty`)}
                      </div>
                    )}
                  </section>
                ))
              : null}
          </div>
        </TabsContent>

        <TabsContent value="policies" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("alarms.alertPolicies.title", "Alert Policies")}</CardTitle>
              <CardDescription>
                {t("alarms.alertPolicies.description", "Automated rules that generate alarms when specific conditions are met.")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {alertPoliciesQuery.isPending ? (
                <div className="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
                  {t("alarms.alertPolicies.loading", "Loading alert policies...")}
                </div>
              ) : alertPoliciesQuery.isError ? (
                <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {alertPoliciesQuery.error instanceof Error ? alertPoliciesQuery.error.message : t("alarms.alertPolicies.error", "Failed to load alert policies")}
                </div>
              ) : alertPolicies.length === 0 ? (
                <div className="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
                  {t("alarms.alertPolicies.empty", "No alert policies configured.")}
                </div>
              ) : (
                <div className="divide-y rounded-lg border">
                  {alertPolicies.map((policy) => (
                    <button
                      key={policy.id}
                      type="button"
                      className="flex w-full items-center justify-between gap-4 px-4 py-3 text-left transition-colors hover:bg-muted/50"
                      onClick={() => setSelectedPolicyID(policy.id)}
                    >
                      <div className="min-w-0 space-y-1">
                        <div className="flex items-center gap-2">
                          <ShieldAlertIcon className="size-4 shrink-0 text-muted-foreground" />
                          <span className="truncate text-sm font-medium">{policy.name}</span>
                          <Badge variant={policy.severity === "critical" || policy.severity === "high" ? "destructive" : "outline"} className="shrink-0">
                            {policy.severity}
                          </Badge>
                        </div>
                        {policy.description ? (
                          <p className="truncate text-sm text-muted-foreground">{policy.description}</p>
                        ) : null}
                      </div>
                      <StatusDot
                        tone={policy.enabled && policy.status === "active" ? "success" : "warning"}
                        label={policy.enabled && policy.status === "active" ? t("alarms.alertPolicies.enabled", "Enabled") : t("alarms.alertPolicies.disabled", "Disabled")}
                      />
                    </button>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <Sheet open={Boolean(selectedPolicyID)} onOpenChange={(open) => { if (!open) setSelectedPolicyID(null) }}>
          <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
            <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
              <SheetTitle>{policyDetail?.name ?? t("alarms.alertPolicies.detail.title", "Policy Details")}</SheetTitle>
              <SheetDescription>
                {policyDetail?.description || t("alarms.alertPolicies.detail.noDescription", "No description")}
              </SheetDescription>
            </SheetHeader>
            <div className="space-y-5 px-6 py-5">
              {alertPolicyDetailQuery.isPending ? (
                <p className="text-sm text-muted-foreground">{t("alarms.alertPolicies.detail.loading", "Loading...")}</p>
              ) : alertPolicyDetailQuery.isError ? (
                <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {alertPolicyDetailQuery.error instanceof Error ? alertPolicyDetailQuery.error.message : t("alarms.alertPolicies.detail.error", "Failed to load policy")}
                </div>
              ) : policyDetail ? (
                <>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.status", "Status")}</p>
                    <StatusDot
                      tone={policyDetail.enabled && policyDetail.status === "active" ? "success" : "warning"}
                      label={policyDetail.enabled && policyDetail.status === "active" ? t("alarms.alertPolicies.enabled", "Enabled") : t("alarms.alertPolicies.disabled", "Disabled")}
                    />
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.category", "Category")}</p>
                    <p className="text-sm">{policyDetail.category}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.trigger", "Trigger")}</p>
                    <p className="text-sm">{policyDetail.trigger}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.severity", "Severity")}</p>
                    <Badge variant={policyDetail.severity === "critical" || policyDetail.severity === "high" ? "destructive" : "outline"}>
                      {policyDetail.severity}
                    </Badge>
                  </div>
                  {policyDetail.condition_expression ? (
                    <div className="space-y-1">
                      <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.condition", "Condition")}</p>
                      <code className="block rounded bg-muted/50 px-2 py-1.5 text-xs">{policyDetail.condition_expression}</code>
                    </div>
                  ) : null}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.threshold", "Threshold")}</p>
                      <p className="text-sm">{policyDetail.threshold}</p>
                    </div>
                    <div className="space-y-1">
                      <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.window", "Window")}</p>
                      <p className="text-sm">{formatPolicySeconds(policyDetail.window_seconds)}</p>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">
                      <ClockIcon className="mr-1 inline-block size-3.5" />
                      {t("alarms.alertPolicies.detail.cooldown", "Cooldown")}
                    </p>
                    <p className="text-sm">{formatPolicySeconds(policyDetail.cooldown_seconds)}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.channels", "Notification Channels")}</p>
                    <div className="flex gap-2">
                      {policyDetail.channels.email ? (
                        <Badge variant="secondary">
                          <MailIcon className="mr-1 size-3" />
                          {t("alarms.notificationPolicy.channel.email")}
                        </Badge>
                      ) : null}
                      {policyDetail.channels.whatsapp ? (
                        <Badge variant="secondary">
                          <MessageCircleIcon className="mr-1 size-3" />
                          {t("alarms.notificationPolicy.channel.whatsApp")}
                        </Badge>
                      ) : null}
                      {!policyDetail.channels.email && !policyDetail.channels.whatsapp ? (
                        <span className="text-sm text-muted-foreground">{t("alarms.alertPolicies.detail.noChannels", "No channels configured")}</span>
                      ) : null}
                    </div>
                  </div>
                  {policyDetail.receiver_groups && policyDetail.receiver_groups.length > 0 ? (
                    <div className="space-y-1">
                      <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.receiverGroups", "Receiver Groups")}</p>
                      <div className="flex flex-wrap gap-1.5">
                        {policyDetail.receiver_groups.map((group) => (
                          <Badge key={group} variant="outline">{group}</Badge>
                        ))}
                      </div>
                    </div>
                  ) : null}
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{t("alarms.alertPolicies.detail.lastUpdated", "Last Updated")}</p>
                    <p className="text-sm">{new Date(policyDetail.updated_at).toLocaleString(i18n.language)}</p>
                  </div>
                </>
              ) : null}
            </div>
          </SheetContent>
        </Sheet>

        <TabsContent value="notifications" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("alarms.notificationPolicy.title")}</CardTitle>
              <CardDescription>
                {buildingAdmin
                  ? t("alarms.notificationPolicy.descriptionBuildingAdmin")
                  : t("alarms.notificationPolicy.descriptionDefault")}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-[minmax(0,1fr)_auto_auto_auto]">
                <Select value={notifyGroupDraft} onValueChange={setNotifyGroupDraft}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("alarms.notificationPolicy.selectGroupPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {notifyGroupOptions.map((group) => (
                      <SelectItem key={group} value={group}>
                        {renderNotifyGroupLabel(group)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  type="button"
                  variant="outline"
                  className="w-full lg:w-auto"
                  onClick={() => {
                    addNotifyGroup(notifyGroupDraft)
                  }}
                >
                  {t("alarms.notificationPolicy.addGroup")}
                </Button>
                <Button
                  type="button"
                  variant={notifyEmail ? "default" : "outline"}
                  className="w-full lg:w-auto"
                  onClick={() => {
                    setNotifyEmail((current) => !current)
                  }}
                >
                  <MailIcon className="mr-1.5 size-4" />
                  {t("alarms.notificationPolicy.channel.email")}
                </Button>
                <Button
                  type="button"
                  variant={notifyWhatsApp ? "default" : "outline"}
                  className="w-full lg:w-auto"
                  onClick={() => {
                    setNotifyWhatsApp((current) => !current)
                  }}
                >
                  <MessageCircleIcon className="mr-1.5 size-4" />
                  {t("alarms.notificationPolicy.channel.whatsApp")}
                </Button>
              </div>
              <div className="flex flex-wrap gap-2">
                {notifyGroups.map((group) => (
                  <Badge key={group} variant="secondary" className="inline-flex items-center gap-1">
                    {renderNotifyGroupLabel(group)}
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
                {t("alarms.notificationPolicy.note")}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("alarms.notificationLogs.title")}</CardTitle>
              <CardDescription>{t("alarms.notificationLogs.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex justify-end">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button type="button" variant="outline" size="sm">
                      <SlidersHorizontalIcon className="mr-1.5 size-4" />
                      {t("alarms.notificationLogs.columnDisplay")}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {notificationToggleableColumns.map((column) => (
                      <DropdownMenuCheckboxItem
                        key={column.id}
                        checked={column.getIsVisible()}
                        onCheckedChange={(checked) => column.toggleVisibility(Boolean(checked))}
                      >
                        {notificationColumnLabels[column.id] || column.id}
                      </DropdownMenuCheckboxItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <ListPagination
                page={notificationPage}
                onPageChange={setNotificationPage}
                pageSize={notificationPageSize}
                onPageSizeChange={setNotificationPageSize}
                hasNextPage={notificationTable.getCanNextPage()}
                disabled={notificationLogs.length === 0}
              />
              <Table>
                <TableHeader>
                  {notificationTable.getHeaderGroups().map((headerGroup) => (
                    <TableRow key={headerGroup.id}>
                      {headerGroup.headers.map((header) => (
                        <TableHead key={header.id}>
                          {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                        </TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  {notificationLogs.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={visibleNotificationColumnCount} className="py-6 text-center text-muted-foreground">
                        {t("alarms.notificationLogs.empty")}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {notificationTable.getRowModel().rows.map((row) => (
                    <TableRow key={row.id}>
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
