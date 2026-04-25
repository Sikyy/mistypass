import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ArrowUpDownIcon, BellRingIcon, FilterIcon, MailIcon, MessageCircleIcon, SirenIcon, SlidersHorizontalIcon, TriangleAlertIcon, XIcon } from "lucide-react"

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
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { ListPagination } from "@/components/ui/list-pagination"
import {
  consumeServerSentEvents,
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

const DEFAULT_NOTIFY_GROUP = "__default_security_group__"

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
  const [pendingStatus, setPendingStatus] = useState<Record<string, AlarmWorkflowStatus>>({})
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
  const [alarmSorting, setAlarmSorting] = useState<SortingState>([])
  const [alarmColumnVisibility, setAlarmColumnVisibility] = useState<VisibilityState>({})
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
  const alarmColumns = useMemo<ColumnDef<Alarm>[]>(
    () => {
      const definition: ColumnDef<Alarm>[] = [
        {
          id: "id",
          accessorKey: "id",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.id")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem] font-medium">{row.original.id}</TableCellText>,
        },
        {
          id: "type",
          accessorKey: "type",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.type")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem]">{row.original.type}</TableCellText>,
        },
        {
          id: "location",
          accessorKey: "location",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.location")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[16rem]">{row.original.location}</TableCellText>,
        },
        {
          id: "severity",
          accessorKey: "severity",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.severity")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <Badge variant={severityVariant(row.original.severity)}>
              {severityLabel(row.original.severity, t)}
            </Badge>
          ),
        },
        {
          id: "status",
          accessorKey: "status",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.status")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <Badge variant={statusVariant(row.original.status)}>
              {statusLabel(row.original.status, t)}
            </Badge>
          ),
        },
        {
          id: "created_at",
          accessorKey: "created_at",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.createdAt")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => new Date(row.original.created_at).toLocaleString(i18n.language),
        },
        {
          id: "actions",
          header: () => <span className="block text-right">{t("alarms.table.actions")}</span>,
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => (
            <div className="inline-flex items-center gap-2">
              <Select
                value={pendingStatus[row.original.id] ?? row.original.status}
                onValueChange={(value) => {
                  setPendingStatus((current) => ({
                    ...current,
                    [row.original.id]: value as AlarmWorkflowStatus,
                  }))
                }}
              >
                <SelectTrigger className="w-[150px]">
                  <SelectValue placeholder={t("alarms.table.statusPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {statusOptions.map((status) => (
                    <SelectItem key={status} value={status}>
                      {statusLabel(status, t)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                disabled={updatingAlarmID === row.original.id}
                onClick={() => {
                  void applyStatus(row.original.id)
                }}
              >
                {t("alarms.table.update")}
              </Button>
            </div>
          ),
        },
      ]
      if (platformViewer) {
        definition.splice(1, 0, {
          id: "tenant",
          accessorFn: (row) => tenantByID.get(row.tenant_id)?.name ?? row.tenant_id,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("alarms.table.tenant")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[13rem]">{tenantByID.get(row.original.tenant_id)?.name ?? row.original.tenant_id}</TableCellText>,
        })
      }
      return definition
    },
    [pendingStatus, platformViewer, tenantByID, updatingAlarmID, t, i18n.language]
  )
  const alarmsTable = useReactTable({
    columns: alarmColumns,
    data: filteredAlarms,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility: alarmColumnVisibility,
      sorting: alarmSorting,
    },
    onColumnVisibilityChange: setAlarmColumnVisibility,
    onSortingChange: setAlarmSorting,
  })
  const visibleAlarmColumnCount = alarmsTable.getVisibleLeafColumns().length
  const alarmToggleableColumns = alarmsTable.getAllLeafColumns().filter((column) => column.getCanHide())
  const alarmColumnLabels: Record<string, string> = {
    created_at: t("alarms.table.createdAt"),
    id: t("alarms.table.id"),
    location: t("alarms.table.location"),
    severity: t("alarms.table.severity"),
    status: t("alarms.table.status"),
    tenant: t("alarms.table.tenant"),
    type: t("alarms.table.type"),
  }
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
      const message = err instanceof Error ? err.message : t("alarms.error.updateStatusFailed")
      setError(message)
    } finally {
      setUpdatingAlarmID(null)
    }
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
          <div className="grid gap-3 md:grid-cols-[1fr_220px_220px_auto]">
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
              onValueChange={(value: "all" | Alarm["status"]) => {
                setStatusFilter(value)
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
              onValueChange={(value: "all" | Alarm["severity"]) => {
                setSeverityFilter(value)
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
          <div className="grid gap-2 md:grid-cols-[1fr_auto_auto_auto]">
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
              onClick={() => {
                addNotifyGroup(notifyGroupDraft)
              }}
            >
              {t("alarms.notificationPolicy.addGroup")}
            </Button>
            <Button
              type="button"
              variant={notifyEmail ? "default" : "outline"}
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
          <CardTitle className="text-base">{t("alarms.list.title")}</CardTitle>
          <CardDescription>
            {platformViewer
              ? t("alarms.list.matchedPlatform", { count: filteredAlarms.length })
              : t("alarms.list.matchedDefault", { count: filteredAlarms.length })}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error || queryError ? (
            <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error || queryError}
            </div>
          ) : null}

          <div className="mb-3">
            <div className="mb-2 flex justify-end">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" size="sm">
                    <SlidersHorizontalIcon className="mr-1.5 size-4" />
                    {t("alarms.list.columnDisplay")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {alarmToggleableColumns.map((column) => (
                    <DropdownMenuCheckboxItem
                      key={column.id}
                      checked={column.getIsVisible()}
                      onCheckedChange={(checked) => column.toggleVisibility(Boolean(checked))}
                    >
                      {alarmColumnLabels[column.id] || column.id}
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
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

          <Table>
            <TableHeader>
              {alarmsTable.getHeaderGroups().map((headerGroup) => (
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
              {loading ? (
                <TableRow>
                  <TableCell colSpan={visibleAlarmColumnCount} className="py-10 text-center text-muted-foreground">
                    {t("alarms.list.loading")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredAlarms.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleAlarmColumnCount} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? t("alarms.list.empty.missingScope")
                      : hasActiveFilters
                        ? t("alarms.list.empty.filtered")
                      : buildingAdmin
                        ? t("alarms.list.empty.buildingScope")
                        : t("alarms.list.empty.default")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                alarmsTable.getRowModel().rows.map((row) => (
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
    </div>
  )
}
