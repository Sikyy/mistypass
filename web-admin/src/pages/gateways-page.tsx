import { FormEvent, useEffect, useMemo, useState } from "react"
import { CpuIcon, Plug2Icon, RefreshCwIcon, SearchIcon, SendIcon, ShieldEllipsisIcon, UnplugIcon } from "lucide-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
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
  batchUpdateGatewaySerialInventoryStatus,
  bindGatewayDoor,
  exportGatewaySerialInventoryCSV,
  importGatewaySerialInventoryCSV,
  importGatewaySerialInventory,
  probeGatewayLegacyDevices,
  listDoors,
  listGateways,
  listGatewayEventCheckpointSummary,
  listGatewaySerialInventory,
  listTenants,
  publishGatewayConfig,
  rebootGateway,
  registerGatewayDevice,
  registerGateway,
  updateGatewaySerialInventoryStatus,
  unbindGatewayDoor,
  type CurrentUser,
  type Door,
  type Gateway,
  type GatewayCommandAck,
  type GatewayCheckpointSummaryResponse,
  type GatewayDevice,
  type GatewaySerialInventoryItem,
  type GatewaySerialInventoryProductType,
  type GatewaySerialInventoryStatus,
  type Tenant,
} from "@/lib/api"
import {
  canAccessGatewayInventory,
  canEditGatewayInventory,
  canManageGateways,
  canRegisterGateways,
  getViewerBuildingIDs,
  getViewerTenantID,
  isBuildingAdmin,
  isPlatformViewer,
} from "@/lib/viewer"

type GatewaysPageProps = {
  token: string
  viewer: CurrentUser
}

type CommandTaskStatus = "queued" | "dispatching" | "delivered" | "acknowledged"
type CommandTask = {
  task_id: string
  gateway_id: string
  command: string
  status: CommandTaskStatus
  created_at: string
  updated_at: string
}

type GatewayDeviceKind = "reader" | "door_controller" | "relay" | "sensor" | "legacy_reader" | "legacy_controller"
type GatewayDeviceSource = "mistypass_procured" | "legacy_integration"
type GatewayDeviceProtocol = "auto" | "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"

function statusVariant(status: string) {
  switch (status) {
    case "online":
      return "outline"
    case "offline":
      return "destructive"
    default:
      return "secondary"
  }
}

function statusLabel(status: string) {
  switch (status) {
    case "online":
      return "在线"
    case "offline":
      return "离线"
    default:
      return status
  }
}

function checkpointTrendDirectionLabel(direction: string) {
  switch (direction) {
    case "up":
      return "上升"
    case "down":
      return "下降"
    case "flat":
      return "持平"
    default:
      return direction
  }
}

function checkpointTrendDirectionVariant(direction: string) {
  switch (direction) {
    case "up":
      return "outline"
    case "down":
      return "destructive"
    default:
      return "secondary"
  }
}

function commandStatusLabel(status: CommandTaskStatus) {
  switch (status) {
    case "queued":
      return "已入队"
    case "dispatching":
      return "下发中"
    case "delivered":
      return "设备已收"
    case "acknowledged":
      return "执行回执"
    default:
      return status
  }
}

function commandStatusVariant(status: CommandTaskStatus) {
  switch (status) {
    case "queued":
      return "secondary"
    case "dispatching":
      return "outline"
    case "delivered":
      return "outline"
    case "acknowledged":
      return "default"
    default:
      return "secondary"
  }
}

function deviceKindLabel(kind: GatewayDevice["kind"]) {
  switch (kind) {
    case "reader":
      return "读卡器"
    case "door_controller":
      return "门控板"
    case "relay":
      return "继电器"
    case "sensor":
      return "传感器"
    case "legacy_reader":
      return "旧读卡器"
    case "legacy_controller":
      return "旧门控板"
    default:
      return kind
  }
}

function deviceSourceLabel(source: GatewayDevice["source"]) {
  switch (source) {
    case "mistypass_procured":
      return "我方采购"
    case "legacy_integration":
      return "客户旧设备"
    default:
      return source
  }
}

function deviceProtocolLabel(protocol: GatewayDevice["protocol"]) {
  switch (protocol) {
    case "wiegand_26":
      return "Wiegand 26"
    case "wiegand_34":
      return "Wiegand 34"
    case "osdp_v2":
      return "OSDP v2"
    case "rs485":
      return "RS485"
    case "ble":
      return "BLE"
    default:
      return protocol || "-"
  }
}

function serialInventoryStatusLabel(status: GatewaySerialInventoryItem["status"]) {
  switch (status) {
    case "available":
      return "可用"
    case "consumed":
      return "已核销"
    case "frozen":
      return "冻结"
    case "scrapped":
      return "报废"
    default:
      return status
  }
}

function serialInventoryStatusVariant(status: GatewaySerialInventoryItem["status"]) {
  switch (status) {
    case "available":
      return "outline"
    case "consumed":
      return "secondary"
    case "frozen":
      return "destructive"
    case "scrapped":
      return "destructive"
    default:
      return "secondary"
  }
}

function serialInventoryProductTypeLabel(productType: GatewaySerialInventoryItem["product_type"]) {
  switch (productType) {
    case "gateway":
      return "网关"
    case "reader":
      return "读卡器"
    case "controller":
      return "门控板"
    case "relay":
      return "继电器"
    case "sensor":
      return "传感器"
    default:
      return productType
  }
}

function parseSerialNumbers(value: string): string[] {
  const output = new Set<string>()
  const tokens = value.split(/[\s,;]+/g)
  for (const token of tokens) {
    const normalized = token.trim().toUpperCase()
    if (!normalized) {
      continue
    }
    output.add(normalized)
  }
  return Array.from(output)
}

type GatewaysPageData = {
  gateways: Gateway[]
  tenants: Tenant[]
  doors: Door[]
  serialInventory: GatewaySerialInventoryItem[]
}

type LoadGatewaysPageDataArgs = {
  token: string
  platformViewer: boolean
  viewerTenantID: string
  buildingAdmin: boolean
  missingBuildingScope: boolean
  viewerBuildingIDs: Set<string>
  inventoryVisible: boolean
}

async function loadGatewaysPageData(args: LoadGatewaysPageDataArgs): Promise<GatewaysPageData> {
  const [gatewayItems, doorItems, tenantItems, serialInventoryItems] = await Promise.all([
    listGateways(args.token),
    listDoors(args.token),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
    args.inventoryVisible
      ? listGatewaySerialInventory(args.token, args.platformViewer ? undefined : args.viewerTenantID || undefined)
      : Promise.resolve([]),
  ])
  const scopedGateways = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? gatewayItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : gatewayItems
  const scopedDoors = args.missingBuildingScope
    ? []
    : args.buildingAdmin
      ? doorItems.filter((item) => args.viewerBuildingIDs.has(item.building_id))
      : doorItems
  return {
    gateways: scopedGateways,
    doors: scopedDoors,
    tenants: tenantItems,
    serialInventory: serialInventoryItems,
  }
}

