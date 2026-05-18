import { type ColumnDef, type SortingState, type VisibilityState, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  ArrowUpDownIcon,
} from "lucide-react"
import QRCode from "qrcode"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  activateWalletPass,
  getWalletPass,
  getWalletPassSaveLink,
  issueWalletPass,
  issueWalletPassBatch,
  revokeWalletPass,
  suspendWalletPass,
  type WalletIssueJob,
  type WalletPassInstance,
  type WalletPassTemplate,
} from "@/lib/api"
import {
  createWalletScenarioCounters,
  formatDateTime,
  inferPassScenario,
  normalizeDateTimeInput,
  passStatusLabel,
  passStatusVariant,
  passTypeLabel,
  targetTypeLabel,
  walletScenarioLabel,
  type WalletScenarioKind,
} from "../pages/wallet-page-utils"

type UseWalletPassesParams = {
  token: string
  tenantID: string
  writable: boolean
  templates: WalletPassTemplate[]
  templateByID: Map<string, WalletPassTemplate>
  resolveTargetType: (templateID: string) => "user" | "visitor"
  loadWalletOps: (tenantID: string) => Promise<void>
}

export function useWalletPasses({
  token,
  tenantID,
  writable,
  templates,
  templateByID,
  resolveTargetType,
  loadWalletOps,
}: UseWalletPassesParams) {
  const { t } = useTranslation()

  const [passes, setPasses] = useState<WalletPassInstance[]>([])
  const [passQuery, setPassQuery] = useState("")
  const [passStatusFilter, setPassStatusFilter] = useState<"all" | "issued" | "active" | "suspended" | "revoked">("all")
  const [passTargetTypeFilter, setPassTargetTypeFilter] = useState<"all" | "user" | "visitor">("all")
  const [passTemplateFilter, setPassTemplateFilter] = useState("all")
  const [selectedPassIDs, setSelectedPassIDs] = useState<string[]>([])
  const [passPage, setPassPage] = useState(1)
  const [passPageSize, setPassPageSize] = useState(25)
  const [passSorting, setPassSorting] = useState<SortingState>([])
  const [passColumnVisibility, setPassColumnVisibility] = useState<VisibilityState>({})

  const [singleTemplateID, setSingleTemplateID] = useState("")
  const [singleTargetID, setSingleTargetID] = useState("")
  const [singleExpiresAt, setSingleExpiresAt] = useState("")
  const [batchTemplateID, setBatchTemplateID] = useState("")
  const [batchTargetIDs, setBatchTargetIDs] = useState("")
  const [batchExpiresAt, setBatchExpiresAt] = useState("")
  const [batchExecutionMode, setBatchExecutionMode] = useState<"inline" | "queued">("queued")

  const [issuingSingle, setIssuingSingle] = useState(false)
  const [issuingBatch, setIssuingBatch] = useState(false)
  const [updatingPassID, setUpdatingPassID] = useState("")
  const [batchUpdatingPassAction, setBatchUpdatingPassAction] = useState<"" | "activate" | "suspend" | "revoke">("")
  const [lastIssuedJobs, setLastIssuedJobs] = useState<WalletIssueJob[]>([])
  const [issuanceSummary, setIssuanceSummary] = useState("")
  const [error, setError] = useState("")

  // QR dialog state
  const [resolvingSaveLinkPassID, setResolvingSaveLinkPassID] = useState("")
  const [qrDialogOpen, setQrDialogOpen] = useState(false)
  const [qrDialogPass, setQrDialogPass] = useState<WalletPassInstance | null>(null)
  const [qrDialogSaveLink, setQrDialogSaveLink] = useState("")
  const [qrDialogSVG, setQrDialogSVG] = useState("")
  const [qrDialogPreviewURL, setQrDialogPreviewURL] = useState("")
  const [qrDialogLoading, setQrDialogLoading] = useState(false)

  useEffect(() => {
    if (!qrDialogSVG) {
      setQrDialogPreviewURL("")
      return
    }

    const blob = new Blob([qrDialogSVG], { type: "image/svg+xml;charset=utf-8" })
    const objectURL = URL.createObjectURL(blob)
    setQrDialogPreviewURL(objectURL)

    return () => {
      URL.revokeObjectURL(objectURL)
    }
  }, [qrDialogSVG])

  const passByID = useMemo(() => new Map(passes.map((item) => [item.id, item])), [passes])

  const filteredPasses = useMemo(() => {
    const q = passQuery.trim().toLowerCase()
    return passes.filter((item) => {
      if (passStatusFilter !== "all" && item.status !== passStatusFilter) {
        return false
      }
      if (passTargetTypeFilter !== "all" && item.target_type !== passTargetTypeFilter) {
        return false
      }
      if (passTemplateFilter !== "all" && item.template_id !== passTemplateFilter) {
        return false
      }
      if (!q) {
        return true
      }
      const templateName = templateByID.get(item.template_id)?.name ?? item.template_id
      return (
        item.id.toLowerCase().includes(q) ||
        item.target_id.toLowerCase().includes(q) ||
        item.object_id.toLowerCase().includes(q) ||
        item.status.toLowerCase().includes(q) ||
        item.target_type.toLowerCase().includes(q) ||
        templateName.toLowerCase().includes(q)
      )
    })
  }, [passQuery, passStatusFilter, passTargetTypeFilter, passTemplateFilter, passes, templateByID])

  const passMaxPage = Math.max(1, Math.ceil(filteredPasses.length / passPageSize))
  const selectedPassIDSet = useMemo(() => new Set(selectedPassIDs), [selectedPassIDs])

  const selectedFilteredPassCount = useMemo(() => {
    return filteredPasses.reduce((sum, item) => (selectedPassIDSet.has(item.id) ? sum + 1 : sum), 0)
  }, [filteredPasses, selectedPassIDSet])

  const hasPassFilters =
    passQuery.trim().length > 0 ||
    passStatusFilter !== "all" ||
    passTargetTypeFilter !== "all" ||
    passTemplateFilter !== "all"

  const employeePassCount = useMemo(() => passes.filter((item) => item.target_type === "user").length, [passes])
  const visitorPassCount = useMemo(() => passes.filter((item) => item.target_type === "visitor").length, [passes])
  const suspendedPassCount = useMemo(() => passes.filter((item) => item.status === "suspended").length, [passes])
  const revocablePassCount = useMemo(() => passes.filter((item) => item.status !== "revoked").length, [passes])

  const passScenarioCounts = useMemo(() => {
    const next = createWalletScenarioCounters()
    passes.forEach((item) => {
      next[inferPassScenario(item, templateByID.get(item.template_id))] += 1
    })
    return next
  }, [passes, templateByID])

  const saveLinkScenarioCounts = useMemo(() => {
    const next = createWalletScenarioCounters()
    passes.forEach((item) => {
      if (!item.save_link) {
        return
      }
      next[inferPassScenario(item, templateByID.get(item.template_id))] += 1
    })
    return next
  }, [passes, templateByID])

  const selectedSingleTemplate = useMemo(
    () => templates.find((item) => item.id === singleTemplateID) ?? null,
    [singleTemplateID, templates]
  )
  const selectedBatchTemplate = useMemo(
    () => templates.find((item) => item.id === batchTemplateID) ?? null,
    [batchTemplateID, templates]
  )

  const employeeCardEligiblePasses = useMemo(() => {
    return passes.filter((item) => item.target_type === "user")
  }, [passes])

  const deliveryDeskPasses = useMemo(() => {
    return filteredPasses
      .filter((item) => {
        const scenario = inferPassScenario(item, templateByID.get(item.template_id))
        return item.save_link || scenario === "employee_physical"
      })
      .slice(0, 6)
  }, [filteredPasses, templateByID])

  const deliverablePasses = useMemo(() => {
    return passes.filter((item) => item.save_link)
  }, [passes])

  const qrDialogTemplate = useMemo(
    () => (qrDialogPass ? templateByID.get(qrDialogPass.template_id) : undefined),
    [qrDialogPass, templateByID]
  )

  // Column definitions for pass table
  const passColumns = useMemo<ColumnDef<WalletPassInstance>[]>(
    () => {
      const definition: ColumnDef<WalletPassInstance>[] = [
        {
          id: "template",
          accessorFn: (row) => templateByID.get(row.template_id)?.name ?? row.template_id,
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.template")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => {
            const itemTemplate = templateByID.get(row.original.template_id)
            return (
              <div className="space-y-1">
                <p className="font-medium">{itemTemplate?.name ?? row.original.template_id}</p>
                <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  <span>{itemTemplate ? passTypeLabel(t, itemTemplate.pass_type) : targetTypeLabel(t, row.original.target_type)}</span>
                  <Badge variant="secondary">
                    {walletScenarioLabel(t, inferPassScenario(row.original, itemTemplate))}
                  </Badge>
                  {itemTemplate?.status === "inactive" ? (
                    <Badge variant="outline">{t("walletPage.table.templateInactive")}</Badge>
                  ) : null}
                </div>
              </div>
            )
          },
        },
        {
          id: "target",
          accessorFn: (row) => row.target_id,
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.target")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <div className="space-y-1">
              <p>{row.original.target_id}</p>
              <p className="mp-kpi-note">
                {targetTypeLabel(t, row.original.target_type)} · object {row.original.object_id}
              </p>
            </div>
          ),
        },
        {
          id: "status",
          accessorKey: "status",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.status")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <Badge variant={passStatusVariant(row.original.status)}>{passStatusLabel(t, row.original.status)}</Badge>,
        },
        {
          id: "expires_at",
          accessorFn: (row) => row.expires_at || "",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.expiresAt")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <span className="mp-kpi-note">
              {row.original.expires_at ? formatDateTime(row.original.expires_at) : t("walletPage.table.expiresAtDefaultPolicy")}
            </span>
          ),
        },
        {
          id: "save_link",
          accessorFn: (row) => row.save_link || "",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.saveLink")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <div className="text-xs">
              {row.original.save_link ? (
                <div className="flex flex-col items-start gap-1">
                  <a
                    className="text-primary underline-offset-2 hover:underline"
                    href={row.original.save_link}
                    rel="noreferrer"
                    target="_blank"
                  >
                    {t("walletPage.actions.openSaveLink")}
                  </a>
                  <button
                    className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                    type="button"
                    onClick={() => void copySaveLink(row.original)}
                  >
                    {t("walletPage.actions.copyLink")}
                  </button>
                  <button
                    className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                    type="button"
                    onClick={() => void openPassQrDialog(row.original)}
                  >
                    {t("walletPage.actions.viewQrCode")}
                  </button>
                </div>
              ) : (
                <button
                  className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                  type="button"
                  onClick={() => void refreshPassSaveLink(row.original)}
                  disabled={resolvingSaveLinkPassID === row.original.id}
                >
                  {resolvingSaveLinkPassID === row.original.id
                    ? t("walletPage.actions.refreshing")
                    : t("walletPage.actions.refreshLink")}
                </button>
              )}
            </div>
          ),
        },
        {
          id: "updated_at",
          accessorKey: "updated_at",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.updatedAt")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <span className="mp-kpi-note">{formatDateTime(row.original.updated_at)}</span>,
        },
        {
          id: "actions",
          header: () => t("walletPage.table.columns.actions"),
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => {
            const canSuspend = canApplyPassAction(row.original, "suspend")
            const canActivate = canApplyPassAction(row.original, "activate")
            const canRevoke = canApplyPassAction(row.original, "revoke")
            return (
              <div className="flex flex-wrap gap-2">
                {canSuspend ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!writable || updatingPassID === row.original.id || batchUpdatingPassAction.length > 0}
                    onClick={() => void updatePassStatus(row.original, "suspend")}
                  >
                    {t("walletPage.actions.suspend")}
                  </Button>
                ) : null}
                {canActivate ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!writable || updatingPassID === row.original.id || batchUpdatingPassAction.length > 0}
                    onClick={() => void updatePassStatus(row.original, "activate")}
                  >
                    {t("walletPage.actions.activate")}
                  </Button>
                ) : null}
                {canRevoke ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!writable || updatingPassID === row.original.id || batchUpdatingPassAction.length > 0}
                    onClick={() => void updatePassStatus(row.original, "revoke")}
                  >
                    {t("walletPage.actions.revoke")}
                  </Button>
                ) : null}
                {!writable ? <span className="mp-kpi-note">{t("walletPage.hints.readOnlyBoundaryOnly")}</span> : null}
              </div>
            )
          },
        },
      ]
      if (writable) {
        definition.unshift({
          id: "select",
          header: ({ table }) => {
            const pageRows = table.getRowModel().rows
            const allSelected =
              pageRows.length > 0 && pageRows.every((row) => selectedPassIDSet.has(row.original.id))
            return (
              <input
                aria-label="select all visible wallet passes"
                type="checkbox"
                className="size-4 rounded border"
                disabled={pageRows.length === 0 || batchUpdatingPassAction.length > 0}
                checked={allSelected}
                onChange={(event) => {
                  const visiblePassIDs = pageRows.map((row) => row.original.id)
                  if (visiblePassIDs.length === 0) {
                    return
                  }
                  setSelectedPassIDs((current) => {
                    if (!event.target.checked) {
                      const removable = new Set(visiblePassIDs)
                      return current.filter((item) => !removable.has(item))
                    }
                    const merged = new Set(current)
                    visiblePassIDs.forEach((item) => merged.add(item))
                    return Array.from(merged)
                  })
                }}
              />
            )
          },
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => (
            <input
              aria-label={`select wallet pass ${row.original.id}`}
              type="checkbox"
              className="size-4 rounded border"
              checked={selectedPassIDSet.has(row.original.id)}
              disabled={batchUpdatingPassAction.length > 0}
              onChange={(event) => onSelectPass(row.original.id, event.target.checked)}
            />
          ),
        })
      }
      return definition
    },
    [
      batchUpdatingPassAction.length,
      resolvingSaveLinkPassID,
      selectedPassIDSet,
      templateByID,
      updatingPassID,
      writable,
    ]
  )

  const passTable = useReactTable({
    columns: passColumns,
    data: filteredPasses,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility: passColumnVisibility,
      pagination: {
        pageIndex: Math.max(0, passPage - 1),
        pageSize: passPageSize,
      },
      sorting: passSorting,
    },
    onColumnVisibilityChange: setPassColumnVisibility,
    onSortingChange: setPassSorting,
  })

  // Sync effects
  useEffect(() => {
    const visiblePassIDSet = new Set(passes.map((item) => item.id))
    setSelectedPassIDs((current) => current.filter((item) => visiblePassIDSet.has(item)))
    if (passTemplateFilter !== "all" && !templates.some((item) => item.id === passTemplateFilter)) {
      setPassTemplateFilter("all")
    }
  }, [passTemplateFilter, passes, templates])

  useEffect(() => {
    setPassPage(1)
  }, [passPageSize, passQuery, passStatusFilter, passTargetTypeFilter, passTemplateFilter])

  useEffect(() => {
    if (passPage > passMaxPage) {
      setPassPage(passMaxPage)
    }
  }, [passMaxPage, passPage])

  function canApplyPassAction(pass: WalletPassInstance, action: "activate" | "suspend" | "revoke") {
    switch (action) {
      case "activate":
        return pass.status === "issued" || pass.status === "suspended"
      case "suspend":
        return pass.status === "issued" || pass.status === "active"
      case "revoke":
        return pass.status !== "revoked"
      default:
        return false
    }
  }

  function onSelectPass(passID: string, checked: boolean) {
    setSelectedPassIDs((current) => {
      if (checked) {
        if (current.includes(passID)) {
          return current
        }
        return [...current, passID]
      }
      return current.filter((item) => item !== passID)
    })
  }

  async function updatePassStatus(pass: WalletPassInstance, action: "activate" | "suspend" | "revoke") {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setUpdatingPassID(pass.id)
    setIssuanceSummary("")
    setError("")
    try {
      const payload = {
        tenant_id: nextTenantID,
        actor: `web_admin.wallet.pass.${action}`,
      }
      const updated =
        action === "activate"
          ? await activateWalletPass(token, pass.id, payload)
          : action === "suspend"
            ? await suspendWalletPass(token, pass.id, payload)
            : await revokeWalletPass(token, pass.id, payload)
      setIssuanceSummary(
        t("walletPage.summaries.passStatusUpdated", {
          targetType: targetTypeLabel(t, updated.target_type),
          targetID: updated.target_id,
          status: passStatusLabel(t, updated.status),
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.updatePassStatusFailed")
      setError(message)
    } finally {
      setUpdatingPassID("")
    }
  }

  async function updateSelectedPasses(action: "activate" | "suspend" | "revoke") {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    const targetPasses = filteredPasses.filter(
      (item) => selectedPassIDSet.has(item.id) && canApplyPassAction(item, action)
    )
    if (targetPasses.length === 0) {
      setError(t("walletPage.errors.noActionableSelectedPasses"))
      return
    }

    setBatchUpdatingPassAction(action)
    setIssuanceSummary("")
    setError("")
    try {
      const settled = await Promise.allSettled(
        targetPasses.map((pass) => {
          const payload = {
            tenant_id: nextTenantID,
            actor: `web_admin.wallet.pass.batch.${action}`,
          }
          if (action === "activate") {
            return activateWalletPass(token, pass.id, payload)
          }
          if (action === "suspend") {
            return suspendWalletPass(token, pass.id, payload)
          }
          return revokeWalletPass(token, pass.id, payload)
        })
      )
      const succeeded = settled.filter((item) => item.status === "fulfilled").length
      const failed = settled.length - succeeded
      setSelectedPassIDs([])
      setIssuanceSummary(
        t("walletPage.summaries.batchPassActionCompleted", {
          action:
            action === "activate"
              ? t("walletPage.actions.activate")
              : action === "suspend"
                ? t("walletPage.actions.suspend")
                : t("walletPage.actions.revoke"),
          succeeded,
          failed,
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchUpdatePassStatusFailed")
      setError(message)
    } finally {
      setBatchUpdatingPassAction("")
    }
  }

  async function submitSingleIssue(payload: {
    templateID: string
    targetID: string
    expiresAt: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.templateID.trim()) {
      setError(t("walletPage.errors.issueTemplateRequired"))
      return false
    }
    if (!payload.targetID.trim()) {
      setError(t("walletPage.errors.issueTargetRequired"))
      return false
    }

    setIssuingSingle(true)
    setIssuanceSummary("")
    setError("")
    try {
      const targetType = resolveTargetType(payload.templateID)
      const pass = await issueWalletPass(token, {
        tenant_id: nextTenantID,
        template_id: payload.templateID,
        target_type: targetType,
        target_id: payload.targetID.trim(),
        expires_at: normalizeDateTimeInput(payload.expiresAt),
        actor: "web_admin.wallet.issue.single",
      })
      setIssuanceSummary(
        t("walletPage.summaries.singleIssueSubmitted", {
          targetType: targetTypeLabel(t, pass.target_type),
          targetID: pass.target_id,
          status: passStatusLabel(t, pass.status),
        })
      )
      setSingleTargetID("")
      setSingleExpiresAt("")
      setLastIssuedJobs([])
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.singleIssueFailed")
      setError(message)
      return false
    } finally {
      setIssuingSingle(false)
    }
  }

  async function submitBatchIssue(payload: {
    templateID: string
    targetIDs: string[]
    expiresAt: string
    executionMode: "inline" | "queued"
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.templateID.trim()) {
      setError(t("walletPage.errors.batchIssueTemplateRequired"))
      return false
    }

    const targetIDs = payload.targetIDs
    if (targetIDs.length === 0) {
      setError(t("walletPage.errors.batchIssueTargetsRequired"))
      return false
    }

    setIssuingBatch(true)
    setIssuanceSummary("")
    setError("")
    try {
      const targetType = resolveTargetType(payload.templateID)
      const result = await issueWalletPassBatch(token, {
        tenant_id: nextTenantID,
        template_id: payload.templateID,
        target_type: targetType,
        target_ids: targetIDs,
        expires_at: normalizeDateTimeInput(payload.expiresAt),
        execution_mode: payload.executionMode,
        actor: "web_admin.wallet.issue.batch",
      })
      setLastIssuedJobs(result.items)
      setIssuanceSummary(
        t("walletPage.summaries.batchIssueSubmitted", {
          count: targetIDs.length,
          targetType: targetTypeLabel(t, targetType),
          executionMode: result.execution_mode,
        })
      )
      setBatchTargetIDs("")
      setBatchExpiresAt("")
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchIssueFailed")
      setError(message)
      return false
    } finally {
      setIssuingBatch(false)
    }
  }

  function patchPassRecord(passID: string, updater: (current: WalletPassInstance) => WalletPassInstance) {
    setPasses((current) => current.map((item) => (item.id === passID ? updater(item) : item)))
  }

  async function refreshPassRecord(pass: WalletPassInstance) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      throw new Error(t("walletPage.errors.tenantRequired"))
    }
    const latest = await getWalletPass(token, pass.id, nextTenantID)
    patchPassRecord(pass.id, () => latest)
    return latest
  }

  async function resolvePassSaveLink(pass: WalletPassInstance) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      throw new Error(t("walletPage.errors.tenantRequired"))
    }
    if (pass.save_link) {
      return pass.save_link
    }
    const latest = await refreshPassRecord(pass)
    if (latest.save_link) {
      return latest.save_link
    }
    const link = await getWalletPassSaveLink(token, pass.id, nextTenantID)
    patchPassRecord(pass.id, (current) => ({ ...current, save_link: link }))
    return link
  }

  async function refreshPassSaveLink(pass: WalletPassInstance) {
    setResolvingSaveLinkPassID(pass.id)
    setError("")
    try {
      const link = await resolvePassSaveLink(pass)
      setIssuanceSummary(t("walletPage.summaries.saveLinkRefreshed", { targetID: pass.target_id }))
      if (qrDialogPass?.id === pass.id) {
        setQrDialogSaveLink(link)
        const svg = await QRCode.toString(link, {
          type: "svg",
          width: 280,
          margin: 1,
          color: { dark: "#0f172a", light: "#ffffff" },
        })
        setQrDialogSVG(svg)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.refreshSaveLinkFailed")
      setError(message)
    } finally {
      setResolvingSaveLinkPassID("")
    }
  }

  async function openPassQrDialog(pass: WalletPassInstance) {
    setQrDialogOpen(true)
    setQrDialogPass(pass)
    setQrDialogSaveLink("")
    setQrDialogSVG("")
    setQrDialogLoading(true)
    setError("")
    try {
      let latest = pass
      if (!latest.save_link) {
        latest = await refreshPassRecord(pass)
      }
      const link = latest.save_link || (await getWalletPassSaveLink(token, pass.id, tenantID.trim()))
      if (!latest.save_link) {
        latest = { ...latest, save_link: link }
        patchPassRecord(pass.id, () => latest)
      }
      const svg = await QRCode.toString(link, {
        type: "svg",
        width: 280,
        margin: 1,
        color: { dark: "#0f172a", light: "#ffffff" },
      })
      setQrDialogPass(latest)
      setQrDialogSaveLink(link)
      setQrDialogSVG(svg)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.generateQrCodeFailed")
      setQrDialogOpen(false)
      setQrDialogPass(null)
      setError(message)
    } finally {
      setQrDialogLoading(false)
    }
  }

  function downloadQrSVG() {
    if (!qrDialogSVG || !qrDialogPass) {
      return
    }
    const blob = new Blob([qrDialogSVG], { type: "image/svg+xml;charset=utf-8" })
    const objectURL = URL.createObjectURL(blob)
    const anchor = document.createElement("a")
    anchor.href = objectURL
    anchor.download = `${qrDialogPass.target_id || qrDialogPass.id}-mistypass-qr.svg`
    document.body.append(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(objectURL)
  }

  async function copySaveLink(pass: WalletPassInstance) {
    if (!pass.save_link) {
      return
    }
    if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
      setError(t("walletPage.errors.clipboardUnsupported"))
      return
    }
    try {
      await navigator.clipboard.writeText(pass.save_link)
      setIssuanceSummary(t("walletPage.summaries.saveLinkCopied", { targetID: pass.target_id }))
      setError("")
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.copySaveLinkFailed")
      setError(message)
    }
  }

  const singleTargetType = selectedSingleTemplate?.pass_type === "visitor" ? "visitor" : "user"
  const batchTargetType = selectedBatchTemplate?.pass_type === "visitor" ? "visitor" : "user"

  return {
    passes,
    setPasses,
    passByID,
    filteredPasses,
    passQuery,
    setPassQuery,
    passStatusFilter,
    setPassStatusFilter,
    passTargetTypeFilter,
    setPassTargetTypeFilter,
    passTemplateFilter,
    setPassTemplateFilter,
    selectedPassIDs,
    setSelectedPassIDs,
    passPage,
    setPassPage,
    passPageSize,
    setPassPageSize,
    passTable,
    selectedFilteredPassCount,
    hasPassFilters,
    employeePassCount,
    visitorPassCount,
    suspendedPassCount,
    revocablePassCount,
    passScenarioCounts,
    saveLinkScenarioCounts,
    selectedSingleTemplate,
    selectedBatchTemplate,
    singleTemplateID,
    setSingleTemplateID,
    singleTargetID,
    setSingleTargetID,
    singleExpiresAt,
    setSingleExpiresAt,
    batchTemplateID,
    setBatchTemplateID,
    batchTargetIDs,
    setBatchTargetIDs,
    batchExpiresAt,
    setBatchExpiresAt,
    batchExecutionMode,
    setBatchExecutionMode,
    issuingSingle,
    issuingBatch,
    updatingPassID,
    batchUpdatingPassAction,
    lastIssuedJobs,
    issuanceSummary,
    setIssuanceSummary,
    error,
    setError,
    employeeCardEligiblePasses,
    deliveryDeskPasses,
    deliverablePasses,
    singleTargetType,
    batchTargetType,
    // QR dialog
    resolvingSaveLinkPassID,
    qrDialogOpen,
    setQrDialogOpen,
    qrDialogPass,
    setQrDialogPass,
    qrDialogSaveLink,
    setQrDialogSaveLink,
    qrDialogSVG,
    setQrDialogSVG,
    qrDialogPreviewURL,
    qrDialogLoading,
    setQrDialogLoading,
    qrDialogTemplate,
    // Actions
    updatePassStatus,
    updateSelectedPasses,
    submitSingleIssue,
    submitBatchIssue,
    copySaveLink,
    refreshPassSaveLink,
    openPassQrDialog,
    downloadQrSVG,
    onSelectPass,
  }
}
