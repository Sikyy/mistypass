import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ArrowRightIcon, ArrowUpDownIcon, Building2Icon, PlusCircleIcon, SearchIcon, SlidersHorizontalIcon } from "lucide-react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
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
import { createTenant, listTenants, type Tenant, updateTenantStatus } from "@/lib/api"

type TenantsPageProps = {
  token: string
}

type TenantType = "studio" | "company" | "government" | "factory" | "public_facility"

const tenantTypeValues = ["studio", "company", "government", "factory", "public_facility"] as const
type CreateTenantFormValues = {
  name: string
  type: TenantType
  hq_region?: string
}

function statusVariant(status: Tenant["status"]) {
  switch (status) {
    case "active":
      return "outline"
    case "suspended":
      return "secondary"
    case "inactive":
      return "destructive"
    default:
      return "outline"
  }
}

function statusLabel(status: Tenant["status"], t: (key: string) => string) {
  switch (status) {
    case "active":
      return t("tenants.status.active")
    case "suspended":
      return t("tenants.status.suspended")
    case "inactive":
      return t("tenants.status.inactive")
    default:
      return status
  }
}

function tenantTypeLabel(type: Tenant["type"], t: (key: string) => string) {
  switch (type) {
    case "studio":
      return t("tenants.type.studio")
    case "company":
      return t("tenants.type.company")
    case "government":
      return t("tenants.type.government")
    case "factory":
      return t("tenants.type.factory")
    case "public_facility":
      return t("tenants.type.publicFacility")
    default:
      return type
  }
}

