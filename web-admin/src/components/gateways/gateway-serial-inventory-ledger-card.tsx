import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ArrowUpDownIcon, SlidersHorizontalIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

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
import { Textarea } from "@/components/ui/textarea"
import {
  type GatewaySerialInventoryItem,
  type GatewaySerialInventoryProductType,
  type GatewaySerialInventoryStatus,
  type Tenant,
} from "@/lib/api"

const batchStatusValues = ["available", "frozen", "scrapped"] as const
type BatchUpdateFormValues = {
  batch_serials?: string
  batch_status: "available" | "frozen" | "scrapped"
}

function parseBatchSerialNumbers(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(/[\s,;]+/g)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

type GatewaySerialInventoryLedgerCardProps = {
  platformViewer: boolean
  inventoryEditable: boolean
  inventoryFilterProductType: "all" | GatewaySerialInventoryProductType
  onInventoryFilterProductTypeChange: (value: "all" | GatewaySerialInventoryProductType) => void
  inventoryFilterStatus: "all" | GatewaySerialInventoryStatus
  onInventoryFilterStatusChange: (value: "all" | GatewaySerialInventoryStatus) => void
  inventoryFilterQuery: string
  onInventoryFilterQueryChange: (value: string) => void
  inventoryPage: number
  onInventoryPageChange: (page: number) => void
  inventoryPageSize: number
  onInventoryPageSizeChange: (pageSize: number) => void
  inventoryHasNextPage: boolean
  onResetFilters: () => void
  commandBusy: boolean
  tenantID: string
  onBatchUpdateSerialInventoryStatus: (payload: {
    status: "available" | "frozen" | "scrapped"
    manualSerialNumbers: string[]
  }) => Promise<boolean>
  onClearBatchTargets: () => void
  selectedInventorySerialNumbersLength: number
  visibleSerialInventory: GatewaySerialInventoryItem[]
  allVisibleInventorySelected: boolean
  onSelectAllVisibleSerialInventory: (checked: boolean) => void
  selectedInventorySerialSet: Set<string>
  onSelectSerialInventory: (serialNumber: string, checked: boolean) => void
  tenantByID: Map<string, Tenant>
  onUpdateSerialInventoryStatus: (item: GatewaySerialInventoryItem, status: "available" | "frozen" | "scrapped") => void
  serialInventoryProductTypeLabel: (productType: GatewaySerialInventoryItem["product_type"]) => string
  serialInventoryStatusLabel: (status: GatewaySerialInventoryItem["status"]) => string
  serialInventoryStatusVariant: (status: GatewaySerialInventoryItem["status"]) => "outline" | "destructive" | "secondary"
}

export function GatewaySerialInventoryLedgerCard({
  platformViewer,
  inventoryEditable,
  inventoryFilterProductType,
  onInventoryFilterProductTypeChange,
  inventoryFilterStatus,
  onInventoryFilterStatusChange,
  inventoryFilterQuery,
  onInventoryFilterQueryChange,
  inventoryPage,
  onInventoryPageChange,
  inventoryPageSize,
  onInventoryPageSizeChange,
  inventoryHasNextPage,
  onResetFilters,
  commandBusy,
  tenantID,
  onBatchUpdateSerialInventoryStatus,
  onClearBatchTargets,
  selectedInventorySerialNumbersLength,
  visibleSerialInventory,
  allVisibleInventorySelected,
  onSelectAllVisibleSerialInventory,
  selectedInventorySerialSet,
  onSelectSerialInventory,
  tenantByID,
  onUpdateSerialInventoryStatus,
  serialInventoryProductTypeLabel,
  serialInventoryStatusLabel,
  serialInventoryStatusVariant,
}: GatewaySerialInventoryLedgerCardProps) {
  const { t, i18n } = useTranslation()
  const batchUpdateSchema = useMemo(
    () =>
      z.object({
        batch_serials: z
          .string()
          .trim()
          .max(100000, t("gateways.inventoryLedger.validation.batchSerialsMax"))
          .optional()
          .or(z.literal("")),
        batch_status: z.enum(batchStatusValues),
      }),
    [t]
  )
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const batchUpdateForm = useForm<BatchUpdateFormValues>({
    resolver: zodResolver(batchUpdateSchema),
    defaultValues: {
      batch_serials: "",
      batch_status: "frozen",
    },
  })
  const batchFormError =
    batchUpdateForm.formState.errors.batch_serials?.message ||
    batchUpdateForm.formState.errors.batch_status?.message ||
    ""
  const watchedBatchSerials = batchUpdateForm.watch("batch_serials")
  const manualBatchSerialNumbers = useMemo(() => parseBatchSerialNumbers(watchedBatchSerials || ""), [watchedBatchSerials])
  const batchTargetSerialNumbersLength =
    manualBatchSerialNumbers.length > 0 ? manualBatchSerialNumbers.length : selectedInventorySerialNumbersLength
  const columns = useMemo<ColumnDef<GatewaySerialInventoryItem>[]>(
    () => {
      const definition: ColumnDef<GatewaySerialInventoryItem>[] = [
        {
          id: "serial_number",
          accessorKey: "serial_number",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.inventoryLedger.table.serialNumber")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[14rem] font-medium">{row.original.serial_number}</TableCellText>,
        },
        {
          id: "product_type",
          accessorKey: "product_type",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.inventoryLedger.table.productType")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => serialInventoryProductTypeLabel(row.original.product_type),
        },
        {
          id: "status",
          accessorKey: "status",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.inventoryLedger.table.status")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <Badge variant={serialInventoryStatusVariant(row.original.status)}>{serialInventoryStatusLabel(row.original.status)}</Badge>,
        },
        {
          id: "consumed_gateway_id",
          accessorFn: (row) => row.consumed_gateway_id || "-",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.inventoryLedger.table.consumedGateway")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => row.original.consumed_gateway_id || t("gateways.inventoryLedger.table.emptyDash"),
        },
        {
          id: "updated_at",
          accessorKey: "updated_at",
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.inventoryLedger.table.updatedAt")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => new Date(row.original.updated_at).toLocaleString(i18n.language),
        },
      ]
      if (inventoryEditable) {
        definition.unshift({
          id: "select",
          header: () => (
            <input
              aria-label="select all visible serial inventory rows"
              type="checkbox"
              className="size-4 rounded border"
              disabled={visibleSerialInventory.length === 0}
              checked={allVisibleInventorySelected}
              onChange={(event) => onSelectAllVisibleSerialInventory(event.target.checked)}
            />
          ),
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => (
            <input
              aria-label={`select serial inventory ${row.original.serial_number}`}
              type="checkbox"
              className="size-4 rounded border"
              checked={selectedInventorySerialSet.has(row.original.serial_number)}
              onChange={(event) => onSelectSerialInventory(row.original.serial_number, event.target.checked)}
            />
          ),
        })
      }
      if (platformViewer) {
        definition.splice(inventoryEditable ? 3 : 2, 0, {
          id: "tenant",
          accessorFn: (row) => tenantByID.get(row.tenant_id)?.name ?? row.tenant_id,
          header: ({ column }) => (
            <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("gateways.inventoryLedger.table.tenant")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <TableCellText className="max-w-[12rem]">{tenantByID.get(row.original.tenant_id)?.name ?? row.original.tenant_id}</TableCellText>,
        })
      }
      if (inventoryEditable) {
        definition.push({
          id: "actions",
          header: () => t("gateways.inventoryLedger.table.actions"),
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => (
            <div className="flex flex-wrap gap-1">
              <Button
                size="sm"
                variant="outline"
                disabled={commandBusy || row.original.status === "available"}
                onClick={() => {
                  onUpdateSerialInventoryStatus(row.original, "available")
                }}
              >
                {t("gateways.inventoryLedger.actions.available")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={commandBusy || row.original.status === "frozen" || row.original.status === "scrapped"}
                onClick={() => {
                  onUpdateSerialInventoryStatus(row.original, "frozen")
                }}
              >
                {t("gateways.inventoryLedger.actions.frozen")}
              </Button>
              <Button
                size="sm"
                variant="destructive"
                disabled={commandBusy || row.original.status === "scrapped"}
                onClick={() => {
                  onUpdateSerialInventoryStatus(row.original, "scrapped")
                }}
              >
                {t("gateways.inventoryLedger.actions.scrapped")}
              </Button>
            </div>
          ),
        })
      }
      return definition
    },
    [
      allVisibleInventorySelected,
      commandBusy,
      inventoryEditable,
      onSelectAllVisibleSerialInventory,
      onSelectSerialInventory,
      onUpdateSerialInventoryStatus,
      platformViewer,
      selectedInventorySerialSet,
      serialInventoryProductTypeLabel,
      serialInventoryStatusLabel,
      serialInventoryStatusVariant,
      t,
      i18n.language,
      tenantByID,
      visibleSerialInventory.length,
    ]
  )
  const table = useReactTable({
    columns,
    data: visibleSerialInventory,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility,
      sorting,
    },
    onColumnVisibilityChange: setColumnVisibility,
    onSortingChange: setSorting,
  })
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const visibleColumnCount = table.getVisibleLeafColumns().length
  const columnLabels: Record<string, string> = {
    consumed_gateway_id: t("gateways.inventoryLedger.table.consumedGateway"),
    product_type: t("gateways.inventoryLedger.table.productType"),
    serial_number: t("gateways.inventoryLedger.table.serialNumber"),
    status: t("gateways.inventoryLedger.table.status"),
    tenant: t("gateways.inventoryLedger.table.tenant"),
    updated_at: t("gateways.inventoryLedger.table.updatedAt"),
  }

  async function onSubmitBatchUpdate(values: BatchUpdateFormValues) {
    const succeeded = await onBatchUpdateSerialInventoryStatus({
      status: values.batch_status,
      manualSerialNumbers: parseBatchSerialNumbers(values.batch_serials || ""),
    })
    if (succeeded) {
      batchUpdateForm.reset({
        batch_serials: "",
        batch_status: values.batch_status,
      })
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("gateways.inventoryLedger.title")}</CardTitle>
        <CardDescription>
          {inventoryEditable
            ? t("gateways.inventoryLedger.descriptionEditable")
            : t("gateways.inventoryLedger.descriptionReadonly")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid gap-3 lg:grid-cols-[220px_220px_1fr_auto]">
          <Select
            value={inventoryFilterProductType}
            onValueChange={(value: "all" | GatewaySerialInventoryProductType) => onInventoryFilterProductTypeChange(value)}
          >
            <SelectTrigger aria-label={t("gateways.inventoryLedger.filter.productTypePlaceholder")}>
              <SelectValue placeholder={t("gateways.inventoryLedger.filter.productTypePlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("gateways.inventoryLedger.filter.allProductTypes")}</SelectItem>
              <SelectItem value="gateway">{t("gateways.inventoryProductType.gateway")}</SelectItem>
              <SelectItem value="reader">{t("gateways.inventoryProductType.reader")}</SelectItem>
              <SelectItem value="controller">{t("gateways.inventoryProductType.controller")}</SelectItem>
              <SelectItem value="relay">{t("gateways.inventoryProductType.relay")}</SelectItem>
              <SelectItem value="sensor">{t("gateways.inventoryProductType.sensor")}</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={inventoryFilterStatus}
            onValueChange={(value: "all" | GatewaySerialInventoryStatus) => onInventoryFilterStatusChange(value)}
          >
            <SelectTrigger aria-label={t("gateways.inventoryLedger.filter.statusPlaceholder")}>
              <SelectValue placeholder={t("gateways.inventoryLedger.filter.statusPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("gateways.inventoryLedger.filter.allStatuses")}</SelectItem>
              <SelectItem value="available">{t("gateways.inventoryStatus.available")}</SelectItem>
              <SelectItem value="consumed">{t("gateways.inventoryStatus.consumed")}</SelectItem>
              <SelectItem value="frozen">{t("gateways.inventoryStatus.frozen")}</SelectItem>
              <SelectItem value="scrapped">{t("gateways.inventoryStatus.scrapped")}</SelectItem>
            </SelectContent>
          </Select>
          <Input
            value={inventoryFilterQuery}
            onChange={(event) => onInventoryFilterQueryChange(event.target.value)}
            aria-label={t("gateways.inventoryLedger.filter.queryPlaceholder")}
            placeholder={t("gateways.inventoryLedger.filter.queryPlaceholder")}
          />
          <Button type="button" variant="outline" onClick={onResetFilters}>
            {t("gateways.inventoryLedger.filter.reset")}
          </Button>
        </div>

        {inventoryEditable ? (
          <form className="space-y-2 rounded-lg border bg-muted/20 p-3" onSubmit={batchUpdateForm.handleSubmit(onSubmitBatchUpdate)}>
            <p className="text-xs font-medium text-muted-foreground">{t("gateways.inventoryLedger.batch.title")}</p>
            <Textarea
              {...batchUpdateForm.register("batch_serials")}
              rows={3}
              placeholder={t("gateways.inventoryLedger.batch.serialsPlaceholder")}
            />
            <div className="grid gap-2 md:grid-cols-[220px_auto_auto]">
              <Controller
                control={batchUpdateForm.control}
                name="batch_status"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder={t("gateways.inventoryLedger.batch.targetStatusPlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="available">{t("gateways.inventoryLedger.batch.targetStatus.available")}</SelectItem>
                      <SelectItem value="frozen">{t("gateways.inventoryLedger.batch.targetStatus.frozen")}</SelectItem>
                      <SelectItem value="scrapped">{t("gateways.inventoryLedger.batch.targetStatus.scrapped")}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
              <Button
                type="submit"
                variant="secondary"
                disabled={commandBusy || batchUpdateForm.formState.isSubmitting || !tenantID.trim() || batchTargetSerialNumbersLength === 0}
              >
                {t("gateways.inventoryLedger.batch.submit")}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  batchUpdateForm.setValue("batch_serials", "")
                  onClearBatchTargets()
                }}
              >
                {t("gateways.inventoryLedger.batch.clear")}
              </Button>
            </div>
            <p className="mp-kpi-note">
              {t("gateways.inventoryLedger.batch.summary", {
                total: batchTargetSerialNumbersLength,
                manual: manualBatchSerialNumbers.length,
                selected: selectedInventorySerialNumbersLength,
              })}
            </p>
            {batchFormError ? (
              <p className="text-sm text-destructive">{batchFormError}</p>
            ) : null}
          </form>
        ) : null}

        <ListPagination
          page={inventoryPage}
          onPageChange={onInventoryPageChange}
          pageSize={inventoryPageSize}
          onPageSizeChange={onInventoryPageSizeChange}
          hasNextPage={inventoryHasNextPage}
          disabled={visibleSerialInventory.length === 0}
        />
        <div className="flex justify-end">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="outline" size="sm">
                <SlidersHorizontalIcon className="mr-1.5 size-4" />
                {t("gateways.inventoryLedger.columnDisplay")}
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
            {visibleSerialInventory.length === 0 ? (
              <TableRow>
                <TableCell colSpan={visibleColumnCount} className="py-6 text-center text-muted-foreground">
                  {t("gateways.inventoryLedger.empty")}
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
      </CardContent>
    </Card>
  )
}
