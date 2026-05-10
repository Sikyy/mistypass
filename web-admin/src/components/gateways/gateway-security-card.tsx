import { RotateCcwIcon, ShieldOffIcon } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { Gateway, GatewayCertificateRevocation, Tenant } from "@/lib/api"

type GatewaySecurityCardProps = {
  gatewayOpsEditable: boolean
  readOnlyBoundaryHint: string
  commandBusy: boolean
  tenantID: string
  onTenantIDChange: (value: string) => void
  platformViewer: boolean
  tenants: Tenant[]
  gateways: Gateway[]
  selectedGateway: string
  revocations: GatewayCertificateRevocation[]
  revocationsLoading: boolean
  onRevokeCertificateSerial: (payload: {
    tenantID?: string
    gatewayID?: string
    serialNumber: string
    reason?: string
  }) => Promise<boolean>
  onRestoreCertificateSerial: (item: GatewayCertificateRevocation) => void
}

export function GatewaySecurityCard({
  gatewayOpsEditable,
  readOnlyBoundaryHint,
  commandBusy,
  tenantID,
  onTenantIDChange,
  platformViewer,
  tenants,
  gateways,
  selectedGateway,
  revocations,
  revocationsLoading,
  onRevokeCertificateSerial,
  onRestoreCertificateSerial,
}: GatewaySecurityCardProps) {
  const { t, i18n } = useTranslation()
  const [serialNumber, setSerialNumber] = useState("")
  const [gatewayID, setGatewayID] = useState(selectedGateway)
  const [reason, setReason] = useState("")
  const editable = gatewayOpsEditable && !commandBusy
  const submitDisabledReason = !gatewayOpsEditable
    ? t("gateways.disabledReasons.readOnly")
    : commandBusy
      ? t("gateways.disabledReasons.commandBusy")
      : !serialNumber.trim()
        ? t("gateways.disabledReasons.enterCertificateSerial")
        : platformViewer && !tenantID.trim()
          ? t("gateways.disabledReasons.selectTenant")
          : ""
  const visibleGatewayID = gatewayID || selectedGateway
  const revokedRuntimeTotal = revocations.filter((item) => item.source === "runtime").length
  const revokedEnvironmentTotal = revocations.filter((item) => item.source === "environment").length

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("gateways.security.title")}</CardTitle>
        <CardDescription>
          {gatewayOpsEditable ? t("gateways.security.descriptionEditable") : t("gateways.security.descriptionReadonly")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!gatewayOpsEditable ? (
          <div className="rounded-lg border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
            {t("gateways.security.readonlyNotice", { hint: readOnlyBoundaryHint })}
          </div>
        ) : null}

        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div className="space-y-1.5">
            <Label>{t("gateways.security.form.serialLabel")}</Label>
            <Input
              value={serialNumber}
              onChange={(event) => setSerialNumber(event.target.value)}
              placeholder={t("gateways.security.form.serialPlaceholder")}
              disabled={!editable}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{t("gateways.security.form.gatewayLabel")}</Label>
            <Select value={visibleGatewayID} disabled={!editable || gateways.length === 0} onValueChange={setGatewayID}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("gateways.security.form.gatewayPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                {gateways.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.id} ({item.serial_number})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {platformViewer ? (
            <div className="space-y-1.5">
              <Label>{t("gateways.security.form.tenantLabel")}</Label>
              <Select value={tenantID} disabled={!editable} onValueChange={onTenantIDChange}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("gateways.security.form.tenantPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {tenants.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name} ({item.id})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : null}
          <div className={platformViewer ? "space-y-1.5" : "space-y-1.5 lg:col-span-2"}>
            <Label>{t("gateways.security.form.reasonLabel")}</Label>
            <Textarea
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={t("gateways.security.form.reasonPlaceholder")}
              disabled={!editable}
              className="min-h-20"
            />
          </div>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="destructive">
              {t("gateways.security.runtimeTotal", { count: revokedRuntimeTotal })}
            </Badge>
            <Badge variant="outline">
              {t("gateways.security.environmentTotal", { count: revokedEnvironmentTotal })}
            </Badge>
          </div>
          <Button
            type="button"
            variant="destructive"
            disabled={Boolean(submitDisabledReason)}
            title={submitDisabledReason || undefined}
            onClick={async () => {
              const ok = await onRevokeCertificateSerial({
                tenantID: tenantID.trim() || undefined,
                gatewayID: visibleGatewayID || undefined,
                serialNumber: serialNumber.trim(),
                reason: reason.trim() || undefined,
              })
              if (ok) {
                setSerialNumber("")
                setReason("")
              }
            }}
          >
            <ShieldOffIcon className="mr-1.5 size-4" />
            {t("gateways.security.form.submit")}
          </Button>
        </div>
        {submitDisabledReason ? <p className="text-xs text-muted-foreground">{submitDisabledReason}</p> : null}

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("gateways.security.table.serial")}</TableHead>
              <TableHead>{t("gateways.security.table.gateway")}</TableHead>
              <TableHead>{t("gateways.security.table.source")}</TableHead>
              <TableHead>{t("gateways.security.table.revokedAt")}</TableHead>
              <TableHead>{t("gateways.security.table.reason")}</TableHead>
              <TableHead>{t("gateways.security.table.action")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {revocationsLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="py-6 text-center text-muted-foreground">
                  {t("gateways.security.loading")}
                </TableCell>
              </TableRow>
            ) : null}
            {!revocationsLoading && revocations.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-6 text-center text-muted-foreground">
                  {t("gateways.security.empty")}
                </TableCell>
              </TableRow>
            ) : null}
            {!revocationsLoading &&
              revocations.map((item) => {
                const restoreDisabledReason =
                  item.source === "environment"
                    ? t("gateways.disabledReasons.environmentRevocation")
                    : !gatewayOpsEditable
                      ? t("gateways.disabledReasons.readOnly")
                      : commandBusy
                        ? t("gateways.disabledReasons.commandBusy")
                        : ""
                return (
                  <TableRow key={`${item.source}-${item.serial_number}`}>
                    <TableCell className="font-medium">
                      <TableCellText className="max-w-[12rem]">{item.serial_number}</TableCellText>
                    </TableCell>
                    <TableCell>{item.gateway_id || "-"}</TableCell>
                    <TableCell>
                      <Badge variant={item.source === "environment" ? "outline" : "destructive"}>
                        {item.source === "environment"
                          ? t("gateways.security.source.environment")
                          : t("gateways.security.source.runtime")}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {item.revoked_at ? new Date(item.revoked_at).toLocaleString(i18n.language) : "-"}
                    </TableCell>
                    <TableCell>
                      <TableCellText className="max-w-[14rem]">{item.reason || "-"}</TableCellText>
                    </TableCell>
                    <TableCell>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={Boolean(restoreDisabledReason)}
                        title={restoreDisabledReason || undefined}
                        onClick={() => onRestoreCertificateSerial(item)}
                      >
                        <RotateCcwIcon className="mr-1.5 size-4" />
                        {t("gateways.security.restore")}
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
