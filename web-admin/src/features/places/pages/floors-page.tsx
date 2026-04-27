import { useEffect, useMemo, useState } from "react"
import {
  BarChart3Icon,
  BellIcon,
  ChevronDownIcon,
  KeyRoundIcon,
  LayersIcon,
  PlusIcon,
  ServerIcon,
} from "lucide-react"

import {
  FormField,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
} from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { useKisiPlaceContext } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

export function FloorsAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const [activeTab, setActiveTab] = useState("General")
  const [selectedFloorID, setSelectedFloorID] = useState("")
  const resourceQuery = useKisiPlaceContext(token, viewer, placeID)
  const { place, floors, doors, hardware } = resourceQuery.context
  const selectedFloor = floors.find((floor) => floor.id === selectedFloorID) ?? floors[0]
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

  return (
    <PageFrame
      breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Floors"]}
      title="Floors"
      count={resourceQuery.isPending ? "--" : floors.length}
      description="Manage floors, areas, and door topology for this place"
      actions={
        <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
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
        tabs={["General", "Doors", "Hardware", "Settings"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={<Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">Save</Button>}
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title={selectedFloor?.name ?? "No floor selected"} description="Floor metadata and default access behavior." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <FormField label="Name" value={selectedFloor?.name ?? "No floor selected"} />
              <FormField label="Description" value={selectedFloor?.description ?? "No areas mapped yet"} />
              <FormField label="Default group" value="Engineering Team access" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <FormField label="Timezone" value="Asia/Jakarta" />
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
  )
}
