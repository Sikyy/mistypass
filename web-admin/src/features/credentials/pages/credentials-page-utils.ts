import type { ActivityTone } from "@/components/mistyislet/primitives"
import type { WalletIssueJob, WalletPhysicalCardInventoryItem } from "@/lib/api"

type CredentialBatchAuditUser = {
  id: string
  name?: string
  email?: string
}

export type CredentialBatchIssueSummary = {
  total: number
  succeeded: number
  failed: number
  queued: number
}

export type CredentialBatchJobStatusFilter = "all" | "success" | "queued" | "failed"
export type PhysicalCardInventoryStatusFilter = "all" | WalletPhysicalCardInventoryItem["status"]

export function summarizeCredentialBatchIssue(items: WalletIssueJob[]): CredentialBatchIssueSummary {
  return items.reduce(
    (summary, item) => {
      const status = walletIssueJobStatusBucket(item.status)
      summary.total += 1
      if (status === "success") {
        summary.succeeded += 1
      } else if (status === "failed") {
        summary.failed += 1
      } else {
        summary.queued += 1
      }
      return summary
    },
    { total: 0, succeeded: 0, failed: 0, queued: 0 }
  )
}

export function formatCredentialBatchIssueNotice(items: WalletIssueJob[]) {
  const summary = summarizeCredentialBatchIssue(items)
  const parts = [
    `${summary.succeeded} issued`,
    summary.queued ? `${summary.queued} queued` : "",
    summary.failed ? `${summary.failed} failed` : "",
  ].filter(Boolean)

  if (parts.length === 0) {
    return "Batch issue completed with no jobs."
  }

  return `Batch issue completed: ${parts.join(", ")}.`
}

export function walletIssueJobStatusBucket(status: string): Exclude<CredentialBatchJobStatusFilter, "all"> {
  const normalized = status.trim().toLowerCase()
  if (normalized === "success") {
    return "success"
  }
  if (normalized === "failed" || normalized === "dlq") {
    return "failed"
  }
  return "queued"
}

export function walletIssueJobStatusTone(status: string): ActivityTone {
  const bucket = walletIssueJobStatusBucket(status)
  if (bucket === "success") {
    return "success"
  }
  if (bucket === "failed") {
    return "danger"
  }
  return "warning"
}

export function walletIssueJobTargetLabel(job: WalletIssueJob, users: CredentialBatchAuditUser[]) {
  const user = users.find((item) => item.id === job.target_id)
  if (user?.name && user.email) {
    return `${user.name} · ${user.email}`
  }
  if (user?.name || user?.email) {
    return user.name || user.email || job.target_id || "None"
  }
  return job.target_id || "None"
}

export function walletIssueJobErrorLabel(job: WalletIssueJob) {
  return job.error_message || job.error_code || "None"
}

export function filterCredentialBatchIssueJobs(
  jobs: WalletIssueJob[],
  users: CredentialBatchAuditUser[],
  options: {
    query?: string
    status?: CredentialBatchJobStatusFilter
  }
) {
  const query = options.query?.trim().toLowerCase() ?? ""
  const status = options.status ?? "all"

  return jobs.filter((job) => {
    if (status !== "all" && walletIssueJobStatusBucket(job.status) !== status) {
      return false
    }
    if (!query) {
      return true
    }

    const haystack = [
      job.id,
      job.batch_id,
      job.template_id,
      job.target_type,
      job.target_id,
      walletIssueJobTargetLabel(job, users),
      job.pass_id ?? "",
      job.status,
      job.error_code ?? "",
      job.error_message ?? "",
    ]
      .join(" ")
      .toLowerCase()

    return haystack.includes(query)
  })
}

function escapeCSVCell(value: string | number | undefined) {
  const text = String(value ?? "")
  if (/[",\n\r]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }
  return text
}

export function credentialBatchIssueJobsToCSV(jobs: WalletIssueJob[], users: CredentialBatchAuditUser[]) {
  const header = [
    "job_id",
    "batch_id",
    "target_type",
    "target_id",
    "target",
    "status",
    "pass_id",
    "error",
    "retry_count",
    "created_at",
    "updated_at",
  ]

  const rows = jobs.map((job) => [
    job.id,
    job.batch_id,
    job.target_type,
    job.target_id,
    walletIssueJobTargetLabel(job, users),
    job.status,
    job.pass_id ?? "",
    walletIssueJobErrorLabel(job),
    job.retry_count,
    job.created_at,
    job.updated_at,
  ])

  return [header, ...rows].map((row) => row.map(escapeCSVCell).join(",")).join("\n")
}

export function filterPhysicalCardInventory(
  items: WalletPhysicalCardInventoryItem[],
  options: {
    query?: string
    status?: PhysicalCardInventoryStatusFilter
    vendorID?: string
  }
) {
  const query = options.query?.trim().toLowerCase() ?? ""
  const status = options.status ?? "all"
  const vendorID = options.vendorID ?? "all"

  return items.filter((item) => {
    if (status !== "all" && item.status !== status) {
      return false
    }
    if (vendorID !== "all" && (item.vendor_id || "") !== vendorID) {
      return false
    }
    if (!query) {
      return true
    }

    return [
      item.id,
      item.card_number,
      item.uid ?? "",
      item.vendor_id ?? "",
      item.vendor_name ?? "",
      item.source ?? "",
      item.reader_id ?? "",
      item.status,
      item.assigned_pass_id ?? "",
      item.active_task_id ?? "",
    ]
      .join(" ")
      .toLowerCase()
      .includes(query)
  })
}

export function physicalCardInventoryToCSV(items: WalletPhysicalCardInventoryItem[]) {
  const header = [
    "inventory_id",
    "card_number",
    "uid",
    "status",
    "vendor_id",
    "vendor_name",
    "source",
    "reader_id",
    "assigned_pass_id",
    "active_task_id",
    "scanned_at",
    "created_at",
    "updated_at",
  ]

  const rows = items.map((item) => [
    item.id,
    item.card_number,
    item.uid ?? "",
    item.status,
    item.vendor_id ?? "",
    item.vendor_name ?? "",
    item.source ?? "",
    item.reader_id ?? "",
    item.assigned_pass_id ?? "",
    item.active_task_id ?? "",
    item.scanned_at ?? "",
    item.created_at,
    item.updated_at,
  ])

  return [header, ...rows].map((row) => row.map(escapeCSVCell).join(",")).join("\n")
}
