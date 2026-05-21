import i18next from "i18next"
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
      setActionNotice(t("kisi.teams.created"))
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
      setActionNotice(t("kisi.teams.saved"))
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
      setActionNotice(t("kisi.teams.deleted"))
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
      setActionNotice(t("kisi.teams.memberAdded"))
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
      setActionNotice(t("kisi.teams.memberRemoved"))
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
      setActionNotice(t("kisi.teams.assignAR"))
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
      description={t("kisi.teams.pageDesc")}
      actions={
        <>
          <Button
            disabled={!canMutate}
            onClick={openMemberSheet}
            className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Member
          </Button>
          <Button
            variant="outline"
            disabled={!canMutate}
            onClick={openTeamSheet}
            className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
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
            className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
          >
            Delete Team
          </Button>
        </>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="mp-alert-warning">
          Live team resources are unavailable. Showing reference data.
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

      <InfoBanner>
        {t("kisi.teams.pageDesc")}
      </InfoBanner>

      <section className="overflow-hidden rounded-[6px] border border-line-default bg-white">
        <div className="border-b border-line-subtle px-6 py-5">
          <h2 className="text-base font-semibold text-content-heading">{t("kisi.teams.title")}</h2>
          <p className="mt-1 text-sm text-content-subtle">{t("kisi.teams.pageDesc")}</p>
        </div>
        <div className="overflow-x-auto">
          <table className={cn("mp-table", "min-w-[780px]")}>
            <thead className="bg-surface-page">
              <tr className="border-b border-line-subtle">
                <th className="px-6 py-4 font-semibold">{t("kisi.teams.name")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.scope")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.teams.members")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.teams.accessRights")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.status")}</th>
              </tr>
            </thead>
            <tbody>
              {teams.map((team) => (
                <tr
                  key={team.id}
                  className={cn("mp-table-row", selectedTeam?.id === team.id && "bg-[#f7f7ff]")}
                >
                  <td className="px-6 py-5">
                    <button type="button" onClick={() => setSelectedTeamID(team.id)} className="font-semibold text-brand">
                      {team.name}
                    </button>
                  </td>
                  <td className="px-4 py-5 text-content-body">{team.scopeLabel}</td>
                  <td className="px-4 py-5 text-content-subtle">{team.memberCount}</td>
                  <td className="px-4 py-5 text-content-subtle">{team.accessRightCount}</td>
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
        tabs={[t("kisi.teams.tabGeneral"), t("kisi.teams.tabMembers"), t("kisi.teams.tabAccessRights"), t("kisi.teams.tabSettings")]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button
            disabled={!canMutate || !selectedTeam || updateTeamMutation.isPending || !teamName.trim()}
            onClick={() => updateTeamMutation.mutate()}
            className="h-10 rounded-[8px] bg-brand px-8 text-white hover:bg-brand-hover disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            Save
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title={t("common.general")} description={t("kisi.teams.generalDesc")} />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("kisi.teams.teamName")}</span>
                <input
                  value={teamName}
                  disabled={!selectedTeam}
                  onChange={(event) => setTeamName(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-sunken"
                />
              </label>
              <FormField label={t("kisi.teams.owner")} value="Organization Admin" />
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("kisi.teams.defaultPlace")}</span>
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
                    className="h-11 w-full appearance-none rounded-[6px] border border-line-default bg-white px-3 pr-9 text-sm text-content-body disabled:bg-surface-sunken"
                  >
                    {!placeScoped ? <option value="organization">{t("common.organization")}</option> : null}
                    {places.map((place) => (
                      <option key={place.id} value={place.id}>
                        {place.name}
                      </option>
                    ))}
                  </select>
                  <ChevronDownIcon className="pointer-events-none absolute right-3 top-3.5 size-4 text-content-subtle" />
                </div>
              </label>
              <FormField label="Directory source" value={selectedTeam?.sourceLabel ?? "Manual"} />
            </div>
            <label className="block border-t border-line-subtle px-7 py-5">
              <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.description")}</span>
              <textarea
                value={teamDescription}
                disabled={!selectedTeam}
                onChange={(event) => setTeamDescription(event.target.value)}
                rows={3}
                className="w-full rounded-[6px] border border-line-default px-3 py-2 text-sm text-content-body disabled:bg-surface-sunken"
              />
            </label>
          </>
        ) : null}

        {activeTab === "Members" ? (
          <>
            <PanelHeader
              title={t("common.members")}
              description={t("kisi.teams.membersDesc")}
              action={
                <Button
                  variant="outline"
                  disabled={!canMutate || !selectedTeam}
                  onClick={openMemberSheet}
                  className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
                >
                  Add Members
                </Button>
              }
            />
            <div className="overflow-x-auto">
              <table className={cn("mp-table", "min-w-[760px]")}>
                <thead>
                  <tr className="border-b border-line-subtle bg-surface-page text-content-body">
                    <th className="px-7 py-4 font-semibold">{t("kisi.teams.name")}</th>
                    <th className="px-4 py-4 font-semibold">{t("common.email")}</th>
                    <th className="px-4 py-4 font-semibold">{t("kisi.teams.source")}</th>
                    <th className="px-4 py-4 font-semibold">{t("common.status")}</th>
                    <th className="px-7 py-4 text-right font-semibold">{t("common.actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {selectedMemberships.map((membership) => (
                    <tr key={membership.id} className="mp-table-row">
                      <td className="px-7 py-5 font-semibold text-brand">{membership.name}</td>
                      <td className="px-4 py-5 text-content-subtle">{membership.email}</td>
                      <td className="px-4 py-5 text-content-body">{membership.sourceLabel}</td>
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
                    <MistyisletEmptyTableRow colSpan={5}>{t("kisi.teams.noMembers")}</MistyisletEmptyTableRow>
                  ) : null}
                </tbody>
              </table>
            </div>
          </>
        ) : null}

        {activeTab === "Access Rights" ? (
          <>
            <PanelHeader
              title={t("kisi.teams.accessRights")}
              description={t("kisi.teams.arDesc")}
              action={
                <Button
                  variant="outline"
                  disabled={!canMutate || !selectedTeam}
                  onClick={openAccessSheet}
                  className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
                >
                  Assign Access Right
                </Button>
              }
            />
            <div className="divide-y divide-line-subtle">
              {selectedAccessRights.map((accessRight) => (
                <div key={accessRight.id} className="grid gap-3 px-7 py-5 md:grid-cols-[210px_170px_1fr_170px] md:items-center">
                  <span className="font-semibold text-content-heading">{accessRight.ruleLabel}</span>
                  <span className="text-sm text-content-body">{accessRight.targetLabel}</span>
                  <span className="text-sm text-content-subtle">{accessRight.subjectType}</span>
                  <StatusDot tone={accessRight.tone} label={accessRight.statusLabel} />
                </div>
              ))}
              {selectedAccessRights.length === 0 ? (
                <div className="px-7 py-10 text-sm text-content-subtle">{t("kisi.teams.noAR")}</div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Settings" ? (
          <>
            <PanelHeader title={t("common.settings")} description={t("kisi.teams.settingsDesc")} />
            <SettingToggleRows
              rows={[
                ["Sync from SCIM", true, "Maintain team membership from the connected directory.", ShieldCheckIcon],
                [i18next.t("kisi.teams.autoShare"), true, i18next.t("kisi.teams.settingsDesc"), KeyRoundIcon],
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
        title={t("kisi.teams.deleteTeam")}
        description={
          <>
            This removes <span className="font-semibold text-content-heading">{selectedTeam?.name ?? "this team"}</span> and its team
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
        title={t("kisi.teams.removeTeamMember")}
        description={
          <>
            This removes <span className="font-semibold text-content-heading">{deleteMembershipTarget?.name ?? t("common.members")}</span>{" "}
            from {selectedTeam?.name ?? "this team"}.
          </>
        }
        confirmLabel={t("kisi.teams.removeMember")}
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
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{t("kisi.teams.newTeam")}</SheetTitle>
            <SheetDescription>{t("kisi.teams.createDesc")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createTeamMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.teams.name")}</span>
              <input
                value={teamName}
                onChange={(event) => setTeamName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.scope")}</span>
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
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                {!placeScoped ? <option value="organization">{t("common.organization")}</option> : null}
                {places.map((place) => (
                  <option key={place.id} value={place.id}>
                    {place.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.description")}</span>
              <textarea
                value={teamDescription}
                onChange={(event) => setTeamDescription(event.target.value)}
                rows={3}
                className="w-full rounded-[6px] border border-line-default px-3 py-2 text-sm text-content-body"
              />
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !teamName.trim() || createTeamMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
              >
                {createTeamMutation.isPending ? "Creating..." : "Create Team"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={memberSheetOpen} onOpenChange={setMemberSheetOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{t("kisi.teams.addMemberSheet")}</SheetTitle>
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.teams.memberType")}</span>
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
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                <option value="User">{t("common.user")}</option>
                <option value="Guest">{t("common.guest")}</option>
              </select>
            </label>
            {memberType === "User" ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.user")}</span>
                <select
                  value={memberID}
                  onChange={(event) => updateMemberID(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
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
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.teams.guestId")}</span>
                  <input
                    value={memberID}
                    onChange={(event) => {
                      setMemberID(event.target.value)
                      setMemberEmail(event.target.value)
                    }}
                    className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.teams.guestName")}</span>
                  <input
                    value={memberName}
                    onChange={(event) => setMemberName(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
                  />
                </label>
              </>
            )}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !selectedTeam || !memberID.trim() || createMembershipMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
              >
                {createMembershipMutation.isPending ? "Adding..." : "Add Member"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={accessSheetOpen} onOpenChange={setAccessSheetOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[500px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{t("kisi.teams.assignAR")}</SheetTitle>
            <SheetDescription>{selectedTeam?.name ?? "Selected team"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              assignAccessRightMutation.mutate()
            }}
          >
            <div className="grid grid-cols-3 gap-2 rounded-[6px] bg-surface-sunken p-1">
              {accessScopeOptions.map((scope) => (
                <button
                  key={scope}
                  type="button"
                  onClick={() => updateAccessScope(scope)}
                  className={cn("h-9 rounded-[5px] text-sm font-semibold", accessScope === scope ? "bg-white text-content-heading shadow-sm" : "text-content-subtle")}
                >
                  {scope}
                </button>
              ))}
            </div>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.target")}</span>
              <select
                value={accessScopeID}
                onChange={(event) => setAccessScopeID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                {assignableScopes.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.role")}</span>
              <select
                value={accessRoleID}
                onChange={(event) => setAccessRoleID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                {accessRoleOptions.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.teams.validUntil")}</span>
              <input
                type="datetime-local"
                value={validUntil}
                onChange={(event) => setValidUntil(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !selectedTeam || !accessScopeID || !accessRoleID || assignAccessRightMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
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
