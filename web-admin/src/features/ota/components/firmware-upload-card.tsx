import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useMemo, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { uploadFirmware } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"
import { computeSha256Hex, isCryptoSubtleAvailable, isSignatureHex } from "../lib/firmware-utils"

type UploadFormValues = { version: string; channel?: string; signature: string; file: FileList }

export function FirmwareUploadCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [serverError, setServerError] = useState("")
  const cryptoOk = isCryptoSubtleAvailable()

  const schema = useMemo(
    () =>
      z.object({
        version: z.string().trim().min(1, t("ota.firmware.upload.validation.versionRequired")).max(64, t("ota.firmware.upload.validation.versionMax")),
        channel: z.string().trim().max(32).optional().or(z.literal("")),
        signature: z.string().trim().refine(isSignatureHex, t("ota.firmware.upload.validation.signatureHex")),
        file: z.custom<FileList>((v) => v instanceof FileList && v.length > 0, t("ota.firmware.upload.validation.fileRequired")),
      }),
    [t]
  )
  const form = useForm<UploadFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { version: "", channel: "", signature: "", file: undefined as unknown as FileList },
  })

  const mutation = useMutation({
    mutationFn: async (values: UploadFormValues) => {
      const file = values.file[0]
      const sha256 = await computeSha256Hex(file)
      return uploadFirmware(token, tenantID, {
        version: values.version.trim(),
        channel: values.channel?.trim() || undefined,
        sha256,
        signature: values.signature.trim(),
        file,
      })
    },
    onSuccess: () => {
      setServerError("")
      form.reset()
      void queryClient.invalidateQueries({ queryKey: queryKeys.firmwareList._base })
      void queryClient.invalidateQueries({ queryKey: queryKeys.firmwareSummary._base })
    },
    onError: (err) => setServerError(err instanceof Error ? err.message : t("ota.firmware.errors.generic")),
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.firmware.upload.title")}</CardTitle>
        <CardDescription>{t("ota.firmware.upload.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {!cryptoOk ? (
          <p className="text-sm text-destructive">{t("ota.firmware.upload.cryptoUnavailable")}</p>
        ) : (
          <form className="space-y-3" onSubmit={form.handleSubmit((v) => mutation.mutate(v))}>
            <div>
              <Input placeholder={t("ota.firmware.upload.versionPlaceholder")} {...form.register("version")} />
              {form.formState.errors.version && <p className="text-sm text-destructive">{form.formState.errors.version.message}</p>}
            </div>
            <Input placeholder={t("ota.firmware.upload.channelPlaceholder")} {...form.register("channel")} />
            <div>
              <Textarea rows={3} placeholder={t("ota.firmware.upload.signaturePlaceholder")} {...form.register("signature")} />
              {form.formState.errors.signature && <p className="text-sm text-destructive">{form.formState.errors.signature.message}</p>}
            </div>
            <div>
              <Controller
                control={form.control}
                name="file"
                render={({ field }) => (
                  <Input type="file" onChange={(e) => field.onChange(e.target.files ?? undefined)} />
                )}
              />
              {form.formState.errors.file && <p className="text-sm text-destructive">{String(form.formState.errors.file.message)}</p>}
            </div>
            {serverError && <p className="text-sm text-destructive">{serverError}</p>}
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("ota.firmware.upload.submitting") : t("ota.firmware.upload.submit")}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  )
}
