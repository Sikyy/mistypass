import { type Column, flexRender, type Table as TanstackTable } from "@tanstack/react-table"
import { SlidersHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

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
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type WalletPassInstance, type WalletPassTemplate } from "@/lib/api"

export type WalletPassStatusFilter = "all" | "issued" | "active" | "suspended" | "revoked"
export type WalletPassTargetTypeFilter = "all" | "user" | "visitor"
export type WalletBatchPassAction = "" | "activate" | "suspend" | "revoke"

type WalletIssuedPassesCardProps = {
  templates: WalletPassTemplate[]
  passTable: TanstackTable<WalletPassInstance>
  passQuery: string
  passStatusFilter: WalletPassStatusFilter
  passTargetTypeFilter: WalletPassTargetTypeFilter
  passTemplateFilter: string
  filteredPassCount: number
  selectedFilteredPassCount: number
  passesCount: number
  hasPassFilters: boolean
  writable: boolean
  loading: boolean
  batchUpdatingPassAction: WalletBatchPassAction
  passPage: number
  passPageSize: number
  onPassQueryChange: (value: string) => void
  onPassStatusFilterChange: (value: WalletPassStatusFilter) => void
  onPassTargetTypeFilterChange: (value: WalletPassTargetTypeFilter) => void
  onPassTemplateFilterChange: (value: string) => void
  onPassPageChange: (value: number) => void
  onPassPageSizeChange: (value: number) => void
  onClearFilters: () => void
  onClearSelection: () => void
  onUpdateSelectedPasses: (action: Exclude<WalletBatchPassAction, "">) => void
}

function columnDisplayLabel(
  column: Column<WalletPassInstance, unknown>,
  labels: Record<string, string>
) {
  return labels[column.id] || column.id
}

