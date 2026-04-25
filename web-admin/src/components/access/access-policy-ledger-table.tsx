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
import { type AccessPolicy } from "@/lib/api"

type PolicyLedgerRow = {
  policy: AccessPolicy
  scopeLabel: string
  scheduleLabel: string
  membersLabel: string
  statusLabel: string
  statusVariant: "outline" | "secondary"
}

type AccessPolicyLedgerTableProps = {
  rows: PolicyLedgerRow[]
  emptyState: string
  onEdit: (policy: AccessPolicy) => void
}

export function AccessPolicyLedgerTable({
  rows,
  emptyState,
  onEdit,
}: AccessPolicyLedgerTableProps) {
  const { t } = useTranslation()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState("")
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: 25,
  })
  const columns = useMemo<ColumnDef<PolicyLedgerRow>[]>(
    () => [
      {
        id: "name",
        accessorFn: (row) => row.policy.name,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("accessPage.components.policyLedgerTable.columns.policy", { defaultValue: "Policy" })}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[14rem] font-medium">{row.original.policy.name}</TableCellText>,
      },
      {
        id: "scope",
        accessorFn: (row) => row.scopeLabel,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("accessPage.components.policyLedgerTable.columns.scope", { defaultValue: "Scope" })}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[16rem]">{row.original.scopeLabel}</TableCellText>,
      },
      {
        id: "schedule",
        accessorFn: (row) => row.scheduleLabel,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("accessPage.components.policyLedgerTable.columns.schedule", { defaultValue: "Schedule" })}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[12rem]">{row.original.scheduleLabel}</TableCellText>,
      },
      {
        id: "members",
        accessorFn: (row) => row.membersLabel,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("accessPage.components.policyLedgerTable.columns.members", { defaultValue: "Members" })}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => row.original.membersLabel,
      },
      {
        id: "status",
        accessorFn: (row) => row.statusLabel,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("accessPage.components.policyLedgerTable.columns.status", { defaultValue: "Status" })}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <Badge variant={row.original.statusVariant}>{row.original.statusLabel}</Badge>,
      },
      {
        id: "actions",
        header: () => <span className="block text-right">{t("accessPage.components.policyLedgerTable.columns.actions", { defaultValue: "Actions" })}</span>,
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <div className="text-right">
            <Button variant="outline" size="sm" onClick={() => onEdit(row.original.policy)}>
              {t("accessPage.components.policyLedgerTable.edit", { defaultValue: "Edit" })}
            </Button>
          </div>
        ),
      },
    ],
    [onEdit]
  )
  const table = useReactTable({
    columns,
    data: rows,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.policy.id,
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
        row.original.policy.name,
        row.original.scopeLabel,
        row.original.scheduleLabel,
        row.original.membersLabel,
        row.original.statusLabel,
      ].some((value) => value.toLowerCase().includes(query))
    },
  })
  const columnLabels: Record<string, string> = {
    members: t("accessPage.components.policyLedgerTable.columns.members", { defaultValue: "Members" }),
    name: t("accessPage.components.policyLedgerTable.columns.policy", { defaultValue: "Policy" }),
    schedule: t("accessPage.components.policyLedgerTable.columns.schedule", { defaultValue: "Schedule" }),
    scope: t("accessPage.components.policyLedgerTable.columns.scope", { defaultValue: "Scope" }),
    status: t("accessPage.components.policyLedgerTable.columns.status", { defaultValue: "Status" }),
  }
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const filteredRowCount = table.getFilteredRowModel().rows.length
  const visibleColumnCount = table.getVisibleLeafColumns().length

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 rounded-lg border bg-muted/10 px-3 py-2">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <Input
            className="md:max-w-sm"
            value={globalFilter}
            onChange={(event) => setGlobalFilter(event.target.value)}
            placeholder={t("accessPage.components.policyLedgerTable.search", {
              defaultValue: "Filter by policy / scope / schedule / status",
            })}
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="outline" size="sm" className="w-full md:w-auto">
                <SlidersHorizontalIcon className="mr-1.5 size-4" />
                {t("accessPage.components.policyLedgerTable.columnDisplay", { defaultValue: "Columns" })}
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
