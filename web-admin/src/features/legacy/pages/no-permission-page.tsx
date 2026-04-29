import { LogOutIcon, ShieldOffIcon, SmartphoneIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { MistyIslandMark } from "@/components/brand/misty-island-mark"
import { Button } from "@/components/ui/button"
import type { CurrentUser } from "@/lib/api"
import { getViewerRoleLabel } from "@/lib/viewer"

type NoPermissionPageProps = {
  viewer: CurrentUser
  onLogout: () => void
}

export function NoPermissionPage({ viewer, onLogout }: NoPermissionPageProps) {
  const { t } = useTranslation()

  return (
    <main className="mp-fog-surface flex min-h-screen items-center justify-center bg-background px-4 py-10 text-foreground">
      <section className="w-full max-w-xl rounded-[22px] border border-white/10 bg-white/[0.055] p-6 text-center shadow-[inset_0_1px_0_rgba(255,255,255,0.10)] backdrop-blur-2xl md:p-8">
        <div className="mx-auto flex size-16 items-center justify-center rounded-2xl border border-white/10 bg-black/20">
          <MistyIslandMark className="size-12" markClassName="h-10 w-14" />
        </div>
        <div className="mt-6 space-y-2">
          <p className="mx-auto inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.06] px-3 py-1 text-xs text-muted-foreground">
            <ShieldOffIcon className="size-3.5" />
            {getViewerRoleLabel(viewer)}
          </p>
          <h1 className="text-2xl font-semibold tracking-normal md:text-3xl">{t("noPermission.title")}</h1>
          <p className="mx-auto max-w-md text-sm leading-6 text-muted-foreground">{t("noPermission.description")}</p>
        </div>
        <div className="mt-6 rounded-2xl border border-white/10 bg-black/20 px-4 py-3 text-left text-sm text-muted-foreground">
          <div className="flex items-start gap-3">
            <SmartphoneIcon className="mt-0.5 size-4 shrink-0 text-sky-200" />
            <p>{t("noPermission.mobileHint", { email: viewer.email })}</p>
          </div>
        </div>
        <div className="mt-6 flex flex-col justify-center gap-2 sm:flex-row">
          <Button variant="outline" className="border-white/10 bg-white/[0.045]" onClick={onLogout}>
            <LogOutIcon className="mr-1.5 size-4" />
            {t("noPermission.logout")}
          </Button>
        </div>
      </section>
    </main>
  )
}
