import { request, requestItems, withTenantQuery, encodePathSegment } from "./core"

export type Building = {
  id: string
  tenant_id: string
  name: string
  address: string
  region?: string
  status?: "active" | "archived"
  archived_at?: string
  created_at: string
}

export type Place = Building

export type Floor = {
  id: string
  tenant_id: string
  building_id: string
  name: string
  created_at: string
}

export type Area = {
  id: string
  tenant_id: string
  building_id: string
  floor_id: string
  name: string
  created_at: string
}

export type SpaceActionResult = {
  id: string
  resource_type: "PlaceAction" | "LockAction"
  tenant_id: string
  place_id?: string
  lock_id?: string
  action: "unlock" | "lock_down" | "cancel_lockdown"
  status: string
  lock_count?: number
  created_at: string
}

// --- Buildings / Places ---

export async function listBuildings(token: string | undefined, tenantID?: string): Promise<Building[]> {
  return requestItems<Building>(withTenantQuery("/api/v1/places", tenantID), token)
}

export async function listPlaces(token: string | undefined, tenantID?: string): Promise<Place[]> {
  return requestItems<Place>(withTenantQuery("/api/v1/places", tenantID), token)
}

export async function createPlace(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    address?: string
    region?: string
  }
): Promise<Place> {
  return request<Place>(
    "/api/v1/places",
    {
      method: "POST",
      body: JSON.stringify({ place: payload }),
    },
    token
  )
}

export async function getPlace(token: string | undefined, placeID: string, tenantID?: string): Promise<Place> {
  return request<Place>(
    withTenantQuery(`/api/v1/places/${encodePathSegment(placeID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function updatePlace(
  token: string | undefined,
  placeID: string,
  payload: {
    tenant_id?: string
    name?: string
    address?: string
    region?: string
  }
): Promise<Place> {
  return request<Place>(
    `/api/v1/places/${encodePathSegment(placeID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ place: payload }),
    },
    token
  )
}

export async function deletePlace(token: string | undefined, placeID: string, tenantID?: string): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/places/${encodePathSegment(placeID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

export async function lockDownPlace(token: string | undefined, placeID: string, tenantID?: string): Promise<SpaceActionResult> {
  return request<SpaceActionResult>(
    withTenantQuery(`/api/v1/places/${encodePathSegment(placeID)}/lock_down`, tenantID),
    { method: "POST" },
    token
  )
}

export async function cancelPlaceLockdown(token: string | undefined, placeID: string, tenantID?: string): Promise<SpaceActionResult> {
  return request<SpaceActionResult>(
    withTenantQuery(`/api/v1/places/${encodePathSegment(placeID)}/cancel_lockdown`, tenantID),
    { method: "POST" },
    token
  )
}

export async function createBuilding(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    address?: string
    region?: string
  }
): Promise<Building> {
  return request<Building>(
    "/api/v1/places",
    {
      method: "POST",
      body: JSON.stringify({ place: payload }),
    },
    token
  )
}

export async function favoritePlace(token: string | undefined, placeID: string): Promise<{ place_id: string; favorited: boolean }> {
  return request(`/api/v1/places/${encodePathSegment(placeID)}/favorite`, { method: "POST" }, token)
}

export async function unfavoritePlace(token: string | undefined, placeID: string): Promise<{ place_id: string; favorited: boolean }> {
  return request(`/api/v1/places/${encodePathSegment(placeID)}/unfavorite`, { method: "POST" }, token)
}

// --- Floors ---

export async function listFloors(token: string | undefined, tenantID?: string): Promise<Floor[]> {
  return requestItems<Floor>(withTenantQuery("/api/v1/floors", tenantID), token)
}

export async function createFloor(
  token: string | undefined,
  payload: {
    tenant_id: string
    building_id: string
    place_id?: string
    name: string
  }
): Promise<Floor> {
  return request<Floor>(
    "/api/v1/floors",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function getFloor(token: string | undefined, floorID: string, tenantID?: string): Promise<Floor> {
  return request<Floor>(
    withTenantQuery(`/api/v1/floors/${encodePathSegment(floorID)}`, tenantID),
    { method: "GET" },
    token
  )
}

export async function updateFloor(
  token: string | undefined,
  floorID: string,
  payload: {
    tenant_id?: string
    building_id?: string
    place_id?: string
    name?: string
  }
): Promise<Floor> {
  return request<Floor>(
    `/api/v1/floors/${encodePathSegment(floorID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function deleteFloor(token: string | undefined, floorID: string, tenantID?: string): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/floors/${encodePathSegment(floorID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

// --- Areas ---

export async function listAreas(token: string | undefined, tenantID?: string): Promise<Area[]> {
  return requestItems<Area>(withTenantQuery("/api/v1/areas", tenantID), token)
}

export async function createArea(
  token: string | undefined,
  payload: {
    tenant_id: string
    building_id: string
    floor_id: string
    name: string
  }
): Promise<Area> {
  return request<Area>(
    "/api/v1/areas",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateArea(
  token: string | undefined,
  areaID: string,
  payload: {
    tenant_id?: string
    building_id?: string
    place_id?: string
    floor_id?: string
    name?: string
  }
): Promise<Area> {
  return request<Area>(
    `/api/v1/areas/${encodePathSegment(areaID)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}
