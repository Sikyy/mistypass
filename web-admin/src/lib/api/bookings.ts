import { request, requestItems, withTenantQuery, encodePathSegment } from "./core"

// --- Types ---

export type BookableSpace = {
  id: string
  tenant_id: string
  name: string
  description?: string
  space_type: "meeting_room" | "prayer_room" | "phone_booth" | "custom"
  capacity_mode: "single_occupancy" | "limited_capacity" | "unlimited"
  max_capacity: number
  current_occupancy: number
  lock_id?: string
  requires_booking: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export type Booking = {
  id: string
  tenant_id: string
  space_id: string
  user_id: string
  user_name: string
  title?: string
  start_time: string
  end_time: string
  status: "confirmed" | "checked_in" | "completed" | "cancelled" | "no_show"
  checked_in_at?: string
  checked_out_at?: string
  created_at: string
  updated_at: string
}

export type BookableSpaceStatus = {
  space: BookableSpace
  current_occupancy: number
  active_bookings: Booking[]
}

export type BookableSpaceUsage = {
  space_id: string
  total: number
  checked_in: number
  completed: number
  cancelled: number
  no_show: number
}

// --- Bookable Spaces ---

export async function listBookableSpaces(
  token: string | undefined,
  tenantID?: string
): Promise<BookableSpace[]> {
  return requestItems<BookableSpace>(withTenantQuery("/api/v1/bookable-spaces", tenantID), token)
}

export async function createBookableSpace(
  token: string | undefined,
  payload: {
    tenant_id: string
    name: string
    description?: string
    space_type: BookableSpace["space_type"]
    capacity_mode: BookableSpace["capacity_mode"]
    max_capacity?: number
    lock_id?: string
    requires_booking?: boolean
    enabled?: boolean
  }
): Promise<BookableSpace> {
  return request<BookableSpace>(
    "/api/v1/bookable-spaces",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function updateBookableSpace(
  token: string | undefined,
  spaceID: string,
  payload: {
    tenant_id?: string
    name?: string
    description?: string
    space_type?: BookableSpace["space_type"]
    capacity_mode?: BookableSpace["capacity_mode"]
    max_capacity?: number
    lock_id?: string
    requires_booking?: boolean
    enabled?: boolean
  }
): Promise<BookableSpace> {
  return request<BookableSpace>(
    `/api/v1/bookable-spaces/${encodePathSegment(spaceID)}`,
    { method: "PATCH", body: JSON.stringify(payload) },
    token
  )
}

export async function deleteBookableSpace(
  token: string | undefined,
  spaceID: string,
  tenantID?: string
): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/bookable-spaces/${encodePathSegment(spaceID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

export async function getBookableSpaceStatus(
  token: string | undefined,
  spaceID: string,
  tenantID?: string
): Promise<BookableSpaceStatus> {
  return request<BookableSpaceStatus>(
    withTenantQuery(`/api/v1/bookable-spaces/${encodePathSegment(spaceID)}/status`, tenantID),
    { method: "GET" },
    token
  )
}

// --- Bookings ---

export async function listBookings(
  token: string | undefined,
  tenantID?: string,
  spaceID?: string
): Promise<Booking[]> {
  let path = "/api/v1/bookings"
  const params = new URLSearchParams()
  if (tenantID) params.set("tenant_id", tenantID)
  if (spaceID) params.set("space_id", spaceID)
  const q = params.toString()
  if (q) path += "?" + q
  return requestItems<Booking>(path, token)
}

export async function createBooking(
  token: string | undefined,
  payload: {
    tenant_id: string
    space_id: string
    title?: string
    start_time: string
    end_time: string
  }
): Promise<Booking> {
  return request<Booking>(
    "/api/v1/bookings",
    { method: "POST", body: JSON.stringify(payload) },
    token
  )
}

export async function updateBooking(
  token: string | undefined,
  bookingID: string,
  payload: {
    tenant_id?: string
    title?: string
    start_time?: string
    end_time?: string
  }
): Promise<Booking> {
  return request<Booking>(
    `/api/v1/bookings/${encodePathSegment(bookingID)}`,
    { method: "PATCH", body: JSON.stringify(payload) },
    token
  )
}

export async function deleteBooking(
  token: string | undefined,
  bookingID: string,
  tenantID?: string
): Promise<void> {
  return request<void>(
    withTenantQuery(`/api/v1/bookings/${encodePathSegment(bookingID)}`, tenantID),
    { method: "DELETE" },
    token
  )
}

export async function checkInBooking(
  token: string | undefined,
  bookingID: string,
  tenantID?: string
): Promise<Booking> {
  return request<Booking>(
    withTenantQuery(`/api/v1/bookings/${encodePathSegment(bookingID)}/check-in`, tenantID),
    { method: "POST" },
    token
  )
}

export async function checkOutBooking(
  token: string | undefined,
  bookingID: string,
  tenantID?: string
): Promise<Booking> {
  return request<Booking>(
    withTenantQuery(`/api/v1/bookings/${encodePathSegment(bookingID)}/check-out`, tenantID),
    { method: "POST" },
    token
  )
}
