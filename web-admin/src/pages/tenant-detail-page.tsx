import { useMemo } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeftIcon, Building2Icon, DoorOpenIcon, Layers3Icon, MapPinnedIcon } from "lucide-react"
import { useQuery } from "@tanstack/react-query"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getTenantTopology, listTenants, type Building, type Tenant, type TenantTopology } from "@/lib/api"

type TenantDetailPageProps = {
  token: string
}

function tenantTypeLabel(type: Tenant["type"]) {
  switch (type) {
    case "studio":
      return "个人工作室"
    case "company":
      return "公司"
    case "government":
      return "政府"
    case "factory":
      return "工厂"
    case "public_facility":
      return "公共设施"
    default:
      return type
  }
}

function sortByRegion(items: Building[]) {
  return [...items].sort((a, b) => (a.region ?? "").localeCompare(b.region ?? "", "zh-CN"))
}

export function TenantDetailPage({ token }: TenantDetailPageProps) {
  const { tenantID = "" } = useParams()
  const tenantsQuery = useQuery({
    queryKey: ["tenants", token],
    queryFn: () => listTenants(token),
    staleTime: 60 * 1000,
  })
  const topologyQuery = useQuery({
    queryKey: ["tenant-topology", token, tenantID],
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

  const buildings = useMemo(() => sortByRegion(topology?.buildings ?? []), [topology?.buildings])
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

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link to="/tenants">
            <ArrowLeftIcon className="mr-1.5 size-4" />
            返回租户列表
          </Link>
        </Button>
        <div>
          <p className="mp-page-eyebrow">租户控制区域</p>
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
            <CardDescription>租户类型</CardDescription>
            <CardTitle className="text-xl">
              {tenant ? tenantTypeLabel(tenant.type) : "--"}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            总部区域：{tenant?.hq_region || "-"}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>楼宇</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (topology?.buildings?.length ?? 0)}
              <Building2Icon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>区域</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (topology?.areas?.length ?? 0)}
              <Layers3Icon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>门点</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (topology?.doors?.length ?? 0)}
              <DoorOpenIcon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">实际控制区域</CardTitle>
          <CardDescription>按地区查看租户在不同城市/园区的楼宇控制范围。</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>楼宇</TableHead>
                <TableHead>地区</TableHead>
                <TableHead>地址</TableHead>
                <TableHead>楼层数</TableHead>
                <TableHead>区域数</TableHead>
                <TableHead>门点数</TableHead>
                <TableHead>运行状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="py-10 text-center text-muted-foreground">
                    正在加载租户控制区域...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                buildings.map((item) => {
                  const doors = doorByBuilding.get(item.id) ?? 0
                  const online = (topology?.doors ?? []).filter(
                    (door) => door.building_id === item.id && door.status === "online"
                  ).length

                  return (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">{item.name}</TableCell>
                      <TableCell>
                        <div className="inline-flex items-center gap-1.5">
                          <MapPinnedIcon className="size-3.5 text-muted-foreground" />
                          {item.region || "-"}
                        </div>
                      </TableCell>
                      <TableCell>{item.address || "-"}</TableCell>
                      <TableCell>{floorByBuilding.get(item.id) ?? 0}</TableCell>
                      <TableCell>{areaByBuilding.get(item.id) ?? 0}</TableCell>
                      <TableCell>{doors}</TableCell>
                      <TableCell>
                        <Badge variant={online === doors && doors > 0 ? "outline" : "secondary"}>
                          在线 {online}/{doors}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  )
                })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
