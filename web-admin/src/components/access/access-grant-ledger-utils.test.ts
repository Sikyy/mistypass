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

import { buildTenantGrantViewModel } from "./access-grant-ledger-utils"

describe("access grant ledger view model", () => {
  it("filters grants by tenant and derives lifecycle counts", () => {
    const nowTick = Date.parse("2026-04-24T00:00:00Z")
    const grants = [
      {
        authorized_at: "2026-04-23T10:00:00Z",
        created_at: "2026-04-23T09:00:00Z",
        delivery_method: "wallet",
        pass_type: "employee_temp",
        tenant_id: "tenant_a",
        valid_until: "2026-04-26T00:00:00Z",
      },
      {
        authorized_at: "2026-04-24T01:00:00Z",
        created_at: "2026-04-24T01:00:00Z",
        delivery_method: "email_qr",
        pass_type: "visitor",
        tenant_id: "tenant_a",
        valid_until: "2026-04-24T12:00:00Z",
      },
      {
        authorized_at: "2026-04-22T01:00:00Z",
        created_at: "2026-04-22T01:00:00Z",
        delivery_method: "wallet",
        pass_type: "visitor",
        tenant_id: "tenant_a",
        valid_until: "2026-04-23T12:00:00Z",
      },
      {
        authorized_at: "2026-04-24T01:00:00Z",
        created_at: "2026-04-24T01:00:00Z",
        delivery_method: "wallet",
        pass_type: "visitor",
        tenant_id: "tenant_b",
        valid_until: "2026-04-25T12:00:00Z",
      },
    ]

    const result = buildTenantGrantViewModel({
      grantDateFrom: "",
      grantDateTo: "",
      grantMethodFilter: "all",
      grantPassTypeFilter: "all",
      grantStatusFilter: "all",
      grants: grants as never,
      nowTick,
      selectedTenantID: "tenant_a",
    })

    expect(result.tenantGrants).toHaveLength(3)
    expect(result.activeGrantCount).toBe(1)
    expect(result.expiringSoonGrantCount).toBe(1)
    expect(result.expiredGrantCount).toBe(1)
    expect(result.visitorGrantCount).toBe(2)
    expect(result.grantPassTypeOptions).toEqual(["visitor", "employee_temp"])
    expect(result.filteredGrantLedger.map((item) => item.pass_type)).toEqual(["visitor", "employee_temp", "visitor"])
  })

  it("applies method, pass type, status, and date filters on top of tenant scope", () => {
    const result = buildTenantGrantViewModel({
      grantDateFrom: "2026-04-24",
      grantDateTo: "2026-04-24",
      grantMethodFilter: "email_qr",
      grantPassTypeFilter: "visitor",
      grantStatusFilter: "expiring_soon",
      grants: [
        {
          authorized_at: "2026-04-24T09:00:00Z",
          created_at: "2026-04-24T08:00:00Z",
          delivery_method: "email_qr",
          pass_type: "visitor",
          tenant_id: "tenant_a",
          valid_until: "2026-04-24T20:00:00Z",
        },
        {
          authorized_at: "2026-04-24T10:00:00Z",
          created_at: "2026-04-24T09:00:00Z",
          delivery_method: "wallet",
          pass_type: "visitor",
          tenant_id: "tenant_a",
          valid_until: "2026-04-24T20:00:00Z",
        },
      ] as never,
      nowTick: Date.parse("2026-04-24T00:00:00Z"),
      selectedTenantID: "tenant_a",
    })

    expect(result.filteredGrantLedger).toHaveLength(1)
    expect(result.filteredGrantLedger[0]?.delivery_method).toBe("email_qr")
  })
})
