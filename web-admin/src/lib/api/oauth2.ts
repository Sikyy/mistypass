import { encodePathSegment, request, requestItems } from "./core"

export type OAuth2Client = {
  id: string
  tenant_id: string
  name: string
  client_secret?: string
  redirect_uris: string[]
  scopes: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export type OAuth2ClientCreatePayload = {
  name: string
  redirect_uris: string[]
  scopes: string[]
}

export type OAuth2ClientUpdatePayload = Partial<OAuth2ClientCreatePayload> & {
  enabled?: boolean
}

export async function listOAuth2Clients(token: string | undefined): Promise<OAuth2Client[]> {
  return requestItems<OAuth2Client>("/api/v1/oauth2/clients", token)
}

export async function createOAuth2Client(
  token: string | undefined,
  payload: OAuth2ClientCreatePayload
): Promise<OAuth2Client> {
  return request<OAuth2Client>(
    "/api/v1/oauth2/clients",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function updateOAuth2Client(
  token: string | undefined,
  clientID: string,
  payload: OAuth2ClientUpdatePayload
): Promise<OAuth2Client> {
  return request<OAuth2Client>(
    `/api/v1/oauth2/clients/${encodePathSegment(clientID)}`,
    { method: "PATCH", body: JSON.stringify(payload) },
    token
  )
}

export async function deleteOAuth2Client(token: string | undefined, clientID: string): Promise<void> {
  return request<void>(
    `/api/v1/oauth2/clients/${encodePathSegment(clientID)}`,
    { method: "DELETE" },
    token
  )
}
