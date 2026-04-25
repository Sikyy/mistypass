import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {t("accessPage.components.roleTemplateTable.title", { defaultValue: "Role-based group and permission templates" })}
        </CardTitle>
        <CardDescription>
          {t("accessPage.components.roleTemplateTable.description", {
            defaultValue:
              "During employee sync, user groups and permission presets are mapped by role/department automatically; default office coverage avoids per-person assignment.",
          })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("accessPage.components.roleTemplateTable.columns.positionKeywords", { defaultValue: "Role keywords" })}</TableHead>
              <TableHead>{t("accessPage.components.roleTemplateTable.columns.defaultGroup", { defaultValue: "Default group" })}</TableHead>
              <TableHead>{t("accessPage.components.roleTemplateTable.columns.accessRole", { defaultValue: "Access role" })}</TableHead>
              <TableHead>{t("accessPage.components.roleTemplateTable.columns.permissionPreset", { defaultValue: "Permission preset" })}</TableHead>
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
