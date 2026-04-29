import { useEffect, useMemo, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import {
  BarChart3Icon,
  BellIcon,
  ChevronDownIcon,
  KeyRoundIcon,
  LayersIcon,
  PlusIcon,
  ServerIcon,
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
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import type { MistyisletFloorResource } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletPlaceContext } from "@/features/mistyislet-shell/use-resource-summary"
import { createArea, createFloor, deleteFloor, updateArea, updateFloor, type CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

export function FloorsAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState("General")
  const [selectedFloorID, setSelectedFloorID] = useState("")
  const [floorName, setFloorName] = useState("")
  const [addFloorOpen, setAddFloorOpen] = useState(false)
  const [newFloorName, setNewFloorName] = useState("")
  const [selectedAreaID, setSelectedAreaID] = useState("")
  const [areaName, setAreaName] = useState("")
  const [addAreaOpen, setAddAreaOpen] = useState(false)
  const [newAreaName, setNewAreaName] = useState("")
  const [deleteFloorTarget, setDeleteFloorTarget] = useState<MistyisletFloorResource | null>(null)
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletPlaceContext(token, viewer, placeID)
  const { place, floors, zones, doors, hardware } = resourceQuery.context
  const selectedFloor = floors.find((floor) => floor.id === selectedFloorID) ?? floors[0]
  const tenantID = getViewerTenantID(viewer)
  const canMutate = Boolean(tenantID && place && !resourceQuery.usingFallback)
  const selectedFloorAreas = useMemo(() => {
    if (!selectedFloor) {
      return []
    }
    return zones.filter((zone) => zone.floorId === selectedFloor.id)
  }, [selectedFloor, zones])
  const selectedArea = selectedFloorAreas.find((area) => area.id === selectedAreaID) ?? selectedFloorAreas[0]
  const selectedFloorDoors = useMemo(() => {
    if (!selectedFloor) {
      return []
    }
    return doors.filter((door) => door.floorId === selectedFloor.id)
  }, [doors, selectedFloor])
  const selectedFloorHardware = useMemo(() => {
    const doorNames = new Set(selectedFloorDoors.map((door) => door.name))
    return hardware.filter((item) => item.doorNames.some((doorName) => doorNames.has(doorName)))
  }, [hardware, selectedFloorDoors])

  useEffect(() => {
    if (!selectedFloorID && floors[0]) {
      setSelectedFloorID(floors[0].id)
      return
    }
    if (selectedFloorID && floors.length > 0 && !floors.some((floor) => floor.id === selectedFloorID)) {
      setSelectedFloorID(floors[0].id)
    }
  }, [floors, selectedFloorID])

  useEffect(() => {
    setFloorName(selectedFloor?.name ?? "")
  }, [selectedFloor])

  useEffect(() => {
    if (!selectedAreaID && selectedFloorAreas[0]) {
      setSelectedAreaID(selectedFloorAreas[0].id)
      return
    }
    if (selectedAreaID && selectedFloorAreas.length > 0 && !selectedFloorAreas.some((area) => area.id === selectedAreaID)) {
      setSelectedAreaID(selectedFloorAreas[0].id)
      return
    }
    if (selectedAreaID && selectedFloorAreas.length === 0) {
      setSelectedAreaID("")
    }
  }, [selectedAreaID, selectedFloorAreas])

  useEffect(() => {
    setAreaName(selectedArea?.name ?? "")
  }, [selectedArea])

  async function refreshFloors() {
    await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
  }

  const createFloorMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !place) {
        throw new Error("place is required")
      }
      return createFloor(token, {
        tenant_id: tenantID,
        building_id: place.id,
        place_id: place.id,
        name: newFloorName.trim(),
      })
    },
    onSuccess: async (floor) => {
      setSelectedFloorID(floor.id)
      setNewFloorName("")
      setAddFloorOpen(false)
      setActionNotice("Floor created.")
      setActionError("")
      await refreshFloors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Floor create failed")
    },
  })

  const updateFloorMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedFloor) {
        throw new Error("floor is required")
      }
      return updateFloor(token, selectedFloor.id, {
        tenant_id: tenantID,
        building_id: selectedFloor.placeId,
        place_id: selectedFloor.placeId,
        name: floorName.trim(),
      })
    },
    onSuccess: async () => {
      setActionNotice("Floor saved.")
      setActionError("")
      await refreshFloors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Floor save failed")
    },
  })

  const deleteFloorMutation = useMutation({
    mutationFn: (floor: MistyisletFloorResource) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return deleteFloor(token, floor.id, tenantID)
    },
    onSuccess: async () => {
      setSelectedFloorID("")
      setDeleteFloorTarget(null)
      setActionNotice("Floor deleted.")
      setActionError("")
      await refreshFloors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Floor delete failed")
    },
  })

  const createAreaMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !place || !selectedFloor) {
        throw new Error("floor is required")
      }
      return createArea(token, {
        tenant_id: tenantID,
        building_id: place.id,
        floor_id: selectedFloor.id,
        name: newAreaName.trim(),
      })
    },
    onSuccess: async (area) => {
      setSelectedAreaID(area.id)
      setNewAreaName("")
      setAddAreaOpen(false)
      setActiveTab("Areas")
      setActionNotice("Area created.")
      setActionError("")
      await refreshFloors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Area create failed")
    },
  })

  const updateAreaMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !place || !selectedFloor || !selectedArea) {
        throw new Error("area is required")
      }
      return updateArea(token, selectedArea.id, {
        tenant_id: tenantID,
        building_id: place.id,
        place_id: place.id,
        floor_id: selectedFloor.id,
        name: areaName.trim(),
      })
    },
    onSuccess: async () => {
      setActionNotice("Area saved.")
      setActionError("")
      await refreshFloors()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Area save failed")
    },
  })

  return (
    <>
      <PageFrame
        breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Floors"]}
        title="Floors"
        count={resourceQuery.isPending ? "--" : floors.length}
        description="Manage floors, areas, and door topology for this place"
        actions={
          <Button
            disabled={!canMutate}
            onClick={() => {
              setActionNotice("")
              setActionError("")
              setAddFloorOpen(true)
            }}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Floor
          </Button>
        }
      >
        {resourceQuery.usingFallback ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            Live floor resources are unavailable. Showing reference data.
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
          <div className="grid gap-4 p-6 md:grid-cols-3">
            {floors.map((floor) => (
              <button
                key={floor.id}
                type="button"
                onClick={() => setSelectedFloorID(floor.id)}
                className={cn(
                  "rounded-[6px] border border-[#eceef2] p-5 text-left transition-colors hover:bg-[#fbfbfc]",
                  selectedFloor?.id === floor.id && "bg-[#f4f3ef]"
                )}
              >
                <LayersIcon className="size-6 text-[#6f717c]" />
                <h2 className="mt-5 text-lg font-semibold text-[#17171c]">{floor.name}</h2>
                <p className="mt-2 min-h-10 text-sm text-[#6f717c]">{floor.description}</p>
                <div className="mt-5 flex items-center justify-between">
                  <span className="text-sm font-semibold text-[#2f3037]">{floor.doorCount} doors</span>
                  <StatusDot tone={floor.tone} label={floor.statusLabel} />
                </div>
              </button>
            ))}
            {floors.length === 0 ? (
              <div className="col-span-full px-6 py-10 text-center text-sm text-[#6f717c]">No floors found for this place.</div>
            ) : null}
          </div>
        </section>

        <SettingsPanel
          tabs={["General", "Areas", "Doors", "Hardware", "Settings"]}
          active={activeTab}
          onTabChange={setActiveTab}
          footer={
            activeTab === "General" ? (
              <Button
                disabled={!canMutate || !selectedFloor || updateFloorMutation.isPending || !floorName.trim()}
                onClick={() => updateFloorMutation.mutate()}
                className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
              >
                {updateFloorMutation.isPending ? "Saving..." : "Save"}
              </Button>
            ) : activeTab === "Areas" ? (
              <Button
                disabled={!canMutate || !selectedArea || updateAreaMutation.isPending || !areaName.trim()}
                onClick={() => updateAreaMutation.mutate()}
                className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
              >
                {updateAreaMutation.isPending ? "Saving..." : "Save Area"}
              </Button>
            ) : null
          }
        >
          {activeTab === "General" ? (
            <>
              <PanelHeader
                title={selectedFloor?.name ?? "No floor selected"}
                description="Floor metadata and default access behavior."
                action={
                  <Button
                    type="button"
                    variant="destructive"
                    disabled={!canMutate || !selectedFloor || deleteFloorMutation.isPending}
                    onClick={() => {
                      if (selectedFloor) {
                        setActionError("")
                        setDeleteFloorTarget(selectedFloor)
                      }
                    }}
                    className="h-10 rounded-[6px] px-5"
                  >
                    <Trash2Icon className="mr-1.5 size-4" />
                    Delete Floor
                  </Button>
                }
              />
              <div className="grid gap-6 p-7 md:grid-cols-2">
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Name</span>
                  <input
                    value={floorName}
                    disabled={!selectedFloor}
                    onChange={(event) => setFloorName(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                  />
                </label>
                <FormField label="Description" value={selectedFloor?.description ?? "No areas mapped yet"} />
                <FormField label="Default group" value="Engineering Team access" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
                <FormField label="Timezone" value="Asia/Jakarta" />
              </div>
            </>
	          ) : null}

          {activeTab === "Areas" ? (
            <>
              <PanelHeader
                title="Areas"
                description="Zones within this floor used by door placement and access scopes."
                action={
                  <Button
                    type="button"
                    variant="outline"
                    disabled={!canMutate || !selectedFloor}
                    onClick={() => {
                      setActionNotice("")
                      setActionError("")
                      setAddAreaOpen(true)
                    }}
                    className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                  >
                    <PlusIcon className="mr-1.5 size-4" />
                    Add Area
                  </Button>
                }
              />
              <div className="grid gap-6 p-7 lg:grid-cols-[minmax(220px,280px)_1fr]">
                <div className="space-y-2">
                  {selectedFloorAreas.map((area) => (
                    <button
                      key={area.id}
                      type="button"
                      onClick={() => setSelectedAreaID(area.id)}
                      className={cn(
                        "flex w-full items-center justify-between rounded-[6px] border border-[#eceef2] px-4 py-3 text-left text-sm hover:bg-[#fbfbfc]",
                        selectedArea?.id === area.id && "bg-[#f4f3ef]"
                      )}
                    >
                      <span className="font-semibold text-[#17171c]">{area.name}</span>
                      <span className="text-[#6f717c]">{area.doorCount}</span>
                    </button>
                  ))}
                  {selectedFloorAreas.length === 0 ? (
                    <div className="rounded-[6px] border border-[#eceef2] px-4 py-8 text-center text-sm text-[#6f717c]">No areas are mapped to this floor.</div>
                  ) : null}
                </div>
                <div className="grid gap-6 md:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Name</span>
                    <input
                      value={areaName}
                      disabled={!selectedArea}
                      onChange={(event) => setAreaName(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                    />
                  </label>
                  <FormField label="Door count" value={selectedArea ? String(selectedArea.doorCount) : "No area selected"} />
                  <FormField label="Floor" value={selectedFloor?.name ?? "No floor selected"} />
                  <FormField label="Place" value={place?.name ?? "Assigned Place"} />
                </div>
              </div>
            </>
          ) : null}

          {activeTab === "Doors" ? (
            <>
              <PanelHeader title="Doors" description="Doors currently mapped to this floor." />
              <div className="divide-y divide-[#eceef2]">
                {selectedFloorDoors.map((door) => (
                  <div key={door.id} className="grid gap-3 px-7 py-5 md:grid-cols-[240px_160px_1fr] md:items-center">
                    <span className="font-semibold text-[#4f55ff]">{door.name}</span>
                    <StatusDot tone={door.status === "online" ? "success" : "warning"} label={door.status === "online" ? "Online" : "Review"} />
                    <span className="text-sm text-[#6f717c]">{door.gatewaySerial}</span>
                  </div>
                ))}
                {selectedFloorDoors.length === 0 ? (
                  <div className="px-7 py-10 text-center text-sm text-[#6f717c]">No doors are mapped to this floor.</div>
                ) : null}
              </div>
            </>
          ) : null}

          {activeTab === "Hardware" ? (
            <>
              <PanelHeader title="Hardware" description="Devices installed on this floor." />
              <div className="grid gap-4 p-7 md:grid-cols-2">
                {selectedFloorHardware.map((item) => (
                  <div key={item.id} className="rounded-[6px] border border-[#eceef2] p-5">
                    <ServerIcon className="size-6 text-[#6f717c]" />
                    <h3 className="mt-5 font-semibold text-[#17171c]">{item.name}</h3>
                    <div className="mt-4"><StatusDot tone={item.tone} label={item.statusLabel} /></div>
                    <p className="mt-2 text-sm text-[#6f717c]">{item.location}</p>
                  </div>
                ))}
                {selectedFloorHardware.length === 0 ? (
                  <div className="col-span-full px-7 py-10 text-center text-sm text-[#6f717c]">No hardware is mapped to this floor.</div>
                ) : null}
              </div>
            </>
          ) : null}

          {activeTab === "Settings" ? (
            <>
              <PanelHeader title="Settings" description="Floor-level automation and alerts." />
              <SettingToggleRows
                rows={[
                  ["Apply new doors to default group", true, "New doors on this floor inherit the selected default group.", KeyRoundIcon],
                  ["Alert on offline reader", true, "Notify place admins if any floor reader goes offline.", BellIcon],
                  ["Show in occupancy reports", false, "Include this floor in capacity management once enabled.", BarChart3Icon],
                ]}
              />
            </>
          ) : null}
        </SettingsPanel>
      </PageFrame>

      <ConfirmActionDialog
        open={Boolean(deleteFloorTarget)}
        onOpenChange={(open) => {
          if (!deleteFloorMutation.isPending && !open) {
            setDeleteFloorTarget(null)
          }
        }}
        title="Delete floor"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{deleteFloorTarget?.name ?? "this floor"}</span> from{" "}
            {place?.name ?? "this place"}. It currently contains {deleteFloorTarget?.doorCount ?? 0} door
            {(deleteFloorTarget?.doorCount ?? 0) === 1 ? "" : "s"}.
          </>
        }
        confirmLabel="Delete floor"
        pending={deleteFloorMutation.isPending}
        disabled={!canMutate || !deleteFloorTarget}
        destructive
        onConfirm={() => {
          if (deleteFloorTarget) {
            deleteFloorMutation.mutate(deleteFloorTarget)
          }
        }}
      />

      <Sheet open={addFloorOpen} onOpenChange={setAddFloorOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[420px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Add Floor</SheetTitle>
            <SheetDescription>{place?.name ?? "Selected place"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createFloorMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Name</span>
              <input
                value={newFloorName}
                onChange={(event) => setNewFloorName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setAddFloorOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!canMutate || createFloorMutation.isPending || !newFloorName.trim()}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createFloorMutation.isPending ? "Creating..." : "Add Floor"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={addAreaOpen} onOpenChange={setAddAreaOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[420px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Add Area</SheetTitle>
            <SheetDescription>{selectedFloor?.name ?? place?.name ?? "Selected floor"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createAreaMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Name</span>
              <input
                value={newAreaName}
                onChange={(event) => setNewAreaName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setAddAreaOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!canMutate || !selectedFloor || createAreaMutation.isPending || !newAreaName.trim()}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createAreaMutation.isPending ? "Creating..." : "Add Area"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}
