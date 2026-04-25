import { useEffect, useMemo, useRef, useState } from "react"
import { CpuIcon, DoorOpenIcon, NetworkIcon, RadioTowerIcon } from "lucide-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

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
import { CheckpointMonitor } from "@/components/gateways/checkpoint-monitor"
import { GatewayCommandCenterCard } from "@/components/gateways/gateway-command-center-card"
import { GatewayListCard } from "@/components/gateways/gateway-list-card"
import { GatewaySearchCard } from "@/components/gateways/gateway-search-card"
import { GatewaySerialInventoryIngestCard } from "@/components/gateways/gateway-serial-inventory-ingest-card"
import { GatewaySerialInventoryLedgerCard } from "@/components/gateways/gateway-serial-inventory-ledger-card"
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

const gatewayDeviceCapacityValues = ["4", "8"] as const
type GatewayRegistrationFormValues = {
  serial_number: string
  tenant_id: string
  building_id?: string
  device_capacity: "4" | "8"
}

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

function statusLabel(status: string, t: (key: string) => string) {
  switch (status) {
    case "online":
      return t("gateways.status.online")
    case "offline":
      return t("gateways.status.offline")
    default:
      return status
  }
}

function commandStatusLabel(status: CommandTaskStatus, t: (key: string) => string) {
  switch (status) {
    case "queued":
      return t("gateways.commandStatus.queued")
    case "dispatching":
      return t("gateways.commandStatus.dispatching")
    case "delivered":
      return t("gateways.commandStatus.delivered")
    case "acknowledged":
      return t("gateways.commandStatus.acknowledged")
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

function deviceKindLabel(kind: GatewayDevice["kind"], t: (key: string) => string) {
  switch (kind) {
    case "reader":
      return t("gateways.deviceKind.reader")
    case "door_controller":
      return t("gateways.deviceKind.doorController")
    case "relay":
      return t("gateways.deviceKind.relay")
    case "sensor":
      return t("gateways.deviceKind.sensor")
    case "legacy_reader":
      return t("gateways.deviceKind.legacyReader")
    case "legacy_controller":
      return t("gateways.deviceKind.legacyController")
    default:
      return kind
  }
}

function deviceSourceLabel(source: GatewayDevice["source"], t: (key: string) => string) {
  switch (source) {
    case "mistypass_procured":
      return t("gateways.deviceSource.mistypassProcured")
    case "legacy_integration":
      return t("gateways.deviceSource.legacyIntegration")
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

function serialInventoryStatusLabel(status: GatewaySerialInventoryItem["status"], t: (key: string) => string) {
  switch (status) {
    case "available":
      return t("gateways.inventoryStatus.available")
    case "consumed":
      return t("gateways.inventoryStatus.consumed")
    case "frozen":
      return t("gateways.inventoryStatus.frozen")
    case "scrapped":
      return t("gateways.inventoryStatus.scrapped")
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

function serialInventoryProductTypeLabel(productType: GatewaySerialInventoryItem["product_type"], t: (key: string) => string) {
  switch (productType) {
    case "gateway":
      return t("gateways.inventoryProductType.gateway")
    case "reader":
      return t("gateways.inventoryProductType.reader")
    case "controller":
      return t("gateways.inventoryProductType.controller")
    case "relay":
      return t("gateways.inventoryProductType.relay")
    case "sensor":
      return t("gateways.inventoryProductType.sensor")
    default:
      return productType
  }
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
  const { t } = useTranslation()
  const gatewayRegistrationSchema = useMemo(
    () =>
      z.object({
        serial_number: z
          .string()
          .trim()
          .min(3, t("gateways.validation.serialNumberMin"))
          .max(128, t("gateways.validation.serialNumberMax")),
        tenant_id: z.string().trim().min(1, t("gateways.validation.tenantRequired")),
        building_id: z
          .string()
          .trim()
          .max(128, t("gateways.validation.buildingIdMax"))
          .optional()
          .or(z.literal("")),
        device_capacity: z.enum(gatewayDeviceCapacityValues),
      }),
    [t]
  )
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
  const readOnlyBoundaryHint = t("gateways.readOnlyBoundaryHint")

  const [tenantID, setTenantID] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")
  const [query, setQuery] = useState("")
  const [gatewayStatusFilter, setGatewayStatusFilter] = useState<"all" | "online" | "offline">("all")
  const gatewayRegistrationForm = useForm<GatewayRegistrationFormValues>({
    resolver: zodResolver(gatewayRegistrationSchema),
    defaultValues: {
      serial_number: "",
      tenant_id: "",
      building_id: "",
      device_capacity: "4",
    },
  })

  const [selectedGateway, setSelectedGateway] = useState("")
  const [selectedDoorID, setSelectedDoorID] = useState("")
  const [selectedBoundDoorID, setSelectedBoundDoorID] = useState("")
  const [configVersion, setConfigVersion] = useState("v0.1.0")
  const [commandBusy, setCommandBusy] = useState(false)
  const [commandLog, setCommandLog] = useState(t("gateways.commandLog.empty"))
  const [commandTasks, setCommandTasks] = useState<CommandTask[]>([])
  const pendingTimeoutIDsRef = useRef<number[]>([])
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
  const [inventoryFilterProductType, setInventoryFilterProductType] = useState<
    "all" | GatewaySerialInventoryProductType
  >("all")
  const [inventoryFilterStatus, setInventoryFilterStatus] = useState<"all" | GatewaySerialInventoryStatus>("all")
  const [inventoryFilterQuery, setInventoryFilterQuery] = useState("")
  const [inventoryPage, setInventoryPage] = useState(1)
  const [inventoryPageSize, setInventoryPageSize] = useState(25)
  const [selectedInventorySerialNumbers, setSelectedInventorySerialNumbers] = useState<string[]>([])
  const [checkpointSummary, setCheckpointSummary] = useState<GatewayCheckpointSummaryResponse | null>(null)
  const [checkpointSummaryLoading, setCheckpointSummaryLoading] = useState(false)
  const [checkpointSummaryError, setCheckpointSummaryError] = useState("")
  const [checkpointTrendWindowMinutes, setCheckpointTrendWindowMinutes] = useState<"15" | "60" | "240">("60")

  const gatewaysQueryKey = useMemo(
    () =>
      [
        "gateways-page-data",
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
  const inventoryMaxPage = Math.max(1, Math.ceil(visibleSerialInventory.length / inventoryPageSize))
  const pagedVisibleSerialInventory = useMemo(() => {
    const start = (inventoryPage - 1) * inventoryPageSize
    return visibleSerialInventory.slice(start, start + inventoryPageSize)
  }, [inventoryPage, inventoryPageSize, visibleSerialInventory])
  const inventoryHasNextPage = inventoryPage < inventoryMaxPage
  const selectedInventorySerialSet = useMemo(
    () => new Set(selectedInventorySerialNumbers),
    [selectedInventorySerialNumbers]
  )
  const selectedVisibleInventoryCount = useMemo(() => {
    return pagedVisibleSerialInventory.reduce(
      (sum, item) => (selectedInventorySerialSet.has(item.serial_number) ? sum + 1 : sum),
      0
    )
  }, [pagedVisibleSerialInventory, selectedInventorySerialSet])
  const allVisibleInventorySelected =
    pagedVisibleSerialInventory.length > 0 && selectedVisibleInventoryCount === pagedVisibleSerialInventory.length

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
  const gatewayStatusLabel = (status: string) => statusLabel(status, t)
  const gatewayCommandStatusLabel = (status: CommandTaskStatus) => commandStatusLabel(status, t)
  const gatewayDeviceKindLabel = (kind: GatewayDevice["kind"]) => deviceKindLabel(kind, t)
  const gatewayDeviceSourceLabel = (source: GatewayDevice["source"]) => deviceSourceLabel(source, t)
  const gatewaySerialInventoryStatusLabel = (status: GatewaySerialInventoryItem["status"]) =>
    serialInventoryStatusLabel(status, t)
  const gatewaySerialInventoryProductTypeLabel = (productType: GatewaySerialInventoryItem["product_type"]) =>
    serialInventoryProductTypeLabel(productType, t)

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
    setInventoryPage(1)
  }, [inventoryFilterProductType, inventoryFilterQuery, inventoryFilterStatus, inventoryPageSize, tenantID])

  useEffect(() => {
    if (inventoryPage > inventoryMaxPage) {
      setInventoryPage(inventoryMaxPage)
    }
  }, [inventoryMaxPage, inventoryPage])

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
    gatewayRegistrationForm.setValue("tenant_id", tenantID)
  }, [gatewayRegistrationForm, tenantID])

  useEffect(() => {
    return () => {
      pendingTimeoutIDsRef.current.forEach((timeoutID) => {
        window.clearTimeout(timeoutID)
      })
      pendingTimeoutIDsRef.current = []
    }
  }, [])

  function scheduleTimeout(callback: () => void, delayMS: number) {
    const timeoutID = window.setTimeout(() => {
      pendingTimeoutIDsRef.current = pendingTimeoutIDsRef.current.filter((item) => item !== timeoutID)
      callback()
    }, delayMS)
    pendingTimeoutIDsRef.current = [...pendingTimeoutIDsRef.current, timeoutID]
  }

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
        const message = err instanceof Error ? err.message : t("gateways.error.loadCheckpointSummaryFailed")
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
      scheduleTimeout(() => {
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

  async function onRegisterGateway(values: GatewayRegistrationFormValues) {
    setSubmitting(true)
    setError("")
    try {
      const created = await registerGateway(token, {
        serial_number: values.serial_number.trim(),
        tenant_id: values.tenant_id.trim(),
        building_id: values.building_id?.trim() || undefined,
        device_capacity: values.device_capacity === "8" ? 8 : 4,
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
      gatewayRegistrationForm.reset({
        serial_number: "",
        tenant_id: values.tenant_id,
        building_id: "",
        device_capacity: values.device_capacity,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.registerGatewayFailed")
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  async function onImportSerialInventory(values: {
    serial_number: string
    product_type: "gateway" | "reader" | "controller" | "relay" | "sensor"
    batch_code?: string
  }): Promise<boolean> {
    if (!tenantID.trim() || !values.serial_number.trim()) {
      return false
    }

    setSubmitting(true)
    setError("")
    try {
      await importGatewaySerialInventory(token, {
        tenant_id: tenantID.trim(),
        items: [
          {
            serial_number: values.serial_number.trim(),
            product_type: values.product_type,
            batch_code: values.batch_code?.trim() || undefined,
            source: "factory",
          },
        ],
      })
      setCommandLog(
        t("gateways.commandLog.importSerialSuccess", {
          serial: values.serial_number.trim(),
          productType: values.product_type,
        })
      )
      await gatewaysPageQuery.refetch()
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.importSerialFailed")
      setError(message)
      return false
    } finally {
      setSubmitting(false)
    }
  }

  async function onImportSerialInventoryCSV(csvContent: string): Promise<boolean> {
    if (!tenantID.trim() || !csvContent.trim()) {
      return false
    }

    setSubmitting(true)
    setError("")
    try {
      const importedItems = await importGatewaySerialInventoryCSV(token, {
        tenant_id: tenantID.trim(),
        csv_content: csvContent,
      })
      setCommandLog(t("gateways.commandLog.importCsvSuccess", { count: importedItems.length }))
      await gatewaysPageQuery.refetch()
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.importCsvFailed")
      setError(message)
      return false
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
      setCommandLog(t("gateways.commandLog.exportCsvSuccess", { fileName }))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.exportCsvFailed")
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
      setCommandLog(
        t("gateways.commandLog.updateSerialStatusSuccess", {
          serial: item.serial_number,
          status: serialInventoryStatusLabel(status, t),
        })
      )
      await gatewaysPageQuery.refetch()
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.updateSerialStatusFailed")
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
    const visibleSerials = pagedVisibleSerialInventory.map((item) => item.serial_number)
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

  async function onBatchUpdateSerialInventoryStatus(payload: {
    status: "available" | "frozen" | "scrapped"
    manualSerialNumbers: string[]
  }): Promise<boolean> {
    const targetSerialNumbers =
      payload.manualSerialNumbers.length > 0 ? payload.manualSerialNumbers : selectedInventorySerialNumbers
    if (!tenantID.trim() || targetSerialNumbers.length === 0) {
      return false
    }

    setCommandBusy(true)
    setError("")
    try {
      const updatedItems = await batchUpdateGatewaySerialInventoryStatus(token, {
        tenant_id: tenantID.trim(),
        status: payload.status,
        serial_numbers: targetSerialNumbers,
      })
      setSelectedInventorySerialNumbers((current) =>
        current.filter((item) => !targetSerialNumbers.includes(item))
      )
      setCommandLog(
        t("gateways.commandLog.batchUpdateSerialStatusSuccess", {
          count: updatedItems.length,
          status: serialInventoryStatusLabel(payload.status, t),
        })
      )
      await gatewaysPageQuery.refetch()
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.batchUpdateSerialStatusFailed")
      setError(message)
      return false
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
      setCommandLog(
        t("gateways.commandLog.bindDoorSuccess", {
          doorID: selectedDoorID.trim(),
          gatewayID: selectedGateway,
        })
      )
      setSelectedDoorID("")
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.bindDoorFailed")
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
      setCommandLog(
        t("gateways.commandLog.unbindDoorSuccess", {
          doorID: selectedBoundDoorID.trim(),
          gatewayID: selectedGateway,
        })
      )
      setSelectedBoundDoorID("")
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.unbindDoorFailed")
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
      setCommandLog(
        t("gateways.commandLog.registerDeviceSuccess", {
          serial: deviceSerialNumber.trim(),
          gatewayID: selectedGateway,
        })
      )
      setLegacyProbeCandidates((current) => current.filter((item) => item !== deviceSerialNumber.trim()))
      setDeviceSerialNumber("")
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.registerDeviceFailed")
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
      setCommandLog(t("gateways.commandLog.probeLegacySuccess", { count: items.length }))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.probeLegacyFailed")
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
      setCommandLog(t("gateways.commandLog.publishConfigQueued", { taskID: ack.task_id, gatewayID: ack.gateway_id }))
      pushCommandProgress(ack)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.publishConfigFailed")
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
      setCommandLog(t("gateways.commandLog.rebootQueued", { taskID: ack.task_id, gatewayID: ack.gateway_id }))
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
      scheduleTimeout(() => {
        void gatewaysPageQuery.refetch()
      }, 1500)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("gateways.error.rebootFailed")
      setError(message)
    } finally {
      setCommandBusy(false)
    }
  }

  const effectiveError = error || queryError
  const gatewayRegistrationFormError =
    gatewayRegistrationForm.formState.errors.serial_number?.message ||
    gatewayRegistrationForm.formState.errors.tenant_id?.message ||
    gatewayRegistrationForm.formState.errors.building_id?.message ||
    gatewayRegistrationForm.formState.errors.device_capacity?.message ||
    ""

  return (
    <div className="space-y-6">
      <div className="mp-page-hero">
        <div className="relative z-10 flex flex-col gap-5 xl:flex-row xl:items-end xl:justify-between">
          <div className="max-w-3xl space-y-2">
            <p className="mp-page-eyebrow">{t("gateways.header.eyebrow")}</p>
            <h1 className="mp-page-title">
              {platformViewer
                ? t("gateways.header.titlePlatform")
                : buildingAdmin
                  ? t("gateways.header.titleBuildingAdmin")
                  : t("gateways.header.titleTenant")}
            </h1>
            <p className="mp-page-description">
              {platformViewer
                ? t("gateways.header.descriptionPlatform")
                : buildingAdmin
                  ? t("gateways.header.descriptionBuildingAdmin")
                  : t("gateways.header.descriptionTenant")}
            </p>
          </div>
          <div className="grid gap-2 sm:grid-cols-3 xl:min-w-[420px]">
            <div className="rounded-2xl border border-white/10 bg-black/[0.18] p-3">
              <p className="mp-kpi-note">{t("gateways.heroMetrics.controllerMesh")}</p>
              <p className="mt-1 text-xl font-semibold">{gateways.length}</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-black/[0.18] p-3">
              <p className="mp-kpi-note">{t("gateways.heroMetrics.heartbeatOnline")}</p>
              <p className="mt-1 text-xl font-semibold">{gateways.filter((item) => item.status === "online").length}</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-black/[0.18] p-3">
              <p className="mp-kpi-note">{t("gateways.heroMetrics.deviceSlots")}</p>
              <p className="mt-1 text-xl font-semibold">{gatewayDeviceOnlineTotal}/{gatewayDeviceTotal}</p>
            </div>
          </div>
        </div>
      </div>

      {missingBuildingScope ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          {t("gateways.notice.missingScope")}
        </div>
      ) : buildingAdmin ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          {t("gateways.notice.buildingScopeHint")}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div className="mp-metric-card">
          <div className="relative z-10 flex items-start justify-between gap-3">
            <div>
              <p className="text-sm text-muted-foreground">{t("gateways.kpi.gatewayTotal.title")}</p>
              <p className="mt-1 text-3xl font-semibold tracking-[-0.04em]">{gateways.length}</p>
            </div>
            <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
              <CpuIcon className="size-4 text-white/75" />
            </div>
          </div>
          <p className="relative z-10 mt-4 mp-kpi-note">
            {platformViewer
              ? t("gateways.kpi.gatewayTotal.notePlatform")
              : buildingAdmin
                ? t("gateways.kpi.gatewayTotal.noteBuildingAdmin")
                : t("gateways.kpi.gatewayTotal.noteTenant")}
          </p>
        </div>
        <div className="mp-metric-card">
          <div className="relative z-10 flex items-start justify-between gap-3">
            <div>
              <p className="text-sm text-muted-foreground">{t("gateways.kpi.online.title")}</p>
              <p className="mt-1 text-3xl font-semibold tracking-[-0.04em]">
                {gateways.filter((item) => item.status === "online").length}
              </p>
            </div>
            <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
              <RadioTowerIcon className="size-4 text-emerald-300" />
            </div>
          </div>
          <p className="relative z-10 mt-4 mp-kpi-note">{t("gateways.kpi.online.note")}</p>
        </div>
        <div className="mp-metric-card">
          <div className="relative z-10 flex items-start justify-between gap-3">
            <div>
              <p className="text-sm text-muted-foreground">{t("gateways.kpi.boundDoors.title")}</p>
              <p className="mt-1 text-3xl font-semibold tracking-[-0.04em]">
                {gateways.reduce((sum, item) => sum + (item.bound_door_ids?.length ?? 0), 0)}
              </p>
            </div>
            <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
              <DoorOpenIcon className="size-4 text-white/75" />
            </div>
          </div>
          <p className="relative z-10 mt-4 mp-kpi-note">
            {platformViewer
              ? t("gateways.kpi.boundDoors.notePlatform")
              : buildingAdmin
                ? t("gateways.kpi.boundDoors.noteBuildingAdmin")
                : t("gateways.kpi.boundDoors.noteTenant")}
          </p>
        </div>
        <div className="mp-metric-card">
          <div className="relative z-10 flex items-start justify-between gap-3">
            <div>
              <p className="text-sm text-muted-foreground">{t("gateways.kpi.deviceOnline.title")}</p>
              <p className="mt-1 text-3xl font-semibold tracking-[-0.04em]">
                {gatewayDeviceOnlineTotal}/{gatewayDeviceTotal}
              </p>
            </div>
            <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
              <NetworkIcon className="size-4 text-white/75" />
            </div>
          </div>
          <p className="relative z-10 mt-4 mp-kpi-note">{t("gateways.kpi.deviceOnline.note")}</p>
        </div>
      </div>

      <CheckpointMonitor
        checkpointTrendWindowMinutes={checkpointTrendWindowMinutes}
        onWindowMinutesChange={setCheckpointTrendWindowMinutes}
        selectedGatewayRecord={selectedGatewayRecord ? { id: selectedGatewayRecord.id } : null}
        checkpointSummaryLoading={checkpointSummaryLoading}
        checkpointSummaryError={checkpointSummaryError}
        checkpointSummary={checkpointSummary}
      />

      {gatewayRegistrationVisible ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("gateways.register.title")}</CardTitle>
            <CardDescription>
              {platformViewer
                ? t("gateways.register.descriptionPlatform")
                : t("gateways.register.descriptionTenant")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              className={`grid gap-3 ${platformViewer ? "md:grid-cols-[1fr_220px_1fr_160px_auto]" : "md:grid-cols-[1fr_1fr_160px_auto]"}`}
              onSubmit={gatewayRegistrationForm.handleSubmit(onRegisterGateway)}
            >
              <Input
                {...gatewayRegistrationForm.register("serial_number")}
                placeholder={t("gateways.register.serialPlaceholder")}
              />
              {platformViewer ? (
                <Controller
                  control={gatewayRegistrationForm.control}
                  name="tenant_id"
                  render={({ field }) => (
                    <Select
                      value={field.value}
                      onValueChange={(value) => {
                        field.onChange(value)
                        setTenantID(value)
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("gateways.register.tenantPlaceholder")} />
                      </SelectTrigger>
                      <SelectContent>
                        {tenants.map((item) => (
                          <SelectItem key={item.id} value={item.id}>
                            {item.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
              ) : null}
              <Input
                {...gatewayRegistrationForm.register("building_id")}
                placeholder={t("gateways.register.buildingPlaceholder")}
              />
              <Controller
                control={gatewayRegistrationForm.control}
                name="device_capacity"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder={t("gateways.register.capacityPlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="4">{t("gateways.register.capacity4")}</SelectItem>
                      <SelectItem value="8">{t("gateways.register.capacity8")}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
              <Button type="submit" disabled={submitting || gatewayRegistrationForm.formState.isSubmitting || !tenantID}>
                {submitting ? t("gateways.register.submitting") : t("gateways.register.submit")}
              </Button>
              {gatewayRegistrationFormError ? (
                <p className={`text-sm text-destructive ${platformViewer ? "md:col-span-5" : "md:col-span-4"}`}>
                  {gatewayRegistrationFormError}
                </p>
              ) : null}
            </form>
          </CardContent>
        </Card>
      ) : null}

      {inventoryVisible ? (
        <GatewaySerialInventoryIngestCard
          platformViewer={platformViewer}
          inventoryEditable={inventoryEditable}
          readOnlyBoundaryHint={readOnlyBoundaryHint}
          submitting={submitting}
          tenantID={tenantID}
          onImportSerialInventory={async (values) => {
            return onImportSerialInventory(values)
          }}
          onImportSerialInventoryCSV={async (csvContent) => {
            return onImportSerialInventoryCSV(csvContent)
          }}
          commandBusy={commandBusy}
          onExportSerialInventoryCSV={() => {
            void onExportSerialInventoryCSV()
          }}
          visibleSerialInventory={visibleSerialInventory}
        />
      ) : null}

      <div className="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <GatewayCommandCenterCard
          gatewayOpsEditable={gatewayOpsEditable}
          readOnlyBoundaryHint={readOnlyBoundaryHint}
          selectedGateway={selectedGateway}
          onSelectedGatewayChange={setSelectedGateway}
          gateways={gateways}
          selectedDoorID={selectedDoorID}
          onSelectedDoorIDChange={setSelectedDoorID}
          availableDoors={availableDoors}
          commandBusy={commandBusy}
          onBindDoor={() => void onBindDoor()}
          selectedBoundDoorID={selectedBoundDoorID}
          onSelectedBoundDoorIDChange={setSelectedBoundDoorID}
          boundDoors={boundDoors}
          onUnbindDoor={() => void onUnbindDoor()}
          selectedGatewayRecord={selectedGatewayRecord}
          platformViewer={platformViewer}
          buildingAdmin={buildingAdmin}
          tenantByID={tenantByID}
          configVersion={configVersion}
          onConfigVersionChange={setConfigVersion}
          onPublishConfig={() => void onPublishConfig()}
          onRebootGateway={() => void onRebootGateway()}
          commandLog={commandLog}
          selectedGatewayRemainSlots={selectedGatewayRemainSlots}
          selectedGatewayDeviceOnline={selectedGatewayDeviceOnline}
          selectedGatewayDevices={selectedGatewayDevices}
          deviceSerialNumber={deviceSerialNumber}
          onDeviceSerialNumberChange={setDeviceSerialNumber}
          deviceKind={deviceKind}
          onDeviceKindChange={setDeviceKind}
          deviceSource={deviceSource}
          onDeviceSourceChange={setDeviceSource}
          deviceStatus={deviceStatus}
          onDeviceStatusChange={setDeviceStatus}
          deviceProtocol={deviceProtocol}
          onDeviceProtocolChange={setDeviceProtocol}
          onRegisterGatewayDevice={() => void onRegisterGatewayDevice()}
          rs485BaudRate={rs485BaudRate}
          onRS485BaudRateChange={setRS485BaudRate}
          rs485Parity={rs485Parity}
          onRS485ParityChange={setRS485Parity}
          rs485StopBits={rs485StopBits}
          onRS485StopBitsChange={setRS485StopBits}
          rs485Address={rs485Address}
          onRS485AddressChange={setRS485Address}
          rs485TimeoutMS={rs485TimeoutMS}
          onRS485TimeoutMSChange={setRS485TimeoutMS}
          onProbeLegacyDevices={() => void onProbeLegacyDevices()}
          legacyProbeCandidates={legacyProbeCandidates}
          onUseLegacyCandidate={(value) => {
            setDeviceSerialNumber(value)
            setDeviceSource("legacy_integration")
          }}
          deviceKindLabel={gatewayDeviceKindLabel}
          deviceSourceLabel={gatewayDeviceSourceLabel}
          deviceProtocolLabel={deviceProtocolLabel}
          statusLabel={gatewayStatusLabel}
          commandTasks={commandTasks}
          commandStatusLabel={gatewayCommandStatusLabel}
          commandStatusVariant={commandStatusVariant}
        />

        <GatewaySearchCard
          platformViewer={platformViewer}
          query={query}
          onQueryChange={setQuery}
          gatewayStatusFilter={gatewayStatusFilter}
          onGatewayStatusFilterChange={setGatewayStatusFilter}
          onResetFilters={() => {
            setQuery("")
            setGatewayStatusFilter("all")
          }}
        />
      </div>

      {inventoryVisible ? (
        <GatewaySerialInventoryLedgerCard
          platformViewer={platformViewer}
          inventoryEditable={inventoryEditable}
          inventoryFilterProductType={inventoryFilterProductType}
          onInventoryFilterProductTypeChange={setInventoryFilterProductType}
          inventoryFilterStatus={inventoryFilterStatus}
          onInventoryFilterStatusChange={setInventoryFilterStatus}
          inventoryFilterQuery={inventoryFilterQuery}
          onInventoryFilterQueryChange={setInventoryFilterQuery}
          inventoryPage={inventoryPage}
          onInventoryPageChange={setInventoryPage}
          inventoryPageSize={inventoryPageSize}
          onInventoryPageSizeChange={setInventoryPageSize}
          inventoryHasNextPage={inventoryHasNextPage}
          onResetFilters={() => {
            setInventoryFilterProductType("all")
            setInventoryFilterStatus("all")
            setInventoryFilterQuery("")
            setInventoryPage(1)
          }}
          commandBusy={commandBusy}
          tenantID={tenantID}
          onBatchUpdateSerialInventoryStatus={async (payload) => onBatchUpdateSerialInventoryStatus(payload)}
          onClearBatchTargets={() => {
            setSelectedInventorySerialNumbers([])
          }}
          selectedInventorySerialNumbersLength={selectedInventorySerialNumbers.length}
          visibleSerialInventory={pagedVisibleSerialInventory}
          allVisibleInventorySelected={allVisibleInventorySelected}
          onSelectAllVisibleSerialInventory={onSelectAllVisibleSerialInventory}
          selectedInventorySerialSet={selectedInventorySerialSet}
          onSelectSerialInventory={onSelectSerialInventory}
          tenantByID={tenantByID}
          onUpdateSerialInventoryStatus={(item, status) => {
            void onUpdateSerialInventoryStatus(item, status)
          }}
          serialInventoryProductTypeLabel={gatewaySerialInventoryProductTypeLabel}
          serialInventoryStatusLabel={gatewaySerialInventoryStatusLabel}
          serialInventoryStatusVariant={serialInventoryStatusVariant}
        />
      ) : null}

      <GatewayListCard
        platformViewer={platformViewer}
        loading={loading}
        effectiveError={effectiveError}
        filteredGateways={filteredGateways}
        missingBuildingScope={missingBuildingScope}
        hasActiveGatewayFilters={hasActiveGatewayFilters}
        buildingAdmin={buildingAdmin}
        tenantByID={tenantByID}
        statusLabel={gatewayStatusLabel}
        statusVariant={statusVariant}
      />
    </div>
  )
}
