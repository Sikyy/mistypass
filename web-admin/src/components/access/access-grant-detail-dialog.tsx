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
  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen)
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>被授权人基本信息</DialogTitle>
          <DialogDescription>用于核验授权对象身份、联系方式与设备信息。</DialogDescription>
        </DialogHeader>
        {grant ? (
          <div className="grid gap-2 text-sm">
            <div>
              <span className="text-muted-foreground">授权 ID：</span>
              <span className="font-medium">{grant.id}</span>
            </div>
            <div>
              <span className="text-muted-foreground">姓名：</span>
              <span>{grant.grantee_name}</span>
            </div>
            <div>
              <span className="text-muted-foreground">性别：</span>
              <span>{grant.grantee_gender || "-"}</span>
            </div>
            <div>
              <span className="text-muted-foreground">手机号：</span>
              <span>{grant.grantee_phone}</span>
            </div>
            <div>
              <span className="text-muted-foreground">邮箱：</span>
              <span>{grant.grantee_email}</span>
            </div>
            <div>
              <span className="text-muted-foreground">手机型号：</span>
              <span>{grant.mobile_model || "-"}</span>
            </div>
            <div>
              <span className="text-muted-foreground">对象类型：</span>
              <span>{grant.pass_type || "-"}</span>
            </div>
            <div>
              <span className="text-muted-foreground">授权人：</span>
              <span>
                {grant.authorized_by_email || "-"} {grant.authorized_by_role ? `(${grant.authorized_by_role})` : ""}
              </span>
            </div>
            <div>
              <span className="text-muted-foreground">授权时间：</span>
              <span>{new Date(grant.authorized_at || grant.created_at).toLocaleString("zh-CN")}</span>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
