import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createRollout, getRolloutDetail, listRollouts, rolloutAction, type CreateRolloutInput, type RolloutActionName } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"

export function useRolloutList(token: string | undefined, tenantID: string | undefined) {
  return useQuery({
    queryKey: queryKeys.rolloutList._key(tenantID ?? "self"),
    queryFn: () => listRollouts(token, tenantID),
    staleTime: 15 * 1000,
  })
}

const POLLING_STATES = ["active", "scheduled", "awaiting_approval", "paused"]
export function useRolloutDetail(token: string | undefined, tenantID: string | undefined, id: string) {
  return useQuery({
    queryKey: queryKeys.rolloutDetail._key(tenantID ?? "self", id),
    queryFn: () => getRolloutDetail(token, tenantID, id),
    refetchInterval: (query) => {
      const state = query.state.data?.rollout.state
      return state && POLLING_STATES.includes(state) ? 5000 : false
    },
  })
}

export function useCreateRolloutMutation(token: string | undefined, tenantID: string | undefined) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateRolloutInput) => createRollout(token, tenantID, input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: queryKeys.rolloutList._base }),
  })
}

export function useRolloutActionMutation(token: string | undefined, tenantID: string | undefined, id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (action: RolloutActionName) => rolloutAction(token, tenantID, id, action),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.rolloutDetail._base })
      void qc.invalidateQueries({ queryKey: queryKeys.rolloutList._base })
    },
  })
}
