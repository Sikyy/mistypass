import { useQuery } from "@tanstack/react-query"
import { getFirmwareSummary, listFirmware } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"

export function useFirmwareSummary(token: string | undefined, tenantID: string | undefined) {
  return useQuery({
    queryKey: queryKeys.firmwareSummary._key(tenantID ?? "self"),
    queryFn: () => getFirmwareSummary(token, tenantID),
    staleTime: 30 * 1000,
  })
}

export function useFirmwareList(token: string | undefined, tenantID: string | undefined, channel: string) {
  return useQuery({
    queryKey: queryKeys.firmwareList._key(tenantID ?? "self", channel || "all"),
    queryFn: () => listFirmware(token, tenantID, channel || undefined),
    staleTime: 30 * 1000,
  })
}
