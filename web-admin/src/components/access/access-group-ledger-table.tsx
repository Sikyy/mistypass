import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type UserGroup } from "@/lib/api"

type AccessGroupLedgerRow = {
  descriptionLabel: string
  group: UserGroup
  membersLabel: string
}

type AccessGroupLedgerTableProps = {
  emptyState: string
  onEdit: (group: UserGroup) => void
  rows: AccessGroupLedgerRow[]
}

export function AccessGroupLedgerTable({
  emptyState,
  onEdit,
  rows,
}: AccessGroupLedgerTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>用户组</TableHead>
          <TableHead>描述</TableHead>
          <TableHead>成员</TableHead>
          <TableHead className="text-right">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
              {emptyState}
            </TableCell>
          </TableRow>
        ) : null}
        {rows.map((row) => (
          <TableRow key={row.group.id}>
            <TableCell className="font-medium">
              <TableCellText className="max-w-[14rem]">{row.group.name}</TableCellText>
            </TableCell>
            <TableCell>
              <TableCellText className="max-w-[18rem]">{row.descriptionLabel}</TableCellText>
            </TableCell>
            <TableCell>{row.membersLabel}</TableCell>
            <TableCell className="text-right">
              <Button variant="outline" size="sm" onClick={() => onEdit(row.group)}>
                编辑
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
