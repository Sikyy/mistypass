import type { TFunction } from "i18next"
import type { RolloutPhase, RolloutSchedule, RolloutTarget } from "@/lib/api/ota"

export type { RolloutActionName } from "@/lib/api/ota"
import type { RolloutActionName } from "@/lib/api/ota"

export function validatePhases(phases: RolloutPhase[]): boolean {
  if (phases.length === 0) return false
  let prev = 0
  for (const p of phases) {
    if (!Number.isInteger(p.percentage) || p.percentage < 1 || p.percentage > 100 || p.percentage <= prev) return false
    prev = p.percentage
  }
  return prev === 100
}

export function availableRolloutActions(state: string): RolloutActionName[] {
  switch (state) {
    case "awaiting_approval": return ["approve", "abort"]
    case "active": return ["pause", "abort"]
    case "paused": return ["resume", "abort"]
    case "scheduled":
    case "pending": return ["abort"]
    default: return []
  }
}

type BadgeVariant = "default" | "secondary" | "destructive" | "success" | "warning" | "danger" | "outline" | "ghost" | "link"

export function rolloutStateBadgeVariant(state: string): BadgeVariant {
  switch (state) {
    case "active": return "default"
    case "completed": return "success"
    case "failed": return "destructive"
    case "paused":
    case "awaiting_approval": return "warning"
    case "scheduled": return "outline"
    default: return "secondary"
  }
}

export function buildingOptions(gateways: Array<{ building_id?: string }>): string[] {
  const set = new Set<string>()
  for (const g of gateways) {
    if (g.building_id && g.building_id.trim() !== "") set.add(g.building_id.trim())
  }
  return Array.from(set).sort()
}

export function isHHMM(s: string): boolean {
  return /^([01][0-9]|2[0-3]):[0-5][0-9]$/.test(s.trim())
}

export function buildSchedulePayload(input: { startAtLocal?: string; windowStart?: string; windowEnd?: string; timezone?: string }): RolloutSchedule | undefined {
  const sch: RolloutSchedule = {}
  if (input.startAtLocal && input.startAtLocal.trim() !== "") sch.start_at = new Date(input.startAtLocal).toISOString()
  if (input.windowStart && input.windowStart.trim() !== "") sch.window_start = input.windowStart.trim()
  if (input.windowEnd && input.windowEnd.trim() !== "") sch.window_end = input.windowEnd.trim()
  if (input.timezone && input.timezone.trim() !== "") sch.timezone = input.timezone.trim()
  return Object.keys(sch).length > 0 ? sch : undefined
}

export function formatSchedule(s: RolloutSchedule, locale?: string): string {
  const parts: string[] = []
  if (s.window_start && s.window_end) {
    parts.push(`${s.window_start}–${s.window_end}${s.timezone ? ` (${s.timezone})` : ""}`)
  } else if (s.timezone) {
    parts.push(s.timezone)
  }
  if (s.start_at) {
    parts.push(`≥ ${new Date(s.start_at).toLocaleString(locale)}`)
  }
  return parts.length > 0 ? parts.join(", ") : "—"
}

export function targetSummary(target: RolloutTarget, t: TFunction): string {
  if (target.kind === "all") return t("ota.rollout.targetSummary.all")
  if (target.kind === "building") return t("ota.rollout.targetSummary.building", { id: target.building_id ?? "—" })
  return t("ota.rollout.targetSummary.gateways", { count: target.gateway_ids?.length ?? 0 })
}
