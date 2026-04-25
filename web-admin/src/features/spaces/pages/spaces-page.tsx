import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Controller, useForm } from "react-hook-form"
import {
  ArrowUpDownIcon,
  Building2Icon,
  DoorOpenIcon,
  Layers3Icon,
  MapPinIcon,
  PlusCircleIcon,
  SearchIcon,
  SlidersHorizontalIcon,
} from "lucide-react"

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
  createArea,
  createBuilding,
  createDoor,
  createFloor,
  listAreas,
  listBuildings,
  listDoors,
  listFloors,
  listTenants,
  type CurrentUser,
  type Area,
  type Building,
  type Door,
  type Floor,
  type Tenant,
} from "@/lib/api"
import {
  canCreateBuildings,
  canManageScopedSpaces,
  getViewerBuildingIDs,
  getViewerTenantID,
  isBuildingAdmin,
  isPlatformViewer,
} from "@/lib/viewer"
import { z } from "zod"

type SpacesPageProps = {
  token: string
  viewer: CurrentUser
}

type DoorKind =
  | "office"
  | "turnstile"
  | "server-room"
  | "elevator"
  | "parking-gate"
  | "emergency-exit"
type DoorStatus = "online" | "offline"
const doorKindValues = [
  "office",
  "turnstile",
  "server-room",
  "elevator",
  "parking-gate",
  "emergency-exit",
] as const
const doorStatusValues = ["online", "offline"] as const

type CreateBuildingFormValues = {
  building_name: string
  building_address?: string
  building_region?: string
}
type CreateFloorFormValues = {
  floor_building_id: string
  floor_name: string
}
type CreateAreaFormValues = {
  area_floor_id: string
  area_name: string
}
type CreateDoorFormValues = {
  door_area_id: string
  door_name: string
  door_gateway_id?: string
  door_kind: DoorKind
  door_status: DoorStatus
}

function statusVariant(status: Door["status"]) {
  switch (status) {
    case "online":
      return "outline"
    default:
      return "destructive"
  }
}

function statusLabel(status: Door["status"], t: (key: string) => string) {
  switch (status) {
    case "online":
      return t("spaces.door.status.online")
    case "offline":
      return t("spaces.door.status.offline")
    default:
      return status
  }
}

function kindLabel(kind: Door["kind"], t: (key: string) => string) {
  switch (kind) {
    case "office":
      return t("spaces.door.kind.office")
    case "turnstile":
      return t("spaces.door.kind.turnstile")
    case "server-room":
      return t("spaces.door.kind.serverRoom")
    case "elevator":
      return t("spaces.door.kind.elevator")
    case "parking-gate":
      return t("spaces.door.kind.parkingGate")
    case "emergency-exit":
      return t("spaces.door.kind.emergencyExit")
    default:
      return kind
  }
}

type SpacesTopologyData = {
  buildings: Building[]
  floors: Floor[]
  areas: Area[]
  doors: Door[]
}

async function loadSpacesTenants(args: {
  token: string
  platformViewer: boolean
  viewerTenantID: string
}): Promise<Tenant[]> {
  if (args.platformViewer) {
    return listTenants(args.token)
  }
  return []
}

async function loadSpacesTopology(args: {
  token: string
  selectedTenantID: string
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerBuildingIDs: Set<string>
}): Promise<SpacesTopologyData> {
  const [buildingItems, floorItems, areaItems, doorItems] = await Promise.all([
    listBuildings(args.token, args.selectedTenantID),
    listFloors(args.token, args.selectedTenantID),
    listAreas(args.token, args.selectedTenantID),
    listDoors(args.token, args.selectedTenantID),
  ])
  const scopedBuildings = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? buildingItems.filter((item) => args.viewerBuildingIDs.has(item.id))
      : buildingItems
  const scopedFloors = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? floorItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : floorItems
  const scopedAreas = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? areaItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : areaItems
  const scopedDoors = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? doorItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : doorItems

  return {
    buildings: scopedBuildings,
    floors: scopedFloors,
    areas: scopedAreas,
    doors: scopedDoors,
  }
}

