import { useState } from "react"
import { PlusIcon } from "lucide-react"

import { KisiEmptyTableRow, KisiFilterButton, KisiSearchField, KisiTablePagination } from "@/components/kisi/data-display"
import { PageFrame, StatusDot } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { selectKisiPlaceContext } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

type AccessRightRow = {
  id: string
  name: string
  type: string
  target: string
  rule: string
  status: string
  tone: "success" | "warning" | "danger" | "info"
}

export function AccessRightsAdaptedPage({
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
  const tabs = ["Groups", "Places", "Teams", "Users", "Access Links"]
  const [activeTab, setActiveTab] = useState("Places")
  const [query, setQuery] = useState("")
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const placeContext = selectKisiPlaceContext(resourceQuery.summary, placeID)
  const accessRights = placeScoped ? placeContext.accessRights : resourceQuery.summary.accessRights
  const accessRightRows = (subjectType: AccessRightRow["type"]) =>
    accessRights
      .filter((item) => item.subjectType === subjectType)
      .map((item) => ({
        id: item.id,
        name: item.name,
        type: item.subjectType,
        target: item.targetLabel,
        rule: item.ruleLabel,
        status: item.statusLabel,
        tone: item.tone,
      }))
  const rowsByTab: Record<string, AccessRightRow[]> = {
    Groups: accessRightRows("Group"),
    Places: accessRightRows("Place"),
    Teams: accessRightRows("Team"),
    Users: accessRightRows("User"),
    "Access Links": accessRightRows("Access Link"),
  }
  const rows = rowsByTab[activeTab].filter(
    (row) => query.trim() === "" || Object.values(row).join(" ").toLowerCase().includes(query.trim().toLowerCase())
  )

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Access Rights"] : ["Home", "Access Rights"]}
      title="Access Rights"
      count={resourceQuery.isPending ? "--" : accessRights.length}
      description="Share access by connecting users, teams, groups, places, and schedules"
      actions={
        <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
          <PlusIcon className="mr-1.5 size-4" />
          Share Access
        </Button>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live access-right resources are unavailable. Showing reference data.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="flex gap-8 overflow-x-auto border-b border-[#eceef2] px-6">
          {tabs.map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={cn(
                "whitespace-nowrap py-5 text-base font-semibold",
                activeTab === tab ? "border-b-2 border-[#4f55ff] text-[#4f55ff]" : "text-[#2f3037]"
              )}
            >
              {tab}
            </button>
          ))}
        </div>
        <div className="flex flex-col gap-4 border-b border-[#eceef2] p-6 md:flex-row md:items-center">
          <KisiSearchField value={query} onChange={setQuery} placeholder={`Search ${activeTab.toLowerCase()}...`} className="h-12 bg-transparent" />
          <KisiFilterButton label="Scope" className="h-12 md:w-44" />
          <KisiFilterButton label="Schedule" className="h-12 md:w-44" />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[840px] text-left text-sm">
            <thead className="bg-[#fbfbfc]">
              <tr className="border-b border-[#eceef2]">
                <th className="w-16 px-6 py-4">
                  <span className="block size-5 rounded-[3px] border border-[#9a9ca7]" />
                </th>
                <th className="px-4 py-4 font-semibold">Name</th>
                <th className="px-4 py-4 font-semibold">Type</th>
                <th className="px-4 py-4 font-semibold">Target</th>
                <th className="px-4 py-4 font-semibold">Rule</th>
                <th className="px-4 py-4 font-semibold">Status</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${activeTab}-${row.id}`} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-5">
                    <span className="block size-5 rounded-[3px] border border-[#d9dbe3] bg-white" />
                  </td>
                  <td className="px-4 py-5 font-semibold text-[#4f55ff]">{row.name}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{row.type}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{row.target}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{row.rule}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={row.tone} label={row.status} />
                  </td>
                </tr>
              ))}
              {rows.length === 0 ? (
                <KisiEmptyTableRow colSpan={6}>No access rights match this search.</KisiEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
        <KisiTablePagination />
      </section>
    </PageFrame>
  )
}
