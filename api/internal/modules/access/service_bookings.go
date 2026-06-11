package access

import (
	"strings"
	"time"
)

func (s *Service) ListBookableSpaces(tenantID string) []BookableSpace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]BookableSpace, 0, len(s.bookableSpaces))
	for i := range s.bookableSpaces {
		if filterTenantID != "" && s.bookableSpaces[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.bookableSpaces[i])
	}
	return items
}

func (s *Service) GetBookableSpace(tenantID, spaceID string) (BookableSpace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nextID := strings.TrimSpace(spaceID)
	for i := range s.bookableSpaces {
		if s.bookableSpaces[i].ID == nextID && (tenantID == "" || s.bookableSpaces[i].TenantID == tenantID) {
			return s.bookableSpaces[i], nil
		}
	}
	return BookableSpace{}, ErrSpaceNotFound
}

func validateSpaceType(t string) bool {
	switch t {
	case "meeting_room", "prayer_room", "phone_booth", "custom":
		return true
	}
	return false
}

func validateCapacityMode(m string) bool {
	switch m {
	case "single_occupancy", "limited_capacity", "unlimited":
		return true
	}
	return false
}

func (s *Service) CreateBookableSpace(in CreateBookableSpaceInput) (BookableSpace, error) {
	nextTenantID := strings.TrimSpace(in.TenantID)
	if nextTenantID == "" {
		return BookableSpace{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(in.Name)
	if nextName == "" {
		return BookableSpace{}, ErrSpaceNameRequired
	}
	nextType := strings.TrimSpace(strings.ToLower(in.SpaceType))
	if nextType == "" {
		return BookableSpace{}, ErrSpaceTypeRequired
	}
	if !validateSpaceType(nextType) {
		return BookableSpace{}, ErrInvalidSpaceType
	}
	nextCapMode := strings.TrimSpace(strings.ToLower(in.CapacityMode))
	if nextCapMode == "" {
		nextCapMode = "unlimited"
	}
	if !validateCapacityMode(nextCapMode) {
		return BookableSpace{}, ErrInvalidCapacityMode
	}

	id, err := accessID("bsp_")
	if err != nil {
		return BookableSpace{}, err
	}
	now := time.Now().UTC()
	record := BookableSpace{
		ID:              id,
		TenantID:        nextTenantID,
		Name:            nextName,
		Description:     strings.TrimSpace(in.Description),
		SpaceType:       nextType,
		CapacityMode:    nextCapMode,
		MaxCapacity:     in.MaxCapacity,
		LockID:          strings.TrimSpace(in.LockID),
		RequiresBooking: in.RequiresBooking,
		Enabled:         in.Enabled,
		PriceIDR:        in.PriceIDR,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	s.mu.Lock()
	s.bookableSpaces = append([]BookableSpace{record}, s.bookableSpaces...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return BookableSpace{}, err
	}
	s.mu.Unlock()
	return record, nil
}

func (s *Service) UpdateBookableSpace(tenantID, spaceID string, in UpdateBookableSpaceInput) (BookableSpace, error) {
	nextID := strings.TrimSpace(spaceID)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.bookableSpaces {
		if s.bookableSpaces[i].ID == nextID && (tenantID == "" || s.bookableSpaces[i].TenantID == tenantID) {
			if in.Name != nil {
				n := strings.TrimSpace(*in.Name)
				if n == "" {
					return BookableSpace{}, ErrSpaceNameRequired
				}
				s.bookableSpaces[i].Name = n
			}
			if in.Description != nil {
				s.bookableSpaces[i].Description = strings.TrimSpace(*in.Description)
			}
			if in.SpaceType != nil {
				t := strings.TrimSpace(strings.ToLower(*in.SpaceType))
				if !validateSpaceType(t) {
					return BookableSpace{}, ErrInvalidSpaceType
				}
				s.bookableSpaces[i].SpaceType = t
			}
			if in.CapacityMode != nil {
				m := strings.TrimSpace(strings.ToLower(*in.CapacityMode))
				if !validateCapacityMode(m) {
					return BookableSpace{}, ErrInvalidCapacityMode
				}
				s.bookableSpaces[i].CapacityMode = m
			}
			if in.MaxCapacity != nil {
				s.bookableSpaces[i].MaxCapacity = *in.MaxCapacity
			}
			if in.LockID != nil {
				s.bookableSpaces[i].LockID = strings.TrimSpace(*in.LockID)
			}
			if in.RequiresBooking != nil {
				s.bookableSpaces[i].RequiresBooking = *in.RequiresBooking
			}
			if in.Enabled != nil {
				s.bookableSpaces[i].Enabled = *in.Enabled
			}
			if in.PriceIDR != nil {
				s.bookableSpaces[i].PriceIDR = *in.PriceIDR
			}
			s.bookableSpaces[i].UpdatedAt = now
			if err := s.persistLocked(); err != nil {
				return BookableSpace{}, err
			}
			return s.bookableSpaces[i], nil
		}
	}
	return BookableSpace{}, ErrSpaceNotFound
}

func (s *Service) DeleteBookableSpace(tenantID, spaceID string) error {
	nextID := strings.TrimSpace(spaceID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.bookableSpaces {
		if s.bookableSpaces[i].ID == nextID && (tenantID == "" || s.bookableSpaces[i].TenantID == tenantID) {
			s.bookableSpaces = append(s.bookableSpaces[:i], s.bookableSpaces[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrSpaceNotFound
}

func (s *Service) GetBookableSpaceStatus(tenantID, spaceID string) (BookableSpaceStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextID := strings.TrimSpace(spaceID)
	var found *BookableSpace
	for i := range s.bookableSpaces {
		if s.bookableSpaces[i].ID == nextID && (tenantID == "" || s.bookableSpaces[i].TenantID == tenantID) {
			found = &s.bookableSpaces[i]
			break
		}
	}
	if found == nil {
		return BookableSpaceStatus{}, ErrSpaceNotFound
	}

	active := make([]Booking, 0)
	for i := range s.bookings {
		if s.bookings[i].SpaceID == nextID && s.bookings[i].TenantID == found.TenantID {
			switch s.bookings[i].Status {
			case "confirmed", "checked_in":
				active = append(active, s.bookings[i])
			}
		}
	}

	return BookableSpaceStatus{
		Space:          *found,
		ActiveBookings: active,
	}, nil
}

func (s *Service) ListBookings(tenantID string, spaceID string) []Booking {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filterTenantID := strings.TrimSpace(tenantID)
	filterSpaceID := strings.TrimSpace(spaceID)
	items := make([]Booking, 0, len(s.bookings))
	for i := range s.bookings {
		if filterTenantID != "" && s.bookings[i].TenantID != filterTenantID {
			continue
		}
		if filterSpaceID != "" && s.bookings[i].SpaceID != filterSpaceID {
			continue
		}
		items = append(items, s.bookings[i])
	}
	return items
}

func (s *Service) GetBooking(tenantID, bookingID string) (Booking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nextID := strings.TrimSpace(bookingID)
	for i := range s.bookings {
		if s.bookings[i].ID == nextID && (tenantID == "" || s.bookings[i].TenantID == tenantID) {
			return s.bookings[i], nil
		}
	}
	return Booking{}, ErrBookingNotFound
}

func (s *Service) CreateBooking(in CreateBookingInput) (Booking, error) {
	nextTenantID := strings.TrimSpace(in.TenantID)
	if nextTenantID == "" {
		return Booking{}, ErrTenantIDRequired
	}
	nextSpaceID := strings.TrimSpace(in.SpaceID)
	if nextSpaceID == "" {
		return Booking{}, ErrBookingSpaceIDRequired
	}
	nextUserID := strings.TrimSpace(in.UserID)
	if nextUserID == "" {
		return Booking{}, ErrBookingUserIDRequired
	}
	nextStart := strings.TrimSpace(in.StartTime)
	if nextStart == "" {
		return Booking{}, ErrBookingStartTimeRequired
	}
	nextEnd := strings.TrimSpace(in.EndTime)
	if nextEnd == "" {
		return Booking{}, ErrBookingEndTimeRequired
	}

	startT, err := time.Parse(time.RFC3339, nextStart)
	if err != nil {
		return Booking{}, ErrBookingStartTimeRequired
	}
	endT, err := time.Parse(time.RFC3339, nextEnd)
	if err != nil {
		return Booking{}, ErrBookingEndTimeRequired
	}
	if !endT.After(startT) {
		return Booking{}, ErrBookingEndBeforeStart
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// verify the space exists
	var space *BookableSpace
	for i := range s.bookableSpaces {
		if s.bookableSpaces[i].ID == nextSpaceID && s.bookableSpaces[i].TenantID == nextTenantID {
			space = &s.bookableSpaces[i]
			break
		}
	}
	if space == nil {
		return Booking{}, ErrSpaceNotFound
	}

	// Check capacity for overlapping bookings. pending_payment bookings hold
	// their slot while payment is in flight, so they count too.
	overlapping := 0
	for i := range s.bookings {
		b := &s.bookings[i]
		if b.SpaceID != nextSpaceID || b.TenantID != nextTenantID {
			continue
		}
		if b.Status != "confirmed" && b.Status != "checked_in" && b.Status != "pending_payment" {
			continue
		}
		bStart, err1 := time.Parse(time.RFC3339, b.StartTime)
		bEnd, err2 := time.Parse(time.RFC3339, b.EndTime)
		if err1 != nil || err2 != nil {
			continue
		}
		if startT.Before(bEnd) && endT.After(bStart) {
			overlapping++
		}
	}

	switch space.CapacityMode {
	case "single_occupancy":
		if overlapping > 0 {
			return Booking{}, ErrBookingTimeConflict
		}
	case "limited_capacity":
		if space.MaxCapacity > 0 && overlapping >= space.MaxCapacity {
			return Booking{}, ErrSpaceAtCapacity
		}
	}

	id, err := accessID("bkg_")
	if err != nil {
		return Booking{}, err
	}
	now := time.Now().UTC()
	record := Booking{
		ID:        id,
		TenantID:  nextTenantID,
		SpaceID:   nextSpaceID,
		UserID:    nextUserID,
		UserName:  strings.TrimSpace(in.UserName),
		Title:     strings.TrimSpace(in.Title),
		StartTime: nextStart,
		EndTime:   nextEnd,
		Status:    "confirmed",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if space.PriceIDR > 0 {
		// Priced spaces require payment before the booking is confirmed.
		record.Status = "pending_payment"
		record.PriceIDR = space.PriceIDR
		record.PaymentStatus = "pending"
	}

	s.bookings = append([]Booking{record}, s.bookings...)
	if err := s.persistLocked(); err != nil {
		return Booking{}, err
	}
	return record, nil
}

// AttachBookingPayment records the payment order id and hosted checkout URL on
// a pending_payment booking.
func (s *Service) AttachBookingPayment(tenantID, bookingID, orderID, paymentURL string) (Booking, error) {
	nextID := strings.TrimSpace(bookingID)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.bookings {
		if s.bookings[i].ID == nextID && (tenantID == "" || s.bookings[i].TenantID == tenantID) {
			s.bookings[i].PaymentOrderID = strings.TrimSpace(orderID)
			s.bookings[i].PaymentURL = strings.TrimSpace(paymentURL)
			s.bookings[i].UpdatedAt = now
			if err := s.persistLocked(); err != nil {
				return Booking{}, err
			}
			return s.bookings[i], nil
		}
	}
	return Booking{}, ErrBookingNotFound
}

// SettleBookingPaymentByOrderID applies a payment outcome reported by the
// provider webhook. Outcome "paid" confirms the booking; "expired" and "failed"
// cancel it. The webhook carries no tenant, so lookup is by the globally unique
// payment order id.
func (s *Service) SettleBookingPaymentByOrderID(orderID, outcome string) (Booking, error) {
	nextOrderID := strings.TrimSpace(orderID)
	if nextOrderID == "" {
		return Booking{}, ErrBookingNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.bookings {
		if s.bookings[i].PaymentOrderID != nextOrderID {
			continue
		}
		switch outcome {
		case "paid":
			s.bookings[i].Status = "confirmed"
			s.bookings[i].PaymentStatus = "paid"
			s.bookings[i].PaidAt = now.Format(time.RFC3339)
		case "expired", "failed":
			s.bookings[i].Status = "cancelled"
			s.bookings[i].PaymentStatus = outcome
		default:
			return s.bookings[i], nil
		}
		s.bookings[i].UpdatedAt = now
		if err := s.persistLocked(); err != nil {
			return Booking{}, err
		}
		return s.bookings[i], nil
	}
	return Booking{}, ErrBookingNotFound
}

func (s *Service) UpdateBooking(tenantID, bookingID string, in UpdateBookingInput) (Booking, error) {
	nextID := strings.TrimSpace(bookingID)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.bookings {
		if s.bookings[i].ID == nextID && (tenantID == "" || s.bookings[i].TenantID == tenantID) {
			if in.Title != nil {
				s.bookings[i].Title = strings.TrimSpace(*in.Title)
			}
			if in.StartTime != nil {
				st := strings.TrimSpace(*in.StartTime)
				if _, err := time.Parse(time.RFC3339, st); err != nil {
					return Booking{}, ErrBookingStartTimeRequired
				}
				s.bookings[i].StartTime = st
			}
			if in.EndTime != nil {
				et := strings.TrimSpace(*in.EndTime)
				if _, err := time.Parse(time.RFC3339, et); err != nil {
					return Booking{}, ErrBookingEndTimeRequired
				}
				s.bookings[i].EndTime = et
			}
			if in.Status != nil {
				st := strings.ToLower(strings.TrimSpace(*in.Status))
				switch st {
				case "confirmed", "checked_in", "completed", "cancelled", "no_show", "pending_payment":
				default:
					return Booking{}, ErrBookingStatusInvalid
				}
				s.bookings[i].Status = st
			}
			// validate start < end after updates
			startT, err1 := time.Parse(time.RFC3339, s.bookings[i].StartTime)
			endT, err2 := time.Parse(time.RFC3339, s.bookings[i].EndTime)
			if err1 == nil && err2 == nil && !endT.After(startT) {
				return Booking{}, ErrBookingEndBeforeStart
			}
			s.bookings[i].UpdatedAt = now
			if err := s.persistLocked(); err != nil {
				return Booking{}, err
			}
			return s.bookings[i], nil
		}
	}
	return Booking{}, ErrBookingNotFound
}

func (s *Service) DeleteBooking(tenantID, bookingID string) error {
	nextID := strings.TrimSpace(bookingID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.bookings {
		if s.bookings[i].ID == nextID && (tenantID == "" || s.bookings[i].TenantID == tenantID) {
			s.bookings = append(s.bookings[:i], s.bookings[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrBookingNotFound
}

func (s *Service) CheckInBooking(tenantID, bookingID string) (Booking, error) {
	nextID := strings.TrimSpace(bookingID)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.bookings {
		if s.bookings[i].ID == nextID && (tenantID == "" || s.bookings[i].TenantID == tenantID) {
			if s.bookings[i].Status == "checked_in" {
				return Booking{}, ErrBookingAlreadyCheckedIn
			}
			s.bookings[i].Status = "checked_in"
			s.bookings[i].CheckedInAt = now.Format(time.RFC3339)
			s.bookings[i].UpdatedAt = now

			// increment space occupancy
			for j := range s.bookableSpaces {
				if s.bookableSpaces[j].ID == s.bookings[i].SpaceID && s.bookableSpaces[j].TenantID == s.bookings[i].TenantID {
					s.bookableSpaces[j].CurrentOccupancy++
					s.bookableSpaces[j].UpdatedAt = now
					break
				}
			}

			if err := s.persistLocked(); err != nil {
				return Booking{}, err
			}
			return s.bookings[i], nil
		}
	}
	return Booking{}, ErrBookingNotFound
}

func (s *Service) CheckOutBooking(tenantID, bookingID string) (Booking, error) {
	nextID := strings.TrimSpace(bookingID)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.bookings {
		if s.bookings[i].ID == nextID && (tenantID == "" || s.bookings[i].TenantID == tenantID) {
			if s.bookings[i].Status != "checked_in" {
				return Booking{}, ErrBookingNotCheckedIn
			}
			s.bookings[i].Status = "completed"
			s.bookings[i].CheckedOutAt = now.Format(time.RFC3339)
			s.bookings[i].UpdatedAt = now

			// decrement space occupancy
			for j := range s.bookableSpaces {
				if s.bookableSpaces[j].ID == s.bookings[i].SpaceID && s.bookableSpaces[j].TenantID == s.bookings[i].TenantID {
					if s.bookableSpaces[j].CurrentOccupancy > 0 {
						s.bookableSpaces[j].CurrentOccupancy--
					}
					s.bookableSpaces[j].UpdatedAt = now
					break
				}
			}

			if err := s.persistLocked(); err != nil {
				return Booking{}, err
			}
			return s.bookings[i], nil
		}
	}
	return Booking{}, ErrBookingNotFound
}

// CleanupOldBookings removes completed/cancelled/no_show bookings older than the given cutoff.
// Returns the number of bookings removed.
func (s *Service) CleanupOldBookings(olderThan time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]Booking, 0, len(s.bookings))
	removed := 0
	for _, b := range s.bookings {
		terminal := b.Status == "completed" || b.Status == "cancelled" || b.Status == "no_show"
		if terminal && b.UpdatedAt.Before(olderThan) {
			removed++
			continue
		}
		kept = append(kept, b)
	}
	if removed == 0 {
		return 0
	}
	s.bookings = kept
	_ = s.persistLocked()
	return removed
}

// ---------------------------------------------------------------------------
// OAuth2 Client CRUD
// ---------------------------------------------------------------------------

// ListOAuth2Clients returns all OAuth2 clients for the given tenant.
func (s *Service) ListOAuth2Clients(tenantID string) []OAuth2Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]OAuth2Client, 0)
	for _, c := range s.oauth2Clients {
		if c.TenantID == tenantID {
			result = append(result, c)
		}
	}
	return result
}

// GetOAuth2Client returns a single client by ID (across all tenants).
func (s *Service) GetOAuth2Client(clientID string) (OAuth2Client, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.oauth2Clients {
		if c.ID == clientID {
			return c, true
		}
	}
	return OAuth2Client{}, false
}

// CreateOAuth2Client adds a new OAuth2 client and persists the state.
func (s *Service) CreateOAuth2Client(client OAuth2Client) error {
	if strings.TrimSpace(client.Name) == "" {
		return ErrOAuth2ClientNameRequired
	}
	if len(client.RedirectURIs) == 0 {
		return ErrOAuth2ClientRedirectURIRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.oauth2Clients = append(s.oauth2Clients, client)
	return s.persistLocked()
}

// UpdateOAuth2Client updates an existing OAuth2 client and persists the state.
func (s *Service) UpdateOAuth2Client(tenantID, clientID string, updated OAuth2Client) (OAuth2Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.oauth2Clients {
		if s.oauth2Clients[i].ID == clientID && s.oauth2Clients[i].TenantID == tenantID {
			s.oauth2Clients[i] = updated
			if err := s.persistLocked(); err != nil {
				return OAuth2Client{}, err
			}
			return s.oauth2Clients[i], nil
		}
	}
	return OAuth2Client{}, ErrOAuth2ClientNotFound
}

// DeleteOAuth2Client removes an OAuth2 client and persists the state.
func (s *Service) DeleteOAuth2Client(tenantID, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.oauth2Clients {
		if s.oauth2Clients[i].ID == clientID && s.oauth2Clients[i].TenantID == tenantID {
			s.oauth2Clients = append(s.oauth2Clients[:i], s.oauth2Clients[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrOAuth2ClientNotFound
}
