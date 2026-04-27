import { useState } from "react"
import { ChevronDownIcon, KeyRoundIcon, PlusIcon, ShieldCheckIcon, Trash2Icon } from "lucide-react"

import { KisiEmptyTableRow } from "@/components/kisi/data-display"
import {
  FormField,
  InfoBanner,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
} from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { selectKisiPlaceContext } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"

export function TeamsAdaptedPage({
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
  const [activeTab, setActiveTab] = useState("Members")
  const [selectedTeamID, setSelectedTeamID] = useState("")
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const placeContext = selectKisiPlaceContext(resourceQuery.summary, placeID)
  const teams = placeScoped ? placeContext.teams : resourceQuery.summary.teams
  const teamMemberships = placeScoped ? placeContext.teamMemberships : resourceQuery.summary.teamMemberships
  const accessRights = placeScoped ? placeContext.accessRights : resourceQuery.summary.accessRights
  const selectedTeam = teams.find((team) => team.id === selectedTeamID) ?? teams[0] ?? null
  const selectedMemberships = selectedTeam ? teamMemberships.filter((membership) => membership.teamId === selectedTeam.id) : []
  const selectedAccessRights = selectedTeam
    ? accessRights.filter((accessRight) => accessRight.subjectType === "Team" && accessRight.name === selectedTeam.name)
    : []

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Teams"] : ["Home", "Teams"]}
      title={selectedTeam?.name ?? "Teams"}
      count={resourceQuery.isPending ? "--" : teams.length}
      description="Use teams to group similar users and assign Access Rights in batches"
      actions={
        <>
          <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
            <PlusIcon className="mr-1.5 size-4" />
            Add Member
          </Button>
          <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
            Delete Team
          </Button>
        </>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live team resources are unavailable. Showing reference data.
        </div>
      ) : null}

      <InfoBanner>
        Teams and Groups are both kept because they solve different Mistyislet workflows: Teams collect users; Groups define access-control resources and restrictions.
      </InfoBanner>

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-base font-semibold text-[#17171c]">Teams</h2>
          <p className="mt-1 text-sm text-[#6f717c]">Select a team to review membership and assigned Access Rights.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[780px] text-left text-sm">
            <thead className="bg-[#fbfbfc]">
              <tr className="border-b border-[#eceef2]">
                <th className="px-6 py-4 font-semibold">Name</th>
                <th className="px-4 py-4 font-semibold">Scope</th>
                <th className="px-4 py-4 font-semibold">Members</th>
                <th className="px-4 py-4 font-semibold">Access Rights</th>
                <th className="px-4 py-4 font-semibold">Status</th>
              </tr>
            </thead>
            <tbody>
              {teams.map((team) => (
                <tr
                  key={team.id}
                  className={`border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc] ${selectedTeam?.id === team.id ? "bg-[#f7f7ff]" : ""}`}
                >
                  <td className="px-6 py-5">
                    <button type="button" onClick={() => setSelectedTeamID(team.id)} className="font-semibold text-[#4f55ff]">
                      {team.name}
                    </button>
                  </td>
                  <td className="px-4 py-5 text-[#2f3037]">{team.scopeLabel}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{team.memberCount}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{team.accessRightCount}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={team.tone} label={team.statusLabel} />
                  </td>
                </tr>
              ))}
              {teams.length === 0 ? <KisiEmptyTableRow colSpan={5}>No teams found.</KisiEmptyTableRow> : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Members", "Access Rights", "Settings"]}
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
            <PanelHeader title="General" description="Team identity and ownership." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <FormField label="Team name" value={selectedTeam?.name ?? "No team selected"} />
              <FormField label="Owner" value="Organization Admin" />
              <FormField label="Default place" value={selectedTeam?.scopeLabel ?? "Unassigned"} trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <FormField label="Directory source" value={selectedTeam?.sourceLabel ?? "Manual"} />
            </div>
            <div className="border-t border-[#eceef2] px-7 py-5 text-sm text-[#6f717c]">
              {selectedTeam?.description ?? "Create a team to start assigning batch access rights."}
            </div>
          </>
        ) : null}

        {activeTab === "Members" ? (
          <>
            <PanelHeader
              title="Members"
              description="Users assigned directly or through directory sync."
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                  Add Members
                </Button>
              }
            />
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-left text-sm">
                <thead>
                  <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                    <th className="px-7 py-4 font-semibold">Name</th>
                    <th className="px-4 py-4 font-semibold">Email</th>
                    <th className="px-4 py-4 font-semibold">Source</th>
                    <th className="px-4 py-4 font-semibold">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {selectedMemberships.map((membership) => (
                    <tr key={membership.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                      <td className="px-7 py-5 font-semibold text-[#4f55ff]">{membership.name}</td>
                      <td className="px-4 py-5 text-[#6f717c]">{membership.email}</td>
                      <td className="px-4 py-5 text-[#2f3037]">{membership.sourceLabel}</td>
                      <td className="px-4 py-5">
                        <StatusDot tone={membership.tone} label={membership.statusLabel} />
                      </td>
                    </tr>
                  ))}
                  {selectedMemberships.length === 0 ? (
                    <KisiEmptyTableRow colSpan={4}>No team members found.</KisiEmptyTableRow>
                  ) : null}
                </tbody>
              </table>
            </div>
          </>
        ) : null}

        {activeTab === "Access Rights" ? (
          <>
            <PanelHeader
              title="Access Rights"
              description="Access Rights assigned to every current and future member of this team."
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                  Assign Access Right
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {selectedAccessRights.map((accessRight) => (
                <div key={accessRight.id} className="grid gap-3 px-7 py-5 md:grid-cols-[210px_170px_1fr_170px] md:items-center">
                  <span className="font-semibold text-[#17171c]">{accessRight.ruleLabel}</span>
                  <span className="text-sm text-[#2f3037]">{accessRight.targetLabel}</span>
                  <span className="text-sm text-[#6f717c]">{accessRight.subjectType}</span>
                  <StatusDot tone={accessRight.tone} label={accessRight.statusLabel} />
                </div>
              ))}
              {selectedAccessRights.length === 0 ? (
                <div className="px-7 py-10 text-sm text-[#6f717c]">No team Access Rights assigned.</div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Settings" ? (
          <>
            <PanelHeader title="Settings" description="Directory sync and lifecycle automation." />
            <SettingToggleRows
              rows={[
                ["Sync from SCIM", true, "Maintain team membership from the connected directory.", ShieldCheckIcon],
                ["Auto-share access", true, "Apply assigned Access Rights when a user joins this team.", KeyRoundIcon],
                ["Remove access on leave", true, "Revoke team-based Access Rights when membership is removed.", Trash2Icon],
              ]}
            />
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
