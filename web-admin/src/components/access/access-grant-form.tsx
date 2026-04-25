import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { type Area, type Building, type Door } from "@/lib/api"

type ScopeType = "all" | "building" | "area" | "door"
type DeliveryMethod = "email_qr" | "wallet"

const accessGrantScopeTypeValues = ["all", "building", "area", "door"] as const
const accessGrantDeliveryMethodValues = ["email_qr", "wallet"] as const
const accessGrantFormSchema = z
  .object({
    grant_scope_type: z.enum(accessGrantScopeTypeValues),
    grant_delivery_method: z.enum(accessGrantDeliveryMethodValues),
    grant_building_id: z
      .string()
      .trim()
      .max(64, "Building ID must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    grant_area_id: z
      .string()
      .trim()
      .max(64, "Area ID must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    grant_door_id: z
      .string()
      .trim()
      .max(64, "Door ID must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    grant_grantee_name: z
      .string()
      .trim()
      .min(1, "Please enter grantee name")
      .max(64, "Grantee name must be at most 64 characters"),
    grant_grantee_gender: z
      .string()
      .trim()
      .max(16, "Gender must be at most 16 characters")
      .optional()
      .or(z.literal("")),
    grant_grantee_phone: z
      .string()
      .trim()
      .min(1, "Please enter phone number")
      .max(32, "Phone number must be at most 32 characters"),
    grant_grantee_email: z
      .string()
      .trim()
      .min(1, "Please enter email")
      .email("Invalid email format"),
    grant_mobile_model: z
      .string()
      .trim()
      .max(64, "Mobile model must be at most 64 characters")
      .optional()
      .or(z.literal("")),
    grant_pass_type: z
      .string()
      .trim()
      .min(1, "Please enter subject type")
      .max(32, "Subject type must be at most 32 characters"),
    grant_valid_until: z
      .string()
      .trim()
      .min(1, "Please enter valid-until time")
      .refine((value) => !Number.isNaN(new Date(value).getTime()), "Invalid valid-until time format"),
  })
  .superRefine((values, context) => {
    if (values.grant_scope_type !== "all" && !values.grant_building_id?.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["grant_building_id"],
        message: "Building is required for this scope type",
      })
    }
    if ((values.grant_scope_type === "area" || values.grant_scope_type === "door") && !values.grant_area_id?.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["grant_area_id"],
        message: "Area is required for this scope type",
      })
    }
    if (values.grant_scope_type === "door" && !values.grant_door_id?.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["grant_door_id"],
        message: "Door scope requires a selected door",
      })
    }
  })

type AccessGrantFormValues = z.infer<typeof accessGrantFormSchema>

type AccessGrantFormProps = {
  onSubmit: (payload: {
    scopeType: ScopeType
    deliveryMethod: DeliveryMethod
    buildingID: string
    areaID: string
    doorID: string
    granteeName: string
    granteeGender: string
    granteePhone: string
    granteeEmail: string
    mobileModel: string
    passType: string
    validUntil: string
  }) => void
  scopeType: ScopeType
  onScopeTypeChange: (value: ScopeType) => void
  deliveryMethod: DeliveryMethod
  onDeliveryMethodChange: (value: DeliveryMethod) => void
  buildingID: string
  onBuildingChange: (value: string) => void
  areaID: string
  onAreaChange: (value: string) => void
  doorID: string
  onDoorChange: (value: string) => void
  buildings: Building[]
  areaOptions: Area[]
  doorOptions: Door[]
  scopeSummaryLabel: string
  granteeName: string
  onGranteeNameChange: (value: string) => void
  granteeGender: string
  onGranteeGenderChange: (value: string) => void
  granteePhone: string
  onGranteePhoneChange: (value: string) => void
  granteeEmail: string
  onGranteeEmailChange: (value: string) => void
  mobileModel: string
  onMobileModelChange: (value: string) => void
  passType: string
  onPassTypeChange: (value: string) => void
  validUntil: string
  onValidUntilChange: (value: string) => void
}

