import { lazy, Suspense, type ReactNode } from "react"
import { Navigate, Route, Routes, useLocation, useParams } from "react-router"

import { PageFrame } from "@/components/mistyislet/primitives"
import type { CurrentUser } from "@/lib/api"

import { DEMO_PLACE_ID } from "./navigation"

const AccessRightsAdaptedPage = lazy(() =>
  import("@/features/access-rights/pages/access-rights-page").then((module) => ({ default: module.AccessRightsAdaptedPage }))
)
const SchedulesAdaptedPage = lazy(() =>
  import("@/features/access-rights/pages/schedules-page").then((module) => ({ default: module.SchedulesAdaptedPage }))
)
const HolidayCalendarsAdaptedPage = lazy(() =>
  import("@/features/access-rights/pages/holiday-calendars-page").then((module) => ({ default: module.HolidayCalendarsAdaptedPage }))
)
const MyAccountPage = lazy(() =>
  import("@/features/account/pages/my-account-page").then((module) => ({ default: module.MyAccountPage }))
)
const CredentialsAdaptedPage = lazy(() =>
  import("@/features/credentials/pages/credentials-page").then((module) => ({ default: module.CredentialsAdaptedPage }))
)
const EventHistoryAdaptedPage = lazy(() =>
  import("@/features/event-history/pages/event-history-page").then((module) => ({ default: module.EventHistoryAdaptedPage }))
)
const GroupsAdaptedPage = lazy(() =>
  import("@/features/groups/pages/groups-page").then((module) => ({ default: module.GroupsAdaptedPage }))
)
const OrganizationSetupAdaptedPage = lazy(() =>
  import("@/features/organization/pages/organization-setup-page").then((module) => ({ default: module.OrganizationSetupAdaptedPage }))
)
const DoorDetailAdaptedPage = lazy(() =>
  import("@/features/places/pages/door-detail-page").then((module) => ({ default: module.DoorDetailAdaptedPage }))
)
const FloorsAdaptedPage = lazy(() =>
  import("@/features/places/pages/floors-page").then((module) => ({ default: module.FloorsAdaptedPage }))
)
const HardwareAdaptedPage = lazy(() =>
  import("@/features/places/pages/hardware-page").then((module) => ({ default: module.HardwareAdaptedPage }))
)
const PlaceDashboardAdaptedPage = lazy(() =>
  import("@/features/places/pages/place-dashboard-page").then((module) => ({ default: module.PlaceDashboardAdaptedPage }))
)
const ElevatorsAdaptedPage = lazy(() =>
  import("@/features/elevators/pages/elevators-page").then((module) => ({ default: module.ElevatorsAdaptedPage }))
)
const PlacesAdaptedPage = lazy(() =>
  import("@/features/places/pages/places-list-page").then((module) => ({ default: module.PlacesAdaptedPage }))
)
const PlaceOperationsAdaptedPage = lazy(() =>
  import("@/features/places/pages/place-operations-page").then((module) => ({ default: module.PlaceOperationsAdaptedPage }))
)
const PlaceSettingsAdaptedPage = lazy(() =>
  import("@/features/places/pages/place-settings-page").then((module) => ({ default: module.PlaceSettingsAdaptedPage }))
)
const ReportsAdaptedPage = lazy(() =>
  import("@/features/reports/pages/reports-page").then((module) => ({ default: module.ReportsAdaptedPage }))
)
const TeamsAdaptedPage = lazy(() =>
  import("@/features/teams/pages/teams-page").then((module) => ({ default: module.TeamsAdaptedPage }))
)
const UserDetailAdaptedPage = lazy(() =>
  import("@/features/users/pages/user-detail-page").then((module) => ({ default: module.UserDetailAdaptedPage }))
)
const InvitationsAdaptedPage = lazy(() =>
  import("@/features/users/pages/invitations-page").then((module) => ({ default: module.InvitationsAdaptedPage }))
)
const UsersAdaptedPage = lazy(() =>
  import("@/features/users/pages/users-page").then((module) => ({ default: module.UsersAdaptedPage }))
)
const AnalyticsPage = lazy(() =>
  import("@/features/analytics/pages/analytics-page").then((module) => ({ default: module.AnalyticsPage }))
)
const NetworkTopologyPage = lazy(() =>
  import("@/features/network/pages/network-topology-page").then((module) => ({ default: module.NetworkTopologyPage }))
)
const AlarmSchedulePage = lazy(() =>
  import("@/features/alarms/pages/alarm-schedule-page").then((module) => ({ default: module.AlarmSchedulePage }))
)
const ReportSchedulePage = lazy(() =>
  import("@/features/reports/pages/report-schedule-page").then((module) => ({ default: module.ReportSchedulePage }))
)
const CamerasPage = lazy(() =>
  import("@/features/cameras/pages/cameras-page").then((module) => ({ default: module.CamerasPage }))
)
const VisitorsPage = lazy(() =>
  import("@/features/visitors/pages/visitors-page").then((module) => ({ default: module.VisitorsPage }))
)
const BookingsPage = lazy(() =>
  import("@/features/bookings/pages/bookings-page").then((module) => ({ default: module.BookingsPage }))
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
  import("@/features/legacy/pages/access-legacy-section-redirect-page").then((module) => ({ default: module.AccessLegacySectionRedirectPage }))
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
const AccessLinkClaimPage = lazy(() =>
  import("@/features/legacy/pages/access-link-claim-page").then((module) => ({ default: module.AccessLinkClaimPage }))
)

type MistyisletConsoleRoutesProps = {
  homeContent: ReactNode
  token: string
  viewer: CurrentUser
  onViewerChange: (viewer: CurrentUser) => void
  onLogout: () => void
}

type AuthenticatedResourcePageProps = Pick<MistyisletConsoleRoutesProps, "token" | "viewer">
type PlaceResourcePageProps = AuthenticatedResourcePageProps & {
  section?: string
}

function titleFromSegment(segment: string) {
  return segment
    .split("-")
    .map((item) => item.charAt(0).toUpperCase() + item.slice(1))
    .join(" ")
}

function GenericAdaptedPage({ title, placeScoped = false }: { title: string; placeScoped?: boolean }) {
  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", "Assigned Place", title] : ["Home", title]}
      title={title}
      description="This workspace follows the same Mistyislet console layout while the detailed workflow is being adapted."
    >
      <section className="rounded-[6px] border border-line-default bg-white">
        <div className="border-b border-line-subtle px-6 py-5">
          <h2 className="text-base font-semibold text-content-heading">{title}</h2>
          <p className="mt-1 text-sm text-content-subtle">Primary actions, filters, and records will stay in one flat surface.</p>
        </div>
        <div className="grid gap-4 p-6 md:grid-cols-3">
          {["Active", "Pending", "Needs review"].map((item, index) => (
            <div key={item} className="rounded-[6px] border border-line-subtle p-4">
              <p className="text-sm text-content-subtle">{item}</p>
              <p className="mt-3 text-3xl font-bold text-content-heading">{index === 0 ? 12 : index === 1 ? 3 : 1}</p>
            </div>
          ))}
        </div>
      </section>
    </PageFrame>
  )
}

