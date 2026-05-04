import { Plug2Icon, RefreshCwIcon, SendIcon, ShieldEllipsisIcon, UnplugIcon } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type Door, type Gateway, type GatewayDevice, type Tenant } from "@/lib/api"

type CommandTaskStatus = "queued" | "dispatching" | "delivered" | "acknowledged"

type CommandTask = {
  task_id: string
  gateway_id: string
  command: string
  status: CommandTaskStatus
}

type GatewayDeviceKind = "reader" | "door_controller" | "relay" | "sensor" | "legacy_reader" | "legacy_controller"
type GatewayDeviceSource = "mistypass_procured" | "legacy_integration"
type GatewayDeviceProtocol = "auto" | "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"

type GatewayCommandCenterCardProps = {
  gatewayOpsEditable: boolean
  readOnlyBoundaryHint: string
  selectedGateway: string
  onSelectedGatewayChange: (value: string) => void
  gateways: Gateway[]
  selectedDoorID: string
  onSelectedDoorIDChange: (value: string) => void
  availableDoors: Door[]
  commandBusy: boolean
  onBindDoor: () => void
  selectedBoundDoorID: string
  onSelectedBoundDoorIDChange: (value: string) => void
  boundDoors: { id: string; name: string }[]
  onUnbindDoor: () => void
  selectedGatewayRecord?: Gateway
  platformViewer: boolean
  buildingAdmin: boolean
  tenantByID: Map<string, Tenant>
  configVersion: string
  onConfigVersionChange: (value: string) => void
  onPublishConfig: () => void
  onRebootGateway: () => void
  commandLog: string
  selectedGatewayRemainSlots: number
  selectedGatewayDeviceOnline: number
  selectedGatewayDevices: GatewayDevice[]
  deviceSerialNumber: string
  onDeviceSerialNumberChange: (value: string) => void
  deviceKind: GatewayDeviceKind
  onDeviceKindChange: (value: GatewayDeviceKind) => void
  deviceSource: GatewayDeviceSource
  onDeviceSourceChange: (value: GatewayDeviceSource) => void
  deviceStatus: "online" | "offline"
  onDeviceStatusChange: (value: "online" | "offline") => void
  deviceProtocol: GatewayDeviceProtocol
  onDeviceProtocolChange: (value: GatewayDeviceProtocol) => void
  onRegisterGatewayDevice: () => void
  rs485BaudRate: string
  onRS485BaudRateChange: (value: string) => void
  rs485Parity: "none" | "even" | "odd"
  onRS485ParityChange: (value: "none" | "even" | "odd") => void
  rs485StopBits: 1 | 2
  onRS485StopBitsChange: (value: 1 | 2) => void
  rs485Address: string
  onRS485AddressChange: (value: string) => void
  rs485TimeoutMS: string
  onRS485TimeoutMSChange: (value: string) => void
  onProbeLegacyDevices: () => void
  legacyProbeCandidates: string[]
  onUseLegacyCandidate: (value: string) => void
  deviceKindLabel: (kind: GatewayDevice["kind"]) => string
  deviceSourceLabel: (source: GatewayDevice["source"]) => string
  deviceProtocolLabel: (protocol: GatewayDevice["protocol"]) => string
  statusLabel: (status: string) => string
  commandTasks: CommandTask[]
  commandStatusLabel: (status: CommandTaskStatus) => string
  commandStatusVariant: (status: CommandTaskStatus) => "secondary" | "outline" | "default"
}

