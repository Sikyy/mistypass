import { useTranslation } from "react-i18next"

type AccessGrantOverviewCardsProps = {
  activeCount: number
  expiringSoonCount: number
  visitorCount: number
  expiredCount: number
  onShowActive: () => void
  onShowExpiringSoon: () => void
  onShowVisitors: () => void
  onShowExpired: () => void
}

export function AccessGrantOverviewCards({
  activeCount,
  expiringSoonCount,
  visitorCount,
  expiredCount,
  onShowActive,
  onShowExpiringSoon,
  onShowVisitors,
  onShowExpired,
}: AccessGrantOverviewCardsProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowActive}
      >
        <p className="mp-kpi-note">{t("accessPage.components.grantOverview.active", { defaultValue: "Active" })}</p>
        <p className="mt-1 text-sm font-medium">
          {t("accessPage.components.grantOverview.count", { defaultValue: "{{count}}", count: activeCount })}
        </p>
      </button>
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowExpiringSoon}
      >
        <p className="mp-kpi-note">{t("accessPage.components.grantOverview.expiringSoon", { defaultValue: "Expiring within 24h" })}</p>
        <p className="mt-1 text-sm font-medium">
          {t("accessPage.components.grantOverview.count", { defaultValue: "{{count}}", count: expiringSoonCount })}
        </p>
      </button>
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowVisitors}
      >
        <p className="mp-kpi-note">{t("accessPage.components.grantOverview.visitor", { defaultValue: "Visitor grants" })}</p>
        <p className="mt-1 text-sm font-medium">
          {t("accessPage.components.grantOverview.count", { defaultValue: "{{count}}", count: visitorCount })}
        </p>
      </button>
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowExpired}
      >
        <p className="mp-kpi-note">{t("accessPage.components.grantOverview.expired", { defaultValue: "Expired" })}</p>
        <p className="mt-1 text-sm font-medium">
          {t("accessPage.components.grantOverview.count", { defaultValue: "{{count}}", count: expiredCount })}
        </p>
      </button>
    </div>
  )
}
