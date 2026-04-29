import type { TFunction } from "i18next"
import { Suspense, lazy, useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { Link, Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom"
import {
  ActivityIcon,
  BellIcon,
  BriefcaseBusinessIcon,
  Building2Icon,
  ChevronRightIcon,
  DoorOpenIcon,
  LayoutDashboardIcon,
  LogOutIcon,
  NetworkIcon,
  ScrollTextIcon,
  SearchIcon,
  SatelliteDishIcon,
  ShieldAlertIcon,
  ShieldCheckIcon,
  WalletCardsIcon,
} from "lucide-react"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarSeparator,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { TooltipProvider } from "@/components/ui/tooltip"
import { MistyIslandMark } from "@/components/brand/misty-island-mark"
import { ProtectedRoute } from "@/components/protected-route"
import { RoleScopeBanner } from "@/components/role-scope-banner"
import { isMistyisletPreviewRoute } from "@/features/mistyislet-shell/navigation"

const HomePage = lazy(() =>
  import("@/features/home/pages/home-page").then((module) => ({ default: module.HomePage }))
)
import { listAlarms, type CurrentUser } from "@/lib/api"
import { useUIStore } from "@/stores/ui-store"
import { useAuth } from "@/context/auth-context"
import {
  canAccessAccessPage,
  canAccessAlarmsPage,
  canAccessAuditPage,
  canAccessEnterprisePage,
  canAccessEventsPage,
  canAccessGatewaysPage,
  canAccessGatewayInventory,
  canAccessIssuancePage,
  canAccessSpacesPage,
  canManageEnterprise,
  getViewerBuildingIDs,
  getViewerRoleLabel,
  isBuildingAdmin,
  isPlatformViewer,
} from "@/lib/viewer"

const DashboardPage = lazy(() =>
  import("@/features/legacy/pages/dashboard-page").then((module) => ({ default: module.DashboardPage }))
)
const TenantsPage = lazy(() =>
  import("@/features/legacy/pages/tenants-page").then((module) => ({ default: module.TenantsPage }))
)
const TenantDetailPage = lazy(() =>
  import("@/features/legacy/pages/tenant-detail-page").then((module) => ({ default: module.TenantDetailPage }))
)
const EnterprisePage = lazy(() =>
  import("@/features/legacy/pages/enterprise-page").then((module) => ({ default: module.EnterprisePage }))
)
const SpacesPage = lazy(() =>
  import("@/features/legacy/pages/spaces-page").then((module) => ({ default: module.SpacesPage }))
)
const AccessDirectoryPage = lazy(() =>
  import("@/features/legacy/pages/access-directory-page").then((module) => ({ default: module.AccessDirectoryPage }))
)
const AccessPoliciesPage = lazy(() =>
  import("@/features/legacy/pages/access-policies-page").then((module) => ({ default: module.AccessPoliciesPage }))
)
const AccessGrantsPage = lazy(() =>
  import("@/features/legacy/pages/access-grants-page").then((module) => ({ default: module.AccessGrantsPage }))
)
const AccessLegacySectionRedirectPage = lazy(() =>
  import("@/features/legacy/pages/access-legacy-section-redirect-page").then((module) => ({
    default: module.AccessLegacySectionRedirectPage,
  }))
)
const WalletPage = lazy(() =>
  import("@/features/legacy/pages/wallet-page").then((module) => ({ default: module.WalletPage }))
)
const GatewaysPage = lazy(() =>
  import("@/features/legacy/pages/gateways-page").then((module) => ({ default: module.GatewaysPage }))
)
const EventsPage = lazy(() =>
  import("@/features/legacy/pages/events-page").then((module) => ({ default: module.EventsPage }))
)
const AlarmsPage = lazy(() =>
  import("@/features/legacy/pages/alarms-page").then((module) => ({ default: module.AlarmsPage }))
)
const AuditPage = lazy(() =>
  import("@/features/legacy/pages/audit-page").then((module) => ({ default: module.AuditPage }))
)
const LoginPage = lazy(() =>
  import("@/features/legacy/pages/login-page").then((module) => ({ default: module.LoginPage }))
)
const AccessLinkClaimPage = lazy(() =>
  import("@/features/legacy/pages/access-link-claim-page").then((module) => ({ default: module.AccessLinkClaimPage }))
)
const NotFoundPage = lazy(() =>
  import("@/features/legacy/pages/not-found-page").then((module) => ({ default: module.NotFoundPage }))
)
const NoPermissionPage = lazy(() =>
  import("@/features/legacy/pages/no-permission-page").then((module) => ({ default: module.NoPermissionPage }))
)

type NavGroupID = "command" | "sites" | "people" | "platform"

const NAV_GROUPS: readonly NavGroupID[] = ["command", "sites", "people", "platform"]

type NavItem = {
  to: string
  group: NavGroupID
  label: string
  description: string
  icon: typeof LayoutDashboardIcon
}

function RouteFallback() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-[240px] items-center justify-center rounded-xl border bg-background text-sm text-muted-foreground">
      {t("app.routeLoading")}
    </div>
  )
}

