import { useState } from "react"
import { Link } from "react-router-dom"
import { PlusIcon, UsersIcon } from "lucide-react"

import { KisiEmptyTableRow, KisiSearchField } from "@/components/kisi/data-display"
import { EnabledCheck, InfoBanner, PageFrame, StatusDot } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { selectKisiPlaceContext } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

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
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const placeContext = selectKisiPlaceContext(resourceQuery.summary, placeID)
  const rows = placeScoped ? placeContext.users : resourceQuery.summary.users
  const [query, setQuery] = useState("")
  const [selectedUsers, setSelectedUsers] = useState<Set<string>>(() => new Set())
  const normalizedQuery = query.trim().toLowerCase()
  const filteredRows = normalizedQuery
    ? rows.filter((row) => [row.name, row.email, row.role, row.statusLabel, row.sourceLabel].join(" ").toLowerCase().includes(normalizedQuery))
    : rows
  const selectedCount = filteredRows.filter((row) => selectedUsers.has(row.email)).length
  const allFilteredSelected = filteredRows.length > 0 && selectedCount === filteredRows.length

  function toggleUser(email: string) {
    setSelectedUsers((current) => {
      const next = new Set(current)
      if (next.has(email)) {
        next.delete(email)
      } else {
        next.add(email)
      }
      return next
    })
  }

  function toggleAllFilteredUsers() {
    setSelectedUsers((current) => {
      const next = new Set(current)
      if (allFilteredSelected) {
        filteredRows.forEach((row) => next.delete(row.email))
      } else {
        filteredRows.forEach((row) => next.add(row.email))
      }
      return next
    })
  }

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Users"] : ["Home", "Users"]}
      title={placeScoped ? "Place Users" : "Users"}
      count={resourceQuery.isPending ? "--" : rows.length}
      description={placeScoped ? "Manage users associated with this place" : "Manage users across the organization"}
      actions={
        <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
          <PlusIcon className="mr-1.5 size-4" />
          Add User
        </Button>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live user resources are unavailable. Showing reference data.
        </div>
      ) : null}

      {placeScoped ? (
        <InfoBanner>
          This is a list of users that have access to at least one door in this place. Organization users remain available from the global Users page.
        </InfoBanner>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="flex items-center gap-3 border-b border-[#eceef2] p-5">
          <KisiSearchField value={query} onChange={setQuery} placeholder="Search Users..." className="h-11 border-[#8589ff]" />
          <Button variant="outline" className="h-11 rounded-[6px] border-[#c9ccff] bg-white text-[#4f55ff] hover:bg-[#fbfbfc]">
            <UsersIcon className="mr-1.5 size-4" />
            Invite
          </Button>
        </div>
        {selectedUsers.size > 0 ? (
          <div className="flex flex-wrap items-center gap-5 border-b border-[#eceef2] bg-white px-6 py-4 text-sm">
            <span className="font-semibold text-[#17171c]">{selectedUsers.size} users selected</span>
            <button type="button" className="font-semibold text-[#4f55ff]">
              Suspend Access
            </button>
            <button type="button" className="font-semibold text-[#6f717c]">
              Enable Access
            </button>
            {placeScoped ? (
              <button type="button" className="font-semibold text-[#4f55ff]">
                Remove From Place
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
                <th className="px-4 py-4 font-semibold">Name</th>
                <th className="px-4 py-4 font-semibold">Email</th>
                <th className="px-4 py-4 font-semibold">{placeScoped ? "Access Date" : "Role"}</th>
                <th className="px-4 py-4 font-semibold">Access Enabled</th>
              </tr>
            </thead>
            <tbody>
              {filteredRows.map((row, index) => (
                <tr
                  key={row.id}
                  className={cn(
                    "border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]",
                    selectedUsers.has(row.email) ? "bg-[#e7e5df]" : placeScoped && index === 0 && "bg-[#f4f3ef]"
                  )}
                >
                  <td className="px-6 py-4">
                    <button
                      type="button"
                      onClick={() => toggleUser(row.email)}
                      className={cn(
                        "block size-5 rounded-[3px] border",
                        selectedUsers.has(row.email) ? "border-[#4f55ff] bg-[#4f55ff]" : "border-[#9a9ca7] bg-white"
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
                <KisiEmptyTableRow colSpan={5}>No users match this search.</KisiEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  )
}
