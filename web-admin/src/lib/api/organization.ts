import { request, withTenantQuery } from "./core"

export type OrganizationSettings = {
  tenant_id: string
  name: string
  primary_domain: string
  timezone: string
  support_email: string
  email_notifications: boolean
  push_notifications: boolean
  weekly_reports: boolean
  enforce_mfa: boolean
  webauthn_enabled: boolean
  password_policy: string
  session_timeout_minutes: number
  updated_at: string
}

export async function getOrganizationSettings(
  token: string | undefined,
  tenantID?: string
): Promise<OrganizationSettings> {
  return request<OrganizationSettings>(
    withTenantQuery("/api/v1/organization/settings", tenantID),
    {},
    token
  )
}

export async function updateOrganizationSettings(
  token: string | undefined,
  payload: Partial<OrganizationSettings> & { tenant_id?: string }
): Promise<OrganizationSettings> {
  return request<OrganizationSettings>(
    "/api/v1/organization/settings",
    { method: "PATCH", body: JSON.stringify(payload) },
    token
  )
}

export async function exportOrganizationAudit(
  token: string | undefined,
  tenantID?: string
): Promise<{ status: string; message: string }> {
  return request<{ status: string; message: string }>(
    withTenantQuery("/api/v1/organization/export-audit", tenantID),
    { method: "POST" },
    token
  )
}

export async function rotateOrganizationWebhooks(
  token: string | undefined,
  tenantID?: string
): Promise<{ status: string; message: string }> {
  return request<{ status: string; message: string }>(
    withTenantQuery("/api/v1/organization/rotate-webhooks", tenantID),
    { method: "POST" },
    token
  )
}

export async function disableOrganization(
  token: string | undefined,
  tenantID?: string
): Promise<{ status: string; message: string }> {
  return request<{ status: string; message: string }>(
    withTenantQuery("/api/v1/organization/disable", tenantID),
    { method: "POST" },
    token
  )
}
