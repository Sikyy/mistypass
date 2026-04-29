import { createContext, useContext, useMemo, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { useLocation, useNavigate } from "react-router-dom"

import { useAuth } from "@/context/auth-context"
import {
  DEMO_PLACE_ID,
  formatPlaceName,
  isPlaceAdminView,
  parsePlaceRoute,
  placePath,
} from "@/features/mistyislet-shell/navigation"
import { listBuildings, type CurrentUser } from "@/lib/api"
import { getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

export type NavigationView = "organization" | "place"

type NavigationContextValue = {
  currentView: NavigationView
  selectedPlaceID: string | null
  selectedPlaceName: string | null
  isCanonicalPlaceRoute: boolean
  enterPlace: (placeID?: string, section?: string) => void
  backToOrganization: () => void
}

const NavigationContext = createContext<NavigationContextValue | null>(null)

export function NavigationProvider({
  viewer,
  children,
}: {
  viewer: CurrentUser
  children: ReactNode
}) {
  const location = useLocation()
  const navigate = useNavigate()
  const { token } = useAuth()
  const placeRoute = parsePlaceRoute(location.pathname)
  const placeID =
    placeRoute?.placeId === "assigned" ? DEMO_PLACE_ID : placeRoute?.placeId ?? DEMO_PLACE_ID
  const currentView: NavigationView = isPlaceAdminView(viewer, location.pathname)
    ? "place"
    : "organization"
  const buildingsQuery = useQuery({
    queryKey: ["kisi-navigation-buildings", viewer.id, viewer.role, viewer.tenant_id],
    queryFn: () => listBuildings(token ?? undefined, isPlatformViewer(viewer) ? undefined : getViewerTenantID(viewer)),
    enabled: currentView === "place" && Boolean(token),
    staleTime: 30 * 1000,
  })
  const selectedPlaceName =
    buildingsQuery.data?.find((building) => building.id === placeID)?.name ?? formatPlaceName(placeID)

  const value = useMemo<NavigationContextValue>(
    () => ({
      currentView,
      selectedPlaceID: currentView === "place" ? placeID : null,
      selectedPlaceName: currentView === "place" ? selectedPlaceName : null,
      isCanonicalPlaceRoute: placeRoute !== null && placeRoute.placeId !== "assigned",
      enterPlace(nextPlaceID = placeID, section = "dashboard") {
        navigate(placePath(section, nextPlaceID))
      },
      backToOrganization() {
        navigate("/places")
      },
    }),
    [currentView, navigate, placeID, placeRoute, selectedPlaceName]
  )

  return <NavigationContext.Provider value={value}>{children}</NavigationContext.Provider>
}

export function useNavigationContext() {
  const context = useContext(NavigationContext)
  if (!context) {
    throw new Error("useNavigationContext must be used inside NavigationProvider")
  }
  return context
}