function RouteModuleFallback() {
  return (
    <div className="flex min-h-[240px] items-center justify-center rounded-[6px] border border-line-default bg-white text-sm text-content-subtle">
      Loading workspace...
    </div>
  )
}

function PlaceRoute({ section, token, viewer }: PlaceResourcePageProps) {
  const { placeId } = useParams()
  const placeSection = section ?? "dashboard"

  if (placeSection === "dashboard") {
    return <PlaceDashboardAdaptedPage token={token} viewer={viewer} placeID={placeId} />
  }
  if (placeSection === "users") {
    return <UsersAdaptedPage token={token} viewer={viewer} placeID={placeId} placeScoped />
  }
  if (placeSection === "groups") {
    return <GroupsAdaptedPage token={token} viewer={viewer} placeID={placeId} placeScoped />
  }
  if (placeSection === "doors") {
    return <DoorDetailAdaptedPage token={token} viewer={viewer} placeID={placeId} />
  }
  if (placeSection === "floors") {
    return <FloorsAdaptedPage token={token} viewer={viewer} placeID={placeId} />
  }
  if (placeSection === "unlock-history") {
    return <EventHistoryAdaptedPage token={token} viewer={viewer} placeID={placeId} placeScoped />
  }
  if (placeSection === "analytics") {
    return <ReportsAdaptedPage token={token} viewer={viewer} placeID={placeId} placeScoped />
  }
  if (placeSection === "elevators") {
    return <ElevatorsAdaptedPage token={token} viewer={viewer} placeID={placeId} />
  }
  if (placeSection === "hardware") {
    return <HardwareAdaptedPage token={token} viewer={viewer} placeID={placeId} />
  }
  if (placeSection === "settings") {
    return <PlaceSettingsAdaptedPage token={token} viewer={viewer} placeID={placeId} />
  }

  return <PlaceOperationsAdaptedPage title={titleFromSegment(placeSection)} />
}

