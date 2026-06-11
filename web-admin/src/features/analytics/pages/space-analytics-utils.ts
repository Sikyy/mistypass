export type OccupancyDayInput = {
  date: string
  unique_users: number
}

export type OccupancyBar = {
  date: string
  uniqueUsers: number
  heightPx: number
}

const MIN_BAR_PX = 2

/**
 * Scales daily occupancy to bar heights: the busiest day fills maxPx, others
 * scale proportionally, and every bar keeps a small floor so empty days stay
 * visible. Safe when all days are zero or the list is empty.
 */
export function occupancyBarHeights(days: OccupancyDayInput[], maxPx: number): OccupancyBar[] {
  const peak = days.reduce((acc, day) => Math.max(acc, day.unique_users), 0)
  return days.map((day) => {
    const scaled = peak > 0 ? (day.unique_users / peak) * maxPx : 0
    return {
      date: day.date,
      uniqueUsers: day.unique_users,
      heightPx: Math.max(Math.round(scaled), MIN_BAR_PX),
    }
  })
}

/** Renders a 0..1 retention fraction as a whole-number percent string. */
export function formatRetentionRate(rate: number): string {
  return `${Math.round(rate * 100)}%`
}
