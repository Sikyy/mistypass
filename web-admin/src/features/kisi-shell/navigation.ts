import {
  BarChart3Icon,
  BellIcon,
  Building2Icon,
  CreditCardIcon,
  DoorOpenIcon,
  FileTextIcon,
  HistoryIcon,
  HomeIcon,
  KeyRoundIcon,
  LayersIcon,
  MapPinPlusIcon,
  ServerIcon,
  SettingsIcon,
  ShieldCheckIcon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react"

import type { CurrentUser } from "@/lib/api"

export type NavEntry = {
  label: string
  icon: LucideIcon
  to?: string
  placeReadOnlyVisible?: boolean
}

export type NavSection = {
  title?: string
  collapsible?: boolean
  entries: NavEntry[]
}

export const DEMO_PLACE_ID = "sudirman-hub"

const kisiPreviewRoutePrefixes = [
  "/my-account",
  "/users",
  "/teams",
  "/groups",
  "/access-rights",
  "/credentials",
  "/places",
  "/event-history",
  "/reports",
  "/organization",
]

export type PlaceRouteMatch = {
  placeId: string
  section: string
}

export function placePath(section: string, placeId = DEMO_PLACE_ID) {
  return `/places/${placeId}/${section}`
}

export const organizationNavSections: NavSection[] = [
  {
    title: "Home",
    entries: [{ label: "Home", icon: HomeIcon, to: "/home" }],
  },
  {
    title: "People & Access",
    entries: [
      { label: "Users", icon: UsersIcon, to: "/users" },
      { label: "Teams", icon: UsersIcon, to: "/teams" },
      { label: "Groups", icon: UsersIcon, to: "/groups" },
      { label: "Access Rights", icon: KeyRoundIcon, to: "/access-rights" },
      { label: "Credentials", icon: CreditCardIcon, to: "/credentials" },
    ],
  },
  {
    title: "Places",
    entries: [{ label: "Places", icon: Building2Icon, to: "/places" }],
  },
  {
    title: "Events & Reports",
    entries: [
      { label: "Event History", icon: HistoryIcon, to: "/event-history" },
      { label: "Reports", icon: BarChart3Icon, to: "/reports" },
    ],
  },
  {
    title: "Organization Setup",
    collapsible: true,
    entries: [
      { label: "Alert Policies", icon: BellIcon, to: "/organization/alert-policies" },
      { label: "Integrations", icon: ShieldCheckIcon, to: "/organization/integrations" },
      { label: "Billing", icon: FileTextIcon, to: "/organization/billing" },
      { label: "Create Place", icon: MapPinPlusIcon, to: "/organization/create-place" },
      { label: "Settings", icon: SettingsIcon, to: "/organization/settings" },
      { label: "SSO & SCIM", icon: KeyRoundIcon, to: "/organization/sso-scim" },
    ],
  },
]

export const placeNavSections: NavSection[] = [
  {
    title: "Home",
    entries: [{ label: "Home", icon: HomeIcon, to: "/home", placeReadOnlyVisible: true }],
  },
  {
    title: "Dashboard",
    entries: [{ label: "Place Dashboard", icon: BarChart3Icon, to: placePath("dashboard"), placeReadOnlyVisible: true }],
  },
  {
    title: "People & Access",
    entries: [
      { label: "Place Users", icon: UsersIcon, to: placePath("users") },
      { label: "Place Groups", icon: UsersIcon, to: placePath("groups") },
    ],
  },
  {
    title: "Site Structure",
    entries: [
      { label: "Doors", icon: DoorOpenIcon, to: placePath("doors") },
      { label: "Floors", icon: LayersIcon, to: placePath("floors") },
      { label: "Elevators", icon: Building2Icon, to: placePath("elevators") },
    ],
  },
  {
    title: "Activity",
    entries: [
      { label: "Unlock History", icon: HistoryIcon, to: placePath("unlock-history"), placeReadOnlyVisible: true },
      { label: "Analytics", icon: BarChart3Icon, to: placePath("analytics"), placeReadOnlyVisible: true },
    ],
  },
  {
    title: "Operations",
    entries: [
      { label: "Capacity Management", icon: BarChart3Icon, to: placePath("capacity-management") },
      { label: "Intrusion Detection", icon: ShieldCheckIcon, to: placePath("intrusion-detection") },
      { label: "Integrations", icon: ShieldCheckIcon, to: placePath("integrations") },
      { label: "Hardware", icon: ServerIcon, to: placePath("hardware") },
      { label: "Place Settings", icon: SettingsIcon, to: placePath("settings") },
    ],
  },
]

export function normalizePathname(pathname: string) {
  return pathname.replace(/\/+$/, "") || "/"
}

export function parsePlaceRoute(pathname: string): PlaceRouteMatch | null {
  const parts = normalizePathname(pathname).split("/").filter(Boolean)
  if (parts[0] !== "places" || parts.length < 2) {
    return null
  }
  const placeId = parts[1]
  if (!placeId) {
    return null
  }
  return {
    placeId,
    section: parts[2] || "dashboard",
  }
}

export function formatPlaceName(placeId: string) {
  if (placeId === "assigned" || placeId === DEMO_PLACE_ID) {
    return "Sudirman Hub"
  }
  return decodeURIComponent(placeId)
    .split(/[-_]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

export function isPlaceContextPath(pathname: string) {
  return parsePlaceRoute(pathname) !== null
}

export function isKisiPreviewRoute(pathname: string) {
  const current = normalizePathname(pathname)
  return (
    current === "/home" ||
    kisiPreviewRoutePrefixes.some((prefix) => current === prefix || current.startsWith(`${prefix}/`))
  )
}

export function isPlaceAdminView(viewer: CurrentUser, pathname = "") {
  return viewer.role === "building_admin" || viewer.role === "operator" || isPlaceContextPath(pathname)
}

export function formatKisiRoleLabel(viewer: CurrentUser, pathname = "") {
  return isPlaceAdminView(viewer, pathname) ? "Place Admin" : "Organization Admin"
}

export function formatAccountMenuTitle(viewer: CurrentUser, pathname = "") {
  const placeRoute = parsePlaceRoute(pathname)
  if (isPlaceAdminView(viewer, pathname)) {
    return `${formatPlaceName(placeRoute?.placeId ?? DEMO_PLACE_ID)} / Place Admin`
  }
  return "Mistyislet / Organization Admin"
}

export function resolveNavSections(viewer: CurrentUser, pathname = ""): NavSection[] {
  const placeRoute = parsePlaceRoute(pathname)
  const currentPlaceID = placeRoute?.placeId ?? DEMO_PLACE_ID
  const sections = isPlaceAdminView(viewer, pathname)
    ? placeNavSections.map((section) => ({
        ...section,
        entries: section.entries.map((entry) => ({
          ...entry,
          to: entry.to?.startsWith("/places/") ? placePath(parsePlaceRoute(entry.to)?.section ?? "dashboard", currentPlaceID) : entry.to,
        })),
      }))
    : organizationNavSections
  if (viewer.role !== "operator") {
    return sections
  }

  return sections
    .map((section) => ({
      ...section,
      entries: section.entries.filter((entry) => entry.placeReadOnlyVisible),
    }))
    .filter((section) => section.entries.length > 0)
}

export function isNavEntryActive(entry: NavEntry, pathname: string) {
  if (!entry.to) {
    return false
  }
  const current = normalizePathname(pathname)
  const target = normalizePathname(entry.to)
  const currentPlaceRoute = parsePlaceRoute(current)
  const targetPlaceRoute = parsePlaceRoute(target)
  if (currentPlaceRoute && targetPlaceRoute) {
    return currentPlaceRoute.section === targetPlaceRoute.section
  }
  if (target === "/home") {
    return current === "/home"
  }
  return current === target || current.startsWith(`${target}/`)
}
