import { request, requestItems, withTenantQuery, encodePathSegment, withPageQuery } from "./core"
import type { ListPageOptions } from "./core"
import type { Share } from "./access-rights"
import { listShares, createShare } from "./access-rights"

export type AccessPolicy = {
  id: string
  tenant_id: string
  name: string
  scope_type: "all" | "building" | "area" | "door"
  building_id?: string
  area_id?: string
  door_id?: string
  schedule: string
  members: number
  status: "active" | "inactive" | "draft"
  updated_at: string
}

export type DoorGroup = {
  id: string
  tenant_id: string
  name: string
  door_ids?: string[]
  created_at: string
}

export type TemporaryAccess = {
  id: string
  tenant_id: string
  scope_type: "all" | "building" | "area" | "door"
  building_id?: string
  area_id?: string
  door_id?: string
  delivery_method: "wallet" | "email_qr"
  grantee_name: string
  grantee_gender?: string
  grantee_phone: string
  grantee_email: string
  mobile_model?: string
  pass_type?: string
  valid_from?: string
  valid_until: string
  authorized_by_id?: string
  authorized_by_email?: string
  authorized_by_role?: string
  authorized_at?: string
  reviewed_at?: string
  reviewed_by?: string
  created_at: string
}

export type VisitorPass = {
  id: string
  tenant_id: string
  host: string
  visitor: string
  delivery_method: "wallet" | "email_qr"
  expires_at: string
  created_at: string
}

// --- Access Policies ---

export async function listAccessPolicies(token: string | undefined): Promise<AccessPolicy[]> {
  return requestItems<AccessPolicy>("/api/v1/access-policies", token)
}

export async function createAccessPolicy(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    scope_type: "all" | "building" | "area" | "door"
    building_id?: string
    area_id?: string
    door_id?: string
    schedule?: string
    members?: number
    status?: "active" | "inactive" | "draft"
  }
): Promise<AccessPolicy> {
  return request<AccessPolicy>(
    "/api/v1/access-policies",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateAccessPolicy(
  token: string | undefined,
  policyID: string,
  payload: {
    name: string
    scope_type: "all" | "building" | "area" | "door"
    building_id?: string
    area_id?: string
    door_id?: string
    schedule?: string
    members?: number
    status?: "active" | "inactive" | "draft"
  }
): Promise<AccessPolicy> {
  return request<AccessPolicy>(
    `/api/v1/access-policies/${encodePathSegment(policyID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

// --- Door Groups ---

export async function listDoorGroups(token: string | undefined, tenantID?: string): Promise<DoorGroup[]> {
  return requestItems<DoorGroup>(withTenantQuery("/api/v1/door_groups", tenantID), token)
}

// --- Temporary Access ---

export async function listTemporaryAccess(token: string | undefined): Promise<TemporaryAccess[]> {
  const shares = await listShares(token)
  return shares.map(temporaryAccessFromShare)
}

export async function createTemporaryAccess(
  token: string | undefined,
  payload: {
    tenant_id: string
    scope_type: "all" | "building" | "area" | "door"
    building_id?: string
    area_id?: string
    door_id?: string
    delivery_method: "wallet" | "email_qr"
    grantee_name: string
    grantee_gender?: string
    grantee_phone: string
    grantee_email: string
    mobile_model?: string
    pass_type?: string
    valid_until: string
  }
): Promise<TemporaryAccess> {
  const created = await createShare(token, {
    tenant_id: payload.tenant_id,
    email: payload.grantee_email,
    place_id: payload.building_id,
    area_id: payload.area_id,
    lock_id: payload.door_id,
    delivery_method: payload.delivery_method,
    grantee_name: payload.grantee_name,
    grantee_phone: payload.grantee_phone,
    mobile_model: payload.mobile_model,
    pass_type: payload.pass_type,
    valid_until: payload.valid_until,
  })
  return temporaryAccessFromShare(created)
}

function temporaryAccessFromShare(item: Share): TemporaryAccess {
  const scopeType: TemporaryAccess["scope_type"] = item.lock_id
    ? "door"
    : item.area_id
      ? "area"
      : item.place_id
        ? "building"
        : "all"
  return {
    id: item.id,
    tenant_id: item.tenant_id,
    scope_type: scopeType,
    building_id: item.place_id,
    area_id: item.area_id,
    door_id: item.lock_id,
    delivery_method: item.delivery_method,
    grantee_name: item.grantee_name ?? item.email,
    grantee_phone: item.grantee_phone ?? "",
    grantee_email: item.email,
    mobile_model: item.mobile_model,
    pass_type: item.pass_type,
    valid_from: item.valid_from,
    valid_until: item.valid_until,
    authorized_by_id: item.authorized_by_id,
    authorized_by_email: item.authorized_by_email,
    authorized_by_role: item.authorized_by_role,
    authorized_at: item.authorized_at,
    reviewed_at: item.reviewed_at,
    reviewed_by: item.reviewed_by,
    created_at: item.created_at,
  }
}

// --- Visitor Passes ---

export async function listVisitorPasses(token: string | undefined): Promise<VisitorPass[]> {
  return requestItems<VisitorPass>("/api/v1/visitor-passes", token)
}

export async function createVisitorPass(
  token: string | undefined,
  payload: {
    tenant_id: string
    host: string
    visitor: string
    delivery_method: "wallet" | "email_qr"
    expires_at: string
  }
): Promise<VisitorPass> {
  return request<VisitorPass>(
    "/api/v1/visitor-passes",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}
