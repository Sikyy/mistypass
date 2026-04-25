import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { ArrowUpDownIcon, SlidersHorizontalIcon } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { ListPagination } from "@/components/ui/list-pagination"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type TemporaryAccess } from "@/lib/api"

type LedgerBadgeVariant = "outline" | "secondary" | "destructive"

export type AccessGrantLedgerRow = {
  grant: TemporaryAccess
  tenantLabel?: string
  scopeLabel: string
  granteeLabel: string
  deliveryLabel: string
  authorizedByRole: string
  authorizedByEmail: string
  authorizedAtLabel: string
  statusLabel: string
  statusVariant: LedgerBadgeVariant
  validUntilLabel: string
  remainingLabel: string
  remainingVariant: LedgerBadgeVariant
}

type AccessGrantLedgerTableProps = {
  rows: AccessGrantLedgerRow[]
  platformViewer: boolean
  emptyState: string
  onOpenGrant: (grant: TemporaryAccess) => void
}

export function AccessGrantLedgerTable({
  rows,
  platformViewer,
  emptyState,
  onOpenGrant,
}: AccessGrantLedgerTableProps) {
  const { t } = useTranslation()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState("")
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: 25,
  })
  const columns = useMemo<ColumnDef<AccessGrantLedgerRow>[]>(
    () => {
      const definition: ColumnDef<AccessGrantLedgerRow>[] = [
        {
          id: "grant_id",
          accessorFn: (row) => row.grant.id,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.grantID", { defaultValue: "Grant ID" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem] font-medium">{row.original.grant.id}</TableCellText>,
        },
        {
          id: "scope",
          accessorFn: (row) => row.scopeLabel,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.scope", { defaultValue: "Scope" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[16rem]">{row.original.scopeLabel}</TableCellText>,
        },
        {
          id: "grantee",
          accessorFn: (row) => row.granteeLabel,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.grantee", { defaultValue: "Grantee" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem]">{row.original.granteeLabel}</TableCellText>,
        },
        {
          id: "delivery",
          accessorFn: (row) => row.deliveryLabel,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.delivery", { defaultValue: "Delivery" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => row.original.deliveryLabel,
        },
        {
          id: "authorized_by",
          accessorFn: (row) => row.authorizedByEmail,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.authorizedBy", { defaultValue: "Authorized by" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <div>
              <div className="mp-kpi-note">{row.original.authorizedByRole}</div>
              <TableCellText className="max-w-[14rem]">{row.original.authorizedByEmail}</TableCellText>
            </div>
          ),
        },
        {
          id: "authorized_at",
          accessorFn: (row) => row.authorizedAtLabel,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.authorizedAt", { defaultValue: "Authorized at" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => row.original.authorizedAtLabel,
        },
        {
          id: "status",
          accessorFn: (row) => row.statusLabel,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.status", { defaultValue: "Status" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <Badge variant={row.original.statusVariant}>{row.original.statusLabel}</Badge>,
        },
        {
          id: "validity",
          accessorFn: (row) => row.remainingLabel,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.validity", { defaultValue: "Validity countdown" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <div>
              <div>{row.original.validUntilLabel}</div>
              <Badge variant={row.original.remainingVariant} className="mt-1">
                {row.original.remainingLabel}
              </Badge>
            </div>
          ),
        },
        {
          id: "actions",
          header: () => <span className="block text-right">{t("accessPage.components.grantLedgerTable.columns.details", { defaultValue: "Details" })}</span>,
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => (
            <div className="text-right">
              <Button variant="outline" size="sm" onClick={() => onOpenGrant(row.original.grant)}>
                {t("accessPage.components.grantLedgerTable.view", { defaultValue: "View" })}
              </Button>
            </div>
          ),
        },
      ]
      if (platformViewer) {
        definition.splice(1, 0, {
          id: "tenant",
          accessorFn: (row) => row.tenantLabel || "-",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("accessPage.components.grantLedgerTable.columns.tenant", { defaultValue: "Tenant" })}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[13rem]">{row.original.tenantLabel || "-"}</TableCellText>,
        })
      }
      return definition
    },
    [onOpenGrant, platformViewer]
  )
  const table = useReactTable({
    columns,
    data: rows,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.grant.id,
    state: {
      columnVisibility,
      globalFilter,
      pagination,
      sorting,
    },
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange: setGlobalFilter,
    onPaginationChange: setPagination,
    onSortingChange: setSorting,
    globalFilterFn: (row, _columnID, filterValue) => {
      const query = String(filterValue ?? "")
        .trim()
        .toLowerCase()
      if (!query) {
        return true
      }
      return [
        row.original.grant.id,
        row.original.tenantLabel || "-",
        row.original.scopeLabel,
        row.original.granteeLabel,
        row.original.deliveryLabel,
        row.original.authorizedByRole,
        row.original.authorizedByEmail,
        row.original.authorizedAtLabel,
        row.original.statusLabel,
        row.original.validUntilLabel,
        row.original.remainingLabel,
      ].some((value) => value.toLowerCase().includes(query))
    },
  })
  const columnLabels: Record<string, string> = {
    authorized_at: t("accessPage.components.grantLedgerTable.columns.authorizedAt", { defaultValue: "Authorized at" }),
    authorized_by: t("accessPage.components.grantLedgerTable.columns.authorizedBy", { defaultValue: "Authorized by" }),
    delivery: t("accessPage.components.grantLedgerTable.columns.delivery", { defaultValue: "Delivery" }),
    grant_id: t("accessPage.components.grantLedgerTable.columns.grantID", { defaultValue: "Grant ID" }),
    grantee: t("accessPage.components.grantLedgerTable.columns.grantee", { defaultValue: "Grantee" }),
    scope: t("accessPage.components.grantLedgerTable.columns.scope", { defaultValue: "Scope" }),
    status: t("accessPage.components.grantLedgerTable.columns.status", { defaultValue: "Status" }),
    tenant: t("accessPage.components.grantLedgerTable.columns.tenant", { defaultValue: "Tenant" }),
    validity: t("accessPage.components.grantLedgerTable.columns.validity", { defaultValue: "Validity countdown" }),
  }
  const filteredRowCount = table.getFilteredRowModel().rows.length
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const visibleColumnCount = table.getVisibleLeafColumns().length

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 rounded-lg border bg-muted/10 px-3 py-2">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <Input
            className="md:max-w-sm"
            value={globalFilter}
            onChange={(event) => setGlobalFilter(event.target.value)}
            aria-label={t("accessPage.components.grantLedgerTable.search", {
              defaultValue: "Filter by grant ID / tenant / scope / grantee",
            })}
            placeholder={t("accessPage.components.grantLedgerTable.search", {
              defaultValue: "Filter by grant ID / tenant / scope / grantee",
            })}
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="outline" size="sm" className="w-full md:w-auto">
                <SlidersHorizontalIcon className="mr-1.5 size-4" />
                {t("accessPage.components.grantLedgerTable.columnDisplay", { defaultValue: "Columns" })}
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
      </div>
      <ListPagination
        page={table.getState().pagination.pageIndex + 1}
        onPageChange={(page) =>
          setPagination((current) => ({
            ...current,
            pageIndex: Math.max(0, page - 1),
          }))
        }
        pageSize={table.getState().pagination.pageSize}
        onPageSizeChange={(pageSize) =>
          setPagination({
            pageIndex: 0,
            pageSize,
          })
        }
        hasNextPage={table.getCanNextPage()}
        disabled={filteredRowCount === 0}
      />
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
          {filteredRowCount === 0 ? (
            <TableRow>
              <TableCell colSpan={visibleColumnCount} className="py-8 text-center text-muted-foreground">
                {emptyState}
              </TableCell>
            </TableRow>
          ) : null}
          {table.getRowModel().rows.map((row) => (
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
    </div>
  )
}
