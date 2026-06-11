import { Suspense, lazy } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, Route, Routes, useLocation } from "react-router"

import { useAuth } from "@/context/auth-context"

const HomePage = lazy(() =>
  import("@/features/home/pages/home-page").then((module) => ({ default: module.HomePage }))
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
const KioskPage = lazy(() =>
  import("@/features/kiosk/pages/kiosk-page").then((module) => ({ default: module.KioskPage }))
)

function RouteFallback() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
      Loading...
    </div>
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

  if (location.pathname === "/login" || location.pathname === "/") {
    return <Navigate to="/home" replace />
  }

  // Kiosk mode renders full-screen without the admin shell (tablet self check-in).
  if (location.pathname === "/kiosk") {
    return (
      <Suspense fallback={<RouteFallback />}>
        <KioskPage token={token} viewer={viewer} />
      </Suspense>
    )
  }

  return (
    <Suspense fallback={<RouteFallback />}>
      <HomePage token={token} viewer={viewer} onViewerChange={updateViewer} onLogout={logout} />
    </Suspense>
  )
}
