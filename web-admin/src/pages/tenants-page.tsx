import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FormEvent, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { ArrowRightIcon, Building2Icon, PlusCircleIcon, SearchIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
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
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { createTenant, listTenants, type Tenant, updateTenantStatus } from "@/lib/api"

type TenantsPageProps = {
  token: string
}

type TenantType = "studio" | "company" | "government" | "factory" | "public_facility"

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

function statusLabel(status: Tenant["status"]) {
  switch (status) {
    case "active":
      return "启用"
    case "suspended":
      return "暂停"
    case "inactive":
      return "停用"
    default:
      return status
  }
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

export function TenantsPage({ token }: TenantsPageProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState("")
  const [tenantType, setTenantType] = useState<TenantType>("company")
  const [hqRegion, setHQRegion] = useState("")
  const [query, setQuery] = useState("")
  const [error, setError] = useState("")
  const [rowUpdating, setRowUpdating] = useState<Record<string, boolean>>({})

  const tenantsQuery = useQuery({
    queryKey: ["tenants", token],
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
      queryClient.setQueryData<Tenant[]>(["tenants", token], (current) => [created, ...(current ?? [])])
      setName("")
      setTenantType("company")
      setHQRegion("")
    },
  })

  const updateTenantStatusMutation = useMutation({
    mutationFn: (payload: { tenantID: string; status: "active" | "suspended" | "inactive" }) =>
      updateTenantStatus(token, payload.tenantID, payload.status),
    onSuccess: (updated) => {
      queryClient.setQueryData<Tenant[]>(
        ["tenants", token],
        (current) => current?.map((item) => (item.id === updated.id ? updated : item)) ?? []
      )
    },
  })

  const filteredTenants = useMemo(() => {
    const q = query.trim().toLowerCase()
    return tenants.filter((item) => {
      if (!q) {
        return true
      }
      return item.name.toLowerCase().includes(q) || item.id.toLowerCase().includes(q)
    })
  }, [query, tenants])

  async function onCreateTenant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!name.trim()) {
      return
    }

    setError("")
    try {
      await createTenantMutation.mutateAsync({
        name,
        type: tenantType,
        hq_region: hqRegion.trim() || undefined,
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建租户失败"
      setError(message)
    }
  }

  async function onChangeTenantStatus(tenantID: string, status: "active" | "suspended" | "inactive") {
    setRowUpdating((current) => ({ ...current, [tenantID]: true }))
    setError("")
    try {
      await updateTenantStatusMutation.mutateAsync({ tenantID, status })
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新租户状态失败"
      setError(message)
    } finally {
      setRowUpdating((current) => ({ ...current, [tenantID]: false }))
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">租户管理</p>
        <h1 className="mp-page-title">租户</h1>
        <p className="mp-page-description">
          在分配楼宇、用户和网关之前，先完成租户开通与数据隔离。
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>租户总数</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {tenants.length} <Building2Icon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            当前多组织隔离与开通基线。
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>企业/工厂</CardDescription>
            <CardTitle className="text-2xl">
              {tenants.filter((item) => item.type === "company" || item.type === "factory").length}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">重点关注租户。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>运行中</CardDescription>
            <CardTitle className="text-2xl">
              {tenants.filter((item) => item.status === "active").length}
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">状态正常。</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">创建租户</CardTitle>
          <CardDescription>创建新的租户工作空间并隔离数据范围。</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-3 md:grid-cols-[1fr_180px_160px_auto]" onSubmit={onCreateTenant}>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="租户名称（例如：总部园区 / 华东工厂 / 联合办公空间）"
            />
            <Select value={tenantType} onValueChange={(value: TenantType) => setTenantType(value)}>
              <SelectTrigger>
                <SelectValue placeholder="租户类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="studio">个人工作室</SelectItem>
                <SelectItem value="company">公司</SelectItem>
                <SelectItem value="government">政府</SelectItem>
                <SelectItem value="factory">工厂</SelectItem>
                <SelectItem value="public_facility">公共设施</SelectItem>
              </SelectContent>
            </Select>
            <Input
              value={hqRegion}
              onChange={(event) => setHQRegion(event.target.value)}
              placeholder="总部区域（如 ID-JK）"
            />
            <Button type="submit" disabled={createTenantMutation.isPending}>
              <PlusCircleIcon className="mr-1.5 size-4" />
              {createTenantMutation.isPending ? "创建中..." : "创建租户"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">租户列表</CardTitle>
          <CardDescription>检索并更新租户生命周期状态。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="relative max-w-sm">
            <SearchIcon className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              className="pl-8"
              placeholder="按租户编号或名称搜索"
            />
          </div>

          {error || queryError ? (
            <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error || queryError}
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>租户编号</TableHead>
                <TableHead>名称</TableHead>
                <TableHead>类型</TableHead>
                <TableHead>总部区域</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>创建时间</TableHead>
                <TableHead>控制区域</TableHead>
                <TableHead className="w-[220px]">设置状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={8} className="py-10 text-center text-muted-foreground">
                    正在加载租户...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                filteredTenants.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.id}</TableCell>
                    <TableCell>{item.name}</TableCell>
                    <TableCell>{tenantTypeLabel(item.type)}</TableCell>
                    <TableCell>{item.hq_region || "-"}</TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(item.status)} className="capitalize">
                        {statusLabel(item.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>{new Date(item.created_at).toLocaleString("zh-CN")}</TableCell>
                    <TableCell>
                      <Button asChild variant="outline" size="sm">
                        <Link to={`/tenants/${item.id}`}>
                          查看
                          <ArrowRightIcon className="ml-1.5 size-4" />
                        </Link>
                      </Button>
                    </TableCell>
                    <TableCell>
                      <Select
                        disabled={rowUpdating[item.id]}
                        value={item.status}
                        onValueChange={(value: "active" | "suspended" | "inactive") => {
                          void onChangeTenantStatus(item.id, value)
                        }}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="active">启用（active）</SelectItem>
                          <SelectItem value="suspended">暂停（suspended）</SelectItem>
                          <SelectItem value="inactive">停用（inactive）</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
