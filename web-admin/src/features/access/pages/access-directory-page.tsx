import { AccessPage, type AccessPageProps } from "@/features/legacy/pages/access-page"

type AccessDirectoryPageProps = Omit<AccessPageProps, "activeSectionOverride">

export function AccessDirectoryPage(props: AccessDirectoryPageProps) {
  return <AccessPage {...props} activeSectionOverride="directory" />
}
