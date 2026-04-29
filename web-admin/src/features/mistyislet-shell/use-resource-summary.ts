import { useQuery } from "@tanstack/react-query"

import type { CurrentUser } from "@/lib/api"

import {
  fallbackMistyisletResourceSummary,
  loadMistyisletResourceSummary,
  selectMistyisletPlaceContext,
  type MistyisletResourceSummary,
} from "./resource-data"

export function useMistyisletResourceSummary(token: string, viewer: CurrentUser) {
  const query = useQuery({
    queryKey: [
      "kisi-resource-summary",
      viewer.id,
      viewer.role,
      viewer.tenant_id,
      viewer.building_ids?.slice().sort().join(",") ?? "",
    ],
    queryFn: () => loadMistyisletResourceSummary(token, viewer),
    staleTime: 30 * 1000,
    refetchInterval: 15 * 1000,
  })
  const summary = query.data ?? fallbackMistyisletResourceSummary

  return {
    ...query,
    summary,
    usingFallback: !query.data,
  }
}

export function useMistyisletPlaceContext(token: string, viewer: CurrentUser, placeID?: string | null) {
  const query = useMistyisletResourceSummary(token, viewer)
  const context = selectMistyisletPlaceContext(query.summary as MistyisletResourceSummary, placeID)

  return {
    ...query,
    context,
  }
}
