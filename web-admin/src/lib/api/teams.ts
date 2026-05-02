import { request, requestItems, encodePathSegment } from "./core"

export type Team = {
  id: string
  resource_type: "Team"
  tenant_id: string
  name: string
  scope: "organization" | "place"
  place_id?: string
  description?: string
  source?: string
  created_at: string
  updated_at: string
}

export type TeamMembership = {
  id: string
  resource_type: "TeamMembership"
  tenant_id: string
  team_id: string
  member_type: "User" | "Guest"
  member_id: string
  member_email?: string
  member_name?: string
  source?: string
  created_at: string
  updated_at: string
}

export async function listTeams(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    query?: string
    scope?: Team["scope"]
    place_id?: string
    sort?: "name" | "-name"
  }
): Promise<Team[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.query?.trim()) query.set("query", options.query.trim())
  if (options?.scope?.trim()) query.set("scope", options.scope.trim())
  if (options?.place_id?.trim()) query.set("place_id", options.place_id.trim())
  if (options?.sort?.trim()) query.set("sort", options.sort.trim())
  const suffix = query.toString()
  return requestItems<Team>(suffix ? `/api/v1/teams?${suffix}` : "/api/v1/teams", token)
}

export async function getTeam(token: string | undefined, teamID: string, tenantID?: string): Promise<Team> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<Team>(
    suffix ? `/api/v1/teams/${encodePathSegment(teamID)}?${suffix}` : `/api/v1/teams/${encodePathSegment(teamID)}`,
    { method: "GET" },
    token
  )
}

export async function createTeam(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    scope?: Team["scope"]
    place_id?: string
    description?: string
    source?: string
  }
): Promise<Team> {
  return request<Team>(
    "/api/v1/teams",
    {
      method: "POST",
      body: JSON.stringify({ team: payload }),
    },
    token
  )
}

export async function updateTeam(
  token: string | undefined,
  teamID: string,
  payload: {
    tenant_id?: string
    name: string
    scope?: Team["scope"]
    place_id?: string
    description?: string
    source?: string
  }
): Promise<Team> {
  return request<Team>(
    `/api/v1/teams/${encodePathSegment(teamID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ team: payload }),
    },
    token
  )
}

export async function deleteTeam(token: string | undefined, teamID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/teams/${encodePathSegment(teamID)}?${suffix}` : `/api/v1/teams/${encodePathSegment(teamID)}`,
    { method: "DELETE" },
    token
  )
}

export async function listTeamMemberships(
  token: string | undefined,
  options?: {
    tenant_id?: string
    ids?: string[]
    team_id?: string
    member_type?: TeamMembership["member_type"]
    member_id?: string
  }
): Promise<TeamMembership[]> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) query.set("tenant_id", options.tenant_id.trim())
  if (options?.ids && options.ids.length > 0) query.set("ids", options.ids.join(","))
  if (options?.team_id?.trim()) query.set("team_id", options.team_id.trim())
  if (options?.member_type?.trim()) query.set("member_type", options.member_type.trim())
  if (options?.member_id?.trim()) query.set("member_id", options.member_id.trim())
  const suffix = query.toString()
  return requestItems<TeamMembership>(suffix ? `/api/v1/team_memberships?${suffix}` : "/api/v1/team_memberships", token)
}

export async function createTeamMembership(
  token: string | undefined,
  payload: {
    tenant_id: string
    team_id: string
    member_type: TeamMembership["member_type"]
    member_id: string
    member_email?: string
    member_name?: string
    source?: string
  }
): Promise<TeamMembership> {
  return request<TeamMembership>(
    "/api/v1/team_memberships",
    {
      method: "POST",
      body: JSON.stringify({ team_membership: payload }),
    },
    token
  )
}

export async function deleteTeamMembership(token: string | undefined, membershipID: string, tenantID?: string): Promise<void> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) query.set("tenant_id", tenantID.trim())
  const suffix = query.toString()
  return request<void>(
    suffix ? `/api/v1/team_memberships/${encodePathSegment(membershipID)}?${suffix}` : `/api/v1/team_memberships/${encodePathSegment(membershipID)}`,
    { method: "DELETE" },
    token
  )
}