function buildNavItems(viewer: CurrentUser, t: TFunction): NavItem[] {
  const items: NavItem[] = [
    {
      to: "/home",
      group: "command",
      label: t("app.nav.dashboard.label"),
      description: isPlatformViewer(viewer)
        ? t("app.nav.dashboard.descriptionPlatform")
        : t("app.nav.dashboard.descriptionTenant"),
      icon: LayoutDashboardIcon,
    },
  ]

  if (isPlatformViewer(viewer)) {
    items.push({
      to: "/tenants",
      group: "platform",
      label: t("app.nav.tenants.label"),
      description: t("app.nav.tenants.description"),
      icon: Building2Icon,
    })
  }

  if (canAccessAlarmsPage(viewer)) {
    items.push({
      to: "/alarms",
      group: "command",
      label: t("app.nav.alarms.label"),
      description: t("app.nav.alarms.description"),
      icon: ShieldAlertIcon,
    })
  }

  if (canAccessEventsPage(viewer)) {
    items.push({
      to: "/events",
      group: "command",
      label: t("app.nav.events.label"),
      description: isPlatformViewer(viewer)
        ? t("app.nav.events.descriptionPlatform")
        : t("app.nav.events.descriptionTenant"),
      icon: ScrollTextIcon,
    })
  }

  if (canAccessEnterprisePage(viewer)) {
    items.push({
      to: "/enterprise",
      group: "people",
      label: isPlatformViewer(viewer)
        ? t("app.nav.enterprise.labelPlatform")
        : t("app.nav.enterprise.labelTenant"),
      description: isPlatformViewer(viewer)
        ? t("app.nav.enterprise.descriptionPlatform")
        : canManageEnterprise(viewer)
          ? t("app.nav.enterprise.descriptionManager")
          : t("app.nav.enterprise.descriptionViewer"),
      icon: BriefcaseBusinessIcon,
    })
  }

  if (canAccessSpacesPage(viewer)) {
    items.push({
      to: "/spaces",
      group: "sites",
      label: t("app.nav.spaces.label"),
      description: t("app.nav.spaces.description"),
      icon: DoorOpenIcon,
    })
  }

  if (canAccessAccessPage(viewer)) {
    items.push({
      to: "/access",
      group: "people",
      label: t("app.nav.access.label"),
      description: t("app.nav.access.description"),
      icon: ShieldCheckIcon,
    })
  }

  if (canAccessIssuancePage(viewer)) {
    items.push({
      to: "/wallet",
      group: "people",
      label: t("app.nav.wallet.label"),
      description: t("app.nav.wallet.description"),
      icon: WalletCardsIcon,
    })
  }

  if (canAccessGatewaysPage(viewer)) {
    items.push({
      to: "/gateways",
      group: "sites",
      label: t("app.nav.gateways.label"),
      description: canAccessGatewayInventory(viewer)
        ? t("app.nav.gateways.descriptionInventory")
        : t("app.nav.gateways.descriptionStatus"),
      icon: NetworkIcon,
    })
  }

  if (canAccessAuditPage(viewer)) {
    items.push({
      to: "/audit",
      group: "platform",
      label: t("app.nav.audit.label"),
      description: t("app.nav.audit.description"),
      icon: SearchIcon,
    })
  }

  return items
}

