import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { type GatewaySerialInventoryItem } from "@/lib/api"

const inventoryProductTypeValues = ["gateway", "reader", "controller", "relay", "sensor"] as const
type SingleImportFormValues = {
  serial_number: string
  product_type: "gateway" | "reader" | "controller" | "relay" | "sensor"
  batch_code?: string
}
type CSVImportFormValues = {
  csv_content: string
}

type GatewaySerialInventoryIngestCardProps = {
  platformViewer: boolean
  inventoryEditable: boolean
  readOnlyBoundaryHint: string
  submitting: boolean
  tenantID: string
  onImportSerialInventory: (values: {
    serial_number: string
    product_type: "gateway" | "reader" | "controller" | "relay" | "sensor"
    batch_code?: string
  }) => Promise<boolean>
  onImportSerialInventoryCSV: (csvContent: string) => Promise<boolean>
  commandBusy: boolean
  onExportSerialInventoryCSV: () => void
  visibleSerialInventory: GatewaySerialInventoryItem[]
}

export function GatewaySerialInventoryIngestCard({
  platformViewer,
  inventoryEditable,
  readOnlyBoundaryHint,
  submitting,
  tenantID,
  onImportSerialInventory,
  onImportSerialInventoryCSV,
  commandBusy,
  onExportSerialInventoryCSV,
  visibleSerialInventory,
}: GatewaySerialInventoryIngestCardProps) {
  const { t } = useTranslation()
  const singleImportSchema = useMemo(
    () =>
      z.object({
        serial_number: z
          .string()
          .trim()
          .min(3, t("gateways.validation.serialNumberMin"))
          .max(128, t("gateways.validation.serialNumberMax")),
        product_type: z.enum(inventoryProductTypeValues),
        batch_code: z
          .string()
          .trim()
          .max(128, t("gateways.inventoryIngest.validation.batchCodeMax"))
          .optional()
          .or(z.literal("")),
      }),
    [t]
  )
  const csvImportSchema = useMemo(
    () =>
      z.object({
        csv_content: z
          .string()
          .trim()
          .min(1, t("gateways.inventoryIngest.validation.csvContentRequired"))
          .max(200000, t("gateways.inventoryIngest.validation.csvContentMax")),
      }),
    [t]
  )
  const availableCount = visibleSerialInventory.filter((item) => item.status === "available").length
  const consumedCount = visibleSerialInventory.filter((item) => item.status === "consumed").length
  const frozenCount = visibleSerialInventory.filter((item) => item.status === "frozen").length
  const scrappedCount = visibleSerialInventory.filter((item) => item.status === "scrapped").length
  const singleImportForm = useForm<SingleImportFormValues>({
    resolver: zodResolver(singleImportSchema),
    defaultValues: {
      serial_number: "",
      product_type: "gateway",
      batch_code: "",
    },
  })
  const csvImportForm = useForm<CSVImportFormValues>({
    resolver: zodResolver(csvImportSchema),
    defaultValues: {
      csv_content: "",
    },
  })
  const singleFormError =
    singleImportForm.formState.errors.serial_number?.message ||
    singleImportForm.formState.errors.product_type?.message ||
    singleImportForm.formState.errors.batch_code?.message ||
    ""
  const csvFormError = csvImportForm.formState.errors.csv_content?.message || ""
  const inventorySubmitDisabledReason = submitting
    ? t("gateways.disabledReasons.commandBusy")
    : !tenantID.trim()
      ? t("gateways.disabledReasons.selectTenant")
      : ""
  const singleImportDisabledReason =
    inventorySubmitDisabledReason ||
    (singleImportForm.formState.isSubmitting ? t("gateways.disabledReasons.commandBusy") : "")
  const csvImportDisabledReason =
    inventorySubmitDisabledReason ||
    (csvImportForm.formState.isSubmitting ? t("gateways.disabledReasons.commandBusy") : "")
  const exportDisabledReason = commandBusy ? t("gateways.disabledReasons.commandBusy") : ""

  async function onSubmitSingleImport(values: SingleImportFormValues) {
    const succeeded = await onImportSerialInventory({
      serial_number: values.serial_number.trim(),
      product_type: values.product_type,
      batch_code: values.batch_code?.trim() || undefined,
    })
    if (succeeded) {
      singleImportForm.reset({
        serial_number: "",
        product_type: values.product_type,
        batch_code: "",
      })
    }
  }

  async function onSubmitCSVImport(values: CSVImportFormValues) {
    const succeeded = await onImportSerialInventoryCSV(values.csv_content)
    if (succeeded) {
      csvImportForm.reset({
        csv_content: "",
      })
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("gateways.inventoryIngest.title")}</CardTitle>
        <CardDescription>
          {inventoryEditable
            ? t("gateways.inventoryIngest.descriptionEditable")
            : t("gateways.inventoryIngest.descriptionReadonly")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {inventoryEditable ? (
          <>
            <form className="grid gap-3 md:grid-cols-[1.4fr_220px_1fr_auto]" onSubmit={singleImportForm.handleSubmit(onSubmitSingleImport)}>
              <Input
                {...singleImportForm.register("serial_number")}
                placeholder={t("gateways.inventoryIngest.serialPlaceholder")}
              />
              <Controller
                control={singleImportForm.control}
                name="product_type"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder={t("gateways.inventoryIngest.productTypePlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="gateway">{t("gateways.inventoryProductType.gateway")}</SelectItem>
                      <SelectItem value="reader">{t("gateways.inventoryProductType.reader")}</SelectItem>
                      <SelectItem value="controller">{t("gateways.inventoryProductType.controller")}</SelectItem>
                      <SelectItem value="relay">{t("gateways.inventoryProductType.relay")}</SelectItem>
                      <SelectItem value="sensor">{t("gateways.inventoryProductType.sensor")}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
              <Input
                {...singleImportForm.register("batch_code")}
                placeholder={t("gateways.inventoryIngest.batchCodePlaceholder")}
              />
              <Button
                type="submit"
                disabled={Boolean(singleImportDisabledReason)}
                title={singleImportDisabledReason || undefined}
              >
                {submitting ? t("gateways.inventoryIngest.importSubmitting") : t("gateways.inventoryIngest.importSubmit")}
              </Button>
              {singleImportDisabledReason ? (
                <p className="text-xs text-muted-foreground md:col-span-4">{singleImportDisabledReason}</p>
              ) : null}
              {singleFormError ? (
                <p className="text-sm text-destructive md:col-span-4">{singleFormError}</p>
              ) : null}
            </form>
            <form className="space-y-2 rounded-lg border bg-muted/20 p-3" onSubmit={csvImportForm.handleSubmit(onSubmitCSVImport)}>
              <p className="text-xs font-medium text-muted-foreground">{t("gateways.inventoryIngest.csvSectionTitle")}</p>
              <Textarea
                {...csvImportForm.register("csv_content")}
                rows={5}
                placeholder={
                  "serial_number,product_type,batch_code,source\nMP-GW-FACTORY-0001,gateway,batch-01,factory\nRD-FACTORY-0002,reader,batch-01,factory"
                }
              />
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="submit"
                  variant="secondary"
                  disabled={Boolean(csvImportDisabledReason)}
                  title={csvImportDisabledReason || undefined}
                >
                  {submitting ? t("gateways.inventoryIngest.csvSubmitting") : t("gateways.inventoryIngest.csvSubmit")}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={Boolean(exportDisabledReason)}
                  title={exportDisabledReason || undefined}
                  onClick={onExportSerialInventoryCSV}
                >
                  {t("gateways.inventoryIngest.exportCsv")}
                </Button>
                {csvImportDisabledReason || exportDisabledReason ? (
                  <p className="w-full basis-full text-xs text-muted-foreground">
                    {csvImportDisabledReason || exportDisabledReason}
                  </p>
                ) : null}
              </div>
              {csvFormError ? (
                <p className="text-sm text-destructive">{csvFormError}</p>
              ) : null}
              <p className="mp-kpi-note">
                {t("gateways.inventoryIngest.csvHint")}
              </p>
            </form>
          </>
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-muted/20 px-3 py-3">
            <p className="text-sm text-muted-foreground">
              {t("gateways.inventoryIngest.readonlyHint", { hint: readOnlyBoundaryHint })}
            </p>
            <Button
              type="button"
              variant="outline"
              disabled={Boolean(exportDisabledReason)}
              title={exportDisabledReason || undefined}
              onClick={onExportSerialInventoryCSV}
            >
              {t("gateways.inventoryIngest.exportCsv")}
            </Button>
            {exportDisabledReason ? (
              <p className="w-full basis-full text-xs text-muted-foreground">{exportDisabledReason}</p>
            ) : null}
          </div>
        )}
        <p className="mp-kpi-note">
          {t(platformViewer ? "gateways.inventoryIngest.summary.platform" : "gateways.inventoryIngest.summary.tenant", {
            available: availableCount,
            consumed: consumedCount,
            frozen: frozenCount,
            scrapped: scrappedCount,
          })}
        </p>
      </CardContent>
    </Card>
  )
}
