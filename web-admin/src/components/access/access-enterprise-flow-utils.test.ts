import { describe, expect, it, vi } from "vitest"

function interpolate(defaultValue: string, options?: Record<string, unknown>) {
  return defaultValue.replace(/\{\{\s*(\w+)\s*\}\}/g, (_, key: string) => String(options?.[key] ?? ""))
}

vi.mock("@/lib/i18n", () => ({
  default: {
    t: (key: string, options?: Record<string, unknown> & { defaultValue?: string }) =>
      typeof options?.defaultValue === "string" ? interpolate(options.defaultValue, options) : key,
  },
}))

import {
  buildEnterpriseStageSearch,
  buildEnterpriseSyncWorkerReviewLink,
  findHintedGroupMember,
  hasWorkerAlertFlowHints,
  parseEnterpriseFlowContext,
} from "./access-enterprise-flow-utils"

function expectSearchParams(search: string, expected: Record<string, string>) {
  const params = new URLSearchParams(search.startsWith("?") ? search.slice(1) : search)
  expect(Object.fromEntries(params.entries())).toEqual(expected)
}

describe("access enterprise flow helpers", () => {
  it("parses enterprise-only query context", () => {
    expect(parseEnterpriseFlowContext("?stage=directory&tenant_id=tenant_a")).toBeNull()
    expect(
      parseEnterpriseFlowContext("?from=enterprise&stage=policies&tenant_id=tenant_a&worker_action=receipt_retry")
    ).toMatchObject({
      stage: "policies",
      tenantID: "tenant_a",
      workerAction: "receipt_retry",
    })
  })

  it("builds stage search by preserving base query and applying hint overrides", () => {
    const context = parseEnterpriseFlowContext("?from=enterprise&flow=sync_to_access&tenant_id=tenant_a")

    expectSearchParams(
      buildEnterpriseStageSearch({
        baseSearch: "?lang=id&worker_action=old",
        context,
        hints: {
          worker_action: "receipt_retry",
          worker_query_hint: "",
        },
        selectedTenantID: "tenant_b",
        stage: "issuance",
      }),
      {
        flow: "sync_to_access",
        from: "enterprise",
        lang: "id",
        stage: "issuance",
        tenant_id: "tenant_b",
        worker_action: "receipt_retry",
      }
    )
  })

  it("finds hinted employee and derives worker review link state", () => {
    const context = parseEnterpriseFlowContext(
      "?from=enterprise&flow=sync_to_access&tenant_id=tenant_a&group_member_email=user@example.com&worker_alert_level=hot&worker_action=receipt_retry&worker_kind=receipt_worker"
    )

    expect(
      findHintedGroupMember(context, [
        {
          email: "user@example.com",
          full_name: "User Example",
          id: "emp_1",
        },
      ] as never)
    ).toMatchObject({
      id: "emp_1",
    })
    expect(hasWorkerAlertFlowHints(context)).toBe(true)
    const link = buildEnterpriseSyncWorkerReviewLink({
      activeSection: "policies",
      baseSearch: "?lang=en",
      context,
      selectedTenantID: "tenant_b",
    })
    const [pathWithQuery, hash] = link.split("#")

    expect(pathWithQuery.startsWith("/enterprise?")).toBe(true)
    expect(hash).toBe("sync")
    expectSearchParams(pathWithQuery.slice("/enterprise".length), {
      flow: "sync_to_access",
      from: "enterprise",
      lang: "en",
      sync_focus_hint: "worker_alert",
      tenant_id: "tenant_b",
      worker_action: "receipt_retry",
      worker_alert_level: "hot",
      worker_filter_hint: "hot",
      worker_kind: "receipt_worker",
      worker_review_stage_hint: "policies",
      worker_review_status_hint: "handled",
    })
  })
})