function AppShell({ token, viewer, onLogout }: { token: string; viewer: CurrentUser; onLogout: () => void }) {
  const { t } = useTranslation()
  const location = useLocation()
  const navItems = useMemo(() => buildNavItems(viewer, t), [viewer, t])
  const navGroups = useMemo(() => {
    return NAV_GROUPS.map((group) => ({
      id: group,
      label: t(`app.navGroups.${group}`),
      items: navItems.filter((item) => item.group === group),
    })).filter((group) => group.items.length > 0)
  }, [navItems, t])
  const sidebarOpen = useUIStore((state) => state.sidebarOpen)
  const setSidebarOpen = useUIStore((state) => state.setSidebarOpen)
  const platformViewer = isPlatformViewer(viewer)
  const buildingAdmin = isBuildingAdmin(viewer)
  const viewerBuildingIDs = useMemo(() => new Set(getViewerBuildingIDs(viewer)), [viewer])
  const viewerBuildingScopeKey = useMemo(
    () => Array.from(viewerBuildingIDs).sort((a, b) => a.localeCompare(b)).join(","),
    [viewerBuildingIDs]
  )
  const missingBuildingScope = buildingAdmin && viewerBuildingIDs.size === 0
  const unreadAlarmQuery = useQuery({
    queryKey: ["app-shell-open-alarms", viewer.id, platformViewer, buildingAdmin, viewerBuildingScopeKey],
    queryFn: async () => {
      if (!canAccessAlarmsPage(viewer) || missingBuildingScope) {
        return 0
      }
      const alarms = await listAlarms(token, { page: 1, limit: 200 })
      const scopedAlarms = buildingAdmin
        ? alarms.filter((item) => viewerBuildingIDs.has(item.building_id))
        : alarms
      return scopedAlarms.filter((item) => item.status === "open").length
    },
    enabled: canAccessAlarmsPage(viewer),
    staleTime: 30 * 1000,
  })
  const unreadAlarmCount = unreadAlarmQuery.data ?? 0

  const activeNav = useMemo(() => {
    return (
      navItems.find(
        (item) => location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)
      ) ?? navItems[0]
    )
  }, [location.pathname, navItems])

  return (
    <TooltipProvider>
      <SidebarProvider open={sidebarOpen} onOpenChange={setSidebarOpen}>
        <Sidebar collapsible="icon" variant="inset" className="border-white/10">
          <SidebarHeader>
            <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-white/[0.045] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.10)] group-data-[collapsible=icon]:p-1.5">
              <div className="flex items-center gap-3">
                <MistyIslandMark className="size-10 shrink-0" markClassName="h-9 w-12" />
                <div className="min-w-0 group-data-[collapsible=icon]:hidden">
                  <p className="text-[11px] font-semibold tracking-[0.22em] text-sidebar-foreground/55 uppercase">
                    Mistyislet
                  </p>
                  <p className="truncate text-sm font-semibold">{t("app.shell.title")}</p>
                </div>
              </div>
              <div className="mt-3 rounded-xl border border-white/10 bg-black/20 px-3 py-2 group-data-[collapsible=icon]:hidden">
                <div className="flex items-center gap-2 text-xs text-sidebar-foreground/70">
                  <SatelliteDishIcon className="size-3.5" />
                  {isPlatformViewer(viewer)
                    ? t("app.shell.subtitlePlatform")
                    : t("app.shell.subtitleTenant")}
                </div>
              </div>
            </div>
          </SidebarHeader>

          <SidebarSeparator />

          <SidebarContent>
            {navGroups.map((group) => (
              <SidebarGroup key={group.id}>
                <SidebarGroupLabel className="tracking-[0.18em] uppercase">{group.label}</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu className="gap-1">
                    {group.items.map((item) => {
                      const isActive =
                        location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)

                      return (
                        <SidebarMenuItem key={item.to}>
                          <SidebarMenuButton
                            asChild
                            isActive={isActive}
                            size="lg"
                            tooltip={item.label}
                            className="h-12 rounded-xl data-active:bg-white/[0.14] data-active:shadow-[inset_0_1px_0_rgba(255,255,255,0.16),0_0_24px_rgba(255,255,255,0.08)]"
                          >
                            <NavLink to={item.to}>
                              <item.icon />
                              <span className="flex min-w-0 flex-col gap-0.5 group-data-[collapsible=icon]:hidden">
                                <span className="truncate">{item.label}</span>
                                <span className="truncate text-[11px] font-normal text-sidebar-foreground/48">
                                  {item.description}
                                </span>
                              </span>
                            </NavLink>
                          </SidebarMenuButton>
                        </SidebarMenuItem>
                      )
                    })}
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            ))}
          </SidebarContent>

          <SidebarFooter>
            <div className="rounded-2xl border border-white/10 bg-white/[0.045] p-3 group-data-[collapsible=icon]:hidden">
              <div className="flex items-center gap-2">
                <div className="flex size-9 items-center justify-center rounded-full border border-white/15 bg-white/10 text-xs font-semibold">
                  {viewer.email.slice(0, 2).toUpperCase()}
                </div>
                <div className="min-w-0">
                  <p className="truncate text-xs font-medium">{viewer.email}</p>
                  <p className="truncate text-[11px] text-sidebar-foreground/55">{getViewerRoleLabel(viewer)}</p>
                </div>
              </div>
            </div>
            <Button variant="outline" className="w-full justify-start border-white/10 bg-white/[0.045]" onClick={onLogout}>
              <LogOutIcon className="mr-1.5 size-4" />
              <span className="group-data-[collapsible=icon]:hidden">{t("app.shell.logout")}</span>
            </Button>
          </SidebarFooter>
          <SidebarRail />
        </Sidebar>

        <SidebarInset className="mp-fog-surface bg-background">
          <header className="sticky top-0 z-20 border-b border-white/10 bg-background/72 px-4 py-3 backdrop-blur-2xl md:px-6">
            <div className="flex flex-wrap items-center gap-3">
              <SidebarTrigger className="border border-white/10 bg-white/5" />

              <Breadcrumb>
                <BreadcrumbList>
                  <BreadcrumbItem>
                    <Link to="/home" className="text-muted-foreground hover:text-foreground">
                      {t("app.shell.console")}
                    </Link>
                  </BreadcrumbItem>
                  <BreadcrumbSeparator />
                  <BreadcrumbItem>
                    <BreadcrumbPage>{activeNav.label}</BreadcrumbPage>
                  </BreadcrumbItem>
                </BreadcrumbList>
              </Breadcrumb>

              <div className="ml-auto hidden min-w-[220px] items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-xs text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] md:flex">
                <SearchIcon className="size-3.5" />
                <span>{activeNav.description}</span>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <RoleScopeBanner token={token} viewer={viewer} />

                <Button variant="outline" size="sm" className="border-white/10 bg-white/[0.045]">
                  <BellIcon className="mr-1.5 size-4" />
                  {t("app.shell.unreadAlarms", { count: unreadAlarmCount })}
                </Button>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm" className="border-white/10 bg-white/[0.045]">
                      <ActivityIcon className="mr-1.5 size-4 text-emerald-300" />
                      <span className="hidden max-w-[12rem] truncate sm:inline">{viewer.email}</span>
                      <ChevronRightIcon className="ml-1 size-3.5 opacity-60" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuLabel>{t("app.shell.session")}</DropdownMenuLabel>
                    <DropdownMenuItem disabled>{getViewerRoleLabel(viewer)}</DropdownMenuItem>
                    <DropdownMenuItem disabled>{activeNav.description}</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={onLogout}>{t("app.shell.logout")}</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          </header>

          <main className="relative flex-1 px-4 py-5 md:px-6">
            <Suspense fallback={<RouteFallback />}>
              <Routes>
                <Route path="/dashboard" element={<Navigate to="/home" replace />} />
                <Route
                  path="/tenants"
                  element={
                    <ProtectedRoute allow={isPlatformViewer(viewer)}>
                      <TenantsPage token={token} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/tenants/:tenantID"
                  element={
                    <ProtectedRoute allow={isPlatformViewer(viewer)}>
                      <TenantDetailPage token={token} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/enterprise"
                  element={
                    <ProtectedRoute allow={canAccessEnterprisePage(viewer)}>
                      <EnterprisePage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/spaces"
                  element={
                    <ProtectedRoute allow={canAccessSpacesPage(viewer)}>
                      <SpacesPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/access"
                  element={
                    <ProtectedRoute allow={canAccessAccessPage(viewer)}>
                      <Navigate to="/access/directory" replace />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/access/:section"
                  element={
                    <ProtectedRoute allow={canAccessAccessPage(viewer)}>
                      <AccessLegacySectionRedirectPage />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/access/directory"
                  element={
                    <ProtectedRoute allow={canAccessAccessPage(viewer)}>
                      <AccessDirectoryPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/access/policies"
                  element={
                    <ProtectedRoute allow={canAccessAccessPage(viewer)}>
                      <AccessPoliciesPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/access/grants"
                  element={
                    <ProtectedRoute allow={canAccessAccessPage(viewer)}>
                      <AccessGrantsPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/wallet"
                  element={
                    <ProtectedRoute allow={canAccessIssuancePage(viewer)}>
                      <WalletPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/gateways"
                  element={
                    <ProtectedRoute allow={canAccessGatewaysPage(viewer)}>
                      <GatewaysPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/events"
                  element={
                    <ProtectedRoute allow={canAccessEventsPage(viewer)}>
                      <EventsPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/alarms"
                  element={
                    <ProtectedRoute allow={canAccessAlarmsPage(viewer)}>
                      <AlarmsPage token={token} viewer={viewer} />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/audit"
                  element={
                    <ProtectedRoute allow={canAccessAuditPage(viewer)}>
                      <AuditPage token={token} />
                    </ProtectedRoute>
                  }
                />
                <Route path="/login" element={<Navigate to="/home" replace />} />
                <Route path="*" element={<NotFoundPage authenticated />} />
              </Routes>
            </Suspense>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}

export default function App() {
  const { t } = useTranslation()
  const { token, viewer, bootstrapping, updateViewer, logout } = useAuth()
  const location = useLocation()
  const usesAccessLinkClaimRoute =
    location.pathname === "/access-link" ||
    location.pathname.startsWith("/access-link/") ||
    location.pathname === "/access-links/claim"

  if (usesAccessLinkClaimRoute) {
    return (
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/access-link" element={<AccessLinkClaimPage />} />
          <Route path="/access-link/:token" element={<AccessLinkClaimPage />} />
          <Route path="/access-links/claim" element={<AccessLinkClaimPage />} />
          <Route path="*" element={<NotFoundPage authenticated={Boolean(token)} />} />
        </Routes>
      </Suspense>
    )
  }

  if (!token) {
    return (
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/home" element={<Navigate to="/login" replace />} />
          <Route path="*" element={<NotFoundPage authenticated={false} />} />
        </Routes>
      </Suspense>
    )
  }

  if (bootstrapping || !viewer) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        {t("app.restoringSession")}
      </div>
    )
  }

  if (viewer.role === "resident") {
    return (
      <Suspense fallback={<RouteFallback />}>
        <NoPermissionPage viewer={viewer} onLogout={logout} />
      </Suspense>
    )
  }

  if (location.pathname === "/login") {
    return <Navigate to="/home" replace />
  }

  const usesMistyisletShell = isMistyisletPreviewRoute(location.pathname)

  if (usesMistyisletShell) {
    return (
      <Suspense fallback={<RouteFallback />}>
        <HomePage token={token} viewer={viewer} onViewerChange={updateViewer} onLogout={logout} />
      </Suspense>
    )
  }

  return <AppShell token={token} viewer={viewer} onLogout={logout} />
}
