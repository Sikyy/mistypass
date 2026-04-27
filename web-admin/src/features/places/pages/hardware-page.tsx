import { useEffect, useMemo, useState } from "react"
import { CloudIcon, FileTextIcon, PlusIcon, ServerIcon } from "lucide-react"

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

export function HardwareAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const [activeTab, setActiveTab] = useState("General")
  const [selectedDeviceID, setSelectedDeviceID] = useState("")
  const resourceQuery = useKisiPlaceContext(token, viewer, placeID)
  const { place, hardware, events } = resourceQuery.context
  const selectedDevice = hardware.find((item) => item.id === selectedDeviceID) ?? hardware[0]
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

  return (
    <PageFrame
      breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Hardware"]}
      title="Hardware"
      count={resourceQuery.isPending ? "--" : hardware.length}
      description="Gateways, controllers, readers, and terminals assigned to this place"
      actions={
        <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
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

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">Device</th>
                <th className="px-4 py-4 font-semibold">Type</th>
                <th className="px-4 py-4 font-semibold">Status</th>
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
              {hardware.length === 0 ? <KisiEmptyTableRow colSpan={4}>No hardware found for this place.</KisiEmptyTableRow> : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Doors", "Events", "Settings"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={<Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">Save</Button>}
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
            <PanelHeader title="Doors" description="Doors connected to this device or gateway path." />
            <div className="divide-y divide-[#eceef2]">
              {(selectedDevice?.doorNames ?? []).map((door, index) => (
                <div key={door} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_160px_1fr] md:items-center">
                  <span className="font-semibold text-[#4f55ff]">{door}</span>
                  <StatusDot tone={selectedDevice?.tone ?? "success"} label={selectedDevice?.statusLabel ?? "Online"} />
                  <span className="text-sm text-[#6f717c]">{index === 0 ? "Primary controller path" : "Reader assignment"}</span>
                </div>
              ))}
              {(selectedDevice?.doorNames.length ?? 0) === 0 ? (
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
            <PanelHeader title="Settings" description="Remote management and diagnostics." />
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
  )
}
