import i18n from "@/lib/i18n"

import type { AccessSection } from "./access-sections-tabs"

export type AccessSectionOverviewItem = {
  value: AccessSection
  title: string
  description: string
  metric: string
  helper: string
}

export type AccessRecommendedAction = {
  title: string
  description: string
  to: string
  label: string
}

function t(key: string, defaultValue: string, options?: Record<string, unknown>) {
  return i18n.t(key, {
    defaultValue,
    ...options,
  })
}

export function buildAccessSectionsOverview({
  activeEmployeeCount,
  employeeCount,
  expiredGrantCount,
  grantCount,
  groupCount,
  loading,
  policyCount,
  visitorGrantCount,
}: {
  activeEmployeeCount: number
  employeeCount: number
  expiredGrantCount: number
  grantCount: number
  groupCount: number
  loading: boolean
  policyCount: number
  visitorGrantCount: number
}): AccessSectionOverviewItem[] {
  return [
    {
      value: "directory",
      title: t("accessPage.components.recommendation.sections.directory.title", "Employees & user groups"),
      description: t(
        "accessPage.components.recommendation.sections.directory.description",
        "Connect employee directory first, then map people into groups for downstream policy and issuance."
      ),
      metric: loading
        ? "--"
        : t(
            "accessPage.components.recommendation.sections.directory.metric",
            "{{activeEmployeeCount}} active employees / {{groupCount}} groups",
            {
              activeEmployeeCount,
              groupCount,
            }
          ),
      helper:
        employeeCount > 0
          ? t(
              "accessPage.components.recommendation.sections.directory.helperConnected",
              "Directory is connected. You can maintain group members directly."
            )
          : t(
              "accessPage.components.recommendation.sections.directory.helperEmpty",
              "Employee directory is empty. Connect HRIS, SCIM, CSV, or manual import in Enterprise first."
            ),
    },
    {
      value: "policies",
      title: t("accessPage.components.recommendation.sections.policies.title", "Access policies"),
      description: t(
        "accessPage.components.recommendation.sections.policies.description",
        "Apply permissions to building/area/door scopes and prepare policy templates for different groups."
      ),
      metric: loading
        ? "--"
        : t("accessPage.components.recommendation.sections.policies.metric", "{{policyCount}} policies", {
            policyCount,
          }),
      helper: t(
        "accessPage.components.recommendation.sections.policies.helper",
        "Policies are the rule layer and should stay separate from imports and temporary grants."
      ),
    },
    {
      value: "grants",
      title: t("accessPage.components.recommendation.sections.grants.title", "Temporary & visitor grants"),
      description: t(
        "accessPage.components.recommendation.sections.grants.description",
        "Handle short-term grants, visitor access, and temporary email-QR / Mistyislet issuance."
      ),
      metric: loading
        ? "--"
        : t(
            "accessPage.components.recommendation.sections.grants.metric",
            "{{grantCount}} grants / {{visitorGrantCount}} visitor grants",
            {
              grantCount,
              visitorGrantCount,
            }
          ),
      helper:
        expiredGrantCount > 0
          ? t(
              "accessPage.components.recommendation.sections.grants.helperExpired",
              "{{expiredGrantCount}} grants have expired. Review first.",
              {
                expiredGrantCount,
              }
            )
          : t("accessPage.components.recommendation.sections.grants.helperNormal", "Grant validity is normal."),
    },
  ]
}

export function deriveNextRecommendedAction({
  directorySectionLink,
  employeeCount,
  enterpriseSyncLink,
  policyReady,
  policiesSectionLink,
  spacesLink,
  topologyReady,
  walletEmployeeLink,
  groupCount,
}: {
  directorySectionLink: string
  employeeCount: number
  enterpriseSyncLink: string
  groupCount: number
  policiesSectionLink: string
  policyReady: boolean
  spacesLink: string
  topologyReady: boolean
  walletEmployeeLink: string
}): AccessRecommendedAction {
  if (employeeCount === 0) {
    return {
      title: t("accessPage.components.recommendation.actions.importDirectory.title", "Connect employee directory first"),
      description: t(
        "accessPage.components.recommendation.actions.importDirectory.description",
        "Without employee directory, groups, policies, and long-term issuance will all miss upstream data."
      ),
      to: enterpriseSyncLink,
      label: t("accessPage.components.recommendation.actions.importDirectory.label", "Import employees"),
    }
  }
  if (groupCount === 0) {
    return {
      title: t("accessPage.components.recommendation.actions.organizeGroups.title", "Organize user groups first"),
      description: t(
        "accessPage.components.recommendation.actions.organizeGroups.description",
        "Employee directory exists, but stable groups are still missing for policy and issuance targets."
      ),
      to: directorySectionLink,
      label: t("accessPage.components.recommendation.actions.organizeGroups.label", "Go to employees & groups"),
    }
  }
  if (!topologyReady) {
    return {
      title: t("accessPage.components.recommendation.actions.fixTopology.title", "Complete space topology"),
      description: t(
        "accessPage.components.recommendation.actions.fixTopology.description",
        "Policies require precise building/area/door topology, and current topology is incomplete."
      ),
      to: spacesLink,
      label: t("accessPage.components.recommendation.actions.fixTopology.label", "Go to topology"),
    }
  }
  if (!policyReady) {
    return {
      title: t("accessPage.components.recommendation.actions.seedPolicies.title", "Create first access policies"),
      description: t(
        "accessPage.components.recommendation.actions.seedPolicies.description",
        "Groups and topology are ready; start defining building/area/door access rules."
      ),
      to: policiesSectionLink,
      label: t("accessPage.components.recommendation.actions.seedPolicies.label", "Go to policies"),
    }
  }
  return {
    title: t("accessPage.components.recommendation.actions.goIssuance.title", "Move to pass issuance"),
    description: t(
      "accessPage.components.recommendation.actions.goIssuance.description",
      "Directory, groups, and policies are ready; continue long-term issuance and status operations in issuance center."
    ),
    to: walletEmployeeLink,
    label: t("accessPage.components.recommendation.actions.goIssuance.label", "Go to pass issuance"),
  }
}
