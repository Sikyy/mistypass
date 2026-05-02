import { request, requestItems, withTenantQuery, encodePathSegment } from "./core"

export type Integration = {
  id: string
  resource_type: "Integration"
  tenant_id: string
  type: "identity_provider" | "hris" | "webhook" | "mqtt" | "device_api" | string
  provider: string
  name: string
  description: string
  status: string
  configured: boolean
  sync_mode?: string
  source_id?: string
  last_sync_at?: string
  created_at: string
  updated_at: string
}

export type IntegrationMutationPayload = {
  tenant_id?: string
  type?: Integration["type"]
  provider?: string
  status?: string
  sync_mode?: string
  credential_ref?: string
  webhook_secret_ref?: string
  issuer_url?: string
  client_id?: string
  auth_url?: string
  token_url?: string
  jwks_url?: string
  user_info_url?: string
  saml_acs_url?: string
  saml_x509_cert?: string
  scopes?: string[]
  actor?: string
}

export async function listIntegrations(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    type?: Integration["type"]
    provider?: string
    status?: string
    sort?: "name" | "-name"
  }
): Promise<Integration[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.type?.trim()) query.set("type", options.type.trim())
  if (options?.provider?.trim()) query.set("provider", options.provider.trim())
  if (options?.status?.trim()) query.set("status", options.status.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Integration>(suffix ? `/api/v1/integrations?${suffix}` : "/api/v1/integrations", token)
}

export async function getIntegration(
  token: string | undefined,
  integrationID: string,
  tenantID?: string
): Promise<Integration> {
  return request<Integration>(
    withTenantQuery(`/api/v1/integrations/${encodePathSegment(integrationID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function createIntegration(
  token: string | undefined,
  payload: IntegrationMutationPayload
): Promise<Integration> {
  return request<Integration>(
    "/api/v1/integrations",
    {
      method: "POST",
      body: JSON.stringify({ integration: payload }),
    },
    token
  )
}

export async function updateIntegration(
  token: string | undefined,
  integrationID: string,
  payload: IntegrationMutationPayload
): Promise<Integration> {
  return request<Integration>(
    `/api/v1/integrations/${encodePathSegment(integrationID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ integration: payload }),
    },
    token
  )
}

export async function deleteIntegration(
  token: string | undefined,
  integrationID: string,
  tenantID?: string
): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/integrations/${encodePathSegment(integrationID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}
