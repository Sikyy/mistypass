import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { ArrowUpDownIcon, FileSearchIcon, ShieldIcon, SlidersHorizontalIcon } from "lucide-react"
import { useQuery } from "@tanstack/react-query"
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
import { ListPagination } from "@/components/ui/list-pagination"
import { listAuditLogs, type AuditLog } from "@/lib/api"

type AuditPageProps = {
  token: string
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

export function AuditPage({ token }: AuditPageProps) {
  const { t, i18n } = useTranslation()
  const [query, setQuery] = useState("")
  const [actionFilter, setActionFilter] = useState<"all" | AuditAction>("all")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const auditLogsQuery = useQuery({
    queryKey: ["audit-logs"],
    queryFn: () => listAuditLogs(token),
  })
  const rows: AuditLog[] = auditLogsQuery.data ?? []
  const loading = auditLogsQuery.isPending
  const error =
    auditLogsQuery.error instanceof Error ? auditLogsQuery.error.message : ""
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

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("audit.eyebrow")}</p>
        <h1 className="mp-page-title">{t("audit.title")}</h1>
        <p className="mp-page-description">
          {t("audit.description")}
        </p>
      </div>

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
