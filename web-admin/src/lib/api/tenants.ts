import { request, requestItems, encodePathSegment } from "./core"

export type Tenant = {
  id: string
  name: string
  type: "studio" | "company" | "government" | "factory" | "public_facility"
  hq_region?: string
  status: "active" | "suspended" | "inactive"
  created_at: string
}

export type TenantTopology = {
  tenant_id: string
  buildings: import("./spaces").Building[]
  floors: import("./spaces").Floor[]
  areas: import("./spaces").Area[]
  doors: import("./locks").Door[]
}

export async function listTenants(token: string | undefined): Promise<Tenant[]> {
  return requestItems<Tenant>("/api/v1/tenants", token)
}

export async function createTenant(
  token: string | undefined,
  payload: {
    name: string
    type?: "studio" | "company" | "government" | "factory" | "public_facility"
    hq_region?: string
  }
): Promise<Tenant> {
  return request<Tenant>(
    "/api/v1/tenants",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateTenantStatus(
  token: string | undefined,
  tenantID: string,
  status: "active" | "suspended" | "inactive"
): Promise<Tenant> {
  return request<Tenant>(
    `/api/v1/tenants/${encodePathSegment(tenantID)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify({ status }),
    },
    token
  )
}

export async function getTenantTopology(token: string | undefined, tenantID: string): Promise<TenantTopology> {
  return request<TenantTopology>(`/api/v1/tenants/${encodePathSegment(tenantID)}/topology`, { method: "GET" }, token)
}
