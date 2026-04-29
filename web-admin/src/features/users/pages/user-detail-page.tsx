import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, useNavigate, useParams } from "react-router-dom"
import {
  CameraIcon,
  CreditCardIcon,
  KeyRoundIcon,
  MailPlusIcon,
  ShieldCheckIcon,
  Trash2Icon,
  UsersIcon,
} from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import {
  InfoBanner,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  StatusDot,
  ToggleSwitch,
  type ActivityTone,
} from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import {
  deleteUser,
  fetchUser,
  listUserInvitations,
  sendUserInvitation,
  updateUser,
  type CurrentUser,
  type UserInvitationDelivery,
} from "@/lib/api"
import { getViewerTenantID } from "@/lib/viewer"

type UserDetailAdaptedPageProps = {
  token: string
  viewer: CurrentUser
}

function userStatusTone(status: string): ActivityTone {
  switch (status.trim().toLowerCase()) {
    case "active":
      return "success"
    case "suspended":
      return "warning"
    default:
      return "danger"
  }
}

function userStatusLabel(status: string) {
  const normalized = status.trim().toLowerCase()
  if (!normalized) {
    return "Unknown"
  }
  return normalized.charAt(0).toUpperCase() + normalized.slice(1)
}

function groupIDsFromText(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

function formatInvitationTime(value?: string) {
  if (!value) {
    return "Pending"
  }
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return "Pending"
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(parsed)
}

function invitationDeliveryDetail(delivery: UserInvitationDelivery) {
  const parts = [delivery.provider || delivery.delivery_method]
  if (delivery.provider_delivery_id) {
    parts.push(delivery.provider_delivery_id)
  }
  if (delivery.provider_error) {
    parts.push(delivery.provider_error)
  }
  if (delivery.retryable) {
    parts.push("retryable")
  }
  return parts.filter(Boolean).join(" / ")
}

export function UserDetailAdaptedPage({ token, viewer }: UserDetailAdaptedPageProps) {
  const { t } = useTranslation()
  const { userID = "" } = useParams<{ userID?: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const tenantID = getViewerTenantID(viewer)
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const [activeTab, setActiveTab] = useState("General")
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [role, setRole] = useState("employee")
  const [status, setStatus] = useState("active")
  const [buildingID, setBuildingID] = useState("")
  const [groupIDsText, setGroupIDsText] = useState("")
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

  const userQuery = useQuery({
    queryKey: ["access-user", userID, tenantID],
    enabled: Boolean(token && userID),
    queryFn: () => fetchUser(token, userID, tenantID || undefined),
  })
  const invitationQuery = useQuery({
    queryKey: ["access-user-invitations", userID, tenantID],
    enabled: Boolean(token && userID),
    queryFn: () => listUserInvitations(token, userID, tenantID || undefined),
  })
  const user = userQuery.data
  const invitations = invitationQuery.data ?? []
  const canMutate = Boolean(user && ["super_admin", "tenant_admin", "building_admin"].includes(viewer.role))
  const canDelete = Boolean(user && ["super_admin", "tenant_admin"].includes(viewer.role))

  useEffect(() => {
    if (!user) {
      return
    }
    setName(user.name)
    setEmail(user.email)
    setRole(user.role || "employee")
    setStatus(user.status || "active")
    setBuildingID(user.building_id ?? "")
    setGroupIDsText((user.group_ids ?? []).join(", "))
  }, [user])

  async function refreshUser() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["access-user", userID] }),
      queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] }),
    ])
  }

  function buildUpdatePayload(nextStatus?: string) {
    return {
      tenant_id: tenantID || undefined,
      building_id: buildingID.trim(),
      name: name.trim(),
      email: email.trim(),
      role: role.trim() || "employee",
      status: nextStatus ?? status,
      group_ids: groupIDsFromText(groupIDsText),
    }
  }

  const updateUserMutation = useMutation({
    mutationFn: ({ nextStatus }: { nextStatus?: string } = {}) => {
      if (!user) {
        throw new Error("user is required")
      }
      return updateUser(token, user.id, buildUpdatePayload(nextStatus))
    },
    onSuccess: async (updated) => {
      setActionNotice(t("common.save"))
      setActionError("")
      setName(updated.name)
      setEmail(updated.email)
      setRole(updated.role || "employee")
      setStatus(updated.status || "active")
      setBuildingID(updated.building_id ?? "")
      setGroupIDsText((updated.group_ids ?? []).join(", "))
      await refreshUser()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "User save failed")
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: () => {
      if (!user) {
        throw new Error("user is required")
      }
      return deleteUser(token, user.id, tenantID || undefined)
    },
    onSuccess: async () => {
      setActionError("")
      setDeleteConfirmOpen(false)
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
      navigate("/users", { replace: true })
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "User delete failed")
    },
  })

  const inviteUserMutation = useMutation({
    mutationFn: () => {
      if (!user) {
        throw new Error("user is required")
      }
      return sendUserInvitation(token, user.id, {
        tenant_id: tenantID || undefined,
        delivery_method: "email",
      })
    },
    onSuccess: () => {
      setActionNotice(t("kisi.users.inviteQueued"))
      setActionError("")
      void queryClient.invalidateQueries({ queryKey: ["access-user-invitations", userID] })
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "User invitation failed")
    },
  })

  const accessRows = user?.group_ids?.length
    ? user.group_ids.map((groupID) => [groupID, "Group", user.building_id || "Organization", "Always"])
    : []
  const eventRows = [
    ["Apr 26, 12:02 PM", user?.building_id || "Organization", "Unlock successful", "Mobile credential"],
    ["Apr 25, 05:44 PM", user?.building_id || "Organization", "Access denied", "Outside schedule"],
    ["Apr 24, 09:11 AM", "Organization", "Credential issued", "Organization Admin"],
  ]
  const suspended = status.trim().toLowerCase() === "suspended"
  const saveDisabled = !canMutate || updateUserMutation.isPending || !name.trim() || !email.trim()
  const title = user?.name ?? (userQuery.isPending ? "Loading user..." : "User not found")

  return (
    <>
      <PageFrame
        breadcrumbs={[t("common.home"), "Users"]}
        title={title}
        description={t("kisi.myAccount.description")}
        actions={
          <>
            <Button
              asChild
              variant="interaction"
              className="h-10 rounded-[6px] text-[#4f55ff]"
            >
              <Link to="/access-rights">{t("common.shareAccess")}</Link>
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!canMutate || inviteUserMutation.isPending}
              onClick={() => inviteUserMutation.mutate()}
              className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
            >
              <MailPlusIcon className="mr-1.5 size-4" />
              {inviteUserMutation.isPending ? "Sending..." : "Resend Invite"}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!canDelete || deleteUserMutation.isPending}
              onClick={() => setDeleteConfirmOpen(true)}
              className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
            >
              <Trash2Icon className="mr-1.5 size-4" />
              Delete User
            </Button>
          </>
        }
      >
        {userQuery.isError ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            {userQuery.error instanceof Error ? userQuery.error.message : "User detail failed to load."}
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
          User membership, credentials, and access history stay together on this page. Groups control door restrictions; Teams control bulk access-right assignment.
        </InfoBanner>

        <SettingsPanel
          tabs={["General", "Access Rights", "Credentials", "Logins", "Analytics", "Events"]}
          active={activeTab}
          onTabChange={setActiveTab}
          footer={
            activeTab === "General" ? (
              <Button
                disabled={saveDisabled}
                onClick={() => updateUserMutation.mutate({})}
                className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
              >
                {updateUserMutation.isPending ? "Saving..." : "Save"}
              </Button>
            ) : (
              <div className="grid w-full grid-cols-3 text-sm text-[#6f717c]">
                <span>{t("common.previousPage")}</span>
                <span className="text-center text-[#17171c]">{t("common.pageOf", { page: 1, total: 1 })}</span>
                <span className="text-right">{t("common.nextPage")}</span>
              </div>
            )
          }
        >
          {activeTab === "General" ? (
            <>
              <PanelHeader title={t("common.general")} description={t("kisi.myAccount.description")} />
              <div className="grid gap-8 p-7 lg:grid-cols-[1fr_260px]">
                <div className="space-y-6">
                  <div className="grid gap-6 md:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
                      <input
                        value={name}
                        disabled={!user || !canMutate}
                        onChange={(event) => setName(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.email")}</span>
                      <input
                        value={email}
                        disabled={!user || !canMutate}
                        onChange={(event) => setEmail(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                      />
                    </label>
                  </div>
                  <div className="grid gap-6 md:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.role")}</span>
                      <select
                        value={role}
                        disabled={!user || !canMutate}
                        onChange={(event) => setRole(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                      >
                        <option value="employee">{t("kisi.users.memberRole")}</option>
                        <option value="operator">{t("kisi.users.operatorRole")}</option>
                        <option value="manager">{t("kisi.users.managerRole")}</option>
                        <option value="building_admin">{t("kisi.users.placeAdminRole")}</option>
                      </select>
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.status")}</span>
                      <select
                        value={status}
                        disabled={!user || !canMutate}
                        onChange={(event) => setStatus(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                      >
                        <option value="active">{t("kisi.users.activeStatus")}</option>
                        <option value="suspended">{t("kisi.users.suspendedStatus")}</option>
                        <option value="inactive">{t("kisi.users.inactiveStatus")}</option>
                      </select>
                    </label>
                  </div>
                  <div className="grid gap-6 md:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.users.placeId")}</span>
                      <input
                        value={buildingID}
                        disabled={!user || !canMutate}
                        onChange={(event) => setBuildingID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.users.groupIds")}</span>
                      <input
                        value={groupIDsText}
                        disabled={!user || !canMutate}
                        onChange={(event) => setGroupIDsText(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                      />
                    </label>
                  </div>
                  <div className="flex gap-5 rounded-[6px] border border-[#eceef2] p-5">
                    <div className="flex size-10 shrink-0 items-center justify-center rounded-[6px] bg-[#f1f2f5]">
                      <UsersIcon className="size-5 text-[#2f3037]" />
                    </div>
                    <div className="max-w-3xl">
                      <p className="font-semibold text-[#17171c]">{t("kisi.users.suspendAccess")}</p>
                      <p className="mt-1 text-sm leading-6 text-[#6f717c]">
                        Prevent this user from unlocking doors while keeping their account and audit history intact.
                      </p>
                    </div>
                    <div className="ml-auto flex items-center gap-3">
                      <ToggleSwitch enabled={suspended} />
                      <Button
                        type="button"
                        variant="outline"
                        disabled={!canMutate || updateUserMutation.isPending}
                        onClick={() => updateUserMutation.mutate({ nextStatus: suspended ? "active" : "suspended" })}
                        className="h-10 rounded-[6px]"
                      >
                        {suspended ? "Enable" : "Suspend"}
                      </Button>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-7 lg:justify-center">
                  <div className="flex size-28 items-center justify-center rounded-[10px] bg-[#eef0f4]">
                    <CameraIcon className="size-12 text-[#17171c]" />
                  </div>
                  <button type="button" className="text-base font-semibold text-[#4f55ff]">
                    Add Photo
                  </button>
                </div>
              </div>
            </>
          ) : null}

          {activeTab === "Access Rights" ? (
            <>
              <PanelHeader
                title={t("kisi.teams.accessRights")}
                description={t("kisi.accessRights.pageDesc")}
                action={
                  <Button asChild variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]">
                    <Link to="/access-rights">{t("common.shareAccess")}</Link>
                  </Button>
                }
              />
              <div className="overflow-x-auto">
                <table className="w-full min-w-[760px] text-left text-sm">
                  <thead>
                    <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                      <th className="px-7 py-4 font-semibold">{t("common.name")}</th>
                      <th className="px-4 py-4 font-semibold">{t("common.type")}</th>
                      <th className="px-4 py-4 font-semibold">{t("common.scope")}</th>
                      <th className="px-4 py-4 font-semibold">{t("kisi.accessRights.schedule")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {accessRows.length > 0 ? (
                      accessRows.map((row) => (
                        <tr key={row[0]} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                          <td className="px-7 py-5 font-semibold text-[#4f55ff]">{row[0]}</td>
                          <td className="px-4 py-5 text-[#2f3037]">{row[1]}</td>
                          <td className="px-4 py-5 text-[#6f717c]">{row[2]}</td>
                          <td className="px-4 py-5 text-[#6f717c]">{row[3]}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={4} className="px-7 py-8 text-center text-sm text-[#6f717c]">
                          No direct groups assigned.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
              {userQuery.data && (
                <div className="border-t border-[#eceef2]">
                  <div className="px-7 py-5">
                    <h3 className="text-sm font-semibold text-[#17171c]">Accessible Doors</h3>
                    <p className="mt-1 text-xs text-[#6f717c]">Doors this user can access based on group membership and role assignments.</p>
                  </div>
                  <div className="px-7 pb-5">
                    {(() => {
                      const userGroupIDs = userQuery.data.group_ids ?? []
                      const doors = resourceQuery.summary.doors
                      const hardware = resourceQuery.summary.hardware
                      const accessibleDoors = doors.filter((door) => {
                        // Check building-level group access
                        const userGroups = resourceQuery.summary.groups?.filter((g) =>
                          userGroupIDs.includes(g.id) && g.placeId === door.placeId
                        ) ?? []
                        return userGroups.length > 0
                      })
                      if (accessibleDoors.length === 0) {
                        return <p className="text-sm text-[#6f717c]">No accessible doors found for this user.</p>
                      }
                      return (
                        <div className="flex flex-wrap gap-2">
                          {accessibleDoors.map((door) => {
                            const gw = hardware.find((h) => (h.type === "Controller" || h.type === "Gateway") && h.doorNames.includes(door.name))
                            return (
                              <div key={door.id} className="flex items-center gap-2 rounded-[6px] border border-[#eceef2] px-3 py-2 text-sm">
                                <StatusDot tone={gw ? gw.tone : "warning"} label="" />
                                <Link to={`/places/${door.placeId}/doors`} className="font-medium text-[#4f55ff] hover:underline">
                                  {door.name}
                                </Link>
                                <span className="text-[#6f717c]">{door.floorName}</span>
                              </div>
                            )
                          })}
                        </div>
                      )
                    })()}
                  </div>
                </div>
              )}
            </>
          ) : null}

          {activeTab === "Credentials" ? (
            <>
              <PanelHeader
                title={t("kisi.credentials.title")}
                description={t("kisi.credentials.title")}
                action={
                  <Button asChild variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]">
                    <Link to="/credentials">{t("kisi.credentials.issue")}</Link>
                  </Button>
                }
              />
              <div className="divide-y divide-[#eceef2]">
                {[
                  ["Mobile credential", userStatusLabel(status), user?.email ?? "No email"],
                  ["Access link", "Pending", "Managed from Access Rights"],
                  ["Card credential", "Unassigned", "Issue from Credentials"],
                ].map((row, index) => (
                  <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                    <span className="font-semibold text-[#17171c]">{row[0]}</span>
                    <StatusDot tone={index === 0 ? userStatusTone(status) : index === 1 ? "warning" : "info"} label={row[1]} />
                    <span className="text-sm text-[#6f717c]">{row[2]}</span>
                  </div>
                ))}
              </div>
            </>
          ) : null}

          {activeTab === "Logins" ? (
            <>
              <PanelHeader title={t("kisi.myAccount.logins")} description={t("kisi.myAccount.description")} />
              <div className="divide-y divide-[#eceef2]">
                {[
                  { label: "Email", detail: user?.email ?? "No email", state: userStatusLabel(status), Icon: KeyRoundIcon },
                  { label: "SSO", detail: "Organization managed", state: "Available", Icon: ShieldCheckIcon },
                  { label: "Credentials", detail: "Mobile and card credentials", state: "Managed", Icon: CreditCardIcon },
                ].map(({ label, detail, state, Icon }) => (
                  <div key={label} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_220px_1fr] md:items-center">
                    <span className="flex items-center gap-2 font-semibold text-[#17171c]">
                      <Icon className="size-4 text-[#6f717c]" />
                      {label}
                    </span>
                    <StatusDot tone={state === "Available" || state === "Managed" ? "success" : userStatusTone(status)} label={state} />
                    <span className="text-sm text-[#6f717c]">{detail}</span>
                  </div>
                ))}
              </div>
              <div className="border-t border-[#eceef2]">
                <div className="px-7 py-5">
                  <h3 className="text-sm font-semibold text-[#17171c]">{t("kisi.users.invite")}</h3>
                </div>
                <div className="divide-y divide-[#eceef2]">
                  {invitations.length > 0 ? (
                    invitations.slice(0, 5).map((delivery) => (
                      <div key={delivery.id} className="grid gap-3 px-7 py-5 text-sm md:grid-cols-[180px_150px_1fr] md:items-center">
                        <span className="font-semibold text-[#17171c]">{formatInvitationTime(delivery.queued_at)}</span>
                        <StatusDot tone={delivery.status === "failed" ? "danger" : delivery.status === "sent" ? "success" : "warning"} label={userStatusLabel(delivery.status)} />
                        <span className="text-[#6f717c]">
                          {invitationDeliveryDetail(delivery)}
                        </span>
                      </div>
                    ))
                  ) : (
                    <div className="px-7 py-6 text-sm text-[#6f717c]">
                      No invitation deliveries yet.
                    </div>
                  )}
                </div>
              </div>
            </>
          ) : null}

          {activeTab === "Analytics" ? (
            <>
              <PanelHeader title={t("kisi.reports.title")} description={t("kisi.myAccount.description")} />
              <div className="grid gap-4 p-7 md:grid-cols-3">
                {[
                  ["Assigned groups", String(groupIDsFromText(groupIDsText).length), buildingID || "No place"],
                  ["Account status", userStatusLabel(status), role || "employee"],
                  ["Credential owner", user?.id ?? "--", user?.tenant_id ?? tenantID],
                ].map((item) => (
                  <div key={item[0]} className="rounded-[6px] border border-[#eceef2] p-5">
                    <p className="text-sm text-[#6f717c]">{item[0]}</p>
                    <p className="mt-3 text-2xl font-bold text-[#17171c]">{item[1]}</p>
                    <p className="mt-2 text-sm text-[#6f717c]">{item[2]}</p>
                  </div>
                ))}
              </div>
            </>
          ) : null}

          {activeTab === "Events" ? (
            <>
              <PanelHeader title={t("common.events")} description={t("kisi.myAccount.description")} />
              <div className="divide-y divide-[#eceef2]">
                {eventRows.map((row, index) => (
                  <div key={`${row[0]}-${row[1]}`} className="grid gap-3 px-7 py-5 text-sm md:grid-cols-[170px_180px_180px_1fr] md:items-center">
                    <span className="text-[#6f717c]">{row[0]}</span>
                    <span className="font-semibold text-[#17171c]">{row[1]}</span>
                    <StatusDot tone={index === 1 ? "warning" : "success"} label={row[2]} />
                    <span className="text-[#6f717c]">{row[3]}</span>
                  </div>
                ))}
              </div>
            </>
          ) : null}
        </SettingsPanel>
      </PageFrame>

      <ConfirmActionDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={t("common.delete")}
        description={user ? `Delete ${user.name} and remove their direct group, team, and role assignment references.` : "Delete this user."}
        confirmLabel="Delete User"
        destructive
        pending={deleteUserMutation.isPending}
        disabled={!canDelete}
        onConfirm={() => deleteUserMutation.mutate()}
      />
    </>
  )
}
