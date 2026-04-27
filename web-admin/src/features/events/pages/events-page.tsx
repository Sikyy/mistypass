import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  type ColumnDef,
  type SortingState,
  type VisibilityState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  ArrowUpDownIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  ChevronUpIcon,
  FilterIcon,
  RadioTowerIcon,
  ShieldAlertIcon,
  SlidersHorizontalIcon,
} from "lucide-react"

import { EventDetailDrawer, type EventDetailDrawerEvent } from "@/components/events/event-detail-drawer"
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
  listAccessEvents,
  consumeServerSentEvents,
  listDeviceEvents,
  listTenants,
  type CurrentUser,
  type AccessEvent,
  type DeviceEvent,
  type Tenant,
} from "@/lib/api"
import {
  getViewerBuildingIDs,
  getViewerTenantID,
  isBuildingAdmin,
  isPlatformViewer,
} from "@/lib/viewer"

type EventsPageProps = {
  token: string
  viewer: CurrentUser
}

type EventType = "access_granted" | "access_denied" | "gateway_event"
type EventSource = "access" | "device"
type EventTimeRange = "24h" | "7d" | "all"

type EventRow = {
  id: string
  source: EventSource
  rawType: string
  tenantID: string
  buildingID: string
  areaID: string
  type: EventType
  actor: string
  door: string
  gateway: string
  detail: string
  result: "success" | "denied" | "warning" | string
  at: string
  raw: AccessEvent | DeviceEvent
}

function toAccessEventRow(item: AccessEvent): EventRow {
  return {
    id: item.id,
    source: "access",
    rawType: item.type,
    tenantID: item.tenant_id,
    buildingID: item.building_id,
    areaID: item.area_id,
    type: item.type as EventType,
    actor: item.actor,
    door: item.door_id,
    gateway: item.gateway_id,
    detail: "",
    result: item.result,
    at: item.at,
    raw: item,
  }
}

function toDeviceEventRow(item: DeviceEvent): EventRow {
  return {
    id: item.id,
    source: "device",
    rawType: item.type,
    tenantID: item.tenant_id,
    buildingID: item.building_id,
    areaID: "-",
    type: "gateway_event",
    actor: "gateway_agent",
    door: "-",
    gateway: item.gateway_id,
    detail: item.detail,
    result: item.result,
    at: item.at,
    raw: item,
  }
}

function resultVariant(result: EventRow["result"]): EventDetailDrawerEvent["resultVariant"] {
  switch (result) {
    case "success":
      return "outline"
    case "denied":
      return "destructive"
    default:
      return "secondary"
  }
}

function resultLabel(result: EventRow["result"], t: (key: string) => string) {
  switch (result) {
    case "success":
      return t("events.result.success")
    case "denied":
      return t("events.result.denied")
    case "warning":
      return t("events.result.warning")
    default:
      return result
  }
}

function eventTypeLabel(type: EventType, t: (key: string) => string) {
  switch (type) {
    case "access_granted":
      return t("events.type.accessGranted")
    case "access_denied":
      return t("events.type.accessDenied")
    case "gateway_event":
      return t("events.type.gatewayEvent")
    default:
      return type
  }
}

function timeRangeLabel(range: EventTimeRange, t: (key: string) => string) {
  switch (range) {
    case "24h":
      return t("events.filter.timeRange.last24h")
    case "7d":
      return t("events.filter.timeRange.last7d")
    default:
      return t("events.filter.timeRange.all")
  }
}

function eventTimestamp(row: EventRow) {
  const value = new Date(row.at).getTime()
  return Number.isNaN(value) ? 0 : value
}

function isWithinTimeRange(row: EventRow, range: EventTimeRange, latestTimestamp: number) {
  if (range === "all" || latestTimestamp <= 0) {
    return true
  }
  const windowMs = range === "24h" ? 24 * 60 * 60 * 1000 : 7 * 24 * 60 * 60 * 1000
  return eventTimestamp(row) >= latestTimestamp - windowMs
}

