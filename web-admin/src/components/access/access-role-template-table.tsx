import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type AccessRoleTemplateRow = {
  accessRole: string
  defaultGroup: string
  permissionPreset: string
  position: string
}

type AccessRoleTemplateTableProps = {
  items: AccessRoleTemplateRow[]
}

export function AccessRoleTemplateTable({ items }: AccessRoleTemplateTableProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">岗位自动分组与权限模板</CardTitle>
        <CardDescription>
          企业员工同步时会按岗位 / 部门自动映射用户组与权限预设，普通办公区由默认组覆盖，避免逐人分配。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>岗位关键字</TableHead>
              <TableHead>默认用户组</TableHead>
              <TableHead>访问角色</TableHead>
              <TableHead>权限预设</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.position}>
                <TableCell className="font-medium">{item.position}</TableCell>
                <TableCell>{item.defaultGroup}</TableCell>
                <TableCell>{item.accessRole}</TableCell>
                <TableCell>{item.permissionPreset}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
