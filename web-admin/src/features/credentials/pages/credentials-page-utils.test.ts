import { describe, expect, it } from "vitest"

import type { WalletIssueJob } from "@/lib/api"

import {
  credentialBatchIssueJobsToCSV,
  filterCredentialBatchIssueJobs,
  formatCredentialBatchIssueNotice,
  summarizeCredentialBatchIssue,
  walletIssueJobErrorLabel,
  walletIssueJobStatusBucket,
  walletIssueJobStatusTone,
  walletIssueJobTargetLabel,
} from "./credentials-page-utils"

function issueJob(overrides: Partial<WalletIssueJob>): WalletIssueJob {
  return {
    id: "wjb_001",
    tenant_id: "tenant_demo_jakarta",
    provider: "google",
    batch_id: "wbt_001",
    template_id: "wpt_employee_demo",
    target_type: "user",
    target_id: "usr_rina",
    status: "success",
    retry_count: 0,
    created_at: "2026-04-27T00:00:00Z",
    updated_at: "2026-04-27T00:00:00Z",
    ...overrides,
  }
}

describe("credentials page batch audit helpers", () => {
  it("summarizes mixed batch issue results", () => {
    const summary = summarizeCredentialBatchIssue([
      issueJob({ status: "success" }),
      issueJob({ status: "pending" }),
      issueJob({ status: "failed" }),
      issueJob({ status: "dlq" }),
    ])

    expect(summary).toEqual({
      total: 4,
      succeeded: 1,
      queued: 1,
      failed: 2,
    })
  })

  it("formats a compact issue notice", () => {
    expect(
      formatCredentialBatchIssueNotice([
        issueJob({ status: "success" }),
        issueJob({ status: "pending" }),
        issueJob({ status: "failed" }),
      ])
    ).toBe("Batch issue completed: 1 issued, 1 queued, 1 failed.")
  })

  it("maps job status to display tone", () => {
    expect(walletIssueJobStatusBucket("success")).toBe("success")
    expect(walletIssueJobStatusBucket("failed")).toBe("failed")
    expect(walletIssueJobStatusBucket("dlq")).toBe("failed")
    expect(walletIssueJobStatusBucket("processing")).toBe("queued")

    expect(walletIssueJobStatusTone("success")).toBe("success")
    expect(walletIssueJobStatusTone("failed")).toBe("danger")
    expect(walletIssueJobStatusTone("dlq")).toBe("danger")
    expect(walletIssueJobStatusTone("processing")).toBe("warning")
    expect(walletIssueJobStatusTone("unknown")).toBe("warning")
  })

  it("resolves target and error labels", () => {
    const users = [{ id: "usr_rina", name: "Rina Hartono", email: "rina@example.com" }]

    expect(walletIssueJobTargetLabel(issueJob({ target_id: "usr_rina" }), users)).toBe(
      "Rina Hartono · rina@example.com"
    )
    expect(walletIssueJobTargetLabel(issueJob({ target_id: "usr_missing" }), users)).toBe("usr_missing")
    expect(walletIssueJobErrorLabel(issueJob({ error_code: "target_id_required" }))).toBe("target_id_required")
    expect(walletIssueJobErrorLabel(issueJob({ error_message: "target_id is required" }))).toBe(
      "target_id is required"
    )
  })

  it("filters jobs by status bucket and query", () => {
    const users = [{ id: "usr_rina", name: "Rina Hartono", email: "rina@example.com" }]
    const jobs = [
      issueJob({ id: "job_success", status: "success", target_id: "usr_rina" }),
      issueJob({ id: "job_pending", status: "pending", target_id: "usr_andi" }),
      issueJob({ id: "job_failed", status: "failed", target_id: "usr_budi", error_message: "Card printer unavailable" }),
    ]

    expect(filterCredentialBatchIssueJobs(jobs, users, { status: "queued" }).map((job) => job.id)).toEqual([
      "job_pending",
    ])
    expect(filterCredentialBatchIssueJobs(jobs, users, { query: "rina" }).map((job) => job.id)).toEqual([
      "job_success",
    ])
    expect(filterCredentialBatchIssueJobs(jobs, users, { query: "printer", status: "failed" }).map((job) => job.id)).toEqual([
      "job_failed",
    ])
  })

  it("exports filtered jobs as escaped csv", () => {
    const users = [{ id: "usr_rina", name: "Rina, Hartono", email: "rina@example.com" }]
    const csv = credentialBatchIssueJobsToCSV(
      [
        issueJob({
          id: "job_success",
          target_id: "usr_rina",
          pass_id: "wps_001",
          error_message: 'provider said "retry"',
        }),
      ],
      users
    )

    expect(csv.split("\n")[0]).toBe(
      "job_id,batch_id,target_type,target_id,target,status,pass_id,error,retry_count,created_at,updated_at"
    )
    expect(csv).toContain('"Rina, Hartono · rina@example.com"')
    expect(csv).toContain('"provider said ""retry"""')
  })
})
