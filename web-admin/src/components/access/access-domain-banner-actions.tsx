import { Link } from "react-router"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

type AccessDomainBannerActionsProps = {
  primaryActionLabel: string
  primaryActionTo: string
  enterpriseHomeLink: string
  walletEmployeeLink: string
  hasWorkerAlertFlowHints: boolean
  enterpriseSyncWorkerReviewLink: string
}

export function AccessDomainBannerActions({
  primaryActionLabel,
  primaryActionTo,
  enterpriseHomeLink,
  walletEmployeeLink,
  hasWorkerAlertFlowHints,
  enterpriseSyncWorkerReviewLink,
}: AccessDomainBannerActionsProps) {
  const { t } = useTranslation()

  return (
    <>
      <Button asChild size="sm">
        <Link to={primaryActionTo}>{primaryActionLabel}</Link>
      </Button>
      <Button asChild size="sm" variant="outline">
        <Link to={enterpriseHomeLink}>
          {t("accessPage.components.bannerActions.openEnterpriseSync", { defaultValue: "Open enterprise directory & sync" })}
        </Link>
      </Button>
      {hasWorkerAlertFlowHints ? (
        <Button asChild size="sm" variant="outline">
          <Link to={enterpriseSyncWorkerReviewLink}>
            {t("accessPage.components.bannerActions.backToSyncReview", {
              defaultValue: "Return to import & sync review after handling",
            })}
          </Link>
        </Button>
      ) : null}
      <Button asChild size="sm" variant="outline">
        <Link to={walletEmployeeLink}>{t("accessPage.components.bannerActions.goPassIssuance", { defaultValue: "Go to pass issuance" })}</Link>
      </Button>
    </>
  )
}
