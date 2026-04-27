import type { ReactNode } from "react"

import { AccessRightsAdaptedPage } from "@/features/access-rights/pages/access-rights-page"
import { MyAccountPage } from "@/features/account/pages/my-account-page"
import { CredentialsAdaptedPage } from "@/features/credentials/pages/credentials-page"
import { EventHistoryAdaptedPage } from "@/features/event-history/pages/event-history-page"
import { GroupsAdaptedPage } from "@/features/groups/pages/groups-page"
import { OrganizationSetupAdaptedPage } from "@/features/organization/pages/organization-setup-page"
import { DoorDetailAdaptedPage } from "@/features/places/pages/door-detail-page"
import { FloorsAdaptedPage } from "@/features/places/pages/floors-page"
import { HardwareAdaptedPage } from "@/features/places/pages/hardware-page"
import { PlaceDashboardAdaptedPage } from "@/features/places/pages/place-dashboard-page"
import { PlacesAdaptedPage } from "@/features/places/pages/places-list-page"
import { PlaceOperationsAdaptedPage } from "@/features/places/pages/place-operations-page"
import { PlaceSettingsAdaptedPage } from "@/features/places/pages/place-settings-page"
import { ReportsAdaptedPage } from "@/features/reports/pages/reports-page"
import { TeamsAdaptedPage } from "@/features/teams/pages/teams-page"
import { UserDetailAdaptedPage } from "@/features/users/pages/user-detail-page"
import { UsersAdaptedPage } from "@/features/users/pages/users-page"
import { PageFrame } from "@/components/kisi/primitives"
import type { CurrentUser } from "@/lib/api"

import { normalizePathname, parsePlaceRoute } from "./navigation"

function GenericAdaptedPage({ title, placeScoped = false }: { title: string; placeScoped?: boolean }) {
  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", "Assigned Place", title] : ["Home", title]}
      title={title}
      description="This workspace follows the same Mistyislet console layout while the detailed workflow is being adapted."
    >
      <section className="rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-base font-semibold text-[#17171c]">{title}</h2>
          <p className="mt-1 text-sm text-[#6f717c]">Primary actions, filters, and records will stay in one flat surface.</p>
        </div>
        <div className="grid gap-4 p-6 md:grid-cols-3">
          {["Active", "Pending", "Needs review"].map((item, index) => (
            <div key={item} className="rounded-[6px] border border-[#eceef2] p-4">
              <p className="text-sm text-[#6f717c]">{item}</p>
              <p className="mt-3 text-3xl font-bold text-[#17171c]">{index === 0 ? 12 : index === 1 ? 3 : 1}</p>
            </div>
          ))}
        </div>
      </section>
    </PageFrame>
  )
}

export function renderAdaptedPage({
  pathname,
  homeContent,
  token,
  viewer,
  onLogout,
}: {
  pathname: string
  homeContent: ReactNode
  token: string
  viewer: CurrentUser
  onLogout: () => void
}) {
  const current = normalizePathname(pathname)
  const placeRoute = parsePlaceRoute(current)
  const placeSection = placeRoute?.section
  const placeScoped = placeRoute !== null
  if (current === "/home") {
    return homeContent
  }
  if (current === "/my-account") {
    return <MyAccountPage viewer={viewer} onLogout={onLogout} />
  }
  if (current.startsWith("/users/")) {
    return <UserDetailAdaptedPage />
  }
  if (current === "/users" || placeSection === "users") {
    return <UsersAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} placeScoped={placeScoped} />
  }
  if (current === "/teams" || placeSection === "teams") {
    return <TeamsAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} placeScoped={placeScoped} />
  }
  if (current === "/groups" || placeSection === "groups") {
    return <GroupsAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} placeScoped={placeScoped} />
  }
  if (current === "/access-rights") {
    return <AccessRightsAdaptedPage token={token} viewer={viewer} />
  }
  if (current === "/credentials") {
    return <CredentialsAdaptedPage token={token} viewer={viewer} />
  }
  if (current === "/event-history" || placeSection === "unlock-history") {
    return <EventHistoryAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} placeScoped={placeScoped} />
  }
  if (current === "/reports" || placeSection === "analytics") {
    return <ReportsAdaptedPage placeScoped={placeScoped} />
  }
  if (current === "/places") {
    return <PlacesAdaptedPage token={token} viewer={viewer} />
  }
  if (placeSection === "dashboard") {
    return <PlaceDashboardAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} />
  }
  if (placeSection === "doors") {
    return <DoorDetailAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} />
  }
  if (placeSection === "floors") {
    return <FloorsAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} />
  }
  if (placeSection === "hardware") {
    return <HardwareAdaptedPage token={token} viewer={viewer} placeID={placeRoute?.placeId} />
  }
  if (placeSection === "settings") {
    return <PlaceSettingsAdaptedPage />
  }
  if (
    placeSection === "elevators" ||
    placeSection === "capacity-management" ||
    placeSection === "intrusion-detection" ||
    placeSection === "integrations"
  ) {
    const segment = placeSection ?? "workspace"
    const title = segment
      .split("-")
      .map((item) => item.charAt(0).toUpperCase() + item.slice(1))
      .join(" ")
    return <PlaceOperationsAdaptedPage title={title} />
  }
  if (current.startsWith("/organization/")) {
    const segment = current.split("/").filter(Boolean).pop() ?? "settings"
    const title =
      segment === "sso-scim"
        ? "SSO & SCIM"
        : segment
            .split("-")
            .map((item) => item.charAt(0).toUpperCase() + item.slice(1))
            .join(" ")
    return <OrganizationSetupAdaptedPage title={title} token={token} viewer={viewer} />
  }

  const segment = current.split("/").filter(Boolean).pop() ?? "workspace"
  const title = segment
    .split("-")
    .map((item) => item.charAt(0).toUpperCase() + item.slice(1))
    .join(" ")
  return <GenericAdaptedPage title={title} placeScoped={placeScoped} />
}
