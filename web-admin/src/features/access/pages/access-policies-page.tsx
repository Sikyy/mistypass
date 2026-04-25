import { AccessPage, type AccessPageProps } from "@/pages/access-page"

type AccessPoliciesPageProps = Omit<AccessPageProps, "activeSectionOverride">

export function AccessPoliciesPage(props: AccessPoliciesPageProps) {
  return <AccessPage {...props} activeSectionOverride="policies" />
}
