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
import { type TemporaryAccess } from "@/lib/api"

type LedgerBadgeVariant = "outline" | "secondary" | "destructive"

export type AccessGrantLedgerRow = {
  grant: TemporaryAccess
  tenantLabel?: string
  scopeLabel: string
  granteeLabel: string
  deliveryLabel: string
  authorizedByRole: string
  authorizedByEmail: string
  authorizedAtLabel: string
  statusLabel: string
  statusVariant: LedgerBadgeVariant
  validUntilLabel: string
  remainingLabel: string
  remainingVariant: LedgerBadgeVariant
}

type AccessGrantLedgerTableProps = {
  rows: AccessGrantLedgerRow[]
  platformViewer: boolean
  emptyState: string
  onOpenGrant: (grant: TemporaryAccess) => void
}

export function AccessGrantLedgerTable({
  rows,
  platformViewer,
  emptyState,
  onOpenGrant,
}: AccessGrantLedgerTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>授权 ID</TableHead>
          {platformViewer ? <TableHead>租户</TableHead> : null}
          <TableHead>范围</TableHead>
          <TableHead>被授权人</TableHead>
          <TableHead>方式</TableHead>
          <TableHead>授权人</TableHead>
          <TableHead>授权时间</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>有效期倒计时</TableHead>
          <TableHead className="text-right">详情</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={platformViewer ? 10 : 9} className="py-8 text-center text-muted-foreground">
              {emptyState}
            </TableCell>
          </TableRow>
        ) : null}
        {rows.map((row) => (
          <TableRow key={row.grant.id}>
            <TableCell className="font-medium">
              <TableCellText className="max-w-[12rem]">{row.grant.id}</TableCellText>
            </TableCell>
            {platformViewer ? (
              <TableCell>
                <TableCellText className="max-w-[13rem]">{row.tenantLabel || "-"}</TableCellText>
              </TableCell>
            ) : null}
            <TableCell>
              <TableCellText className="max-w-[16rem]">{row.scopeLabel}</TableCellText>
            </TableCell>
            <TableCell>
              <TableCellText className="max-w-[12rem]">{row.granteeLabel}</TableCellText>
            </TableCell>
            <TableCell>{row.deliveryLabel}</TableCell>
            <TableCell>
              <div className="mp-kpi-note">{row.authorizedByRole}</div>
              <TableCellText className="max-w-[14rem]">{row.authorizedByEmail}</TableCellText>
            </TableCell>
            <TableCell>{row.authorizedAtLabel}</TableCell>
            <TableCell>
              <Badge variant={row.statusVariant}>{row.statusLabel}</Badge>
            </TableCell>
            <TableCell>
              <div>{row.validUntilLabel}</div>
              <Badge variant={row.remainingVariant} className="mt-1">
                {row.remainingLabel}
              </Badge>
            </TableCell>
            <TableCell className="text-right">
              <Button variant="outline" size="sm" onClick={() => onOpenGrant(row.grant)}>
                查看
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
