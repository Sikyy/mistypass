export type KioskGuest = {
  id: string
  name: string
  phone: string
  host_name: string
  company?: string
  status: string
}

export type KioskNDATemplate = {
  title: string
  body: string
  version: number
  required: boolean
}

/**
 * Returns expected (not yet checked-in) guests matching the search query
 * against name / phone / company / host name, sorted by name.
 */
export function filterExpectedGuests<T extends KioskGuest>(guests: T[], query: string): T[] {
  const needle = query.trim().toLowerCase()
  return guests
    .filter((guest) => guest.status === "expected")
    .filter((guest) => {
      if (needle === "") return true
      return [guest.name, guest.phone, guest.company ?? "", guest.host_name]
        .some((field) => field.toLowerCase().includes(needle))
    })
    .sort((a, b) => a.name.localeCompare(b.name))
}

/**
 * The NDA step is shown whenever a template is configured (version > 0) —
 * required templates block check-in server-side; optional ones can be skipped.
 */
export function ndaStepNeeded(template: KioskNDATemplate | undefined): boolean {
  return Boolean(template && template.version > 0)
}
