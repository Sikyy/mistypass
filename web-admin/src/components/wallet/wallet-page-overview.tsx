import { MessageCircleIcon, RefreshCwIcon, ShieldAlertIcon, WalletCardsIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

type WalletPageOverviewProps = {
  writable: boolean
  readOnlyBoundaryHint: string
}

export function WalletPageOverview({
  writable,
  readOnlyBoundaryHint,
}: WalletPageOverviewProps) {
  const { t } = useTranslation()

  return (
    <>
      <div className="mp-page-hero">
        <div className="relative z-10 max-w-3xl space-y-2">
          <p className="mp-page-eyebrow">{t("walletPage.page.eyebrow")}</p>
          <h1 className="mp-page-title">{t("walletPage.page.title")}</h1>
          <p className="mp-page-description">
            {t("walletPage.page.description")}
          </p>
        </div>
      </div>

      {!writable ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          {t("walletPage.hints.readOnlyIntro")}
          {readOnlyBoundaryHint}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[
          {
            titleKey: "walletPage.overview.employee.title",
            descriptionKey: "walletPage.overview.employee.description",
            icon: WalletCardsIcon,
          },
          {
            titleKey: "walletPage.overview.visitor.title",
            descriptionKey: "walletPage.overview.visitor.description",
            icon: MessageCircleIcon,
          },
          {
            titleKey: "walletPage.overview.batch.title",
            descriptionKey: "walletPage.overview.batch.description",
            icon: RefreshCwIcon,
          },
          {
            titleKey: "walletPage.overview.exceptions.title",
            descriptionKey: "walletPage.overview.exceptions.description",
            icon: ShieldAlertIcon,
          },
        ].map((item) => (
          <div key={item.titleKey} className="mp-metric-card">
            <div className="relative z-10 space-y-3">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-base font-semibold tracking-[-0.02em]">{t(item.titleKey)}</h2>
                <div className="flex size-10 items-center justify-center rounded-full border border-white/10 bg-white/10">
                  <item.icon className="size-4 text-white/75" />
                </div>
              </div>
              <p className="text-sm leading-6 text-muted-foreground">{t(item.descriptionKey)}</p>
            </div>
          </div>
        ))}
      </div>
    </>
  )
}