export function GatewaysPage({ token, viewer }: GatewaysPageProps) {
  const queryClient = useQueryClient()
  const platformViewer = isPlatformViewer(viewer)
  const buildingAdmin = isBuildingAdmin(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const viewerBuildingIDList = useMemo(() => getViewerBuildingIDs(viewer), [viewer])
  const viewerBuildingIDs = useMemo(() => new Set(viewerBuildingIDList), [viewerBuildingIDList])
  const missingBuildingScope = buildingAdmin && viewerBuildingIDs.size === 0
  const inventoryVisible = canAccessGatewayInventory(viewer)
  const inventoryEditable = canEditGatewayInventory(viewer)
  const gatewayOpsEditable = canManageGateways(viewer)
  const gatewayRegistrationVisible = canRegisterGateways(viewer)
  const readOnlyBoundaryHint = "按钮禁用或缺失属于权限边界，不是系统异常。"

  const [serialNumber, setSerialNumber] = useState("")
  const [tenantID, setTenantID] = useState("")
  const [buildingID, setBuildingID] = useState("")
  const [deviceCapacity, setDeviceCapacity] = useState<4 | 8>(4)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")
  const [query, setQuery] = useState("")
  const [gatewayStatusFilter, setGatewayStatusFilter] = useState<"all" | "online" | "offline">("all")

  const [selectedGateway, setSelectedGateway] = useState("")
  const [selectedDoorID, setSelectedDoorID] = useState("")
  const [selectedBoundDoorID, setSelectedBoundDoorID] = useState("")
  const [configVersion, setConfigVersion] = useState("v0.1.0")
  const [commandBusy, setCommandBusy] = useState(false)
  const [commandLog, setCommandLog] = useState("暂无下发命令。")
  const [commandTasks, setCommandTasks] = useState<CommandTask[]>([])
  const [deviceSerialNumber, setDeviceSerialNumber] = useState("")
  const [deviceKind, setDeviceKind] = useState<GatewayDeviceKind>("reader")
  const [deviceSource, setDeviceSource] = useState<GatewayDeviceSource>("mistypass_procured")
  const [deviceProtocol, setDeviceProtocol] = useState<GatewayDeviceProtocol>("auto")
  const [rs485BaudRate, setRS485BaudRate] = useState("9600")
  const [rs485Parity, setRS485Parity] = useState<"none" | "even" | "odd">("none")
  const [rs485StopBits, setRS485StopBits] = useState<1 | 2>(1)
  const [rs485Address, setRS485Address] = useState("1")
  const [rs485TimeoutMS, setRS485TimeoutMS] = useState("800")
  const [deviceStatus, setDeviceStatus] = useState<"online" | "offline">("online")
  const [legacyProbeCandidates, setLegacyProbeCandidates] = useState<string[]>([])
  const [inventorySerialNumber, setInventorySerialNumber] = useState("")
  const [inventoryProductType, setInventoryProductType] = useState<
    "gateway" | "reader" | "controller" | "relay" | "sensor"
  >("gateway")
  const [inventoryBatchCode, setInventoryBatchCode] = useState("")
  const [inventoryCSVContent, setInventoryCSVContent] = useState("")
  const [inventoryFilterProductType, setInventoryFilterProductType] = useState<
    "all" | GatewaySerialInventoryProductType
  >("all")
  const [inventoryFilterStatus, setInventoryFilterStatus] = useState<"all" | GatewaySerialInventoryStatus>("all")
  const [inventoryFilterQuery, setInventoryFilterQuery] = useState("")
  const [inventoryBatchStatus, setInventoryBatchStatus] = useState<"available" | "frozen" | "scrapped">("frozen")
  const [inventoryBatchSerials, setInventoryBatchSerials] = useState("")
  const [selectedInventorySerialNumbers, setSelectedInventorySerialNumbers] = useState<string[]>([])
  const [checkpointSummary, setCheckpointSummary] = useState<GatewayCheckpointSummaryResponse | null>(null)
  const [checkpointSummaryLoading, setCheckpointSummaryLoading] = useState(false)
  const [checkpointSummaryError, setCheckpointSummaryError] = useState("")
  const [checkpointTrendWindowMinutes, setCheckpointTrendWindowMinutes] = useState<"15" | "60" | "240">("60")

  const gatewaysQueryKey = useMemo(
    () =>
      [
        "gateways-page-data",
        token,
        platformViewer ? "platform" : "tenant",
        viewerTenantID,
        buildingAdmin ? "building-admin" : "regular",
        missingBuildingScope ? "missing-scope" : "scope-ready",
        inventoryVisible ? "inventory-on" : "inventory-off",
        viewerBuildingIDList.join(","),
      ] as const,
    [
      buildingAdmin,
      inventoryVisible,
      missingBuildingScope,
      platformViewer,
      token,
      viewerBuildingIDList,
      viewerTenantID,
    ]
  )
  const gatewaysPageQuery = useQuery({
    queryKey: gatewaysQueryKey,
    queryFn: () =>
      loadGatewaysPageData({
        token,
        platformViewer,
        viewerTenantID,
        buildingAdmin,
        missingBuildingScope,
        viewerBuildingIDs: new Set(viewerBuildingIDList),
        inventoryVisible,
      }),
    staleTime: 30 * 1000,
  })
  const gateways = gatewaysPageQuery.data?.gateways ?? []
  const tenants = gatewaysPageQuery.data?.tenants ?? []
  const doors = gatewaysPageQuery.data?.doors ?? []
  const serialInventory = gatewaysPageQuery.data?.serialInventory ?? []
  const loading = gatewaysPageQuery.isPending
  const queryError =
    gatewaysPageQuery.error instanceof Error ? gatewaysPageQuery.error.message : ""

  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])
  const gatewayByID = useMemo(() => new Map(gateways.map((item) => [item.id, item])), [gateways])
  const doorByID = useMemo(() => new Map(doors.map((item) => [item.id, item])), [doors])
  const selectedGatewayRecord = gatewayByID.get(selectedGateway)
  const selectedGatewayDevices = selectedGatewayRecord?.devices ?? []
  const selectedGatewayDeviceOnline = selectedGatewayDevices.filter((item) => item.status === "online").length
  const selectedGatewayRemainSlots = Math.max(
    0,
    (selectedGatewayRecord?.device_capacity ?? 0) - selectedGatewayDevices.length
  )
  const availableDoors = useMemo(() => {
    const bound = new Set(selectedGatewayRecord?.bound_door_ids ?? [])
    return doors.filter((item) => {
      if (selectedGatewayRecord?.tenant_id && item.tenant_id !== selectedGatewayRecord.tenant_id) {
        return false
      }
      if (selectedGatewayRecord?.building_id && item.building_id !== selectedGatewayRecord.building_id) {
        return false
      }
      if (bound.has(item.id)) {
        return false
      }
      return true
    })
  }, [doors, selectedGatewayRecord])
  const boundDoors = useMemo(() => {
    const ids = selectedGatewayRecord?.bound_door_ids ?? []
    return ids.map((id) => ({
      id,
      name: doorByID.get(id)?.name ?? id,
    }))
  }, [doorByID, selectedGatewayRecord?.bound_door_ids])
  const gatewayDeviceOnlineTotal = useMemo(() => {
    return gateways.reduce((sum, item) => {
      return sum + (item.devices ?? []).filter((device) => device.status === "online").length
    }, 0)
  }, [gateways])
  const gatewayDeviceTotal = useMemo(() => {
    return gateways.reduce((sum, item) => sum + (item.devices?.length ?? 0), 0)
  }, [gateways])
  const inventoryManualBatchSerialNumbers = useMemo(() => parseSerialNumbers(inventoryBatchSerials), [inventoryBatchSerials])
  const visibleSerialInventory = useMemo(() => {
    const nextTenantID = tenantID.trim()
    const nextProductType = inventoryFilterProductType === "all" ? "" : inventoryFilterProductType.trim()
    const nextStatus = inventoryFilterStatus === "all" ? "" : inventoryFilterStatus.trim()
    const keyword = inventoryFilterQuery.trim().toLowerCase()
    const rows = (
      nextTenantID === ""
        ? serialInventory
        : serialInventory.filter((item) => item.tenant_id === nextTenantID)
    )
      .filter((item) => {
        if (nextProductType && item.product_type !== nextProductType) {
          return false
        }
        if (nextStatus && item.status !== nextStatus) {
          return false
        }
        if (!keyword) {
          return true
        }
        return (
          item.serial_number.toLowerCase().includes(keyword) ||
          (item.batch_code ?? "").toLowerCase().includes(keyword) ||
          (item.consumed_gateway_id ?? "").toLowerCase().includes(keyword)
        )
      })
    return [...rows].sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
  }, [inventoryFilterProductType, inventoryFilterQuery, inventoryFilterStatus, serialInventory, tenantID])
  const selectedInventorySerialSet = useMemo(
    () => new Set(selectedInventorySerialNumbers),
    [selectedInventorySerialNumbers]
  )
  const selectedVisibleInventoryCount = useMemo(() => {
    return visibleSerialInventory.reduce(
      (sum, item) => (selectedInventorySerialSet.has(item.serial_number) ? sum + 1 : sum),
      0
    )
  }, [selectedInventorySerialSet, visibleSerialInventory])
  const allVisibleInventorySelected =
    visibleSerialInventory.length > 0 && selectedVisibleInventoryCount === visibleSerialInventory.length
  const inventoryBatchTargetSerialNumbers = useMemo(() => {
    if (inventoryManualBatchSerialNumbers.length > 0) {
      return inventoryManualBatchSerialNumbers
    }
    return selectedInventorySerialNumbers
  }, [inventoryManualBatchSerialNumbers, selectedInventorySerialNumbers])

  const filteredGateways = useMemo(() => {
    const q = query.trim().toLowerCase()
    return gateways.filter((item) => {
      if (gatewayStatusFilter !== "all" && item.status !== gatewayStatusFilter) {
        return false
      }
      if (!q) {
        return true
      }
      const tenantName = tenantByID.get(item.tenant_id)?.name ?? item.tenant_id
      return (
        item.id.toLowerCase().includes(q) ||
        item.serial_number.toLowerCase().includes(q) ||
        item.building_id.toLowerCase().includes(q) ||
        item.tenant_id.toLowerCase().includes(q) ||
        tenantName.toLowerCase().includes(q)
      )
    })
  }, [gatewayStatusFilter, gateways, query, tenantByID])
  const hasActiveGatewayFilters = query.trim().length > 0 || gatewayStatusFilter !== "all"

  function patchGatewayInCache(updated: Gateway) {
    queryClient.setQueryData<GatewaysPageData>(gatewaysQueryKey, (current) => {
      if (!current) {
        return current
      }
      return {
        ...current,
        gateways: current.gateways.map((item) => (item.id === updated.id ? updated : item)),
      }
    })
  }

  useEffect(() => {
    const inventorySerialSet = new Set(serialInventory.map((item) => item.serial_number))
    setSelectedInventorySerialNumbers((current) => current.filter((item) => inventorySerialSet.has(item)))
  }, [serialInventory])

  useEffect(() => {
    setSelectedGateway((current) => {
      if (current && gateways.some((item) => item.id === current)) {
        return current
      }
      return gateways[0]?.id ?? ""
    })
  }, [gateways])

  useEffect(() => {
    setSelectedDoorID((current) => {
      if (current && availableDoors.some((item) => item.id === current)) {
        return current
      }
      return availableDoors[0]?.id ?? ""
    })
  }, [availableDoors])

  useEffect(() => {
    setSelectedBoundDoorID((current) => {
      if (current && boundDoors.some((item) => item.id === current)) {
        return current
      }
      return boundDoors[0]?.id ?? ""
    })
  }, [boundDoors])

  useEffect(() => {
    setLegacyProbeCandidates([])
    setDeviceSerialNumber("")
  }, [selectedGateway])

  useEffect(() => {
    if (!platformViewer) {
      setTenantID(viewerTenantID)
    }
  }, [platformViewer, viewerTenantID])

  useEffect(() => {
    if (!platformViewer || tenantID || !tenants[0]) {
      return
    }
    setTenantID(tenants[0].id)
  }, [platformViewer, tenantID, tenants])

  useEffect(() => {
    const selected = gatewayByID.get(selectedGateway)
    if (!selected) {
      setCheckpointSummary(null)
      setCheckpointSummaryError("")
      return
    }
    const selectedRecord = selected

    let canceled = false
    async function loadCheckpointSummary() {
      setCheckpointSummaryLoading(true)
      setCheckpointSummaryError("")
      try {
        const summary = await listGatewayEventCheckpointSummary(token, {
          tenant_id: selectedRecord.tenant_id || undefined,
          gateway_id: selectedRecord.id,
          trend_window_minutes: Number(checkpointTrendWindowMinutes),
          limit: 20,
        })
        if (canceled) {
          return
        }
        setCheckpointSummary(summary)
      } catch (err) {
        if (canceled) {
          return
        }
        const message = err instanceof Error ? err.message : "加载 checkpoint 摘要失败"
        setCheckpointSummaryError(message)
      } finally {
        if (!canceled) {
          setCheckpointSummaryLoading(false)
        }
      }
    }

    void loadCheckpointSummary()
    return () => {
      canceled = true
    }
  }, [checkpointTrendWindowMinutes, gatewayByID, selectedGateway, token])

  function pushCommandProgress(ack: GatewayCommandAck) {
    const createdAt = ack.created_at || new Date().toISOString()
    const base: CommandTask = {
      task_id: ack.task_id,
      gateway_id: ack.gateway_id,
      command: ack.command,
      status: "queued",
      created_at: createdAt,
      updated_at: createdAt,
    }
    setCommandTasks((current) => [base, ...current].slice(0, 12))

    const stages: CommandTaskStatus[] = ["dispatching", "delivered", "acknowledged"]
    stages.forEach((nextStatus, index) => {
      window.setTimeout(() => {
        setCommandTasks((current) =>
          current.map((item) =>
            item.task_id === ack.task_id
              ? {
                  ...item,
                  status: nextStatus,
                  updated_at: new Date().toISOString(),
                }
              : item
          )
        )
      }, (index + 1) * 1200)
    })
  }

  async function onRegisterGateway(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!serialNumber.trim() || !tenantID) {
      return
    }

    setSubmitting(true)
    setError("")
    try {
      const created = await registerGateway(token, {
        serial_number: serialNumber.trim(),
        tenant_id: tenantID,
        building_id: buildingID.trim() || undefined,
        device_capacity: deviceCapacity,
      })
      queryClient.setQueryData<GatewaysPageData>(gatewaysQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          gateways: [created, ...current.gateways],
        }
      })
      setSelectedGateway(created.id)
      setSerialNumber("")
      setBuildingID("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "注册网关失败"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  async function onImportSerialInventory(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!tenantID.trim() || !inventorySerialNumber.trim()) {
      return
    }

    setSubmitting(true)
    setError("")
    try {
      await importGatewaySerialInventory(token, {
        tenant_id: tenantID.trim(),
        items: [
          {
            serial_number: inventorySerialNumber.trim(),
            product_type: inventoryProductType,
            batch_code: inventoryBatchCode.trim() || undefined,
            source: "factory",
          },
        ],
      })
      setInventorySerialNumber("")
      setInventoryBatchCode("")
      setCommandLog(`序列号 ${inventorySerialNumber.trim()} 已入库（${inventoryProductType}）`)
      await gatewaysPageQuery.refetch()
    } catch (err) {
      const message = err instanceof Error ? err.message : "导入序列号失败"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  async function onImportSerialInventoryCSV(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!tenantID.trim() || !inventoryCSVContent.trim()) {
      return
    }

    setSubmitting(true)
    setError("")
    try {
      const importedItems = await importGatewaySerialInventoryCSV(token, {
        tenant_id: tenantID.trim(),
        csv_content: inventoryCSVContent,
      })
      setInventoryCSVContent("")
      setCommandLog(`CSV 入库完成，新增/更新 ${importedItems.length} 条序列号记录`)
      await gatewaysPageQuery.refetch()
    } catch (err) {
      const message = err instanceof Error ? err.message : "CSV 入库失败"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  async function onExportSerialInventoryCSV() {
    setCommandBusy(true)
    setError("")
    try {
      const csvContent = await exportGatewaySerialInventoryCSV(
        token,
        tenantID.trim() || undefined,
        {
          product_type: inventoryFilterProductType === "all" ? undefined : inventoryFilterProductType,
          status: inventoryFilterStatus === "all" ? undefined : inventoryFilterStatus,
        }
      )
      const stamp = new Date().toISOString().replace(/[:.]/g, "-")
      const scope = tenantID.trim() || "all"
      const fileName = `gateway-serial-inventory-${scope}-${stamp}.csv`
      const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8" })
      const url = window.URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = url
      anchor.download = fileName
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      window.URL.revokeObjectURL(url)
      setCommandLog(`库存 CSV 已导出（${fileName}）`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "导出库存 CSV 失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onUpdateSerialInventoryStatus(
    item: GatewaySerialInventoryItem,
    status: "available" | "frozen" | "scrapped"
  ) {
    setCommandBusy(true)
    setError("")
    try {
      await updateGatewaySerialInventoryStatus(token, item.serial_number, {
        tenant_id: item.tenant_id,
        status,
      })
      setCommandLog(`序列号 ${item.serial_number} 状态已更新为 ${serialInventoryStatusLabel(status)}`)
      await gatewaysPageQuery.refetch()
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新序列号状态失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  function onSelectSerialInventory(serialNumber: string, checked: boolean) {
    setSelectedInventorySerialNumbers((current) => {
      if (checked) {
        if (current.includes(serialNumber)) {
          return current
        }
        return [...current, serialNumber]
      }
      return current.filter((item) => item !== serialNumber)
    })
  }

  function onSelectAllVisibleSerialInventory(checked: boolean) {
    const visibleSerials = visibleSerialInventory.map((item) => item.serial_number)
    if (visibleSerials.length === 0) {
      return
    }
    setSelectedInventorySerialNumbers((current) => {
      if (!checked) {
        const removable = new Set(visibleSerials)
        return current.filter((item) => !removable.has(item))
      }
      const merged = new Set(current)
      visibleSerials.forEach((item) => merged.add(item))
      return Array.from(merged)
    })
  }

  async function onBatchUpdateSerialInventoryStatus() {
    if (!tenantID.trim() || inventoryBatchTargetSerialNumbers.length === 0) {
      return
    }

    setCommandBusy(true)
    setError("")
    try {
      const updatedItems = await batchUpdateGatewaySerialInventoryStatus(token, {
        tenant_id: tenantID.trim(),
        status: inventoryBatchStatus,
        serial_numbers: inventoryBatchTargetSerialNumbers,
      })
      setSelectedInventorySerialNumbers((current) =>
        current.filter((item) => !inventoryBatchTargetSerialNumbers.includes(item))
      )
      setInventoryBatchSerials("")
      setCommandLog(
        `批量状态更新完成：${updatedItems.length} 条序列号已设置为 ${serialInventoryStatusLabel(inventoryBatchStatus)}`
      )
      await gatewaysPageQuery.refetch()
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量更新序列号状态失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onBindDoor() {
    if (!selectedGateway || !selectedDoorID.trim()) {
      return
    }
    setCommandBusy(true)
    setError("")
    try {
      const updated = await bindGatewayDoor(token, selectedGateway, selectedDoorID.trim())
      patchGatewayInCache(updated)
      setCommandLog(`门点 ${selectedDoorID.trim()} 已绑定到 ${selectedGateway}`)
      setSelectedDoorID("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "绑定门点失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onUnbindDoor() {
    if (!selectedGateway || !selectedBoundDoorID.trim()) {
      return
    }
    setCommandBusy(true)
    setError("")
    try {
      const updated = await unbindGatewayDoor(token, selectedGateway, selectedBoundDoorID.trim())
      patchGatewayInCache(updated)
      setCommandLog(`门点 ${selectedBoundDoorID.trim()} 已从 ${selectedGateway} 解绑`)
      setSelectedBoundDoorID("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "解绑门点失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onRegisterGatewayDevice() {
    if (!selectedGateway || !deviceSerialNumber.trim()) {
      return
    }
    setCommandBusy(true)
    setError("")
    try {
      const payload: Parameters<typeof registerGatewayDevice>[2] = {
        serial_number: deviceSerialNumber.trim(),
        kind: deviceKind,
        source: deviceSource,
        status: deviceStatus,
      }
      if (deviceProtocol !== "auto") {
        payload.protocol = deviceProtocol
      }
      if (deviceProtocol === "rs485") {
        payload.rs485_config = {
          baud_rate: Number(rs485BaudRate) || 0,
          parity: rs485Parity,
          stop_bits: rs485StopBits,
          device_address: Number(rs485Address) || 0,
          timeout_ms: Number(rs485TimeoutMS) || 0,
        }
      }

      const updated = await registerGatewayDevice(token, selectedGateway, payload)
      patchGatewayInCache(updated)
      setCommandLog(`设备 ${deviceSerialNumber.trim()} 已挂载到 ${selectedGateway}`)
      setLegacyProbeCandidates((current) => current.filter((item) => item !== deviceSerialNumber.trim()))
      setDeviceSerialNumber("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "挂载设备失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onProbeLegacyDevices() {
    if (!selectedGateway) {
      return
    }
    setCommandBusy(true)
    setError("")
    try {
      const items = await probeGatewayLegacyDevices(token, selectedGateway)
      setLegacyProbeCandidates(items)
      if (items[0]) {
        setDeviceSerialNumber(items[0])
      }
      setDeviceSource("legacy_integration")
      setCommandLog(`已探测到 ${items.length} 个 legacy 设备序列号候选`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "探测 legacy 设备失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onPublishConfig() {
    if (!selectedGateway || !configVersion.trim()) {
      return
    }
    setCommandBusy(true)
    setError("")
    try {
      const ack = await publishGatewayConfig(token, selectedGateway, configVersion.trim())
      setCommandLog(`配置发布任务已入队（${ack.task_id}），目标 ${ack.gateway_id}`)
      pushCommandProgress(ack)
    } catch (err) {
      const message = err instanceof Error ? err.message : "发布配置失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  async function onRebootGateway() {
    if (!selectedGateway) {
      return
    }
    setCommandBusy(true)
    setError("")
    try {
      const ack = await rebootGateway(token, selectedGateway)
      setCommandLog(`重启任务已入队（${ack.task_id}），目标 ${ack.gateway_id}`)
      pushCommandProgress(ack)
      queryClient.setQueryData<GatewaysPageData>(gatewaysQueryKey, (current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          gateways: current.gateways.map((item) =>
            item.id === selectedGateway
              ? {
                  ...item,
                  status: "online",
                  last_seen_at: new Date().toISOString(),
                }
              : item
          ),
        }
      })
      window.setTimeout(() => {
        void gatewaysPageQuery.refetch()
      }, 1500)
    } catch (err) {
      const message = err instanceof Error ? err.message : "下发重启失败"
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  const effectiveError = error || queryError

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">网关运维</p>
        <h1 className="mp-page-title">
          {platformViewer ? "网关" : buildingAdmin ? "楼宇网关与边缘设备" : "组织网关与边缘设备"}
        </h1>
        <p className="mp-page-description">
          {platformViewer
            ? "网关必须归属租户，并与租户门点做绑定下发。"
            : buildingAdmin
              ? "聚焦负责楼宇的网关、门点绑定和边缘设备状态，不暴露非本楼宇范围。"
              : "聚焦当前组织的网关、门点绑定和边缘设备状态，不再暴露租户切换。"}
        </p>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何网关、门点或边缘设备数据，避免误操作非本楼宇设备。
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          当前仅管理负责楼宇范围内的网关和门点绑定；网关注册仍由企业管理员或平台管理员执行。
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>网关总数</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {gateways.length} <CpuIcon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {platformViewer ? "全租户设备台账。" : buildingAdmin ? "负责楼宇范围内的网关台账。" : "当前组织网关台账。"}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>在线</CardDescription>
            <CardTitle className="text-2xl">
              {gateways.filter((item) => item.status === "online").length}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">心跳在预期窗口内。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>门点绑定数</CardDescription>
            <CardTitle className="text-2xl">
              {gateways.reduce((sum, item) => sum + (item.bound_door_ids?.length ?? 0), 0)}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {platformViewer ? "租户门点绑定关系。" : buildingAdmin ? "负责楼宇内的门点绑定关系。" : "组织内门点绑定关系。"}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>下挂设备在线</CardDescription>
            <CardTitle className="text-2xl">
              {gatewayDeviceOnlineTotal}/{gatewayDeviceTotal}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">读卡器/旧设备在线统计。</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="gap-3 md:flex md:flex-row md:items-end md:justify-between">
          <div>
            <CardTitle className="text-base">Checkpoint 时间窗口趋势</CardTitle>
            <CardDescription>按当前选中网关统计最近窗口的 checkpoint 审计趋势。</CardDescription>
          </div>
          <div className="w-full max-w-[220px]">
            <Label className="mb-1 block text-xs text-muted-foreground">趋势窗口</Label>
            <Select
              value={checkpointTrendWindowMinutes}
              onValueChange={(value: "15" | "60" | "240") => setCheckpointTrendWindowMinutes(value)}
            >
              <SelectTrigger>
                <SelectValue placeholder="选择窗口" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="15">近 15 分钟</SelectItem>
                <SelectItem value="60">近 60 分钟</SelectItem>
                <SelectItem value="240">近 4 小时</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {!selectedGatewayRecord ? (
            <p className="text-sm text-muted-foreground">请选择一个网关后查看窗口趋势。</p>
          ) : checkpointSummaryLoading ? (
            <p className="text-sm text-muted-foreground">正在加载 checkpoint 趋势...</p>
          ) : checkpointSummaryError ? (
            <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {checkpointSummaryError}
            </div>
          ) : (
            <>
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <div className="rounded-md border bg-muted/20 px-3 py-2">
                  <p className="mp-kpi-note">窗口上报次数</p>
                  <p className="text-xl font-semibold">{checkpointSummary?.time_window_trend.report_total ?? 0}</p>
                </div>
                <div className="rounded-md border bg-muted/20 px-3 py-2">
                  <p className="mp-kpi-note">Acked 增量</p>
                  <p className="text-xl font-semibold">{checkpointSummary?.time_window_trend.acked_delta_total ?? 0}</p>
                </div>
                <div className="rounded-md border bg-muted/20 px-3 py-2">
                  <p className="mp-kpi-note">覆盖队列数</p>
                  <p className="text-xl font-semibold">{checkpointSummary?.time_window_trend.queue_total ?? 0}</p>
                </div>
                <div className="rounded-md border bg-muted/20 px-3 py-2">
                  <p className="mp-kpi-note">总体方向</p>
                  <div className="mt-1">
                    <Badge
                      variant={checkpointTrendDirectionVariant(checkpointSummary?.time_window_trend.direction ?? "flat")}
                    >
                      {checkpointTrendDirectionLabel(checkpointSummary?.time_window_trend.direction ?? "flat")}
                    </Badge>
                  </div>
                </div>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>队列</TableHead>
                    <TableHead>Lag</TableHead>
                    <TableHead>Acked</TableHead>
                    <TableHead>窗口上报</TableHead>
                    <TableHead>窗口方向</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {checkpointSummary?.items.length ? null : (
                    <TableRow>
                      <TableCell colSpan={5} className="py-4 text-center text-muted-foreground">
                        当前网关暂无 checkpoint 队列数据
                      </TableCell>
                    </TableRow>
                  )}
                  {(checkpointSummary?.items ?? []).slice(0, 5).map((item) => (
                    <TableRow key={`${item.gateway_id}-${item.queue}`}>
                      <TableCell className="font-medium">
                        <TableCellText className="max-w-[12rem]">{item.queue}</TableCellText>
                      </TableCell>
                      <TableCell>{item.lag_count}</TableCell>
                      <TableCell>{item.acked_count}</TableCell>
                      <TableCell>{item.time_window_trend.report_total}</TableCell>
                      <TableCell>
                        <Badge variant={checkpointTrendDirectionVariant(item.time_window_trend.direction)}>
                          {checkpointTrendDirectionLabel(item.time_window_trend.direction)}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}
        </CardContent>
      </Card>

      {gatewayRegistrationVisible ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">注册网关</CardTitle>
            <CardDescription>{platformViewer ? "注册时指定归属租户和可挂载设备数（4/8）。" : "当前组织下注册新网关并绑定楼宇范围。"}</CardDescription>
          </CardHeader>
          <CardContent>
            <form
              className={`grid gap-3 ${platformViewer ? "md:grid-cols-[1fr_220px_1fr_160px_auto]" : "md:grid-cols-[1fr_1fr_160px_auto]"}`}
              onSubmit={onRegisterGateway}
            >
              <Input
                value={serialNumber}
                onChange={(event) => setSerialNumber(event.target.value)}
                placeholder="序列号（必填）"
              />
              {platformViewer ? (
                <Select value={tenantID} onValueChange={setTenantID}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择租户" />
                  </SelectTrigger>
                  <SelectContent>
                    {tenants.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : null}
              <Input
                value={buildingID}
                onChange={(event) => setBuildingID(event.target.value)}
                placeholder="楼宇编号（可选）"
              />
              <Select
                value={String(deviceCapacity)}
                onValueChange={(value) => setDeviceCapacity((value === "8" ? 8 : 4))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="设备容量" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="4">4 设备网关</SelectItem>
                  <SelectItem value="8">8 设备网关</SelectItem>
                </SelectContent>
              </Select>
              <Button type="submit" disabled={submitting || !tenantID}>
                {submitting ? "注册中..." : "注册"}
              </Button>
            </form>
          </CardContent>
        </Card>
      ) : null}

      {inventoryVisible ? (
        <Card>
        <CardHeader>
          <CardTitle className="text-base">序列号库存</CardTitle>
          <CardDescription>
            {inventoryEditable
              ? "我方设备需先入库，再注册核销。支持回库、冻结、报废状态流转。"
              : "查看当前组织的序列号库存与核销状态，只读角色可导出当前筛选结果。"}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {inventoryEditable ? (
            <>
              <form className="grid gap-3 md:grid-cols-[1.4fr_220px_1fr_auto]" onSubmit={onImportSerialInventory}>
                <Input
                  value={inventorySerialNumber}
                  onChange={(event) => setInventorySerialNumber(event.target.value)}
                  placeholder="序列号（如 MP-GW-xxx / RD-xxx）"
                />
                <Select
                  value={inventoryProductType}
                  onValueChange={(value: "gateway" | "reader" | "controller" | "relay" | "sensor") =>
                    setInventoryProductType(value)
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder="产品类型" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="gateway">网关</SelectItem>
                    <SelectItem value="reader">读卡器</SelectItem>
                    <SelectItem value="controller">门控板</SelectItem>
                    <SelectItem value="relay">继电器</SelectItem>
                    <SelectItem value="sensor">传感器</SelectItem>
                  </SelectContent>
                </Select>
                <Input
                  value={inventoryBatchCode}
                  onChange={(event) => setInventoryBatchCode(event.target.value)}
                  placeholder="批次号（可选）"
                />
                <Button type="submit" disabled={submitting || !tenantID.trim()}>
                  {submitting ? "入库中..." : "入库"}
                </Button>
              </form>
              <form className="space-y-2 rounded-lg border bg-muted/20 p-3" onSubmit={onImportSerialInventoryCSV}>
                <p className="text-xs font-medium text-muted-foreground">CSV 批量入库</p>
                <Textarea
                  value={inventoryCSVContent}
                  onChange={(event) => setInventoryCSVContent(event.target.value)}
                  rows={5}
                  placeholder={
                    "serial_number,product_type,batch_code,source\nMP-GW-FACTORY-0001,gateway,batch-01,factory\nRD-FACTORY-0002,reader,batch-01,factory"
                  }
                />
                <div className="flex flex-wrap items-center gap-2">
                  <Button type="submit" variant="secondary" disabled={submitting || !tenantID.trim()}>
                    {submitting ? "CSV 导入中..." : "CSV 入库"}
                  </Button>
                  <Button type="button" variant="outline" disabled={commandBusy} onClick={onExportSerialInventoryCSV}>
                    导出当前筛选 CSV
                  </Button>
                </div>
                <p className="mp-kpi-note">
                  支持首行表头；列顺序：`serial_number,product_type,batch_code,source`，其中 `batch_code/source` 可留空。
                </p>
              </form>
            </>
          ) : (
            <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-3">
              <p className="text-sm text-muted-foreground">
                当前角色为只读库存视图，仅支持导出当前筛选结果。{readOnlyBoundaryHint}
              </p>
              <Button type="button" variant="outline" disabled={commandBusy} onClick={onExportSerialInventoryCSV}>
                导出当前筛选 CSV
              </Button>
            </div>
          )}
          <p className="mp-kpi-note">
            当前{platformViewer ? "租户" : "组织"}库存统计：
            可用 {visibleSerialInventory.filter((item) => item.status === "available").length} /
            已核销 {visibleSerialInventory.filter((item) => item.status === "consumed").length} /
            冻结 {visibleSerialInventory.filter((item) => item.status === "frozen").length} /
            报废 {visibleSerialInventory.filter((item) => item.status === "scrapped").length}
          </p>
        </CardContent>
        </Card>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">网关命令中心</CardTitle>
            <CardDescription>
              {gatewayOpsEditable
                ? "执行门点绑定/解绑、下挂设备管理、配置发布和重启下发。"
                : "当前角色仅查看网关、checkpoint 和下挂设备状态。"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {!gatewayOpsEditable ? (
              <div className="rounded-lg border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
                当前角色无网关写权限，仅可查看状态。{readOnlyBoundaryHint}
              </div>
            ) : null}
            <div className="space-y-1.5">
              <Label>目标网关</Label>
              <Select value={selectedGateway} onValueChange={setSelectedGateway}>
                <SelectTrigger>
                  <SelectValue placeholder="选择网关" />
                </SelectTrigger>
                <SelectContent>
                  {gateways.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.id} ({item.serial_number})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2 md:grid-cols-[1fr_auto]">
              <Select value={selectedDoorID} onValueChange={setSelectedDoorID}>
                <SelectTrigger>
                  <SelectValue placeholder="选择待绑定门点 ID" />
                </SelectTrigger>
                <SelectContent>
                  {availableDoors.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name} ({item.id})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button variant="outline" disabled={commandBusy || !gatewayOpsEditable} onClick={onBindDoor}>
                <ShieldEllipsisIcon className="mr-1.5 size-4" />
                绑定门点
              </Button>
            </div>
            <div className="grid gap-2 md:grid-cols-[1fr_auto]">
              <Select value={selectedBoundDoorID} onValueChange={setSelectedBoundDoorID}>
                <SelectTrigger>
                  <SelectValue placeholder="选择待解绑门点 ID" />
                </SelectTrigger>
                <SelectContent>
                  {boundDoors.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name} ({item.id})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button variant="outline" disabled={commandBusy || !gatewayOpsEditable} onClick={onUnbindDoor}>
                <UnplugIcon className="mr-1.5 size-4" />
                解绑门点
              </Button>
            </div>
            {selectedGatewayRecord ? (
              <p className="mp-kpi-note">
                当前网关范围：{platformViewer
                  ? `租户 ${tenantByID.get(selectedGatewayRecord.tenant_id)?.name ?? selectedGatewayRecord.tenant_id}`
                  : buildingAdmin
                    ? "负责楼宇"
                    : "当前组织"}
                {selectedGatewayRecord.building_id ? ` / 楼宇 ${selectedGatewayRecord.building_id}` : ""}
              </p>
            ) : null}
            {selectedGatewayRecord && availableDoors.length === 0 ? (
              <p className="mp-kpi-note">当前无可绑定门点，请先在空间页创建门点或切换网关。</p>
            ) : null}

            <div className="grid gap-2 md:grid-cols-[1fr_auto_auto]">
              <Input
                value={configVersion}
                onChange={(event) => setConfigVersion(event.target.value)}
                placeholder="配置版本号"
                disabled={!gatewayOpsEditable}
              />
              <Button variant="secondary" disabled={commandBusy || !gatewayOpsEditable} onClick={onPublishConfig}>
                <SendIcon className="mr-1.5 size-4" />
                发布配置
              </Button>
              <Button variant="outline" disabled={commandBusy || !gatewayOpsEditable} onClick={onRebootGateway}>
                <RefreshCwIcon className="mr-1.5 size-4" />
                重启
              </Button>
            </div>

            <div className="rounded-lg border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              {commandLog}
            </div>

            <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-xs font-medium text-muted-foreground">下挂设备管理</p>
                <Badge variant={selectedGatewayRemainSlots > 0 ? "outline" : "destructive"}>
                  余量 {selectedGatewayRemainSlots}/{selectedGatewayRecord?.device_capacity ?? 0}
                </Badge>
              </div>
              <p className="mp-kpi-note">
                在线设备 {selectedGatewayDeviceOnline}/{selectedGatewayDevices.length}
              </p>
              <div className="grid gap-2 md:grid-cols-[1.1fr_0.8fr_0.8fr_0.8fr_0.8fr_auto]">
                <Input
                  value={deviceSerialNumber}
                  onChange={(event) => setDeviceSerialNumber(event.target.value)}
                  placeholder="设备序列号（支持旧设备）"
                  disabled={!gatewayOpsEditable}
                />
                <Select value={deviceKind} onValueChange={(value: GatewayDeviceKind) => setDeviceKind(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="设备类型" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="reader">读卡器</SelectItem>
                    <SelectItem value="door_controller">门控板</SelectItem>
                    <SelectItem value="relay">继电器</SelectItem>
                    <SelectItem value="sensor">传感器</SelectItem>
                    <SelectItem value="legacy_reader">旧读卡器</SelectItem>
                    <SelectItem value="legacy_controller">旧门控板</SelectItem>
                  </SelectContent>
                </Select>
                <Select value={deviceSource} onValueChange={(value: GatewayDeviceSource) => setDeviceSource(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="设备来源" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="mistypass_procured">我方采购</SelectItem>
                    <SelectItem value="legacy_integration">客户旧设备</SelectItem>
                  </SelectContent>
                </Select>
                <Select value={deviceStatus} onValueChange={(value: "online" | "offline") => setDeviceStatus(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="在线状态" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="online">在线</SelectItem>
                    <SelectItem value="offline">离线</SelectItem>
                  </SelectContent>
                </Select>
                <Select value={deviceProtocol} onValueChange={(value: GatewayDeviceProtocol) => setDeviceProtocol(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="协议（默认自动）" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="auto">自动（legacy/wiegand，modern/osdp）</SelectItem>
                    <SelectItem value="wiegand_26">Wiegand 26</SelectItem>
                    <SelectItem value="wiegand_34">Wiegand 34</SelectItem>
                    <SelectItem value="osdp_v2">OSDP v2</SelectItem>
                    <SelectItem value="rs485">RS485</SelectItem>
                    <SelectItem value="ble">BLE</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  variant="outline"
                  disabled={commandBusy || selectedGatewayRemainSlots <= 0 || !gatewayOpsEditable}
                  onClick={onRegisterGatewayDevice}
                >
                  <Plug2Icon className="mr-1.5 size-4" />
                  挂载设备
                </Button>
              </div>
              {deviceProtocol === "rs485" ? (
                <div className="grid gap-2 md:grid-cols-5">
                  <Input
                    value={rs485BaudRate}
                    onChange={(event) => setRS485BaudRate(event.target.value)}
                    placeholder="baud_rate (9600)"
                  />
                  <Select value={rs485Parity} onValueChange={(value: "none" | "even" | "odd") => setRS485Parity(value)}>
                    <SelectTrigger>
                      <SelectValue placeholder="parity" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">none</SelectItem>
                      <SelectItem value="even">even</SelectItem>
                      <SelectItem value="odd">odd</SelectItem>
                    </SelectContent>
                  </Select>
                  <Select value={String(rs485StopBits)} onValueChange={(value: string) => setRS485StopBits(value === "2" ? 2 : 1)}>
                    <SelectTrigger>
                      <SelectValue placeholder="stop_bits" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">stop_bits=1</SelectItem>
                      <SelectItem value="2">stop_bits=2</SelectItem>
                    </SelectContent>
                  </Select>
                  <Input
                    value={rs485Address}
                    onChange={(event) => setRS485Address(event.target.value)}
                    placeholder="device_address (1..247)"
                  />
                  <Input
                    value={rs485TimeoutMS}
                    onChange={(event) => setRS485TimeoutMS(event.target.value)}
                    placeholder="timeout_ms (100..5000)"
                  />
                </div>
              ) : null}
              <div className="flex flex-wrap items-center gap-2">
                <Button variant="secondary" size="sm" disabled={commandBusy || !gatewayOpsEditable} onClick={onProbeLegacyDevices}>
                  探测旧设备序列号
                </Button>
                {legacyProbeCandidates.map((item) => (
                  <Button
                    key={item}
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      setDeviceSerialNumber(item)
                      setDeviceSource("legacy_integration")
                    }}
                  >
                    {item}
                  </Button>
                ))}
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>序列号</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>来源</TableHead>
                    <TableHead>协议</TableHead>
                    <TableHead>状态</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {selectedGatewayDevices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="py-4 text-center text-muted-foreground">
                        暂无下挂设备
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {selectedGatewayDevices.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">
                        <TableCellText className="max-w-[14rem]">{item.serial_number}</TableCellText>
                      </TableCell>
                      <TableCell>{deviceKindLabel(item.kind)}</TableCell>
                      <TableCell>{deviceSourceLabel(item.source)}</TableCell>
                      <TableCell>
                        <div className="space-y-0.5">
                          <div>{deviceProtocolLabel(item.protocol)}</div>
                          {item.protocol === "rs485" && item.rs485_config ? (
                            <div className="text-[11px] text-muted-foreground">
                              {`addr=${item.rs485_config.device_address} baud=${item.rs485_config.baud_rate}`}
                            </div>
                          ) : null}
                          {item.protocol === "rs485" && item.rs485_health ? (
                            <div className="text-[11px] text-muted-foreground">
                              {`retry=${item.rs485_health.retry_count} timeout=${item.rs485_health.timeout_count} collision=${item.rs485_health.collision_count}`}
                            </div>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={item.status === "online" ? "outline" : "destructive"}>
                          {statusLabel(item.status)}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
              <p className="text-xs font-medium text-muted-foreground">命令进度</p>
              {commandTasks.length === 0 ? (
                <p className="mp-kpi-note">暂无命令记录。</p>
              ) : (
                <div className="space-y-2">
                  {commandTasks.map((item) => (
                    <div key={item.task_id} className="flex items-center justify-between gap-3 rounded-md border bg-background px-2 py-1.5">
                      <div className="min-w-0">
                        <p className="truncate text-xs font-medium">
                          {item.command} / {item.gateway_id}
                        </p>
                        <p className="text-[11px] text-muted-foreground">{item.task_id}</p>
                      </div>
                      <Badge variant={commandStatusVariant(item.status)}>{commandStatusLabel(item.status)}</Badge>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">网关检索</CardTitle>
            <CardDescription>{platformViewer ? "按租户、状态、网关编号、序列号或楼宇过滤。" : "按状态、网关编号、序列号或楼宇过滤。"}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-3 md:grid-cols-[1fr_220px_auto]">
              <div className="relative">
                <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  className="pl-8"
                  placeholder="搜索网关记录"
                />
              </div>
              <Select
                value={gatewayStatusFilter}
                onValueChange={(value: "all" | "online" | "offline") => {
                  setGatewayStatusFilter(value)
                }}
              >
                <SelectTrigger aria-label="网关状态筛选">
                  <SelectValue placeholder="筛选状态" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部状态</SelectItem>
                  <SelectItem value="online">在线</SelectItem>
                  <SelectItem value="offline">离线</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                onClick={() => {
                  setQuery("")
                  setGatewayStatusFilter("all")
                }}
              >
                重置网关筛选
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      {inventoryVisible ? (
        <Card>
        <CardHeader>
          <CardTitle className="text-base">序列号库存台账</CardTitle>
          <CardDescription>{inventoryEditable ? "查看核销状态，并执行回库/冻结/报废操作。" : "查看核销状态与库存变化。"}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 lg:grid-cols-[220px_220px_1fr_auto]">
            <Select
              value={inventoryFilterProductType}
              onValueChange={(value: "all" | GatewaySerialInventoryProductType) => setInventoryFilterProductType(value)}
            >
              <SelectTrigger>
                <SelectValue placeholder="筛选产品类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部产品类型</SelectItem>
                <SelectItem value="gateway">网关</SelectItem>
                <SelectItem value="reader">读卡器</SelectItem>
                <SelectItem value="controller">门控板</SelectItem>
                <SelectItem value="relay">继电器</SelectItem>
                <SelectItem value="sensor">传感器</SelectItem>
              </SelectContent>
            </Select>
            <Select
              value={inventoryFilterStatus}
              onValueChange={(value: "all" | GatewaySerialInventoryStatus) => setInventoryFilterStatus(value)}
            >
              <SelectTrigger>
                <SelectValue placeholder="筛选库存状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="available">可用</SelectItem>
                <SelectItem value="consumed">已核销</SelectItem>
                <SelectItem value="frozen">冻结</SelectItem>
                <SelectItem value="scrapped">报废</SelectItem>
              </SelectContent>
            </Select>
            <Input
              value={inventoryFilterQuery}
              onChange={(event) => setInventoryFilterQuery(event.target.value)}
              placeholder="搜索序列号 / 批次号 / 核销网关"
            />
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setInventoryFilterProductType("all")
                setInventoryFilterStatus("all")
                setInventoryFilterQuery("")
              }}
            >
              重置筛选
            </Button>
          </div>

          {inventoryEditable ? (
            <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
              <p className="text-xs font-medium text-muted-foreground">批量状态流转</p>
              <Textarea
                value={inventoryBatchSerials}
                onChange={(event) => setInventoryBatchSerials(event.target.value)}
                rows={3}
                placeholder="粘贴多个序列号（支持逗号、空格或换行分隔）。留空则使用下方表格勾选项。"
              />
              <div className="grid gap-2 md:grid-cols-[220px_auto_auto]">
                <Select
                  value={inventoryBatchStatus}
                  onValueChange={(value: "available" | "frozen" | "scrapped") => setInventoryBatchStatus(value)}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="目标状态" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="available">回库（available）</SelectItem>
                    <SelectItem value="frozen">冻结（frozen）</SelectItem>
                    <SelectItem value="scrapped">报废（scrapped）</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  type="button"
                  variant="secondary"
                  disabled={commandBusy || !tenantID.trim() || inventoryBatchTargetSerialNumbers.length === 0}
                  onClick={onBatchUpdateSerialInventoryStatus}
                >
                  批量更新状态
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setInventoryBatchSerials("")
                    setSelectedInventorySerialNumbers([])
                  }}
                >
                  清空批量目标
                </Button>
              </div>
              <p className="mp-kpi-note">
                当前批量目标 {inventoryBatchTargetSerialNumbers.length} 条（手工输入 {inventoryManualBatchSerialNumbers.length} 条，表格勾选 {selectedInventorySerialNumbers.length} 条）。
              </p>
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                {inventoryEditable ? (
                  <TableHead className="w-12">
                    <input
                      aria-label="select all visible serial inventory rows"
                      type="checkbox"
                      className="size-4 rounded border"
                      disabled={visibleSerialInventory.length === 0}
                      checked={allVisibleInventorySelected}
                      onChange={(event) => onSelectAllVisibleSerialInventory(event.target.checked)}
                    />
                  </TableHead>
                ) : null}
                <TableHead>序列号</TableHead>
                <TableHead>类型</TableHead>
                {platformViewer ? <TableHead>租户</TableHead> : null}
                <TableHead>状态</TableHead>
                <TableHead>核销网关</TableHead>
                <TableHead>更新时间</TableHead>
                {inventoryEditable ? <TableHead>操作</TableHead> : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleSerialInventory.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={inventoryEditable ? (platformViewer ? 8 : 7) : platformViewer ? 6 : 5} className="py-6 text-center text-muted-foreground">
                    暂无序列号库存记录
                  </TableCell>
                </TableRow>
              ) : null}
              {visibleSerialInventory.map((item) => (
                <TableRow key={item.id}>
                  {inventoryEditable ? (
                    <TableCell>
                      <input
                        aria-label={`select serial inventory ${item.serial_number}`}
                        type="checkbox"
                        className="size-4 rounded border"
                        checked={selectedInventorySerialSet.has(item.serial_number)}
                        onChange={(event) => onSelectSerialInventory(item.serial_number, event.target.checked)}
                      />
                    </TableCell>
                  ) : null}
                  <TableCell className="font-medium">
                    <TableCellText className="max-w-[14rem]">{item.serial_number}</TableCellText>
                  </TableCell>
                  <TableCell>{serialInventoryProductTypeLabel(item.product_type)}</TableCell>
                  {platformViewer ? (
                    <TableCell>
                      <TableCellText className="max-w-[12rem]">
                        {tenantByID.get(item.tenant_id)?.name ?? item.tenant_id}
                      </TableCellText>
                    </TableCell>
                  ) : null}
                  <TableCell>
                    <Badge variant={serialInventoryStatusVariant(item.status)}>{serialInventoryStatusLabel(item.status)}</Badge>
                  </TableCell>
                  <TableCell>{item.consumed_gateway_id || "-"}</TableCell>
                  <TableCell>{new Date(item.updated_at).toLocaleString("zh-CN")}</TableCell>
                  {inventoryEditable ? (
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={commandBusy || item.status === "available"}
                          onClick={() => {
                            void onUpdateSerialInventoryStatus(item, "available")
                          }}
                        >
                          回库
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={commandBusy || item.status === "frozen" || item.status === "scrapped"}
                          onClick={() => {
                            void onUpdateSerialInventoryStatus(item, "frozen")
                          }}
                        >
                          冻结
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={commandBusy || item.status === "scrapped"}
                          onClick={() => {
                            void onUpdateSerialInventoryStatus(item, "scrapped")
                          }}
                        >
                          报废
                        </Button>
                      </div>
                    </TableCell>
                  ) : null}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">网关台账</CardTitle>
          <CardDescription>{platformViewer ? "展示租户归属、在线时间和门点绑定关系。" : "展示当前组织的在线时间和门点绑定关系。"}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {effectiveError ? (
            <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {effectiveError}
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>网关编号</TableHead>
                {platformViewer ? <TableHead>租户</TableHead> : null}
                <TableHead>序列号</TableHead>
                <TableHead>楼宇</TableHead>
                <TableHead>设备容量</TableHead>
                <TableHead>设备在线</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>最近在线</TableHead>
                <TableHead>已绑门点</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 9 : 8} className="py-10 text-center text-muted-foreground">
                    正在加载网关...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredGateways.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={platformViewer ? 9 : 8} className="py-8 text-center text-muted-foreground">
                    {missingBuildingScope
                      ? "当前楼宇管理员尚未分配楼宇范围。"
                      : hasActiveGatewayFilters
                        ? "当前筛选条件下没有匹配的网关。"
                        : buildingAdmin
                          ? "当前楼宇范围内暂无网关。"
                          : "当前范围内暂无网关。"}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                filteredGateways.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">
                      <TableCellText className="max-w-[10rem]">{item.id}</TableCellText>
                    </TableCell>
                    {platformViewer ? (
                      <TableCell>
                        <TableCellText className="max-w-[12rem]">
                          {tenantByID.get(item.tenant_id)?.name ?? item.tenant_id}
                        </TableCellText>
                      </TableCell>
                    ) : null}
                    <TableCell>
                      <TableCellText className="max-w-[14rem]">{item.serial_number}</TableCellText>
                    </TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[10rem]">{item.building_id || "-"}</TableCellText>
                    </TableCell>
                    <TableCell>{item.device_capacity || "-"}</TableCell>
                    <TableCell>
                      {(item.devices ?? []).filter((device) => device.status === "online").length}/{item.devices?.length ?? 0}
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(item.status)} className="capitalize">
                        {statusLabel(item.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>{new Date(item.last_seen_at).toLocaleString("zh-CN")}</TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[16rem]">{item.bound_door_ids?.join(", ") || "-"}</TableCellText>
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
