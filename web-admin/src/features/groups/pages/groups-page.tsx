import { useState } from "react"
import {
  ChevronDownIcon,
  CreditCardIcon,
  DoorOpenIcon,
  LayersIcon,
  MapPinPlusIcon,
  PlusIcon,
  ShieldCheckIcon,
} from "lucide-react"

import { FormField, PageFrame, PanelHeader, SettingsPanel, StatusDot, ToggleSwitch } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { selectKisiPlaceContext } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

export function GroupsAdaptedPage({
  token,
  viewer,
  placeID,
  placeScoped = false,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
  placeScoped?: boolean
}) {
  const [activeTab, setActiveTab] = useState("Permissions")
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const placeContext = selectKisiPlaceContext(resourceQuery.summary, placeID)
  const groupRows = placeScoped ? placeContext.groups : resourceQuery.summary.groups
  const currentGroup = groupRows[0]
  const currentAccessRights = (placeScoped ? placeContext.accessRights : resourceQuery.summary.accessRights)
    .filter((item) => !currentGroup || item.name === currentGroup.name || item.subjectType === "Group")
    .slice(0, 5)
  const memberRows = (placeScoped ? placeContext.users : resourceQuery.summary.users).slice(0, 5)
  const doorRows = (placeScoped ? placeContext.doors : resourceQuery.summary.doors).slice(0, 5)
  const floorRows = (placeScoped ? placeContext.floors : resourceQuery.summary.floors).slice(0, 4)
  const restrictions = [
    ["Primary Device Restriction", "Only allow unlocks from primary smartphones.", true, ShieldCheckIcon],
    ["Allow App Access", "If enabled, users may access using Mistyislet mobile and web apps.", true, CreditCardIcon],
    ["Geofence Restriction", "Require users in this group to be at the place location for this group's doors.", true, MapPinPlusIcon],
    ["Reader Restriction", "Require unlocks at the reader for this group's doors.", false, DoorOpenIcon],
  ] as const

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Groups"] : ["Home", "Groups"]}
      title={currentGroup?.name ?? (placeScoped ? "Place Groups" : "Groups")}
      count={resourceQuery.isPending ? "--" : groupRows.length}
      description="Configure the users, doors, floors, schedules, and restrictions for access groups"
      actions={
        <>
          <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
            <PlusIcon className="mr-1.5 size-4" />
            Add Group
          </Button>
          <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
            Delete Group
          </Button>
        </>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live group resources are unavailable. Showing reference data.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-base font-semibold text-[#17171c]">{placeScoped ? "Place Groups" : "Organization Groups"}</h2>
          <p className="mt-1 text-sm text-[#6f717c]">Review user and door groups that define access coverage for this scope.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead className="bg-[#fbfbfc]">
              <tr className="border-b border-[#eceef2]">
                <th className="px-6 py-4 font-semibold">Name</th>
                <th className="px-4 py-4 font-semibold">Type</th>
                <th className="px-4 py-4 font-semibold">Target</th>
                <th className="px-4 py-4 font-semibold">Members / Doors</th>
                <th className="px-4 py-4 font-semibold">Status</th>
              </tr>
            </thead>
            <tbody>
              {groupRows.map((group) => (
                <tr key={group.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-4 font-semibold text-[#4f55ff]">{group.name}</td>
                  <td className="px-4 py-4 text-[#2f3037]">{group.kind}</td>
                  <td className="px-4 py-4 text-[#6f717c]">{group.targetLabel}</td>
                  <td className="px-4 py-4 text-[#2f3037]">{group.memberCount}</td>
                  <td className="px-4 py-4">
                    <StatusDot tone={group.tone} label={group.statusLabel} />
                  </td>
                </tr>
              ))}
              {groupRows.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-10 text-center text-sm text-[#6f717c]">
                    No groups found for this scope.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Members", "Doors", "Floors", "Time Restrictions", "Permissions"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">
            Save
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Group identity and default access behavior." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <FormField label="Group name" value={currentGroup?.name ?? "No group selected"} />
              <FormField label="Access enabled" value={currentGroup?.statusLabel ?? "Unavailable"} trailing={<ToggleSwitch enabled={currentGroup?.tone !== "danger"} />} />
              <FormField label="Default role" value="Basic" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <FormField label="Place scope" value={placeScoped ? placeContext.place?.name ?? "Assigned Place" : "All organization places"} />
              <FormField label="Assigned rights" value={`${currentAccessRights.length} access rights`} />
            </div>
          </>
        ) : null}

        {activeTab === "Members" ? (
          <>
            <PanelHeader
              title="Members"
              description="Users and teams receiving this group's door access."
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                  Add Members
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {memberRows.map((row) => (
                <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row.name}</span>
                  <span className="text-sm text-[#2f3037]">{row.role}</span>
                  <span className="text-sm text-[#6f717c]">{row.email}</span>
                </div>
              ))}
              {memberRows.length === 0 ? <div className="px-7 py-8 text-sm text-[#6f717c]">No users found for this group scope.</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "Doors" ? (
          <>
            <PanelHeader
              title="Doors"
              description="Door resources controlled by this group."
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                  Add Doors
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {doorRows.map((row) => (
                <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_180px_1fr] md:items-center">
                  <span className="font-semibold text-[#4f55ff]">{row.name}</span>
                  <span className="text-sm text-[#2f3037]">{row.floorName}</span>
                  <StatusDot tone={row.status === "online" ? "success" : "warning"} label={row.status === "online" ? "Online" : "Review"} />
                </div>
              ))}
              {doorRows.length === 0 ? <div className="px-7 py-8 text-sm text-[#6f717c]">No doors found for this group scope.</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "Floors" ? (
          <>
            <PanelHeader title="Floors" description="Floor-level access shortcuts for this group." />
            <div className="grid gap-4 p-7 md:grid-cols-2">
              {floorRows.map((row) => (
                <div key={row.id} className="rounded-[6px] border border-[#eceef2] p-5">
                  <LayersIcon className="size-6 text-[#6f717c]" />
                  <h3 className="mt-5 font-semibold text-[#17171c]">{row.name}</h3>
                  <p className="mt-1 text-sm text-[#6f717c]">{row.description}</p>
                  <p className="mt-4 text-sm font-semibold text-[#2f3037]">{row.doorCount} doors</p>
                </div>
              ))}
              {floorRows.length === 0 ? <div className="text-sm text-[#6f717c]">No floors found for this group scope.</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "Time Restrictions" ? (
          <>
            <PanelHeader title="Time Restrictions" description="Weekly access windows and exceptions." />
            <div className="p-7">
              <div className="grid min-h-[240px] grid-cols-[56px_repeat(5,minmax(90px,1fr))] overflow-hidden rounded-[6px] border border-[#eceef2]">
                <div className="border-r border-[#eceef2] bg-white pt-12 text-right text-xs font-semibold text-[#9a9ca7]">
                  {["8 AM", "12 PM", "4 PM", "8 PM"].map((time) => (
                    <div key={time} className="h-11 pr-3">
                      {time}
                    </div>
                  ))}
                </div>
                {["Mon", "Tue", "Wed", "Thu", "Fri"].map((day) => (
                  <div key={day} className="border-r border-white/80 bg-[#f1f2f5] text-center text-sm font-semibold text-[#6f717c] last:border-r-0">
                    <div className="border-b border-white/80 bg-white py-4">{day}</div>
                    <div className="mx-auto mt-16 w-full max-w-[118px] rounded-[5px] bg-[#202443] p-3 text-left text-xs text-white">
                      <p className="truncate font-semibold">Access permitted</p>
                      <p className="mt-1 text-white/70">08:00 - 18:00</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </>
        ) : null}

        {activeTab === "Permissions" ? (
          <>
            <PanelHeader title="Permissions" description="App, geofence, reader, and primary-device restrictions." />
            <div className="divide-y divide-[#eceef2] px-7">
              {restrictions.map(([title, description, enabled, Icon]) => (
                <div key={title} className="flex gap-5 py-6">
                  <div
                    className={cn(
                      "flex size-12 shrink-0 items-center justify-center rounded-[6px]",
                      enabled ? "bg-[#4f55ff] text-white" : "bg-[#f1f2f5] text-[#2f3037]"
                    )}
                  >
                    <Icon className="size-6" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="text-lg font-semibold text-[#17171c]">{title}</h3>
                    <p className="mt-1 max-w-3xl text-sm leading-6 text-[#6f717c]">
                      {description} <span className="text-[#4f55ff] underline underline-offset-2">Learn more</span>
                    </p>
                  </div>
                  <div className="ml-auto pt-2">
                    <ToggleSwitch enabled={enabled} />
                  </div>
                </div>
              ))}
            </div>
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
