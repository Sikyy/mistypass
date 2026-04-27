import { useQuery } from "@tanstack/react-query"

import type { CurrentUser } from "@/lib/api"

import {
  fallbackKisiResourceSummary,
  loadKisiResourceSummary,
  selectKisiPlaceContext,
  type KisiResourceSummary,
} from "./resource-data"

export function useKisiResourceSummary(token: string, viewer: CurrentUser) {
  const query = useQuery({
    queryKey: [
      "kisi-resource-summary",
      viewer.id,
      viewer.role,
      viewer.tenant_id,
      viewer.building_ids?.slice().sort().join(",") ?? "",
    ],
    queryFn: () => loadKisiResourceSummary(token, viewer),
    staleTime: 30 * 1000,
  })
  const summary = query.data ?? fallbackKisiResourceSummary

  return {
    ...query,
    summary,
    usingFallback: !query.data,
  }
}

export function useKisiPlaceContext(token: string, viewer: CurrentUser, placeID?: string | null) {
  const query = useKisiResourceSummary(token, viewer)
  const context = selectKisiPlaceContext(query.summary as KisiResourceSummary, placeID)

  return {
    ...query,
    context,
  }
}
