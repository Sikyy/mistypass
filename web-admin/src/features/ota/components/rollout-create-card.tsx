import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery } from "@tanstack/react-query"
import { useMemo, useState } from "react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { listGateways } from "@/lib/api/gateways"
import { listFirmware, type CreateRolloutInput, type RolloutTarget } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"
import { useCreateRolloutMutation } from "../hooks/use-rollouts"
import { buildSchedulePayload, buildingOptions, isHHMM, validatePhases } from "../lib/rollout-utils"

type FormValues = {
  firmware_id: string
  targetKind: "all" | "building" | "gateways"
  building_id?: string
  gateway_ids: string[]
  phases: { percentage: number; requires_approval: boolean }[]
  failure_threshold_pct: number
  startAtLocal?: string
  windowStart?: string
  windowEnd?: string
  timezone?: string
}

export function RolloutCreateCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [serverError, setServerError] = useState("")

  const firmwareQ = useQuery({
    queryKey: queryKeys.firmwareList._key(tenantID ?? "self", "all"),
    queryFn: () => listFirmware(token, tenantID),
    staleTime: 30000,
  })
  const gatewaysQ = useQuery({
    queryKey: ["ota-rollout-gateways", tenantID ?? "self"],
    queryFn: () => listGateways(token),
    staleTime: 30000,
  })
  const gateways = gatewaysQ.data ?? []
  const buildings = useMemo(() => buildingOptions(gateways), [gateways])
  const create = useCreateRolloutMutation(token, tenantID)

  const schema = useMemo(
    () =>
      z
        .object({
          firmware_id: z.string().min(1, t("ota.rollout.create.validation.firmwareRequired")),
          targetKind: z.enum(["all", "building", "gateways"]),
          building_id: z.string().optional(),
          gateway_ids: z.array(z.string()),
          phases: z
            .array(z.object({ percentage: z.coerce.number(), requires_approval: z.boolean() }))
            .refine(validatePhases, t("ota.rollout.create.validation.phasesInvalid")),
          failure_threshold_pct: z.coerce.number().min(0).max(100),
          startAtLocal: z.string().optional(),
          windowStart: z
            .string()
            .optional()
            .refine((v) => !v || isHHMM(v), t("ota.rollout.create.validation.hhmm")),
          windowEnd: z
            .string()
            .optional()
            .refine((v) => !v || isHHMM(v), t("ota.rollout.create.validation.hhmm")),
          timezone: z.string().optional(),
        })
        .refine((d) => d.targetKind !== "building" || !!d.building_id, {
          message: t("ota.rollout.create.validation.buildingRequired"),
          path: ["building_id"],
        })
        .refine((d) => d.targetKind !== "gateways" || d.gateway_ids.length > 0, {
          message: t("ota.rollout.create.validation.gatewaysRequired"),
          path: ["gateway_ids"],
        }),
    [t]
  )

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      firmware_id: "",
      targetKind: "all",
      building_id: "",
      gateway_ids: [],
      phases: [{ percentage: 100, requires_approval: false }],
      failure_threshold_pct: 20,
      startAtLocal: "",
      windowStart: "",
      windowEnd: "",
      timezone: "",
    },
  })
  const phases = useFieldArray({ control: form.control, name: "phases" })
  const kind = form.watch("targetKind")
  const tzOptions = useMemo<string[]>(() => {
    try {
      return (
        (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf?.("timeZone") ?? []
      )
    } catch {
      return []
    }
  }, [])

  function onSubmit(v: FormValues) {
    const target: RolloutTarget =
      v.targetKind === "all"
        ? { kind: "all" }
        : v.targetKind === "building"
          ? { kind: "building", building_id: v.building_id }
          : { kind: "gateways", gateway_ids: v.gateway_ids }
    const input: CreateRolloutInput = {
      firmware_id: v.firmware_id,
      target,
      phases: v.phases,
      failure_threshold_pct: v.failure_threshold_pct,
      schedule: buildSchedulePayload({
        startAtLocal: v.startAtLocal,
        windowStart: v.windowStart,
        windowEnd: v.windowEnd,
        timezone: v.timezone,
      }),
    }
    create.mutate(input, {
      onSuccess: (r) => navigate(`/ota/rollouts/${r.id}`),
      onError: (e) => setServerError(e instanceof Error ? e.message : t("ota.rollout.errors.generic")),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.rollout.create.title")}</CardTitle>
        <CardDescription>{t("ota.rollout.create.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
          {/* Firmware select */}
          <Controller
            control={form.control}
            name="firmware_id"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger>
                  <SelectValue placeholder={t("ota.rollout.create.firmwarePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {(firmwareQ.data ?? []).map((fw) => (
                    <SelectItem key={fw.id} value={fw.id}>
                      {fw.version}
                      {fw.channel ? ` · ${fw.channel}` : ""}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          {form.formState.errors.firmware_id && (
            <p className="text-sm text-destructive">{form.formState.errors.firmware_id.message}</p>
          )}

          {/* Target kind */}
          <Controller
            control={form.control}
            name="targetKind"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("ota.rollout.create.targetAll")}</SelectItem>
                  <SelectItem value="building">{t("ota.rollout.create.targetBuilding")}</SelectItem>
                  <SelectItem value="gateways">{t("ota.rollout.create.targetGateways")}</SelectItem>
                </SelectContent>
              </Select>
            )}
          />

          {/* Building select (conditional) */}
          {kind === "building" && (
            <Controller
              control={form.control}
              name="building_id"
              render={({ field }) => (
                <Select value={field.value ?? ""} onValueChange={field.onChange}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("ota.rollout.create.buildingPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {buildings.map((b) => (
                      <SelectItem key={b} value={b}>
                        {b}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          )}

          {/* Gateway multi-select (conditional) */}
          {kind === "gateways" && (
            <Controller
              control={form.control}
              name="gateway_ids"
              render={({ field }) => (
                <div className="max-h-48 overflow-auto rounded border p-2 space-y-1">
                  {gateways.map((gw) => (
                    <label key={gw.id} className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={field.value.includes(gw.id)}
                        onChange={(e) =>
                          field.onChange(
                            e.target.checked
                              ? [...field.value, gw.id]
                              : field.value.filter((x: string) => x !== gw.id)
                          )
                        }
                      />
                      <span>{gw.id}</span>
                    </label>
                  ))}
                </div>
              )}
            />
          )}

          {(form.formState.errors.building_id || form.formState.errors.gateway_ids) && (
            <p className="text-sm text-destructive">
              {String(
                form.formState.errors.building_id?.message ?? form.formState.errors.gateway_ids?.message
              )}
            </p>
          )}

          {/* Phases */}
          <div className="space-y-2">
            <p className="text-sm font-medium">{t("ota.rollout.create.phases")}</p>
            {phases.fields.map((f, i) => (
              <div key={f.id} className="flex items-center gap-2">
                <Input
                  type="number"
                  className="w-24"
                  {...form.register(`phases.${i}.percentage`, { valueAsNumber: true })}
                />
                <Controller
                  control={form.control}
                  name={`phases.${i}.requires_approval`}
                  render={({ field }) => (
                    <label className="flex items-center gap-2 text-sm">
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                      {t("ota.rollout.create.requiresApproval")}
                    </label>
                  )}
                />
                {phases.fields.length > 1 && (
                  <Button type="button" variant="outline" size="sm" onClick={() => phases.remove(i)}>
                    {t("ota.rollout.create.removePhase")}
                  </Button>
                )}
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => phases.append({ percentage: 100, requires_approval: false })}
            >
              {t("ota.rollout.create.addPhase")}
            </Button>
            {form.formState.errors.phases && (
              <p className="text-sm text-destructive">
                {String(form.formState.errors.phases.message ?? t("ota.rollout.create.validation.phasesInvalid"))}
              </p>
            )}
          </div>

          {/* Failure threshold */}
          <div>
            <label className="text-sm">{t("ota.rollout.create.failureThreshold")}</label>
            <Input
              type="number"
              className="w-24"
              {...form.register("failure_threshold_pct", { valueAsNumber: true })}
            />
          </div>

          {/* Schedule (collapsible) */}
          <details className="rounded border p-2">
            <summary className="text-sm cursor-pointer">{t("ota.rollout.create.scheduleSection")}</summary>
            <div className="mt-2 space-y-2">
              <Input type="datetime-local" {...form.register("startAtLocal")} />
              <div className="flex gap-2">
                <Input placeholder="HH:MM" {...form.register("windowStart")} />
                <Input placeholder="HH:MM" {...form.register("windowEnd")} />
              </div>
              <Controller
                control={form.control}
                name="timezone"
                render={({ field }) => (
                  <Select value={field.value ?? ""} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder={t("ota.rollout.create.timezonePlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      {tzOptions.slice(0, 400).map((tz) => (
                        <SelectItem key={tz} value={tz}>
                          {tz}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {(form.formState.errors.windowStart || form.formState.errors.windowEnd) && (
                <p className="text-sm text-destructive">{t("ota.rollout.create.validation.hhmm")}</p>
              )}
            </div>
          </details>

          {serverError && <p className="text-sm text-destructive">{serverError}</p>}

          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? t("ota.rollout.create.submitting") : t("ota.rollout.create.submit")}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
