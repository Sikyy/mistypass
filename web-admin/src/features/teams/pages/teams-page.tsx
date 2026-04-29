import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ChevronDownIcon, KeyRoundIcon, PlusIcon, ShieldCheckIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { MistyisletEmptyTableRow } from "@/components/mistyislet/data-display"
import {
  FormField,
  InfoBanner,
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
import { selectMistyisletPlaceContext, type MistyisletTeamMembershipResource } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import {
  createRoleAssignment,
  createTeam,
  createTeamMembership,
  deleteTeam,
  deleteTeamMembership,
  listRoles,
  updateTeam,
  type CurrentUser,
  type Role,
  type Team,
  type TeamMembership,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

function defaultRoleForScope(roles: Role[], scope: Role["applies_to"]) {
  return roles.find((role) => role.applies_to === scope)?.id ?? (scope === "Group" ? "role_group_access" : scope === "Place" ? "role_place_admin" : "role_organization_admin")
}

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
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState("Members")
  const [selectedTeamID, setSelectedTeamID] = useState("")
  const [deleteTeamConfirmOpen, setDeleteTeamConfirmOpen] = useState(false)
  const [teamSheetOpen, setTeamSheetOpen] = useState(false)
  const [teamName, setTeamName] = useState("")
  const [teamDescription, setTeamDescription] = useState("")
  const [teamScope, setTeamScope] = useState<Team["scope"]>("place")
  const [teamPlaceID, setTeamPlaceID] = useState("")
  const [memberSheetOpen, setMemberSheetOpen] = useState(false)
  const [memberType, setMemberType] = useState<TeamMembership["member_type"]>("User")
  const [memberID, setMemberID] = useState("")
  const [memberEmail, setMemberEmail] = useState("")
  const [memberName, setMemberName] = useState("")
  const [deleteMembershipTarget, setDeleteMembershipTarget] = useState<MistyisletTeamMembershipResource | null>(null)
  const [accessSheetOpen, setAccessSheetOpen] = useState(false)
  const [accessScope, setAccessScope] = useState<Role["applies_to"]>("Place")
  const [accessScopeID, setAccessScopeID] = useState("")
  const [accessRoleID, setAccessRoleID] = useState("role_place_admin")
  const [validUntil, setValidUntil] = useState("")
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const rolesQuery = useQuery({
    queryKey: ["reference-roles"],
    queryFn: () => listRoles(token),
    staleTime: 60 * 1000,
  })
  const placeContext = selectMistyisletPlaceContext(resourceQuery.summary, placeID)
  const tenantID = getViewerTenantID(viewer)
  const teams = placeScoped ? placeContext.teams : resourceQuery.summary.teams
  const teamMemberships = placeScoped ? placeContext.teamMemberships : resourceQuery.summary.teamMemberships
  const accessRights = placeScoped ? placeContext.accessRights : resourceQuery.summary.accessRights
  const users = placeScoped ? placeContext.users : resourceQuery.summary.users
  const places = placeScoped && placeContext.place ? [placeContext.place] : resourceQuery.summary.places
  const groups = placeScoped ? placeContext.groups : resourceQuery.summary.groups
  const selectedTeam = teams.find((team) => team.id === selectedTeamID) ?? teams[0] ?? null
  const selectedMemberships = selectedTeam ? teamMemberships.filter((membership) => membership.teamId === selectedTeam.id) : []
  const selectedAccessRights = selectedTeam
    ? accessRights.filter((accessRight) => accessRight.subjectType === "Team" && accessRight.name === selectedTeam.name)
    : []
  const roles = rolesQuery.data ?? []
  const accessRoleOptions = roles.filter((role) => role.applies_to === accessScope)
  const accessScopeOptions: Role["applies_to"][] = placeScoped ? ["Place", "Group"] : ["Organization", "Place", "Group"]
  const assignableScopes =
    accessScope === "Organization"
      ? [{ id: tenantID, name: "Organization" }]
      : accessScope === "Place"
        ? places
        : groups
  const canMutate = Boolean(tenantID && !resourceQuery.usingFallback)

  useEffect(() => {
    if (!selectedTeam) {
      return
    }
    setTeamName(selectedTeam.name)
    setTeamDescription(selectedTeam.description)
    setTeamScope(selectedTeam.placeId ? "place" : "organization")
    setTeamPlaceID(selectedTeam.placeId || places[0]?.id || "")
  }, [selectedTeam, places])

  async function refreshTeams() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] }),
      queryClient.invalidateQueries({ queryKey: ["reference-roles"] }),
    ])
  }

  function openTeamSheet() {
    setTeamName("")
    setTeamDescription("")
    setTeamScope(placeScoped ? "place" : "place")
    setTeamPlaceID((placeScoped ? placeContext.place?.id : places[0]?.id) ?? "")
    setActionNotice("")
    setActionError("")
    setTeamSheetOpen(true)
  }

  function openMemberSheet() {
    const firstUser = users[0]
    setMemberType("User")
    setMemberID(firstUser?.id ?? "")
    setMemberEmail(firstUser?.email ?? "")
    setMemberName(firstUser?.name ?? "")
    setActionNotice("")
    setActionError("")
    setMemberSheetOpen(true)
  }

  function updateMemberID(nextID: string) {
    setMemberID(nextID)
    const user = users.find((item) => item.id === nextID)
    setMemberEmail(user?.email ?? "")
    setMemberName(user?.name ?? "")
  }

  function openAccessSheet() {
    const nextScope: Role["applies_to"] = placeScoped ? "Place" : "Place"
    setAccessScope(nextScope)
    setAccessScopeID((placeScoped ? placeContext.place?.id : places[0]?.id) ?? "")
    setAccessRoleID(defaultRoleForScope(roles, nextScope))
    setValidUntil("")
    setActionNotice("")
    setActionError("")
    setAccessSheetOpen(true)
  }

  function updateAccessScope(nextScope: Role["applies_to"]) {
    setAccessScope(nextScope)
    setAccessRoleID(defaultRoleForScope(roles, nextScope))
    if (nextScope === "Organization") {
      setAccessScopeID(tenantID)
    } else if (nextScope === "Place") {
      setAccessScopeID((placeScoped ? placeContext.place?.id : places[0]?.id) ?? "")
    } else {
      setAccessScopeID(groups[0]?.id ?? "")
    }
  }

  const createTeamMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return createTeam(token, {
        tenant_id: tenantID,
        name: teamName.trim(),
        scope: teamScope,
        place_id: teamScope === "place" ? teamPlaceID : undefined,
        description: teamDescription.trim() || undefined,
        source: "Manual",
      })
    },
    onSuccess: async (team) => {
      setSelectedTeamID(team.id)
      setTeamSheetOpen(false)
      setActionNotice("Team created.")
      setActionError("")
      await refreshTeams()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Team create failed"),
  })

  const updateTeamMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedTeam) {
        throw new Error("team is required")
      }
      return updateTeam(token, selectedTeam.id, {
        tenant_id: tenantID,
        name: teamName.trim(),
        scope: teamScope,
        place_id: teamScope === "place" ? teamPlaceID : undefined,
        description: teamDescription.trim() || undefined,
        source: selectedTeam.sourceLabel,
      })
    },
    onSuccess: async () => {
      setActionNotice("Team saved.")
      setActionError("")
      await refreshTeams()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Team save failed"),
  })

  const deleteTeamMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedTeam) {
        throw new Error("team is required")
      }
      return deleteTeam(token, selectedTeam.id, tenantID)
    },
    onSuccess: async () => {
      setSelectedTeamID("")
      setDeleteTeamConfirmOpen(false)
      setActionNotice("Team deleted.")
      setActionError("")
      await refreshTeams()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Team delete failed"),
  })

  const createMembershipMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedTeam) {
        throw new Error("team is required")
      }
      return createTeamMembership(token, {
        tenant_id: tenantID,
        team_id: selectedTeam.id,
        member_type: memberType,
        member_id: memberID.trim(),
        member_email: memberEmail.trim() || undefined,
        member_name: memberName.trim() || undefined,
        source: "Manual",
      })
    },
    onSuccess: async () => {
      setMemberSheetOpen(false)
      setActionNotice("Member added.")
      setActionError("")
      await refreshTeams()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Member add failed"),
  })

  const deleteMembershipMutation = useMutation({
    mutationFn: (membershipID: string) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return deleteTeamMembership(token, membershipID, tenantID)
    },
    onSuccess: async () => {
      setDeleteMembershipTarget(null)
      setActionNotice("Member removed.")
      setActionError("")
      await refreshTeams()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Member remove failed"),
  })

  const assignAccessRightMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !selectedTeam) {
        throw new Error("team is required")
      }
      const nextScopeID = accessScopeID.trim()
      if (!nextScopeID) {
        throw new Error("scope is required")
      }
      return createRoleAssignment(token, {
        tenant_id: tenantID,
        role_id: accessRoleID || defaultRoleForScope(roles, accessScope),
        applies_to_type: accessScope,
        applies_to_id: nextScopeID,
        assignee_type: "Team",
        assignee_id: selectedTeam.id,
        valid_until: validUntil.trim() || undefined,
      })
    },
    onSuccess: async () => {
      setAccessSheetOpen(false)
      setActionNotice("Access right assigned.")
      setActionError("")
      await refreshTeams()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access right assignment failed"),
  })

  return (
    <>
      <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Teams"] : ["Home", "Teams"]}
      title={selectedTeam?.name ?? "Teams"}
      count={resourceQuery.isPending ? "--" : teams.length}
      description="Use teams to group similar users and assign Access Rights in batches"
      actions={
        <>
          <Button
            disabled={!canMutate}
            onClick={openMemberSheet}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Member
          </Button>
          <Button
            variant="outline"
            disabled={!canMutate}
            onClick={openTeamSheet}
            className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
          >
            New Team
          </Button>
          <Button
            variant="outline"
            disabled={!canMutate || !selectedTeam || deleteTeamMutation.isPending}
            onClick={() => {
              setActionError("")
              setDeleteTeamConfirmOpen(true)
            }}
            className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
          >
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
                <th className="px-6 py-4 font-semibold">{t("kisi.teams.name")}</th>
                <th className="px-4 py-4 font-semibold">Scope</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.teams.members")}</th>
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
              {teams.length === 0 ? <MistyisletEmptyTableRow colSpan={5}>{t("kisi.teams.noMatch")}</MistyisletEmptyTableRow> : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={["General", "Members", "Access Rights", "Settings"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button
            disabled={!canMutate || !selectedTeam || updateTeamMutation.isPending || !teamName.trim()}
            onClick={() => updateTeamMutation.mutate()}
            className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            Save
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Team identity and ownership." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Team name</span>
                <input
                  value={teamName}
                  disabled={!selectedTeam}
                  onChange={(event) => setTeamName(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                />
              </label>
              <FormField label="Owner" value="Organization Admin" />
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Default place</span>
                <div className="relative">
                  <select
                    value={teamScope === "organization" ? "organization" : teamPlaceID}
                    disabled={!selectedTeam || placeScoped}
                    onChange={(event) => {
                      if (event.target.value === "organization") {
                        setTeamScope("organization")
                        setTeamPlaceID("")
                      } else {
                        setTeamScope("place")
                        setTeamPlaceID(event.target.value)
                      }
                    }}
                    className="h-11 w-full appearance-none rounded-[6px] border border-[#d9dbe3] bg-white px-3 pr-9 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                  >
                    {!placeScoped ? <option value="organization">Organization</option> : null}
                    {places.map((place) => (
                      <option key={place.id} value={place.id}>
                        {place.name}
                      </option>
                    ))}
                  </select>
                  <ChevronDownIcon className="pointer-events-none absolute right-3 top-3.5 size-4 text-[#6f717c]" />
                </div>
              </label>
              <FormField label="Directory source" value={selectedTeam?.sourceLabel ?? "Manual"} />
            </div>
            <label className="block border-t border-[#eceef2] px-7 py-5">
              <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Description</span>
              <textarea
                value={teamDescription}
                disabled={!selectedTeam}
                onChange={(event) => setTeamDescription(event.target.value)}
                rows={3}
                className="w-full rounded-[6px] border border-[#d9dbe3] px-3 py-2 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
              />
            </label>
          </>
        ) : null}

        {activeTab === "Members" ? (
          <>
            <PanelHeader
              title="Members"
              description="Users assigned directly or through directory sync."
              action={
                <Button
                  variant="outline"
                  disabled={!canMutate || !selectedTeam}
                  onClick={openMemberSheet}
                  className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                >
                  Add Members
                </Button>
              }
            />
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-left text-sm">
                <thead>
                  <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                    <th className="px-7 py-4 font-semibold">{t("kisi.teams.name")}</th>
                    <th className="px-4 py-4 font-semibold">Email</th>
                    <th className="px-4 py-4 font-semibold">Source</th>
                    <th className="px-4 py-4 font-semibold">Status</th>
                    <th className="px-7 py-4 text-right font-semibold">Actions</th>
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
                      <td className="px-7 py-5 text-right">
                        <RowActionsMenu
                          label={`Actions for ${membership.name}`}
                          items={[
                            {
                              id: "remove",
                              label: "Remove",
                              icon: Trash2Icon,
                              destructive: true,
                              disabled: !canMutate || deleteMembershipMutation.isPending,
                              onSelect: () => {
                                setActionError("")
                                setDeleteMembershipTarget(membership)
                              },
                            },
                          ]}
                        />
                      </td>
                    </tr>
                  ))}
                  {selectedMemberships.length === 0 ? (
                    <MistyisletEmptyTableRow colSpan={5}>No team members found.</MistyisletEmptyTableRow>
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
                <Button
                  variant="outline"
                  disabled={!canMutate || !selectedTeam}
                  onClick={openAccessSheet}
                  className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                >
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

      <ConfirmActionDialog
        open={deleteTeamConfirmOpen}
        onOpenChange={(open) => {
          if (!deleteTeamMutation.isPending) {
            setDeleteTeamConfirmOpen(open)
          }
        }}
        title="Delete team"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{selectedTeam?.name ?? "this team"}</span> and its team
            membership records from this workspace.
          </>
        }
        confirmLabel="Delete team"
        pending={deleteTeamMutation.isPending}
        disabled={!canMutate || !selectedTeam}
        destructive
        onConfirm={() => deleteTeamMutation.mutate()}
      />

      <ConfirmActionDialog
        open={Boolean(deleteMembershipTarget)}
        onOpenChange={(open) => {
          if (!deleteMembershipMutation.isPending && !open) {
            setDeleteMembershipTarget(null)
          }
        }}
        title="Remove team member"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{deleteMembershipTarget?.name ?? "this member"}</span>{" "}
            from {selectedTeam?.name ?? "this team"}.
          </>
        }
        confirmLabel="Remove member"
        pending={deleteMembershipMutation.isPending}
        disabled={!canMutate || !deleteMembershipTarget}
        destructive
        onConfirm={() => {
          if (deleteMembershipTarget) {
            deleteMembershipMutation.mutate(deleteMembershipTarget.id)
          }
        }}
      />

      <Sheet open={teamSheetOpen} onOpenChange={setTeamSheetOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>New Team</SheetTitle>
            <SheetDescription>Create a team for batch access assignments.</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createTeamMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.teams.name")}</span>
              <input
                value={teamName}
                onChange={(event) => setTeamName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Scope</span>
              <select
                value={teamScope === "organization" ? "organization" : teamPlaceID}
                onChange={(event) => {
                  if (event.target.value === "organization") {
                    setTeamScope("organization")
                    setTeamPlaceID("")
                  } else {
                    setTeamScope("place")
                    setTeamPlaceID(event.target.value)
                  }
                }}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                {!placeScoped ? <option value="organization">Organization</option> : null}
                {places.map((place) => (
                  <option key={place.id} value={place.id}>
                    {place.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Description</span>
              <textarea
                value={teamDescription}
                onChange={(event) => setTeamDescription(event.target.value)}
                rows={3}
                className="w-full rounded-[6px] border border-[#d9dbe3] px-3 py-2 text-sm text-[#2f3037]"
              />
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !teamName.trim() || createTeamMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createTeamMutation.isPending ? "Creating..." : "Create Team"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={memberSheetOpen} onOpenChange={setMemberSheetOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Add Member</SheetTitle>
            <SheetDescription>{selectedTeam?.name ?? "Selected team"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createMembershipMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Member type</span>
              <select
                value={memberType}
                onChange={(event) => {
                  const nextType = event.target.value as TeamMembership["member_type"]
                  setMemberType(nextType)
                  if (nextType === "User") {
                    updateMemberID(users[0]?.id ?? "")
                  } else {
                    setMemberID("")
                    setMemberEmail("")
                    setMemberName("")
                  }
                }}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="User">User</option>
                <option value="Guest">Guest</option>
              </select>
            </label>
            {memberType === "User" ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">User</span>
                <select
                  value={memberID}
                  onChange={(event) => updateMemberID(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                >
                  {users.map((user) => (
                    <option key={user.id} value={user.id}>
                      {user.name} · {user.email}
                    </option>
                  ))}
                </select>
              </label>
            ) : (
              <>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Guest ID or email</span>
                  <input
                    value={memberID}
                    onChange={(event) => {
                      setMemberID(event.target.value)
                      setMemberEmail(event.target.value)
                    }}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Guest name</span>
                  <input
                    value={memberName}
                    onChange={(event) => setMemberName(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
                  />
                </label>
              </>
            )}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !selectedTeam || !memberID.trim() || createMembershipMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createMembershipMutation.isPending ? "Adding..." : "Add Member"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={accessSheetOpen} onOpenChange={setAccessSheetOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[500px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Assign Access Right</SheetTitle>
            <SheetDescription>{selectedTeam?.name ?? "Selected team"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              assignAccessRightMutation.mutate()
            }}
          >
            <div className="grid grid-cols-3 gap-2 rounded-[6px] bg-[#f3f4f8] p-1">
              {accessScopeOptions.map((scope) => (
                <button
                  key={scope}
                  type="button"
                  onClick={() => updateAccessScope(scope)}
                  className={cn("h-9 rounded-[5px] text-sm font-semibold", accessScope === scope ? "bg-white text-[#17171c] shadow-sm" : "text-[#6f717c]")}
                >
                  {scope}
                </button>
              ))}
            </div>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Target</span>
              <select
                value={accessScopeID}
                onChange={(event) => setAccessScopeID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                {assignableScopes.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Role</span>
              <select
                value={accessRoleID}
                onChange={(event) => setAccessRoleID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                {accessRoleOptions.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid until</span>
              <input
                type="datetime-local"
                value={validUntil}
                onChange={(event) => setValidUntil(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !selectedTeam || !accessScopeID || !accessRoleID || assignAccessRightMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {assignAccessRightMutation.isPending ? "Assigning..." : "Assign Access Right"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}
