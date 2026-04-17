import { Badge } from "@/components/ui/badge"
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
import { type AccessPolicy } from "@/lib/api"

type PolicyLedgerRow = {
  policy: AccessPolicy
  scopeLabel: string
  scheduleLabel: string
  membersLabel: string
  statusLabel: string
  statusVariant: "outline" | "secondary"
}

type AccessPolicyLedgerTableProps = {
  rows: PolicyLedgerRow[]
  emptyState: string
  onEdit: (policy: AccessPolicy) => void
}

export function AccessPolicyLedgerTable({
  rows,
  emptyState,
  onEdit,
}: AccessPolicyLedgerTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>策略</TableHead>
          <TableHead>范围</TableHead>
          <TableHead>计划</TableHead>
          <TableHead>人数</TableHead>
          <TableHead>状态</TableHead>
          <TableHead className="text-right">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
              {emptyState}
            </TableCell>
          </TableRow>
        ) : null}
        {rows.map((row) => (
          <TableRow key={row.policy.id}>
            <TableCell className="font-medium">
              <TableCellText className="max-w-[14rem]">{row.policy.name}</TableCellText>
            </TableCell>
            <TableCell>
              <TableCellText className="max-w-[16rem]">{row.scopeLabel}</TableCellText>
            </TableCell>
            <TableCell>
              <TableCellText className="max-w-[12rem]">{row.scheduleLabel}</TableCellText>
            </TableCell>
            <TableCell>{row.membersLabel}</TableCell>
            <TableCell>
              <Badge variant={row.statusVariant}>{row.statusLabel}</Badge>
            </TableCell>
            <TableCell className="text-right">
              <Button variant="outline" size="sm" onClick={() => onEdit(row.policy)}>
                编辑
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
