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
  getGrantLifecycleStatus,
  inferPolicyStarterScope,
  inferPositionTemplateByGroupName,
  inferStarterGroupMemberIDs,
  parseDateTime,
  validateScope,
} from "./access-page-utils"

describe("access page helpers", () => {
  it("parses datetime values with ISO and space-separated input", () => {
    expect(parseDateTime("2026-04-24T08:30:00Z")?.toISOString()).toBe("2026-04-24T08:30:00.000Z")
    expect(parseDateTime("2026-04-24 08:30:00")?.getTime()).toBeTypeOf("number")
    expect(parseDateTime("")).toBeNull()
  })

  it("classifies grant lifecycle against current time", () => {
    const now = Date.parse("2026-04-24T00:00:00Z")

    expect(getGrantLifecycleStatus("2026-04-26T00:00:00Z", now)).toBe("active")
    expect(getGrantLifecycleStatus("2026-04-24T12:00:00Z", now)).toBe("expiring_soon")
    expect(getGrantLifecycleStatus("2026-04-23T23:59:59Z", now)).toBe("expired")
  })

  it("infers policy starter scopes from access role and topology completeness", () => {
    expect(inferPolicyStarterScope("tenant_admin", "b1", "a1", "d1")).toBe("all")
    expect(inferPolicyStarterScope("operator", "b1", "a1", "d1")).toBe("door")
    expect(inferPolicyStarterScope("operator", "b1", "a1", "")).toBe("area")
    expect(inferPolicyStarterScope("resident", "b1", "", "")).toBe("building")
  })

  it("validates scope requirements and maps starter templates by group name", () => {
    expect(validateScope("door", "building_1", "area_1", "")).toBe("Door scope requires building, area, and door")
    expect(validateScope("all", "", "", "")).toBe("")
    expect(inferPositionTemplateByGroupName("Factory Security")).toMatchObject({
      accessRole: "operator",
      defaultGroup: "Factory Security",
    })
  })

  it("derives starter group members from template role and employee metadata", () => {
    const template = inferPositionTemplateByGroupName("Common Office Access")
    expect(template).not.toBeNull()

    expect(
      inferStarterGroupMemberIDs(template!, [
        {
          access_role: "resident",
          department: "Finance",
          id: "emp_1",
          job_title: "Analyst",
          location: "Jakarta",
          status: "active",
        },
        {
          access_role: "operator",
          department: "Security",
          id: "emp_2",
          job_title: "Guard",
          location: "Jakarta",
          status: "active",
        },
        {
          access_role: "resident",
          department: "HR",
          id: "emp_3",
          job_title: "Generalist",
          location: "Jakarta",
          status: "inactive",
        },
      ] as never)
    ).toEqual(["emp_1"])
  })
})
