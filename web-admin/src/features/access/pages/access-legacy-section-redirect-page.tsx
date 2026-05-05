import { Navigate, useLocation, useParams } from "react-router"

import { resolveAccessSection } from "@/components/access/access-page-utils"

export function AccessLegacySectionRedirectPage() {
  const { section } = useParams<{ section?: string }>()
  const location = useLocation()
  const normalizedSection = resolveAccessSection(section)

  return (
    <Navigate
      to={{
        pathname: `/access/${normalizedSection}`,
        search: location.search,
      }}
      replace
    />
  )
}
