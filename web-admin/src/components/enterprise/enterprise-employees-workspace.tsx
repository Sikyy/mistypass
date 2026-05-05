import { Link } from "react-router"
import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()
  return (
    <TabsContent value="employees">
      <Card>
        <CardHeader className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div>
            <CardTitle className="text-base">{t("enterpriseEmployees.title")}</CardTitle>
            <CardDescription>{t("enterpriseEmployees.description")}</CardDescription>
          </div>
          <Button asChild variant="outline" size="sm">
            <Link to={directoryLink}>{t("enterpriseEmployees.goDirectory")}</Link>
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("enterpriseEmployees.table.name")}</TableHead>
                <TableHead>{t("enterpriseEmployees.table.email")}</TableHead>
                <TableHead>{t("enterpriseEmployees.table.departmentJob")}</TableHead>
                <TableHead>{t("enterpriseEmployees.table.source")}</TableHead>
                <TableHead>{t("enterpriseEmployees.table.status")}</TableHead>
                <TableHead>{t("enterpriseEmployees.table.lastSynced")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {!loading && employees.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                    {t("enterpriseEmployees.empty")}
                  </TableCell>
                </TableRow>
              ) : null}
              {employees.slice(0, 12).map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.full_name}</TableCell>
                  <TableCell>{item.email}</TableCell>
                  <TableCell>
                    {item.department || t("enterpriseEmployees.table.emptyDash")} /{" "}
                    {item.job_title || t("enterpriseEmployees.table.emptyDash")}
                  </TableCell>
                  <TableCell>{item.source || t("enterpriseEmployees.table.emptyDash")}</TableCell>
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