export function WalletIssuedPassesCard({
  templates,
  passTable,
  passQuery,
  passStatusFilter,
  passTargetTypeFilter,
  passTemplateFilter,
  filteredPassCount,
  selectedFilteredPassCount,
  passesCount,
  hasPassFilters,
  writable,
  loading,
  batchUpdatingPassAction,
  passPage,
  passPageSize,
  onPassQueryChange,
  onPassStatusFilterChange,
  onPassTargetTypeFilterChange,
  onPassTemplateFilterChange,
  onPassPageChange,
  onPassPageSizeChange,
  onClearFilters,
  onClearSelection,
  onUpdateSelectedPasses,
}: WalletIssuedPassesCardProps) {
  const { t } = useTranslation()
  const passVisibleColumnCount = passTable.getVisibleLeafColumns().length
  const passToggleableColumns = passTable.getAllLeafColumns().filter((column) => column.getCanHide())
  const batchActionDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : batchUpdatingPassAction.length > 0
      ? t("walletPage.disabledReasons.busy")
      : selectedFilteredPassCount === 0
        ? t("walletPage.disabledReasons.selectPasses")
        : ""
  const passColumnLabels: Record<string, string> = {
    expires_at: t("walletPage.table.columns.expiresAt"),
    save_link: t("walletPage.table.columns.saveLink"),
    status: t("walletPage.table.columns.status"),
    target: t("walletPage.table.columns.target"),
    template: t("walletPage.table.columns.template"),
    updated_at: t("walletPage.table.columns.updatedAt"),
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.cards.issuedPasses.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.cards.issuedPasses.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,180px)_minmax(0,180px)_minmax(0,220px)]">
          <Input
            value={passQuery}
            onChange={(event) => onPassQueryChange(event.target.value)}
            placeholder={t("walletPage.placeholders.passSearch")}
          />
          <Select value={passStatusFilter} onValueChange={(value) => onPassStatusFilterChange(value as WalletPassStatusFilter)}>
            <SelectTrigger className="w-full min-w-0">
              <SelectValue placeholder={t("walletPage.placeholders.passStatus")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("walletPage.filters.passStatus.all")}</SelectItem>
              <SelectItem value="issued">{t("walletPage.filters.passStatus.issued")}</SelectItem>
              <SelectItem value="active">{t("walletPage.filters.passStatus.active")}</SelectItem>
              <SelectItem value="suspended">{t("walletPage.filters.passStatus.suspended")}</SelectItem>
              <SelectItem value="revoked">{t("walletPage.filters.passStatus.revoked")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={passTargetTypeFilter} onValueChange={(value) => onPassTargetTypeFilterChange(value as WalletPassTargetTypeFilter)}>
            <SelectTrigger className="w-full min-w-0">
              <SelectValue placeholder={t("walletPage.placeholders.targetType")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("walletPage.filters.targetType.all")}</SelectItem>
              <SelectItem value="user">{t("walletPage.filters.targetType.user")}</SelectItem>
              <SelectItem value="visitor">{t("walletPage.filters.targetType.visitor")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={passTemplateFilter} onValueChange={onPassTemplateFilterChange}>
            <SelectTrigger className="w-full min-w-0">
              <SelectValue placeholder={t("walletPage.placeholders.template")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("walletPage.filters.template.all")}</SelectItem>
              {templates.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-muted/10 px-4 py-3">
          <div className="space-y-1">
            <p className="text-sm font-medium">{t("walletPage.cards.issuedPasses.matchedCount", { count: filteredPassCount })}</p>
            <p className="mp-kpi-note">
              {selectedFilteredPassCount > 0
                ? t("walletPage.cards.issuedPasses.selectedCount", { count: selectedFilteredPassCount })
                : t("walletPage.cards.issuedPasses.selectionHint")}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {hasPassFilters ? (
              <Button size="sm" variant="outline" onClick={onClearFilters}>
                {t("walletPage.actions.clearFilters")}
              </Button>
            ) : null}
            {selectedFilteredPassCount > 0 ? (
              <Button
                size="sm"
                variant="outline"
                onClick={onClearSelection}
                disabled={!writable || batchUpdatingPassAction.length > 0}
                title={!writable ? t("walletPage.disabledReasons.readOnly") : batchUpdatingPassAction.length > 0 ? t("walletPage.disabledReasons.busy") : undefined}
              >
                {t("walletPage.actions.clearSelection")}
              </Button>
            ) : null}
            <Button
              size="sm"
              variant="outline"
              onClick={() => onUpdateSelectedPasses("suspend")}
              disabled={!writable || selectedFilteredPassCount === 0 || batchUpdatingPassAction.length > 0}
              title={batchActionDisabledReason || undefined}
            >
              {batchUpdatingPassAction === "suspend" ? t("walletPage.actions.batchSuspending") : t("walletPage.actions.batchSuspend")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => onUpdateSelectedPasses("activate")}
              disabled={!writable || selectedFilteredPassCount === 0 || batchUpdatingPassAction.length > 0}
              title={batchActionDisabledReason || undefined}
            >
              {batchUpdatingPassAction === "activate" ? t("walletPage.actions.batchActivating") : t("walletPage.actions.batchActivate")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => onUpdateSelectedPasses("revoke")}
              disabled={!writable || selectedFilteredPassCount === 0 || batchUpdatingPassAction.length > 0}
              title={batchActionDisabledReason || undefined}
            >
              {batchUpdatingPassAction === "revoke" ? t("walletPage.actions.batchRevoking") : t("walletPage.actions.batchRevoke")}
            </Button>
            {batchActionDisabledReason ? (
              <p className="w-full basis-full text-xs text-muted-foreground">{batchActionDisabledReason}</p>
            ) : null}
          </div>
        </div>

        <ListPagination
          page={passPage}
          onPageChange={onPassPageChange}
          pageSize={passPageSize}
          onPageSizeChange={onPassPageSizeChange}
          hasNextPage={passTable.getCanNextPage()}
          disabled={loading || filteredPassCount === 0}
        />

        <div className="flex justify-end">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="outline" size="sm">
                <SlidersHorizontalIcon className="mr-1.5 size-4" />
                {t("walletPage.actions.columnDisplay")}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {passToggleableColumns.map((column) => (
                <DropdownMenuCheckboxItem
                  key={column.id}
                  checked={column.getIsVisible()}
                  onCheckedChange={(checked) => column.toggleVisibility(Boolean(checked))}
                >
                  {columnDisplayLabel(column, passColumnLabels)}
                </DropdownMenuCheckboxItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <Table>
          <TableHeader>
            {passTable.getHeaderGroups().map((headerGroup) => (
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
                <TableCell colSpan={passVisibleColumnCount} className="py-10 text-center text-muted-foreground">
                  {t("walletPage.cards.issuedPasses.loading")}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading && filteredPassCount === 0 ? (
              <TableRow>
                <TableCell colSpan={passVisibleColumnCount} className="py-8 text-center text-muted-foreground">
                  {passesCount === 0
                    ? t("walletPage.cards.issuedPasses.emptyNoPasses")
                    : t("walletPage.cards.issuedPasses.emptyFiltered")}
                </TableCell>
              </TableRow>
            ) : null}
            {!loading &&
              passTable.getRowModel().rows.length > 0 &&
              passTable.getRowModel().rows.map((row) => (
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
