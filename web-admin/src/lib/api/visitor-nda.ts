import { encodePathSegment, request, withTenantQuery } from "./core"
import type { Guest } from "./users"

// --- Visitor NDA ---

export type VisitorNDATemplate = {
  tenant_id: string
  title: string
  body: string
  version: number
  required: boolean
  updated_at?: string
}

export async function getVisitorNDATemplate(token: string | undefined, tenantID?: string): Promise<VisitorNDATemplate> {
  return request<VisitorNDATemplate>(withTenantQuery("/api/v1/visitor-nda/template", tenantID), {}, token)
}

export async function updateVisitorNDATemplate(
  token: string | undefined,
  payload: { tenant_id?: string; title?: string; body?: string; required?: boolean }
): Promise<VisitorNDATemplate> {
  return request<VisitorNDATemplate>("/api/v1/visitor-nda/template", {
    method: "PUT",
    body: JSON.stringify(payload),
  }, token)
}

export async function signGuestNDA(
  token: string | undefined,
  guestID: string,
  payload: { tenant_id?: string; signer_name: string; signature_data_url: string }
): Promise<Guest> {
  return request<Guest>(`/api/v1/guests/${encodePathSegment(guestID)}/nda/sign`, {
    method: "POST",
    body: JSON.stringify(payload),
  }, token)
}
