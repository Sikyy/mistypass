import i18next from "i18next"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
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
  const { t } = useTranslation()
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
      setActionNotice(t("kisi.floors.addFloor"))
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
      setActionNotice(t("common.save"))
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
      setActionNotice(t("kisi.floors.deleteFloor"))
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
      setActionNotice(t("kisi.floors.addArea"))
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
      setActionNotice(t("common.save"))
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
        breadcrumbs={[t("common.home"), "Places", place?.name ?? "Assigned Place", "Floors"]}
        title={t("kisi.floors.title")}
        count={resourceQuery.isPending ? "--" : floors.length}
        description={t("kisi.floors.description")}
        actions={
          <Button
            disabled={!canMutate}
            onClick={() => {
              setActionNotice("")
              setActionError("")
              setAddFloorOpen(true)
            }}
            className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Floor
          </Button>
        }
      >
        {resourceQuery.usingFallback ? (
          <div className="mp-alert-warning">
            Live floor resources are unavailable. Showing reference data.
          </div>
        ) : null}
        {actionNotice ? (
          <div className="mp-alert-success">
            {actionNotice}
          </div>
        ) : null}
        {actionError ? (
          <div className="mp-alert-warning">
            {actionError}
          </div>
        ) : null}

        <section className="overflow-hidden rounded-[6px] border border-line-default bg-white">
          <div className="grid gap-4 p-6 md:grid-cols-3">
            {floors.map((floor) => (
              <button
                key={floor.id}
                type="button"
                onClick={() => setSelectedFloorID(floor.id)}
                className={cn(
                  "rounded-[6px] border border-line-subtle p-5 text-left transition-colors hover:bg-surface-page",
                  selectedFloor?.id === floor.id && "bg-surface-selected"
                )}
              >
                <LayersIcon className="size-6 text-content-subtle" />
                <h2 className="mt-5 text-lg font-semibold text-content-heading">{floor.name}</h2>
                <p className="mt-2 min-h-10 text-sm text-content-subtle">{floor.description}</p>
                <div className="mt-5 flex items-center justify-between">
                  <span className="text-sm font-semibold text-content-body">{floor.doorCount} doors</span>
                  <StatusDot tone={floor.tone} label={floor.statusLabel} />
                </div>
              </button>
            ))}
            {floors.length === 0 ? (
              <div className="col-span-full px-6 py-10 text-center text-sm text-content-subtle">{t("kisi.floors.noAreas")}</div>
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
                className="h-10 rounded-[8px] bg-brand px-8 text-white hover:bg-brand-hover disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
              >
                {updateFloorMutation.isPending ? "Saving..." : "Save"}
              </Button>
            ) : activeTab === "Areas" ? (
              <Button
                disabled={!canMutate || !selectedArea || updateAreaMutation.isPending || !areaName.trim()}
                onClick={() => updateAreaMutation.mutate()}
                className="h-10 rounded-[8px] bg-brand px-8 text-white hover:bg-brand-hover disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
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
                description={t("kisi.floors.description")}
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
                  <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.name")}</span>
                  <input
                    value={floorName}
                    disabled={!selectedFloor}
                    onChange={(event) => setFloorName(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-sunken"
                  />
                </label>
                <FormField label={t("common.description")} value={selectedFloor?.description ?? "No areas mapped yet"} />
                <FormField label={t("common.group")} value={i18next.t("common.group")} trailing={<ChevronDownIcon className="size-4 text-content-subtle" />} />
                <FormField label={t("kisi.doors.timezone")} value="Asia/Jakarta" />
              </div>
            </>
	          ) : null}

          {activeTab === "Areas" ? (
            <>
              <PanelHeader
                title={t("kisi.floors.areas")}
                description={t("kisi.floors.description")}
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
                    className="h-10 rounded-[6px] border-brand-ring bg-white px-5 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
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
                        "flex w-full items-center justify-between rounded-[6px] border border-line-subtle px-4 py-3 text-left text-sm hover:bg-surface-page",
                        selectedArea?.id === area.id && "bg-surface-selected"
                      )}
                    >
                      <span className="font-semibold text-content-heading">{area.name}</span>
                      <span className="text-content-subtle">{area.doorCount}</span>
                    </button>
                  ))}
                  {selectedFloorAreas.length === 0 ? (
                    <div className="rounded-[6px] border border-line-subtle px-4 py-8 text-center text-sm text-content-subtle">{t("kisi.floors.noAreas")}</div>
                  ) : null}
                </div>
                <div className="grid gap-6 md:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.name")}</span>
                    <input
                      value={areaName}
                      disabled={!selectedArea}
                      onChange={(event) => setAreaName(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-sunken"
                    />
                  </label>
                  <FormField label={t("kisi.doors.title")} value={selectedArea ? String(selectedArea.doorCount) : "No area selected"} />
                  <FormField label={t("kisi.doors.floor")} value={selectedFloor?.name ?? "No floor selected"} />
                  <FormField label={t("common.place")} value={place?.name ?? "Assigned Place"} />
                </div>
              </div>
            </>
          ) : null}

          {activeTab === "Doors" ? (
            <>
              <PanelHeader title={t("kisi.floors.doors")} description={t("kisi.floors.description")} />
              <div className="divide-y divide-line-subtle">
                {selectedFloorDoors.map((door) => (
                  <div key={door.id} className="grid gap-3 px-7 py-5 md:grid-cols-[240px_160px_1fr] md:items-center">
                    <span className="font-semibold text-brand">{door.name}</span>
                    <StatusDot tone={door.status === "online" ? "success" : "warning"} label={door.status === "online" ? "Online" : "Review"} />
                    <span className="text-sm text-content-subtle">{door.gatewaySerial}</span>
                  </div>
                ))}
                {selectedFloorDoors.length === 0 ? (
                  <div className="px-7 py-10 text-center text-sm text-content-subtle">{t("kisi.floors.noDoors")}</div>
                ) : null}
              </div>
            </>
          ) : null}

          {activeTab === "Hardware" ? (
            <>
              <PanelHeader title={t("common.hardware")} description={t("kisi.floors.description")} />
              <div className="grid gap-4 p-7 md:grid-cols-2">
                {selectedFloorHardware.map((item) => (
                  <div key={item.id} className="rounded-[6px] border border-line-subtle p-5">
                    <ServerIcon className="size-6 text-content-subtle" />
                    <h3 className="mt-5 font-semibold text-content-heading">{item.name}</h3>
                    <div className="mt-4"><StatusDot tone={item.tone} label={item.statusLabel} /></div>
                    <p className="mt-2 text-sm text-content-subtle">{item.location}</p>
                  </div>
                ))}
                {selectedFloorHardware.length === 0 ? (
                  <div className="col-span-full px-7 py-10 text-center text-sm text-content-subtle">{t("kisi.floors.noDoors")}</div>
                ) : null}
              </div>
            </>
          ) : null}

          {activeTab === "Settings" ? (
            <>
              <PanelHeader title={t("common.settings")} description={t("kisi.floors.description")} />
              <SettingToggleRows
                rows={[
                  [i18next.t("common.permissions"), true, "New doors on this floor inherit the selected default group.", KeyRoundIcon],
                  [i18next.t("kisi.orgSetup.notifications"), true, "Notify place admins if any floor reader goes offline.", BellIcon],
                  [i18next.t("kisi.reports.title"), false, "Include this floor in capacity management once enabled.", BarChart3Icon],
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
        title={t("kisi.floors.deleteFloor")}
        description={
          <>
            This removes <span className="font-semibold text-content-heading">{deleteFloorTarget?.name ?? "this floor"}</span> from{" "}
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
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{t("kisi.floors.addFloor")}</SheetTitle>
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.name")}</span>
              <input
                value={newFloorName}
                onChange={(event) => setNewFloorName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            {actionError ? (
              <div className={cn("mp-alert-warning", "px-4 py-3")}>
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setAddFloorOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!canMutate || createFloorMutation.isPending || !newFloorName.trim()}
                className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
              >
                {createFloorMutation.isPending ? "Creating..." : t("kisi.floors.addFloor")}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={addAreaOpen} onOpenChange={setAddAreaOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[420px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{t("kisi.floors.addArea")}</SheetTitle>
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.name")}</span>
              <input
                value={newAreaName}
                onChange={(event) => setNewAreaName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            {actionError ? (
              <div className={cn("mp-alert-warning", "px-4 py-3")}>
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setAddAreaOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!canMutate || !selectedFloor || createAreaMutation.isPending || !newAreaName.trim()}
                className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
              >
                {createAreaMutation.isPending ? "Creating..." : t("kisi.floors.addArea")}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}
