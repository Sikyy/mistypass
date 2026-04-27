import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { ArrowUpDownIcon, SlidersHorizontalIcon } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

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
import { type Gateway, type Tenant } from "@/lib/api"

type GatewayListCardProps = {
  platformViewer: boolean
  loading: boolean
  effectiveError: string
  filteredGateways: Gateway[]
  missingBuildingScope: boolean
  hasActiveGatewayFilters: boolean
  buildingAdmin: boolean
  tenantByID: Map<string, Tenant>
  statusLabel: (status: string) => string
  statusVariant: (status: string) => "outline" | "destructive" | "secondary"
}

export function GatewayListCard({
  platformViewer,
  loading,
  effectiveError,
  filteredGateways,
  missingBuildingScope,
  hasActiveGatewayFilters,
  buildingAdmin,
  tenantByID,
  statusLabel,
  statusVariant,
}: GatewayListCardProps) {
  const { t, i18n } = useTranslation()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState("")
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(() => ({
    bound_door_ids: false,
    building: !platformViewer,
    device_capacity: false,
  }))
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: 25,
  })
  const columns = useMemo<ColumnDef<Gateway>[]>(
    () => {
      const definition: ColumnDef<Gateway>[] = [
        {
          id: "id",
          accessorKey: "id",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.id")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[10rem] font-medium">{row.original.id}</TableCellText>,
        },
        {
          id: "serial_number",
          accessorKey: "serial_number",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.serialNumber")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[14rem]">{row.original.serial_number}</TableCellText>,
        },
        {
          id: "building",
          accessorFn: (row) => row.building_id || "-",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.building")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[10rem]">{row.original.building_id || t("gateways.list.table.emptyDash")}</TableCellText>,
        },
        {
          id: "device_capacity",
          accessorFn: (row) => row.device_capacity || 0,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.deviceCapacity")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => row.original.device_capacity || t("gateways.list.table.emptyDash"),
        },
        {
          id: "device_online",
          accessorFn: (row) => (row.devices ?? []).filter((device) => device.status === "online").length,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.deviceOnline")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) =>
            `${(row.original.devices ?? []).filter((device) => device.status === "online").length}/${row.original.devices?.length ?? 0}`,
        },
        {
          id: "status",
          accessorKey: "status",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.status")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <Badge variant={statusVariant(row.original.status)} className="capitalize">
              {statusLabel(row.original.status)}
            </Badge>
          ),
        },
        {
          id: "last_seen",
          accessorKey: "last_seen_at",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.lastSeen")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => new Date(row.original.last_seen_at).toLocaleString(i18n.language),
        },
        {
          id: "bound_door_ids",
          accessorFn: (row) => row.bound_door_ids?.join(", ") || "-",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.boundDoors")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[16rem]">{row.original.bound_door_ids?.join(", ") || t("gateways.list.table.emptyDash")}</TableCellText>,
        },
      ]
      if (platformViewer) {
        definition.splice(1, 0, {
          id: "tenant",
          accessorFn: (row) => tenantByID.get(row.tenant_id)?.name ?? row.tenant_id,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.list.table.tenant")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <TableCellText className="max-w-[12rem]">{tenantByID.get(row.original.tenant_id)?.name ?? row.original.tenant_id}</TableCellText>
          ),
        })
      }
      return definition
    },
    [platformViewer, statusLabel, statusVariant, tenantByID, t, i18n.language]
  )
  const table = useReactTable({
    columns,
    data: filteredGateways,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
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
        row.original.id,
        tenantByID.get(row.original.tenant_id)?.name ?? row.original.tenant_id,
        row.original.serial_number,
        row.original.building_id || "-",
        row.original.bound_door_ids?.join(", ") || "-",
      ].some((value) => value.toLowerCase().includes(query))
    },
  })
  const columnLabels: Record<string, string> = {
    bound_door_ids: t("gateways.list.table.boundDoors"),
    building: t("gateways.list.table.building"),
    device_capacity: t("gateways.list.table.deviceCapacity"),
    device_online: t("gateways.list.table.deviceOnline"),
    id: t("gateways.list.table.id"),
    last_seen: t("gateways.list.table.lastSeen"),
    serial_number: t("gateways.list.table.serialNumber"),
    status: t("gateways.list.table.status"),
    tenant: t("gateways.list.table.tenant"),
  }
  const filteredRowCount = table.getFilteredRowModel().rows.length
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const visibleColumnCount = table.getVisibleLeafColumns().length

  useEffect(() => {
    setPagination((current) => ({
      ...current,
      pageIndex: 0,
    }))
  }, [filteredGateways.length])

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("gateways.list.title")}</CardTitle>
        <CardDescription>
          {platformViewer ? t("gateways.list.descriptionPlatform") : t("gateways.list.descriptionDefault")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {effectiveError ? (
          <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {effectiveError}
          </div>
        ) : null}
        <div className="flex flex-col gap-2 rounded-lg border bg-muted/10 px-3 py-2">
          <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <Input
              className="min-w-0 md:max-w-sm"
              value={globalFilter}
              onChange={(event) => {
                setGlobalFilter(event.target.value)
                setPagination((current) => ({
                  ...current,
                  pageIndex: 0,
                }))
              }}
              placeholder={t("gateways.list.globalSearchPlaceholder")}
            />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button type="button" variant="outline" size="sm" className="w-full md:w-auto">
                  <SlidersHorizontalIcon className="mr-1.5 size-4" />
                  {t("gateways.list.columnDisplay")}
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
          disabled={loading || filteredRowCount === 0}
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
            {loading ? (
              <TableRow>
                <TableCell colSpan={visibleColumnCount} className="py-10 text-center text-muted-foreground">
                  {t("gateways.list.loading")}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading && filteredRowCount === 0 ? (
              <TableRow>
                <TableCell colSpan={visibleColumnCount} className="py-8 text-center text-muted-foreground">
                  {missingBuildingScope
                    ? t("gateways.list.empty.missingScope")
                    : hasActiveGatewayFilters || globalFilter.trim()
                      ? t("gateways.list.empty.filtered")
                      : buildingAdmin
                        ? t("gateways.list.empty.buildingScope")
                        : t("gateways.list.empty.default")}
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
  )
}
