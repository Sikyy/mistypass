import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CloudIcon, FileTextIcon, LinkIcon, PlusIcon, RotateCwIcon, SendIcon, ServerIcon, UnlinkIcon, ZapIcon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import {
  FormField,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
} from "@/components/mistyislet/primitives"
import { MistyisletEmptyTableRow } from "@/components/mistyislet/data-display"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import type { MistyisletDoorResource, MistyisletHardwareResource } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletPlaceContext } from "@/features/mistyislet-shell/use-resource-summary"
import {
  assignController,
  assignReader,
  bindControllerLock,
  deassignController,
  deassignReader,
  importGatewaySerialInventory,
  listGatewaySerialInventory,
  publishControllerConfig,
  rebootController,
  rebootReader,
  rebootTerminal,
  triggerTerminal,
  unbindControllerLock,
  type Controller,
  type CurrentUser,
  type GatewaySerialInventoryProductType,
  type Reader,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

type HardwareCreateMode = "controller" | "reader"
type GatewayDeviceProtocol = "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"
type HardwareDoorUnbindTarget = {
  controllerID: string
  deviceName: string
  door: MistyisletDoorResource
}
type HardwareRebootTarget = {
  command: "reboot"
  controllerID: string
  device: MistyisletHardwareResource
}
type HardwareTerminalTriggerTarget = {
  command: "trigger"
  device: MistyisletHardwareResource
}
type HardwarePublishConfigTarget = {
  command: "publish_config"
  controllerID: string
  configVersion: string
  device: MistyisletHardwareResource
}
type HardwareCommandTarget = HardwareRebootTarget | HardwareTerminalTriggerTarget | HardwarePublishConfigTarget

export function HardwareAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState("General")
  const [selectedDeviceID, setSelectedDeviceID] = useState("")
  const [addHardwareOpen, setAddHardwareOpen] = useState(false)
  const [hardwareMode, setHardwareMode] = useState<HardwareCreateMode>("controller")
  const [hardwareSerial, setHardwareSerial] = useState("")
  const [hardwareGatewayID, setHardwareGatewayID] = useState("")
  const [hardwareCapacity, setHardwareCapacity] = useState("8")
  const [hardwareProtocol, setHardwareProtocol] = useState<GatewayDeviceProtocol>("osdp_v2")
  const [hardwareStatus, setHardwareStatus] = useState<"online" | "offline">("online")
  const [bindDoorID, setBindDoorID] = useState("")
  const [configVersion, setConfigVersion] = useState("place-config-v1")
  const [unbindDoorTarget, setUnbindDoorTarget] = useState<HardwareDoorUnbindTarget | null>(null)
  const [deassignTarget, setDeassignTarget] = useState<MistyisletHardwareResource | null>(null)
  const [hardwareCommandTarget, setHardwareCommandTarget] = useState<HardwareCommandTarget | null>(null)
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletPlaceContext(token, viewer, placeID)
  const { place, doors, hardware, events } = resourceQuery.context
  const selectedDevice = hardware.find((item) => item.id === selectedDeviceID) ?? hardware[0]
  const tenantID = getViewerTenantID(viewer)
  const canManageHardware = Boolean(tenantID && place && !resourceQuery.usingFallback)
  const selectedGatewayID =
    selectedDevice?.type === "Controller" || selectedDevice?.type === "Gateway"
      ? selectedDevice.id
      : selectedDevice?.gatewayId ?? ""
  const canDeassignSelectedDevice = selectedDevice?.type === "Controller" || selectedDevice?.type === "Gateway" || selectedDevice?.type === "Reader"
  const controllerOptions = useMemo(() => {
    const seen = new Set<string>()
    return hardware.filter((item) => {
      if (item.type !== "Controller" && item.type !== "Gateway") {
        return false
      }
      const gatewayID = item.gatewayId || item.id
      if (!gatewayID || seen.has(gatewayID)) {
        return false
      }
      seen.add(gatewayID)
      return true
    })
  }, [hardware])
  const selectedDeviceDoorRows = useMemo(() => {
    const doorNames = new Set(selectedDevice?.doorNames ?? [])
    return doors.filter((door) => doorNames.has(door.name))
  }, [doors, selectedDevice])
  const bindableDoors = useMemo(() => {
    const boundDoorIDs = new Set(selectedDeviceDoorRows.map((door) => door.id))
    return doors.filter((door) => !boundDoorIDs.has(door.id))
  }, [doors, selectedDeviceDoorRows])
  const hardwareInventoryProductType: GatewaySerialInventoryProductType = hardwareMode === "controller" ? "gateway" : "reader"
  const serialInventoryQuery = useQuery({
    queryKey: ["gateway-serial-inventory", tenantID, hardwareInventoryProductType, "available"],
    queryFn: () =>
      listGatewaySerialInventory(token, tenantID, {
        product_type: hardwareInventoryProductType,
        status: "available",
      }),
    enabled: Boolean(addHardwareOpen && tenantID && !resourceQuery.usingFallback),
  })
  const availableSerials = serialInventoryQuery.data ?? []
  const selectedDeviceEvents = useMemo(() => {
    if (!selectedDevice) {
      return []
    }
    const doorNames = new Set(selectedDevice.doorNames)
    return events.filter((event) => doorNames.has(event.object)).slice(0, 6)
  }, [events, selectedDevice])

  useEffect(() => {
    if (!selectedDeviceID && hardware[0]) {
      setSelectedDeviceID(hardware[0].id)
      return
    }
    if (selectedDeviceID && hardware.length > 0 && !hardware.some((item) => item.id === selectedDeviceID)) {
      setSelectedDeviceID(hardware[0].id)
    }
  }, [hardware, selectedDeviceID])

  useEffect(() => {
    if (!addHardwareOpen) {
      return
    }
    if (hardwareMode === "reader" && !hardwareGatewayID && controllerOptions[0]) {
      setHardwareGatewayID(controllerOptions[0].gatewayId || controllerOptions[0].id)
    }
  }, [addHardwareOpen, controllerOptions, hardwareGatewayID, hardwareMode])

  useEffect(() => {
    if (!addHardwareOpen || hardwareSerial) {
      return
    }
    if (availableSerials[0]) {
      setHardwareSerial(availableSerials[0].serial_number)
    }
  }, [addHardwareOpen, availableSerials, hardwareSerial])

  useEffect(() => {
    if (bindDoorID && bindableDoors.some((door) => door.id === bindDoorID)) {
      return
    }
    setBindDoorID(bindableDoors[0]?.id ?? "")
  }, [bindDoorID, bindableDoors])

  async function refreshHardware() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] }),
      queryClient.invalidateQueries({ queryKey: ["gateway-serial-inventory"] }),
    ])
  }

  async function ensureSerialInventory(serialNumber: string, productType: GatewaySerialInventoryProductType) {
    const normalizedSerial = serialNumber.trim().toUpperCase()
    if (!normalizedSerial || !tenantID) {
      return normalizedSerial
    }
    const alreadyAvailable = availableSerials.some((item) => item.serial_number.toUpperCase() === normalizedSerial)
    if (alreadyAvailable) {
      return normalizedSerial
    }
    await importGatewaySerialInventory(token, {
      tenant_id: tenantID,
      items: [
        {
          serial_number: normalizedSerial,
          product_type: productType,
          batch_code: "hardware-ui",
          source: "web_admin",
        },
      ],
    })
    return normalizedSerial
  }

  const addHardwareMutation = useMutation({
    mutationFn: async () => {
      if (!tenantID || !place) {
        throw new Error("place is required")
      }
      const serialNumber = await ensureSerialInventory(hardwareSerial, hardwareInventoryProductType)
      if (!serialNumber) {
        throw new Error("serial_number is required")
      }
      if (hardwareMode === "controller") {
        return assignController(token, serialNumber, {
          tenant_id: tenantID,
          place_id: place.id,
          device_capacity: Number.parseInt(hardwareCapacity, 10) || 8,
        })
      }
      if (!hardwareGatewayID) {
        throw new Error("controller is required")
      }
      return assignReader(token, serialNumber, {
        tenant_id: tenantID,
        controller_id: hardwareGatewayID,
        kind: "reader",
        source: "mistypass_procured",
        protocol: hardwareProtocol,
        status: hardwareStatus,
      })
    },
    onSuccess: async (result) => {
      setSelectedDeviceID(result.id)
      setHardwareSerial("")
      setHardwareGatewayID("")
      setHardwareCapacity("8")
      setHardwareProtocol("osdp_v2")
      setHardwareStatus("online")
      setAddHardwareOpen(false)
      setActionNotice("Hardware registered.")
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Hardware registration failed")
    },
  })

  const bindDoorMutation = useMutation({
    mutationFn: () => {
      if (!selectedGatewayID) {
        throw new Error("controller is required")
      }
      if (!bindDoorID) {
        throw new Error("door is required")
      }
      return bindControllerLock(token, selectedGatewayID, bindDoorID, tenantID)
    },
    onSuccess: async () => {
      setActionNotice("Door bound to hardware.")
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Door binding failed")
    },
  })

  const unbindDoorMutation = useMutation({
    mutationFn: (target: HardwareDoorUnbindTarget) => {
      if (!target.controllerID) {
        throw new Error("controller is required")
      }
      return unbindControllerLock(token, target.controllerID, target.door.id, tenantID)
    },
    onSuccess: async () => {
      setUnbindDoorTarget(null)
      setActionNotice("Door removed from hardware.")
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Door removal failed")
    },
  })

  const publishConfigMutation = useMutation({
    mutationFn: (target: HardwarePublishConfigTarget) => {
      if (!target.controllerID) {
        throw new Error("controller is required")
      }
      return publishControllerConfig(token, target.controllerID, target.configVersion, tenantID)
    },
    onSuccess: async (ack) => {
      setHardwareCommandTarget(null)
      setActionNotice(`Config publish ${ack.status}.`)
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Config publish failed")
    },
  })

  const rebootMutation = useMutation({
    mutationFn: (target: HardwareRebootTarget) => {
      if (!target.controllerID) {
        throw new Error("controller is required")
      }
      if (target.device.type === "Reader") {
        return rebootReader(token, target.device.id, tenantID)
      }
      if (target.device.type === "Terminal") {
        return rebootTerminal(token, target.device.id, tenantID)
      }
      return rebootController(token, target.controllerID, tenantID)
    },
    onSuccess: async (ack) => {
      setHardwareCommandTarget(null)
      setActionNotice(`Reboot ${ack.status}.`)
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Reboot failed")
    },
  })

  const triggerTerminalMutation = useMutation({
    mutationFn: (target: HardwareTerminalTriggerTarget) => {
      if (target.device.type !== "Terminal") {
        throw new Error("terminal is required")
      }
      return triggerTerminal(token, target.device.id, tenantID)
    },
    onSuccess: async () => {
      setHardwareCommandTarget(null)
      setActionNotice("Terminal trigger sent.")
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Terminal trigger failed")
    },
  })

  const deassignMutation = useMutation<Controller | Reader, Error, MistyisletHardwareResource>({
    mutationFn: (device) => {
      if (!device) {
        throw new Error("hardware is required")
      }
      if (device.type === "Reader") {
        return deassignReader(token, device.id, tenantID)
      }
      if (device.type === "Controller" || device.type === "Gateway") {
        return deassignController(token, device.id, tenantID)
      }
      throw new Error("selected hardware cannot be deassigned")
    },
    onSuccess: async () => {
      setSelectedDeviceID("")
      setDeassignTarget(null)
      setActionNotice("Hardware deassigned.")
      setActionError("")
      await refreshHardware()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Hardware deassign failed")
    },
  })

  const hardwareCommandPending =
    hardwareCommandTarget?.command === "trigger"
      ? triggerTerminalMutation.isPending
      : hardwareCommandTarget?.command === "publish_config"
        ? publishConfigMutation.isPending
        : rebootMutation.isPending
  const hardwareCommandDisabled =
    !canManageHardware ||
    !hardwareCommandTarget ||
    (hardwareCommandTarget.command === "reboot" && !hardwareCommandTarget.controllerID) ||
    (hardwareCommandTarget.command === "publish_config" && (!hardwareCommandTarget.controllerID || !hardwareCommandTarget.configVersion)) ||
    (hardwareCommandTarget.command === "trigger" && hardwareCommandTarget.device.type !== "Terminal")
  const hardwareCommandTitle =
    hardwareCommandTarget?.command === "trigger"
      ? "Trigger terminal"
      : hardwareCommandTarget?.command === "publish_config"
        ? "Publish controller config"
        : "Reboot hardware"
  const hardwareCommandConfirmLabel =
    hardwareCommandTarget?.command === "trigger"
      ? "Trigger terminal"
      : hardwareCommandTarget?.command === "publish_config"
        ? "Publish config"
        : "Reboot hardware"

  return (
    <>
      <PageFrame
        breadcrumbs={[t("common.home"), "Places", place?.name ?? "Assigned Place", "Hardware"]}
        title={t("kisi.hardware.title")}
        count={resourceQuery.isPending ? "--" : hardware.length}
        description="Gateways, controllers, readers, and terminals assigned to this place"
        actions={
          <Button
            disabled={!canManageHardware}
            onClick={() => {
              setActionNotice("")
              setActionError("")
              setAddHardwareOpen(true)
            }}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Hardware
          </Button>
        }
      >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live hardware resources are unavailable. Showing reference data.
        </div>
      ) : null}
      {actionNotice ? (
        <div className="rounded-[6px] border border-[#b9dfc7] bg-[#f1fff5] px-5 py-4 text-sm text-[#1f6b3a]">
          {actionNotice}
        </div>
      ) : null}
      {actionError ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          {actionError}
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">Device</th>
                <th className="px-4 py-4 font-semibold">{t("common.type")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.status")}</th>
                <th className="px-4 py-4 font-semibold">Location</th>
              </tr>
            </thead>
            <tbody>
              {hardware.map((item) => (
                <tr
                  key={item.id}
                  onClick={() => setSelectedDeviceID(item.id)}
                  className={cn(
                    "cursor-pointer border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]",
                    selectedDevice?.id === item.id && "bg-[#f4f3ef]"
                  )}
                >
                  <td className="px-6 py-5 font-semibold text-[#17171c]">{item.name}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{item.type}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={item.tone} label={item.statusLabel} />
                  </td>
                  <td className="px-4 py-5 text-[#6f717c]">{item.location}</td>
                </tr>
              ))}
              {hardware.length === 0 ? <MistyisletEmptyTableRow colSpan={4}>No hardware found for this place.</MistyisletEmptyTableRow> : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Doors", "Events", "Settings"]}
        active={activeTab}
        onTabChange={setActiveTab}
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title={selectedDevice?.name ?? "No device selected"} description="Device assignment, firmware, and connectivity." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <FormField label="Device name" value={selectedDevice?.name ?? "No device selected"} />
              <FormField label="Type" value={selectedDevice?.type ?? "Unassigned"} />
              <FormField label="Place" value={place?.name ?? "Assigned Place"} />
              <FormField label="Last heartbeat" value={selectedDevice?.lastSeenLabel ?? "Unknown"} />
            </div>
          </>
        ) : null}

        {activeTab === "Doors" ? (
          <>
            <PanelHeader
              title="Doors"
              description="Doors connected to this device or gateway path."
              action={
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <select
                    value={bindDoorID}
                    onChange={(event) => setBindDoorID(event.target.value)}
                    className="h-10 min-w-[220px] rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  >
                    <option value="">Select door</option>
                    {bindableDoors.map((door) => (
                      <option key={door.id} value={door.id}>
                        {door.name}
                      </option>
                    ))}
                  </select>
                  <Button
                    variant="outline"
                    disabled={!canManageHardware || !selectedGatewayID || !bindDoorID || bindDoorMutation.isPending}
                    onClick={() => bindDoorMutation.mutate()}
                    className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                  >
                    <LinkIcon className="mr-1.5 size-4" />
                    Bind Door
                  </Button>
                </div>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {selectedDeviceDoorRows.map((door, index) => (
                <div key={door.id} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_160px_1fr_72px] md:items-center">
                  <span className="font-semibold text-[#4f55ff]">{door.name}</span>
                  <StatusDot tone={selectedDevice?.tone ?? "success"} label={selectedDevice?.statusLabel ?? "Online"} />
                  <span className="text-sm text-[#6f717c]">{index === 0 ? "Primary controller path" : "Reader assignment"}</span>
                  <div className="flex justify-end">
                    <RowActionsMenu
                      label={`Actions for ${door.name}`}
                      items={[
                        {
                          id: "remove",
                          label: "Remove",
                          icon: UnlinkIcon,
                          destructive: true,
                          disabled: !canManageHardware || !selectedGatewayID || unbindDoorMutation.isPending,
                          onSelect: () => {
                            setActionError("")
                            setUnbindDoorTarget({
                              controllerID: selectedGatewayID,
                              deviceName: selectedDevice?.name ?? "this device",
                              door,
                            })
                          },
                        },
                      ]}
                    />
                  </div>
                </div>
              ))}
              {selectedDeviceDoorRows.length === 0 ? (
                <div className="px-7 py-10 text-center text-sm text-[#6f717c]">No doors are mapped to this device.</div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Events" ? (
          <>
            <PanelHeader title="Events" description="Device health and command audit events." />
            <div className="divide-y divide-[#eceef2]">
              {selectedDeviceEvents.map((event) => (
                <div key={event.id} className="grid gap-3 px-7 py-5 md:grid-cols-[120px_1fr_140px] md:items-center">
                  <span className="text-sm text-[#6f717c]">{event.timeLabel}</span>
                  <span className="font-semibold text-[#17171c]">{event.action}</span>
                  <StatusDot tone={event.tone} label={event.statusLabel} />
                </div>
              ))}
              {selectedDeviceEvents.length === 0 ? (
                <div className="px-7 py-10 text-center text-sm text-[#6f717c]">No recent events for this device path.</div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Settings" ? (
          <>
            <PanelHeader
              title="Settings"
              description="Remote management and diagnostics."
              action={
                <div className="flex flex-wrap gap-3">
                  <Button
                    variant="outline"
                    disabled={!canManageHardware || !selectedGatewayID || rebootMutation.isPending}
                    onClick={() => {
                      if (selectedDevice) {
                        setActionError("")
                        setHardwareCommandTarget({
                          command: "reboot",
                          controllerID: selectedGatewayID,
                          device: selectedDevice,
                        })
                      }
                    }}
                    className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                  >
                    <RotateCwIcon className="mr-1.5 size-4" />
                    Reboot
                  </Button>
                  {selectedDevice?.type === "Terminal" ? (
                    <Button
                      variant="outline"
                      disabled={!canManageHardware || triggerTerminalMutation.isPending}
                      onClick={() => {
                        if (selectedDevice) {
                          setActionError("")
                          setHardwareCommandTarget({
                            command: "trigger",
                            device: selectedDevice,
                          })
                        }
                      }}
                      className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                    >
                      <ZapIcon className="mr-1.5 size-4" />
                      Trigger
                    </Button>
                  ) : null}
                  <Button
                    disabled={!canManageHardware || !selectedGatewayID || !configVersion.trim() || publishConfigMutation.isPending}
                    onClick={() => {
                      if (selectedDevice) {
                        setActionError("")
                        setHardwareCommandTarget({
                          command: "publish_config",
                          controllerID: selectedGatewayID,
                          configVersion: configVersion.trim(),
                          device: selectedDevice,
                        })
                      }
                    }}
                    className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
                  >
                    <SendIcon className="mr-1.5 size-4" />
                    Publish Config
                  </Button>
                  <Button
                    variant="outline"
                    disabled={!canManageHardware || !canDeassignSelectedDevice || deassignMutation.isPending}
                    onClick={() => {
                      if (selectedDevice) {
                        setActionError("")
                        setDeassignTarget(selectedDevice)
                      }
                    }}
                    className="h-10 rounded-[6px] border-[#e0a8a8] bg-white px-5 text-[#b42318] hover:bg-[#fff5f5] disabled:border-[#d9dbe3] disabled:text-[#8d909b]"
                  >
                    <UnlinkIcon className="mr-1.5 size-4" />
                    Deassign
                  </Button>
                </div>
              }
            />
            <div className="grid gap-6 border-b border-[#eceef2] p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Config version</span>
                <input
                  value={configVersion}
                  onChange={(event) => setConfigVersion(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
                />
              </label>
              <FormField label="Controller ID" value={selectedGatewayID || "No controller selected"} />
            </div>
            <SettingToggleRows
              rows={[
                ["Remote commands", true, "Allow place admins to restart or resync this device.", ServerIcon],
                ["Automatic firmware updates", true, "Install approved firmware during the maintenance window.", CloudIcon],
                ["Diagnostics upload", false, "Upload detailed diagnostic bundles for support review.", FileTextIcon],
              ]}
            />
          </>
        ) : null}
      </SettingsPanel>
      </PageFrame>

      <ConfirmActionDialog
        open={Boolean(hardwareCommandTarget)}
        onOpenChange={(open) => {
          if (!publishConfigMutation.isPending && !rebootMutation.isPending && !triggerTerminalMutation.isPending && !open) {
            setHardwareCommandTarget(null)
          }
        }}
        title={hardwareCommandTitle}
        description={
          hardwareCommandTarget?.command === "trigger" ? (
            <>
              This sends a trigger command to{" "}
              <span className="font-semibold text-[#17171c]">{hardwareCommandTarget.device.name}</span>.
            </>
          ) : hardwareCommandTarget?.command === "publish_config" ? (
            <>
              This publishes config version{" "}
              <span className="font-semibold text-[#17171c]">{hardwareCommandTarget.configVersion}</span> to{" "}
              <span className="font-semibold text-[#17171c]">{hardwareCommandTarget.device.name}</span>.
            </>
          ) : (
            <>
              This restarts <span className="font-semibold text-[#17171c]">{hardwareCommandTarget?.device.name ?? "this hardware"}</span> and
              can briefly interrupt access control.
            </>
          )
        }
        confirmLabel={hardwareCommandConfirmLabel}
        pending={hardwareCommandPending}
        disabled={hardwareCommandDisabled}
        destructive={hardwareCommandTarget?.command === "reboot"}
        onConfirm={() => {
          if (!hardwareCommandTarget) {
            return
          }
          if (hardwareCommandTarget.command === "trigger") {
            triggerTerminalMutation.mutate(hardwareCommandTarget)
            return
          }
          if (hardwareCommandTarget.command === "publish_config") {
            publishConfigMutation.mutate(hardwareCommandTarget)
            return
          }
          rebootMutation.mutate(hardwareCommandTarget)
        }}
      />

      <ConfirmActionDialog
        open={Boolean(unbindDoorTarget)}
        onOpenChange={(open) => {
          if (!unbindDoorMutation.isPending && !open) {
            setUnbindDoorTarget(null)
          }
        }}
        title="Remove door binding"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{unbindDoorTarget?.door.name ?? "this door"}</span> from{" "}
            {unbindDoorTarget?.deviceName ?? "this hardware"}.
          </>
        }
        confirmLabel="Remove binding"
        pending={unbindDoorMutation.isPending}
        disabled={!canManageHardware || !unbindDoorTarget}
        destructive
        onConfirm={() => {
          if (unbindDoorTarget) {
            unbindDoorMutation.mutate(unbindDoorTarget)
          }
        }}
      />

      <ConfirmActionDialog
        open={Boolean(deassignTarget)}
        onOpenChange={(open) => {
          if (!deassignMutation.isPending && !open) {
            setDeassignTarget(null)
          }
        }}
        title="Deassign hardware"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{deassignTarget?.name ?? "this hardware"}</span> from{" "}
            {place?.name ?? "this place"}.
          </>
        }
        confirmLabel="Deassign hardware"
        pending={deassignMutation.isPending}
        disabled={!canManageHardware || !deassignTarget}
        destructive
        onConfirm={() => {
          if (deassignTarget) {
            deassignMutation.mutate(deassignTarget)
          }
        }}
      />

      <Sheet open={addHardwareOpen} onOpenChange={setAddHardwareOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[480px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Add Hardware</SheetTitle>
            <SheetDescription>{place?.name ?? "Selected place"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              addHardwareMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Type</span>
              <select
                value={hardwareMode}
                onChange={(event) => {
                  setHardwareMode(event.target.value as HardwareCreateMode)
                  setHardwareSerial("")
                }}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="controller">Controller</option>
                <option value="reader">Reader</option>
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Serial number</span>
              <input
                value={hardwareSerial}
                list="hardware-serial-options"
                onChange={(event) => setHardwareSerial(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
              <datalist id="hardware-serial-options">
                {availableSerials.map((item) => (
                  <option key={item.id} value={item.serial_number} />
                ))}
              </datalist>
            </label>
            {hardwareMode === "controller" ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Device capacity</span>
                <select
                  value={hardwareCapacity}
                  onChange={(event) => setHardwareCapacity(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                >
                  <option value="8">8 devices</option>
                  <option value="4">4 devices</option>
                </select>
              </label>
            ) : (
              <>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Controller</span>
                  <select
                    value={hardwareGatewayID}
                    onChange={(event) => setHardwareGatewayID(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  >
                    <option value="">Select controller</option>
                    {controllerOptions.map((item) => (
                      <option key={item.gatewayId || item.id} value={item.gatewayId || item.id}>
                        {item.name}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Protocol</span>
                    <select
                      value={hardwareProtocol}
                      onChange={(event) => setHardwareProtocol(event.target.value as GatewayDeviceProtocol)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      <option value="osdp_v2">OSDP v2</option>
                      <option value="wiegand_26">Wiegand 26</option>
                      <option value="wiegand_34">Wiegand 34</option>
                      <option value="rs485">RS485</option>
                      <option value="ble">BLE</option>
                    </select>
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.status")}</span>
                    <select
                      value={hardwareStatus}
                      onChange={(event) => setHardwareStatus(event.target.value as "online" | "offline")}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      <option value="online">Online</option>
                      <option value="offline">Offline</option>
                    </select>
                  </label>
                </div>
              </>
            )}
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setAddHardwareOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={
                  !canManageHardware ||
                  addHardwareMutation.isPending ||
                  !hardwareSerial.trim() ||
                  (hardwareMode === "reader" && !hardwareGatewayID)
                }
                className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {addHardwareMutation.isPending ? "Registering..." : t("kisi.hardware.addHardware")}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}
