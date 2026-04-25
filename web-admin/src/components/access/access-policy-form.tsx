import { zodResolver } from "@hookform/resolvers/zod"
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
const accessPolicyFormSchema = z
  .object({
    policy_name: z
      .string()
      .trim()
      .min(1, "Please enter a policy name")
      .max(64, "Policy name must be at most 64 characters"),
    policy_scope_type: z.enum(accessPolicyScopeTypeValues),
    policy_status: z.enum(accessPolicyStatusValues),
    policy_building_id: z
      .string()
      .trim()
      .max(64, "Building ID must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    policy_area_id: z
      .string()
      .trim()
      .max(64, "Area ID must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    policy_door_id: z
      .string()
      .trim()
      .max(64, "Door ID must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    policy_schedule: z
      .string()
      .trim()
      .max(128, "Schedule must be at most 128 characters")
      .optional()
      .or(z.literal("")),
    policy_members: z
      .string()
      .trim()
      .min(1, "Please enter member count")
      .max(10, "Invalid member count format")
      .refine((value) => {
        const parsed = Number.parseInt(value, 10)
        return Number.isFinite(parsed) && parsed >= 0
      }, "Member count must be an integer greater than or equal to 0"),
  })
  .superRefine((values, context) => {
    if (values.policy_scope_type !== "all" && !values.policy_building_id?.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["policy_building_id"],
        message: "Building is required for this scope type",
      })
    }
    if ((values.policy_scope_type === "area" || values.policy_scope_type === "door") && !values.policy_area_id?.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["policy_area_id"],
        message: "Area is required for this scope type",
      })
    }
    if (values.policy_scope_type === "door" && !values.policy_door_id?.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["policy_door_id"],
        message: "Door scope requires a selected door",
      })
    }
  })

type AccessPolicyFormValues = z.infer<typeof accessPolicyFormSchema>

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
            ? t("accessPage.components.policyForm.titleEdit", { defaultValue: "Edit policy" })
            : t("accessPage.components.policyForm.titleCreate", { defaultValue: "Create policy" })}
        </CardTitle>
        <CardDescription>
          {t("accessPage.components.policyForm.description", {
            defaultValue: "Keep access rules as an independent layer scoped to building/area/door.",
          })}
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
            placeholder={t("accessPage.components.policyForm.policyName", { defaultValue: "Policy name" })}
          />

          <div className="grid grid-cols-2 gap-2">
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
                  <SelectTrigger>
                    <SelectValue placeholder={t("accessPage.components.policyForm.scopeType", { defaultValue: "Scope type" })} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("accessPage.components.policyForm.scopeAll", { defaultValue: "All areas" })}</SelectItem>
                    <SelectItem value="building">{t("accessPage.components.policyForm.scopeBuilding", { defaultValue: "Building" })}</SelectItem>
                    <SelectItem value="area">{t("accessPage.components.policyForm.scopeArea", { defaultValue: "Area" })}</SelectItem>
                    <SelectItem value="door">{t("accessPage.components.policyForm.scopeDoor", { defaultValue: "Door" })}</SelectItem>
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
                  <SelectTrigger>
                    <SelectValue placeholder={t("accessPage.components.policyForm.status", { defaultValue: "Status" })} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">{t("accessPage.components.policyForm.statusActive", { defaultValue: "Active" })}</SelectItem>
                    <SelectItem value="inactive">{t("accessPage.components.policyForm.statusInactive", { defaultValue: "Inactive" })}</SelectItem>
                    <SelectItem value="draft">{t("accessPage.components.policyForm.statusDraft", { defaultValue: "Draft" })}</SelectItem>
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
                <SelectTrigger disabled={scopeType === "all"}>
                  <SelectValue placeholder={t("accessPage.components.policyForm.buildingOptional", { defaultValue: "Building (optional)" })} />
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
                <SelectTrigger disabled={scopeType === "all" || scopeType === "building" || !buildingID}>
                  <SelectValue placeholder={t("accessPage.components.policyForm.areaOptional", { defaultValue: "Area (optional)" })} />
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
                <SelectTrigger disabled={scopeType !== "door" || !areaID}>
                  <SelectValue placeholder={t("accessPage.components.policyForm.doorOptional", { defaultValue: "Door (optional)" })} />
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
              defaultValue: "Current scope: {{scopeSummaryLabel}}",
              scopeSummaryLabel,
            })}
          </div>

          <Input
            {...policyScheduleField}
            onChange={(event) => {
              policyScheduleField.onChange(event)
              onScheduleChange(event.target.value)
            }}
            placeholder={t("accessPage.components.policyForm.schedule", {
              defaultValue: "Schedule (e.g. Mon-Fri 07:00-19:00)",
            })}
          />
          <Input
            {...policyMembersField}
            onChange={(event) => {
              policyMembersField.onChange(event)
              onMembersChange(event.target.value)
            }}
            placeholder={t("accessPage.components.policyForm.memberCount", { defaultValue: "Member count" })}
          />
          <Button type="submit" className="w-full" disabled={policyForm.formState.isSubmitting}>
            {isEditing
              ? t("accessPage.components.policyForm.submitEdit", { defaultValue: "Update policy" })
              : t("accessPage.components.policyForm.submitCreate", { defaultValue: "Create policy" })}
          </Button>
          {policyFormError ? <p className="text-sm text-destructive">{policyFormError}</p> : null}
        </form>
      </CardContent>
    </Card>
  )
}
