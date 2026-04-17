import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { TabsContent } from "@/components/ui/tabs"
import { type EnterpriseEmployee } from "@/lib/api"

type EnterpriseEmployeesWorkspaceProps = {
  directoryLink: string
  employees: EnterpriseEmployee[]
  formatDateTime: (value?: string) => string
  loading: boolean
  statusBadgeVariant: (status?: string) => "outline" | "secondary" | "destructive"
}

export function EnterpriseEmployeesWorkspace({
  directoryLink,
  employees,
  formatDateTime,
  loading,
  statusBadgeVariant,
}: EnterpriseEmployeesWorkspaceProps) {
  return (
    <TabsContent value="employees">
      <Card>
        <CardHeader className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div>
            <CardTitle className="text-base">员工目录</CardTitle>
            <CardDescription>同步完成后，去权限页建立用户组和访问策略。</CardDescription>
          </div>
          <Button asChild variant="outline" size="sm">
            <Link to={directoryLink}>去员工与用户组</Link>
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>姓名</TableHead>
                <TableHead>邮箱</TableHead>
                <TableHead>部门 / 岗位</TableHead>
                <TableHead>来源</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>最近同步</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {!loading && employees.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                    还没有企业员工目录，请先在“导入与同步”里接入 HRIS、SCIM 或 CSV。
                  </TableCell>
                </TableRow>
              ) : null}
              {employees.slice(0, 12).map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.full_name}</TableCell>
                  <TableCell>{item.email}</TableCell>
                  <TableCell>
                    {item.department || "-"} / {item.job_title || "-"}
                  </TableCell>
                  <TableCell>{item.source || "-"}</TableCell>
                  <TableCell>
                    <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                  </TableCell>
                  <TableCell>{formatDateTime(item.last_synced_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </TabsContent>
  )
}
