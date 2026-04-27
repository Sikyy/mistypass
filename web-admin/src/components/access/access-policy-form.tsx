import { zodResolver } from "@hookform/resolvers/zod"
import { useMemo } from "react"
import { Controller, useForm } from "react-hook-form"
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

type AccessPolicyScopeType = "all" | "building" | "area" | "door"
type AccessPolicyStatus = "active" | "inactive" | "draft"
type AccessPolicyOption = {
  id: string
  name: string
}

const accessPolicyScopeTypeValues = ["all", "building", "area", "door"] as const
const accessPolicyStatusValues = ["active", "inactive", "draft"] as const
function buildAccessPolicyFormSchema(t: (key: string) => string) {
  return z
    .object({
      policy_name: z
        .string()
        .trim()
        .min(1, t("accessPage.components.policyForm.validation.policyNameRequired"))
        .max(64, t("accessPage.components.policyForm.validation.policyNameMax")),
      policy_scope_type: z.enum(accessPolicyScopeTypeValues),
      policy_status: z.enum(accessPolicyStatusValues),
      policy_building_id: z
        .string()
        .trim()
        .max(64, t("accessPage.components.policyForm.validation.buildingIdMax"))
        .optional()
        .or(z.literal("")),
      policy_area_id: z
        .string()
        .trim()
        .max(64, t("accessPage.components.policyForm.validation.areaIdMax"))
        .optional()
        .or(z.literal("")),
      policy_door_id: z
        .string()
        .trim()
        .max(64, t("accessPage.components.policyForm.validation.doorIdMax"))
        .optional()
        .or(z.literal("")),
      policy_schedule: z
        .string()
        .trim()
        .max(128, t("accessPage.components.policyForm.validation.scheduleMax"))
        .optional()
        .or(z.literal("")),
      policy_members: z
        .string()
        .trim()
        .min(1, t("accessPage.components.policyForm.validation.memberCountRequired"))
        .max(10, t("accessPage.components.policyForm.validation.memberCountMax"))
        .refine((value) => {
          const parsed = Number.parseInt(value, 10)
          return Number.isFinite(parsed) && parsed >= 0
        }, t("accessPage.components.policyForm.validation.memberCountInteger")),
    })
    .superRefine((values, context) => {
      if (values.policy_scope_type !== "all" && !values.policy_building_id?.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["policy_building_id"],
          message: t("accessPage.components.policyForm.validation.buildingRequired"),
        })
      }
      if ((values.policy_scope_type === "area" || values.policy_scope_type === "door") && !values.policy_area_id?.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["policy_area_id"],
          message: t("accessPage.components.policyForm.validation.areaRequired"),
        })
      }
      if (values.policy_scope_type === "door" && !values.policy_door_id?.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["policy_door_id"],
          message: t("accessPage.components.policyForm.validation.doorRequired"),
        })
      }
    })
}

type AccessPolicyFormValues = z.infer<ReturnType<typeof buildAccessPolicyFormSchema>>

type AccessPolicyFormProps = {
  areaID: string
  areaOptions: AccessPolicyOption[]
  buildingID: string
  buildingOptions: AccessPolicyOption[]
  doorID: string
  doorOptions: AccessPolicyOption[]
  isEditing: boolean
  members: string
  name: string
  onAreaIDChange: (value: string) => void
  onBuildingIDChange: (value: string) => void
  onDoorIDChange: (value: string) => void
  onMembersChange: (value: string) => void
  onNameChange: (value: string) => void
  onScheduleChange: (value: string) => void
  onScopeTypeChange: (value: AccessPolicyScopeType) => void
  onStatusChange: (value: AccessPolicyStatus) => void
  onSubmit: (payload: {
    name: string
    scopeType: AccessPolicyScopeType
    status: AccessPolicyStatus
    buildingID: string
    areaID: string
    doorID: string
    schedule: string
    members: string
  }) => void
  schedule: string
  scopeSummaryLabel: string
  scopeType: AccessPolicyScopeType
  status: AccessPolicyStatus
}

