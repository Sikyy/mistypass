import { beforeEach, describe, expect, it, vi } from "vitest"

vi.mock("@/lib/api", () => ({
  createEventSet: vi.fn(),
  listAccessEvents: vi.fn(),
  listAccessPolicies: vi.fn(),
  listAccessUsers: vi.fn(),
  listAreas: vi.fn(),
  listCardAssignments: vi.fn(),
  listCards: vi.fn(),
  listControllers: vi.fn(),
  listDoorGroups: vi.fn(),
  listFloors: vi.fn(),
  listGateways: vi.fn(),
  listGroupLocks: vi.fn(),
  listGroupZones: vi.fn(),
  listGroups: vi.fn(),
  listLocks: vi.fn(),
  listPlaces: vi.fn(),
  listReaders: vi.fn(),
  listRoleAssignments: vi.fn(),
  listRoles: vi.fn(),
  listShares: vi.fn(),
  listTeamMemberships: vi.fn(),
  listTeams: vi.fn(),
  listTerminals: vi.fn(),
  listTemporaryAccess: vi.fn(),
  listWalletPasses: vi.fn(),
}))

import {
  buildMistyisletAccessRightsFromReferenceResources,
  buildMistyisletHardwareFromReferenceResources,
  loadMistyisletResourceSummary,
  selectMistyisletPlaceContext,
  type MistyisletGroupResource,
  type MistyisletPlaceResource,
  type MistyisletResourceSummary,
  type MistyisletTeamResource,
  type MistyisletUserResource,
} from "./resource-data"
import {
  createEventSet,
  listAccessEvents,
  listAccessPolicies,
  listAccessUsers,
  listAreas,
  listCardAssignments,
  listCards,
  listControllers,
  listDoorGroups,
  listFloors,
  listGateways,
  listGroupLocks,
  listGroupZones,
  listGroups,
  listLocks,
  listPlaces,
  listReaders,
  listRoleAssignments,
  listRoles,
  listShares,
  listTeamMemberships,
  listTeams,
  listTerminals,
  listTemporaryAccess,
  listWalletPasses,
  type AccessEvent,
  type AccessUser,
  type Area,
  type Building,
  type Card,
  type CardAssignment,
  type Controller,
  type CurrentUser,
  type Door,
  type DoorGroup,
  type EventSet,
  type Floor,
  type Gateway,
  type GroupLock,
  type Reader,
  type Role,
  type RoleAssignment,
  type Share,
  type Terminal,
  type UserGroup,
  type WalletPassInstance,
} from "@/lib/api"

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
} satisfies MistyisletPlaceResource

const group = {
  id: "group_service",
  placeId: place.id,
  name: "Service Personnel",
  kind: "Group",
  memberCount: 3,
  targetLabel: "3 members",
  description: "Service access",
  statusLabel: "Enabled",
  tone: "success",
} satisfies MistyisletGroupResource

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
} satisfies MistyisletUserResource

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
} satisfies MistyisletTeamResource

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

const viewer = {
  id: "usr_org_admin",
  email: "organization.admin@mistypass.local",
  role: "tenant_admin",
  tenant_id: "tenant_001",
} satisfies CurrentUser

const building = {
  id: place.id,
  tenant_id: "tenant_001",
  name: place.name,
  address: place.address,
  region: place.region,
  created_at: "2026-04-26T00:00:00Z",
} satisfies Building

const floor = {
  id: "floor_001",
  tenant_id: "tenant_001",
  building_id: place.id,
  name: "Level 1",
  created_at: "2026-04-26T00:00:00Z",
} satisfies Floor

const area = {
  id: "area_001",
  tenant_id: "tenant_001",
  building_id: place.id,
  floor_id: floor.id,
  name: "Lobby",
  created_at: "2026-04-26T00:00:00Z",
} satisfies Area

