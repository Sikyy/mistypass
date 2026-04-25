import { useTranslation } from "react-i18next"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { type TemporaryAccess } from "@/lib/api"

type AccessGrantDetailDialogProps = {
  grant: TemporaryAccess | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AccessGrantDetailDialog({
  grant,
  open,
  onOpenChange,
}: AccessGrantDetailDialogProps) {
  const { t, i18n } = useTranslation()

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen)
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("accessPage.components.grantDetailDialog.title", { defaultValue: "Grantee profile" })}</DialogTitle>
          <DialogDescription>
            {t("accessPage.components.grantDetailDialog.description", {
              defaultValue: "Used to verify identity, contact details, and device info for granted subject.",
            })}
          </DialogDescription>
        </DialogHeader>
        {grant ? (
          <div className="grid gap-2 text-sm">
            <div>
              <span className="text-muted-foreground">{t("accessPage.components.grantDetailDialog.labels.grantID", { defaultValue: "Grant ID:" })}</span>
              <span className="font-medium">{grant.id}</span>
            </div>
            <div>
              <span className="text-muted-foreground">{t("accessPage.components.grantDetailDialog.labels.name", { defaultValue: "Name:" })}</span>
              <span>{grant.grantee_name}</span>
            </div>
            <div>
              <span className="text-muted-foreground">{t("accessPage.components.grantDetailDialog.labels.gender", { defaultValue: "Gender:" })}</span>
              <span>{grant.grantee_gender || "-"}</span>
            </div>
            <div>
              <span className="text-muted-foreground">{t("accessPage.components.grantDetailDialog.labels.phone", { defaultValue: "Phone:" })}</span>
              <span>{grant.grantee_phone}</span>
            </div>
            <div>
              <span className="text-muted-foreground">{t("accessPage.components.grantDetailDialog.labels.email", { defaultValue: "Email:" })}</span>
              <span>{grant.grantee_email}</span>
            </div>
            <div>
              <span className="text-muted-foreground">
                {t("accessPage.components.grantDetailDialog.labels.mobileModel", { defaultValue: "Mobile model:" })}
              </span>
              <span>{grant.mobile_model || "-"}</span>
            </div>
            <div>
              <span className="text-muted-foreground">
                {t("accessPage.components.grantDetailDialog.labels.subjectType", { defaultValue: "Subject type:" })}
              </span>
              <span>{grant.pass_type || "-"}</span>
            </div>
            <div>
              <span className="text-muted-foreground">
                {t("accessPage.components.grantDetailDialog.labels.authorizedBy", { defaultValue: "Authorized by:" })}
              </span>
              <span>
                {grant.authorized_by_email || "-"} {grant.authorized_by_role ? `(${grant.authorized_by_role})` : ""}
              </span>
            </div>
            <div>
              <span className="text-muted-foreground">
                {t("accessPage.components.grantDetailDialog.labels.authorizedAt", { defaultValue: "Authorized at:" })}
              </span>
              <span>{new Date(grant.authorized_at || grant.created_at).toLocaleString(i18n.language)}</span>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
