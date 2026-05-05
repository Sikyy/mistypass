import { zodResolver } from "@hookform/resolvers/zod"
import { useMemo } from "react"
import { Link } from "react-router"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

type AccessGroupFormEmployee = {
  email: string
  fullName: string
  id: string
}

function buildAccessGroupFormSchema(t: (key: string) => string) {
  return z.object({
    group_name: z
      .string()
      .trim()
      .min(1, t("accessPage.components.groupForm.validation.groupNameRequired"))
      .max(64, t("accessPage.components.groupForm.validation.groupNameMax")),
    group_description: z
      .string()
      .trim()
      .max(128, t("accessPage.components.groupForm.validation.descriptionMax"))
      .optional()
      .or(z.literal("")),
    group_member_query: z
      .string()
      .trim()
      .max(128, t("accessPage.components.groupForm.validation.memberSearchMax"))
      .optional()
      .or(z.literal("")),
  })
}

type AccessGroupFormValues = z.infer<ReturnType<typeof buildAccessGroupFormSchema>>

type AccessGroupFormProps = {
  filteredEmployees: AccessGroupFormEmployee[]
  groupDescription: string
  groupMemberQuery: string
  groupName: string
  isEditing: boolean
  onDescriptionChange: (value: string) => void
  onMemberQueryChange: (value: string) => void
  onNameChange: (value: string) => void
  onSubmit: (payload: { name: string; description: string }) => void
  onToggleMember: (employeeID: string) => void
  selectedMemberIDs: string[]
}

export function AccessGroupForm({
  filteredEmployees,
  groupDescription,
  groupMemberQuery,
  groupName,
  isEditing,
  onDescriptionChange,
  onMemberQueryChange,
  onNameChange,
  onSubmit,
  onToggleMember,
  selectedMemberIDs,
}: AccessGroupFormProps) {
  const { t } = useTranslation()
  const accessGroupFormSchema = useMemo(() => buildAccessGroupFormSchema(t), [t])

  const groupForm = useForm<AccessGroupFormValues>({
    resolver: zodResolver(accessGroupFormSchema),
    values: {
      group_name: groupName,
      group_description: groupDescription,
      group_member_query: groupMemberQuery,
    },
  })
  const groupNameField = groupForm.register("group_name")
  const groupDescriptionField = groupForm.register("group_description")
  const groupMemberQueryField = groupForm.register("group_member_query")
  const groupFormError =
    groupForm.formState.errors.group_name?.message ||
    groupForm.formState.errors.group_description?.message ||
    groupForm.formState.errors.group_member_query?.message ||
    ""

  function onSubmitGroupForm(values: AccessGroupFormValues) {
    onSubmit({
      name: values.group_name.trim(),
      description: values.group_description || "",
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {isEditing
            ? t("accessPage.components.groupForm.titleEdit")
            : t("accessPage.components.groupForm.titleCreate")}
        </CardTitle>
        <CardDescription>
          {t("accessPage.components.groupForm.description")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-3" onSubmit={groupForm.handleSubmit(onSubmitGroupForm)}>
          <Input
            {...groupNameField}
            onChange={(event) => {
              groupNameField.onChange(event)
              onNameChange(event.target.value)
            }}
            placeholder={t("accessPage.components.groupForm.groupName")}
          />
          <Input
            {...groupDescriptionField}
            onChange={(event) => {
              groupDescriptionField.onChange(event)
              onDescriptionChange(event.target.value)
            }}
            placeholder={t("accessPage.components.groupForm.descriptionInput")}
          />
          <Input
            {...groupMemberQueryField}
            onChange={(event) => {
              groupMemberQueryField.onChange(event)
              onMemberQueryChange(event.target.value)
            }}
            placeholder={t("accessPage.components.groupForm.memberSearch")}
          />
          <div className="max-h-48 space-y-1 overflow-auto rounded-md border bg-muted/20 p-2">
            {filteredEmployees.length === 0 ? (
              <div className="space-y-2 px-2 py-3">
                <p className="mp-kpi-note">
                  {t("accessPage.components.groupForm.emptyEmployees")}
                </p>
                <Button asChild variant="outline" size="sm">
                  <Link to="/enterprise">{t("accessPage.components.groupForm.importEmployees")}</Link>
                </Button>
              </div>
            ) : null}
            {filteredEmployees.map((employee) => (
              <label
                key={employee.id}
                className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/40"
              >
                <input
                  type="checkbox"
                  checked={selectedMemberIDs.includes(employee.id)}
                  onChange={() => onToggleMember(employee.id)}
                />
                <span className="min-w-0 text-sm">
                  <span className="font-medium">{employee.fullName}</span>
                  <span className="ml-1 text-xs text-muted-foreground">({employee.email})</span>
                </span>
              </label>
            ))}
          </div>
          <p className="mp-kpi-note">
            {t("accessPage.components.groupForm.selectedMembers", {
              count: selectedMemberIDs.length,
            })}
          </p>
          <Button type="submit" className="w-full" disabled={groupForm.formState.isSubmitting}>
            {isEditing
              ? t("accessPage.components.groupForm.submitEdit")
              : t("accessPage.components.groupForm.submitCreate")}
          </Button>
          {groupFormError ? <p className="text-sm text-destructive">{groupFormError}</p> : null}
        </form>
      </CardContent>
    </Card>
  )
}
