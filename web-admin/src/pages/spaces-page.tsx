import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FormEvent, useEffect, useMemo, useState } from "react"
import {
  Building2Icon,
  DoorOpenIcon,
  Layers3Icon,
  MapPinIcon,
  PlusCircleIcon,
  SearchIcon,
} from "lucide-react"

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
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
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

function statusVariant(status: Door["status"]) {
  switch (status) {
    case "online":
      return "outline"
    default:
      return "destructive"
  }
}

function statusLabel(status: Door["status"]) {
  switch (status) {
    case "online":
      return "在线"
    case "offline":
      return "离线"
    default:
      return status
  }
}

function kindLabel(kind: Door["kind"]) {
  switch (kind) {
    case "office":
      return "办公门"
    case "turnstile":
      return "闸机"
    case "server-room":
      return "机房门"
    case "elevator":
      return "电梯"
    case "parking-gate":
      return "停车闸口"
    case "emergency-exit":
      return "消防通道"
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
    queryKey: ["spaces-tenants", token, platformViewer, viewerTenantID],
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
        token,
        selectedTenantID,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingScopeKey,
      ] as const,
    [token, selectedTenantID, buildingAdmin, missingBuildingScope, viewerBuildingScopeKey]
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

  async function onCreateBuilding(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !buildingName.trim()) {
      return
    }

    setError("")
    try {
      await createBuildingMutation.mutateAsync({
        tenant_id: selectedTenantID,
        name: buildingName.trim(),
        address: buildingAddress.trim() || undefined,
        region: buildingRegion.trim() || undefined,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建楼宇失败"
      setError(message)
    }
  }

  async function onCreateFloor(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !selectedBuildingID || !floorName.trim()) {
      return
    }

    setError("")
    try {
      await createFloorMutation.mutateAsync({
        tenant_id: selectedTenantID,
        building_id: selectedBuildingID,
        name: floorName.trim(),
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建楼层失败"
      setError(message)
    }
  }

  async function onCreateArea(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !selectedFloorID || !areaName.trim()) {
      return
    }

    const floor = floorByID.get(selectedFloorID)
    if (!floor) {
      return
    }

    setError("")
    try {
      await createAreaMutation.mutateAsync({
        tenant_id: selectedTenantID,
        building_id: floor.building_id,
        floor_id: floor.id,
        name: areaName.trim(),
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建区域失败"
      setError(message)
    }
  }

  async function onCreateDoor(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !selectedAreaID || !doorName.trim()) {
      return
    }

    const area = areaByID.get(selectedAreaID)
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
        name: doorName.trim(),
        gateway_id: doorGatewayID.trim() || undefined,
        kind: doorKind,
        status: doorStatus,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建门点失败"
      setError(message)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex flex-col gap-1">
          <p className="mp-page-eyebrow">
            {platformViewer ? "租户空间拓扑" : buildingAdmin ? "楼宇空间拓扑" : "组织空间拓扑"}
          </p>
          <h1 className="mp-page-title">楼宇、楼层、区域与门点</h1>
          <p className="mp-page-description">
            {platformViewer
              ? "先明确租户归属，再落地门点与网关绑定。"
              : buildingAdmin
                ? "按负责楼宇维护楼层、区域和门点，不暴露非本楼宇范围。"
                : "按当前组织维护楼宇和门点，不再暴露租户切换。"}
          </p>
        </div>
        {platformViewer ? (
          <div className="w-full lg:w-[340px]">
            <div className="space-y-2">
              <Input
                value={tenantQuery}
                onChange={(event) => setTenantQuery(event.target.value)}
                placeholder="先搜索租户（名称/ID/地区）"
              />
              <Select value={selectedTenantID} onValueChange={setSelectedTenantID}>
                <SelectTrigger>
                  <SelectValue placeholder="选择租户" />
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
                ? "未分配楼宇范围"
                : `${buildings.length > 0 ? `${buildings.length} 个负责楼宇` : "当前楼宇范围"}`
              : selectedTenantID || "当前组织"}
          </Badge>
        )}
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何楼宇、楼层、区域或门点数据，也不会开放新增操作。
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          当前仅可维护负责楼宇范围内的楼层、区域和门点；楼宇节点仍由企业管理员或平台管理员创建。
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>楼宇</CardDescription>
            <CardTitle className="text-2xl">{buildings.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {buildingAdmin ? "当前楼宇管理员负责的物理站点。" : "当前租户下的物理站点。"}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>楼层</CardDescription>
            <CardTitle className="text-2xl">{floors.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">用于区域授权继承。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>区域</CardDescription>
            <CardTitle className="text-2xl">{areas.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">部门分区与控制范围。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>门点</CardDescription>
            <CardTitle className="text-2xl">{doors.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">电梯、闸机、办公门统一管理。</CardContent>
        </Card>
      </div>

      <div className={`grid gap-4 ${canCreateBuilding ? "xl:grid-cols-2" : "xl:grid-cols-1"}`}>
        {canCreateBuilding ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">创建楼宇</CardTitle>
              <CardDescription>当前选中租户下新增楼宇节点。</CardDescription>
            </CardHeader>
            <CardContent>
              <form className="grid gap-3 md:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_auto]" onSubmit={onCreateBuilding}>
                <Input
                  value={buildingName}
                  onChange={(event) => setBuildingName(event.target.value)}
                  placeholder="楼宇名称（必填）"
                />
                <Input
                  value={buildingAddress}
                  onChange={(event) => setBuildingAddress(event.target.value)}
                  placeholder="地址（可选）"
                />
                <Input
                  value={buildingRegion}
                  onChange={(event) => setBuildingRegion(event.target.value)}
                  placeholder="地区（如 ID-JK）"
                />
                <Button type="submit" disabled={createBuildingMutation.isPending || !selectedTenantID}>
                  <PlusCircleIcon className="mr-1.5 size-4" />
                  {createBuildingMutation.isPending ? "创建中..." : "创建"}
                </Button>
              </form>
            </CardContent>
          </Card>
        ) : null}

        <Card>
          <CardHeader>
            <CardTitle className="text-base">创建楼层与区域</CardTitle>
            <CardDescription>
              {buildingAdmin ? "仅在负责楼宇范围内扩展楼层与区域。" : "以楼宇为边界扩展楼层与区域。"}
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <form className="space-y-3 rounded-lg border p-3" onSubmit={onCreateFloor}>
              <p className="text-sm font-medium">新增楼层</p>
              <Select value={selectedBuildingID} onValueChange={setSelectedBuildingID}>
                <SelectTrigger>
                  <SelectValue placeholder="选择楼宇" />
                </SelectTrigger>
                <SelectContent>
                  {buildings.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                value={floorName}
                onChange={(event) => setFloorName(event.target.value)}
                placeholder="楼层名称（例如 L9）"
              />
                <Button
                  type="submit"
                  disabled={createFloorMutation.isPending || !selectedTenantID || !effectiveCanManageSpaces}
                  className="w-full"
                >
                  {createFloorMutation.isPending ? "添加中..." : "新增楼层"}
                </Button>
            </form>

            <form className="space-y-3 rounded-lg border p-3" onSubmit={onCreateArea}>
              <p className="text-sm font-medium">新增区域</p>
              <Select value={selectedFloorID} onValueChange={setSelectedFloorID}>
                <SelectTrigger>
                  <SelectValue placeholder="选择楼层" />
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
              <Input
                value={areaName}
                onChange={(event) => setAreaName(event.target.value)}
                placeholder="区域名称（例如 财务区）"
              />
                <Button
                  type="submit"
                  disabled={createAreaMutation.isPending || !selectedTenantID || !effectiveCanManageSpaces}
                  className="w-full"
                >
                  {createAreaMutation.isPending ? "添加中..." : "新增区域"}
                </Button>
            </form>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">登记门点</CardTitle>
          <CardDescription>
            {buildingAdmin ? "门点仅能登记到负责楼宇下的区域，状态仍只区分在线/离线。" : "门点需明确租户归属，状态仅区分在线/离线。"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-3 md:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_1fr_1fr_auto]" onSubmit={onCreateDoor}>
            <Select value={selectedAreaID} onValueChange={setSelectedAreaID}>
              <SelectTrigger>
                <SelectValue placeholder="选择区域" />
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

            <Input
              value={doorName}
              onChange={(event) => setDoorName(event.target.value)}
              placeholder="门点名称"
            />
            <Input
              value={doorGatewayID}
              onChange={(event) => setDoorGatewayID(event.target.value)}
              placeholder="网关编号（可选）"
            />
            <Select value={doorKind} onValueChange={(value: DoorKind) => setDoorKind(value)}>
              <SelectTrigger>
                <SelectValue placeholder="门点类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="office">办公门</SelectItem>
                <SelectItem value="turnstile">闸机</SelectItem>
                <SelectItem value="elevator">电梯</SelectItem>
                <SelectItem value="server-room">机房门</SelectItem>
                <SelectItem value="parking-gate">停车闸口</SelectItem>
                <SelectItem value="emergency-exit">消防通道</SelectItem>
              </SelectContent>
            </Select>
            <Select value={doorStatus} onValueChange={(value: DoorStatus) => setDoorStatus(value)}>
              <SelectTrigger>
                <SelectValue placeholder="在线状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="online">在线</SelectItem>
                <SelectItem value="offline">离线</SelectItem>
              </SelectContent>
            </Select>
            <Button type="submit" disabled={createDoorMutation.isPending || !selectedTenantID || !effectiveCanManageSpaces}>
              {createDoorMutation.isPending ? "添加中..." : "新增门点"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
          <CardHeader>
            <CardTitle className="text-base">门点台账</CardTitle>
            <CardDescription>
              {platformViewer
                ? "当前选中租户的门点与在线情况。"
                : buildingAdmin
                  ? "当前负责楼宇范围内的门点与在线情况。"
                  : "当前组织的门点与在线情况。"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
          <div className="relative max-w-sm">
            <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                className="pl-8"
                placeholder={
                  platformViewer
                    ? "按租户、楼宇、区域、门点、网关搜索"
                    : buildingAdmin
                      ? "按负责楼宇、区域、门点、网关搜索"
                      : "按楼宇、区域、门点、网关搜索"
                }
              />
            </div>

          {error || queryError ? (
            <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error || queryError}
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>门点 ID</TableHead>
                {platformViewer ? <TableHead>租户</TableHead> : null}
                <TableHead>楼宇 / 楼层</TableHead>
                <TableHead>区域</TableHead>
                <TableHead>门点名称</TableHead>
                <TableHead>类型</TableHead>
                <TableHead>网关</TableHead>
                <TableHead>状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 8 : 7} className="py-10 text-center text-muted-foreground">
                    正在加载空间拓扑...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredDoors.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 8 : 7} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? "当前楼宇管理员尚未分配楼宇范围。"
                      : query.trim()
                        ? "当前搜索条件下没有匹配的门点。"
                        : "当前范围内暂无门点，请先补齐楼宇拓扑。"}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                filteredDoors.map((door) => {
                  const building = buildingByID.get(door.building_id)
                  const floor = floorByID.get(door.floor_id)
                  const area = areaByID.get(door.area_id)
                  const tenant = tenantByID.get(door.tenant_id)

                  return (
                    <TableRow key={door.id}>
                      <TableCell className="font-medium">{door.id}</TableCell>
                      {platformViewer ? <TableCell>{tenant?.name ?? door.tenant_id}</TableCell> : null}
                      <TableCell>
                        <div className="flex items-center gap-1.5">
                          <MapPinIcon className="size-3.5 text-muted-foreground" />
                          <span>
                            {building?.name ?? door.building_id} / {floor?.name ?? door.floor_id}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>{area?.name ?? door.area_id}</TableCell>
                      <TableCell>{door.name}</TableCell>
                      <TableCell>{kindLabel(door.kind)}</TableCell>
                      <TableCell>{door.gateway_id || "-"}</TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(door.status)}>
                          {statusLabel(door.status)}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  )
                })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">拓扑层级</CardTitle>
            <CardDescription>
              {platformViewer
                ? "租户 → 楼宇（地区） → 楼层 → 区域 → 门点。"
                : buildingAdmin
                  ? "负责楼宇 → 楼层 → 区域 → 门点。"
                  : "组织 → 楼宇（地区） → 楼层 → 区域 → 门点。"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3">
              <Building2Icon className="size-4 text-primary" />
              <div>
                <p className="font-medium">楼宇</p>
                <p className="mp-kpi-note">
                  {platformViewer
                    ? "可跨地区部署，归属唯一租户。"
                    : buildingAdmin
                      ? "仅展示当前楼宇管理员负责的楼宇集合。"
                      : "可跨地区部署，归属当前组织。"}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3">
              <Layers3Icon className="size-4 text-sky-500" />
              <div>
                <p className="font-medium">楼层 + 区域</p>
                <p className="mp-kpi-note">作为授权范围与事件归档边界。</p>
              </div>
            </div>
            <div className="flex items-center gap-2 rounded-lg border bg-muted/20 p-3">
              <DoorOpenIcon className="size-4 text-violet-500" />
              <div>
                <p className="font-medium">门点</p>
                <p className="mp-kpi-note">支持闸机、电梯、停车闸口等类型。</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
