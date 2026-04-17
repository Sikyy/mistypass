import { getGrantLifecycleStatus, parseDateTime, type DeliveryMethod } from "@/components/access/access-page-utils"
import type { TemporaryAccess } from "@/lib/api"

export type GrantStatusFilter = "all" | "active" | "expiring_soon" | "expired"

export type TenantGrantViewModel = {
  activeGrantCount: number
  expiredGrantCount: number
  expiringSoonGrantCount: number
  filteredGrantLedger: TemporaryAccess[]
  grantPassTypeOptions: string[]
  tenantGrants: TemporaryAccess[]
  visitorGrantCount: number
}

export function buildTenantGrantViewModel({
  grantDateFrom,
  grantDateTo,
  grantMethodFilter,
  grantPassTypeFilter,
  grantStatusFilter,
  grants,
  nowTick,
  selectedTenantID,
}: {
  grantDateFrom: string
  grantDateTo: string
  grantMethodFilter: "all" | DeliveryMethod
  grantPassTypeFilter: string
  grantStatusFilter: GrantStatusFilter
  grants: TemporaryAccess[]
  nowTick: number
  selectedTenantID: string
}): TenantGrantViewModel {
  let tenantGrants = grants.filter((item) => (selectedTenantID ? item.tenant_id === selectedTenantID : true))

  if (grantDateFrom) {
    const from = new Date(`${grantDateFrom}T00:00:00`)
    tenantGrants = tenantGrants.filter((item) => {
      const at = parseDateTime(item.authorized_at || item.created_at)
      return at ? at.getTime() >= from.getTime() : false
    })
  }

  if (grantDateTo) {
    const to = new Date(`${grantDateTo}T23:59:59`)
    tenantGrants = tenantGrants.filter((item) => {
      const at = parseDateTime(item.authorized_at || item.created_at)
      return at ? at.getTime() <= to.getTime() : false
    })
  }

  tenantGrants = [...tenantGrants].sort((a, b) => {
    const left = parseDateTime(a.authorized_at || a.created_at)?.getTime() ?? 0
    const right = parseDateTime(b.authorized_at || b.created_at)?.getTime() ?? 0
    return right - left
  })

  const grantPassTypeOptions = Array.from(
    new Set(
      tenantGrants
        .map((item) => item.pass_type?.trim())
        .filter((item): item is string => Boolean(item))
    )
  )

  const visitorGrantCount = tenantGrants.filter((item) => (item.pass_type || "").toLowerCase() === "visitor").length
  const activeGrantCount = tenantGrants.filter((item) => getGrantLifecycleStatus(item.valid_until, nowTick) === "active").length
  const expiringSoonGrantCount = tenantGrants.filter(
    (item) => getGrantLifecycleStatus(item.valid_until, nowTick) === "expiring_soon"
  ).length
  const expiredGrantCount = tenantGrants.filter((item) => getGrantLifecycleStatus(item.valid_until, nowTick) === "expired").length

  const filteredGrantLedger = tenantGrants.filter((item) => {
    if (grantMethodFilter !== "all" && item.delivery_method !== grantMethodFilter) {
      return false
    }
    if (grantPassTypeFilter !== "all" && (item.pass_type || "").trim() !== grantPassTypeFilter) {
      return false
    }
    const lifecycleStatus = getGrantLifecycleStatus(item.valid_until, nowTick)
    if (grantStatusFilter === "active" && lifecycleStatus !== "active") {
      return false
    }
    if (grantStatusFilter === "expiring_soon" && lifecycleStatus !== "expiring_soon") {
      return false
    }
    if (grantStatusFilter === "expired" && lifecycleStatus !== "expired") {
      return false
    }
    return true
  })

  return {
    activeGrantCount,
    expiredGrantCount,
    expiringSoonGrantCount,
    filteredGrantLedger,
    grantPassTypeOptions,
    tenantGrants,
    visitorGrantCount,
  }
}
