import { useMemo, useState } from "react"
import { FileSearchIcon, ShieldIcon } from "lucide-react"
import { useQuery } from "@tanstack/react-query"

import { Badge } from "@/components/ui/badge"
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

function labelForAction(action: string) {
  switch (action) {
    case "login":
      return "登录"
    case "tenant_update":
      return "租户更新"
    case "policy_publish":
      return "策略发布"
    case "gateway_reboot":
      return "网关重启"
    case "visitor_issue":
      return "访客签发"
    default:
      return action.replaceAll("_", " ")
  }
}

function roleLabel(role: string) {
  switch (role) {
    case "super_admin":
      return "平台管理员"
    case "tenant_admin":
      return "租户管理员"
    case "operator":
      return "运维人员"
    default:
      return role.replaceAll("_", " ")
  }
}

export function AuditPage({ token }: AuditPageProps) {
  const [query, setQuery] = useState("")
  const [actionFilter, setActionFilter] = useState<"all" | AuditAction>("all")
  const auditLogsQuery = useQuery({
    queryKey: ["audit-logs", token],
    queryFn: () => listAuditLogs(token),
  })
  const rows: AuditLog[] = auditLogsQuery.data ?? []
  const loading = auditLogsQuery.isPending
  const error =
    auditLogsQuery.error instanceof Error ? auditLogsQuery.error.message : ""

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

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">审计与合规</p>
        <h1 className="mp-page-title">管理端审计日志</h1>
        <p className="mp-page-description">
          记录租户、策略与网关相关敏感操作的不可篡改轨迹。
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>审计记录</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : rows.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">包含系统与管理员操作。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>敏感操作</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {rows.filter((item) => item.action !== "login").length}{" "}
              <ShieldIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            包含重启、策略发布与角色变更。
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>待人工复核</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {rows.filter((item) => item.action === "gateway_reboot").length}{" "}
              <FileSearchIcon className="size-4 text-sky-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            被运维风险规则标记。
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">审计检索</CardTitle>
          <CardDescription>按动作类型和执行人/目标关键字过滤。</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 md:grid-cols-[1fr_220px]">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="按执行人、目标 ID 或审计 ID 搜索"
            />
            <Select
              value={actionFilter}
              onValueChange={(value: "all" | AuditAction) => {
                setActionFilter(value)
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder="动作类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部动作</SelectItem>
                <SelectItem value="login">登录（login）</SelectItem>
                <SelectItem value="tenant_update">租户更新（tenant_update）</SelectItem>
                <SelectItem value="policy_publish">策略发布（policy_publish）</SelectItem>
                <SelectItem value="gateway_reboot">网关重启（gateway_reboot）</SelectItem>
                <SelectItem value="visitor_issue">访客签发（visitor_issue）</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">审计记录</CardTitle>
          <CardDescription>匹配到 {filtered.length} 条记录。</CardDescription>
        </CardHeader>
        <CardContent>
          {error ? (
            <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>审计 ID</TableHead>
                <TableHead>执行人</TableHead>
                <TableHead>角色</TableHead>
                <TableHead>动作</TableHead>
                <TableHead>目标</TableHead>
                <TableHead>来源</TableHead>
                <TableHead>时间戳</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="py-10 text-center text-muted-foreground">
                    正在加载审计日志...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                filtered.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="font-medium">{row.id}</TableCell>
                    <TableCell>{row.actor}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {roleLabel(row.role)}
                      </Badge>
                    </TableCell>
                    <TableCell>{labelForAction(row.action)}</TableCell>
                    <TableCell>{row.target}</TableCell>
                    <TableCell>{row.source}</TableCell>
                    <TableCell>{new Date(row.at).toLocaleString("zh-CN")}</TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
