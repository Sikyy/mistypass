import { useEffect, useMemo, useState } from "react"
import {
  ChevronDownIcon,
  Clock3Icon,
  DoorOpenIcon,
  KeyRoundIcon,
  MapPinPlusIcon,
  SearchIcon,
  ShieldCheckIcon,
} from "lucide-react"

import {
  FormField,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
} from "@/components/kisi/primitives"
import { KisiEmptyTableRow } from "@/components/kisi/data-display"
import { Button } from "@/components/ui/button"
import { useKisiPlaceContext } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

export function DoorDetailAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const [activeTab, setActiveTab] = useState("General")
  const [selectedDoorID, setSelectedDoorID] = useState("")
  const resourceQuery = useKisiPlaceContext(token, viewer, placeID)
  const { place, doors, hardware, events } = resourceQuery.context
  const selectedDoor = doors.find((door) => door.id === selectedDoorID) ?? doors[0]
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

  return (
    <PageFrame
      breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Doors"]}
      title={selectedDoor?.name ?? "Doors"}
      count={resourceQuery.isPending ? "--" : doors.length}
      description={selectedDoor ? `${selectedDoor.kind} door on ${selectedDoor.floorName}` : "Door settings and access schedules"}
      actions={
        <>
          <Button variant="interaction" className="h-10 rounded-[6px] text-[#4f55ff]">Lockdown</Button>
          <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:bg-[#fbfbfc]">Delete Door</Button>
        </>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live door resources are unavailable. Showing reference data.
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
              {doors.length === 0 ? <KisiEmptyTableRow colSpan={5}>No doors found for this place.</KisiEmptyTableRow> : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Groups", "Hardware", "Events", "Permissions"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={<Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">Save</Button>}
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Door name, floor, location, and lock behavior." />
            <div className="space-y-6 p-7">
              <FormField label="Name" value={selectedDoor?.name ?? "No door selected"} />
              <FormField label="Description" value={selectedDoor?.description ?? "No door selected"} />
              <FormField label="Belongs to floor" value={selectedDoor?.floorName ?? "Unassigned floor"} trailing={<SearchIcon className="size-4 text-[#6f717c]" />} />
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
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
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
          <Button variant="outline" className="h-11 rounded-[6px] border-[#8589ff] bg-white text-[#4f55ff] hover:bg-[#fbfbfc]">
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
  )
}
