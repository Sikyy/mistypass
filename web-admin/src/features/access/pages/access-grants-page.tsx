import { AccessPage, type AccessPageProps } from "@/features/legacy/pages/access-page"

type AccessGrantsPageProps = Omit<AccessPageProps, "activeSectionOverride">

export function AccessGrantsPage(props: AccessGrantsPageProps) {
  return <AccessPage {...props} activeSectionOverride="grants" />
}
