import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { ArrowUpDownIcon, FileSearchIcon, SendIcon, ShieldIcon, SlidersHorizontalIcon, WebhookIcon } from "lucide-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { ToggleSwitch } from "@/components/mistyislet/primitives"
import { ListPagination } from "@/components/ui/list-pagination"
import {
  APIError,
  dispatchAuditWebhook,
  getAuditWebhookConfig,
  listAuditLogs,
  listAuditWebhookDeliveries,
  updateAuditWebhookConfig,
  type AuditLog,
  type AuditWebhookConfig,
  type AuditWebhookDelivery,
  type CurrentUser,
} from "@/lib/api"

type AuditPageProps = {
  token: string
  viewer: CurrentUser
}

type AuditAction =
  | "login"
  | "tenant_update"
  | "policy_publish"
  | "gateway_reboot"
  | "visitor_issue"

const AUDIT_DATE_LOCALE: Record<string, string> = {
  "zh-CN": "zh-CN",
  "en-US": "en-US",
  "id-ID": "id-ID",
}

function labelForAction(action: string, t: TFunction) {
  switch (action) {
    case "login":
      return t("audit.actionLabel.login")
    case "tenant_update":
      return t("audit.actionLabel.tenantUpdate")
    case "policy_publish":
      return t("audit.actionLabel.policyPublish")
    case "gateway_reboot":
      return t("audit.actionLabel.gatewayReboot")
    case "visitor_issue":
      return t("audit.actionLabel.visitorIssue")
    default:
      return action.replaceAll("_", " ")
  }
}

function roleLabel(role: string, t: TFunction) {
  switch (role) {
    case "super_admin":
      return t("audit.roleLabel.superAdmin")
    case "tenant_admin":
      return t("audit.roleLabel.tenantAdmin")
    case "operator":
      return t("audit.roleLabel.operator")
    default:
      return role.replaceAll("_", " ")
  }
}

