import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useFirmwareList } from "../hooks/use-firmware"
import { formatBytes, truncateHex } from "../lib/firmware-utils"

export function FirmwareListCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t, i18n } = useTranslation()
  const [channel, setChannel] = useState("")
  const query = useFirmwareList(token, tenantID, channel)
  const items = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.firmware.list.title")}</CardTitle>
        <CardDescription>{t("ota.firmware.list.description")}</CardDescription>
        <Select value={channel || "all"} onValueChange={(v) => setChannel(v === "all" ? "" : v)}>
          <SelectTrigger className="w-40"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("ota.firmware.list.channelAll")}</SelectItem>
            <SelectItem value="stable">stable</SelectItem>
            <SelectItem value="beta">beta</SelectItem>
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.list.loading")}</p>
        ) : query.error ? (
          <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.firmware.errors.generic")}</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.list.empty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("ota.firmware.list.colVersion")}</TableHead>
                <TableHead>{t("ota.firmware.list.colChannel")}</TableHead>
                <TableHead>{t("ota.firmware.list.colSha")}</TableHead>
                <TableHead>{t("ota.firmware.list.colSize")}</TableHead>
                <TableHead>{t("ota.firmware.list.colUploadedBy")}</TableHead>
                <TableHead>{t("ota.firmware.list.colCreated")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((fw) => (
                <TableRow key={fw.id}>
                  <TableCell>{fw.version}</TableCell>
                  <TableCell>{fw.channel ?? "—"}</TableCell>
                  <TableCell><span title={fw.sha256} className="font-mono text-xs">{truncateHex(fw.sha256)}</span></TableCell>
                  <TableCell>{formatBytes(fw.size_bytes)}</TableCell>
                  <TableCell>{fw.uploaded_by ?? "—"}</TableCell>
                  <TableCell>{new Date(fw.created_at).toLocaleString(i18n.language)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
