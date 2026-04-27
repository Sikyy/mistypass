import { describe, expect, it } from "vitest"

import {
  buildKisiAccessRightsFromReferenceResources,
  type KisiGroupResource,
  type KisiPlaceResource,
  type KisiTeamResource,
  type KisiUserResource,
} from "./resource-data"
import type { Door, Role, RoleAssignment, Share } from "@/lib/api"

const place = {
  id: "place_001",
  name: "Sudirman Hub",
  address: "Jakarta",
  region: "Jakarta",
  doorCount: 1,
  gatewayCount: 1,
  offlineCount: 0,
  statusLabel: "All online",
  tone: "success",
} satisfies KisiPlaceResource

const group = {
  id: "group_service",
  placeId: place.id,
  name: "Service Personnel",
  kind: "User group",
  memberCount: 3,
  targetLabel: "3 members",
  description: "Service access",
  statusLabel: "Enabled",
  tone: "success",
} satisfies KisiGroupResource

const user = {
  id: "user_rina",
  placeId: place.id,
  name: "Rina Hartono",
  email: "rina@example.com",
  role: "Place Admin",
  statusLabel: "Active",
  tone: "success",
  accessDateLabel: "Apr 26, 2026",
  sourceLabel: "Manual",
} satisfies KisiUserResource

const team = {
  id: "team_engineering",
  placeId: place.id,
  name: "Engineering Team",
  scopeLabel: "Sudirman Hub",
  sourceLabel: "SCIM",
  description: "Engineering staff",
  memberCount: 2,
  accessRightCount: 1,
  statusLabel: "Enabled",
  tone: "success",
} satisfies KisiTeamResource

const roles: Role[] = [
  {
    id: "role_place_admin",
    name: "Place Admin",
    applies_to: "Place",
    permissions: {},
    built_in: true,
  },
  {
    id: "role_group_access",
    name: "Group Access",
    applies_to: "Group",
    permissions: {},
    built_in: true,
  },
]

const door = {
  id: "lock_001",
  tenant_id: "tenant_001",
  building_id: place.id,
  floor_id: "floor_001",
  area_id: "area_001",
  name: "Lobby Turnstile",
  gateway_id: "gateway_001",
  kind: "turnstile",
  status: "online",
  created_at: "2026-04-26T00:00:00Z",
} satisfies Door

describe("kisi resource-data access rights mapper", () => {
  it("maps role assignments to reference access-right rows", () => {
    const assignments: RoleAssignment[] = [
      {
        id: "ra_place_admin",
        tenant_id: "tenant_001",
        role_id: "role_place_admin",
        applies_to_type: "Place",
        applies_to_id: place.id,
        assignee_type: "User",
        assignee_id: user.id,
        assignee_email: user.email,
        created_at: "2026-04-26T00:00:00Z",
        updated_at: "2026-04-26T00:00:00Z",
      },
    ]

    const rows = buildKisiAccessRightsFromReferenceResources({
      roleAssignments: assignments,
      shares: [],
      roles,
      places: [place],
      groups: [group],
      teams: [team],
      users: [user],
      doors: [door],
    })

    expect(rows).toEqual([
      expect.objectContaining({
        id: "ra_place_admin",
        placeId: place.id,
        name: "Sudirman Hub",
        subjectType: "Place",
        targetLabel: "Rina Hartono",
        ruleLabel: "Place Admin",
        statusLabel: "Enabled",
        tone: "success",
      }),
    ])
  })

  it("maps shares to access-link rows and resolves group scope", () => {
    const shares: Share[] = [
      {
        id: "share_guest",
        tenant_id: "tenant_001",
        email: "guest@example.com",
        group_id: group.id,
        role_id: "role_group_access",
        valid_until: "2099-05-01T10:00:00Z",
        status: "active",
        delivery_method: "email_qr",
        grantee_name: "Guest Visitor",
        created_at: "2026-04-26T00:00:00Z",
      },
    ]

    const rows = buildKisiAccessRightsFromReferenceResources({
      roleAssignments: [],
      shares,
      roles,
      places: [place],
      groups: [group],
      teams: [team],
      users: [user],
      doors: [door],
    })

    expect(rows).toEqual([
      expect.objectContaining({
        id: "share_guest",
        placeId: place.id,
        name: "Guest Visitor",
        subjectType: "Access Link",
        targetLabel: "Service Personnel",
        statusLabel: "Enabled",
        tone: "success",
      }),
    ])
    expect(rows[0].ruleLabel).toContain("Expires")
  })

  it("resolves team assignee role assignments to team rows", () => {
    const assignments: RoleAssignment[] = [
      {
        id: "ra_team_group",
        tenant_id: "tenant_001",
        role_id: "role_group_access",
        applies_to_type: "Group",
        applies_to_id: group.id,
        assignee_type: "Team",
        assignee_id: team.id,
        created_at: "2026-04-26T00:00:00Z",
        updated_at: "2026-04-26T00:00:00Z",
      },
    ]

    const rows = buildKisiAccessRightsFromReferenceResources({
      roleAssignments: assignments,
      shares: [],
      roles,
      places: [place],
      groups: [group],
      teams: [team],
      users: [user],
      doors: [door],
    })

    expect(rows).toEqual([
      expect.objectContaining({
        id: "ra_team_group",
        placeId: place.id,
        name: "Engineering Team",
        subjectType: "Team",
        targetLabel: "Service Personnel",
        ruleLabel: "Group Access",
      }),
    ])
  })
})