function OrganizationRoute({ token, viewer }: AuthenticatedResourcePageProps) {
  const { section } = useParams()
  const title = section === "sso-scim" ? "SSO & SCIM" : titleFromSegment(section ?? "settings")
  return <OrganizationSetupAdaptedPage title={title} token={token} viewer={viewer} />
}

function GenericRoute() {
  const location = useLocation()
  const segment = location.pathname.split("/").filter(Boolean).pop() ?? "workspace"
  const placeScoped = location.pathname.startsWith("/places/")
  return <GenericAdaptedPage title={titleFromSegment(segment)} placeScoped={placeScoped} />
}

export function MistyisletConsoleRoutes({ homeContent, token, viewer, onViewerChange, onLogout }: MistyisletConsoleRoutesProps) {
  return (
    <Suspense fallback={<RouteModuleFallback />}>
      <Routes>
        <Route path="/home" element={homeContent} />
        <Route
          path="/my-account"
          element={<MyAccountPage token={token} viewer={viewer} onViewerChange={onViewerChange} onLogout={onLogout} />}
        />
        <Route path="/users" element={<UsersAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/users/:userID" element={<UserDetailAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/teams" element={<TeamsAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/groups" element={<GroupsAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/access-rights" element={<AccessRightsAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/schedules" element={<SchedulesAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/holiday-calendars" element={<HolidayCalendarsAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/invitations" element={<InvitationsAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/credentials" element={<CredentialsAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/event-history" element={<EventHistoryAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/reports" element={<ReportSchedulePage token={token} viewer={viewer} />} />
        <Route path="/analytics" element={<AnalyticsPage token={token} viewer={viewer} />} />
        <Route path="/network" element={<NetworkTopologyPage token={token} viewer={viewer} />} />
        <Route path="/alarm-schedules" element={<AlarmSchedulePage token={token} viewer={viewer} />} />
        <Route path="/visitors" element={<VisitorsPage token={token} viewer={viewer} />} />
        <Route path="/bookings" element={<BookingsPage token={token} viewer={viewer} />} />
        <Route path="/places" element={<PlacesAdaptedPage token={token} viewer={viewer} />} />
        <Route path="/places/assigned" element={<Navigate to={`/places/${DEMO_PLACE_ID}/dashboard`} replace />} />
        <Route path="/places/assigned/:section" element={<AssignedPlaceRedirect />} />
        <Route path="/places/:placeId" element={<PlaceRoute token={token} viewer={viewer} />} />
        <Route path="/places/:placeId/:section" element={<PlaceSectionRoute token={token} viewer={viewer} />} />
        <Route path="/organization" element={<Navigate to="/organization/settings" replace />} />
        <Route path="/organization/:section" element={<OrganizationRoute token={token} viewer={viewer} />} />
        <Route path="/tenants" element={<TenantsPage token={token} />} />
        <Route path="/tenants/:tenantID" element={<TenantDetailPage token={token} />} />
        <Route path="/enterprise" element={<EnterprisePage token={token} viewer={viewer} />} />
        <Route path="/spaces" element={<SpacesPage token={token} viewer={viewer} />} />
        <Route path="/access" element={<Navigate to="/access/directory" replace />} />
        <Route path="/access/:section" element={<AccessLegacySectionRedirectPage />} />
        <Route path="/access/directory" element={<AccessDirectoryPage token={token} viewer={viewer} />} />
        <Route path="/access/policies" element={<AccessPoliciesPage token={token} viewer={viewer} />} />
        <Route path="/access/grants" element={<AccessGrantsPage token={token} viewer={viewer} />} />
        <Route path="/wallet" element={<WalletPage token={token} viewer={viewer} />} />
        <Route path="/cameras" element={<CamerasPage token={token} viewer={viewer} />} />
        <Route path="/gateways" element={<GatewaysPage token={token} viewer={viewer} />} />
        <Route path="/events" element={<EventsPage token={token} viewer={viewer} />} />
        <Route path="/alarms" element={<AlarmsPage token={token} viewer={viewer} />} />
        <Route path="/audit" element={<AuditPage token={token} />} />
        <Route path="/access-links/claim" element={<AccessLinkClaimPage />} />
        <Route path="/access-link/:token" element={<AccessLinkClaimPage />} />
        <Route path="/dashboard" element={<Navigate to="/home" replace />} />
        <Route path="*" element={<GenericRoute />} />
      </Routes>
    </Suspense>
  )
}

function PlaceSectionRoute(props: AuthenticatedResourcePageProps) {
  const { section } = useParams()
  return <PlaceRoute {...props} section={section} />
}

function AssignedPlaceRedirect() {
  const { section } = useParams()
  return <Navigate to={`/places/${DEMO_PLACE_ID}/${section ?? "dashboard"}`} replace />
}
