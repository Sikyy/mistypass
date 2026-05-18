import { useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import {
  createEnterpriseHRISConnector,
  getEnterpriseHRISWebhookExecution,
  listEnterpriseEmployees,
  listEnterpriseHRISWebhookExecutions,
  listEnterpriseHRISWebhookReceipts,
  listEnterpriseHRISWebhookDLQ,
  listEnterpriseSyncJobs,
  processEnterpriseHRISWebhookReceipt,
  processBatchEnterpriseHRISWebhookReceipts,
  reconcilePendingEnterpriseSyncRequests,
  replayBatchEnterpriseHRISWebhookDLQ,
  replayEnterpriseHRISWebhookExecution,
  replayEnterpriseHRISWebhookDLQ,
  syncEnterpriseEmployees,
  updateEnterpriseHRISConnector,
  type CurrentUser,
  type EmployeeSyncInput,
  type EnterpriseEmployee,
  type EnterpriseHRISConnector,
  type EnterpriseHRISWebhookExecution,
  type EnterpriseHRISWebhookExecutionStatusCounts,
  type EnterpriseHRISWebhookDLQBatchReplayResult,
  type EnterpriseHRISWebhookReceipt,
  type EnterpriseHRISWebhookReceiptBatchProcessResult,
  type EnterpriseHRISWebhookDLQEntry,
  type EnterpriseHRISWebhookRuntimeCounts,
  type EnterpriseSyncRequestRecord,
  type EnterpriseSyncJob,
} from "@/lib/api"
import { type TalentaConnectorSaveInput } from "@/components/enterprise/enterprise-hris-connector-panel"
import { canManageEnterprise } from "@/lib/viewer"

// ── Filter types ────────────────────────────────────────────────────────────

export type HRISWebhookExecutionKindFilter = "all" | "receipt_process" | "dlq_replay"
export type HRISWebhookExecutionStatusFilter = "all" | "queued" | "running" | "succeeded" | "failed"
export type HRISWebhookExecutionReplayScopeFilter = "all" | "replayed" | "worker_required"
export type HRISWebhookExecutionModeFilter = "all" | "inline" | "queued"
export type HRISWebhookExecutionDispatchFilter = "all" | "worker_tick" | "worker_task_channel" | "goroutine_fallback"
export type HRISWebhookExecutionQueueStateFilter = "all" | "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"

// ── Constants ───────────────────────────────────────────────────────────────

const hrisWebhookReceiptPageSize = 12
const hrisWebhookDLQPageSize = 12
const hrisWebhookExecutionPageSize = 12

const emptyHRISWebhookRuntimeCounts: EnterpriseHRISWebhookRuntimeCounts = {
  all: 0,
  ready: 0,
  cooldown: 0,
  in_flight: 0,
  attempt_limit: 0,
  terminal: 0,
}

const emptyHRISWebhookExecutionStatusCounts: EnterpriseHRISWebhookExecutionStatusCounts = {
  all: 0,
  queued: 0,
  running: 0,
  succeeded: 0,
  failed: 0,
}

const sampleSyncPayload = `[
  {
    "external_id": "emp-001",
    "email": "employee@company.local",
    "full_name": "Alex Chen",
    "department": "Finance",
    "job_title": "Finance Manager",
    "location": "HQ / 18F",
    "status": "active"
  }
]`

// ── Helpers ─────────────────────────────────────────────────────────────────

function buildHRISWebhookExecutionListOptions(args: {
  tenantID: string
  kind: HRISWebhookExecutionKindFilter
  status: HRISWebhookExecutionStatusFilter
  queueState: HRISWebhookExecutionQueueStateFilter
  replayScope: HRISWebhookExecutionReplayScopeFilter
  executionMode: HRISWebhookExecutionModeFilter
  dispatchMode: HRISWebhookExecutionDispatchFilter
  targetStatus: string
  query: string
  limit: number
  offset?: number
}) {
  const options: Parameters<typeof listEnterpriseHRISWebhookExecutions>[1] = {
    tenant_id: args.tenantID,
    q: args.query.trim() || undefined,
    limit: args.limit,
    offset: args.offset ?? 0,
  }
  if (args.kind !== "all") {
    options.kind = args.kind
  }
  if (args.status !== "all") {
    options.status = args.status
  }
  if (args.queueState !== "all") {
    options.queue_state = args.queueState
  }
  if (args.replayScope !== "all") {
    options.replay_scope = args.replayScope
  }
  if (args.executionMode !== "all") {
    options.execution_mode = args.executionMode
  }
  if (args.dispatchMode !== "all") {
    options.dispatch_mode = args.dispatchMode
  }
  if (args.targetStatus.trim()) {
    options.target_status = args.targetStatus.trim().toLowerCase()
  }
  return options
}

function normalizeHRISWebhookRuntimeCounts(value: unknown): EnterpriseHRISWebhookRuntimeCounts | null {
  if (!value || typeof value !== "object") {
    return null
  }
  const source = value as Record<string, unknown>
  let hasNumericCount = false
  const result = { ...emptyHRISWebhookRuntimeCounts }
  ;(Object.keys(result) as Array<keyof EnterpriseHRISWebhookRuntimeCounts>).forEach((key) => {
    const count = source[key]
    if (typeof count === "number" && Number.isFinite(count) && count >= 0) {
      result[key] = Math.floor(count)
      hasNumericCount = true
    }
  })
  return hasNumericCount ? result : null
}

function extractHRISWebhookRuntimeCounts(...sources: unknown[]): EnterpriseHRISWebhookRuntimeCounts | null {
  for (const source of sources) {
    const counts = normalizeHRISWebhookRuntimeCounts(source)
    if (counts) {
      return counts
    }
  }
  return null
}

function formatDispatchModeLabel(value?: string) {
  const normalized = value?.trim()
  if (!normalized) {
    return "-"
  }
  return normalized
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((token) => token.charAt(0).toUpperCase() + token.slice(1))
    .join(" ")
}

function normalizeWebhookQueueActionError(message: string, t: ReturnType<typeof useTranslation>["t"]) {
  const normalized = message.trim().toLowerCase()
  if (!normalized) {
    return message
  }
  if (normalized.includes("require_worker=true requires enabled receipt worker")) {
    return t("enterprisePage.errors.webhookReceiptWorkerRequired")
  }
  if (normalized.includes("require_worker=true requires enabled dlq worker")) {
    return t("enterprisePage.errors.webhookDLQWorkerRequired")
  }
  return message
}

function syncSourceFailureHint(t: ReturnType<typeof useTranslation>["t"], source: string, rejected: number) {
  if (source === "scim") {
    return t("enterprisePage.syncResult.failure.scim", { rejected })
  }
  if (source === "csv_import") {
    return t("enterprisePage.syncResult.failure.csvImport", { rejected })
  }
  if (source === "manual_sync") {
    return t("enterprisePage.syncResult.failure.manual", { rejected })
  }
  return t("enterprisePage.syncResult.failure.hris", { rejected })
}

function syncSourceSuccessHint(t: ReturnType<typeof useTranslation>["t"], source: string, deactivated: number) {
  if (source === "scim" && deactivated > 0) {
    return t("enterprisePage.syncResult.success.scimDeactivated", { deactivated })
  }
  if (source === "csv_import") {
    return t("enterprisePage.syncResult.success.csvImport")
  }
  if (source === "manual_sync") {
    return t("enterprisePage.syncResult.success.manual")
  }
  return t("enterprisePage.syncResult.success.default")
}

// ── Enterprise data loader type ─────────────────────────────────────────────

type EnterpriseSyncDataSlice = {
  syncJobs: EnterpriseSyncJob[]
  syncRequests: EnterpriseSyncRequestRecord[]
}

// ── Hook params ─────────────────────────────────────────────────────────────

type UseEnterpriseSyncParams = {
  token: string
  viewer: CurrentUser
  selectedTenantID: string
  selectedTenantName: string | undefined
  enterpriseData: (EnterpriseSyncDataSlice & { hrisConnectors: EnterpriseHRISConnector[] }) | undefined
  enterpriseRouteHintExecutionID: string
  reloadEnterpriseData: (tenantID: string) => Promise<void>
  setEmployees: (employees: EnterpriseEmployee[]) => void
  setHRISConnectors: (connectors: EnterpriseHRISConnector[]) => void
}

export function useEnterpriseSync({
  token,
  viewer,
  selectedTenantID,
  selectedTenantName,
  enterpriseData,
  enterpriseRouteHintExecutionID,
  reloadEnterpriseData,
  setEmployees,
  setHRISConnectors,
}: UseEnterpriseSyncParams) {
  const { t } = useTranslation()
  const writable = canManageEnterprise(viewer)

  // ── Core sync state ─────────────────────────────────────────────────────

  const [syncJobs, setSyncJobs] = useState<EnterpriseSyncJob[]>([])
  const [syncRequests, setSyncRequests] = useState<EnterpriseSyncRequestRecord[]>([])
  const [syncSource, setSyncSource] = useState("hris")
  const [syncRequestID, setSyncRequestID] = useState(() => `sync-${Date.now()}`)
  const [syncPayload, setSyncPayload] = useState(sampleSyncPayload)
  const [syncSummary, setSyncSummary] = useState("")
  const [syncing, setSyncing] = useState(false)
  const [error, setError] = useState("")

  // ── HRIS webhook executions state ─────────────────────────────────────

  const [hrisWebhookExecutions, setHRISWebhookExecutions] = useState<EnterpriseHRISWebhookExecution[]>([])
  const [hrisWebhookExecutionStatusCounts, setHRISWebhookExecutionStatusCounts] =
    useState<EnterpriseHRISWebhookExecutionStatusCounts>(emptyHRISWebhookExecutionStatusCounts)
  const [hrisWebhookExecutionQueueCounts, setHRISWebhookExecutionQueueCounts] =
    useState<EnterpriseHRISWebhookRuntimeCounts | null>(null)
  const [hrisWebhookExecutionTotal, setHRISWebhookExecutionTotal] = useState(0)
  const [hrisWebhookExecutionHasMore, setHRISWebhookExecutionHasMore] = useState(false)
  const [hrisWebhookExecutionNextOffset, setHRISWebhookExecutionNextOffset] = useState<number | null>(null)
  const [hrisWebhookExecutionKindFilter, setHRISWebhookExecutionKindFilter] =
    useState<HRISWebhookExecutionKindFilter>("all")
  const [hrisWebhookExecutionStatusFilter, setHRISWebhookExecutionStatusFilter] =
    useState<HRISWebhookExecutionStatusFilter>("all")
  const [hrisWebhookExecutionQueueStateFilter, setHRISWebhookExecutionQueueStateFilter] =
    useState<HRISWebhookExecutionQueueStateFilter>("all")
  const [hrisWebhookExecutionReplayScopeFilter, setHRISWebhookExecutionReplayScopeFilter] =
    useState<HRISWebhookExecutionReplayScopeFilter>("all")
  const [hrisWebhookExecutionModeFilter, setHRISWebhookExecutionModeFilter] =
    useState<HRISWebhookExecutionModeFilter>("all")
  const [hrisWebhookExecutionDispatchFilter, setHRISWebhookExecutionDispatchFilter] =
    useState<HRISWebhookExecutionDispatchFilter>("all")
  const [hrisWebhookExecutionTargetStatusFilter, setHRISWebhookExecutionTargetStatusFilter] = useState("")
  const [hrisWebhookExecutionQuery, setHRISWebhookExecutionQuery] = useState("")
  const [hrisWebhookExecutionListLoading, setHRISWebhookExecutionListLoading] = useState(false)
  const [hrisWebhookExecutionListLoadingMore, setHRISWebhookExecutionListLoadingMore] = useState(false)
  const [selectedHRISWebhookExecution, setSelectedHRISWebhookExecution] =
    useState<EnterpriseHRISWebhookExecution | null>(null)
  const [selectedHRISWebhookExecutionLoading, setSelectedHRISWebhookExecutionLoading] = useState(false)
  const [selectedHRISWebhookExecutionError, setSelectedHRISWebhookExecutionError] = useState("")

  // ── HRIS webhook receipts state ───────────────────────────────────────

  const [hrisWebhookReceipts, setHRISWebhookReceipts] = useState<EnterpriseHRISWebhookReceipt[]>([])
  const [hrisWebhookReceiptQueueCounts, setHRISWebhookReceiptQueueCounts] =
    useState<EnterpriseHRISWebhookRuntimeCounts | null>(null)
  const [hrisWebhookReceiptTotal, setHRISWebhookReceiptTotal] = useState(0)
  const [hrisWebhookReceiptHasMore, setHRISWebhookReceiptHasMore] = useState(false)
  const [hrisWebhookReceiptNextOffset, setHRISWebhookReceiptNextOffset] = useState<number | null>(null)
  const [hrisWebhookReceiptListLoading, setHRISWebhookReceiptListLoading] = useState(false)
  const [hrisWebhookReceiptListLoadingMore, setHRISWebhookReceiptListLoadingMore] = useState(false)

  // ── HRIS webhook DLQ state ────────────────────────────────────────────

  const [hrisWebhookDLQEntries, setHRISWebhookDLQEntries] = useState<EnterpriseHRISWebhookDLQEntry[]>([])
  const [hrisWebhookDLQReplayCounts, setHRISWebhookDLQReplayCounts] =
    useState<EnterpriseHRISWebhookRuntimeCounts | null>(null)
  const [hrisWebhookDLQTotal, setHRISWebhookDLQTotal] = useState(0)
  const [hrisWebhookDLQHasMore, setHRISWebhookDLQHasMore] = useState(false)
  const [hrisWebhookDLQNextOffset, setHRISWebhookDLQNextOffset] = useState<number | null>(null)
  const [hrisWebhookDLQListLoading, setHRISWebhookDLQListLoading] = useState(false)
  const [hrisWebhookDLQListLoadingMore, setHRISWebhookDLQListLoadingMore] = useState(false)

  // ── Action ID state (for busy indicators) ─────────────────────────────

  const [dlqActionID, setDLQActionID] = useState<string | null>(null)
  const [executionActionID, setExecutionActionID] = useState<string | null>(null)
  const [receiptActionID, setReceiptActionID] = useState<string | null>(null)
  const [syncRequestActionID, setSyncRequestActionID] = useState<string | null>(null)
  const [latestWebhookReceiptBatchProcessResult, setLatestWebhookReceiptBatchProcessResult] =
    useState<EnterpriseHRISWebhookReceiptBatchProcessResult | null>(null)
  const [latestWebhookDLQBatchReplayResult, setLatestWebhookDLQBatchReplayResult] =
    useState<EnterpriseHRISWebhookDLQBatchReplayResult | null>(null)

  // ── Derived data ──────────────────────────────────────────────────────

  const sortedSyncJobs = useMemo(
    () =>
      [...syncJobs].sort((left, right) => {
        const leftTime = new Date(left.ended_at || left.started_at).getTime() || 0
        const rightTime = new Date(right.ended_at || right.started_at).getTime() || 0
        return rightTime - leftTime
      }),
    [syncJobs]
  )
  const latestSyncJob = sortedSyncJobs[0] ?? null
  const failedSyncJobCount = useMemo(
    () => syncJobs.filter((item) => item.status !== "completed" || item.rejected > 0).length,
    [syncJobs]
  )

  const selectedHRISWebhookExecutionID = enterpriseRouteHintExecutionID || null
  const selectedHRISWebhookExecutionListItem = useMemo(
    () =>
      selectedHRISWebhookExecutionID
        ? hrisWebhookExecutions.find((item) => item.id === selectedHRISWebhookExecutionID) ?? null
        : null,
    [hrisWebhookExecutions, selectedHRISWebhookExecutionID]
  )
  const selectedHRISWebhookExecutionItem = useMemo(() => {
    if (selectedHRISWebhookExecution?.id === selectedHRISWebhookExecutionID) {
      return selectedHRISWebhookExecution
    }
    return selectedHRISWebhookExecutionListItem
  }, [selectedHRISWebhookExecution, selectedHRISWebhookExecutionID, selectedHRISWebhookExecutionListItem])

  // ── Fetch helpers ─────────────────────────────────────────────────────

  const fetchHRISWebhookReceiptsPage = (tenantID: string, offset = 0) =>
    listEnterpriseHRISWebhookReceipts(token, {
      tenant_id: tenantID,
      limit: hrisWebhookReceiptPageSize,
      offset,
    })

  const fetchHRISWebhookDLQPage = (tenantID: string, offset = 0) =>
    listEnterpriseHRISWebhookDLQ(token, {
      tenant_id: tenantID,
      limit: hrisWebhookDLQPageSize,
      offset,
    })

  const fetchHRISWebhookExecutionsPage = (tenantID: string, offset = 0) =>
    listEnterpriseHRISWebhookExecutions(
      token,
      buildHRISWebhookExecutionListOptions({
        tenantID,
        kind: hrisWebhookExecutionKindFilter,
        status: hrisWebhookExecutionStatusFilter,
        queueState: hrisWebhookExecutionQueueStateFilter,
        replayScope: hrisWebhookExecutionReplayScopeFilter,
        executionMode: hrisWebhookExecutionModeFilter,
        dispatchMode: hrisWebhookExecutionDispatchFilter,
        targetStatus: hrisWebhookExecutionTargetStatusFilter,
        query: hrisWebhookExecutionQuery,
        limit: hrisWebhookExecutionPageSize,
        offset,
      })
    )

  const applyHRISWebhookReceiptPage = (
    response: Awaited<ReturnType<typeof listEnterpriseHRISWebhookReceipts>>,
    mode: "replace" | "append" = "replace"
  ) => {
    setHRISWebhookReceipts((current) => (mode === "append" ? [...current, ...response.items] : response.items))
    setHRISWebhookReceiptQueueCounts(
      extractHRISWebhookRuntimeCounts(response.queue_counts, response.filter_counts, response.summary)
    )
    setHRISWebhookReceiptTotal(response.total)
    setHRISWebhookReceiptHasMore(response.has_more)
    setHRISWebhookReceiptNextOffset(response.next_offset ?? null)
  }

  const applyHRISWebhookExecutionPage = (
    response: Awaited<ReturnType<typeof listEnterpriseHRISWebhookExecutions>>,
    mode: "replace" | "append" = "replace"
  ) => {
    setHRISWebhookExecutions((current) => (mode === "append" ? [...current, ...response.items] : response.items))
    setHRISWebhookExecutionStatusCounts(response.status_counts ?? emptyHRISWebhookExecutionStatusCounts)
    setHRISWebhookExecutionQueueCounts(extractHRISWebhookRuntimeCounts(response.queue_counts))
    setHRISWebhookExecutionTotal(response.total)
    setHRISWebhookExecutionHasMore(response.has_more)
    setHRISWebhookExecutionNextOffset(response.next_offset ?? null)
  }

  const applyHRISWebhookDLQPage = (
    response: Awaited<ReturnType<typeof listEnterpriseHRISWebhookDLQ>>,
    mode: "replace" | "append" = "replace"
  ) => {
    setHRISWebhookDLQEntries((current) => (mode === "append" ? [...current, ...response.items] : response.items))
    setHRISWebhookDLQReplayCounts(
      extractHRISWebhookRuntimeCounts(response.replay_counts, response.filter_counts, response.summary)
    )
    setHRISWebhookDLQTotal(response.total)
    setHRISWebhookDLQHasMore(response.has_more)
    setHRISWebhookDLQNextOffset(response.next_offset ?? null)
  }

  // ── Effects: load enterprise data slice ───────────────────────────────

  useEffect(() => {
    const effectiveTenantID = selectedTenantID.trim()
    if (!effectiveTenantID) {
      setSyncJobs([])
      setSyncRequests([])
      setHRISWebhookExecutions([])
      setHRISWebhookExecutionStatusCounts(emptyHRISWebhookExecutionStatusCounts)
      setHRISWebhookExecutionQueueCounts(null)
      setHRISWebhookExecutionTotal(0)
      setHRISWebhookExecutionHasMore(false)
      setHRISWebhookExecutionNextOffset(null)
      setSelectedHRISWebhookExecution(null)
      setSelectedHRISWebhookExecutionLoading(false)
      setSelectedHRISWebhookExecutionError("")
      setHRISWebhookReceipts([])
      setHRISWebhookReceiptQueueCounts(null)
      setHRISWebhookReceiptTotal(0)
      setHRISWebhookReceiptHasMore(false)
      setHRISWebhookReceiptNextOffset(null)
      setHRISWebhookDLQEntries([])
      setHRISWebhookDLQReplayCounts(null)
      setHRISWebhookDLQTotal(0)
      setHRISWebhookDLQHasMore(false)
      setHRISWebhookDLQNextOffset(null)
      setHRISWebhookExecutionListLoading(false)
      setHRISWebhookExecutionListLoadingMore(false)
      setHRISWebhookReceiptListLoading(false)
      setHRISWebhookReceiptListLoadingMore(false)
      setHRISWebhookDLQListLoading(false)
      setHRISWebhookDLQListLoadingMore(false)
      return
    }

    if (!enterpriseData) {
      return
    }

    setSyncJobs(enterpriseData.syncJobs)
    setSyncRequests(enterpriseData.syncRequests)
  }, [enterpriseData, selectedTenantID])

  useEffect(() => {
    setLatestWebhookReceiptBatchProcessResult(null)
    setLatestWebhookDLQBatchReplayResult(null)
  }, [selectedTenantID])

  // ── Effect: load HRIS webhook executions ──────────────────────────────

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setHRISWebhookExecutionListLoading(true)
    setError("")
    void fetchHRISWebhookExecutionsPage(tenantID)
      .then((response) => {
        if (cancelled) {
          return
        }
        applyHRISWebhookExecutionPage(response)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookExecutionsFailed")
        setError(message)
        setHRISWebhookExecutions([])
        setHRISWebhookExecutionStatusCounts(emptyHRISWebhookExecutionStatusCounts)
        setHRISWebhookExecutionQueueCounts(null)
        setHRISWebhookExecutionTotal(0)
        setHRISWebhookExecutionHasMore(false)
        setHRISWebhookExecutionNextOffset(null)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setHRISWebhookExecutionListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [
    hrisWebhookExecutionDispatchFilter,
    hrisWebhookExecutionKindFilter,
    hrisWebhookExecutionModeFilter,
    hrisWebhookExecutionQuery,
    hrisWebhookExecutionQueueStateFilter,
    hrisWebhookExecutionReplayScopeFilter,
    hrisWebhookExecutionStatusFilter,
    hrisWebhookExecutionTargetStatusFilter,
    selectedTenantID,
    t,
    token,
  ])

  // ── Effect: load selected webhook execution detail ────────────────────

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    const executionID = enterpriseRouteHintExecutionID.trim()
    if (!tenantID || !executionID) {
      setSelectedHRISWebhookExecution(null)
      setSelectedHRISWebhookExecutionLoading(false)
      setSelectedHRISWebhookExecutionError("")
      return
    }
    let cancelled = false
    setSelectedHRISWebhookExecutionLoading(true)
    setSelectedHRISWebhookExecutionError("")
    void getEnterpriseHRISWebhookExecution(token, executionID, {
      tenant_id: tenantID,
    })
      .then((response) => {
        if (cancelled) {
          return
        }
        setSelectedHRISWebhookExecution(response.item)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookExecutionDetailFailed")
        setSelectedHRISWebhookExecution(null)
        setSelectedHRISWebhookExecutionError(message)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setSelectedHRISWebhookExecutionLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [enterpriseRouteHintExecutionID, selectedTenantID, t, token])

  // ── Effect: load HRIS webhook receipts ────────────────────────────────

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setHRISWebhookReceiptListLoading(true)
    setError("")
    void fetchHRISWebhookReceiptsPage(tenantID)
      .then((response) => {
        if (cancelled) {
          return
        }
        applyHRISWebhookReceiptPage(response)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookReceiptsFailed")
        setError(message)
        setHRISWebhookReceipts([])
        setHRISWebhookReceiptQueueCounts(null)
        setHRISWebhookReceiptTotal(0)
        setHRISWebhookReceiptHasMore(false)
        setHRISWebhookReceiptNextOffset(null)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setHRISWebhookReceiptListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [selectedTenantID, t, token])

  // ── Effect: load HRIS webhook DLQ ─────────────────────────────────────

  useEffect(() => {
    const tenantID = selectedTenantID.trim()
    if (!tenantID) {
      return
    }
    let cancelled = false
    setHRISWebhookDLQListLoading(true)
    setError("")
    void fetchHRISWebhookDLQPage(tenantID)
      .then((response) => {
        if (cancelled) {
          return
        }
        applyHRISWebhookDLQPage(response)
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof Error
            ? err.message
            : t("enterprisePage.errors.loadWebhookDLQFailed")
        setError(message)
        setHRISWebhookDLQEntries([])
        setHRISWebhookDLQReplayCounts(null)
        setHRISWebhookDLQTotal(0)
        setHRISWebhookDLQHasMore(false)
        setHRISWebhookDLQNextOffset(null)
      })
      .finally(() => {
        if (cancelled) {
          return
        }
        setHRISWebhookDLQListLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [selectedTenantID, t, token])

  // ── Reload helpers ────────────────────────────────────────────────────

  async function reloadHRISWebhookExecutions(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setHRISWebhookExecutionListLoading(true)
    try {
      const response = await fetchHRISWebhookExecutionsPage(effectiveTenantID)
      applyHRISWebhookExecutionPage(response)
    } finally {
      setHRISWebhookExecutionListLoading(false)
    }
  }

  async function reloadHRISWebhookReceipts(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setHRISWebhookReceiptListLoading(true)
    try {
      const response = await fetchHRISWebhookReceiptsPage(effectiveTenantID)
      applyHRISWebhookReceiptPage(response)
    } finally {
      setHRISWebhookReceiptListLoading(false)
    }
  }

  async function reloadHRISWebhookDLQ(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    setHRISWebhookDLQListLoading(true)
    try {
      const response = await fetchHRISWebhookDLQPage(effectiveTenantID)
      applyHRISWebhookDLQPage(response)
    } finally {
      setHRISWebhookDLQListLoading(false)
    }
  }

  // ── Load more handlers ────────────────────────────────────────────────

  async function onLoadMoreHRISWebhookExecutions() {
    const tenantID = selectedTenantID.trim()
    const nextOffset = hrisWebhookExecutionNextOffset ?? hrisWebhookExecutions.length
    if (
      !tenantID ||
      hrisWebhookExecutionListLoading ||
      hrisWebhookExecutionListLoadingMore ||
      !hrisWebhookExecutionHasMore ||
      nextOffset < 0
    ) {
      return
    }
    setHRISWebhookExecutionListLoadingMore(true)
    setError("")
    try {
      const response = await fetchHRISWebhookExecutionsPage(tenantID, nextOffset)
      applyHRISWebhookExecutionPage(response, "append")
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWebhookExecutionsFailed")
      setError(message)
    } finally {
      setHRISWebhookExecutionListLoadingMore(false)
    }
  }

  async function onLoadMoreHRISWebhookReceipts() {
    const tenantID = selectedTenantID.trim()
    const nextOffset = hrisWebhookReceiptNextOffset ?? hrisWebhookReceipts.length
    if (
      !tenantID ||
      hrisWebhookReceiptListLoading ||
      hrisWebhookReceiptListLoadingMore ||
      !hrisWebhookReceiptHasMore ||
      nextOffset < 0
    ) {
      return
    }
    setHRISWebhookReceiptListLoadingMore(true)
    setError("")
    try {
      const response = await fetchHRISWebhookReceiptsPage(tenantID, nextOffset)
      applyHRISWebhookReceiptPage(response, "append")
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWebhookReceiptsFailed")
      setError(message)
    } finally {
      setHRISWebhookReceiptListLoadingMore(false)
    }
  }

  async function onLoadMoreHRISWebhookDLQ() {
    const tenantID = selectedTenantID.trim()
    const nextOffset = hrisWebhookDLQNextOffset ?? hrisWebhookDLQEntries.length
    if (
      !tenantID ||
      hrisWebhookDLQListLoading ||
      hrisWebhookDLQListLoadingMore ||
      !hrisWebhookDLQHasMore ||
      nextOffset < 0
    ) {
      return
    }
    setHRISWebhookDLQListLoadingMore(true)
    setError("")
    try {
      const response = await fetchHRISWebhookDLQPage(tenantID, nextOffset)
      applyHRISWebhookDLQPage(response, "append")
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("enterprisePage.errors.loadWebhookDLQFailed")
      setError(message)
    } finally {
      setHRISWebhookDLQListLoadingMore(false)
    }
  }

  // ── View change handlers ──────────────────────────────────────────────

  function onHRISWebhookExecutionHistoryViewChange(input: {
    kind: HRISWebhookExecutionKindFilter
    status: HRISWebhookExecutionStatusFilter
    queueState: HRISWebhookExecutionQueueStateFilter
    replayScope: HRISWebhookExecutionReplayScopeFilter
    executionMode: HRISWebhookExecutionModeFilter
    dispatchMode: HRISWebhookExecutionDispatchFilter
    targetStatus: string
    query: string
  }) {
    setHRISWebhookExecutionKindFilter((current) => (current === input.kind ? current : input.kind))
    setHRISWebhookExecutionStatusFilter((current) => (current === input.status ? current : input.status))
    setHRISWebhookExecutionQueueStateFilter((current) =>
      current === input.queueState ? current : input.queueState
    )
    setHRISWebhookExecutionReplayScopeFilter((current) =>
      current === input.replayScope ? current : input.replayScope
    )
    setHRISWebhookExecutionModeFilter((current) => (current === input.executionMode ? current : input.executionMode))
    setHRISWebhookExecutionDispatchFilter((current) =>
      current === input.dispatchMode ? current : input.dispatchMode
    )
    setHRISWebhookExecutionTargetStatusFilter((current) =>
      current === input.targetStatus ? current : input.targetStatus
    )
    setHRISWebhookExecutionQuery((current) => (current === input.query ? current : input.query))
  }

  // ── Mutation handlers ─────────────────────────────────────────────────

  async function onSyncEmployees(payload: { source: string; requestID: string; payload: string }) {
    if (!writable || !selectedTenantID.trim()) {
      return
    }

    const source = payload.source.trim()
    const requestID = payload.requestID.trim()
    const rawPayload = payload.payload

    setSyncSource(source)
    setSyncRequestID(requestID)
    setSyncPayload(rawPayload)

    let employeesToSync: EmployeeSyncInput[]
    try {
      const parsed = JSON.parse(rawPayload) as EmployeeSyncInput[]
      employeesToSync = Array.isArray(parsed) ? parsed : []
    } catch {
      setError(t("enterprisePage.errors.invalidSyncPayload"))
      return
    }

    if (employeesToSync.length === 0) {
      setError(t("enterprisePage.errors.syncAtLeastOne"))
      return
    }

    setSyncing(true)
    setError("")
    setSyncSummary("")
    try {
      const result = await syncEnterpriseEmployees(token, {
        tenant_id: selectedTenantID.trim(),
        source,
        actor: viewer.email,
        request_id: requestID || undefined,
        employees: employeesToSync,
      })
      setSyncSummary(
        t("enterprisePage.syncSummary.submitted", {
          jobID: result.job.id,
          created: result.job.created,
          updated: result.job.updated,
          accessSyncCreated: result.access_sync.created,
        })
      )
      setSyncRequestID(`sync-${Date.now()}`)
      const [employeeItems, syncJobItems] = await Promise.all([
        listEnterpriseEmployees(token, selectedTenantID.trim()),
        listEnterpriseSyncJobs(token, selectedTenantID.trim()),
      ])
      setEmployees(employeeItems)
      setSyncJobs(syncJobItems)
      const followUp =
        result.job.rejected > 0
          ? syncSourceFailureHint(t, source, result.job.rejected)
          : employeeItems.length === 0
            ? t("enterprisePage.syncSummary.submittedButEmpty")
            : syncSourceSuccessHint(t, source, result.job.deactivated)
      setSyncSummary(
        t("enterprisePage.syncSummary.submittedWithFollowUp", {
          jobID: result.job.id,
          created: result.job.created,
          updated: result.job.updated,
          accessSyncCreated: result.access_sync.created,
          followUp,
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.submitSyncFailed")
      setError(message)
    } finally {
      setSyncing(false)
    }
  }

  async function onSaveTalentaConnector(input: TalentaConnectorSaveInput) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID) {
      return
    }

    setError("")
    setSyncSummary("")
    try {
      const existingConnector = (enterpriseData?.hrisConnectors ?? []).find((item) => item.vendor === "talenta") ?? null
      if (existingConnector) {
        await updateEnterpriseHRISConnector(token, existingConnector.id, {
          tenant_id: tenantID,
          status: input.status,
          sync_strategy: input.sync_strategy,
          credential_ref: input.credential_ref,
          credential_value: input.credential_value,
          webhook_secret_ref: input.webhook_secret_ref,
          webhook_secret_value: input.webhook_secret_value,
          updated_by: viewer.email,
        })
      } else {
        await createEnterpriseHRISConnector(token, {
          tenant_id: tenantID,
          vendor: "talenta",
          status: input.status,
          sync_strategy: input.sync_strategy,
          credential_ref: input.credential_ref,
          credential_value: input.credential_value,
          webhook_secret_ref: input.webhook_secret_ref,
          webhook_secret_value: input.webhook_secret_value,
          updated_by: viewer.email,
        })
      }
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.talentaConnectorSaved", {
          action: existingConnector
            ? t("enterprisePage.syncSummary.talentaConnectorAction.updated")
            : t("enterprisePage.syncSummary.talentaConnectorAction.created"),
          syncStrategy: t(`enterpriseSyncWorkspace.hrisConnector.syncStrategy.${input.sync_strategy}`),
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.saveTalentaConnectorFailed")
      setError(message)
      throw err instanceof Error ? err : new Error(message)
    }
  }

  async function onReplayHRISWebhookDLQ(entryID: string) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || dlqActionID) {
      return
    }

    setDLQActionID(entryID)
    setError("")
    setSyncSummary("")
    try {
      const result = await replayEnterpriseHRISWebhookDLQ(token, entryID, {
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.dlqReplayQueued", {
              entryID: result.item.id,
              status: result.item.status,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.dlqReplayCompleted")
      )
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.replayDLQFailed")
      setError(message)
    } finally {
      setDLQActionID(null)
    }
  }

  async function onReplayHRISWebhookExecution(executionID: string) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || executionActionID) {
      return
    }

    setExecutionActionID(executionID)
    setError("")
    setSyncSummary("")
    try {
      const result = await replayEnterpriseHRISWebhookExecution(token, {
        tenant_id: tenantID,
        execution_id: executionID,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.webhookExecutionReplayQueued", {
              sourceExecutionID: result.source_execution_id,
              executionID: result.execution_id || "",
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.webhookExecutionReplayCompleted", {
              sourceExecutionID: result.source_execution_id,
            })
      )
      return result
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.replayWebhookExecutionFailed")
      setError(message)
    } finally {
      setExecutionActionID(null)
    }
  }

  async function onProcessHRISWebhookReceipt(receiptID: string) {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || receiptActionID) {
      return
    }

    setReceiptActionID(receiptID)
    setError("")
    setSyncSummary("")
    try {
      const result = await processEnterpriseHRISWebhookReceipt(token, {
        tenant_id: tenantID,
        receipt_id: receiptID,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.webhookReceiptQueued", {
              receiptID: result.item.id,
              status: result.item.status,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.webhookReceiptProcessed", {
              receiptID: result.item.id,
              status: result.item.status,
            })
      )
      if (result.item.status !== "processed" && result.item.last_error?.trim()) {
        setError(result.item.last_error.trim())
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.processWebhookReceiptBatchFailed")
      setError(message)
    } finally {
      setReceiptActionID(null)
    }
  }

  async function onBatchReplayHRISWebhookDLQ(entryIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const normalizedEntryIDs = Array.from(new Set(entryIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || dlqActionID || normalizedEntryIDs.length === 0) {
      return
    }

    setDLQActionID("__batch__")
    setError("")
    setSyncSummary("")
    try {
      const result = await replayBatchEnterpriseHRISWebhookDLQ(token, {
        tenant_id: tenantID,
        entry_ids: normalizedEntryIDs,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setLatestWebhookDLQBatchReplayResult(result)
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.dlqBatchReplayQueued", {
              totalEntries: result.total_entries,
              queued: result.queued ?? 0,
              skipped: result.skipped,
              failed: result.failed,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.dlqBatchReplayCompleted", {
              totalEntries: result.total_entries,
              replayed: result.replayed,
              skipped: result.skipped,
              failed: result.failed,
            })
      )
      if (result.failed > 0) {
        setError(t("enterprisePage.errors.replayDLQBatchPartialFailure"))
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.replayDLQFailed")
      setError(message)
    } finally {
      setDLQActionID(null)
    }
  }

  async function onBatchProcessHRISWebhookReceipts(receiptIDs: string[]) {
    const tenantID = selectedTenantID.trim()
    const normalizedReceiptIDs = Array.from(new Set(receiptIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || receiptActionID || normalizedReceiptIDs.length === 0) {
      return
    }

    setReceiptActionID("__batch__")
    setError("")
    setSyncSummary("")
    try {
      const result = await processBatchEnterpriseHRISWebhookReceipts(token, {
        tenant_id: tenantID,
        receipt_ids: normalizedReceiptIDs,
        execution_mode: "queued",
        require_worker: true,
      })
      await Promise.all([
        reloadEnterpriseData(tenantID),
        reloadHRISWebhookExecutions(tenantID),
        reloadHRISWebhookReceipts(tenantID),
        reloadHRISWebhookDLQ(tenantID),
      ])
      setLatestWebhookReceiptBatchProcessResult(result)
      setSyncSummary(
        result.execution_mode === "queued"
          ? t("enterprisePage.syncSummary.webhookReceiptBatchQueued", {
              totalReceipts: result.total_receipts,
              queued: result.queued ?? 0,
              skipped: result.skipped,
              failed: result.failed,
              dispatchMode: formatDispatchModeLabel(result.dispatch_mode),
            })
          : t("enterprisePage.syncSummary.webhookReceiptBatchProcessCompleted", {
              totalReceipts: result.total_receipts,
              processed: result.processed,
              skipped: result.skipped,
              failed: result.failed,
              dlq: result.dlq,
            })
      )
      if (result.failed > 0) {
        setError(t("enterprisePage.errors.processWebhookReceiptBatchPartialFailure"))
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? normalizeWebhookQueueActionError(err.message, t)
          : t("enterprisePage.errors.processWebhookReceiptBatchFailed")
      setError(message)
    } finally {
      setReceiptActionID(null)
    }
  }

  async function onReconcilePendingSyncRequests() {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || syncRequestActionID) {
      return
    }

    setSyncRequestActionID("reconcile-pending")
    setError("")
    setSyncSummary("")
    try {
      const result = await reconcilePendingEnterpriseSyncRequests(token, {
        tenant_id: tenantID,
        limit: 20,
      })
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        t("enterprisePage.syncSummary.reconcilePendingCompleted", {
          processed: result.processed,
          applied: result.applied,
          failed: result.failed,
          skippedByAttemptLimit: result.skipped_by_attempt_limit || 0,
          skippedByCooldown: result.skipped_by_cooldown || 0,
        })
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterprisePage.errors.reconcilePendingFailed")
      setError(message)
    } finally {
      setSyncRequestActionID(null)
    }
  }

  return {
    // Sync state
    syncJobs,
    setSyncJobs,
    syncRequests,
    sortedSyncJobs,
    latestSyncJob,
    failedSyncJobCount,
    syncSource,
    setSyncSource,
    syncRequestID,
    setSyncRequestID,
    syncPayload,
    setSyncPayload,
    syncSummary,
    setSyncSummary,
    syncing,
    error,
    setError,
    sampleSyncPayload,

    // HRIS webhook executions
    hrisWebhookExecutions,
    hrisWebhookExecutionStatusCounts,
    hrisWebhookExecutionQueueCounts,
    hrisWebhookExecutionTotal,
    hrisWebhookExecutionHasMore,
    hrisWebhookExecutionListLoading,
    hrisWebhookExecutionListLoadingMore,
    selectedHRISWebhookExecutionID,
    selectedHRISWebhookExecutionItem,
    selectedHRISWebhookExecutionLoading,
    selectedHRISWebhookExecutionError,
    executionActionID,

    // HRIS webhook receipts
    hrisWebhookReceipts,
    hrisWebhookReceiptQueueCounts,
    hrisWebhookReceiptTotal,
    hrisWebhookReceiptHasMore,
    hrisWebhookReceiptListLoading,
    hrisWebhookReceiptListLoadingMore,
    receiptActionID,
    latestWebhookReceiptBatchProcessResult,

    // HRIS webhook DLQ
    hrisWebhookDLQEntries,
    hrisWebhookDLQReplayCounts,
    hrisWebhookDLQTotal,
    hrisWebhookDLQHasMore,
    hrisWebhookDLQListLoading,
    hrisWebhookDLQListLoadingMore,
    dlqActionID,
    latestWebhookDLQBatchReplayResult,

    // Sync request
    syncRequestActionID,

    // Handlers
    onSyncEmployees,
    onSaveTalentaConnector,
    onReplayHRISWebhookDLQ,
    onReplayHRISWebhookExecution,
    onProcessHRISWebhookReceipt,
    onBatchReplayHRISWebhookDLQ,
    onBatchProcessHRISWebhookReceipts,
    onReconcilePendingSyncRequests,
    onHRISWebhookExecutionHistoryViewChange,
    onLoadMoreHRISWebhookExecutions,
    onLoadMoreHRISWebhookReceipts,
    onLoadMoreHRISWebhookDLQ,
  }
}