export function GatewayCommandCenterCard({
  gatewayOpsEditable,
  readOnlyBoundaryHint,
  selectedGateway,
  onSelectedGatewayChange,
  gateways,
  selectedDoorID,
  onSelectedDoorIDChange,
  availableDoors,
  commandBusy,
  onBindDoor,
  selectedBoundDoorID,
  onSelectedBoundDoorIDChange,
  boundDoors,
  onUnbindDoor,
  selectedGatewayRecord,
  platformViewer,
  buildingAdmin,
  tenantByID,
  configVersion,
  onConfigVersionChange,
  onPublishConfig,
  onRebootGateway,
  commandLog,
  selectedGatewayRemainSlots,
  selectedGatewayDeviceOnline,
  selectedGatewayDevices,
  deviceSerialNumber,
  onDeviceSerialNumberChange,
  deviceKind,
  onDeviceKindChange,
  deviceSource,
  onDeviceSourceChange,
  deviceStatus,
  onDeviceStatusChange,
  deviceProtocol,
  onDeviceProtocolChange,
  onRegisterGatewayDevice,
  rs485BaudRate,
  onRS485BaudRateChange,
  rs485Parity,
  onRS485ParityChange,
  rs485StopBits,
  onRS485StopBitsChange,
  rs485Address,
  onRS485AddressChange,
  rs485TimeoutMS,
  onRS485TimeoutMSChange,
  onProbeLegacyDevices,
  legacyProbeCandidates,
  onUseLegacyCandidate,
  deviceKindLabel,
  deviceSourceLabel,
  deviceProtocolLabel,
  statusLabel,
  commandTasks,
  commandStatusLabel,
  commandStatusVariant,
}: GatewayCommandCenterCardProps) {
  const { t } = useTranslation()
  const [rebootConfirmOpen, setRebootConfirmOpen] = useState(false)
  const gatewayScopeLabel = selectedGatewayRecord
    ? platformViewer
      ? t("gateways.commandCenter.scope.platform", {
          tenant:
            tenantByID.get(selectedGatewayRecord.tenant_id)?.name ??
            selectedGatewayRecord.tenant_id,
        })
      : buildingAdmin
        ? t("gateways.commandCenter.scope.buildingAdmin")
        : t("gateways.commandCenter.scope.tenant")
    : ""
  const deviceSlotLabel = t("gateways.commandCenter.deviceMgmt.remaining", {
    remain: selectedGatewayRemainSlots,
    capacity: selectedGatewayRecord?.device_capacity ?? 0,
  })
  const commandBusyDisabledReason = commandBusy ? t("gateways.disabledReasons.commandBusy") : ""
  const commandActionDisabledReason = !gatewayOpsEditable
    ? t("gateways.disabledReasons.readOnly")
    : commandBusyDisabledReason || (!selectedGateway ? t("gateways.disabledReasons.selectGateway") : "")
  const bindDoorDisabledReason =
    commandActionDisabledReason || (!selectedDoorID ? t("gateways.disabledReasons.selectDoor") : "")
  const unbindDoorDisabledReason =
    commandActionDisabledReason || (!selectedBoundDoorID ? t("gateways.disabledReasons.selectBoundDoor") : "")
  const publishConfigDisabledReason =
    commandActionDisabledReason || (!configVersion.trim() ? t("gateways.disabledReasons.enterConfigVersion") : "")
  const rebootDisabledReason = commandActionDisabledReason
  const deviceFieldDisabledReason = !gatewayOpsEditable ? t("gateways.disabledReasons.readOnly") : ""
  const mountDeviceDisabledReason =
    commandActionDisabledReason ||
    (selectedGatewayRemainSlots <= 0
      ? t("gateways.disabledReasons.noDeviceSlots", {
          remaining: deviceSlotLabel,
        })
      : !deviceSerialNumber.trim()
        ? t("gateways.disabledReasons.enterDeviceSerial")
        : "")
  const probeLegacyDisabledReason = commandActionDisabledReason

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("gateways.commandCenter.title")}</CardTitle>
        <CardDescription>
          {gatewayOpsEditable
            ? t("gateways.commandCenter.descriptionEditable")
            : t("gateways.commandCenter.descriptionReadonly")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!gatewayOpsEditable ? (
          <div className="rounded-lg border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
            {t("gateways.commandCenter.readonlyNotice", {
              hint: readOnlyBoundaryHint,
            })}
          </div>
        ) : null}
        <div className="space-y-1.5">
          <Label>{t("gateways.commandCenter.targetGatewayLabel")}</Label>
          <Select value={selectedGateway} onValueChange={onSelectedGatewayChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("gateways.commandCenter.targetGatewayPlaceholder")} />
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

        <div className="grid gap-2 lg:grid-cols-[minmax(0,1fr)_auto]">
          <Select value={selectedDoorID} onValueChange={onSelectedDoorIDChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("gateways.commandCenter.bindDoorPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {availableDoors.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name} ({item.id})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="outline"
            className="w-full lg:w-auto"
            disabled={Boolean(bindDoorDisabledReason)}
            title={bindDoorDisabledReason || undefined}
            onClick={onBindDoor}
          >
            <ShieldEllipsisIcon className="mr-1.5 size-4" />
            {t("gateways.commandCenter.bindDoor")}
          </Button>
          {bindDoorDisabledReason ? (
            <p className="lg:col-span-2 text-xs text-muted-foreground">{bindDoorDisabledReason}</p>
          ) : null}
        </div>
        <div className="grid gap-2 lg:grid-cols-[minmax(0,1fr)_auto]">
          <Select value={selectedBoundDoorID} onValueChange={onSelectedBoundDoorIDChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("gateways.commandCenter.unbindDoorPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {boundDoors.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name} ({item.id})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="outline"
            className="w-full lg:w-auto"
            disabled={Boolean(unbindDoorDisabledReason)}
            title={unbindDoorDisabledReason || undefined}
            onClick={onUnbindDoor}
          >
            <UnplugIcon className="mr-1.5 size-4" />
            {t("gateways.commandCenter.unbindDoor")}
          </Button>
          {unbindDoorDisabledReason ? (
            <p className="lg:col-span-2 text-xs text-muted-foreground">{unbindDoorDisabledReason}</p>
          ) : null}
        </div>
        {selectedGatewayRecord ? (
          <p className="mp-kpi-note">
            {t("gateways.commandCenter.scope.prefix")}
            {gatewayScopeLabel}
            {selectedGatewayRecord.building_id
              ? t("gateways.commandCenter.scope.buildingSuffix", {
                  building: selectedGatewayRecord.building_id,
                })
              : ""}
          </p>
        ) : null}
        {selectedGatewayRecord && availableDoors.length === 0 ? (
          <p className="mp-kpi-note">{t("gateways.commandCenter.noAvailableDoors")}</p>
        ) : null}

        <div className="grid gap-2 lg:grid-cols-[minmax(0,1fr)_auto_auto]">
          <Input
            value={configVersion}
            onChange={(event) => onConfigVersionChange(event.target.value)}
            placeholder={t("gateways.commandCenter.configVersionPlaceholder")}
            disabled={!gatewayOpsEditable}
            title={deviceFieldDisabledReason || undefined}
          />
          <Button
            variant="secondary"
            className="w-full lg:w-auto"
            disabled={Boolean(publishConfigDisabledReason)}
            title={publishConfigDisabledReason || undefined}
            onClick={onPublishConfig}
          >
            <SendIcon className="mr-1.5 size-4" />
            {t("gateways.commandCenter.publishConfig")}
          </Button>
          <Button
            variant="outline"
            className="w-full lg:w-auto"
            disabled={Boolean(rebootDisabledReason)}
            title={rebootDisabledReason || undefined}
            onClick={() => setRebootConfirmOpen(true)}
          >
            <RefreshCwIcon className="mr-1.5 size-4" />
            {t("gateways.commandCenter.reboot")}
          </Button>
          {publishConfigDisabledReason || rebootDisabledReason ? (
            <p className="lg:col-span-3 text-xs text-muted-foreground">
              {publishConfigDisabledReason || rebootDisabledReason}
            </p>
          ) : null}
        </div>

        <Dialog open={rebootConfirmOpen} onOpenChange={setRebootConfirmOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("gateways.commandCenter.rebootConfirm.title")}</DialogTitle>
              <DialogDescription>
                {t("gateways.commandCenter.rebootConfirm.description")}
              </DialogDescription>
            </DialogHeader>
            <div className="rounded-lg border border-card-task-border bg-[#fafafa] px-3 py-2">
              <p className="text-xs font-medium uppercase text-[#62636a]">
                {t("gateways.commandCenter.rebootConfirm.targetLabel")}
              </p>
              <p className="mt-1 text-sm font-medium text-content-heading">
                {selectedGatewayRecord?.id ?? selectedGateway}
              </p>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="interaction">
                  {t("gateways.commandCenter.rebootConfirm.cancel")}
                </Button>
              </DialogClose>
              <Button
                type="button"
                variant="destructive"
                disabled={Boolean(rebootDisabledReason)}
                title={rebootDisabledReason || undefined}
                onClick={() => {
                  setRebootConfirmOpen(false)
                  onRebootGateway()
                }}
              >
                {t("gateways.commandCenter.rebootConfirm.confirm")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <div className="rounded-lg border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
          {commandLog}
        </div>

        <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs font-medium text-muted-foreground">{t("gateways.commandCenter.deviceMgmt.title")}</p>
            <Badge variant={selectedGatewayRemainSlots > 0 ? "outline" : "destructive"}>
              {deviceSlotLabel}
            </Badge>
          </div>
          <p className="mp-kpi-note">
            {t("gateways.commandCenter.deviceMgmt.online", {
              online: selectedGatewayDeviceOnline,
              total: selectedGatewayDevices.length,
            })}
          </p>
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_auto]">
            <Input
              value={deviceSerialNumber}
              onChange={(event) => onDeviceSerialNumberChange(event.target.value)}
              placeholder={t("gateways.commandCenter.deviceMgmt.serialPlaceholder")}
              disabled={!gatewayOpsEditable}
              title={deviceFieldDisabledReason || undefined}
            />
            <Select value={deviceKind} disabled={!gatewayOpsEditable} onValueChange={(value: GatewayDeviceKind) => onDeviceKindChange(value)}>
              <SelectTrigger title={deviceFieldDisabledReason || undefined}>
                <SelectValue placeholder={t("gateways.commandCenter.deviceMgmt.kindPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="reader">{t("gateways.deviceKind.reader")}</SelectItem>
                <SelectItem value="door_controller">{t("gateways.deviceKind.doorController")}</SelectItem>
                <SelectItem value="relay">{t("gateways.deviceKind.relay")}</SelectItem>
                <SelectItem value="sensor">{t("gateways.deviceKind.sensor")}</SelectItem>
                <SelectItem value="legacy_reader">{t("gateways.deviceKind.legacyReader")}</SelectItem>
                <SelectItem value="legacy_controller">{t("gateways.deviceKind.legacyController")}</SelectItem>
              </SelectContent>
            </Select>
            <Select value={deviceSource} disabled={!gatewayOpsEditable} onValueChange={(value: GatewayDeviceSource) => onDeviceSourceChange(value)}>
              <SelectTrigger title={deviceFieldDisabledReason || undefined}>
                <SelectValue placeholder={t("gateways.commandCenter.deviceMgmt.sourcePlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="mistypass_procured">{t("gateways.deviceSource.mistypassProcured")}</SelectItem>
                <SelectItem value="legacy_integration">{t("gateways.deviceSource.legacyIntegration")}</SelectItem>
              </SelectContent>
            </Select>
            <Select value={deviceStatus} disabled={!gatewayOpsEditable} onValueChange={(value: "online" | "offline") => onDeviceStatusChange(value)}>
              <SelectTrigger title={deviceFieldDisabledReason || undefined}>
                <SelectValue placeholder={t("gateways.commandCenter.deviceMgmt.statusPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="online">{t("gateways.status.online")}</SelectItem>
                <SelectItem value="offline">{t("gateways.status.offline")}</SelectItem>
              </SelectContent>
            </Select>
            <Select value={deviceProtocol} disabled={!gatewayOpsEditable} onValueChange={(value: GatewayDeviceProtocol) => onDeviceProtocolChange(value)}>
              <SelectTrigger title={deviceFieldDisabledReason || undefined}>
                <SelectValue placeholder={t("gateways.commandCenter.deviceMgmt.protocolPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="auto">{t("gateways.commandCenter.deviceMgmt.protocolAuto")}</SelectItem>
                <SelectItem value="wiegand_26">Wiegand 26</SelectItem>
                <SelectItem value="wiegand_34">Wiegand 34</SelectItem>
                <SelectItem value="osdp_v2">OSDP v2</SelectItem>
                <SelectItem value="rs485">RS485</SelectItem>
                <SelectItem value="ble">BLE</SelectItem>
              </SelectContent>
            </Select>
            <Button
              variant="outline"
              className="w-full md:col-span-2 xl:col-span-1 xl:w-auto"
              disabled={Boolean(mountDeviceDisabledReason)}
              title={mountDeviceDisabledReason || undefined}
              onClick={onRegisterGatewayDevice}
            >
              <Plug2Icon className="mr-1.5 size-4" />
              {t("gateways.commandCenter.deviceMgmt.mountDevice")}
            </Button>
            {mountDeviceDisabledReason ? (
              <p className="md:col-span-2 xl:col-span-6 text-xs text-muted-foreground">{mountDeviceDisabledReason}</p>
            ) : null}
          </div>
          {deviceProtocol === "rs485" ? (
            <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
              <Input
                value={rs485BaudRate}
                onChange={(event) => onRS485BaudRateChange(event.target.value)}
                placeholder="baud_rate (9600)"
                disabled={!gatewayOpsEditable}
                title={deviceFieldDisabledReason || undefined}
              />
              <Select value={rs485Parity} disabled={!gatewayOpsEditable} onValueChange={(value: "none" | "even" | "odd") => onRS485ParityChange(value)}>
                <SelectTrigger title={deviceFieldDisabledReason || undefined}>
                  <SelectValue placeholder="parity" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">none</SelectItem>
                  <SelectItem value="even">even</SelectItem>
                  <SelectItem value="odd">odd</SelectItem>
                </SelectContent>
              </Select>
              <Select value={String(rs485StopBits)} disabled={!gatewayOpsEditable} onValueChange={(value: string) => onRS485StopBitsChange(value === "2" ? 2 : 1)}>
                <SelectTrigger title={deviceFieldDisabledReason || undefined}>
                  <SelectValue placeholder="stop_bits" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">stop_bits=1</SelectItem>
                  <SelectItem value="2">stop_bits=2</SelectItem>
                </SelectContent>
              </Select>
              <Input
                value={rs485Address}
                onChange={(event) => onRS485AddressChange(event.target.value)}
                placeholder="device_address (1..247)"
                disabled={!gatewayOpsEditable}
                title={deviceFieldDisabledReason || undefined}
              />
              <Input
                value={rs485TimeoutMS}
                onChange={(event) => onRS485TimeoutMSChange(event.target.value)}
                placeholder="timeout_ms (100..5000)"
                disabled={!gatewayOpsEditable}
                title={deviceFieldDisabledReason || undefined}
              />
            </div>
          ) : null}
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={Boolean(probeLegacyDisabledReason)}
              title={probeLegacyDisabledReason || undefined}
              onClick={onProbeLegacyDevices}
            >
              {t("gateways.commandCenter.deviceMgmt.probeLegacy")}
            </Button>
            {legacyProbeCandidates.map((item) => (
              <Button
                key={item}
                variant="outline"
                size="sm"
                onClick={() => {
                  onUseLegacyCandidate(item)
                }}
              >
                {item}
              </Button>
            ))}
            {probeLegacyDisabledReason ? (
              <p className="w-full basis-full text-xs text-muted-foreground">{probeLegacyDisabledReason}</p>
            ) : null}
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("gateways.commandCenter.deviceMgmt.table.serial")}</TableHead>
                <TableHead>{t("gateways.commandCenter.deviceMgmt.table.kind")}</TableHead>
                <TableHead>{t("gateways.commandCenter.deviceMgmt.table.source")}</TableHead>
                <TableHead>{t("gateways.commandCenter.deviceMgmt.table.protocol")}</TableHead>
                <TableHead>{t("gateways.commandCenter.deviceMgmt.table.status")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {selectedGatewayDevices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-4 text-center text-muted-foreground">
                    {t("gateways.commandCenter.deviceMgmt.table.empty")}
                  </TableCell>
                </TableRow>
              ) : null}
              {selectedGatewayDevices.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">
                    <TableCellText className="max-w-[14rem]">{item.serial_number}</TableCellText>
                  </TableCell>
                  <TableCell>{deviceKindLabel(item.kind)}</TableCell>
                  <TableCell>{deviceSourceLabel(item.source)}</TableCell>
                  <TableCell>
                    <div className="space-y-0.5">
                      <div>{deviceProtocolLabel(item.protocol)}</div>
                      {item.protocol === "rs485" && item.rs485_config ? (
                        <div className="text-[11px] text-muted-foreground">
                          {`addr=${item.rs485_config.device_address} baud=${item.rs485_config.baud_rate}`}
                        </div>
                      ) : null}
                      {item.protocol === "rs485" && item.rs485_health ? (
                        <div className="text-[11px] text-muted-foreground">
                          {`retry=${item.rs485_health.retry_count} timeout=${item.rs485_health.timeout_count} collision=${item.rs485_health.collision_count}`}
                        </div>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={item.status === "online" ? "outline" : "destructive"}>
                      {statusLabel(item.status)}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
          <p className="text-xs font-medium text-muted-foreground">{t("gateways.commandCenter.progress.title")}</p>
          {commandTasks.length === 0 ? (
            <p className="mp-kpi-note">{t("gateways.commandCenter.progress.empty")}</p>
          ) : (
            <div className="space-y-2">
              {commandTasks.map((item) => (
                <div key={item.task_id} className="flex items-center justify-between gap-3 rounded-md border bg-background px-2 py-1.5">
                  <div className="min-w-0">
                    <p className="truncate text-xs font-medium">
                      {item.command} / {item.gateway_id}
                    </p>
                    <p className="text-[11px] text-muted-foreground">{item.task_id}</p>
                  </div>
                  <Badge variant={commandStatusVariant(item.status)}>{commandStatusLabel(item.status)}</Badge>
                </div>
              ))}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
