import { type ColumnDef, type SortingState, type VisibilityState, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useMemo, useState } from "react"
import { Link, useParams } from "react-router"
import { ArrowLeftIcon, ArrowUpDownIcon, Building2Icon, DoorOpenIcon, Layers3Icon, MapPinnedIcon, SearchIcon, SlidersHorizontalIcon } from "lucide-react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
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
import { getTenantTopology, listTenants, type Building, type Tenant, type TenantTopology } from "@/lib/api"

type TenantDetailPageProps = {
  token: string
}

function tenantTypeLabel(type: Tenant["type"], t: TFunction) {
  switch (type) {
    case "studio":
      return t("tenantDetail.type.studio")
    case "company":
      return t("tenantDetail.type.company")
    case "government":
      return t("tenantDetail.type.government")
    case "factory":
      return t("tenantDetail.type.factory")
    case "public_facility":
      return t("tenantDetail.type.publicFacility")
    default:
      return type
  }
}

function sortByRegion(items: Building[], locale: string) {
  return [...items].sort((a, b) => (a.region ?? "").localeCompare(b.region ?? "", locale))
}

export function TenantDetailPage({ token }: TenantDetailPageProps) {
  const { t, i18n } = useTranslation()
  const { tenantID = "" } = useParams()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState("")
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: 25,
  })
  const tenantsQuery = useQuery({
    queryKey: ["tenants"],
    queryFn: () => listTenants(token),
    staleTime: 60 * 1000,
  })
  const topologyQuery = useQuery({
    queryKey: ["tenant-topology", tenantID],
    queryFn: () => getTenantTopology(token, tenantID),
    enabled: tenantID.trim().length > 0,
    staleTime: 60 * 1000,
  })

  const tenant = useMemo(
    () => tenantsQuery.data?.find((item) => item.id === tenantID) ?? null,
    [tenantID, tenantsQuery.data]
  )
  const topology: TenantTopology | null = topologyQuery.data ?? null
  const loading = tenantsQuery.isPending || (tenantID.trim().length > 0 && topologyQuery.isPending)
  const error =
    (tenantsQuery.isError && tenantsQuery.error instanceof Error && tenantsQuery.error.message) ||
    (topologyQuery.isError && topologyQuery.error instanceof Error && topologyQuery.error.message) ||
    ""

  const buildings = useMemo(
    () => sortByRegion(topology?.buildings ?? [], i18n.language || "zh-CN"),
    [i18n.language, topology?.buildings]
  )
  const floorByBuilding = useMemo(() => {
    const map = new Map<string, number>()
    for (const item of topology?.floors ?? []) {
      map.set(item.building_id, (map.get(item.building_id) ?? 0) + 1)
    }
    return map
  }, [topology?.floors])
  const areaByBuilding = useMemo(() => {
    const map = new Map<string, number>()
    for (const item of topology?.areas ?? []) {
      map.set(item.building_id, (map.get(item.building_id) ?? 0) + 1)
    }
    return map
  }, [topology?.areas])
  const doorByBuilding = useMemo(() => {
    const map = new Map<string, number>()
    for (const item of topology?.doors ?? []) {
      map.set(item.building_id, (map.get(item.building_id) ?? 0) + 1)
    }
    return map
  }, [topology?.doors])
  const columns = useMemo<ColumnDef<Building>[]>(
    () => [
      {
        id: "name",
        accessorKey: "name",
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.name")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => <TableCellText className="max-w-[14rem] font-medium">{row.original.name}</TableCellText>,
      },
      {
        id: "region",
        accessorFn: (row) => row.region || t("tenantDetail.table.emptyDash"),
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.region")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <div className="inline-flex items-center gap-1.5">
            <MapPinnedIcon className="size-3.5 text-muted-foreground" />
            {row.original.region || t("tenantDetail.table.emptyDash")}
          </div>
        ),
      },
      {
        id: "address",
        accessorFn: (row) => row.address || t("tenantDetail.table.emptyDash"),
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.address")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => (
          <TableCellText className="max-w-[16rem]">
            {row.original.address || t("tenantDetail.table.emptyDash")}
          </TableCellText>
        ),
      },
      {
        id: "floors",
        accessorFn: (row) => floorByBuilding.get(row.id) ?? 0,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.floors")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => floorByBuilding.get(row.original.id) ?? 0,
      },
      {
        id: "areas",
        accessorFn: (row) => areaByBuilding.get(row.id) ?? 0,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.areas")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => areaByBuilding.get(row.original.id) ?? 0,
      },
      {
        id: "doors",
        accessorFn: (row) => doorByBuilding.get(row.id) ?? 0,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.doors")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => doorByBuilding.get(row.original.id) ?? 0,
      },
      {
        id: "status",
        accessorFn: (row) => (topology?.doors ?? []).filter((door) => door.building_id === row.id && door.status === "online").length,
        header: ({ column }) => (
          <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
            {t("tenantDetail.table.status")}
            <ArrowUpDownIcon className="ml-1.5 size-3.5" />
          </Button>
        ),
        cell: ({ row }) => {
          const doors = doorByBuilding.get(row.original.id) ?? 0
          const online = (topology?.doors ?? []).filter(
            (door) => door.building_id === row.original.id && door.status === "online"
          ).length
          return (
            <Badge variant={online === doors && doors > 0 ? "outline" : "secondary"}>
              {t("tenantDetail.table.statusOnline", { online, total: doors })}
            </Badge>
          )
        },
      },
    ],
    [areaByBuilding, doorByBuilding, floorByBuilding, t, topology?.doors]
  )
  const table = useReactTable({
    columns,
    data: buildings,
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
      return [row.original.name, row.original.region || "-", row.original.address || "-"].some((value) =>
        value.toLowerCase().includes(query)
      )
    },
  })
  const columnLabels: Record<string, string> = {
    address: t("tenantDetail.table.address"),
    areas: t("tenantDetail.table.areas"),
    doors: t("tenantDetail.table.doors"),
    floors: t("tenantDetail.table.floors"),
    name: t("tenantDetail.table.name"),
    region: t("tenantDetail.table.region"),
    status: t("tenantDetail.table.status"),
  }
  const filteredBuildingCount = table.getFilteredRowModel().rows.length
  const toggleableColumns = table.getAllLeafColumns().filter((column) => column.getCanHide())
  const visibleColumnCount = table.getVisibleLeafColumns().length

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link to="/tenants">
            <ArrowLeftIcon className="mr-1.5 size-4" />
            {t("tenantDetail.backToTenants")}
          </Link>
        </Button>
        <div>
          <p className="mp-page-eyebrow">{t("tenantDetail.eyebrow")}</p>
          <h1 className="mp-page-title">{tenant?.name ?? tenantID}</h1>
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenantDetail.kpi.tenantType.title")}</CardDescription>
            <CardTitle className="text-xl">
              {tenant ? tenantTypeLabel(tenant.type, t) : "--"}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            {t("tenantDetail.kpi.tenantType.hqRegion", {
              region: tenant?.hq_region || t("tenantDetail.table.emptyDash"),
            })}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenantDetail.kpi.buildings")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (topology?.buildings?.length ?? 0)}
              <Building2Icon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenantDetail.kpi.areas")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (topology?.areas?.length ?? 0)}
              <Layers3Icon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("tenantDetail.kpi.doors")}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (topology?.doors?.length ?? 0)}
              <DoorOpenIcon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("tenantDetail.coverage.title")}</CardTitle>
          <CardDescription>{t("tenantDetail.coverage.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-col gap-2 rounded-lg border bg-muted/10 px-3 py-2">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div className="relative w-full md:max-w-sm">
                <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                <Input
                  className="pl-8"
                  value={globalFilter}
                  onChange={(event) => {
                    setGlobalFilter(event.target.value)
                    setPagination((current) => ({
                      ...current,
                      pageIndex: 0,
                    }))
                  }}
                  aria-label={t("tenantDetail.coverage.searchPlaceholder")}
                  placeholder={t("tenantDetail.coverage.searchPlaceholder")}
                />
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" size="sm" className="w-full md:w-auto">
                    <SlidersHorizontalIcon className="mr-1.5 size-4" />
                    {t("tenantDetail.coverage.columnDisplay")}
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
            disabled={loading || filteredBuildingCount === 0}
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
                    {t("tenantDetail.coverage.loading")}
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredBuildingCount === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnCount} className="py-8 text-center text-muted-foreground">
                    {globalFilter.trim()
                      ? t("tenantDetail.coverage.empty.filtered")
                      : t("tenantDetail.coverage.empty.default")}
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
