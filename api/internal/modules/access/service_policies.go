package access

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

func (s *Service) ListPolicies(tenantID string) []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Policy, 0, len(s.policies))
	for i := range s.policies {
		if filterTenantID != "" && s.policies[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.policies[i])
	}
	return items
}

func (s *Service) CreatePolicy(
	tenantID, name, scopeType, buildingID, areaID, doorID, schedule string,
	members int,
	status string,
) (Policy, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Policy{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Policy{}, ErrPolicyNameRequired
	}

	nextScopeType, err := normalizeScopeType(scopeType)
	if err != nil {
		return Policy{}, err
	}

	nextStatus, err := normalizePolicyStatus(status)
	if err != nil {
		return Policy{}, err
	}

	nextMembers := members
	if nextMembers < 0 {
		nextMembers = 0
	}

	id, err := accessID("plc_")
	if err != nil {
		return Policy{}, err
	}

	record := Policy{
		ID:         id,
		TenantID:   nextTenantID,
		Name:       nextName,
		ScopeType:  nextScopeType,
		BuildingID: strings.TrimSpace(buildingID),
		AreaID:     strings.TrimSpace(areaID),
		DoorID:     strings.TrimSpace(doorID),
		Schedule:   strings.TrimSpace(schedule),
		Members:    nextMembers,
		Status:     nextStatus,
		UpdatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.policies = append([]Policy{record}, s.policies...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Policy{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdatePolicy(
	tenantID, policyID, name, scopeType, buildingID, areaID, doorID, schedule string,
	members int,
	status string,
) (Policy, error) {
	nextID := strings.TrimSpace(policyID)
	if nextID == "" {
		return Policy{}, ErrPolicyNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Policy{}, ErrPolicyNameRequired
	}

	nextScopeType, err := normalizeScopeType(scopeType)
	if err != nil {
		return Policy{}, err
	}
	nextStatus, err := normalizePolicyStatus(status)
	if err != nil {
		return Policy{}, err
	}

	nextMembers := members
	if nextMembers < 0 {
		nextMembers = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.policies {
		if s.policies[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.policies[i].TenantID != filterTenantID {
			return Policy{}, ErrPolicyNotFound
		}

		s.policies[i].Name = nextName
		s.policies[i].ScopeType = nextScopeType
		s.policies[i].BuildingID = strings.TrimSpace(buildingID)
		s.policies[i].AreaID = strings.TrimSpace(areaID)
		s.policies[i].DoorID = strings.TrimSpace(doorID)
		s.policies[i].Schedule = strings.TrimSpace(schedule)
		s.policies[i].Members = nextMembers
		s.policies[i].Status = nextStatus
		s.policies[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Policy{}, err
		}
		return s.policies[i], nil
	}

	return Policy{}, ErrPolicyNotFound
}

func (s *Service) ListTemporaryAccess(tenantID string) []TemporaryAccess {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]TemporaryAccess, 0, len(s.temporaryAccess))
	for i := range s.temporaryAccess {
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.temporaryAccess[i])
	}
	return items
}

func (s *Service) GetTemporaryAccess(tenantID, accessID string) (TemporaryAccess, error) {
	nextID := strings.TrimSpace(accessID)
	if nextID == "" {
		return TemporaryAccess{}, ErrTemporaryAccessNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.temporaryAccess {
		if s.temporaryAccess[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			return TemporaryAccess{}, ErrTemporaryAccessNotFound
		}
		return s.temporaryAccess[i], nil
	}
	return TemporaryAccess{}, ErrTemporaryAccessNotFound
}

func (s *Service) CreateTemporaryAccess(
	tenantID, scopeType, buildingID, areaID, doorID, deliveryMethod,
	granteeName, granteeGender, granteePhone, granteeEmail, mobileModel, passType, validUntil,
	authorizedByID, authorizedByEmail, authorizedByRole string,
) (TemporaryAccess, error) {
	return s.CreateTemporaryAccessWithInput(TemporaryAccessInput{
		TenantID:          tenantID,
		ScopeType:         scopeType,
		BuildingID:        buildingID,
		AreaID:            areaID,
		DoorID:            doorID,
		DeliveryMethod:    deliveryMethod,
		GranteeName:       granteeName,
		GranteeGender:     granteeGender,
		GranteePhone:      granteePhone,
		GranteeEmail:      granteeEmail,
		MobileModel:       mobileModel,
		PassType:          passType,
		ValidUntil:        validUntil,
		AuthorizedByID:    authorizedByID,
		AuthorizedByEmail: authorizedByEmail,
		AuthorizedByRole:  authorizedByRole,
	})
}

func (s *Service) CreateTemporaryAccessWithInput(input TemporaryAccessInput) (TemporaryAccess, error) {
	resolved, err := normalizeTemporaryAccessInput(input)
	if err != nil {
		return TemporaryAccess{}, err
	}

	id, err := accessID("tmp_")
	if err != nil {
		return TemporaryAccess{}, err
	}

	now := time.Now().UTC()
	record := TemporaryAccess{
		ID:                id,
		TenantID:          resolved.TenantID,
		ScopeType:         resolved.ScopeType,
		BuildingID:        resolved.BuildingID,
		AreaID:            resolved.AreaID,
		DoorID:            resolved.DoorID,
		GroupID:           resolved.GroupID,
		RoleID:            resolved.RoleID,
		DeliveryMethod:    resolved.DeliveryMethod,
		GranteeName:       resolved.GranteeName,
		GranteeGender:     resolved.GranteeGender,
		GranteePhone:      resolved.GranteePhone,
		GranteeEmail:      resolved.GranteeEmail,
		MobileModel:       resolved.MobileModel,
		PassType:          resolved.PassType,
		ValidFrom:         resolved.ValidFrom,
		ValidUntil:        resolved.ValidUntil,
		AuthorizedByID:    resolved.AuthorizedByID,
		AuthorizedByEmail: resolved.AuthorizedByEmail,
		AuthorizedByRole:  resolved.AuthorizedByRole,
		AuthorizedAt:      now,
		ReviewedAt:        strings.TrimSpace(resolved.ReviewedAt),
		ReviewedBy:        strings.TrimSpace(resolved.ReviewedBy),
		CreatedAt:         now,
	}

	s.mu.Lock()
	s.temporaryAccess = append([]TemporaryAccess{record}, s.temporaryAccess...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return TemporaryAccess{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateTemporaryAccess(tenantID, accessID string, input TemporaryAccessInput) (TemporaryAccess, error) {
	nextID := strings.TrimSpace(accessID)
	if nextID == "" {
		return TemporaryAccess{}, ErrTemporaryAccessNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)
	input.TenantID = firstNonEmpty(input.TenantID, filterTenantID)
	resolved, err := normalizeTemporaryAccessInput(input)
	if err != nil {
		return TemporaryAccess{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.temporaryAccess {
		if s.temporaryAccess[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			return TemporaryAccess{}, ErrTemporaryAccessNotFound
		}
		authorizedAt := time.Now().UTC()
		if input.KeepAuthorizedTime {
			authorizedAt = s.temporaryAccess[i].AuthorizedAt
		}
		createdAt := s.temporaryAccess[i].CreatedAt
		s.temporaryAccess[i] = TemporaryAccess{
			ID:                nextID,
			TenantID:          resolved.TenantID,
			ScopeType:         resolved.ScopeType,
			BuildingID:        resolved.BuildingID,
			AreaID:            resolved.AreaID,
			DoorID:            resolved.DoorID,
			GroupID:           resolved.GroupID,
			RoleID:            resolved.RoleID,
			DeliveryMethod:    resolved.DeliveryMethod,
			GranteeName:       resolved.GranteeName,
			GranteeGender:     resolved.GranteeGender,
			GranteePhone:      resolved.GranteePhone,
			GranteeEmail:      resolved.GranteeEmail,
			MobileModel:       resolved.MobileModel,
			PassType:          resolved.PassType,
			ValidFrom:         resolved.ValidFrom,
			ValidUntil:        resolved.ValidUntil,
			AuthorizedByID:    resolved.AuthorizedByID,
			AuthorizedByEmail: resolved.AuthorizedByEmail,
			AuthorizedByRole:  resolved.AuthorizedByRole,
			AuthorizedAt:      authorizedAt,
			ReviewedAt:        strings.TrimSpace(resolved.ReviewedAt),
			ReviewedBy:        strings.TrimSpace(resolved.ReviewedBy),
			CreatedAt:         createdAt,
		}
		if err := s.persistLocked(); err != nil {
			return TemporaryAccess{}, err
		}
		return s.temporaryAccess[i], nil
	}
	return TemporaryAccess{}, ErrTemporaryAccessNotFound
}

func (s *Service) DeleteTemporaryAccess(tenantID, accessID string) error {
	nextID := strings.TrimSpace(accessID)
	if nextID == "" {
		return ErrTemporaryAccessNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.temporaryAccess {
		if s.temporaryAccess[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			return ErrTemporaryAccessNotFound
		}
		original := cloneTemporaryAccess(s.temporaryAccess)
		s.temporaryAccess = append(s.temporaryAccess[:i], s.temporaryAccess[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.temporaryAccess = original
			return err
		}
		return nil
	}
	return ErrTemporaryAccessNotFound
}

func normalizeTemporaryAccessInput(input TemporaryAccessInput) (TemporaryAccessInput, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return TemporaryAccessInput{}, ErrTenantIDRequired
	}

	nextScopeType, err := normalizeScopeType(input.ScopeType)
	if err != nil {
		return TemporaryAccessInput{}, err
	}

	nextMethod, err := normalizeDeliveryMethod(input.DeliveryMethod)
	if err != nil {
		return TemporaryAccessInput{}, err
	}

	nextGranteeName := strings.TrimSpace(input.GranteeName)
	if nextGranteeName == "" {
		return TemporaryAccessInput{}, ErrGranteeNameRequired
	}
	nextGranteePhone := strings.TrimSpace(input.GranteePhone)
	if nextGranteePhone == "" {
		return TemporaryAccessInput{}, ErrGranteePhoneRequired
	}
	nextGranteeEmail := strings.TrimSpace(input.GranteeEmail)
	if nextGranteeEmail == "" {
		return TemporaryAccessInput{}, ErrGranteeEmailRequired
	}

	nextValidUntil := strings.TrimSpace(input.ValidUntil)
	if nextValidUntil == "" {
		return TemporaryAccessInput{}, ErrValidUntilRequired
	}

	return TemporaryAccessInput{
		TenantID:          nextTenantID,
		ScopeType:         nextScopeType,
		BuildingID:        strings.TrimSpace(input.BuildingID),
		AreaID:            strings.TrimSpace(input.AreaID),
		DoorID:            strings.TrimSpace(input.DoorID),
		GroupID:           strings.TrimSpace(input.GroupID),
		RoleID:            firstNonEmpty(input.RoleID, "role_group_access"),
		DeliveryMethod:    nextMethod,
		GranteeName:       nextGranteeName,
		GranteeGender:     strings.TrimSpace(input.GranteeGender),
		GranteePhone:      nextGranteePhone,
		GranteeEmail:      nextGranteeEmail,
		MobileModel:       strings.TrimSpace(input.MobileModel),
		PassType:          strings.TrimSpace(input.PassType),
		ValidFrom:         strings.TrimSpace(input.ValidFrom),
		ValidUntil:        nextValidUntil,
		AuthorizedByID:    strings.TrimSpace(input.AuthorizedByID),
		AuthorizedByEmail: normalizeEmail(input.AuthorizedByEmail),
		AuthorizedByRole:  strings.TrimSpace(input.AuthorizedByRole),
		ReviewedAt:        strings.TrimSpace(input.ReviewedAt),
		ReviewedBy:        strings.TrimSpace(input.ReviewedBy),
	}, nil
}

func (s *Service) ListVisitorPasses(tenantID string) []VisitorPass {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]VisitorPass, 0, len(s.visitorPasses))
	for i := range s.visitorPasses {
		if filterTenantID != "" && s.visitorPasses[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.visitorPasses[i])
	}
	return items
}

func (s *Service) CreateVisitorPass(tenantID, buildingID, host, visitor, deliveryMethod, expiresAt string) (VisitorPass, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return VisitorPass{}, ErrTenantIDRequired
	}

	nextHost := strings.TrimSpace(host)
	if nextHost == "" {
		return VisitorPass{}, ErrHostRequired
	}

	nextVisitor := strings.TrimSpace(visitor)
	if nextVisitor == "" {
		return VisitorPass{}, ErrVisitorRequired
	}

	nextMethod, err := normalizeDeliveryMethod(deliveryMethod)
	if err != nil {
		return VisitorPass{}, err
	}

	nextExpiresAt := strings.TrimSpace(expiresAt)
	if nextExpiresAt == "" {
		return VisitorPass{}, ErrExpiresAtRequired
	}

	id, err := accessID("vst_")
	if err != nil {
		return VisitorPass{}, err
	}

	record := VisitorPass{
		ID:             id,
		TenantID:       nextTenantID,
		BuildingID:     strings.TrimSpace(buildingID),
		Host:           nextHost,
		Visitor:        nextVisitor,
		DeliveryMethod: nextMethod,
		ExpiresAt:      nextExpiresAt,
		CreatedAt:      time.Now().UTC(),
	}

	s.mu.Lock()
	s.visitorPasses = append([]VisitorPass{record}, s.visitorPasses...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return VisitorPass{}, err
	}
	s.mu.Unlock()

	return record, nil
}

// --- Guest CRUD ---

var ErrGuestNotFound = errors.New("guest not found")
var ErrGuestNameRequired = errors.New("guest name is required")
var ErrGuestPhoneRequired = errors.New("guest phone is required")
var ErrGuestHostRequired = errors.New("guest host name is required")
var ErrGuestStatusInvalid = errors.New("guest status must be expected, checked_in, checked_out, or cancelled")
var ErrGuestIDDocumentTypeInvalid = errors.New("id_document_type must be KTP, KITAS, ITAS, SIM, PASSPORT, or OTHER")

var ErrSpaceNotFound = errors.New("bookable space not found")
var ErrSpaceNameRequired = errors.New("space name is required")
var ErrSpaceTypeRequired = errors.New("space_type is required")
var ErrInvalidSpaceType = errors.New("space_type must be meeting_room, prayer_room, phone_booth, or custom")
var ErrInvalidCapacityMode = errors.New("capacity_mode must be single_occupancy, limited_capacity, or unlimited")
var ErrBookingNotFound = errors.New("booking not found")
var ErrBookingUserIDRequired = errors.New("booking user_id is required")
var ErrBookingSpaceIDRequired = errors.New("booking space_id is required")
var ErrBookingStartTimeRequired = errors.New("booking start_time is required")
var ErrBookingEndTimeRequired = errors.New("booking end_time is required")
var ErrBookingEndBeforeStart = errors.New("booking end_time must be after start_time")
var ErrSpaceAtCapacity = errors.New("bookable space is at capacity")
var ErrBookingTimeConflict = errors.New("booking conflicts with an existing booking")
var ErrBookingAlreadyCheckedIn = errors.New("booking is already checked in")
var ErrBookingNotCheckedIn = errors.New("booking is not checked in")
var ErrBookingStatusInvalid = errors.New("invalid booking status")

func (s *Service) ListGuests(tenantID string) []Guest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Guest, 0, len(s.guests))
	for i := range s.guests {
		if filterTenantID != "" && s.guests[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.guests[i])
	}
	return items
}

func (s *Service) GetGuest(tenantID, guestID string) (Guest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nextID := strings.TrimSpace(guestID)
	for i := range s.guests {
		if s.guests[i].ID == nextID && (tenantID == "" || s.guests[i].TenantID == tenantID) {
			return s.guests[i], nil
		}
	}
	return Guest{}, ErrGuestNotFound
}

func (s *Service) GetGuestByAccessToken(tenantID, token string) (Guest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nextToken := strings.TrimSpace(token)
	if nextToken == "" {
		return Guest{}, false
	}
	now := time.Now().UTC()
	for i := range s.guests {
		g := &s.guests[i]
		if g.AccessToken != nextToken {
			continue
		}
		if tenantID != "" && g.TenantID != tenantID {
			continue
		}
		if g.Status == "cancelled" || g.Status == "checked_out" {
			return Guest{}, false
		}
		if g.AccessTokenExpiresAt != "" {
			exp, err := time.Parse(time.RFC3339, g.AccessTokenExpiresAt)
			if err == nil && now.After(exp) {
				return Guest{}, false
			}
		}
		return *g, true
	}
	return Guest{}, false
}

type CreateGuestInput struct {
	TenantID         string
	BuildingID       string
	Name             string
	Email            string
	Phone            string
	Company          string
	Purpose          string
	HostName         string
	HostEmail        string
	HostPhone        string
	IDDocumentType   string
	IDDocumentNumber string
	ExpectedAt       string
	NotifyHost       bool
	DoorIDs          []string
	AccessTTLHours   int
}

func (s *Service) CreateGuest(in CreateGuestInput) (Guest, error) {
	nextTenantID := strings.TrimSpace(in.TenantID)
	if nextTenantID == "" {
		return Guest{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(in.Name)
	if nextName == "" {
		return Guest{}, ErrGuestNameRequired
	}
	nextPhone := strings.TrimSpace(in.Phone)
	if nextPhone == "" {
		return Guest{}, ErrGuestPhoneRequired
	}
	nextHostName := strings.TrimSpace(in.HostName)
	if nextHostName == "" {
		return Guest{}, ErrGuestHostRequired
	}
	docType := strings.TrimSpace(strings.ToUpper(in.IDDocumentType))
	if docType != "" {
		switch docType {
		// Accept the union of document types the mobile/web clients offer (the
		// pickers list KTP/SIM/Passport/Other; KITAS/ITAS are Indonesian permits).
		case "KTP", "KITAS", "ITAS", "SIM", "PASSPORT", "OTHER":
		default:
			return Guest{}, ErrGuestIDDocumentTypeInvalid
		}
	}

	id, err := accessID("gst_")
	if err != nil {
		return Guest{}, err
	}
	token, err := accessID("gqr_")
	if err != nil {
		return Guest{}, err
	}

	ttlHours := in.AccessTTLHours
	if ttlHours <= 0 {
		ttlHours = 24
	}
	if ttlHours > 72 {
		ttlHours = 72
	}

	now := time.Now().UTC()
	tokenExpiry := now.Add(time.Duration(ttlHours) * time.Hour)

	var doorIDs []string
	for _, d := range in.DoorIDs {
		if trimmed := strings.TrimSpace(d); trimmed != "" {
			doorIDs = append(doorIDs, trimmed)
		}
	}

	record := Guest{
		ID:                   id,
		TenantID:             nextTenantID,
		BuildingID:           strings.TrimSpace(in.BuildingID),
		Name:                 nextName,
		Email:                strings.TrimSpace(in.Email),
		Phone:                nextPhone,
		Company:              strings.TrimSpace(in.Company),
		Purpose:              strings.TrimSpace(in.Purpose),
		HostName:             nextHostName,
		HostEmail:            strings.TrimSpace(in.HostEmail),
		HostPhone:            strings.TrimSpace(in.HostPhone),
		IDDocumentType:       docType,
		IDDocumentNumber:     strings.TrimSpace(in.IDDocumentNumber),
		Status:               "expected",
		ExpectedAt:           strings.TrimSpace(in.ExpectedAt),
		NotifyHost:           in.NotifyHost,
		AccessToken:          token,
		AccessTokenExpiresAt: tokenExpiry.Format(time.RFC3339),
		DoorIDs:              doorIDs,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	s.mu.Lock()
	s.guests = append([]Guest{record}, s.guests...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Guest{}, err
	}
	s.mu.Unlock()
	return record, nil
}

func (s *Service) UpdateGuestStatus(tenantID, guestID, status string) (Guest, error) {
	nextID := strings.TrimSpace(guestID)
	nextStatus := strings.ToLower(strings.TrimSpace(status))
	switch nextStatus {
	case "expected", "checked_in", "checked_out", "cancelled":
	default:
		return Guest{}, ErrGuestStatusInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.guests {
		if s.guests[i].ID == nextID && (tenantID == "" || s.guests[i].TenantID == tenantID) {
			s.guests[i].Status = nextStatus
			s.guests[i].UpdatedAt = now
			if nextStatus == "checked_in" && s.guests[i].CheckedInAt == "" {
				s.guests[i].CheckedInAt = now.Format(time.RFC3339)
			}
			if nextStatus == "checked_out" && s.guests[i].CheckedOutAt == "" {
				s.guests[i].CheckedOutAt = now.Format(time.RFC3339)
			}
			if err := s.persistLocked(); err != nil {
				return Guest{}, err
			}
			return s.guests[i], nil
		}
	}
	return Guest{}, ErrGuestNotFound
}

func (s *Service) DeleteGuest(tenantID, guestID string) error {
	nextID := strings.TrimSpace(guestID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.guests {
		if s.guests[i].ID == nextID && (tenantID == "" || s.guests[i].TenantID == tenantID) {
			s.guests = append(s.guests[:i], s.guests[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrGuestNotFound
}

// --- Elevators CRUD ---

var ErrElevatorNotFound = errors.New("elevator not found")

func (s *Service) ListElevators(tenantID string) []Elevator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Elevator, 0)
	for _, e := range s.elevators {
		if tenantID != "" && e.TenantID != tenantID { continue }
		items = append(items, e)
	}
	return items
}

func (s *Service) GetElevator(tenantID, id string) (Elevator, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.elevators {
		if e.ID == id && (tenantID == "" || e.TenantID == tenantID) { return e, nil }
	}
	return Elevator{}, ErrElevatorNotFound
}

func (s *Service) CreateElevator(tenantID, placeID, name, description string) (Elevator, error) {
	if strings.TrimSpace(name) == "" { return Elevator{}, errors.New("elevator name is required") }
	id, err := accessID("elv_")
	if err != nil { return Elevator{}, err }
	now := time.Now().UTC()
	e := Elevator{ID: id, TenantID: tenantID, PlaceID: placeID, Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	s.elevators = append([]Elevator{e}, s.elevators...)
	_ = s.persistLocked()
	s.mu.Unlock()
	return e, nil
}

func (s *Service) UpdateElevator(tenantID, id string, name, description *string) (Elevator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.elevators {
		if s.elevators[i].ID == id && (tenantID == "" || s.elevators[i].TenantID == tenantID) {
			if name != nil { s.elevators[i].Name = strings.TrimSpace(*name) }
			if description != nil { s.elevators[i].Description = strings.TrimSpace(*description) }
			s.elevators[i].UpdatedAt = time.Now().UTC()
			_ = s.persistLocked()
			return s.elevators[i], nil
		}
	}
	return Elevator{}, ErrElevatorNotFound
}

func (s *Service) DeleteElevator(tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.elevators {
		if s.elevators[i].ID == id && (tenantID == "" || s.elevators[i].TenantID == tenantID) {
			s.elevators = append(s.elevators[:i], s.elevators[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrElevatorNotFound
}

// --- Elevator Stops CRUD ---

var ErrElevatorStopNotFound = errors.New("elevator stop not found")

func (s *Service) ListElevatorStops(tenantID, elevatorID string) []ElevatorStop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ElevatorStop, 0)
	for _, e := range s.elevatorStops {
		if tenantID != "" && e.TenantID != tenantID { continue }
		if elevatorID != "" && e.ElevatorID != elevatorID { continue }
		items = append(items, e)
	}
	return items
}

func (s *Service) GetElevatorStop(tenantID, id string) (ElevatorStop, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.elevatorStops {
		if e.ID == id && (tenantID == "" || e.TenantID == tenantID) { return e, nil }
	}
	return ElevatorStop{}, ErrElevatorStopNotFound
}

func (s *Service) CreateElevatorStop(tenantID, elevatorID, floorID, name string) (ElevatorStop, error) {
	if strings.TrimSpace(name) == "" { return ElevatorStop{}, errors.New("elevator stop name is required") }
	id, err := accessID("els_")
	if err != nil { return ElevatorStop{}, err }
	now := time.Now().UTC()
	e := ElevatorStop{ID: id, TenantID: tenantID, ElevatorID: elevatorID, FloorID: floorID, Name: strings.TrimSpace(name), Status: "active", CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	s.elevatorStops = append([]ElevatorStop{e}, s.elevatorStops...)
	_ = s.persistLocked()
	s.mu.Unlock()
	return e, nil
}

func (s *Service) UpdateElevatorStop(tenantID, id string, name *string) (ElevatorStop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.elevatorStops {
		if s.elevatorStops[i].ID == id && (tenantID == "" || s.elevatorStops[i].TenantID == tenantID) {
			if name != nil { s.elevatorStops[i].Name = strings.TrimSpace(*name) }
			s.elevatorStops[i].UpdatedAt = time.Now().UTC()
			_ = s.persistLocked()
			return s.elevatorStops[i], nil
		}
	}
	return ElevatorStop{}, ErrElevatorStopNotFound
}

func (s *Service) DeleteElevatorStop(tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.elevatorStops {
		if s.elevatorStops[i].ID == id && (tenantID == "" || s.elevatorStops[i].TenantID == tenantID) {
			s.elevatorStops = append(s.elevatorStops[:i], s.elevatorStops[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrElevatorStopNotFound
}

func (s *Service) SetElevatorStopStatus(tenantID, id, status string) (ElevatorStop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.elevatorStops {
		if s.elevatorStops[i].ID == id && (tenantID == "" || s.elevatorStops[i].TenantID == tenantID) {
			s.elevatorStops[i].Status = status
			s.elevatorStops[i].UpdatedAt = time.Now().UTC()
			_ = s.persistLocked()
			return s.elevatorStops[i], nil
		}
	}
	return ElevatorStop{}, ErrElevatorStopNotFound
}

// --- Group Elevator Stops ---

func (s *Service) ListGroupElevatorStops(tenantID, groupID string) []GroupElevatorStop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]GroupElevatorStop, 0)
	for _, e := range s.groupElevatorStops {
		if tenantID != "" && e.TenantID != tenantID { continue }
		if groupID != "" && e.GroupID != groupID { continue }
		items = append(items, e)
	}
	return items
}

func (s *Service) CreateGroupElevatorStop(tenantID, groupID, elevatorStopID string) (GroupElevatorStop, error) {
	id, err := accessID("ges_")
	if err != nil { return GroupElevatorStop{}, err }
	e := GroupElevatorStop{ID: id, TenantID: tenantID, GroupID: groupID, ElevatorStopID: elevatorStopID, CreatedAt: time.Now().UTC()}
	s.mu.Lock()
	s.groupElevatorStops = append([]GroupElevatorStop{e}, s.groupElevatorStops...)
	_ = s.persistLocked()
	s.mu.Unlock()
	return e, nil
}

func (s *Service) DeleteGroupElevatorStop(tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.groupElevatorStops {
		if s.groupElevatorStops[i].ID == id && (tenantID == "" || s.groupElevatorStops[i].TenantID == tenantID) {
			s.groupElevatorStops = append(s.groupElevatorStops[:i], s.groupElevatorStops[i+1:]...)
			return s.persistLocked()
		}
	}
	return errors.New("group elevator stop not found")
}

// --- Group Terminals ---

func (s *Service) ListGroupTerminals(tenantID, groupID string) []GroupTerminal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]GroupTerminal, 0)
	for _, e := range s.groupTerminals {
		if tenantID != "" && e.TenantID != tenantID { continue }
		if groupID != "" && e.GroupID != groupID { continue }
		items = append(items, e)
	}
	return items
}

func (s *Service) CreateGroupTerminal(tenantID, groupID, terminalID string) (GroupTerminal, error) {
	id, err := accessID("gt_")
	if err != nil { return GroupTerminal{}, err }
	e := GroupTerminal{ID: id, TenantID: tenantID, GroupID: groupID, TerminalID: terminalID, CreatedAt: time.Now().UTC()}
	s.mu.Lock()
	s.groupTerminals = append([]GroupTerminal{e}, s.groupTerminals...)
	_ = s.persistLocked()
	s.mu.Unlock()
	return e, nil
}

func (s *Service) DeleteGroupTerminal(tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.groupTerminals {
		if s.groupTerminals[i].ID == id && (tenantID == "" || s.groupTerminals[i].TenantID == tenantID) {
			s.groupTerminals = append(s.groupTerminals[:i], s.groupTerminals[i+1:]...)
			return s.persistLocked()
		}
	}
	return errors.New("group terminal not found")
}

// --- Presences ---

func (s *Service) ListPresences(tenantID, placeID string) []Presence {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Presence, 0)
	for _, p := range s.presences {
		if tenantID != "" && p.TenantID != tenantID { continue }
		if placeID != "" && p.PlaceID != placeID { continue }
		if p.ExitedAt == "" { items = append(items, p) } // only current presences
	}
	return items
}

// --- CSV Card Imports ---

func (s *Service) ListCSVCardImports(tenantID string) []CSVCardImport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]CSVCardImport, 0)
	for _, c := range s.csvCardImports {
		if tenantID != "" && c.TenantID != tenantID { continue }
		items = append(items, c)
	}
	return items
}

func (s *Service) CreateCSVCardImport(tenantID, fileName string) (CSVCardImport, error) {
	if strings.TrimSpace(fileName) == "" { return CSVCardImport{}, errors.New("file_name is required") }
	id, err := accessID("cci_")
	if err != nil { return CSVCardImport{}, err }
	now := time.Now().UTC()
	c := CSVCardImport{ID: id, TenantID: tenantID, FileName: strings.TrimSpace(fileName), Status: "pending", CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	s.csvCardImports = append([]CSVCardImport{c}, s.csvCardImports...)
	_ = s.persistLocked()
	s.mu.Unlock()
	return c, nil
}

func (s *Service) GetCSVCardImport(tenantID, id string) (CSVCardImport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.csvCardImports {
		if c.ID == id && (tenantID == "" || c.TenantID == tenantID) { return c, nil }
	}
	return CSVCardImport{}, errors.New("csv card import not found")
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		return s.stateStore.Save(stateKey, stateSnapshot{
			Users:                    cloneAccessUsers(s.users),
			UserInvitationDeliveries: cloneUserInvitationDeliveries(s.userInvitationDeliveries),
			UserGroups:               cloneUserGroups(s.userGroups),
			Policies:                 clonePolicies(s.policies),
			TemporaryAccess:          cloneTemporaryAccess(s.temporaryAccess),
			VisitorPasses:            cloneVisitorPasses(s.visitorPasses),
			RoleAssignments:          cloneRoleAssignments(s.roleAssignments),
			Teams:                    cloneTeams(s.teams),
			TeamMemberships:          cloneTeamMemberships(s.teamMemberships),
			GroupLinks:               cloneGroupLinks(s.groupLinks),
			OAuth2Clients:            cloneOAuth2Clients(s.oauth2Clients),
		})
	}

	s.mu.Lock()
	s.users = cloneAccessUsers(snapshot.Users)
	s.userInvitationDeliveries = cloneUserInvitationDeliveries(snapshot.UserInvitationDeliveries)
	s.userGroups = cloneUserGroups(snapshot.UserGroups)
	s.policies = clonePolicies(snapshot.Policies)
	s.temporaryAccess = cloneTemporaryAccess(snapshot.TemporaryAccess)
	s.visitorPasses = cloneVisitorPasses(snapshot.VisitorPasses)
	s.guests = cloneGuests(snapshot.Guests)
	if len(snapshot.RoleAssignments) > 0 {
		s.roleAssignments = cloneRoleAssignments(snapshot.RoleAssignments)
	}
	if len(snapshot.Teams) > 0 {
		s.teams = cloneTeams(snapshot.Teams)
	}
	if len(snapshot.TeamMemberships) > 0 {
		s.teamMemberships = cloneTeamMemberships(snapshot.TeamMemberships)
	}
	if len(snapshot.GroupLinks) > 0 {
		s.groupLinks = cloneGroupLinks(snapshot.GroupLinks)
	}
	if len(snapshot.BookableSpaces) > 0 {
		s.bookableSpaces = cloneBookableSpaces(snapshot.BookableSpaces)
	}
	if len(snapshot.Bookings) > 0 {
		s.bookings = cloneBookings(snapshot.Bookings)
	}
	if len(snapshot.OAuth2Clients) > 0 {
		s.oauth2Clients = cloneOAuth2Clients(snapshot.OAuth2Clients)
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Users:                    cloneAccessUsers(s.users),
		UserInvitationDeliveries: cloneUserInvitationDeliveries(s.userInvitationDeliveries),
		UserGroups:               cloneUserGroups(s.userGroups),
		Policies:                 clonePolicies(s.policies),
		TemporaryAccess:          cloneTemporaryAccess(s.temporaryAccess),
		VisitorPasses:            cloneVisitorPasses(s.visitorPasses),
		Guests:                   cloneGuests(s.guests),
		RoleAssignments:          cloneRoleAssignments(s.roleAssignments),
		Teams:                    cloneTeams(s.teams),
		TeamMemberships:          cloneTeamMemberships(s.teamMemberships),
		GroupLinks:               cloneGroupLinks(s.groupLinks),
		BookableSpaces:           cloneBookableSpaces(s.bookableSpaces),
		Bookings:                 cloneBookings(s.bookings),
		OAuth2Clients:            cloneOAuth2Clients(s.oauth2Clients),
	})
}

func cloneAccessUsers(items []AccessUser) []AccessUser {
	output := make([]AccessUser, 0, len(items))
	for i := range items {
		output = append(output, cloneAccessUser(items[i]))
	}
	return output
}

func cloneAccessUser(item AccessUser) AccessUser {
	record := item
	record.GroupIDs = append([]string(nil), item.GroupIDs...)
	return record
}

func cloneUserInvitationDeliveries(items []UserInvitationDelivery) []UserInvitationDelivery {
	output := make([]UserInvitationDelivery, 0, len(items))
	for i := range items {
		output = append(output, cloneUserInvitationDelivery(items[i]))
	}
	return output
}

func cloneUserInvitationDelivery(item UserInvitationDelivery) UserInvitationDelivery {
	record := item
	record.ResourceType = "UserInvitationDelivery"
	if item.DeliveredAt != nil {
		deliveredAt := *item.DeliveredAt
		record.DeliveredAt = &deliveredAt
	}
	return record
}

func cloneUserGroups(items []UserGroup) []UserGroup {
	output := make([]UserGroup, 0, len(items))
	for i := range items {
		output = append(output, cloneUserGroup(items[i]))
	}
	return output
}

func cloneUserGroup(item UserGroup) UserGroup {
	record := item
	applyUserGroupDefaults(&record)
	record.Members = append([]string(nil), item.Members...)
	return record
}

func applyUserGroupDefaults(item *UserGroup) {
	if item == nil {
		return
	}
	item.ResourceType = "Group"
	if strings.TrimSpace(item.PlaceID) == "" {
		item.PlaceID = strings.TrimSpace(item.BuildingID)
	}
	if item.GeofenceRestrictionRadius <= 0 {
		item.GeofenceRestrictionRadius = 150
	}
	item.UsersCount = len(item.Members)
}

func clonePolicies(items []Policy) []Policy {
	output := make([]Policy, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTemporaryAccess(items []TemporaryAccess) []TemporaryAccess {
	output := make([]TemporaryAccess, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneVisitorPasses(items []VisitorPass) []VisitorPass {
	output := make([]VisitorPass, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneGuests(items []Guest) []Guest {
	output := make([]Guest, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneBookableSpaces(items []BookableSpace) []BookableSpace {
	output := make([]BookableSpace, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneBookings(items []Booking) []Booking {
	output := make([]Booking, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneGroupLinks(items []GroupLink) []GroupLink {
	output := make([]GroupLink, 0, len(items))
	for i := range items {
		output = append(output, cloneGroupLink(items[i]))
	}
	return output
}

func cloneGroupLink(item GroupLink) GroupLink {
	record := item
	record.ResourceType = "GroupLink"
	return record
}

func cloneRoleAssignments(items []RoleAssignment) []RoleAssignment {
	output := make([]RoleAssignment, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTeams(items []Team) []Team {
	output := make([]Team, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTeamMemberships(items []TeamMembership) []TeamMembership {
	output := make([]TeamMembership, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneOAuth2Clients(items []OAuth2Client) []OAuth2Client {
	output := make([]OAuth2Client, 0, len(items))
	for i := range items {
		c := items[i]
		c.ClientSecret = "" // never persist plaintext secret
		if len(items[i].RedirectURIs) > 0 {
			c.RedirectURIs = make([]string, len(items[i].RedirectURIs))
			copy(c.RedirectURIs, items[i].RedirectURIs)
		}
		if len(items[i].Scopes) > 0 {
			c.Scopes = make([]string, len(items[i].Scopes))
			copy(c.Scopes, items[i].Scopes)
		}
		output = append(output, c)
	}
	return output
}

func builtInRoles() []Role {
	return []Role{
		{
			ID:          "role_organization_admin",
			Name:        "Organization Admin",
			AppliesTo:   "Organization",
			Description: "Manage organization users, places, access, credentials, integrations, and reports.",
			Permissions: map[string]bool{
				"places_read":            true,
				"places_write":           true,
				"locks_read":             true,
				"locks_write":            true,
				"locks_unlock":           true,
				"users_read":             true,
				"users_write":            true,
				"groups_read":            true,
				"groups_write":           true,
				"teams_read":             true,
				"teams_write":            true,
				"team_memberships_read":  true,
				"team_memberships_write": true,
				"roles_read":             true,
				"role_assignments_read":  true,
				"role_assignments_write": true,
				"shares_read":            true,
				"shares_write":           true,
				"cards_read":             true,
				"cards_write":            true,
				"integrations_read":      true,
				"integrations_write":     true,
				"reports_read":           true,
			},
			BuiltIn: true,
		},
		{
			ID:          "role_place_admin",
			Name:        "Place Admin",
			AppliesTo:   "Place",
			Description: "Manage assigned place users, groups, locks, hardware, and local events.",
			Permissions: map[string]bool{
				"places_read":            true,
				"locks_read":             true,
				"locks_write":            true,
				"locks_unlock":           true,
				"users_read":             true,
				"users_write":            true,
				"groups_read":            true,
				"groups_write":           true,
				"teams_read":             true,
				"team_memberships_read":  true,
				"role_assignments_read":  true,
				"role_assignments_write": true,
				"shares_read":            true,
				"shares_write":           true,
				"events_read":            true,
			},
			BuiltIn: true,
		},
		{
			ID:          "role_group_access",
			Name:        "Group Access",
			AppliesTo:   "Group",
			Description: "Grant access through a group without creating a management persona.",
			Permissions: map[string]bool{
				"locks_read":   true,
				"locks_unlock": true,
			},
			BuiltIn: true,
		},
	}
}

func normalizeRoleAssignmentInput(input RoleAssignmentInput) (RoleAssignmentInput, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return RoleAssignmentInput{}, ErrTenantIDRequired
	}

	nextRoleID := strings.TrimSpace(input.RoleID)
	if nextRoleID == "" {
		return RoleAssignmentInput{}, ErrRoleIDRequired
	}
	role, exists := roleByID(nextRoleID)
	if !exists {
		return RoleAssignmentInput{}, ErrRoleNotFound
	}

	nextAppliesToType, err := normalizeRoleScope(input.AppliesToType)
	if err != nil {
		return RoleAssignmentInput{}, err
	}
	if role.AppliesTo != nextAppliesToType {
		return RoleAssignmentInput{}, ErrInvalidRoleScope
	}

	nextAppliesToID := strings.TrimSpace(input.AppliesToID)
	if nextAppliesToID == "" {
		return RoleAssignmentInput{}, ErrAppliesToIDRequired
	}

	nextAssigneeType, err := normalizeAssigneeType(input.AssigneeType)
	if err != nil {
		return RoleAssignmentInput{}, err
	}
	nextAssigneeID := strings.TrimSpace(input.AssigneeID)
	if nextAssigneeID == "" {
		return RoleAssignmentInput{}, ErrAssigneeIDRequired
	}

	return RoleAssignmentInput{
		TenantID:      nextTenantID,
		RoleID:        nextRoleID,
		AppliesToType: nextAppliesToType,
		AppliesToID:   nextAppliesToID,
		AssigneeType:  nextAssigneeType,
		AssigneeID:    nextAssigneeID,
		AssigneeEmail: strings.TrimSpace(input.AssigneeEmail),
		ValidFrom:     strings.TrimSpace(input.ValidFrom),
		ValidUntil:    strings.TrimSpace(input.ValidUntil),
	}, nil
}

func roleByID(roleID string) (Role, bool) {
	nextRoleID := strings.TrimSpace(roleID)
	for _, role := range builtInRoles() {
		if role.ID == nextRoleID {
			return role, true
		}
	}
	return Role{}, false
}

func normalizeRoleScope(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "organization":
		return "Organization", nil
	case "place":
		return "Place", nil
	case "group":
		return "Group", nil
	default:
		return "", ErrInvalidRoleScope
	}
}

func normalizeAssigneeType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return "User", nil
	case "team":
		return "Team", nil
	case "guest":
		return "Guest", nil
	default:
		return "", ErrInvalidAssigneeType
	}
}

func normalizeTeamScope(scope, placeID string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "organization":
		if strings.TrimSpace(placeID) != "" {
			return "place", nil
		}
		return "organization", nil
	case "place":
		return "place", nil
	default:
		return "", ErrInvalidScopeType
	}
}

func normalizeTeamMemberType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user", "":
		return "User", nil
	case "guest":
		return "Guest", nil
	default:
		return "", ErrInvalidTeamMemberType
	}
}

func firstNonEmpty(values ...string) string {
	for i := range values {
		value := strings.TrimSpace(values[i])
		if value != "" {
			return value
		}
	}
	return ""
}

func accessID(prefix string) (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func uniqueIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}

	if len(items) == 0 {
		return nil
	}

	return items
}

func removeID(values []string, target string) []string {
	if len(values) == 0 {
		return nil
	}
	nextTarget := strings.TrimSpace(target)
	if nextTarget == "" {
		return uniqueIDs(values)
	}
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || value == nextTarget {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func normalizePolicyStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	case "draft":
		return "draft", nil
	default:
		return "", ErrInvalidPolicyStatus
	}
}

func normalizeUserStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	case "suspended":
		return "suspended", nil
	default:
		return "", ErrInvalidUserStatus
	}
}

func normalizeUserInvitationDeliveryMethod(method string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", "email":
		return "email", nil
	case "email_qr":
		return "email_qr", nil
	default:
		return "", ErrDeliveryMethodInvalid
	}
}

func normalizeUserInvitationStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "sent", "delivered":
		return "sent", nil
	case "failed", "bounced", "rejected", "undelivered":
		return "failed", nil
	default:
		return "", ErrInvalidUserInvitationStatus
	}
}

func normalizeScopeType(scopeType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "", "all":
		return "all", nil
	case "building":
		return "building", nil
	case "area":
		return "area", nil
	case "door":
		return "door", nil
	default:
		return "", ErrInvalidScopeType
	}
}

func normalizeDeliveryMethod(method string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", "wallet":
		return "wallet", nil
	case "email_qr":
		return "email_qr", nil
	default:
		return "", ErrDeliveryMethodInvalid
	}
}

func normalizeGroupLinkQRCodeType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "null":
		return "", nil
	case "online":
		return "online", nil
	case "offline":
		return "offline", nil
	default:
		return "", ErrInvalidGroupLinkQRCodeType
	}
}

func parseGroupLinkInstant(value string) (time.Time, bool, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}, false, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for i := range layouts {
		parsed, err := time.Parse(layouts[i], raw)
		if err == nil {
			return parsed.UTC(), true, nil
		}
	}
	return time.Time{}, true, ErrInvalidGroupLinkValidityWindow
}

func normalizeGroupLinkCreatorType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return "User"
	case "marketplaceinstallation", "marketplace_installation":
		return "MarketplaceInstallation"
	default:
		return ""
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeSyncSource(source string) string {
	return strings.ToLower(strings.TrimSpace(source))
}

func accessSyncIdentityKey(source, ref string) string {
	nextSource := normalizeSyncSource(source)
	nextRef := strings.TrimSpace(ref)
	if nextSource == "" || nextRef == "" {
		return ""
	}
	return nextSource + ":" + nextRef
}

// --- Holiday Calendar ---

var ErrHolidayCalendarNameRequired = errors.New("holiday calendar name is required")
var ErrHolidayCalendarNotFound = errors.New("holiday calendar not found")

