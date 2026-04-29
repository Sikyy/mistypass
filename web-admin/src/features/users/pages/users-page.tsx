import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router-dom"
import { MailPlusIcon, PlusIcon, UsersIcon } from "lucide-react"

import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { EnabledCheck, InfoBanner, PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { selectMistyisletPlaceContext } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import {
  batchDeleteUsers,
  batchInviteUsers,
  batchUpdateUserStatus,
  createUser,
  exportUsersCSV,
  importUsersCSV,
  sendUserInvitation,
  type CurrentUser,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

type UserSheetMode = "add" | "invite"

function groupIDsFromText(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

export function UsersAdaptedPage({
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
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const placeContext = selectMistyisletPlaceContext(resourceQuery.summary, placeID)
  const rows = placeScoped ? placeContext.users : resourceQuery.summary.users
  const tenantID = getViewerTenantID(viewer)
  const canMutate = Boolean(tenantID && !resourceQuery.usingFallback && ["super_admin", "tenant_admin", "building_admin"].includes(viewer.role))

  const [query, setQuery] = useState("")
  const [selectedUsers, setSelectedUsers] = useState<Set<string>>(() => new Set())
  const [userSheetOpen, setUserSheetOpen] = useState(false)
  const [userSheetMode, setUserSheetMode] = useState<UserSheetMode>("add")
  const [newName, setNewName] = useState("")
  const [newEmail, setNewEmail] = useState("")
  const [newRole, setNewRole] = useState("employee")
  const [newStatus, setNewStatus] = useState("active")
  const [newBuildingID, setNewBuildingID] = useState("")
  const [newGroupIDsText, setNewGroupIDsText] = useState("")
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")

  const normalizedQuery = query.trim().toLowerCase()
  const filteredRows = normalizedQuery
    ? rows.filter((row) => [row.name, row.email, row.role, row.statusLabel, row.sourceLabel].join(" ").toLowerCase().includes(normalizedQuery))
    : rows
  const selectedRows = rows.filter((row) => selectedUsers.has(row.id))
  const selectedVisibleCount = filteredRows.filter((row) => selectedUsers.has(row.id)).length
  const allFilteredSelected = filteredRows.length > 0 && selectedVisibleCount === filteredRows.length

  function refreshUsers() {
    return queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
  }

  function openUserSheet(mode: UserSheetMode) {
    setUserSheetMode(mode)
    setNewName("")
    setNewEmail("")
    setNewRole("employee")
    setNewStatus(mode === "invite" ? "inactive" : "active")
    setNewBuildingID(placeScoped ? placeContext.place?.id ?? placeID ?? "" : "")
    setNewGroupIDsText("")
    setActionError("")
    setUserSheetOpen(true)
  }

  function toggleUser(userID: string) {
    setSelectedUsers((current) => {
      const next = new Set(current)
      if (next.has(userID)) {
        next.delete(userID)
      } else {
        next.add(userID)
      }
      return next
    })
  }

  function toggleAllFilteredUsers() {
    setSelectedUsers((current) => {
      const next = new Set(current)
      if (allFilteredSelected) {
        filteredRows.forEach((row) => next.delete(row.id))
      } else {
        filteredRows.forEach((row) => next.add(row.id))
      }
      return next
    })
  }

  const createUserMutation = useMutation({
    mutationFn: async () => {
      if (!tenantID) {
        throw new Error("tenant is required")
      }
      const created = await createUser(token, {
        tenant_id: tenantID,
        building_id: newBuildingID.trim(),
        name: newName.trim(),
        email: newEmail.trim(),
        role: newRole,
        status: newStatus,
        group_ids: groupIDsFromText(newGroupIDsText),
      })
      if (userSheetMode === "invite") {
        await sendUserInvitation(token, created.id, {
          tenant_id: tenantID,
          delivery_method: "email",
        })
      }
      return created
    },
    onSuccess: async () => {
      setUserSheetOpen(false)
      setActionNotice(userSheetMode === "invite" ? "User invitation queued." : "User created.")
      setActionError("")
      await refreshUsers()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "User create failed")
    },
  })

  const bulkStatusMutation = useMutation({
    mutationFn: async (nextStatus: "active" | "suspended") => {
      if (!tenantID) throw new Error("tenant is required")
      return batchUpdateUserStatus(token, {
        tenant_id: tenantID,
        user_ids: Array.from(selectedUsers),
        status: nextStatus,
      })
    },
    onSuccess: async (result) => {
      setSelectedUsers(new Set())
      setActionNotice(result.status === "active" ? `${result.updated} users enabled.` : `${result.updated} users suspended.`)
      setActionError("")
      await refreshUsers()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Bulk user update failed")
    },
  })

  const bulkDeleteMutation = useMutation({
    mutationFn: async () => {
      if (!tenantID) throw new Error("tenant is required")
      return batchDeleteUsers(token, {
        tenant_id: tenantID,
        user_ids: Array.from(selectedUsers),
      })
    },
    onSuccess: async (result) => {
      setSelectedUsers(new Set())
      setActionNotice(`${result.deleted} users deleted.`)
      setActionError("")
      await refreshUsers()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Bulk delete failed")
    },
  })

  const bulkInviteMutation = useMutation({
    mutationFn: async () => {
      if (!tenantID) throw new Error("tenant is required")
      return batchInviteUsers(token, {
        tenant_id: tenantID,
        user_ids: Array.from(selectedUsers),
        delivery_method: "email",
      })
    },
    onSuccess: async (result) => {
      setSelectedUsers(new Set())
      setActionNotice(`${result.queued} invitations queued.`)
      setActionError("")
      await refreshUsers()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Bulk invite failed")
    },
  })

  async function handleExportCSV() {
    try {
      const csv = await exportUsersCSV(token, tenantID)
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" })
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = "users.csv"
      a.click()
      URL.revokeObjectURL(url)
      setActionNotice("Users exported.")
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Export failed")
    }
  }

  async function handleImportCSV() {
    const input = document.createElement("input")
    input.type = "file"
    input.accept = ".csv"
    input.onchange = async () => {
      const file = input.files?.[0]
      if (!file || !tenantID) return
      try {
        const csvContent = await file.text()
        const result = await importUsersCSV(token, { tenant_id: tenantID, csv_content: csvContent })
        setActionNotice(`Import complete: ${result.created} created, ${result.updated} updated, ${result.errors} errors.`)
        setActionError("")
        await refreshUsers()
      } catch (error) {
        setActionError(error instanceof Error ? error.message : "Import failed")
      }
    }
    input.click()
  }

  return (
    <>
      <PageFrame
        breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Users"] : ["Home", "Users"]}
        title={placeScoped ? t("kisi.users.placeTitle") : t("kisi.users.title")}
        count={resourceQuery.isPending ? "--" : rows.length}
        description={placeScoped ? t("kisi.users.placeScopeNotice") : t("kisi.users.title")}
        actions={
          <Button
            type="button"
            disabled={!canMutate}
            onClick={() => openUserSheet("add")}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            {t("kisi.users.addUser")}
          </Button>
        }
      >
        {resourceQuery.usingFallback ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            Live user resources are unavailable. Showing reference data.
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

        {placeScoped ? (
          <InfoBanner>
            This is a list of users that have access to at least one door in this place. Organization users remain available from the global Users page.
          </InfoBanner>
        ) : null}

        <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
          <div className="flex items-center gap-3 border-b border-[#eceef2] p-5">
            <MistyisletSearchField value={query} onChange={setQuery} placeholder="Search Users..." className="h-11 border-[#8589ff]" />
            <Button
              type="button"
              variant="outline"
              disabled={!canMutate}
              onClick={() => openUserSheet("invite")}
              className="h-11 rounded-[6px] border-[#c9ccff] bg-white text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
            >
              <MailPlusIcon className="mr-1.5 size-4" />
              {t("kisi.users.invite")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={handleExportCSV}
              className="h-11 rounded-[6px] border-[#d9dbe3] bg-white text-[#2f3037] hover:bg-[#fbfbfc]"
            >
              {t("kisi.users.exportCsv")}
            </Button>
            {canMutate && ["super_admin", "tenant_admin"].includes(viewer.role) ? (
              <Button
                type="button"
                variant="outline"
                onClick={handleImportCSV}
                className="h-11 rounded-[6px] border-[#d9dbe3] bg-white text-[#2f3037] hover:bg-[#fbfbfc]"
              >
                {t("kisi.users.importCsv")}
              </Button>
            ) : null}
          </div>
          {selectedUsers.size > 0 ? (
            <div className="flex flex-wrap items-center gap-5 border-b border-[#eceef2] bg-white px-6 py-4 text-sm">
              <span className="font-semibold text-[#17171c]">{t("kisi.users.selected", { count: selectedUsers.size })}</span>
              <button
                type="button"
                disabled={!canMutate || bulkStatusMutation.isPending}
                onClick={() => bulkStatusMutation.mutate("suspended")}
                className="font-semibold text-[#4f55ff] disabled:text-[#9a9ca7]"
              >
                {t("kisi.users.suspendAccess")}
              </button>
              <button
                type="button"
                disabled={!canMutate || bulkStatusMutation.isPending}
                onClick={() => bulkStatusMutation.mutate("active")}
                className="font-semibold text-[#6f717c] disabled:text-[#9a9ca7]"
              >
                {t("kisi.users.enableAccess")}
              </button>
              <button
                type="button"
                disabled={!canMutate || bulkInviteMutation.isPending}
                onClick={() => bulkInviteMutation.mutate()}
                className="font-semibold text-[#4f55ff] disabled:text-[#9a9ca7]"
              >
                {t("kisi.users.sendInvite")}
              </button>
              {!placeScoped && ["super_admin", "tenant_admin"].includes(viewer.role) ? (
                <button
                  type="button"
                  disabled={!canMutate || bulkDeleteMutation.isPending}
                  onClick={() => { if (window.confirm(t("kisi.users.deleteConfirm", { count: selectedUsers.size }))) bulkDeleteMutation.mutate() }}
                  className="font-semibold text-[#d93025] disabled:text-[#9a9ca7]"
                >
                  Delete
                </button>
              ) : null}
              <button type="button" className="ml-auto text-[#6f717c]" onClick={() => setSelectedUsers(new Set())}>
                Clear
              </button>
            </div>
          ) : null}
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-left text-sm">
              <thead className="bg-white text-[#2f3037]">
                <tr className="border-b border-[#eceef2]">
                  <th className="w-12 px-6 py-4">
                    <button
                      type="button"
                      onClick={toggleAllFilteredUsers}
                      className={cn(
                        "block size-5 rounded-[3px] border",
                        allFilteredSelected ? "border-[#4f55ff] bg-[#4f55ff]" : "border-[#9a9ca7] bg-white"
                      )}
                      aria-label={allFilteredSelected ? "Clear selected users" : "Select all users"}
                    />
                  </th>
                  <th className="px-4 py-4 font-semibold">{t("common.name")}</th>
                  <th className="px-4 py-4 font-semibold">{t("common.email")}</th>
                  <th className="px-4 py-4 font-semibold">{placeScoped ? "Access Date" : "Role"}</th>
                  <th className="px-4 py-4 font-semibold">{t("kisi.users.accessEnabled")}</th>
                </tr>
              </thead>
              <tbody>
                {filteredRows.map((row, index) => (
                  <tr
                    key={row.id}
                    className={cn(
                      "border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]",
                      selectedUsers.has(row.id) ? "bg-[#e7e5df]" : placeScoped && index === 0 && "bg-[#f4f3ef]"
                    )}
                  >
                    <td className="px-6 py-4">
                      <button
                        type="button"
                        onClick={() => toggleUser(row.id)}
                        className={cn(
                          "block size-5 rounded-[3px] border",
                          selectedUsers.has(row.id) ? "border-[#4f55ff] bg-[#4f55ff]" : "border-[#9a9ca7] bg-white"
                        )}
                        aria-label={`Select ${row.name}`}
                      />
                    </td>
                    <td className="px-4 py-4 font-semibold">
                      {placeScoped ? (
                        <span className="text-[#17171c]">{row.name}</span>
                      ) : (
                        <Link to={`/users/${row.id}`} className="text-[#4f55ff] hover:underline">
                          {row.name}
                        </Link>
                      )}
                    </td>
                    <td className="px-4 py-4 text-[#6f717c]">{row.email}</td>
                    <td className="px-4 py-4 text-[#2f3037]">{placeScoped ? row.accessDateLabel : row.role}</td>
                    <td className="px-4 py-4">
                      {placeScoped && row.statusLabel === "Active" ? (
                        <EnabledCheck label="" />
                      ) : (
                        <StatusDot tone={row.tone} label={row.statusLabel} />
                      )}
                    </td>
                  </tr>
                ))}
                {filteredRows.length === 0 ? (
                  <MistyisletEmptyTableRow colSpan={5}>{t("kisi.users.noMatch")}</MistyisletEmptyTableRow>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>
      </PageFrame>

      <Sheet open={userSheetOpen} onOpenChange={setUserSheetOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{userSheetMode === "invite" ? "Invite User" : "Add User"}</SheetTitle>
            <SheetDescription>{placeScoped ? placeContext.place?.name ?? "Selected place" : "Organization directory"}</SheetDescription>
          </SheetHeader>
          <div className="space-y-5 px-6 py-6">
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
              <input
                value={newName}
                onChange={(event) => setNewName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.email")}</span>
              <input
                value={newEmail}
                onChange={(event) => setNewEmail(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <div className="grid gap-5 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.role")}</span>
                <select
                  value={newRole}
                  onChange={(event) => setNewRole(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
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
                  value={newStatus}
                  onChange={(event) => setNewStatus(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                >
                  <option value="active">{t("kisi.users.activeStatus")}</option>
                  <option value="inactive">{t("kisi.users.inactiveStatus")}</option>
                  <option value="suspended">{t("kisi.users.suspendedStatus")}</option>
                </select>
              </label>
            </div>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.users.placeId")}</span>
              <input
                value={newBuildingID}
                disabled={placeScoped}
                onChange={(event) => setNewBuildingID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.users.groupIds")}</span>
              <input
                value={newGroupIDsText}
                onChange={(event) => setNewGroupIDsText(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
          </div>
          <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => setUserSheetOpen(false)}
              className="rounded-[6px]"
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={!canMutate || createUserMutation.isPending || !newName.trim() || !newEmail.trim()}
              onClick={() => createUserMutation.mutate()}
              className="rounded-[6px] bg-[#4f55ff] text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
            >
              {createUserMutation.isPending ? "Saving..." : userSheetMode === "invite" ? "Create Invite" : "Create User"}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