export function AuditPage({ token, viewer }: AuditPageProps) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id
  const [query, setQuery] = useState("")
  const [actionFilter, setActionFilter] = useState<"all" | AuditAction>("all")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [webhookEnabled, setWebhookEnabled] = useState(false)
  const [webhookEndpoint, setWebhookEndpoint] = useState("")
  const [webhookActions, setWebhookActions] = useState("")
  const [webhookSecret, setWebhookSecret] = useState("")
  const [dispatchAuditLogID, setDispatchAuditLogID] = useState("")
  const [webhookMessage, setWebhookMessage] = useState("")
  const [webhookError, setWebhookError] = useState("")
  const auditLogsQuery = useQuery({
    queryKey: ["audit-logs"],
    queryFn: () => listAuditLogs(token),
  })
  const webhookConfigQuery = useQuery({
    queryKey: ["audit-webhook-config", tenantID],
    queryFn: () => getAuditWebhookConfig(token, tenantID),
    enabled: Boolean(token && tenantID),
    retry: false,
  })
  const webhookDeliveriesQuery = useQuery({
    queryKey: ["audit-webhook-deliveries", tenantID],
    queryFn: () => listAuditWebhookDeliveries(token, tenantID, 10),
    enabled: Boolean(token && tenantID),
  })
  const rows: AuditLog[] = auditLogsQuery.data ?? []
  const deliveries = webhookDeliveriesQuery.data?.items ?? []
  const loading = auditLogsQuery.isPending
  const error =
    auditLogsQuery.error instanceof Error ? auditLogsQuery.error.message : ""
  const webhookConfigError = webhookConfigQuery.error instanceof Error ? webhookConfigQuery.error : null
  const configMissing = webhookConfigError instanceof APIError && webhookConfigError.status === 404
  const webhookConfig = webhookConfigQuery.data
  const dateLocale = AUDIT_DATE_LOCALE[i18n.language] ?? "zh-CN"

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return rows.filter((row) => {
      const actionMatched = actionFilter === "all" || row.action === actionFilter
      if (!actionMatched) {
        return false
      }
      if (!q) {
        return true
      }
      return (
        row.id.toLowerCase().includes(q) ||
        row.actor.toLowerCase().includes(q) ||
        row.target.toLowerCase().includes(q)
      )
    })
  }, [query, actionFilter, rows])
  const webhookActionList = useMemo(
    () =>
      webhookActions
        .split(",")
        .map((item) => item.trim().toLowerCase())
        .filter(Boolean),
    [webhookActions]
  )
  const dispatchTargetID = dispatchAuditLogID.trim() || filtered[0]?.id || rows[0]?.id || ""
  const columns = useMemo<ColumnDef<AuditLog>[]>(
    () => [
      {
        id: "id",
        accessorKey: "id",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.id")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[12rem] font-medium">{row.original.id}</TableCellText>,
      },
      {
        id: "actor",
        accessorKey: "actor",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.actor")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => row.original.actor,
      },
      {
        id: "role",
        accessorKey: "role",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.role")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <Badge variant="outline">
            {roleLabel(row.original.role, t)}
          </Badge>
        ),
      },
      {
        id: "action",
        accessorKey: "action",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.action")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => labelForAction(row.original.action, t),
      },
      {
        id: "target",
        accessorKey: "target",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.target")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[14rem]">{row.original.target}</TableCellText>,
      },
      {
        id: "source",
        accessorKey: "source",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.source")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => row.original.source,
      },
      {
        id: "at",
        accessorKey: "at",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("audit.table.at")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => new Date(row.original.at).toLocaleString(dateLocale),
      },
    ],
    [dateLocale, t]
  )
  const table = useReactTable({
    columns,
    data: filtered,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility,
      pagination: {
        pageIndex: Math.max(0, page - 1),
        pageSize,
      },
      sorting,
    },
    onColumnVisibilityChange: setColumnVisibility,
    onSortingChange: setSorting,
  })
  const maxPage = Math.max(1, Math.ceil(filtered.length / pageSize))
  const visibleColumnCount = table.getVisibleLeafColumns().length
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const columnLabels: Record<string, string> = {
    action: t("audit.table.action"),
    actor: t("audit.table.actor"),
    at: t("audit.table.at"),
    id: t("audit.table.id"),
    role: t("audit.table.role"),
    source: t("audit.table.source"),
    target: t("audit.table.target"),
  }

  useEffect(() => {
    setPage(1)
  }, [actionFilter, pageSize, query])

  useEffect(() => {
    if (page > maxPage) {
      setPage(maxPage)
    }
  }, [maxPage, page])

  useEffect(() => {
    if (!webhookConfig) {
      return
    }
    setWebhookEnabled(webhookConfig.enabled)
    setWebhookEndpoint(webhookConfig.endpoint || "")
    setWebhookActions((webhookConfig.actions ?? []).join(", "))
    setWebhookSecret("")
  }, [webhookConfig])

  const updateWebhookMutation = useMutation({
    mutationFn: () =>
      updateAuditWebhookConfig(token, {
        tenant_id: tenantID,
        enabled: webhookEnabled,
        endpoint: webhookEndpoint.trim(),
        actions: webhookActionList,
        signing_secret: webhookSecret.trim() || undefined,
        updated_by: viewer.email,
      }),
    onSuccess: (config) => {
      queryClient.invalidateQueries({ queryKey: ["audit-webhook-config", tenantID] })
      queryClient.invalidateQueries({ queryKey: ["audit-logs"] })
      setWebhookEnabled(config.enabled)
      setWebhookEndpoint(config.endpoint || "")
      setWebhookActions((config.actions ?? []).join(", "))
      setWebhookSecret("")
      setWebhookError("")
      setWebhookMessage("Audit webhook settings saved.")
    },
    onError: (err) => {
      setWebhookMessage("")
      setWebhookError(err instanceof Error ? err.message : "Failed to save audit webhook settings")
    },
  })

  const dispatchWebhookMutation = useMutation({
    mutationFn: () =>
      dispatchAuditWebhook(token, {
        tenant_id: tenantID,
        audit_log_id: dispatchTargetID,
      }),
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ["audit-webhook-deliveries", tenantID] })
      queryClient.invalidateQueries({ queryKey: ["audit-logs"] })
      setWebhookError("")
      setWebhookMessage(`Webhook delivery ${response.delivery.id} ${response.delivery.status}.`)
    },
    onError: (err) => {
      queryClient.invalidateQueries({ queryKey: ["audit-webhook-deliveries", tenantID] })
      setWebhookMessage("")
      setWebhookError(err instanceof Error ? err.message : "Failed to dispatch audit webhook")
    },
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("audit.eyebrow")}</p>
        <h1 className="mp-page-title">{t("audit.title")}</h1>
        <p className="mp-page-description">
          {t("audit.description")}
        </p>
      </div>

      <AuditWebhookPanel
        config={webhookConfig}
        configMissing={configMissing}
        configError={webhookConfigError && !configMissing ? webhookConfigError.message : ""}
        deliveries={deliveries}
        dispatchAuditLogID={dispatchAuditLogID}
        dispatchTargetID={dispatchTargetID}
        enabled={webhookEnabled}
        endpoint={webhookEndpoint}
        actions={webhookActions}
        signingSecret={webhookSecret}
        message={webhookMessage}
        error={webhookError}
        dateLocale={dateLocale}
        loading={webhookConfigQuery.isLoading || webhookDeliveriesQuery.isLoading}
        saving={updateWebhookMutation.isPending}
        dispatching={dispatchWebhookMutation.isPending}
        onEnabledChange={setWebhookEnabled}
        onEndpointChange={setWebhookEndpoint}
        onActionsChange={setWebhookActions}
        onSigningSecretChange={setWebhookSecret}
        onDispatchTargetChange={setDispatchAuditLogID}
        onSave={() => updateWebhookMutation.mutate()}
        onDispatch={() => dispatchWebhookMutation.mutate()}
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("audit.kpi.records.title")}</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : rows.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("audit.kpi.records.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("audit.kpi.sensitive.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {rows.filter((item) => item.action !== "login").length}{" "}
              <ShieldIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("audit.kpi.sensitive.note")}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("audit.kpi.manualReview.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {rows.filter((item) => item.action === "gateway_reboot").length}{" "}
              <FileSearchIcon className="size-4 text-sky-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("audit.kpi.manualReview.note")}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("audit.filter.title")}</CardTitle>
          <CardDescription>{t("audit.filter.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 md:grid-cols-[1fr_220px]">
            <Input
              value={query}
              onChange={(event) => {
                setQuery(event.target.value)
                setPage(1)
              }}
              aria-label={t("audit.filter.queryPlaceholder")}
              placeholder={t("audit.filter.queryPlaceholder")}
            />
            <Select
              value={actionFilter}
              onValueChange={(value: "all" | AuditAction) => {
                setActionFilter(value)
                setPage(1)
              }}
            >
              <SelectTrigger aria-label={t("audit.filter.actionPlaceholder")}>
                <SelectValue placeholder={t("audit.filter.actionPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("audit.filter.actionOptions.all")}</SelectItem>
                <SelectItem value="login">{t("audit.filter.actionOptions.login")}</SelectItem>
                <SelectItem value="tenant_update">{t("audit.filter.actionOptions.tenantUpdate")}</SelectItem>
                <SelectItem value="policy_publish">{t("audit.filter.actionOptions.policyPublish")}</SelectItem>
                <SelectItem value="gateway_reboot">{t("audit.filter.actionOptions.gatewayReboot")}</SelectItem>
                <SelectItem value="visitor_issue">{t("audit.filter.actionOptions.visitorIssue")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("audit.tableCard.title")}</CardTitle>
          <CardDescription>{t("audit.tableCard.matched", { count: filtered.length })}</CardDescription>
        </CardHeader>
        <CardContent>
          {error ? (
            <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          ) : null}

          <div className="mb-3">
            <div className="mb-2 flex justify-end">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" size="sm">
                    <SlidersHorizontalIcon className="mr-1.5 size-4" />
                    {t("audit.columnDisplay")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {toggleableColumns.map((column) => (
                    <DropdownMenuCheckboxItem
                      key={column.id}
                      checked={column.getIsVisible()}
                      onCheckedChange={(checked) => column.toggleVisibility(Boolean(checked))}
                    >
                      {columnLabels[column.id] || column.id}
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            <ListPagination
              page={page}
              onPageChange={setPage}
              pageSize={pageSize}
              onPageSizeChange={setPageSize}
              hasNextPage={table.getCanNextPage()}
              disabled={loading || filtered.length === 0}
            />
          </div>

          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <TableHead key={header.id}>
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnCount} className="py-10 text-center text-muted-foreground">
                    {t("audit.loading")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnCount} className="py-8 text-center text-muted-foreground">
                    {query.trim() || actionFilter !== "all"
                      ? t("audit.empty.filtered")
                      : t("audit.empty.default")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}

function AuditWebhookPanel({
  config,
  configMissing,
  configError,
  deliveries,
  dispatchAuditLogID,
  dispatchTargetID,
  enabled,
  endpoint,
  actions,
  signingSecret,
  message,
  error,
  dateLocale,
  loading,
  saving,
  dispatching,
  onEnabledChange,
  onEndpointChange,
  onActionsChange,
  onSigningSecretChange,
  onDispatchTargetChange,
  onSave,
  onDispatch,
}: {
  config?: AuditWebhookConfig
  configMissing: boolean
  configError: string
  deliveries: AuditWebhookDelivery[]
  dispatchAuditLogID: string
  dispatchTargetID: string
  enabled: boolean
  endpoint: string
  actions: string
  signingSecret: string
  message: string
  error: string
  dateLocale: string
  loading: boolean
  saving: boolean
  dispatching: boolean
  onEnabledChange: (enabled: boolean) => void
  onEndpointChange: (value: string) => void
  onActionsChange: (value: string) => void
  onSigningSecretChange: (value: string) => void
  onDispatchTargetChange: (value: string) => void
  onSave: () => void
  onDispatch: () => void
}) {
  const statusLabel = loading ? "Checking" : config?.enabled ? "Enabled" : configMissing ? "Not configured" : "Disabled"
  const statusVariant = config?.enabled ? "default" : "outline"
  const latestDelivery = deliveries[0]

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <WebhookIcon className="size-4 text-content-subtle" />
              Audit Webhook
            </CardTitle>
            <CardDescription>Endpoint, filters, and delivery history.</CardDescription>
          </div>
          <Badge variant={statusVariant}>{statusLabel}</Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {configError ? (
          <div className="rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{configError}</div>
        ) : null}
        {error ? (
          <div className="rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
        ) : null}
        {message ? (
          <div className="rounded-[6px] border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">{message}</div>
        ) : null}

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              onSave()
            }}
          >
            <div className="flex items-center justify-between rounded-[6px] border border-line-subtle px-4 py-3">
              <div>
                <p className="text-sm font-medium text-content-heading">Delivery enabled</p>
                <p className="mt-1 text-xs text-content-subtle">When disabled, manual dispatch returns a conflict.</p>
              </div>
              <ToggleSwitch enabled={enabled} onToggle={() => onEnabledChange(!enabled)} />
            </div>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Endpoint</span>
              <Input
                value={endpoint}
                onChange={(event) => onEndpointChange(event.target.value)}
                placeholder="https://example.com/hooks/audit"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Action filter</span>
              <Input
                value={actions}
                onChange={(event) => onActionsChange(event.target.value)}
                placeholder="gateway_reboot, tenant_update"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Signing secret</span>
              <Input
                type="password"
                value={signingSecret}
                onChange={(event) => onSigningSecretChange(event.target.value)}
                placeholder={config?.updated_at ? "Leave blank to keep current secret" : "Optional HMAC secret"}
              />
            </label>
            <div className="flex flex-wrap gap-3">
              <Button type="submit" disabled={saving || (enabled && !endpoint.trim())}>
                {saving ? "Saving..." : "Save Webhook"}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={dispatching || !dispatchTargetID}
                onClick={onDispatch}
              >
                <SendIcon className="mr-1.5 size-4" />
                {dispatching ? "Dispatching..." : "Dispatch Latest"}
              </Button>
            </div>
          </form>

          <div className="space-y-4">
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Dispatch audit log ID</span>
              <Input
                value={dispatchAuditLogID}
                onChange={(event) => onDispatchTargetChange(event.target.value)}
                placeholder={dispatchTargetID || "aud_3002"}
              />
            </label>
            <div className="rounded-[6px] border border-line-subtle px-4 py-3 text-sm">
              <p className="font-medium text-content-heading">Latest delivery</p>
              {latestDelivery ? (
                <div className="mt-3 space-y-1 text-content-subtle">
                  <p>
                    <span className="font-medium text-content-heading">{latestDelivery.status}</span> · {latestDelivery.action}
                  </p>
                  <p>{latestDelivery.id} · HTTP {latestDelivery.http_status || "-"}</p>
                  <p>{new Date(latestDelivery.dispatched_at).toLocaleString(dateLocale)}</p>
                  {latestDelivery.error ? <p className="text-red-700">{latestDelivery.error}</p> : null}
                </div>
              ) : (
                <p className="mt-3 text-content-subtle">No webhook deliveries yet.</p>
              )}
            </div>
          </div>
        </div>

        <div className="overflow-hidden rounded-[6px] border border-line-subtle">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Delivery</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Attempts</TableHead>
                <TableHead>Dispatched</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-6 text-center text-muted-foreground">
                    No delivery records.
                  </TableCell>
                </TableRow>
              ) : (
                deliveries.map((delivery) => (
                  <TableRow key={delivery.id}>
                    <TableCell>
                      <TableCellText className="max-w-[11rem] font-medium">{delivery.id}</TableCellText>
                    </TableCell>
                    <TableCell>{delivery.action}</TableCell>
                    <TableCell>
                      <Badge variant={delivery.status === "success" ? "default" : "destructive"}>{delivery.status}</Badge>
                    </TableCell>
                    <TableCell>{delivery.attempt_count || 0}</TableCell>
                    <TableCell>{new Date(delivery.dispatched_at).toLocaleString(dateLocale)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  )
}
