import { describe, expect, it } from "vitest"

import {
  formatRetentionRate,
  occupancyBarHeights,
  type OccupancyDayInput,
} from "./space-analytics-utils"

describe("occupancyBarHeights", () => {
  const days: OccupancyDayInput[] = [
    { date: "2026-06-01", unique_users: 2 },
    { date: "2026-06-02", unique_users: 4 },
    { date: "2026-06-03", unique_users: 0 },
  ]

  it("scales the tallest day to 100% and others proportionally", () => {
    const bars = occupancyBarHeights(days, 60)
    expect(bars[1].heightPx).toBe(60)
    expect(bars[0].heightPx).toBe(30)
  })

  it("gives a small floor height to zero/low days so bars stay visible", () => {
    const bars = occupancyBarHeights(days, 60)
    expect(bars[2].heightPx).toBeGreaterThanOrEqual(2)
  })

  it("handles an all-zero set without dividing by zero", () => {
    const bars = occupancyBarHeights([{ date: "d", unique_users: 0 }], 60)
    expect(bars[0].heightPx).toBeGreaterThanOrEqual(2)
    expect(Number.isFinite(bars[0].heightPx)).toBe(true)
  })

  it("returns an empty array for no days", () => {
    expect(occupancyBarHeights([], 60)).toEqual([])
  })
})

describe("formatRetentionRate", () => {
  it("renders a fraction as a whole-number percent", () => {
    expect(formatRetentionRate(0.5)).toBe("50%")
    expect(formatRetentionRate(0)).toBe("0%")
    expect(formatRetentionRate(1)).toBe("100%")
    expect(formatRetentionRate(0.333)).toBe("33%")
  })
})