function isDeviceAnomaly(row: EventRow) {
  if (row.source !== "device") {
    return false
  }
  const signal = `${row.rawType} ${row.detail} ${row.result}`.toLowerCase()
  return row.result !== "success" || /offline|error|failed|failure|timeout|retry|lag|warning/.test(signal)
}

function isOfflineEvent(row: EventRow) {
  if (row.source !== "device") {
    return false
  }
  const signal = `${row.rawType} ${row.detail} ${row.result}`.toLowerCase()
  return signal.includes("offline")
}

type EventsPageData = {
  events: EventRow[]
  tenants: Tenant[]
  hasNextPage: boolean
}

async function loadEventsPageData(args: {
  token: string
  platformViewer: boolean
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerBuildingIDs: Set<string>
  page: number
  limit: number
}): Promise<EventsPageData> {
  const [accessEvents, deviceEvents, tenantItems] = await Promise.all([
    listAccessEvents(args.token, { page: args.page, limit: args.limit }),
    listDeviceEvents(args.token, { page: args.page, limit: args.limit }),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
  ])
  const hasNextPage = accessEvents.length === args.limit || deviceEvents.length === args.limit

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

  return {
    events: combined,
    tenants: tenantItems,
    hasNextPage,
  }
}

export function EventsPage({ token, viewer }: EventsPageProps) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
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
  const [timeRange, setTimeRange] = useState<EventTimeRange>("24h")
  const [typeFilter, setTypeFilter] = useState<"all" | EventType>("all")
  const [tenantFilter, setTenantFilter] = useState<"all" | string>(
    platformViewer ? "all" : viewerTenantID || "all"
  )
  const [showAdvancedFilters, setShowAdvancedFilters] = useState(false)
  const [selectedEvent, setSelectedEvent] = useState<EventRow | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(() => {
    const visibility: VisibilityState = { gateway: false }
    if (platformViewer) {
      visibility.scope = false
    }
    return visibility
  })
  const eventsQuery = useQuery({
    queryKey: [
      "events-page",
      viewer.id,
      platformViewer,
      buildingAdmin,
      missingBuildingScope,
      viewerBuildingScopeKey,
      page,
      pageSize,
    ],
    queryFn: () =>
      loadEventsPageData({
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

  const events = eventsQuery.data?.events ?? []
  const tenants = eventsQuery.data?.tenants ?? []
  const hasNextPage = eventsQuery.data?.hasNextPage ?? false
  const loading = eventsQuery.isPending
  const queryError =
    eventsQuery.isError && eventsQuery.error instanceof Error ? eventsQuery.error.message : ""

  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])

  useEffect(() => {
    if (platformViewer) {
      return
    }
    setTenantFilter(viewerTenantID || "all")
  }, [platformViewer, viewerTenantID])

  useEffect(() => {
    setPage(1)
  }, [pageSize, query, tenantFilter, timeRange, typeFilter])

  useEffect(() => {
    const controller = new AbortController()
    void consumeServerSentEvents({
      path: "/api/v1/events/stream",
      token,
      signal: controller.signal,
      onEvent: (message) => {
        if (message.event !== "update") {
          return
        }
        void queryClient.invalidateQueries({ queryKey: ["events-page"] })
      },
    })
    return () => {
      controller.abort()
    }
  }, [queryClient, token])

  const latestEventTimestamp = useMemo(
    () => events.reduce((latest, row) => Math.max(latest, eventTimestamp(row)), 0),
    [events]
  )

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return events.filter((row) => {
      const timeMatched = isWithinTimeRange(row, timeRange, latestEventTimestamp)
      const typeMatched = typeFilter === "all" || row.type === typeFilter
      const tenantMatched =
        !platformViewer || tenantFilter === "all" || row.tenantID === tenantFilter
      if (!timeMatched || !typeMatched || !tenantMatched) {
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
        row.detail.toLowerCase().includes(q) ||
        row.rawType.toLowerCase().includes(q) ||
        tenantName.toLowerCase().includes(q)
      )
    })
  }, [events, latestEventTimestamp, query, tenantByID, tenantFilter, timeRange, typeFilter, platformViewer])

  const kpiSource = filtered
  const accessEventCount = kpiSource.filter((item) => item.type === "access_granted" || item.type === "access_denied").length
  const deniedEventCount = kpiSource.filter((item) => item.type === "access_denied").length
  const deviceAnomalyCount = kpiSource.filter(isDeviceAnomaly).length
  const offlineEventCount = kpiSource.filter(isOfflineEvent).length

  const columns = useMemo<ColumnDef<EventRow>[]>(() => {
    const definition: ColumnDef<EventRow>[] = [
      {
        id: "id",
        accessorKey: "id",
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.id")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <TableCellText className="max-w-[12rem] font-medium">{row.original.id}</TableCellText>
        ),
      },
      {
        id: "type",
        accessorKey: "type",
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.type")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => eventTypeLabel(row.original.type, t),
      },
      {
        id: "actor",
        accessorKey: "actor",
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.actor")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[12rem]">{row.original.actor}</TableCellText>,
      },
      {
        id: "scope",
        accessorFn: (row) => `${row.buildingID} / ${row.areaID} / ${row.door}`,
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.scope")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <TableCellText className="max-w-[16rem]">
            {row.original.buildingID} / {row.original.areaID} / {row.original.door}
          </TableCellText>
        ),
      },
      {
        id: "gateway",
        accessorKey: "gateway",
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.gateway")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[10rem]">{row.original.gateway}</TableCellText>,
      },
      {
        id: "result",
        accessorKey: "result",
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.result")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <Badge variant={resultVariant(row.original.result)}>
            {resultLabel(row.original.result, t)}
          </Badge>
        ),
      },
      {
        id: "at",
        accessorKey: "at",
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.at")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => new Date(row.original.at).toLocaleString(i18n.language),
      },
    ]

    if (platformViewer) {
      definition.splice(1, 0, {
        id: "tenant",
        accessorFn: (row) => tenantByID.get(row.tenantID)?.name ?? row.tenantID,
        header: ({ column }) => (
          <Button
            variant="ghost"
            className="-ml-2 h-8 px-2"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            {t("events.table.tenant")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <TableCellText className="max-w-[13rem]">
            {tenantByID.get(row.original.tenantID)?.name ?? row.original.tenantID}
          </TableCellText>
        ),
      })
    }

    return definition
  }, [platformViewer, tenantByID, t, i18n.language])

  const table = useReactTable({
    columns,
    data: filtered,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility,
      sorting,
    },
    onColumnVisibilityChange: setColumnVisibility,
    onSortingChange: setSorting,
  })

  const visibleColumnCount = table.getVisibleLeafColumns().length
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const columnLabels: Record<string, string> = {
    actor: t("events.table.actor"),
    at: t("events.table.at"),
    gateway: t("events.table.gateway"),
    id: t("events.table.id"),
    result: t("events.table.result"),
    scope: t("events.table.scope"),
    tenant: t("events.table.tenant"),
    type: t("events.table.type"),
  }
  const hasActiveFilters =
    query.trim().length > 0 || timeRange !== "24h" || typeFilter !== "all" || (platformViewer && tenantFilter !== "all")
  const selectedEventDetail = useMemo<EventDetailDrawerEvent | null>(() => {
    if (!selectedEvent) {
      return null
    }
    return {
      actor: selectedEvent.actor,
      areaID: selectedEvent.areaID,
      at: selectedEvent.at,
      buildingID: selectedEvent.buildingID,
      detail: selectedEvent.detail,
      door: selectedEvent.door,
      gateway: selectedEvent.gateway,
      id: selectedEvent.id,
      raw: selectedEvent.raw,
      resultLabel: resultLabel(selectedEvent.result, t),
      resultVariant: resultVariant(selectedEvent.result),
      tenantLabel: tenantByID.get(selectedEvent.tenantID)?.name ?? selectedEvent.tenantID,
      typeLabel: eventTypeLabel(selectedEvent.type, t),
    }
  }, [selectedEvent, t, tenantByID])

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("events.header.eyebrow")}</p>
        <h1 className="mp-page-title">
          {platformViewer
            ? t("events.header.titlePlatform")
            : buildingAdmin
              ? t("events.header.titleBuildingAdmin")
              : t("events.header.titleTenant")}
        </h1>
        <p className="mp-page-description">
          {platformViewer
            ? t("events.header.descriptionPlatform")
            : buildingAdmin
              ? t("events.header.descriptionBuildingAdmin")
              : t("events.header.descriptionTenant")}
        </p>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          {t("events.notice.missingScope")}
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          {t("events.notice.buildingScopeHint")}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("events.kpi.accessVolume.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {accessEventCount}{" "}
              <CheckCircle2Icon className="size-4 text-emerald-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin
              ? t("events.kpi.accessVolume.noteBuildingAdmin")
              : t("events.kpi.accessVolume.noteDefault")}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("events.kpi.denied.title")}</CardDescription>
            <CardTitle className="text-2xl">
              {deniedEventCount}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("events.kpi.denied.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("events.kpi.deviceAnomaly.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {deviceAnomalyCount}{" "}
              <ShieldAlertIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin
              ? t("events.kpi.deviceAnomaly.noteBuildingAdmin")
              : t("events.kpi.deviceAnomaly.noteDefault")}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("events.kpi.offline.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {offlineEventCount}{" "}
              <RadioTowerIcon className="size-4 text-red-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin
              ? t("events.kpi.offline.noteBuildingAdmin")
              : t("events.kpi.offline.noteDefault")}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("events.filter.title")}</CardTitle>
          <CardDescription>
            {platformViewer
              ? t("events.filter.descriptionPlatform")
              : buildingAdmin
                ? t("events.filter.descriptionBuildingAdmin")
                : t("events.filter.descriptionTenant")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 lg:grid-cols-[minmax(0,180px)_minmax(0,220px)_minmax(16rem,1fr)_auto]">
            <Select
              value={timeRange}
              onValueChange={(value) => {
                setTimeRange(value as EventTimeRange)
                setPage(1)
              }}
            >
              <SelectTrigger aria-label={t("events.filter.timeRangeAriaLabel")}>
                <CalendarClockIcon className="mr-1.5 size-4 text-muted-foreground" />
                <SelectValue placeholder={t("events.filter.timeRangePlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="24h">{t("events.filter.timeRange.last24h")}</SelectItem>
                <SelectItem value="7d">{t("events.filter.timeRange.last7d")}</SelectItem>
                <SelectItem value="all">{t("events.filter.timeRange.all")}</SelectItem>
              </SelectContent>
            </Select>
            <Select
              value={typeFilter}
              onValueChange={(value) => {
                setTypeFilter(value as "all" | EventType)
                setPage(1)
              }}
            >
              <SelectTrigger aria-label={t("events.filter.typeAriaLabel")}>
                <SelectValue placeholder={t("events.filter.typePlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("events.filter.allTypes")}</SelectItem>
                <SelectItem value="access_granted">
                  {t("events.filter.typeOption.accessGranted")}
                </SelectItem>
                <SelectItem value="access_denied">
                  {t("events.filter.typeOption.accessDenied")}
                </SelectItem>
                <SelectItem value="gateway_event">
                  {t("events.filter.typeOption.gatewayEvent")}
                </SelectItem>
              </SelectContent>
            </Select>
            <div className="relative">
              <FilterIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value)
                  setPage(1)
                }}
                className="pl-8"
                aria-label={
                  platformViewer
                    ? t("events.filter.searchPlaceholderPlatform")
                    : t("events.filter.searchPlaceholderDefault")
                }
                placeholder={
                  platformViewer
                    ? t("events.filter.searchPlaceholderPlatform")
                    : t("events.filter.searchPlaceholderDefault")
                }
              />
            </div>
            <div className="grid gap-2 sm:grid-cols-2 lg:flex lg:flex-wrap">
              <Button
                variant="outline"
                className="w-full lg:w-auto"
                onClick={() => {
                  setQuery("")
                  setTimeRange("24h")
                  setTypeFilter("all")
                  if (platformViewer) {
                    setTenantFilter("all")
                  }
                  setPage(1)
                }}
              >
                {t("events.filter.reset")}
              </Button>
              {platformViewer ? (
                <Button
                  type="button"
                  variant="outline"
                  className="w-full lg:w-auto"
                  onClick={() => setShowAdvancedFilters((current) => !current)}
                  aria-expanded={showAdvancedFilters}
                >
                  {showAdvancedFilters ? (
                    <ChevronUpIcon className="mr-1.5 size-4" />
                  ) : (
                    <ChevronDownIcon className="mr-1.5 size-4" />
                  )}
                  {t("events.filter.advanced")}
                </Button>
              ) : null}
            </div>
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span>{t("events.filter.activeWindow", { window: timeRangeLabel(timeRange, t) })}</span>
            {platformViewer && tenantFilter !== "all" ? (
              <Badge variant="outline">{tenantByID.get(tenantFilter)?.name ?? tenantFilter}</Badge>
            ) : null}
          </div>
          {platformViewer && showAdvancedFilters ? (
            <div className="mt-4 rounded-xl border bg-muted/10 p-3">
              <div className="grid gap-3 lg:grid-cols-[minmax(0,220px)_minmax(0,1fr)]">
                <Select
                  value={tenantFilter}
                  onValueChange={(value) => {
                    setTenantFilter(value)
                    setPage(1)
                  }}
                >
                  <SelectTrigger aria-label={t("events.filter.tenantPlaceholder")}>
                    <SelectValue placeholder={t("events.filter.tenantPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("events.filter.allTenants")}</SelectItem>
                    {tenants.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="self-center text-sm text-muted-foreground">{t("events.filter.advancedDescription")}</p>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("events.list.title")}</CardTitle>
          <CardDescription>{t("events.list.matched", { count: filtered.length })}</CardDescription>
        </CardHeader>
        <CardContent>
          {queryError ? (
            <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {queryError}
            </div>
          ) : null}

          <div className="mb-3">
            <div className="mb-2 flex justify-end">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" size="sm">
                    <SlidersHorizontalIcon className="mr-1.5 size-4" />
                    {t("events.list.columnDisplay")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {toggleableColumns.map((column) => (
                    <DropdownMenuCheckboxItem
                      key={column.id}
                      checked={column.getIsVisible()}
                      onCheckedChange={(checked) => column.toggleVisibility(Boolean(checked))}
                    >
                      {columnLabels[column.id] || column.id}
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
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <TableHead key={header.id}>
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnCount} className="py-10 text-center text-muted-foreground">
                    {t("events.list.loading")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnCount} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? t("events.list.empty.missingScope")
                      : hasActiveFilters
                        ? t("events.list.empty.filtered")
                        : buildingAdmin
                          ? t("events.list.empty.buildingScope")
                          : t("events.list.empty.default")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                table.getRowModel().rows.map((row) => (
                  <TableRow
                    key={row.id}
                    className="cursor-pointer"
                    tabIndex={0}
                    onClick={() => setSelectedEvent(row.original)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault()
                        setSelectedEvent(row.original)
                      }
                    }}
                    aria-label={t("events.list.openDetail", { id: row.original.id })}
                    data-testid="event-row"
                  >
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

      <EventDetailDrawer
        event={selectedEventDetail}
        open={selectedEvent !== null}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedEvent(null)
          }
        }}
      />
    </div>
  )
}