export function SpacesPage({ token, viewer }: SpacesPageProps) {
  const { t } = useTranslation()
  const createBuildingSchema = useMemo(
    () =>
      z.object({
        building_name: z
          .string()
          .trim()
          .min(1, t("spaces.validation.buildingNameRequired"))
          .max(64, t("spaces.validation.buildingNameMax")),
        building_address: z
          .string()
          .trim()
          .max(128, t("spaces.validation.buildingAddressMax"))
          .optional()
          .or(z.literal("")),
        building_region: z
          .string()
          .trim()
          .max(32, t("spaces.validation.buildingRegionMax"))
          .optional()
          .or(z.literal("")),
      }),
    [t]
  )
  const createFloorSchema = useMemo(
    () =>
      z.object({
        floor_building_id: z.string().trim().min(1, t("spaces.validation.floorBuildingRequired")),
        floor_name: z
          .string()
          .trim()
          .min(1, t("spaces.validation.floorNameRequired"))
          .max(64, t("spaces.validation.floorNameMax")),
      }),
    [t]
  )
  const createAreaSchema = useMemo(
    () =>
      z.object({
        area_floor_id: z.string().trim().min(1, t("spaces.validation.areaFloorRequired")),
        area_name: z
          .string()
          .trim()
          .min(1, t("spaces.validation.areaNameRequired"))
          .max(64, t("spaces.validation.areaNameMax")),
      }),
    [t]
  )
  const createDoorSchema = useMemo(
    () =>
      z.object({
        door_area_id: z.string().trim().min(1, t("spaces.validation.doorAreaRequired")),
        door_name: z
          .string()
          .trim()
          .min(1, t("spaces.validation.doorNameRequired"))
          .max(64, t("spaces.validation.doorNameMax")),
        door_gateway_id: z
          .string()
          .trim()
          .max(64, t("spaces.validation.doorGatewayMax"))
          .optional()
          .or(z.literal("")),
        door_kind: z.enum(doorKindValues),
        door_status: z.enum(doorStatusValues),
      }),
    [t]
  )
  const queryClient = useQueryClient()
  const platformViewer = isPlatformViewer(viewer)
  const buildingAdmin = isBuildingAdmin(viewer)
  const canCreateBuilding = canCreateBuildings(viewer)
  const canManageSpaces = canManageScopedSpaces(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const viewerBuildingIDs = useMemo(() => new Set(getViewerBuildingIDs(viewer)), [viewer])
  const viewerBuildingScopeKey = useMemo(
    () => Array.from(viewerBuildingIDs).sort((a, b) => a.localeCompare(b)).join(","),
    [viewerBuildingIDs]
  )
  const missingBuildingScope = buildingAdmin && viewerBuildingIDs.size === 0
  const effectiveCanManageSpaces = canManageSpaces && !missingBuildingScope
  const [selectedTenantID, setSelectedTenantID] = useState(() => (platformViewer ? "" : viewerTenantID))
  const [tenantQuery, setTenantQuery] = useState("")
  const [error, setError] = useState("")
  const [query, setQuery] = useState("")
  const [doorPage, setDoorPage] = useState(1)
  const [doorPageSize, setDoorPageSize] = useState(25)
  const [doorSorting, setDoorSorting] = useState<SortingState>([])
  const [doorColumnVisibility, setDoorColumnVisibility] = useState<VisibilityState>({})

  const [buildingName, setBuildingName] = useState("")
  const [buildingAddress, setBuildingAddress] = useState("")
  const [buildingRegion, setBuildingRegion] = useState("")
  const [floorName, setFloorName] = useState("")
  const [areaName, setAreaName] = useState("")
  const [doorName, setDoorName] = useState("")
  const [doorGatewayID, setDoorGatewayID] = useState("")

  const [selectedBuildingID, setSelectedBuildingID] = useState("")
  const [selectedFloorID, setSelectedFloorID] = useState("")
  const [selectedAreaID, setSelectedAreaID] = useState("")
  const [doorKind, setDoorKind] = useState<DoorKind>("office")
  const [doorStatus, setDoorStatus] = useState<DoorStatus>("offline")

  const tenantsQuery = useQuery({
    queryKey: ["spaces-tenants", platformViewer, viewerTenantID],
    queryFn: () =>
      loadSpacesTenants({
        token,
        platformViewer,
        viewerTenantID,
      }),
    staleTime: 60 * 1000,
  })
  const tenants = tenantsQuery.data ?? []

  useEffect(() => {
    if (platformViewer) {
      setSelectedTenantID((current) => {
        if (current && tenants.some((item) => item.id === current)) {
          return current
        }
        return tenants[0]?.id ?? ""
      })
      return
    }
    setSelectedTenantID(viewerTenantID)
  }, [platformViewer, tenants, viewerTenantID])

  const topologyQueryKey = useMemo(
    () =>
      [
        "spaces-topology",
        selectedTenantID,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingScopeKey,
      ] as const,
    [selectedTenantID, buildingAdmin, missingBuildingScope, viewerBuildingScopeKey]
  )
  const topologyQuery = useQuery({
    queryKey: topologyQueryKey,
    queryFn: () =>
      loadSpacesTopology({
        token,
        selectedTenantID,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingIDs,
      }),
    enabled: selectedTenantID.trim() !== "",
    staleTime: 30 * 1000,
  })

  const buildings = topologyQuery.data?.buildings ?? []
  const floors = topologyQuery.data?.floors ?? []
  const areas = topologyQuery.data?.areas ?? []
  const doors = topologyQuery.data?.doors ?? []
  const loading = tenantsQuery.isPending || (selectedTenantID.trim() !== "" && topologyQuery.isPending)
  const queryError =
    (tenantsQuery.isError && tenantsQuery.error instanceof Error ? tenantsQuery.error.message : "") ||
    (topologyQuery.isError && topologyQuery.error instanceof Error ? topologyQuery.error.message : "")
  const createBuildingForm = useForm<CreateBuildingFormValues>({
    resolver: zodResolver(createBuildingSchema),
    values: {
      building_name: buildingName,
      building_address: buildingAddress,
      building_region: buildingRegion,
    },
  })
  const createFloorForm = useForm<CreateFloorFormValues>({
    resolver: zodResolver(createFloorSchema),
    values: {
      floor_building_id: selectedBuildingID,
      floor_name: floorName,
    },
  })
  const createAreaForm = useForm<CreateAreaFormValues>({
    resolver: zodResolver(createAreaSchema),
    values: {
      area_floor_id: selectedFloorID,
      area_name: areaName,
    },
  })
  const createDoorForm = useForm<CreateDoorFormValues>({
    resolver: zodResolver(createDoorSchema),
    values: {
      door_area_id: selectedAreaID,
      door_name: doorName,
      door_gateway_id: doorGatewayID,
      door_kind: doorKind,
      door_status: doorStatus,
    },
  })
  const buildingNameField = createBuildingForm.register("building_name")
  const buildingAddressField = createBuildingForm.register("building_address")
  const buildingRegionField = createBuildingForm.register("building_region")
  const floorNameField = createFloorForm.register("floor_name")
  const areaNameField = createAreaForm.register("area_name")
  const doorNameField = createDoorForm.register("door_name")
  const doorGatewayIDField = createDoorForm.register("door_gateway_id")
  const createBuildingFormError =
    createBuildingForm.formState.errors.building_name?.message ||
    createBuildingForm.formState.errors.building_address?.message ||
    createBuildingForm.formState.errors.building_region?.message ||
    ""
  const createFloorFormError =
    createFloorForm.formState.errors.floor_building_id?.message ||
    createFloorForm.formState.errors.floor_name?.message ||
    ""
  const createAreaFormError =
    createAreaForm.formState.errors.area_floor_id?.message ||
    createAreaForm.formState.errors.area_name?.message ||
    ""
  const createDoorFormError =
    createDoorForm.formState.errors.door_area_id?.message ||
    createDoorForm.formState.errors.door_name?.message ||
    createDoorForm.formState.errors.door_gateway_id?.message ||
    createDoorForm.formState.errors.door_kind?.message ||
    createDoorForm.formState.errors.door_status?.message ||
    ""

  const createBuildingMutation = useMutation({
    mutationFn: (payload: { tenant_id: string; name: string; address?: string; region?: string }) =>
      createBuilding(token, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<SpacesTopologyData>(topologyQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          buildings: [created, ...current.buildings],
        }
      })
      setSelectedBuildingID((current) => current || created.id)
      setBuildingName("")
      setBuildingAddress("")
      setBuildingRegion("")
    },
  })

  const createFloorMutation = useMutation({
    mutationFn: (payload: { tenant_id: string; building_id: string; name: string }) => createFloor(token, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<SpacesTopologyData>(topologyQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          floors: [created, ...current.floors],
        }
      })
      setSelectedFloorID(created.id)
      setFloorName("")
    },
  })

  const createAreaMutation = useMutation({
    mutationFn: (payload: { tenant_id: string; building_id: string; floor_id: string; name: string }) =>
      createArea(token, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<SpacesTopologyData>(topologyQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          areas: [created, ...current.areas],
        }
      })
      setSelectedAreaID(created.id)
      setAreaName("")
    },
  })

  const createDoorMutation = useMutation({
    mutationFn: (payload: {
      tenant_id: string
      building_id: string
      floor_id: string
      area_id: string
      name: string
      gateway_id?: string
      kind: DoorKind
      status: DoorStatus
    }) => createDoor(token, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<SpacesTopologyData>(topologyQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          doors: [created, ...current.doors],
        }
      })
      setDoorName("")
      setDoorGatewayID("")
      setDoorStatus("offline")
    },
  })

  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])
  const filteredTenants = useMemo(() => {
    const q = tenantQuery.trim().toLowerCase()
    if (!q) {
      return tenants
    }
    return tenants.filter((item) => {
      return (
        item.id.toLowerCase().includes(q) ||
        item.name.toLowerCase().includes(q) ||
        (item.hq_region ?? "").toLowerCase().includes(q)
      )
    })
  }, [tenantQuery, tenants])
  const buildingByID = useMemo(() => new Map(buildings.map((item) => [item.id, item])), [buildings])
  const floorByID = useMemo(() => new Map(floors.map((item) => [item.id, item])), [floors])
  const areaByID = useMemo(() => new Map(areas.map((item) => [item.id, item])), [areas])

  useEffect(() => {
    setSelectedBuildingID((current) => {
      if (current && buildings.some((item) => item.id === current)) {
        return current
      }
      return buildings[0]?.id ?? ""
    })
    setSelectedFloorID((current) => {
      if (current && floors.some((item) => item.id === current)) {
        return current
      }
      return floors[0]?.id ?? ""
    })
    setSelectedAreaID((current) => {
      if (current && areas.some((item) => item.id === current)) {
        return current
      }
      return areas[0]?.id ?? ""
    })
  }, [areas, buildings, floors])

  const filteredDoors = useMemo(() => {
    const q = query.trim().toLowerCase()
    return doors.filter((door) => {
      if (!q) {
        return true
      }

      const buildingName = buildingByID.get(door.building_id)?.name ?? door.building_id
      const floorName = floorByID.get(door.floor_id)?.name ?? door.floor_id
      const areaName = areaByID.get(door.area_id)?.name ?? door.area_id
      const tenantName = tenantByID.get(door.tenant_id)?.name ?? door.tenant_id

      return (
        door.id.toLowerCase().includes(q) ||
        door.name.toLowerCase().includes(q) ||
        door.gateway_id.toLowerCase().includes(q) ||
        door.kind.toLowerCase().includes(q) ||
        door.status.toLowerCase().includes(q) ||
        tenantName.toLowerCase().includes(q) ||
        buildingName.toLowerCase().includes(q) ||
        floorName.toLowerCase().includes(q) ||
        areaName.toLowerCase().includes(q)
      )
    })
  }, [areaByID, buildingByID, doors, floorByID, query, tenantByID])
  const doorColumns = useMemo<ColumnDef<Door>[]>(
    () => {
      const definition: ColumnDef<Door>[] = [
        {
          id: "id",
          accessorKey: "id",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.id")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem] font-medium">{row.original.id}</TableCellText>,
        },
        {
          id: "building_floor",
          accessorFn: (row) => {
            const building = buildingByID.get(row.building_id)
            const floor = floorByID.get(row.floor_id)
            return `${building?.name ?? row.building_id} / ${floor?.name ?? row.floor_id}`
          },
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.buildingFloor")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => {
            const building = buildingByID.get(row.original.building_id)
            const floor = floorByID.get(row.original.floor_id)
            return (
              <div className="flex items-center gap-1.5">
                <MapPinIcon className="size-3.5 text-muted-foreground" />
                <span>
                  {building?.name ?? row.original.building_id} / {floor?.name ?? row.original.floor_id}
                </span>
              </div>
            )
          },
        },
        {
          id: "area",
          accessorFn: (row) => areaByID.get(row.area_id)?.name ?? row.area_id,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.area")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => areaByID.get(row.original.area_id)?.name ?? row.original.area_id,
        },
        {
          id: "name",
          accessorKey: "name",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.name")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem]">{row.original.name}</TableCellText>,
        },
        {
          id: "kind",
          accessorKey: "kind",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.kind")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => kindLabel(row.original.kind, t),
        },
        {
          id: "gateway_id",
          accessorFn: (row) => row.gateway_id || "-",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.gateway")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => row.original.gateway_id || t("spaces.table.emptyDash"),
        },
        {
          id: "status",
          accessorKey: "status",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.status")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <Badge variant={statusVariant(row.original.status)}>
              {statusLabel(row.original.status, t)}
            </Badge>
          ),
        },
      ]
      if (platformViewer) {
        definition.splice(1, 0, {
          id: "tenant",
          accessorFn: (row) => tenantByID.get(row.tenant_id)?.name ?? row.tenant_id,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("spaces.table.tenant")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem]">{tenantByID.get(row.original.tenant_id)?.name ?? row.original.tenant_id}</TableCellText>,
        })
      }
      return definition
    },
    [areaByID, buildingByID, floorByID, platformViewer, tenantByID, t]
  )
  const doorTable = useReactTable({
    columns: doorColumns,
    data: filteredDoors,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility: doorColumnVisibility,
      pagination: {
        pageIndex: Math.max(0, doorPage - 1),
        pageSize: doorPageSize,
      },
      sorting: doorSorting,
    },
    onColumnVisibilityChange: setDoorColumnVisibility,
    onSortingChange: setDoorSorting,
  })
  const doorFilteredCount = doorTable.getFilteredRowModel().rows.length
  const doorMaxPage = Math.max(1, Math.ceil(doorFilteredCount / doorPageSize))
  const visibleDoorColumnCount = doorTable.getVisibleLeafColumns().length
  const toggleableDoorColumns = doorTable.getAllLeafColumns().filter((column) => column.getCanHide())
  const doorColumnLabels: Record<string, string> = {
    area: t("spaces.table.area"),
    building_floor: t("spaces.table.buildingFloor"),
    gateway_id: t("spaces.table.gateway"),
    id: t("spaces.table.id"),
    kind: t("spaces.table.kind"),
    name: t("spaces.table.name"),
    status: t("spaces.table.status"),
    tenant: t("spaces.table.tenant"),
  }

  useEffect(() => {
    setDoorPage(1)
  }, [doorPageSize, query, selectedTenantID])

  useEffect(() => {
    if (doorPage > doorMaxPage) {
      setDoorPage(doorMaxPage)
    }
  }, [doorMaxPage, doorPage])

  async function onCreateBuilding(values: CreateBuildingFormValues) {
    if (!selectedTenantID || !values.building_name.trim()) {
      return
    }

    setError("")
    try {
      await createBuildingMutation.mutateAsync({
        tenant_id: selectedTenantID,
        name: values.building_name.trim(),
        address: values.building_address?.trim() || undefined,
        region: values.building_region?.trim() || undefined,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("spaces.error.createBuildingFailed")
      setError(message)
    }
  }

  async function onCreateFloor(values: CreateFloorFormValues) {
    if (!selectedTenantID || !values.floor_building_id.trim() || !values.floor_name.trim()) {
      return
    }

    setError("")
    try {
      await createFloorMutation.mutateAsync({
        tenant_id: selectedTenantID,
        building_id: values.floor_building_id.trim(),
        name: values.floor_name.trim(),
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("spaces.error.createFloorFailed")
      setError(message)
    }
  }

  async function onCreateArea(values: CreateAreaFormValues) {
    if (!selectedTenantID || !values.area_floor_id.trim() || !values.area_name.trim()) {
      return
    }

    const floor = floorByID.get(values.area_floor_id.trim())
    if (!floor) {
      return
    }

    setError("")
    try {
      await createAreaMutation.mutateAsync({
        tenant_id: selectedTenantID,
        building_id: floor.building_id,
        floor_id: floor.id,
        name: values.area_name.trim(),
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("spaces.error.createAreaFailed")
      setError(message)
    }
  }

  async function onCreateDoor(values: CreateDoorFormValues) {
    if (!selectedTenantID || !values.door_area_id.trim() || !values.door_name.trim()) {
      return
    }

    const area = areaByID.get(values.door_area_id.trim())
    if (!area) {
      return
    }

    setError("")
    try {
      await createDoorMutation.mutateAsync({
        tenant_id: selectedTenantID,
        building_id: area.building_id,
        floor_id: area.floor_id,
        area_id: area.id,
        name: values.door_name.trim(),
        gateway_id: values.door_gateway_id?.trim() || undefined,
        kind: values.door_kind,
        status: values.door_status,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("spaces.error.createDoorFailed")
      setError(message)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex flex-col gap-1">
          <p className="mp-page-eyebrow">
            {platformViewer
              ? t("spaces.header.eyebrowPlatform")
              : buildingAdmin
                ? t("spaces.header.eyebrowBuildingAdmin")
                : t("spaces.header.eyebrowTenant")}
          </p>
          <h1 className="mp-page-title">{t("spaces.header.title")}</h1>
          <p className="mp-page-description">
            {platformViewer
              ? t("spaces.header.descriptionPlatform")
              : buildingAdmin
                ? t("spaces.header.descriptionBuildingAdmin")
                : t("spaces.header.descriptionTenant")}
          </p>
        </div>
        {platformViewer ? (
          <div className="w-full lg:w-[340px]">
            <div className="space-y-2">
              <Input
                value={tenantQuery}
                onChange={(event) => setTenantQuery(event.target.value)}
                placeholder={t("spaces.tenant.searchPlaceholder")}
              />
              <Select value={selectedTenantID} onValueChange={setSelectedTenantID}>
                <SelectTrigger>
                  <SelectValue placeholder={t("spaces.tenant.selectPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {filteredTenants.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        ) : (
          <Badge variant="outline" className="w-fit rounded-full px-3 py-1">
            {buildingAdmin
              ? missingBuildingScope
                ? t("spaces.tenant.badge.unassignedScope")
                : buildings.length > 0
                  ? t("spaces.tenant.badge.assignedBuildings", { count: buildings.length })
                  : t("spaces.tenant.badge.currentScope")
              : selectedTenantID || t("spaces.tenant.badge.currentTenant")}
          </Badge>
        )}
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          {t("spaces.notice.missingScope")}
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          {t("spaces.notice.buildingScopeHint")}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("spaces.kpi.buildings.title")}</CardDescription>
            <CardTitle className="text-2xl">{buildings.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin ? t("spaces.kpi.buildings.noteBuildingAdmin") : t("spaces.kpi.buildings.noteDefault")}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("spaces.kpi.floors.title")}</CardDescription>
            <CardTitle className="text-2xl">{floors.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("spaces.kpi.floors.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("spaces.kpi.areas.title")}</CardDescription>
            <CardTitle className="text-2xl">{areas.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("spaces.kpi.areas.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("spaces.kpi.doors.title")}</CardDescription>
            <CardTitle className="text-2xl">{doors.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("spaces.kpi.doors.note")}</CardContent>
        </Card>
      </div>

      <div className={`grid gap-4 ${canCreateBuilding ? "xl:grid-cols-2" : "xl:grid-cols-1"}`}>
        {canCreateBuilding ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("spaces.createBuilding.title")}</CardTitle>
              <CardDescription>{t("spaces.createBuilding.description")}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                className="grid gap-3 md:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_auto]"
                onSubmit={createBuildingForm.handleSubmit(onCreateBuilding)}
              >
                <Input
                  {...buildingNameField}
                  onChange={(event) => {
                    buildingNameField.onChange(event)
                    setBuildingName(event.target.value)
                  }}
                  placeholder={t("spaces.createBuilding.namePlaceholder")}
                />
                <Input
                  {...buildingAddressField}
                  onChange={(event) => {
                    buildingAddressField.onChange(event)
                    setBuildingAddress(event.target.value)
                  }}
                  placeholder={t("spaces.createBuilding.addressPlaceholder")}
                />
                <Input
                  {...buildingRegionField}
                  onChange={(event) => {
                    buildingRegionField.onChange(event)
                    setBuildingRegion(event.target.value)
                  }}
                  placeholder={t("spaces.createBuilding.regionPlaceholder")}
                />
                <Button
                  type="submit"
                  disabled={
                    createBuildingMutation.isPending || !selectedTenantID || createBuildingForm.formState.isSubmitting
                  }
                >
                  <PlusCircleIcon className="mr-1.5 size-4" />
                  {createBuildingMutation.isPending ? t("spaces.createBuilding.submitting") : t("spaces.createBuilding.submit")}
                </Button>
                {createBuildingFormError ? (
                  <p className="text-sm text-destructive md:col-span-2 xl:col-span-4">{createBuildingFormError}</p>
                ) : null}
              </form>
            </CardContent>
          </Card>
        ) : null}

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("spaces.createFloorArea.title")}</CardTitle>
            <CardDescription>
              {buildingAdmin ? t("spaces.createFloorArea.descriptionBuildingAdmin") : t("spaces.createFloorArea.descriptionDefault")}
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <form className="space-y-3 rounded-lg border p-3" onSubmit={createFloorForm.handleSubmit(onCreateFloor)}>
              <p className="text-sm font-medium">{t("spaces.createFloorArea.floorSectionTitle")}</p>
              <Controller
                control={createFloorForm.control}
                name="floor_building_id"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onValueChange={(value) => {
                      field.onChange(value)
                      setSelectedBuildingID(value)
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t("spaces.createFloorArea.floorBuildingPlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      {buildings.map((item) => (
                        <SelectItem key={item.id} value={item.id}>
                          {item.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              <Input
                {...floorNameField}
                onChange={(event) => {
                  floorNameField.onChange(event)
                  setFloorName(event.target.value)
                }}
                placeholder={t("spaces.createFloorArea.floorNamePlaceholder")}
              />
                <Button
                  type="submit"
                  disabled={
                    createFloorMutation.isPending ||
                    !selectedTenantID ||
                    !effectiveCanManageSpaces ||
                    createFloorForm.formState.isSubmitting
                  }
                  className="w-full"
                >
                  {createFloorMutation.isPending ? t("spaces.createFloorArea.floorSubmitting") : t("spaces.createFloorArea.floorSubmit")}
                </Button>
                {createFloorFormError ? <p className="text-sm text-destructive">{createFloorFormError}</p> : null}
            </form>

            <form className="space-y-3 rounded-lg border p-3" onSubmit={createAreaForm.handleSubmit(onCreateArea)}>
              <p className="text-sm font-medium">{t("spaces.createFloorArea.areaSectionTitle")}</p>
              <Controller
                control={createAreaForm.control}
                name="area_floor_id"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onValueChange={(value) => {
                      field.onChange(value)
                      setSelectedFloorID(value)
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t("spaces.createFloorArea.areaFloorPlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      {floors.map((item) => {
                        const building = buildingByID.get(item.building_id)
                        return (
                          <SelectItem key={item.id} value={item.id}>
                            {building?.name ?? item.building_id} / {item.name}
                          </SelectItem>
                        )
                      })}
                    </SelectContent>
                  </Select>
                )}
              />
              <Input
                {...areaNameField}
                onChange={(event) => {
                  areaNameField.onChange(event)
                  setAreaName(event.target.value)
                }}
                placeholder={t("spaces.createFloorArea.areaNamePlaceholder")}
              />
                <Button
                  type="submit"
                  disabled={
                    createAreaMutation.isPending ||
                    !selectedTenantID ||
                    !effectiveCanManageSpaces ||
                    createAreaForm.formState.isSubmitting
                  }
                  className="w-full"
                >
                  {createAreaMutation.isPending ? t("spaces.createFloorArea.areaSubmitting") : t("spaces.createFloorArea.areaSubmit")}
                </Button>
                {createAreaFormError ? <p className="text-sm text-destructive">{createAreaFormError}</p> : null}
            </form>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("spaces.createDoor.title")}</CardTitle>
          <CardDescription>
            {buildingAdmin ? t("spaces.createDoor.descriptionBuildingAdmin") : t("spaces.createDoor.descriptionDefault")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="grid gap-3 md:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_1fr_1fr_auto]"
            onSubmit={createDoorForm.handleSubmit(onCreateDoor)}
          >
            <Controller
              control={createDoorForm.control}
              name="door_area_id"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value) => {
                    field.onChange(value)
                    setSelectedAreaID(value)
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("spaces.createDoor.areaPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {areas.map((item) => {
                      const building = buildingByID.get(item.building_id)
                      const floor = floorByID.get(item.floor_id)
                      return (
                        <SelectItem key={item.id} value={item.id}>
                          {building?.name ?? item.building_id} / {floor?.name ?? item.floor_id} / {item.name}
                        </SelectItem>
                      )
                    })}
                  </SelectContent>
                </Select>
              )}
            />

            <Input
              {...doorNameField}
              onChange={(event) => {
                doorNameField.onChange(event)
                setDoorName(event.target.value)
              }}
              placeholder={t("spaces.createDoor.namePlaceholder")}
            />
            <Input
              {...doorGatewayIDField}
              onChange={(event) => {
                doorGatewayIDField.onChange(event)
                setDoorGatewayID(event.target.value)
              }}
              placeholder={t("spaces.createDoor.gatewayPlaceholder")}
            />
            <Controller
              control={createDoorForm.control}
              name="door_kind"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value: DoorKind) => {
                    field.onChange(value)
                    setDoorKind(value)
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("spaces.createDoor.kindPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="office">{t("spaces.door.kind.office")}</SelectItem>
                    <SelectItem value="turnstile">{t("spaces.door.kind.turnstile")}</SelectItem>
                    <SelectItem value="elevator">{t("spaces.door.kind.elevator")}</SelectItem>
                    <SelectItem value="server-room">{t("spaces.door.kind.serverRoom")}</SelectItem>
                    <SelectItem value="parking-gate">{t("spaces.door.kind.parkingGate")}</SelectItem>
                    <SelectItem value="emergency-exit">{t("spaces.door.kind.emergencyExit")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
            <Controller
              control={createDoorForm.control}
              name="door_status"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value: DoorStatus) => {
                    field.onChange(value)
                    setDoorStatus(value)
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("spaces.createDoor.statusPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="online">{t("spaces.door.status.online")}</SelectItem>
                    <SelectItem value="offline">{t("spaces.door.status.offline")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
            <Button
              type="submit"
              disabled={
                createDoorMutation.isPending ||
                !selectedTenantID ||
                !effectiveCanManageSpaces ||
                createDoorForm.formState.isSubmitting
              }
            >
              {createDoorMutation.isPending ? t("spaces.createDoor.submitting") : t("spaces.createDoor.submit")}
            </Button>
            {createDoorFormError ? <p className="text-sm text-destructive md:col-span-2 xl:col-span-6">{createDoorFormError}</p> : null}
          </form>
        </CardContent>
      </Card>

      <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("spaces.ledger.title")}</CardTitle>
            <CardDescription>
              {platformViewer
                ? t("spaces.ledger.descriptionPlatform")
                : buildingAdmin
                  ? t("spaces.ledger.descriptionBuildingAdmin")
                  : t("spaces.ledger.descriptionTenant")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="relative max-w-sm">
              <div className="flex flex-col gap-2 md:flex-row md:items-center">
                <div className="relative w-full md:max-w-sm">
                  <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                  <Input
                    value={query}
                    onChange={(event) => {
                      setQuery(event.target.value)
                      setDoorPage(1)
                    }}
                    className="pl-8"
                    placeholder={
                      platformViewer
                        ? t("spaces.ledger.searchPlaceholderPlatform")
                        : buildingAdmin
                          ? t("spaces.ledger.searchPlaceholderBuildingAdmin")
                          : t("spaces.ledger.searchPlaceholderTenant")
                    }
                  />
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button type="button" variant="outline" size="sm" className="w-full md:w-auto">
                      <SlidersHorizontalIcon className="mr-1.5 size-4" />
                      {t("spaces.ledger.columnDisplay")}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {toggleableDoorColumns.map((column) => (
                      <DropdownMenuCheckboxItem
                        key={column.id}
                        checked={column.getIsVisible()}
                        onCheckedChange={(checked) => column.toggleVisibility(Boolean(checked))}
                      >
                        {doorColumnLabels[column.id] || column.id}
                      </DropdownMenuCheckboxItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>

	          {error || queryError ? (
	            <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
	              {error || queryError}
	            </div>
	          ) : null}

            <ListPagination
              page={doorPage}
              onPageChange={setDoorPage}
              pageSize={doorPageSize}
              onPageSizeChange={(pageSize) => {
                setDoorPageSize(pageSize)
                setDoorPage(1)
              }}
              hasNextPage={doorTable.getCanNextPage()}
              disabled={loading || doorFilteredCount === 0}
            />

	          <Table>
            <TableHeader>
              {doorTable.getHeaderGroups().map((headerGroup) => (
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
                  <TableCell colSpan={visibleDoorColumnCount} className="py-10 text-center text-muted-foreground">
                    {t("spaces.ledger.loading")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && doorFilteredCount === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleDoorColumnCount} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? t("spaces.ledger.empty.missingScope")
                      : query.trim()
                        ? t("spaces.ledger.empty.filtered")
                        : t("spaces.ledger.empty.default")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                doorTable.getRowModel().rows.map((row) => (
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

      <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("spaces.topology.title")}</CardTitle>
            <CardDescription>
              {platformViewer
                ? t("spaces.topology.descriptionPlatform")
                : buildingAdmin
                  ? t("spaces.topology.descriptionBuildingAdmin")
                  : t("spaces.topology.descriptionTenant")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3">
              <Building2Icon className="size-4 text-primary" />
              <div>
                <p className="font-medium">{t("spaces.topology.cards.building.title")}</p>
                <p className="mp-kpi-note">
                  {platformViewer
                    ? t("spaces.topology.cards.building.notePlatform")
                    : buildingAdmin
                      ? t("spaces.topology.cards.building.noteBuildingAdmin")
                      : t("spaces.topology.cards.building.noteTenant")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3">
              <Layers3Icon className="size-4 text-sky-500" />
              <div>
                <p className="font-medium">{t("spaces.topology.cards.floorArea.title")}</p>
                <p className="mp-kpi-note">{t("spaces.topology.cards.floorArea.note")}</p>
              </div>
            </div>
            <div className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3">
              <DoorOpenIcon className="size-4 text-violet-500" />
              <div>
                <p className="font-medium">{t("spaces.topology.cards.door.title")}</p>
                <p className="mp-kpi-note">{t("spaces.topology.cards.door.note")}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
