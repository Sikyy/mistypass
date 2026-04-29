import { AccessPage, type AccessPageProps } from "@/features/legacy/pages/access-page"

type AccessPoliciesPageProps = Omit<AccessPageProps, "activeSectionOverride">

export function AccessPoliciesPage(props: AccessPoliciesPageProps) {
  return <AccessPage {...props} activeSectionOverride="policies" />
}