const gateway = {
  id: "gateway_001",
  tenant_id: "tenant_001",
  serial_number: "MP-GW-001",
  building_id: place.id,
  device_capacity: 4,
  status: "online",
  last_seen_at: "2026-04-26T10:00:00Z",
  bound_door_ids: [door.id],
  devices: [
    {
      id: "device_001",
      gateway_id: "gateway_001",
      serial_number: "RD-001",
      kind: "legacy_reader",
      source: "legacy_integration",
      protocol: "wiegand_26",
      status: "offline",
      last_seen_at: "2026-04-26T10:00:00Z",
    },
  ],
} satisfies Gateway

const controller = {
  id: gateway.id,
  resource_type: "Controller",
  tenant_id: "tenant_001",
  place_id: place.id,
  name: gateway.serial_number,
  description: "4-device controller",
  device_id: gateway.serial_number,
  token: gateway.serial_number,
  status: "online",
  configured: true,
  lock_ids: [door.id],
  last_seen_at: "2026-04-26T10:00:00Z",
  created_at: "2026-04-26T10:00:00Z",
  updated_at: "2026-04-26T10:00:00Z",
} satisfies Controller

const reader = {
  id: "reader_001",
  resource_type: "Reader",
  tenant_id: "tenant_001",
  place_id: place.id,
  controller_id: controller.id,
  name: "RD-001",
  device_id: "RD-001",
  token: "RD-001",
  model: "reader-pro-3.0",
  protocol: "osdp_v2",
  status: "online",
  configured: true,
  lock_ids: [door.id],
  last_seen_at: "2026-04-26T10:00:00Z",
  created_at: "2026-04-26T10:00:00Z",
  updated_at: "2026-04-26T10:00:00Z",
} satisfies Reader

const accessUser = {
  id: user.id,
  tenant_id: "tenant_001",
  building_id: place.id,
  name: user.name,
  email: user.email,
  role: "building_admin",
  status: "active",
  sync_source: "okta",
  created_at: "2026-04-26T00:00:00Z",
} satisfies AccessUser

const emptyEventSet = {
  id: "event_set_empty",
  created_at: "2026-04-26T00:00:00Z",
  status: "finished",
  events: [],
} satisfies EventSet

const emptySummary = {
  places: [],
  floors: [],
  zones: [],
  doors: [],
  hardware: [],
  events: [],
  users: [],
  credentials: [],
  groups: [],
  teams: [],
  teamMemberships: [],
  accessRights: [],
  partial: false,
} satisfies MistyisletResourceSummary

function resetResourceSummaryMocks() {
  vi.mocked(listPlaces).mockResolvedValue([])
  vi.mocked(listFloors).mockResolvedValue([])
  vi.mocked(listAreas).mockResolvedValue([])
  vi.mocked(listLocks).mockResolvedValue([])
  vi.mocked(listControllers).mockResolvedValue([])
  vi.mocked(listReaders).mockResolvedValue([])
  vi.mocked(listTerminals).mockResolvedValue([])
  vi.mocked(listGateways).mockResolvedValue([])
  vi.mocked(createEventSet).mockResolvedValue(emptyEventSet)
  vi.mocked(listAccessEvents).mockResolvedValue([])
  vi.mocked(listAccessUsers).mockResolvedValue([])
  vi.mocked(listCards).mockResolvedValue([])
  vi.mocked(listCardAssignments).mockResolvedValue([])
  vi.mocked(listWalletPasses).mockResolvedValue([])
  vi.mocked(listGroups).mockResolvedValue([])
  vi.mocked(listDoorGroups).mockResolvedValue([])
  vi.mocked(listGroupLocks).mockResolvedValue([])
  vi.mocked(listGroupZones).mockResolvedValue([])
  vi.mocked(listTeams).mockResolvedValue([])
  vi.mocked(listTeamMemberships).mockResolvedValue([])
  vi.mocked(listRoles).mockResolvedValue([])
  vi.mocked(listRoleAssignments).mockResolvedValue([])
  vi.mocked(listShares).mockResolvedValue([])
  vi.mocked(listAccessPolicies).mockResolvedValue([])
  vi.mocked(listTemporaryAccess).mockResolvedValue([])
}

beforeEach(() => {
  vi.resetAllMocks()
  resetResourceSummaryMocks()
})

