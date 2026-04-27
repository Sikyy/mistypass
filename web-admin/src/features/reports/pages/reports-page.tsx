import { useState } from "react"

import { KisiEmptyTableRow, KisiFilterButton, KisiSearchField } from "@/components/kisi/data-display"
import { PageFrame, StatusDot } from "@/components/kisi/primitives"
import { cn } from "@/lib/utils"

export function ReportsAdaptedPage({ placeScoped = false }: { placeScoped?: boolean }) {
  const rows = [
    ["Open", "Garage door forced open", "High", "12:02 PM"],
    ["Investigating", "Reader offline at 11th Floor", "Medium", "10:44 AM"],
    ["Resolved", "Credential retry burst", "Low", "Yesterday"],
  ]
  const tabs = placeScoped ? ["Access", "Doors", "Users"] : ["Alerts", "Audit Log", "Analytics"]
  const [activeTab, setActiveTab] = useState(tabs[0])
  const [query, setQuery] = useState("")
  const visibleRows = rows.filter((row) => query.trim() === "" || row.join(" ").toLowerCase().includes(query.trim().toLowerCase()))

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", "Assigned Place", "Analytics"] : ["Home", "Reports"]}
      title={placeScoped ? "Analytics" : "Reports"}
      description={placeScoped ? "Place statistics, trends, and audit slices" : "Organization alerts, audit log, and statistics"}
    >
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
        <div className="flex flex-col gap-3 border-b border-[#eceef2] bg-[#fbfbfc] p-5 md:flex-row md:items-center">
          <KisiFilterButton label="Today" className="font-normal md:w-40" />
          <KisiFilterButton label="All Types" className="font-normal md:w-44" />
          <KisiSearchField value={query} onChange={setQuery} placeholder={`Search ${activeTab.toLowerCase()}...`} />
        </div>
        {activeTab === "Analytics" || activeTab === "Access" ? (
          <div className="grid gap-4 border-b border-[#eceef2] p-6 md:grid-cols-3">
            {["Unlocks", "Denials", "Peak hour"].map((item, index) => (
              <div key={item} className="rounded-[6px] border border-[#eceef2] p-4">
                <p className="text-sm text-[#6f717c]">{item}</p>
                <p className="mt-3 text-3xl font-bold text-[#17171c]">{index === 0 ? "428" : index === 1 ? "12" : "8 AM"}</p>
              </div>
            ))}
          </div>
        ) : null}
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">Status</th>
                <th className="px-4 py-4 font-semibold">Item</th>
                <th className="px-4 py-4 font-semibold">Severity</th>
                <th className="px-4 py-4 font-semibold">Time</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row, index) => (
                <tr key={row[1]} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-5">
                    <StatusDot tone={index === 0 ? "danger" : index === 1 ? "warning" : "success"} label={row[0]} />
                  </td>
                  <td className="px-4 py-5 font-semibold text-[#17171c]">{row[1]}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{row[2]}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{row[3]}</td>
                </tr>
              ))}
              {visibleRows.length === 0 ? (
                <KisiEmptyTableRow colSpan={4}>No report items match this search.</KisiEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  )
}
