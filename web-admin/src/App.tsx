import { Suspense, lazy, useMemo } from "react"
import { Link, Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom"
import {
  BellIcon,
  BriefcaseBusinessIcon,
  Building2Icon,
  DoorOpenIcon,
  LayoutDashboardIcon,
  LogOutIcon,
  NetworkIcon,
  ScrollTextIcon,
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
import { type CurrentUser } from "@/lib/api"
import { useAuth } from "@/context/auth-context"
import {
  canAccessAccessPage,
  canAccessAlarmsPage,
  canAccessEnterprisePage,
  canAccessEventsPage,
  canAccessGatewaysPage,
  canAccessGatewayInventory,
  canAccessIssuancePage,
  canAccessSpacesPage,
  canManageEnterprise,
  getViewerRoleLabel,
  isPlatformViewer,
} from "@/lib/viewer"

const DashboardPage = lazy(() =>
  import("@/pages/dashboard-page").then((module) => ({ default: module.DashboardPage }))
)
const TenantsPage = lazy(() =>
  import("@/pages/tenants-page").then((module) => ({ default: module.TenantsPage }))
)
const TenantDetailPage = lazy(() =>
  import("@/pages/tenant-detail-page").then((module) => ({ default: module.TenantDetailPage }))
)
const EnterprisePage = lazy(() =>
  import("@/pages/enterprise-page").then((module) => ({ default: module.EnterprisePage }))
)
const SpacesPage = lazy(() =>
  import("@/pages/spaces-page").then((module) => ({ default: module.SpacesPage }))
)
const AccessDirectoryPage = lazy(() =>
  import("@/pages/access-directory-page").then((module) => ({ default: module.AccessDirectoryPage }))
)
const AccessPoliciesPage = lazy(() =>
  import("@/pages/access-policies-page").then((module) => ({ default: module.AccessPoliciesPage }))
)
const AccessGrantsPage = lazy(() =>
  import("@/pages/access-grants-page").then((module) => ({ default: module.AccessGrantsPage }))
)
const AccessLegacySectionRedirectPage = lazy(() =>
  import("@/pages/access-legacy-section-redirect-page").then((module) => ({
    default: module.AccessLegacySectionRedirectPage,
  }))
)
const WalletPage = lazy(() =>
  import("@/pages/wallet-page").then((module) => ({ default: module.WalletPage }))
)
const GatewaysPage = lazy(() =>
  import("@/pages/gateways-page").then((module) => ({ default: module.GatewaysPage }))
)
const EventsPage = lazy(() =>
  import("@/pages/events-page").then((module) => ({ default: module.EventsPage }))
)
const AlarmsPage = lazy(() =>
  import("@/pages/alarms-page").then((module) => ({ default: module.AlarmsPage }))
)
const LoginPage = lazy(() =>
  import("@/pages/login-page").then((module) => ({ default: module.LoginPage }))
)

type NavItem = {
  to: string
  label: string
  description: string
  icon: typeof LayoutDashboardIcon
}

function RouteFallback() {
  return (
    <div className="flex min-h-[240px] items-center justify-center rounded-xl border bg-background text-sm text-muted-foreground">
      正在加载页面...
    </div>
  )
}

function buildNavItems(viewer: CurrentUser): NavItem[] {
  const items: NavItem[] = [
    {
      to: "/dashboard",
      label: "仪表盘",
      description: isPlatformViewer(viewer) ? "平台工作台总览" : "组织工作台总览",
      icon: LayoutDashboardIcon,
    },
  ]

  if (isPlatformViewer(viewer)) {
    items.push({
      to: "/tenants",
      label: "租户",
      description: "租户生命周期",
      icon: Building2Icon,
    })
  }

  if (canAccessEnterprisePage(viewer)) {
    items.push({
      to: "/enterprise",
      label: "企业",
      description: canManageEnterprise(viewer) ? "员工、同步与 SSO" : "目录与同步概览",
      icon: BriefcaseBusinessIcon,
    })
  }

  if (canAccessSpacesPage(viewer)) {
    items.push({
      to: "/spaces",
      label: "空间",
      description: "楼宇与门点",
      icon: DoorOpenIcon,
    })
  }

  if (canAccessAccessPage(viewer)) {
    items.push({
      to: "/access",
      label: "权限",
      description: "目录、策略与授权",
      icon: ShieldCheckIcon,
    })
  }

  if (canAccessIssuancePage(viewer)) {
    items.push({
      to: "/wallet",
      label: "凭证发放",
      description: "MistyPass 发放与状态",
      icon: WalletCardsIcon,
    })
  }

  if (canAccessGatewaysPage(viewer)) {
    items.push({
      to: "/gateways",
      label: "网关",
      description: canAccessGatewayInventory(viewer) ? "边缘设备与库存" : "边缘设备状态",
      icon: NetworkIcon,
    })
  }

  if (canAccessEventsPage(viewer)) {
    items.push({
      to: "/events",
      label: "事件",
      description: isPlatformViewer(viewer) ? "实时事件流" : "组织事件流",
      icon: ScrollTextIcon,
    })
  }

  if (canAccessAlarmsPage(viewer)) {
    items.push({
      to: "/alarms",
      label: "告警",
      description: "告警处置",
      icon: ShieldAlertIcon,
    })
  }

  return items
}

function AppShell({ token, viewer, onLogout }: { token: string; viewer: CurrentUser; onLogout: () => void }) {
  const location = useLocation()
  const navItems = useMemo(() => buildNavItems(viewer), [viewer])

  const activeNav = useMemo(() => {
    return (
      navItems.find(
        (item) => location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)
      ) ?? navItems[0]
    )
  }, [location.pathname, navItems])

  return (
    <TooltipProvider>
      <SidebarProvider defaultOpen>
        <Sidebar collapsible="icon" variant="inset">
          <SidebarHeader>
            <div className="rounded-lg border bg-sidebar-accent/50 px-2.5 py-2">
              <p className="text-[11px] font-medium tracking-[0.06em] text-sidebar-foreground/70">MistyPass</p>
              <p className="font-medium">访问管理后台</p>
              <p className="text-xs text-sidebar-foreground/70">
                {isPlatformViewer(viewer) ? "平台工作台" : "企业工作台"}
              </p>
            </div>
          </SidebarHeader>

          <SidebarSeparator />

          <SidebarContent>
            <SidebarGroup>
              <SidebarGroupLabel>管理导航</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {navItems.map((item) => {
                    const isActive =
                      location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)

                    return (
                      <SidebarMenuItem key={item.to}>
                        <SidebarMenuButton asChild isActive={isActive} tooltip={item.label}>
                          <NavLink to={item.to}>
                            <item.icon />
                            <span>{item.label}</span>
                          </NavLink>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    )
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </SidebarContent>

          <SidebarFooter>
            <Button variant="outline" className="w-full justify-start" onClick={onLogout}>
              <LogOutIcon className="mr-1.5 size-4" />
              退出登录
            </Button>
          </SidebarFooter>
          <SidebarRail />
        </Sidebar>

        <SidebarInset className="bg-[radial-gradient(circle_at_10%_0%,rgba(45,212,191,0.10),transparent_30%),radial-gradient(circle_at_95%_100%,rgba(56,189,248,0.10),transparent_35%)]">
          <header className="sticky top-0 z-20 border-b bg-background/90 px-4 py-3 backdrop-blur md:px-6">
            <div className="flex flex-wrap items-center gap-3">
              <SidebarTrigger />

              <Breadcrumb>
                <BreadcrumbList>
                  <BreadcrumbItem>
                    <Link to="/dashboard" className="text-muted-foreground hover:text-foreground">
                      控制台
                    </Link>
                  </BreadcrumbItem>
                  <BreadcrumbSeparator />
                  <BreadcrumbItem>
                    <BreadcrumbPage>{activeNav.label}</BreadcrumbPage>
                  </BreadcrumbItem>
                </BreadcrumbList>
              </Breadcrumb>

              <div className="ml-auto flex items-center gap-2">
                <Button variant="outline" size="sm">
                  <BellIcon className="mr-1.5 size-4" />
                  3 条告警
                </Button>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm">
                      {viewer.email}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuLabel>会话</DropdownMenuLabel>
                    <DropdownMenuItem disabled>{getViewerRoleLabel(viewer)}</DropdownMenuItem>
                    <DropdownMenuItem disabled>{activeNav.description}</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={onLogout}>退出登录</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          </header>

          <main className="flex-1 px-4 py-5 md:px-6">
            <Suspense fallback={<RouteFallback />}>
              <Routes>
                <Route path="/dashboard" element={<DashboardPage token={token} viewer={viewer} />} />
                <Route
                  path="/tenants"
                  element={isPlatformViewer(viewer) ? <TenantsPage token={token} /> : <Navigate to="/dashboard" replace />}
                />
                <Route
                  path="/tenants/:tenantID"
                  element={isPlatformViewer(viewer) ? <TenantDetailPage token={token} /> : <Navigate to="/dashboard" replace />}
                />
                <Route
                  path="/enterprise"
                  element={
                    canAccessEnterprisePage(viewer) ? (
                      <EnterprisePage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/spaces"
                  element={
                    canAccessSpacesPage(viewer) ? (
                      <SpacesPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/access"
                  element={
                    canAccessAccessPage(viewer) ? (
                      <Navigate to="/access/directory" replace />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/access/:section"
                  element={
                    canAccessAccessPage(viewer) ? (
                      <AccessLegacySectionRedirectPage />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/access/directory"
                  element={
                    canAccessAccessPage(viewer) ? (
                      <AccessDirectoryPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/access/policies"
                  element={
                    canAccessAccessPage(viewer) ? (
                      <AccessPoliciesPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/access/grants"
                  element={
                    canAccessAccessPage(viewer) ? (
                      <AccessGrantsPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/wallet"
                  element={
                    canAccessIssuancePage(viewer) ? (
                      <WalletPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/gateways"
                  element={
                    canAccessGatewaysPage(viewer) ? (
                      <GatewaysPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/events"
                  element={
                    canAccessEventsPage(viewer) ? (
                      <EventsPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route
                  path="/alarms"
                  element={
                    canAccessAlarmsPage(viewer) ? (
                      <AlarmsPage token={token} viewer={viewer} />
                    ) : (
                      <Navigate to="/dashboard" replace />
                    )
                  }
                />
                <Route path="/login" element={<Navigate to="/dashboard" replace />} />
                <Route path="*" element={<Navigate to="/dashboard" replace />} />
              </Routes>
            </Suspense>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}

export default function App() {
  const { token, viewer, bootstrapping, logout } = useAuth()

  if (!token) {
    return (
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </Suspense>
    )
  }

  if (bootstrapping || !viewer) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        正在恢复当前会话...
      </div>
    )
  }

  return <AppShell token={token} viewer={viewer} onLogout={logout} />
}