describe("kisi resource-data summary adapter", () => {
  it("does not fall back to another place for an explicit out-of-scope place id", () => {
    const summary = {
      ...emptySummary,
      places: [place],
      doors: [
        {
          id: door.id,
          placeId: place.id,
          floorId: floor.id,
          floorName: floor.name,
          areaName: area.name,
          name: door.name,
          kind: "Turnstile",
          status: "online",
          gatewayId: door.gateway_id,
          gatewaySerial: gateway.serial_number,
          deviceCount: 1,
          description: "Lobby door",
        },
      ],
    } satisfies MistyisletResourceSummary

    const outOfScope = selectMistyisletPlaceContext(summary, "building_demo_002")
    expect(outOfScope.place).toBeNull()
    expect(outOfScope.doors).toEqual([])

    const slugAlias = selectMistyisletPlaceContext(summary, "sudirman-hub")
    expect(slugAlias.place?.id).toBe(place.id)
    expect(slugAlias.doors).toHaveLength(1)
  })

  it("maps reference places, floors, locks, events, users, groups, and credentials", async () => {
    const eventSet = {
      id: "event_set_001",
      created_at: "2026-04-26T10:00:00Z",
      status: "finished",
      events: [
        {
          uuid: "evt_reference_001",
          type: "access_denied",
          actor_name: user.name,
          object_type: "Lock",
          object_id: door.id,
          object_name: door.name,
          place_id: place.id,
          lock_id: door.id,
          success: false,
          result: "denied",
          detail: "credential missing",
          created_at: "2026-04-26T10:30:00Z",
        },
      ],
    } satisfies EventSet
    const userGroup = {
      id: group.id,
      tenant_id: "tenant_001",
      building_id: place.id,
      name: group.name,
      description: group.description,
      members: ["usr_001", "usr_002"],
      login_enabled: true,
      geofence_restriction_enabled: true,
      geofence_restriction_radius: 100,
      created_at: "2026-04-26T00:00:00Z",
      updated_at: "2026-04-26T00:00:00Z",
    } satisfies UserGroup
    const doorGroup = {
      id: "door_group_lobby",
      tenant_id: "tenant_001",
      name: "Lobby Access",
      created_at: "2026-04-26T00:00:00Z",
    } satisfies DoorGroup
    const groupLock = {
      id: "group_lock_lobby",
      tenant_id: "tenant_001",
      group_id: doorGroup.id,
      lock_id: door.id,
      created_at: "2026-04-26T00:00:00Z",
    } satisfies GroupLock
    const card = {
      id: "card_001",
      resource_type: "Card",
      tenant_id: "tenant_001",
      status: "activated",
      token: "card-token-001",
      provider: "desfire",
      credential_kind: "physical_card",
      template_id: "tpl_001",
      user_id: user.id,
      issued_at: "2026-04-26T00:00:00Z",
      created_at: "2026-04-26T00:00:00Z",
      updated_at: "2026-04-26T00:00:00Z",
    } satisfies Card
    const cardAssignment = {
      id: "card_assignment_001",
      resource_type: "CardAssignment",
      tenant_id: "tenant_001",
      status: "activated",
      card_id: card.id,
      card,
      assignee_type: "User",
      assignee_id: user.id,
      user_id: user.id,
      created_at: "2026-04-26T00:00:00Z",
      updated_at: "2026-04-26T00:00:00Z",
    } satisfies CardAssignment

    vi.mocked(listPlaces).mockResolvedValue([building])
    vi.mocked(listFloors).mockResolvedValue([floor])
    vi.mocked(listAreas).mockResolvedValue([area])
    vi.mocked(listLocks).mockResolvedValue([door])
    vi.mocked(listControllers).mockResolvedValue([controller])
    vi.mocked(listReaders).mockResolvedValue([reader])
    vi.mocked(createEventSet).mockResolvedValue(eventSet)
    vi.mocked(listAccessUsers).mockResolvedValue([accessUser])
    vi.mocked(listCards).mockResolvedValue([card])
    vi.mocked(listCardAssignments).mockResolvedValue([cardAssignment])
    vi.mocked(listGroups).mockResolvedValue([userGroup])
    vi.mocked(listDoorGroups).mockResolvedValue([doorGroup])
    vi.mocked(listGroupLocks).mockResolvedValue([groupLock])

    const summary = await loadMistyisletResourceSummary("access-token", viewer)

    expect(summary.places).toEqual([
      expect.objectContaining({
        id: place.id,
        name: place.name,
        doorCount: 1,
        gatewayCount: 1,
        offlineCount: 0,
        statusLabel: "All online",
      }),
    ])
    expect(summary.floors).toEqual([
      expect.objectContaining({
        id: floor.id,
        placeId: place.id,
        description: area.name,
        doorCount: 1,
        hardwareCount: 1,
      }),
    ])
    expect(summary.zones).toEqual([
      expect.objectContaining({
        id: area.id,
        floorName: floor.name,
        doorCount: 1,
      }),
    ])
    expect(summary.doors).toEqual([
      expect.objectContaining({
        id: door.id,
        floorName: floor.name,
        areaName: area.name,
        gatewaySerial: gateway.serial_number,
        deviceCount: 1,
      }),
    ])
    expect(summary.events).toEqual([
      expect.objectContaining({
        id: "evt_reference_001",
        placeId: place.id,
        object: door.name,
        action: "Unlock failed",
        user: user.name,
        statusLabel: "Review",
        tone: "warning",
      }),
    ])
    expect(summary.users).toEqual([
      expect.objectContaining({
        id: user.id,
        placeId: place.id,
        role: "Place Admin",
        statusLabel: "Active",
        sourceLabel: "Okta",
      }),
    ])
    expect(summary.credentials).toEqual([
      expect.objectContaining({
        id: card.id,
        user: user.name,
        userID: user.id,
        type: "Desfire Card",
        statusLabel: "Active",
        tone: "success",
      }),
    ])
    expect(summary.groups).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: group.id,
          kind: "Group",
          memberCount: 2,
          targetLabel: "2 members",
          geofenceRestrictionEnabled: true,
        }),
        expect.objectContaining({
          id: doorGroup.id,
          kind: "Group",
          placeId: place.id,
          targetLabel: door.name,
          doorIds: [door.id],
        }),
      ])
    )
    expect(summary.partial).toBe(false)
    expect(listGateways).not.toHaveBeenCalled()
    expect(listAccessEvents).not.toHaveBeenCalled()
    expect(listWalletPasses).not.toHaveBeenCalled()
    expect(listAccessPolicies).not.toHaveBeenCalled()
    expect(listTemporaryAccess).not.toHaveBeenCalled()
  })

  it("loads legacy gateways only when reference hardware calls fail", async () => {
    vi.mocked(listPlaces).mockResolvedValue([building])
    vi.mocked(listFloors).mockResolvedValue([floor])
    vi.mocked(listAreas).mockResolvedValue([area])
    vi.mocked(listLocks).mockResolvedValue([door])
    vi.mocked(listControllers).mockRejectedValue(new Error("controllers unavailable"))
    vi.mocked(listGateways).mockResolvedValue([gateway])

    const summary = await loadMistyisletResourceSummary("access-token", viewer)

    expect(listGateways).toHaveBeenCalledTimes(1)
    expect(summary.hardware).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: gateway.id,
          type: "Gateway",
          location: place.name,
        }),
      ])
    )
    expect(summary.places).toEqual([
      expect.objectContaining({
        id: place.id,
        gatewayCount: 1,
      }),
    ])
    expect(summary.partial).toBe(false)
  })

  it("falls back to legacy event and wallet credential resources when reference calls fail", async () => {
    const accessEvent = {
      id: "access_event_001",
      tenant_id: "tenant_001",
      building_id: place.id,
      area_id: area.id,
      type: "unlock",
      actor: user.email,
      door_id: door.id,
      gateway_id: gateway.id,
      result: "success",
      at: "2026-04-26T10:30:00Z",
    } satisfies AccessEvent
    const walletPass = {
      id: "wallet_pass_001",
      tenant_id: "tenant_001",
      provider: "apple",
      template_id: "template_001",
      target_type: "user",
      target_id: user.id,
      object_id: door.id,
      status: "issued",
      save_link: "https://example.test/pass",
      issued_at: "2026-04-26T00:00:00Z",
      created_by: "admin",
      updated_by: "admin",
      created_at: "2026-04-26T00:00:00Z",
      updated_at: "2026-04-26T00:00:00Z",
    } satisfies WalletPassInstance

    vi.mocked(listPlaces).mockResolvedValue([building])
    vi.mocked(listFloors).mockResolvedValue([floor])
    vi.mocked(listAreas).mockResolvedValue([area])
    vi.mocked(listLocks).mockResolvedValue([door])
    vi.mocked(listGateways).mockResolvedValue([gateway])
    vi.mocked(listAccessUsers).mockResolvedValue([accessUser])
    vi.mocked(createEventSet).mockRejectedValue(new Error("event_sets unavailable"))
    vi.mocked(listAccessEvents).mockResolvedValue([accessEvent])
    vi.mocked(listCards).mockRejectedValue(new Error("cards unavailable"))
    vi.mocked(listWalletPasses).mockResolvedValue([walletPass])

    const summary = await loadMistyisletResourceSummary("access-token", viewer)

    expect(summary.events).toEqual([
      expect.objectContaining({
        id: accessEvent.id,
        placeId: place.id,
        object: door.name,
        action: "Unlock successful",
        user: "rina",
        statusLabel: "Success",
        tone: "success",
      }),
    ])
    expect(summary.credentials).toEqual([
      expect.objectContaining({
        id: walletPass.id,
        user: user.name,
        userID: user.id,
        type: "Apple Mobile",
        statusLabel: "Pending",
        tone: "warning",
      }),
    ])
    expect(listAccessEvents).toHaveBeenCalledTimes(1)
    expect(listWalletPasses).toHaveBeenCalledTimes(1)
    expect(summary.partial).toBe(false)
  })
})

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

    const rows = buildMistyisletAccessRightsFromReferenceResources({
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

    const rows = buildMistyisletAccessRightsFromReferenceResources({
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

  it("preserves access-right review metadata for review queue filters", () => {
    const assignments: RoleAssignment[] = [
      {
        id: "ra_reviewed_expired",
        tenant_id: "tenant_001",
        role_id: "role_place_admin",
        applies_to_type: "Place",
        applies_to_id: place.id,
        assignee_type: "User",
        assignee_id: user.id,
        assignee_email: user.email,
        valid_until: "2000-01-01T00:00:00Z",
        reviewed_at: "2026-04-28T08:00:00Z",
        reviewed_by: "admin@example.com",
        created_at: "2026-04-26T00:00:00Z",
        updated_at: "2026-04-26T00:00:00Z",
      },
    ]
    const shares: Share[] = [
      {
        id: "share_unreviewed_expired",
        tenant_id: "tenant_001",
        email: "guest@example.com",
        group_id: group.id,
        role_id: "role_group_access",
        valid_until: "2000-01-01T00:00:00Z",
        status: "active",
        delivery_method: "email_qr",
        grantee_name: "Guest Visitor",
        created_at: "2026-04-26T00:00:00Z",
      },
    ]

    const rows = buildMistyisletAccessRightsFromReferenceResources({
      roleAssignments: assignments,
      shares,
      roles,
      places: [place],
      groups: [group],
      teams: [team],
      users: [user],
      doors: [door],
    })

    expect(rows).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "ra_reviewed_expired",
          sourceType: "role_assignment",
          statusLabel: "Expired",
          reviewedAt: "2026-04-28T08:00:00Z",
          reviewedBy: "admin@example.com",
          needsReview: false,
        }),
        expect.objectContaining({
          id: "share_unreviewed_expired",
          sourceType: "share",
          statusLabel: "Expired",
          needsReview: true,
        }),
      ])
    )
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

    const rows = buildMistyisletAccessRightsFromReferenceResources({
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

describe("kisi resource-data hardware mapper", () => {
  it("maps reference controllers, readers, and terminals to hardware rows", () => {
    const controller = {
      id: "controller_001",
      resource_type: "Controller",
      tenant_id: "tenant_001",
      place_id: place.id,
      name: "Controller A",
      device_id: "MP-GW-001",
      token: "MP-GW-001",
      status: "online",
      configured: true,
      lock_ids: [door.id],
      last_seen_at: "2026-04-26T10:00:00Z",
      created_at: "2026-04-26T10:00:00Z",
      updated_at: "2026-04-26T10:00:00Z",
    } satisfies Controller
    const reader = {
      id: "reader_001",
      resource_type: "Reader",
      tenant_id: "tenant_001",
      place_id: place.id,
      controller_id: controller.id,
      name: "Reader A",
      device_id: "RD-001",
      token: "RD-001",
      model: "reader-pro-3.0",
      protocol: "osdp_v2",
      status: "offline",
      configured: true,
      lock_ids: [door.id],
      last_seen_at: "2026-04-26T10:00:00Z",
      created_at: "2026-04-26T10:00:00Z",
      updated_at: "2026-04-26T10:00:00Z",
    } satisfies Reader
    const terminal = {
      id: "terminal_reader_001",
      resource_type: "Terminal",
      tenant_id: "tenant_001",
      created_at: "2026-04-26T10:00:00Z",
      updated_at: "2026-04-26T10:00:00Z",
      name: "Terminal A",
      description: "Reader terminal",
      place_id: place.id,
      place: {
        id: place.id,
        resource_type: "Place",
        name: place.name,
      },
      controller_id: controller.id,
      reader_id: reader.id,
      status: "online",
      last_seen_at: "2026-04-26T10:00:00Z",
    } satisfies Terminal

    const rows = buildMistyisletHardwareFromReferenceResources({
      controllers: [controller],
      readers: [reader],
      terminals: [terminal],
      doors: [door],
      places: [place],
    })

    expect(rows).toEqual([
      expect.objectContaining({
        id: controller.id,
        type: "Controller",
        gatewayId: controller.id,
        location: place.name,
        doorNames: [door.name],
        statusLabel: "Online",
        tone: "success",
      }),
      expect.objectContaining({
        id: reader.id,
        type: "Reader",
        gatewayId: controller.id,
        location: door.name,
        doorNames: [door.name],
        statusLabel: "Needs review",
        tone: "warning",
      }),
      expect.objectContaining({
        id: terminal.id,
        type: "Terminal",
        gatewayId: controller.id,
        location: place.name,
        doorNames: [],
        statusLabel: "Online",
        tone: "success",
      }),
    ])
  })

  it("uses gateway rows only when reference hardware is unavailable", () => {
    const gateway = {
      id: "gateway_001",
      tenant_id: "tenant_001",
      serial_number: "MP-GW-001",
      building_id: place.id,
      device_capacity: 4,
      status: "online",
      last_seen_at: "2026-04-26T10:00:00Z",
      bound_door_ids: [door.id],
      devices: [
        {
          id: "device_001",
          gateway_id: "gateway_001",
          serial_number: "RD-001",
          kind: "legacy_reader",
          source: "legacy_integration",
          protocol: "wiegand_26",
          status: "offline",
          last_seen_at: "2026-04-26T10:00:00Z",
        },
      ],
    } satisfies Gateway

    const rows = buildMistyisletHardwareFromReferenceResources({
      controllers: [],
      readers: [],
      terminals: [],
      gateways: [gateway],
      doors: [door],
      places: [place],
      useGatewayFallback: true,
    })

    expect(rows).toEqual([
      expect.objectContaining({
        id: gateway.id,
        type: "Gateway",
        location: place.name,
        doorNames: [door.name],
      }),
      expect.objectContaining({
        id: "device_001",
        type: "Reader",
        location: door.name,
        statusLabel: "Needs review",
      }),
    ])
  })
})
