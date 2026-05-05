import {
  BarChart3Icon,
  BellIcon,
  Building2Icon,
  CalendarCheckIcon,
  CameraIcon,
  CalendarClockIcon,
  CalendarDaysIcon,
  CreditCardIcon,
  DoorOpenIcon,
  FileTextIcon,
  HistoryIcon,
  HomeIcon,
  KeyRoundIcon,
  LayersIcon,
  MailIcon,
  MapPinPlusIcon,
  RouterIcon,
  ServerIcon,
  SettingsIcon,
  ShieldCheckIcon,
  SmartphoneIcon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react"

import type { TFunction } from "i18next"

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

const mistyisletPreviewRoutePrefixes = [
  "/my-account",
  "/users",
  "/teams",
  "/groups",
  "/access-rights",
  "/schedules",
  "/holiday-calendars",
  "/credentials",
  "/invitations",
  "/places",
  "/event-history",
  "/reports",
  "/organization",
  "/analytics",
  "/network",
  "/alarm-schedules",
  "/visitors",
  "/bookings",
  "/mobile-credentials",
  "/southbound",
  "/elevators",
  "/tenants",
  "/enterprise",
  "/spaces",
  "/access",
  "/access-links",
  "/access-link",
  "/wallet",
  "/cameras",
  "/gateways",
  "/events",
  "/alarms",
  "/audit",
  "/dashboard",
]

export type PlaceRouteMatch = {
  placeId: string
  section: string
}

export function placePath(section: string, placeId = DEMO_PLACE_ID) {
  return `/places/${placeId}/${section}`
}

export function organizationNavSections(t: TFunction): NavSection[] {
  return [
    {
      title: t("kisi.shell.navHome"),
      entries: [{ label: t("kisi.shell.navHome"), icon: HomeIcon, to: "/home" }],
    },
    {
      title: t("kisi.shell.navPeopleAccess"),
      entries: [
        { label: t("kisi.shell.navUsers"), icon: UsersIcon, to: "/users" },
        { label: t("kisi.shell.navTeams"), icon: UsersIcon, to: "/teams" },
        { label: t("kisi.shell.navGroups"), icon: UsersIcon, to: "/groups" },
        { label: t("kisi.shell.navAccessRights"), icon: KeyRoundIcon, to: "/access-rights" },
        { label: t("kisi.schedules.title"), icon: CalendarClockIcon, to: "/schedules" },
        { label: "Holiday Calendars", icon: CalendarDaysIcon, to: "/holiday-calendars" },
        { label: t("kisi.shell.navCredentials"), icon: CreditCardIcon, to: "/credentials" },
        { label: "Mobile Credentials", icon: SmartphoneIcon, to: "/mobile-credentials" },
        { label: t("kisi.invitations.title"), icon: MailIcon, to: "/invitations" },
      ],
    },
    {
      title: t("kisi.shell.navPlaces"),
      entries: [
        { label: t("kisi.shell.navPlaces"), icon: Building2Icon, to: "/places" },
        { label: "Bookings", icon: CalendarCheckIcon, to: "/bookings" },
      ],
    },
    {
      title: t("kisi.shell.navEventsReports"),
      entries: [
        { label: t("kisi.shell.navEventHistory"), icon: HistoryIcon, to: "/event-history" },
        { label: "Analytics", icon: BarChart3Icon, to: "/analytics" },
        { label: t("kisi.shell.navReports"), icon: FileTextIcon, to: "/reports" },
        { label: "Network", icon: ServerIcon, to: "/network" },
        { label: t("cameras.navLabel"), icon: CameraIcon, to: "/cameras" },
        { label: "Southbound Devices", icon: RouterIcon, to: "/southbound" },
        { label: "Alarm Schedules", icon: CalendarClockIcon, to: "/alarm-schedules" },
      ],
    },
    {
      title: t("kisi.shell.navOrgSetup"),
      collapsible: true,
      entries: [
        { label: t("kisi.shell.navAlertPolicies"), icon: BellIcon, to: "/organization/alert-policies" },
        { label: t("kisi.shell.navIntegrations"), icon: ShieldCheckIcon, to: "/organization/integrations" },
        { label: t("kisi.shell.navBilling"), icon: FileTextIcon, to: "/organization/billing" },
        { label: t("kisi.shell.navCreatePlace"), icon: MapPinPlusIcon, to: "/organization/create-place" },
        { label: t("kisi.shell.navSettings"), icon: SettingsIcon, to: "/organization/settings" },
        { label: t("kisi.shell.navSsoScim"), icon: KeyRoundIcon, to: "/organization/sso-scim" },
      ],
    },
  ]
}

export function placeNavSections(t: TFunction): NavSection[] {
  return [
    {
      title: t("kisi.shell.navHome"),
      entries: [{ label: t("kisi.shell.navHome"), icon: HomeIcon, to: "/home", placeReadOnlyVisible: true }],
    },
    {
      title: t("kisi.shell.navDashboard"),
      entries: [{ label: t("kisi.shell.navPlaceDashboard"), icon: BarChart3Icon, to: placePath("dashboard"), placeReadOnlyVisible: true }],
    },
    {
      title: t("kisi.shell.navPeopleAccess"),
      entries: [
        { label: t("kisi.shell.navPlaceUsers"), icon: UsersIcon, to: placePath("users") },
        { label: t("kisi.shell.navPlaceGroups"), icon: UsersIcon, to: placePath("groups") },
      ],
    },
    {
      title: t("kisi.shell.navSiteStructure"),
      entries: [
        { label: t("kisi.shell.navDoors"), icon: DoorOpenIcon, to: placePath("doors") },
        { label: t("kisi.shell.navFloors"), icon: LayersIcon, to: placePath("floors") },
        { label: t("kisi.shell.navElevators"), icon: Building2Icon, to: placePath("elevators") },
      ],
    },
    {
      title: t("kisi.shell.navActivity"),
      entries: [
        { label: t("kisi.shell.navUnlockHistory"), icon: HistoryIcon, to: placePath("unlock-history"), placeReadOnlyVisible: true },
        { label: t("kisi.shell.navAnalytics"), icon: BarChart3Icon, to: placePath("analytics"), placeReadOnlyVisible: true },
      ],
    },
    {
      title: t("kisi.shell.navOperations"),
      entries: [
        { label: t("kisi.shell.navCapacity"), icon: BarChart3Icon, to: placePath("capacity-management") },
        { label: t("kisi.shell.navIntrusion"), icon: ShieldCheckIcon, to: placePath("intrusion-detection") },
        { label: t("kisi.shell.navIntegrations"), icon: ShieldCheckIcon, to: placePath("integrations") },
        { label: t("kisi.shell.navHardware"), icon: ServerIcon, to: placePath("hardware") },
        { label: t("kisi.shell.navPlaceSettings"), icon: SettingsIcon, to: placePath("settings") },
      ],
    },
  ]
}

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

export function isMistyisletPreviewRoute(pathname: string) {
  const current = normalizePathname(pathname)
  return (
    current === "/home" ||
    mistyisletPreviewRoutePrefixes.some((prefix) => current === prefix || current.startsWith(`${prefix}/`))
  )
}

export function isPlaceAdminView(viewer: CurrentUser, pathname = "") {
  return viewer.role === "building_admin" || viewer.role === "operator" || isPlaceContextPath(pathname)
}

export function formatMistyisletRoleLabel(viewer: CurrentUser, pathname = "", t?: TFunction) {
  const translate = t ?? ((k: string) => k.split(".").pop() ?? k)
  return isPlaceAdminView(viewer, pathname) ? translate("common.placeAdmin") : translate("common.orgAdmin")
}

export function formatAccountMenuTitle(viewer: CurrentUser, pathname = "", t?: TFunction) {
  const translate = t ?? ((k: string) => k.split(".").pop() ?? k)
  const placeRoute = parsePlaceRoute(pathname)
  if (isPlaceAdminView(viewer, pathname)) {
    return `${formatPlaceName(placeRoute?.placeId ?? DEMO_PLACE_ID)} / ${translate("common.placeAdmin")}`
  }
  return `Mistyislet / ${translate("common.orgAdmin")}`
}

export function resolveNavSections(viewer: CurrentUser, pathname = "", t?: TFunction): NavSection[] {
  const placeRoute = parsePlaceRoute(pathname)
  const currentPlaceID = placeRoute?.placeId ?? DEMO_PLACE_ID
  const identity = ((k: string) => k) as TFunction
  const translate = t ?? identity
  const sections = isPlaceAdminView(viewer, pathname)
    ? placeNavSections(translate).map((section) => ({
        ...section,
        entries: section.entries.map((entry) => ({
          ...entry,
          to: entry.to?.startsWith("/places/") ? placePath(parsePlaceRoute(entry.to)?.section ?? "dashboard", currentPlaceID) : entry.to,
        })),
      }))
    : organizationNavSections(translate)
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
