import { request, requestItems, requestText, encodePathSegment } from "./core"

export type ReportMetric = {
  key: string
  label: string
  value: number | string | boolean | null
  unit?: string
}

export type Report = {
  id: string
  resource_type: "Report"
  tenant_id: string
  name: string
  description?: string
  category: "access" | "doors" | "audit" | string
  report_type: string
  status: "ready" | "running" | "failed" | string
  severity?: "info" | "warning" | "danger" | string
  place_id?: string
  period_start: string
  period_end: string
  generated_at: string
  format: "csv" | "json" | string
  download_url: string
  metrics: ReportMetric[]
}

export type ScheduledReport = {
  id: string
  resource_type: "ScheduledReport"
  tenant_id: string
  report_id: string
  name: string
  status: "active" | "paused" | string
  cadence: "daily" | "weekly" | "monthly" | string
  format: "csv" | "json" | string
  place_id?: string
  recipients?: string[]
  last_run_at?: string
  next_run_at?: string
  created_at: string
  updated_at: string
}

function withReportQuery(
  path: string,
  options?: {
    tenant_id?: string
    place_id?: string
    category?: string
    query?: string
    report_id?: string
    status?: string
  }
): string {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.category?.trim()) query.set("category", options.category.trim())
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.report_id?.trim()) query.set("report_id", options.report_id.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  const suffix = query.toString()
  return suffix ? `${path}?${suffix}` : path
}

export async function listReports(
  token: string | undefined,
  options?: {
    tenant_id?: string
    place_id?: string
    category?: string
    query?: string
  }
): Promise<Report[]> {
  return requestItems<Report>(withReportQuery("/api/v1/reports", options), token)
}

export async function getReport(
  token: string | undefined,
  reportID: string,
  options?: {
    tenant_id?: string
    place_id?: string
  }
): Promise<Report> {
  return request<Report>(
    withReportQuery(`/api/v1/reports/${encodePathSegment(reportID)}`, options),
    { method: "GET" },
    token
  )
}

export async function downloadReportCSV(
  token: string | undefined,
  reportID: string,
  options?: {
    tenant_id?: string
    place_id?: string
  }
): Promise<string> {
  return requestText(withReportQuery(`/api/v1/reports/${encodePathSegment(reportID)}/download`, options), token)
}

export async function listScheduledReports(
  token: string | undefined,
  options?: {
    tenant_id?: string
    place_id?: string
    report_id?: string
    status?: string
  }
): Promise<ScheduledReport[]> {
  return requestItems<ScheduledReport>(withReportQuery("/api/v1/scheduled_reports", options), token)
}

