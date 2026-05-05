import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"

type NotFoundPageProps = {
  authenticated?: boolean
}

export function NotFoundPage({ authenticated = true }: NotFoundPageProps) {
  const { t } = useTranslation()
  const nextPath = authenticated ? "/dashboard" : "/login"
  const nextLabel = authenticated ? t("notFound.goDashboard") : t("notFound.goLogin")

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 rounded-xl border bg-background px-6 text-center">
      <p className="text-5xl font-semibold leading-none text-foreground/90">404</p>
      <div className="space-y-1">
        <h1 className="text-xl font-semibold">{t("notFound.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("notFound.description")}</p>
      </div>
      <Button asChild>
        <Link to={nextPath}>{nextLabel}</Link>
      </Button>
    </div>
  )
}
