import { describe, expect, it } from "vitest"
import { availableRolloutActions, buildSchedulePayload, buildingOptions, formatSchedule, isHHMM, rolloutStateBadgeVariant, targetSummary, validatePhases } from "./rollout-utils"

describe("rollout-utils", () => {
  it("validatePhases", () => {
    expect(validatePhases([{ percentage: 100, requires_approval: false }])).toBe(true)
    expect(validatePhases([{ percentage: 10, requires_approval: false }, { percentage: 50, requires_approval: false }, { percentage: 100, requires_approval: true }])).toBe(true)
    expect(validatePhases([])).toBe(false)
    expect(validatePhases([{ percentage: 50, requires_approval: false }])).toBe(false)
    expect(validatePhases([{ percentage: 50, requires_approval: false }, { percentage: 50, requires_approval: false }, { percentage: 100, requires_approval: false }])).toBe(false)
    expect(validatePhases([{ percentage: 0, requires_approval: false }, { percentage: 100, requires_approval: false }])).toBe(false)
  })
  it("availableRolloutActions mirrors backend guards", () => {
    expect(availableRolloutActions("awaiting_approval")).toEqual(["approve", "abort"])
    expect(availableRolloutActions("active")).toEqual(["pause", "abort"])
    expect(availableRolloutActions("paused")).toEqual(["resume", "abort"])
    expect(availableRolloutActions("scheduled")).toEqual(["abort"])
    expect(availableRolloutActions("completed")).toEqual([])
    expect(availableRolloutActions("failed")).toEqual([])
  })
  it("rolloutStateBadgeVariant", () => {
    expect(rolloutStateBadgeVariant("completed")).toBe("success")
    expect(rolloutStateBadgeVariant("failed")).toBe("destructive")
    expect(rolloutStateBadgeVariant("active")).toBe("default")
    expect(rolloutStateBadgeVariant("scheduled")).toBe("outline")
  })
  it("buildingOptions dedups + sorts", () => {
    expect(buildingOptions([{ building_id: "b2" }, { building_id: "b1" }, { building_id: "b1" }, { building_id: "" }])).toEqual(["b1", "b2"])
  })
  it("isHHMM", () => {
    expect(isHHMM("02:00")).toBe(true)
    expect(isHHMM("23:59")).toBe(true)
    expect(isHHMM("24:00")).toBe(false)
    expect(isHHMM("2:00")).toBe(false)
  })
  it("buildSchedulePayload", () => {
    expect(buildSchedulePayload({})).toBeUndefined()
    const p = buildSchedulePayload({ windowStart: "02:00", windowEnd: "05:00", timezone: "Asia/Jakarta" })
    expect(p).toEqual({ window_start: "02:00", window_end: "05:00", timezone: "Asia/Jakarta" })
    const p2 = buildSchedulePayload({ startAtLocal: "2026-06-08T02:00" })
    expect(typeof p2?.start_at).toBe("string")
    expect(p2?.start_at).toContain("T")
  })
  it("targetSummary builds i18n keys + params", () => {
    const t = ((key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)) as unknown as import("i18next").TFunction
    expect(targetSummary({ kind: "all" }, t)).toBe("ota.rollout.targetSummary.all")
    expect(targetSummary({ kind: "building", building_id: "b1" }, t)).toContain("b1")
    expect(targetSummary({ kind: "gateways", gateway_ids: ["a", "b"] }, t)).toContain("2")
  })
  it("formatSchedule", () => {
    expect(formatSchedule({ window_start: "02:00", window_end: "05:00", timezone: "Asia/Jakarta" })).toBe("02:00–05:00 (Asia/Jakarta)")
    expect(formatSchedule({ timezone: "UTC" })).toBe("UTC")
    expect(formatSchedule({})).toBe("—")
    expect(formatSchedule({ window_start: "02:00", window_end: "05:00" })).toBe("02:00–05:00")
  })
})
