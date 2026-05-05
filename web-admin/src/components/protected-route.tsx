import { type ReactNode } from "react"
import { Navigate } from "react-router"

type ProtectedRouteProps = {
  allow: boolean
  children: ReactNode
  redirectTo?: string
}

export function ProtectedRoute({ allow, children, redirectTo = "/dashboard" }: ProtectedRouteProps) {
  if (!allow) {
    return <Navigate to={redirectTo} replace />
  }
  return <>{children}</>
}
