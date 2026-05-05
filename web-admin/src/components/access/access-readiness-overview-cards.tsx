import { Link } from "react-router"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

import { AccessReadinessCard } from "./access-readiness-card"

type AccessReadinessOverviewCardsProps = {
  activeEmployeeCount: number
  groupCount: number
  policyCount: number
  buildingCount: number
  areaCount: number
  doorCount: number
  visitorGrantCount: number
  expiredGrantCount: number
  directoryReady: boolean
  policyReady: boolean
  topologyReady: boolean
  issuanceReady: boolean
  hasEmployees: boolean
  enterpriseSyncLink: string
  directorySectionLink: string
  spacesLink: string
  policiesSectionLink: string
  walletEmployeeLink: string
  grantsSectionLink: string
}

export function AccessReadinessOverviewCards({
  activeEmployeeCount,
  groupCount,
  policyCount,
  buildingCount,
  areaCount,
  doorCount,
  visitorGrantCount,
  expiredGrantCount,
  directoryReady,
  policyReady,
  topologyReady,
  issuanceReady,
  hasEmployees,
  enterpriseSyncLink,
  directorySectionLink,
  spacesLink,
  policiesSectionLink,
  walletEmployeeLink,
  grantsSectionLink,
}: AccessReadinessOverviewCardsProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 xl:grid-cols-3">
      <AccessReadinessCard
        title={t("accessPage.components.readiness.directory.title", { defaultValue: "1. Directory readiness" })}
        description={t("accessPage.components.readiness.directory.description", {
          defaultValue: "Confirm employee directory and groups are ready as upstream inputs for policy and issuance.",
        })}
        status={
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={directoryReady ? "outline" : "secondary"}>
              {directoryReady
                ? t("accessPage.components.readiness.status.ready", { defaultValue: "Ready" })
                : t("accessPage.components.readiness.status.pending", { defaultValue: "Pending" })}
            </Badge>
            <span className="text-sm text-muted-foreground">
              {t("accessPage.components.readiness.directory.metric", {
                defaultValue: "Active employees {{activeEmployeeCount}} / groups {{groupCount}}",
                activeEmployeeCount,
                groupCount,
              })}
            </span>
          </div>
        }
        detail={
          directoryReady
            ? t("accessPage.components.readiness.directory.detailReady", {
                defaultValue: "Directory and groups are ready. Continue to policy configuration.",
              })
            : !hasEmployees
              ? t("accessPage.components.readiness.directory.detailNoEmployees", {
                  defaultValue: "No enterprise employee directory yet. Connect HRIS, SCIM, CSV, or manual sync first.",
                })
              : t("accessPage.components.readiness.directory.detailNoGroups", {
                  defaultValue: "Employee directory exists, but stable user groups are still missing.",
                })
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={!hasEmployees ? enterpriseSyncLink : directorySectionLink}>
              {!hasEmployees
                ? t("accessPage.components.readiness.directory.actionImportEmployees", { defaultValue: "Import employees" })
                : t("accessPage.components.readiness.directory.actionGoGroups", { defaultValue: "Go to employees & groups" })}
            </Link>
          </Button>
        }
      />

      <AccessReadinessCard
        title={t("accessPage.components.readiness.policies.title", { defaultValue: "2. Policy readiness" })}
        description={t("accessPage.components.readiness.policies.description", {
          defaultValue: "Keep permissions as rule layer and verify topology can support building/area/door grants.",
        })}
        status={
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={policyReady ? "outline" : "secondary"}>
              {policyReady
                ? t("accessPage.components.readiness.status.ready", { defaultValue: "Ready" })
                : t("accessPage.components.readiness.status.pending", { defaultValue: "Pending" })}
            </Badge>
            <span className="text-sm text-muted-foreground">
              {t("accessPage.components.readiness.policies.metric", {
                defaultValue: "Policies {{policyCount}} / buildings {{buildingCount}} / areas {{areaCount}} / doors {{doorCount}}",
                policyCount,
                buildingCount,
                areaCount,
                doorCount,
              })}
            </span>
          </div>
        }
        detail={
          !directoryReady
            ? t("accessPage.components.readiness.policies.detailDirectoryNotReady", {
                defaultValue: "Prepare directory and groups first, then start policy configuration.",
              })
            : !topologyReady
              ? t("accessPage.components.readiness.policies.detailTopologyNotReady", {
                  defaultValue: "Topology is incomplete. Complete buildings/areas/doors in Spaces first.",
                })
              : policyReady
                ? t("accessPage.components.readiness.policies.detailReady", {
                    defaultValue: "Policy rules are in place. Continue with temporary/visitor grants or issuance center.",
                  })
                : t("accessPage.components.readiness.policies.detailCanStart", {
                    defaultValue: "Directory and topology are ready. Start creating first access policies.",
                  })
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={!topologyReady ? spacesLink : policiesSectionLink}>
              {!topologyReady
                ? t("accessPage.components.readiness.policies.actionGoTopology", { defaultValue: "Go to topology" })
                : t("accessPage.components.readiness.policies.actionGoPolicies", { defaultValue: "Go to policies" })}
            </Link>
          </Button>
        }
      />

      <AccessReadinessCard
        title={t("accessPage.components.readiness.issuance.title", { defaultValue: "3. Issuance readiness" })}
        description={t("accessPage.components.readiness.issuance.description", {
          defaultValue: "Keep temporary grants here, and move long-term employee and batch issuance to issuance center.",
        })}
        status={
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={issuanceReady ? "outline" : "secondary"}>
              {issuanceReady
                ? t("accessPage.components.readiness.issuance.statusReady", { defaultValue: "Ready for issuance" })
                : t("accessPage.components.readiness.issuance.statusPending", { defaultValue: "Prerequisites missing" })}
            </Badge>
            <span className="text-sm text-muted-foreground">
              {t("accessPage.components.readiness.issuance.metric", {
                defaultValue: "Visitor grants {{visitorGrantCount}} / expired {{expiredGrantCount}}",
                visitorGrantCount,
                expiredGrantCount,
              })}
            </span>
          </div>
        }
        detail={
          issuanceReady
            ? t("accessPage.components.readiness.issuance.detailReady", {
                defaultValue:
                  "Handle long-term employee issuance, reissue, and status operations in pass issuance; keep short-term visitor/temp grants here.",
              })
            : t("accessPage.components.readiness.issuance.detailPending", {
                defaultValue: "Before directory and policy are ready, avoid jumping to long-term issuance directly.",
              })
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={issuanceReady ? walletEmployeeLink : grantsSectionLink}>
              {issuanceReady
                ? t("accessPage.components.readiness.issuance.actionGoIssuance", { defaultValue: "Go to pass issuance" })
                : t("accessPage.components.readiness.issuance.actionGoTemporaryGrants", { defaultValue: "Go to temporary grants" })}
            </Link>
          </Button>
        }
      />
    </div>
  )
}