export function AccessPolicyForm({
  areaID,
  areaOptions,
  buildingID,
  buildingOptions,
  doorID,
  doorOptions,
  isEditing,
  members,
  name,
  onAreaIDChange,
  onBuildingIDChange,
  onDoorIDChange,
  onMembersChange,
  onNameChange,
  onScheduleChange,
  onScopeTypeChange,
  onStatusChange,
  onSubmit,
  schedule,
  scopeSummaryLabel,
  scopeType,
  status,
}: AccessPolicyFormProps) {
  const { t } = useTranslation()
  const accessPolicyFormSchema = useMemo(() => buildAccessPolicyFormSchema(t), [t])
  const policyForm = useForm<AccessPolicyFormValues>({
    resolver: zodResolver(accessPolicyFormSchema),
    values: {
      policy_name: name,
      policy_scope_type: scopeType,
      policy_status: status,
      policy_building_id: buildingID,
      policy_area_id: areaID,
      policy_door_id: doorID,
      policy_schedule: schedule,
      policy_members: members,
    },
  })
  const policyNameField = policyForm.register("policy_name")
  const policyScheduleField = policyForm.register("policy_schedule")
  const policyMembersField = policyForm.register("policy_members")
  const policyFormError =
    policyForm.formState.errors.policy_name?.message ||
    policyForm.formState.errors.policy_scope_type?.message ||
    policyForm.formState.errors.policy_status?.message ||
    policyForm.formState.errors.policy_building_id?.message ||
    policyForm.formState.errors.policy_area_id?.message ||
    policyForm.formState.errors.policy_door_id?.message ||
    policyForm.formState.errors.policy_schedule?.message ||
    policyForm.formState.errors.policy_members?.message ||
    ""
  const buildingDisabledReason =
    scopeType === "all" ? t("accessPage.components.policyForm.disabledReason.buildingAllScope") : undefined
  const areaDisabledReason =
    scopeType === "all"
      ? t("accessPage.components.policyForm.disabledReason.areaAllScope")
      : scopeType === "building"
        ? t("accessPage.components.policyForm.disabledReason.areaBuildingScope")
        : !buildingID
          ? t("accessPage.components.policyForm.disabledReason.areaNeedsBuilding")
          : undefined
  const doorDisabledReason =
    scopeType !== "door"
      ? t("accessPage.components.policyForm.disabledReason.doorScopeOnly")
      : !areaID
        ? t("accessPage.components.policyForm.disabledReason.doorNeedsArea")
        : undefined

  function onSubmitPolicyForm(values: AccessPolicyFormValues) {
    onSubmit({
      name: values.policy_name.trim(),
      scopeType: values.policy_scope_type,
      status: values.policy_status,
      buildingID: values.policy_building_id || "",
      areaID: values.policy_area_id || "",
      doorID: values.policy_door_id || "",
      schedule: values.policy_schedule || "",
      members: values.policy_members,
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {isEditing
            ? t("accessPage.components.policyForm.titleEdit")
            : t("accessPage.components.policyForm.titleCreate")}
        </CardTitle>
        <CardDescription>
          {t("accessPage.components.policyForm.description")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-3" onSubmit={policyForm.handleSubmit(onSubmitPolicyForm)}>
          <Input
            {...policyNameField}
            onChange={(event) => {
              policyNameField.onChange(event)
              onNameChange(event.target.value)
            }}
            placeholder={t("accessPage.components.policyForm.policyName")}
          />

          <div className="grid gap-2 sm:grid-cols-2">
            <Controller
              control={policyForm.control}
              name="policy_scope_type"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value: AccessPolicyScopeType) => {
                    field.onChange(value)
                    onScopeTypeChange(value)
                  }}
                >
                  <SelectTrigger className="w-full min-w-0">
                    <SelectValue placeholder={t("accessPage.components.policyForm.scopeType")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("accessPage.components.policyForm.scopeAll")}</SelectItem>
                    <SelectItem value="building">{t("accessPage.components.policyForm.scopeBuilding")}</SelectItem>
                    <SelectItem value="area">{t("accessPage.components.policyForm.scopeArea")}</SelectItem>
                    <SelectItem value="door">{t("accessPage.components.policyForm.scopeDoor")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
            <Controller
              control={policyForm.control}
              name="policy_status"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value: AccessPolicyStatus) => {
                    field.onChange(value)
                    onStatusChange(value)
                  }}
                >
                  <SelectTrigger className="w-full min-w-0">
                    <SelectValue placeholder={t("accessPage.components.policyForm.status")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">{t("accessPage.components.policyForm.statusActive")}</SelectItem>
                    <SelectItem value="inactive">{t("accessPage.components.policyForm.statusInactive")}</SelectItem>
                    <SelectItem value="draft">{t("accessPage.components.policyForm.statusDraft")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
          </div>

          <Controller
            control={policyForm.control}
            name="policy_building_id"
            render={({ field }) => (
              <Select
                value={field.value || ""}
                onValueChange={(value) => {
                  field.onChange(value)
                  onBuildingIDChange(value)
                }}
              >
                <SelectTrigger
                  className="w-full min-w-0"
                  disabled={scopeType === "all"}
                  title={buildingDisabledReason}
                >
                  <SelectValue placeholder={t("accessPage.components.policyForm.buildingOptional")} />
                </SelectTrigger>
                <SelectContent>
                  {buildingOptions.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />

          <Controller
            control={policyForm.control}
            name="policy_area_id"
            render={({ field }) => (
              <Select
                value={field.value || ""}
                onValueChange={(value) => {
                  field.onChange(value)
                  onAreaIDChange(value)
                }}
              >
                <SelectTrigger
                  className="w-full min-w-0"
                  disabled={scopeType === "all" || scopeType === "building" || !buildingID}
                  title={areaDisabledReason}
                >
                  <SelectValue placeholder={t("accessPage.components.policyForm.areaOptional")} />
                </SelectTrigger>
                <SelectContent>
                  {areaOptions.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />

          <Controller
            control={policyForm.control}
            name="policy_door_id"
            render={({ field }) => (
              <Select
                value={field.value || ""}
                onValueChange={(value) => {
                  field.onChange(value)
                  onDoorIDChange(value)
                }}
              >
                <SelectTrigger
                  className="w-full min-w-0"
                  disabled={scopeType !== "door" || !areaID}
                  title={doorDisabledReason}
                >
                  <SelectValue placeholder={t("accessPage.components.policyForm.doorOptional")} />
                </SelectTrigger>
                <SelectContent>
                  {doorOptions.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name} ({item.id})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />

          <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
            {t("accessPage.components.policyForm.currentScope", {
              scopeSummaryLabel,
            })}
          </div>

          <Input
            {...policyScheduleField}
            onChange={(event) => {
              policyScheduleField.onChange(event)
              onScheduleChange(event.target.value)
            }}
            placeholder={t("accessPage.components.policyForm.schedule")}
          />
          <Input
            {...policyMembersField}
            onChange={(event) => {
              policyMembersField.onChange(event)
              onMembersChange(event.target.value)
            }}
            placeholder={t("accessPage.components.policyForm.memberCount")}
          />
          <Button type="submit" className="w-full" disabled={policyForm.formState.isSubmitting}>
            {isEditing
              ? t("accessPage.components.policyForm.submitEdit")
              : t("accessPage.components.policyForm.submitCreate")}
          </Button>
          {policyFormError ? <p className="text-sm text-destructive">{policyFormError}</p> : null}
        </form>
      </CardContent>
    </Card>
  )
}