export function TenantsPage({ token }: TenantsPageProps) {
  const { t, i18n } = useTranslation()
  const createTenantSchema = useMemo(
    () =>
      z.object({
        name: z
          .string()
          .trim()
          .min(2, t("tenants.form.validation.nameMin"))
          .max(64, t("tenants.form.validation.nameMax")),
        type: z.enum(tenantTypeValues),
        hq_region: z
          .string()
          .trim()
          .max(64, t("tenants.form.validation.hqRegionMax"))
          .optional()
          .or(z.literal("")),
      }),
    [t]
  )
  const queryClient = useQueryClient()
  const [query, setQuery] = useState("")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)
  const [error, setError] = useState("")
  const [rowUpdating, setRowUpdating] = useState<Record<string, boolean>>({})
  const createTenantForm = useForm<CreateTenantFormValues>({
    resolver: zodResolver(createTenantSchema),
    defaultValues: {
      name: "",
      type: "company",
      hq_region: "",
    },
  })

  const tenantsQuery = useQuery({
    queryKey: ["tenants"],
    queryFn: () => listTenants(token),
    staleTime: 60 * 1000,
  })

  const tenants = tenantsQuery.data ?? []
  const loading = tenantsQuery.isPending
  const queryError =
    tenantsQuery.isError && tenantsQuery.error instanceof Error ? tenantsQuery.error.message : ""

  const createTenantMutation = useMutation({
    mutationFn: (payload: { name: string; type: TenantType; hq_region?: string }) =>
      createTenant(token, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<Tenant[]>(["tenants"], (current) => [created, ...(current ?? [])])
      createTenantForm.reset({
        name: "",
        type: "company",
        hq_region: "",
      })
    },
  })

  const updateTenantStatusMutation = useMutation({
    mutationFn: (payload: { tenantID: string; status: "active" | "suspended" | "inactive" }) =>
      updateTenantStatus(token, payload.tenantID, payload.status),
    onSuccess: (updated) => {
      queryClient.setQueryData<Tenant[]>(
        ["tenants"],
        (current) => current?.map((item) => (item.id === updated.id ? updated : item)) ?? []
      )
    },
  })

  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const columns = useMemo<ColumnDef<Tenant>[]>(
    () => [
      {
        id: "id",
        accessorKey: "id",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenants.table.id")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[11rem] font-medium">{row.original.id}</TableCellText>,
      },
      {
        id: "name",
        accessorKey: "name",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenants.table.name")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[14rem]">{row.original.name}</TableCellText>,
      },
      {
        id: "type",
        accessorKey: "type",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenants.table.type")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => tenantTypeLabel(row.original.type, t),
      },
      {
        id: "hq_region",
        accessorFn: (row) => row.hq_region || "-",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenants.table.hqRegion")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => row.original.hq_region || t("tenants.table.emptyDash"),
      },
      {
        id: "status",
        accessorKey: "status",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenants.table.status")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <Badge variant={statusVariant(row.original.status)} className="capitalize">
            {statusLabel(row.original.status, t)}
          </Badge>
        ),
      },
      {
        id: "created_at",
        accessorKey: "created_at",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenants.table.createdAt")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => new Date(row.original.created_at).toLocaleString(i18n.language),
      },
      {
        id: "controls",
        header: () => t("tenants.table.controls"),
        enableSorting: false,
        cell: ({ row }) => (
          <Button asChild variant="outline" size="sm">
            <Link to={`/tenants/${row.original.id}`}>
              {t("tenants.table.view")}
              <ArrowRightIcon className="ml-1.5 size-4" />
            </Link>
          </Button>
        ),
      },
      {
        id: "status_action",
        header: () => <span className="inline-block w-[200px]">{t("tenants.table.setStatus")}</span>,
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <Select
            disabled={rowUpdating[row.original.id]}
            value={row.original.status}
            onValueChange={(value: "active" | "suspended" | "inactive") => {
              void onChangeTenantStatus(row.original.id, value)
            }}
          >
            <SelectTrigger>
              <SelectValue placeholder={t("tenants.table.statusPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">{t("tenants.table.statusOption.active")}</SelectItem>
              <SelectItem value="suspended">{t("tenants.table.statusOption.suspended")}</SelectItem>
              <SelectItem value="inactive">{t("tenants.table.statusOption.inactive")}</SelectItem>
            </SelectContent>
          </Select>
        ),
      },
    ],
    [onChangeTenantStatus, rowUpdating, t, i18n.language]
  )
  const table = useReactTable({
    columns,
    data: tenants,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility,
      globalFilter: query,
      pagination: {
        pageIndex: Math.max(0, page - 1),
        pageSize,
      },
      sorting,
    },
    onColumnVisibilityChange: setColumnVisibility,
    onSortingChange: setSorting,
    globalFilterFn: (row, _columnID, filterValue) => {
      const q = String(filterValue ?? "")
        .trim()
        .toLowerCase()
      if (!q) {
        return true
      }
      return [
        row.original.id,
        row.original.name,
        row.original.type,
        tenantTypeLabel(row.original.type, t),
        row.original.hq_region || t("tenants.table.emptyDash"),
        row.original.status,
        statusLabel(row.original.status, t),
      ].some((value) => value.toLowerCase().includes(q))
    },
  })
  const filteredTenantsCount = table.getFilteredRowModel().rows.length
  const visibleColumnCount = table.getVisibleLeafColumns().length
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const columnLabels: Record<string, string> = {
    controls: t("tenants.table.controls"),
    created_at: t("tenants.table.createdAt"),
    hq_region: t("tenants.table.hqRegion"),
    id: t("tenants.table.id"),
    name: t("tenants.table.name"),
    status: t("tenants.table.status"),
    type: t("tenants.table.type"),
  }
  const maxPage = Math.max(1, Math.ceil(filteredTenantsCount / pageSize))

  useEffect(() => {
    if (page > maxPage) {
      setPage(maxPage)
    }
  }, [maxPage, page])

  async function onCreateTenant(values: CreateTenantFormValues) {
    setError("")
    try {
      await createTenantMutation.mutateAsync({
        name: values.name.trim(),
        type: values.type,
        hq_region: values.hq_region?.trim() || undefined,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("tenants.error.createFailed")
      setError(message)
    }
  }

  async function onChangeTenantStatus(tenantID: string, status: "active" | "suspended" | "inactive") {
    setRowUpdating((current) => ({ ...current, [tenantID]: true }))
    setError("")
    try {
      await updateTenantStatusMutation.mutateAsync({ tenantID, status })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("tenants.error.updateStatusFailed")
      setError(message)
    } finally {
      setRowUpdating((current) => ({ ...current, [tenantID]: false }))
    }
  }

  const createTenantFormError =
    createTenantForm.formState.errors.name?.message ||
    createTenantForm.formState.errors.type?.message ||
    createTenantForm.formState.errors.hq_region?.message ||
    ""

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">{t("tenants.header.eyebrow")}</p>
        <h1 className="mp-page-title">{t("tenants.header.title")}</h1>
        <p className="mp-page-description">
          {t("tenants.header.description")}
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenants.kpi.total.title")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {tenants.length} <Building2Icon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("tenants.kpi.total.note")}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenants.kpi.companyFactory.title")}</CardDescription>
            <CardTitle className="text-2xl">
              {tenants.filter((item) => item.type === "company" || item.type === "factory").length}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("tenants.kpi.companyFactory.note")}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenants.kpi.active.title")}</CardDescription>
            <CardTitle className="text-2xl">
              {tenants.filter((item) => item.status === "active").length}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">{t("tenants.kpi.active.note")}</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("tenants.form.title")}</CardTitle>
          <CardDescription>{t("tenants.form.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-3 md:grid-cols-[1fr_180px_160px_auto]" onSubmit={createTenantForm.handleSubmit(onCreateTenant)}>
            <Input
              {...createTenantForm.register("name")}
              placeholder={t("tenants.form.namePlaceholder")}
            />
            <Controller
              control={createTenantForm.control}
              name="type"
              render={({ field }) => (
                <Select value={field.value} onValueChange={(value: TenantType) => field.onChange(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("tenants.form.typePlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="studio">{t("tenants.type.studio")}</SelectItem>
                    <SelectItem value="company">{t("tenants.type.company")}</SelectItem>
                    <SelectItem value="government">{t("tenants.type.government")}</SelectItem>
                    <SelectItem value="factory">{t("tenants.type.factory")}</SelectItem>
                    <SelectItem value="public_facility">{t("tenants.type.publicFacility")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
            <Input
              {...createTenantForm.register("hq_region")}
              placeholder={t("tenants.form.hqRegionPlaceholder")}
            />
            <Button type="submit" disabled={createTenantMutation.isPending || createTenantForm.formState.isSubmitting}>
              <PlusCircleIcon className="mr-1.5 size-4" />
              {createTenantMutation.isPending ? t("tenants.form.submitting") : t("tenants.form.submit")}
            </Button>
            {createTenantFormError ? (
              <p className="text-sm text-destructive md:col-span-4">{createTenantFormError}</p>
            ) : null}
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("tenants.list.title")}</CardTitle>
          <CardDescription>{t("tenants.list.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="relative max-w-sm">
            <div className="flex flex-col gap-2 md:flex-row md:items-center">
              <div className="relative w-full md:max-w-sm">
                <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => {
                    setQuery(event.target.value)
                    setPage(1)
                  }}
                  className="pl-8"
                  aria-label={t("tenants.list.searchPlaceholder")}
                  placeholder={t("tenants.list.searchPlaceholder")}
                />
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" size="sm" className="w-full md:w-auto">
                    <SlidersHorizontalIcon className="mr-1.5 size-4" />
                    {t("tenants.list.columnDisplay")}
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

          {error || queryError ? (
            <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error || queryError}
            </div>
          ) : null}

          <ListPagination
            page={page}
            onPageChange={setPage}
            pageSize={pageSize}
            onPageSizeChange={(value) => {
              setPageSize(value)
              setPage(1)
            }}
            hasNextPage={table.getCanNextPage()}
            disabled={loading || filteredTenantsCount === 0}
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
                    {t("tenants.list.loading")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredTenantsCount === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnCount} className="py-8 text-center text-muted-foreground">
                    {query.trim() ? t("tenants.list.empty.filtered") : t("tenants.list.empty.default")}
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
