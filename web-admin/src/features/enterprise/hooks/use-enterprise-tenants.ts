import { useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"

import {
  listTenants,
  type CurrentUser,
  type Tenant,
} from "@/lib/api"
import { getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

type UseEnterpriseTenantsParams = {
  token: string
  viewer: CurrentUser
  enterpriseRouteHintTenantID: string
}

export function useEnterpriseTenants({
  token,
  viewer,
  enterpriseRouteHintTenantID,
}: UseEnterpriseTenantsParams) {
  const platformViewer = isPlatformViewer(viewer)
  const viewerTenantID = getViewerTenantID(viewer)

  const [tenants, setTenants] = useState<Tenant[]>([])
  const [selectedTenantID, setSelectedTenantID] = useState(viewerTenantID)

  const tenantsQuery = useQuery({
    queryKey: ["enterprise-tenants", platformViewer ? "platform" : "tenant"],
    queryFn: () => (platformViewer ? listTenants(token) : Promise.resolve([])),
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!platformViewer) {
      setSelectedTenantID(viewerTenantID)
      return
    }

    const items = tenantsQuery.data ?? []
    setTenants(items)
    setSelectedTenantID((current) => {
      if (enterpriseRouteHintTenantID && items.some((item) => item.id === enterpriseRouteHintTenantID)) {
        return enterpriseRouteHintTenantID
      }
      return current || items[0]?.id || ""
    })
  }, [enterpriseRouteHintTenantID, platformViewer, tenantsQuery.data, viewerTenantID])

  const selectedTenant = useMemo(
    () => tenants.find((item) => item.id === selectedTenantID) ?? null,
    [selectedTenantID, tenants]
  )

  const tenantsLoading = platformViewer && tenantsQuery.isPending
  const tenantsError =
    tenantsQuery.error instanceof Error ? tenantsQuery.error.message : ""

  return {
    tenants,
    selectedTenantID,
    setSelectedTenantID,
    selectedTenant,
    platformViewer,
    viewerTenantID,
    tenantsLoading,
    tenantsError,
  }
}
