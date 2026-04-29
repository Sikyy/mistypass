import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { DownloadIcon } from "lucide-react"

import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  downloadReportCSV,
  listReports,
  listScheduledReports,
  type CurrentUser,
  type Report,
  type ScheduledReport,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

type ReportsAdaptedPageProps = {
  token: string
  viewer: CurrentUser
  placeID?: string
  placeScoped?: boolean
}

function reportCategoryForTab(tab: string) {
  if (tab === "Doors") return "doors"
  if (tab === "Audit") return "audit"
  return "access"
}

function statusTone(value: string | undefined): "success" | "warning" | "danger" | "info" {
  const normalized = (value ?? "").trim().toLowerCase()
  if (normalized === "danger" || normalized === "failed" || normalized === "paused") return "danger"
  if (normalized === "warning" || normalized === "running") return "warning"
  if (normalized === "ready" || normalized === "active") return "success"
  return "info"
}

function formatDateTime(value?: string) {
  if (!value) return "Not set"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date)
}

function metricSummary(report: Report) {
  return report.metrics
    .slice(0, 3)
    .map((metric) => `${metric.label}: ${metric.value}${metric.unit ? ` ${metric.unit}` : ""}`)
    .join(" | ")
}

function downloadFile(filename: string, content: string) {
  const blob = new Blob([content], { type: "text/csv;charset=utf-8" })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

export function ReportsAdaptedPage({ token, viewer, placeID, placeScoped = false }: ReportsAdaptedPageProps) {
  const tabs = placeScoped ? ["Access", "Doors"] : ["Access", "Doors", "Audit"]
  const [activeTab, setActiveTab] = useState(tabs[0])
  const [query, setQuery] = useState("")
  const [actionError, setActionError] = useState("")
  const [downloadingReportID, setDownloadingReportID] = useState("")
  const tenantID = getViewerTenantID(viewer)
  const activeCategory = reportCategoryForTab(activeTab)
  const scopedPlaceID = placeScoped ? placeID : undefined
  const reportsQuery = useQuery({
    queryKey: ["reference-reports", tenantID, scopedPlaceID, activeCategory, query],
    queryFn: () =>
      listReports(token, {
        tenant_id: tenantID,
        place_id: scopedPlaceID,
        category: activeCategory,
        query,
      }),
    enabled: Boolean(tenantID),
    staleTime: 30 * 1000,
  })
  const scheduledReportsQuery = useQuery({
    queryKey: ["reference-scheduled-reports", tenantID, scopedPlaceID],
    queryFn: () =>
      listScheduledReports(token, {
        tenant_id: tenantID,
        place_id: scopedPlaceID,
      }),
    enabled: Boolean(tenantID),
    staleTime: 30 * 1000,
  })
  const reports = reportsQuery.data ?? []
  const scheduledReports = scheduledReportsQuery.data ?? []

  async function handleDownload(report: Report) {
    setActionError("")
    setDownloadingReportID(report.id)
    try {
      const csv = await downloadReportCSV(token, report.id, {
        tenant_id: tenantID,
        place_id: scopedPlaceID,
      })
      downloadFile(`${report.id}.csv`, csv)
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Report download failed")
    } finally {
      setDownloadingReportID("")
    }
  }

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", "Assigned Place", "Analytics"] : ["Home", "Reports"]}
      title={placeScoped ? "Analytics" : "Reports"}
      count={reportsQuery.isPending ? "--" : reports.length}
      description={placeScoped ? "Place statistics, trends, and audit slices" : "Organization alerts, audit log, and statistics"}
    >
      {actionError ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          {actionError}
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
        <div className="border-b border-[#eceef2] bg-[#fbfbfc] p-5">
          <MistyisletSearchField value={query} onChange={setQuery} placeholder={`Search ${activeTab.toLowerCase()} reports...`} />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[860px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">Report</th>
                <th className="px-4 py-4 font-semibold">Metrics</th>
                <th className="px-4 py-4 font-semibold">Status</th>
                <th className="px-4 py-4 font-semibold">Generated</th>
                <th className="px-4 py-4 text-right font-semibold">Export</th>
              </tr>
            </thead>
            <tbody>
              {reports.map((report) => (
                <tr key={report.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-5">
                    <p className="font-semibold text-[#17171c]">{report.name}</p>
                    <p className="mt-1 text-xs text-[#6f717c]">{report.description}</p>
                  </td>
                  <td className="max-w-[360px] px-4 py-5 text-[#2f3037]">{metricSummary(report)}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={statusTone(report.severity ?? report.status)} label={report.status} />
                  </td>
                  <td className="px-4 py-5 text-[#6f717c]">{formatDateTime(report.generated_at)}</td>
                  <td className="px-4 py-5 text-right">
                    <Button
                      type="button"
                      disabled={downloadingReportID === report.id}
                      onClick={() => void handleDownload(report)}
                      className="h-9 rounded-[6px] border border-[#c8cad6] bg-white px-3 text-[#2f3037] hover:bg-[#f3f4f8]"
                    >
                      <DownloadIcon className="mr-1.5 size-4" />
                      {downloadingReportID === report.id ? "Exporting..." : "CSV"}
                    </Button>
                  </td>
                </tr>
              ))}
              {reportsQuery.isPending ? <MistyisletEmptyTableRow colSpan={5}>Loading reports...</MistyisletEmptyTableRow> : null}
              {!reportsQuery.isPending && reports.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={5}>No reports match this view.</MistyisletEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-base font-semibold text-[#17171c]">Scheduled Reports</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">Name</th>
                <th className="px-4 py-4 font-semibold">Cadence</th>
                <th className="px-4 py-4 font-semibold">Recipients</th>
                <th className="px-4 py-4 font-semibold">Status</th>
                <th className="px-4 py-4 font-semibold">Next Run</th>
              </tr>
            </thead>
            <tbody>
              {scheduledReports.map((item: ScheduledReport) => (
                <tr key={item.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-5 font-semibold text-[#17171c]">{item.name}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{item.cadence}</td>
                  <td className="max-w-[300px] px-4 py-5 text-[#6f717c]">{item.recipients?.join(", ") || "Not set"}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={statusTone(item.status)} label={item.status} />
                  </td>
                  <td className="px-4 py-5 text-[#6f717c]">{formatDateTime(item.next_run_at)}</td>
                </tr>
              ))}
              {scheduledReportsQuery.isPending ? <MistyisletEmptyTableRow colSpan={5}>Loading scheduled reports...</MistyisletEmptyTableRow> : null}
              {!scheduledReportsQuery.isPending && scheduledReports.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={5}>No scheduled reports are configured.</MistyisletEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  )
}
