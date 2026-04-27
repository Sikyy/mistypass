import { useState } from "react"
import { PlusIcon } from "lucide-react"

import { KisiEmptyTableRow, KisiFilterButton, KisiSearchField } from "@/components/kisi/data-display"
import { PageFrame, StatusDot } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

export function CredentialsAdaptedPage({
  token,
  viewer,
}: {
  token: string
  viewer: CurrentUser
}) {
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const rows = resourceQuery.summary.credentials
  const [activeTab, setActiveTab] = useState("Active")
  const [query, setQuery] = useState("")
  const visibleRows = rows.filter((row) => {
    const matchesTab = row.statusLabel === activeTab
    const matchesQuery =
      query.trim() === "" ||
      [row.user, row.type, row.statusLabel, row.issuedLabel, row.expiresLabel].join(" ").toLowerCase().includes(query.trim().toLowerCase())
    return matchesTab && matchesQuery
  })
  const tabs = ["Active", "Pending", "Suspended", "Revoked"]

  return (
    <PageFrame
      breadcrumbs={["Home", "Credentials"]}
      title="Credentials"
      count={resourceQuery.isPending ? "--" : rows.length}
      description="Issue and monitor access credentials"
      actions={
        <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
          <PlusIcon className="mr-1.5 size-4" />
          Issue Credential
        </Button>
      }
    >
      {resourceQuery.summary.partial ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Some live credential resources are unavailable.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="flex gap-10 border-b border-[#eceef2] px-6">
          {tabs.map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={cn("py-5 text-base font-semibold", activeTab === tab ? "border-b-2 border-[#4f55ff] text-[#4f55ff]" : "text-[#2f3037]")}
            >
              {tab}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-3 border-b border-[#eceef2] bg-[#fbfbfc] p-5">
          <KisiSearchField value={query} onChange={setQuery} placeholder="Search credentials..." />
          <KisiFilterButton label="Type" className="gap-2" />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">User</th>
                <th className="px-4 py-4 font-semibold">Type</th>
                <th className="px-4 py-4 font-semibold">Status</th>
                <th className="px-4 py-4 font-semibold">Issued</th>
                <th className="px-4 py-4 font-semibold">Expires</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row) => (
                <tr key={row.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-5 font-semibold text-[#17171c]">{row.user}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{row.type}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={row.tone} label={row.statusLabel} />
                  </td>
                  <td className="px-4 py-5 text-[#6f717c]">{row.issuedLabel}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{row.expiresLabel}</td>
                </tr>
              ))}
              {visibleRows.length === 0 ? (
                <KisiEmptyTableRow colSpan={5}>No {activeTab.toLowerCase()} credentials match this view.</KisiEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  )
}
