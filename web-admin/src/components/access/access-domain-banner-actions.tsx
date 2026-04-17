import { Link } from "react-router-dom"

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
  return (
    <>
      <Button asChild size="sm">
        <Link to={primaryActionTo}>{primaryActionLabel}</Link>
      </Button>
      <Button asChild size="sm" variant="outline">
        <Link to={enterpriseHomeLink}>打开企业目录与同步</Link>
      </Button>
      {hasWorkerAlertFlowHints ? (
        <Button asChild size="sm" variant="outline">
          <Link to={enterpriseSyncWorkerReviewLink}>处理完成后回导入与同步复核</Link>
        </Button>
      ) : null}
      <Button asChild size="sm" variant="outline">
        <Link to={walletEmployeeLink}>去凭证发放</Link>
      </Button>
    </>
  )
}
