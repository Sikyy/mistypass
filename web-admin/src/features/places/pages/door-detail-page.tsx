import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ChevronDownIcon,
  Clock3Icon,
  DoorOpenIcon,
  KeyRoundIcon,
  MapPinPlusIcon,
  PlusIcon,
  SearchIcon,
  ShieldAlertIcon,
  ShieldCheckIcon,
  ShieldOffIcon,
  Trash2Icon,
} from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
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
import { useMistyisletPlaceContext } from "@/features/mistyislet-shell/use-resource-summary"
import {
  cancelLockLockdown,
  createLock,
  deleteLock,
  getLock,
  lockDownLock,
  unlockLock,
  updateLock,
  type CurrentUser,
  type Lock,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

type LockAction = "unlock" | "lock_down" | "cancel_lockdown"

function lockKindFromLabel(value: string | undefined): Lock["kind"] {
  const normalized = value?.toLowerCase().replace(/\s+/g, "-") ?? ""
  switch (normalized) {
    case "turnstile":
    case "server-room":
    case "elevator":
    case "parking-gate":
    case "emergency-exit":
      return normalized
    default:
      return "office"
  }
}

export function DoorDetailAdaptedPage({
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
  const [selectedDoorID, setSelectedDoorID] = useState("")
  const [doorName, setDoorName] = useState("")
  const [doorKind, setDoorKind] = useState<Lock["kind"]>("office")
  const [doorStatus, setDoorStatus] = useState<Lock["status"]>("offline")
  const [gatewayID, setGatewayID] = useState("")
  const [addDoorOpen, setAddDoorOpen] = useState(false)
  const [newDoorName, setNewDoorName] = useState("")
  const [newDoorFloorID, setNewDoorFloorID] = useState("")
  const [newDoorAreaID, setNewDoorAreaID] = useState("")
  const [newDoorGatewayID, setNewDoorGatewayID] = useState("")
  const [newDoorKind, setNewDoorKind] = useState<Lock["kind"]>("office")
  const [newDoorStatus, setNewDoorStatus] = useState<Lock["status"]>("online")
  const [deleteDoorConfirmOpen, setDeleteDoorConfirmOpen] = useState(false)
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletPlaceContext(token, viewer, placeID)
  const { place, floors, zones, doors, hardware, events } = resourceQuery.context
  const selectedDoor = doors.find((door) => door.id === selectedDoorID) ?? doors[0]
  const tenantID = getViewerTenantID(viewer)
  const canCreateDoor = Boolean(tenantID && place && !resourceQuery.usingFallback)
  const canMutate = Boolean(tenantID && selectedDoor && !resourceQuery.usingFallback)
  const gatewayOptions = useMemo(() => {
    const seen = new Set<string>()
    return hardware.filter((item) => {
      if (item.type !== "Controller" && item.type !== "Gateway") {
        return false
      }
      const gateway = item.gatewayId || item.id
      if (!gateway || seen.has(gateway)) {
        return false
      }
      seen.add(gateway)
      return true
    })
  }, [hardware])
  const newDoorAreaOptions = useMemo(() => {
    if (!newDoorFloorID) {
      return zones
    }
    return zones.filter((zone) => zone.floorId === newDoorFloorID)
  }, [newDoorFloorID, zones])
  const lockDetailQuery = useQuery({
    queryKey: ["reference-lock-detail", selectedDoor?.id, tenantID],
    queryFn: () => getLock(token, selectedDoor?.id ?? "", tenantID),
    enabled: Boolean(selectedDoor?.id && tenantID && !resourceQuery.usingFallback),
  })
  const selectedHardware = useMemo(() => {
    if (!selectedDoor) {
      return []
    }
    return hardware.filter((item) => item.gatewayId === selectedDoor.gatewayId || item.doorNames.includes(selectedDoor.name))
  }, [hardware, selectedDoor])
  const selectedEvents = useMemo(() => {
    if (!selectedDoor) {
      return []
    }
    return events.filter((event) => event.doorId === selectedDoor.id || event.object === selectedDoor.name)
  }, [events, selectedDoor])

  useEffect(() => {
    if (!selectedDoorID && doors[0]) {
      setSelectedDoorID(doors[0].id)
      return
    }
    if (selectedDoorID && doors.length > 0 && !doors.some((door) => door.id === selectedDoorID)) {
      setSelectedDoorID(doors[0].id)
    }
  }, [doors, selectedDoorID])

  useEffect(() => {
    if (!selectedDoor) {
      setDoorName("")
      setGatewayID("")
      setDoorKind("office")
      setDoorStatus("offline")
      return
    }
    const detail = lockDetailQuery.data?.id === selectedDoor.id ? lockDetailQuery.data : null
    setDoorName(detail?.name ?? selectedDoor.name)
    setGatewayID(detail?.gateway_id ?? selectedDoor.gatewayId)
    setDoorKind(detail?.kind ?? lockKindFromLabel(selectedDoor.kind))
    setDoorStatus(detail?.status ?? selectedDoor.status)
  }, [lockDetailQuery.data, selectedDoor])

  useEffect(() => {
    if (!addDoorOpen) {
      return
    }
    if (!newDoorFloorID && floors[0]) {
      setNewDoorFloorID(floors[0].id)
    }
    if (!newDoorGatewayID && gatewayOptions[0]) {
      setNewDoorGatewayID(gatewayOptions[0].gatewayId || gatewayOptions[0].id)
    }
  }, [addDoorOpen, floors, gatewayOptions, newDoorFloorID, newDoorGatewayID])

  useEffect(() => {
    if (!addDoorOpen) {
      return
    }
    const areaIsValid = newDoorAreaOptions.some((area) => area.id === newDoorAreaID)
    if (!areaIsValid) {
      setNewDoorAreaID(newDoorAreaOptions[0]?.id ?? "")
    }
  }, [addDoorOpen, newDoorAreaID, newDoorAreaOptions])

  async function refreshDoors() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] }),
      queryClient.invalidateQueries({ queryKey: ["reference-lock-detail"] }),
    ])
  }

  const createLockMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !place) {
        throw new Error("place is required")
      }
      if (!newDoorFloorID) {
        throw new Error("floor is required")
      }
      if (!newDoorAreaID) {
        throw new Error("area is required")
      }
      return createLock(token, {
        tenant_id: tenantID,
        place_id: place.id,
        building_id: place.id,
        floor_id: newDoorFloorID,
        area_id: newDoorAreaID,
        name: newDoorName.trim(),
        gateway_id: newDoorGatewayID.trim() || undefined,
        kind: newDoorKind,
        status: newDoorStatus,
      })
    },
    onSuccess: async (lock) => {
      setSelectedDoorID(lock.id)
      setNewDoorName("")
      setNewDoorFloorID("")
      setNewDoorAreaID("")
      setNewDoorGatewayID("")
      setNewDoorKind("office")
      setNewDoorStatus("online")
      setAddDoorOpen(false)
      setActionNotice("Door created.")
      setActionError("")
      await refreshDoors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Door create failed")
    },
  })

  const updateLockMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedDoor) {
        throw new Error("lock is required")
      }
      return updateLock(token, selectedDoor.id, {
        tenant_id: tenantID,
        name: doorName.trim(),
        gateway_id: gatewayID.trim(),
        kind: doorKind,
        status: doorStatus,
      })
    },
    onSuccess: async () => {
      setActionNotice("Door saved.")
      setActionError("")
      await refreshDoors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Door save failed")
    },
  })

  const deleteLockMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedDoor) {
        throw new Error("lock is required")
      }
      return deleteLock(token, selectedDoor.id, tenantID)
    },
    onSuccess: async () => {
      setSelectedDoorID("")
      setDeleteDoorConfirmOpen(false)
      setActionNotice("Door deleted.")
      setActionError("")
      await refreshDoors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Door delete failed")
    },
  })

  const lockActionMutation = useMutation({
    mutationFn: (action: LockAction) => {
      if (!tenantID || !selectedDoor) {
        throw new Error("lock is required")
      }
      if (action === "unlock") {
        return unlockLock(token, selectedDoor.id, tenantID)
      }
      if (action === "lock_down") {
        return lockDownLock(token, selectedDoor.id, tenantID)
      }
      return cancelLockLockdown(token, selectedDoor.id, tenantID)
    },
    onSuccess: async (result) => {
      const label = result.action === "unlock" ? t("kisi.doors.unlock") : result.action === "lock_down" ? t("kisi.doors.lockdown") : "Cancel lockdown"
      setActionNotice(`${label} accepted.`)
      setActionError("")
      await refreshDoors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Door action failed")
    },
  })

  return (
    <>
      <PageFrame
        breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Doors"]}
        title={selectedDoor?.name ?? "Doors"}
        count={resourceQuery.isPending ? "--" : doors.length}
        description={selectedDoor ? `${selectedDoor.kind} door on ${selectedDoor.floorName}` : "Door settings and access schedules"}
        actions={
          <>
            <Button
              disabled={!canCreateDoor}
              onClick={() => {
                setActionNotice("")
                setActionError("")
                setAddDoorOpen(true)
              }}
              className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
            >
              <PlusIcon className="mr-1.5 size-4" />
              Add Door
            </Button>
            <Button
              variant="outline"
              disabled={!canMutate || lockActionMutation.isPending}
              onClick={() => lockActionMutation.mutate("unlock")}
              className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
            >
              <DoorOpenIcon className="mr-1.5 size-4" />
              Unlock
            </Button>
            <Button
              variant="interaction"
              disabled={!canMutate || lockActionMutation.isPending}
              onClick={() => lockActionMutation.mutate("lock_down")}
              className="h-10 rounded-[6px] text-[#4f55ff]"
            >
              <ShieldAlertIcon className="mr-1.5 size-4" />
              Lockdown
            </Button>
            <Button
              variant="outline"
              disabled={!canMutate || lockActionMutation.isPending}
              onClick={() => lockActionMutation.mutate("cancel_lockdown")}
              className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
            >
              <ShieldOffIcon className="mr-1.5 size-4" />
              Cancel
            </Button>
            <Button
              variant="outline"
              disabled={!canMutate || deleteLockMutation.isPending}
              onClick={() => {
                setActionError("")
                setDeleteDoorConfirmOpen(true)
              }}
              className="h-10 rounded-[6px] border-[#f1b7b2] bg-white px-5 text-[#d93025] hover:border-[#f1b7b2] hover:bg-[#fff7f7] hover:text-[#9f1d1d]"
            >
              <Trash2Icon className="mr-1.5 size-4" />
              Delete Door
            </Button>
          </>
        }
      >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live door resources are unavailable. Showing reference data.
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
                <th className="px-6 py-4 font-semibold">Door</th>
                <th className="px-4 py-4 font-semibold">Floor</th>
                <th className="px-4 py-4 font-semibold">Area</th>
                <th className="px-4 py-4 font-semibold">Gateway</th>
                <th className="px-4 py-4 font-semibold">Status</th>
              </tr>
            </thead>
            <tbody>
              {doors.map((door) => (
                <tr
                  key={door.id}
                  onClick={() => setSelectedDoorID(door.id)}
                  className={cn(
                    "cursor-pointer border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]",
                    selectedDoor?.id === door.id && "bg-[#f4f3ef]"
                  )}
                >
                  <td className="px-6 py-5 font-semibold text-[#17171c]">{door.name}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{door.floorName}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{door.areaName}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{door.gatewaySerial}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={door.status === "online" ? "success" : "warning"} label={door.status === "online" ? "Online" : "Review"} />
                  </td>
                </tr>
              ))}
              {doors.length === 0 ? <MistyisletEmptyTableRow colSpan={5}>No doors found for this place.</MistyisletEmptyTableRow> : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Groups", "Hardware", "Events", "Permissions"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button
            disabled={!canMutate || updateLockMutation.isPending || !doorName.trim()}
            onClick={() => updateLockMutation.mutate()}
            className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            {updateLockMutation.isPending ? "Saving..." : "Save"}
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Door name, floor, location, and lock behavior." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Name</span>
                <input
                  value={doorName}
                  disabled={!selectedDoor}
                  onChange={(event) => setDoorName(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Gateway ID</span>
                <input
                  value={gatewayID}
                  disabled={!selectedDoor}
                  onChange={(event) => setGatewayID(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Kind</span>
                <select
                  value={doorKind}
                  disabled={!selectedDoor}
                  onChange={(event) => setDoorKind(event.target.value as Lock["kind"])}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                >
                  <option value="office">Office</option>
                  <option value="turnstile">Turnstile</option>
                  <option value="server-room">Server Room</option>
                  <option value="elevator">Elevator</option>
                  <option value="parking-gate">Parking Gate</option>
                  <option value="emergency-exit">Emergency Exit</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Status</span>
                <select
                  value={doorStatus}
                  disabled={!selectedDoor}
                  onChange={(event) => setDoorStatus(event.target.value as Lock["status"])}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                >
                  <option value="online">Online</option>
                  <option value="offline">Offline</option>
                </select>
              </label>
              <FormField label="Description" value={selectedDoor?.description ?? "No door selected"} />
              <FormField label="Belongs to floor" value={selectedDoor?.floorName ?? "Unassigned floor"} trailing={<SearchIcon className="size-4 text-[#6f717c]" />} />
              <FormField label="Area" value={selectedDoor?.areaName ?? "Unassigned area"} />
              <FormField label="Timezone" value="Asia/Jakarta" />
            </div>
          </>
        ) : null}

        {activeTab === "Groups" ? (
          <>
            <PanelHeader
              title="Groups"
              description="Groups that include this door as an access target."
              action={
                <Button
                  variant="outline"
                  className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                >
                  Add Group
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {[
                ["Service Personnel", "Weekdays 08:00-18:00", "Primary device + geofence"],
                ["Facilities 24/7", "Always", "Reader required"],
              ].map((row) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_190px_1fr] md:items-center">
                  <span className="font-semibold text-[#4f55ff]">{row[0]}</span>
                  <span className="text-sm text-[#2f3037]">{row[1]}</span>
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Hardware" ? (
          <>
            <PanelHeader title="Hardware" description="Readers, controllers, and gateway path for this door." />
            <div className="divide-y divide-[#eceef2]">
              {selectedHardware.map((item) => (
                <div key={item.id} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_150px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{item.name}</span>
                  <span className="text-sm text-[#2f3037]">{item.type}</span>
                  <StatusDot tone={item.tone} label={item.statusLabel} />
                  <span className="text-sm text-[#6f717c]">{item.lastSeenLabel}</span>
                </div>
              ))}
              {selectedHardware.length === 0 ? (
                <div className="px-7 py-10 text-center text-sm text-[#6f717c]">No hardware is mapped to this door.</div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Events" ? (
          <>
            <PanelHeader title="Events" description="Recent events for this door." />
            <div className="divide-y divide-[#eceef2]">
              {selectedEvents.map((event) => (
                <div key={event.id} className="grid gap-3 px-7 py-5 md:grid-cols-[120px_180px_180px_1fr] md:items-center">
                  <span className="text-sm text-[#6f717c]">{event.timeLabel}</span>
                  <StatusDot tone={event.tone} label={event.action} />
                  <span className="text-sm font-semibold text-[#17171c]">{event.user}</span>
                  <span className="text-sm text-[#6f717c]">{event.details}</span>
                </div>
              ))}
              {selectedEvents.length === 0 ? (
                <div className="px-7 py-10 text-center text-sm text-[#6f717c]">No recent events for this door.</div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Permissions" ? (
          <>
            <PanelHeader title="Permissions" description="Door-level unlock restrictions and emergency behavior." />
            <SettingToggleRows
              rows={[
                ["Remote unlock", true, "Allow authorized admins to unlock this door from the dashboard.", DoorOpenIcon],
                ["Reader unlock required after hours", true, "Require local reader interaction outside business hours.", KeyRoundIcon],
                ["Lockdown eligible", true, "Include this door in place-level lockdown actions.", ShieldCheckIcon],
              ]}
            />
          </>
        ) : null}
      </SettingsPanel>

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="flex items-center justify-center gap-14 border-b border-[#eceef2] px-6 py-4">
          <button className="text-base font-semibold text-[#2f3037]">Unlock Schedules</button>
          <button className="border-b-2 border-[#4f55ff] px-4 pb-4 text-base font-semibold text-[#4f55ff]">Access Schedules</button>
        </div>
        <div className="flex items-center gap-4 border-b border-[#eceef2] px-8 py-6">
          <div className="flex size-10 items-center justify-center rounded-[6px] bg-[#4f55ff] text-white">
            <Clock3Icon className="size-5" />
          </div>
          <div>
            <h3 className="font-semibold text-[#17171c]">Time Restriction</h3>
            <p className="text-sm text-[#6f717c]">Allow users to unlock door only during defined time periods.</p>
          </div>
          <span className="ml-auto h-5 w-10 rounded-full bg-[#7f88ff] p-0.5">
            <span className="ml-auto block size-4 rounded-full bg-[#4f55ff]" />
          </span>
        </div>
        <div className="grid gap-4 border-b border-[#eceef2] px-6 py-5 lg:grid-cols-[180px_1fr_210px] lg:items-center">
          <button type="button" className="flex h-11 items-center justify-between rounded-[6px] border border-[#d9dbe3] px-4 text-sm font-semibold text-[#2f3037]">
            Weekly View
            <ChevronDownIcon className="size-4" />
          </button>
          <div className="flex items-center justify-center gap-5 text-sm font-semibold text-[#2f3037]">
            <span className="text-[#4f55ff]">‹</span>
            <span>Week: Apr 20 - Apr 26</span>
            <span className="text-[#4f55ff]">›</span>
          </div>
          <Button
            variant="outline"
            className="h-11 rounded-[6px] border-[#8589ff] bg-white text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
          >
            Add Schedule
            <ChevronDownIcon className="ml-1.5 size-4" />
          </Button>
        </div>
        <div className="flex items-center gap-2 border-b border-[#eceef2] bg-[#fbfbfc] px-8 py-4 text-sm text-[#6f717c]">
          <MapPinPlusIcon className="size-4" />
          The timezone is Asia/Jakarta
        </div>
        <div className="p-8">
          <div className="grid min-h-[360px] grid-cols-[64px_repeat(7,minmax(100px,1fr))] overflow-hidden rounded-[6px] border border-[#eceef2] bg-white">
            <div className="border-r border-[#eceef2] bg-white pt-12 text-right text-xs font-semibold text-[#9a9ca7]">
              {["3 AM", "6 AM", "9 AM", "12 PM", "3 PM", "6 PM", "9 PM"].map((time) => (
                <div key={time} className="h-11 pr-3">{time}</div>
              ))}
            </div>
            {["Mon 20", "Tue 21", "Wed 22", "Thu 23", "Fri 24", "Sat 25", "Sun 26"].map((day, index) => (
              <div key={day} className={cn("border-r border-white/80 bg-[#f1f2f5] text-center text-sm font-semibold text-[#6f717c] last:border-r-0", index > 3 && "bg-[#c4c6cc]")}>
                <div className="border-b border-white/80 bg-white py-4">{day}</div>
                {day.startsWith("Thu") ? (
                  <div className="mx-auto mt-[188px] w-full max-w-[120px] rounded-[5px] bg-[#202443] p-3 text-left text-xs text-white">
                    <p className="truncate font-semibold">Access permitted</p>
                    <p className="mt-1 text-white/70">6:00 PM - 10:00 PM</p>
                  </div>
                ) : null}
              </div>
            ))}
          </div>
          <div className="mt-6 flex flex-wrap gap-8 text-sm font-semibold text-[#2f3037]">
            <span className="inline-flex items-center gap-2"><span className="size-5 rounded-[3px] bg-[#202443]" />Access permitted</span>
            <span className="inline-flex items-center gap-2"><span className="size-5 rounded-[3px] bg-[#c4c6cc]" />Access restricted</span>
            <span className="inline-flex items-center gap-2"><span className="size-5 rounded-[3px] bg-[#3b3c42]" />Access restricted due to exception</span>
          </div>
        </div>
      </section>
      </PageFrame>

      <ConfirmActionDialog
        open={deleteDoorConfirmOpen}
        onOpenChange={(open) => {
          if (!deleteLockMutation.isPending) {
            setDeleteDoorConfirmOpen(open)
          }
        }}
        title="Delete door"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{selectedDoor?.name ?? "this door"}</span> from{" "}
            {place?.name ?? "this place"}.
          </>
        }
        confirmLabel="Delete door"
        pending={deleteLockMutation.isPending}
        disabled={!canMutate || !selectedDoor}
        destructive
        onConfirm={() => deleteLockMutation.mutate()}
      />

      <Sheet open={addDoorOpen} onOpenChange={setAddDoorOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Add Door</SheetTitle>
            <SheetDescription>{place?.name ?? "Selected place"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createLockMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Name</span>
              <input
                value={newDoorName}
                onChange={(event) => setNewDoorName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Floor</span>
              <select
                value={newDoorFloorID}
                onChange={(event) => setNewDoorFloorID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="">Select floor</option>
                {floors.map((floor) => (
                  <option key={floor.id} value={floor.id}>
                    {floor.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Area</span>
              <select
                value={newDoorAreaID}
                onChange={(event) => setNewDoorAreaID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="">Select area</option>
                {newDoorAreaOptions.map((area) => (
                  <option key={area.id} value={area.id}>
                    {area.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Gateway</span>
              <select
                value={newDoorGatewayID}
                onChange={(event) => setNewDoorGatewayID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="">Unassigned</option>
                {gatewayOptions.map((item) => (
                  <option key={item.gatewayId || item.id} value={item.gatewayId || item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Kind</span>
                <select
                  value={newDoorKind}
                  onChange={(event) => setNewDoorKind(event.target.value as Lock["kind"])}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                >
                  <option value="office">Office</option>
                  <option value="turnstile">Turnstile</option>
                  <option value="server-room">Server Room</option>
                  <option value="elevator">Elevator</option>
                  <option value="parking-gate">Parking Gate</option>
                  <option value="emergency-exit">Emergency Exit</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Status</span>
                <select
                  value={newDoorStatus}
                  onChange={(event) => setNewDoorStatus(event.target.value as Lock["status"])}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                >
                  <option value="online">Online</option>
                  <option value="offline">Offline</option>
                </select>
              </label>
            </div>
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setAddDoorOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={
                  !canCreateDoor ||
                  createLockMutation.isPending ||
                  !newDoorName.trim() ||
                  !newDoorFloorID ||
                  !newDoorAreaID
                }
                className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createLockMutation.isPending ? "Creating..." : t("kisi.doors.addDoor")}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}