export function AccessGrantForm({
  onSubmit,
  scopeType,
  onScopeTypeChange,
  deliveryMethod,
  onDeliveryMethodChange,
  buildingID,
  onBuildingChange,
  areaID,
  onAreaChange,
  doorID,
  onDoorChange,
  buildings,
  areaOptions,
  doorOptions,
  scopeSummaryLabel,
  granteeName,
  onGranteeNameChange,
  granteeGender,
  onGranteeGenderChange,
  granteePhone,
  onGranteePhoneChange,
  granteeEmail,
  onGranteeEmailChange,
  mobileModel,
  onMobileModelChange,
  passType,
  onPassTypeChange,
  validUntil,
  onValidUntilChange,
}: AccessGrantFormProps) {
  const { t } = useTranslation()
  const grantForm = useForm<AccessGrantFormValues>({
    resolver: zodResolver(accessGrantFormSchema),
    values: {
      grant_scope_type: scopeType,
      grant_delivery_method: deliveryMethod,
      grant_building_id: buildingID,
      grant_area_id: areaID,
      grant_door_id: doorID,
      grant_grantee_name: granteeName,
      grant_grantee_gender: granteeGender,
      grant_grantee_phone: granteePhone,
      grant_grantee_email: granteeEmail,
      grant_mobile_model: mobileModel,
      grant_pass_type: passType,
      grant_valid_until: validUntil,
    },
  })
  const granteeNameField = grantForm.register("grant_grantee_name")
  const granteeGenderField = grantForm.register("grant_grantee_gender")
  const granteePhoneField = grantForm.register("grant_grantee_phone")
  const granteeEmailField = grantForm.register("grant_grantee_email")
  const mobileModelField = grantForm.register("grant_mobile_model")
  const passTypeField = grantForm.register("grant_pass_type")
  const validUntilField = grantForm.register("grant_valid_until")
  const grantFormError =
    grantForm.formState.errors.grant_scope_type?.message ||
    grantForm.formState.errors.grant_delivery_method?.message ||
    grantForm.formState.errors.grant_building_id?.message ||
    grantForm.formState.errors.grant_area_id?.message ||
    grantForm.formState.errors.grant_door_id?.message ||
    grantForm.formState.errors.grant_grantee_name?.message ||
    grantForm.formState.errors.grant_grantee_gender?.message ||
    grantForm.formState.errors.grant_grantee_phone?.message ||
    grantForm.formState.errors.grant_grantee_email?.message ||
    grantForm.formState.errors.grant_mobile_model?.message ||
    grantForm.formState.errors.grant_pass_type?.message ||
    grantForm.formState.errors.grant_valid_until?.message ||
    ""

  function onSubmitGrantForm(values: AccessGrantFormValues) {
    onSubmit({
      scopeType: values.grant_scope_type,
      deliveryMethod: values.grant_delivery_method,
      buildingID: values.grant_building_id || "",
      areaID: values.grant_area_id || "",
      doorID: values.grant_door_id || "",
      granteeName: values.grant_grantee_name.trim(),
      granteeGender: values.grant_grantee_gender || "",
      granteePhone: values.grant_grantee_phone.trim(),
      granteeEmail: values.grant_grantee_email.trim(),
      mobileModel: values.grant_mobile_model || "",
      passType: values.grant_pass_type.trim(),
      validUntil: values.grant_valid_until.trim(),
    })
  }

  return (
    <form className="space-y-3" onSubmit={grantForm.handleSubmit(onSubmitGrantForm)}>
      <div className="grid grid-cols-2 gap-2">
        <Controller
          control={grantForm.control}
          name="grant_scope_type"
          render={({ field }) => (
            <Select
              value={field.value}
              onValueChange={(value: ScopeType) => {
                field.onChange(value)
                onScopeTypeChange(value)
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("accessPage.components.grantForm.scope", { defaultValue: "Grant scope" })} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("accessPage.components.grantForm.scopeAll", { defaultValue: "All areas" })}</SelectItem>
                <SelectItem value="building">{t("accessPage.components.grantForm.scopeBuilding", { defaultValue: "Building" })}</SelectItem>
                <SelectItem value="area">{t("accessPage.components.grantForm.scopeArea", { defaultValue: "Area" })}</SelectItem>
                <SelectItem value="door">{t("accessPage.components.grantForm.scopeDoor", { defaultValue: "Door" })}</SelectItem>
              </SelectContent>
            </Select>
          )}
        />
        <Controller
          control={grantForm.control}
          name="grant_delivery_method"
          render={({ field }) => (
            <Select
              value={field.value}
              onValueChange={(value: DeliveryMethod) => {
                field.onChange(value)
                onDeliveryMethodChange(value)
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("accessPage.components.grantForm.deliveryMethod", { defaultValue: "Delivery method" })} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="wallet">
                  {t("accessPage.components.grantForm.deliveryWallet", { defaultValue: "MistyPass mobile pass" })}
                </SelectItem>
                <SelectItem value="email_qr">
                  {t("accessPage.components.grantForm.deliveryEmailQr", { defaultValue: "Email QR pass" })}
                </SelectItem>
              </SelectContent>
            </Select>
          )}
        />
      </div>

      <Controller
        control={grantForm.control}
        name="grant_building_id"
        render={({ field }) => (
          <Select
            value={field.value || ""}
            onValueChange={(value) => {
              field.onChange(value)
              onBuildingChange(value)
            }}
          >
            <SelectTrigger disabled={scopeType === "all"}>
              <SelectValue placeholder={t("accessPage.components.grantForm.buildingOptional", { defaultValue: "Building (optional)" })} />
            </SelectTrigger>
            <SelectContent>
              {buildings.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      />
      <Controller
        control={grantForm.control}
        name="grant_area_id"
        render={({ field }) => (
          <Select
            value={field.value || ""}
            onValueChange={(value) => {
              field.onChange(value)
              onAreaChange(value)
            }}
          >
            <SelectTrigger disabled={scopeType === "all" || scopeType === "building" || !buildingID}>
              <SelectValue placeholder={t("accessPage.components.grantForm.areaOptional", { defaultValue: "Area (optional)" })} />
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
        control={grantForm.control}
        name="grant_door_id"
        render={({ field }) => (
          <Select
            value={field.value || ""}
            onValueChange={(value) => {
              field.onChange(value)
              onDoorChange(value)
            }}
          >
            <SelectTrigger disabled={scopeType !== "door" || !areaID}>
              <SelectValue placeholder={t("accessPage.components.grantForm.doorOptional", { defaultValue: "Door (optional)" })} />
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
        {t("accessPage.components.grantForm.currentScope", {
          defaultValue: "Current scope: {{scopeSummaryLabel}}",
          scopeSummaryLabel,
        })}
      </div>

      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1.5">
          <Label>{t("accessPage.components.grantForm.nameLabel", { defaultValue: "Name" })}</Label>
          <Input
            {...granteeNameField}
            onChange={(event) => {
              granteeNameField.onChange(event)
              onGranteeNameChange(event.target.value)
            }}
            placeholder={t("accessPage.components.grantForm.namePlaceholder", { defaultValue: "Grantee name" })}
          />
        </div>
        <div className="space-y-1.5">
          <Label>{t("accessPage.components.grantForm.genderLabel", { defaultValue: "Gender" })}</Label>
          <Input
            {...granteeGenderField}
            onChange={(event) => {
              granteeGenderField.onChange(event)
              onGranteeGenderChange(event.target.value)
            }}
            placeholder="male/female/other"
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1.5">
          <Label>{t("accessPage.components.grantForm.phoneLabel", { defaultValue: "Phone" })}</Label>
          <Input
            {...granteePhoneField}
            onChange={(event) => {
              granteePhoneField.onChange(event)
              onGranteePhoneChange(event.target.value)
            }}
            placeholder="+62-xxx-xxxx-xxxx"
          />
        </div>
        <div className="space-y-1.5">
          <Label>{t("accessPage.components.grantForm.emailLabel", { defaultValue: "Email" })}</Label>
          <Input
            {...granteeEmailField}
            onChange={(event) => {
              granteeEmailField.onChange(event)
              onGranteeEmailChange(event.target.value)
            }}
            placeholder="name@company.com"
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1.5">
          <Label>{t("accessPage.components.grantForm.mobileModelLabel", { defaultValue: "Mobile model" })}</Label>
          <Input
            {...mobileModelField}
            onChange={(event) => {
              mobileModelField.onChange(event)
              onMobileModelChange(event.target.value)
            }}
            placeholder="Pixel 8 / iPhone 16"
          />
        </div>
        <div className="space-y-1.5">
          <Label>{t("accessPage.components.grantForm.subjectTypeLabel", { defaultValue: "Subject type" })}</Label>
          <Input
            {...passTypeField}
            onChange={(event) => {
              passTypeField.onChange(event)
              onPassTypeChange(event.target.value)
            }}
            placeholder="employee / visitor / customer"
          />
        </div>
      </div>
      <Input
        {...validUntilField}
        type="datetime-local"
        onChange={(event) => {
          validUntilField.onChange(event)
          onValidUntilChange(event.target.value)
        }}
        placeholder={t("accessPage.components.grantForm.validUntilPlaceholder", {
          defaultValue: "Valid until (e.g. 2026-04-11 20:00)",
        })}
      />
      <Button type="submit" className="w-full" disabled={grantForm.formState.isSubmitting}>
        {t("accessPage.components.grantForm.submit", { defaultValue: "Create grant" })}
      </Button>
      {grantFormError ? <p className="text-sm text-destructive">{grantFormError}</p> : null}
    </form>
  )
}
