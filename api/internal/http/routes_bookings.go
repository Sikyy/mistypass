package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

func (s *server) listBookableSpaces(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListBookableSpaces(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createBookableSpace(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID        string `json:"tenant_id"`
		Name            string `json:"name"`
		Description     string `json:"description"`
		SpaceType       string `json:"space_type"`
		CapacityMode    string `json:"capacity_mode"`
		MaxCapacity     int    `json:"max_capacity"`
		LockID          string `json:"lock_id"`
		RequiresBooking bool   `json:"requires_booking"`
		Enabled         bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	space, err := s.accessSvc.CreateBookableSpace(access.CreateBookableSpaceInput{
		TenantID:        tenantID,
		Name:            request.Name,
		Description:     request.Description,
		SpaceType:       request.SpaceType,
		CapacityMode:    request.CapacityMode,
		MaxCapacity:     request.MaxCapacity,
		LockID:          request.LockID,
		RequiresBooking: request.RequiresBooking,
		Enabled:         request.Enabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrSpaceNameRequired),
			errors.Is(err, access.ErrSpaceTypeRequired),
			errors.Is(err, access.ErrInvalidSpaceType),
			errors.Is(err, access.ErrInvalidCapacityMode),
			errors.Is(err, access.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "bookable_space_created",
		fmt.Sprintf("space_id=%s,name=%s,type=%s", space.ID, space.Name, space.SpaceType), "access")
	writeJSON(w, http.StatusCreated, space)
}

func (s *server) updateBookableSpace(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceID")
	var request struct {
		TenantID        string  `json:"tenant_id"`
		Name            *string `json:"name"`
		Description     *string `json:"description"`
		SpaceType       *string `json:"space_type"`
		CapacityMode    *string `json:"capacity_mode"`
		MaxCapacity     *int    `json:"max_capacity"`
		LockID          *string `json:"lock_id"`
		RequiresBooking *bool   `json:"requires_booking"`
		Enabled         *bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	space, err := s.accessSvc.UpdateBookableSpace(tenantID, spaceID, access.UpdateBookableSpaceInput{
		Name:            request.Name,
		Description:     request.Description,
		SpaceType:       request.SpaceType,
		CapacityMode:    request.CapacityMode,
		MaxCapacity:     request.MaxCapacity,
		LockID:          request.LockID,
		RequiresBooking: request.RequiresBooking,
		Enabled:         request.Enabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrSpaceNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrSpaceNameRequired),
			errors.Is(err, access.ErrInvalidSpaceType),
			errors.Is(err, access.ErrInvalidCapacityMode):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "bookable_space_updated",
		fmt.Sprintf("space_id=%s,name=%s", space.ID, space.Name), "access")
	writeJSON(w, http.StatusOK, space)
}

func (s *server) deleteBookableSpace(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	if err := s.accessSvc.DeleteBookableSpace(tenantID, spaceID); err != nil {
		if errors.Is(err, access.ErrSpaceNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "bookable_space_deleted",
		fmt.Sprintf("space_id=%s", spaceID), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) getBookableSpaceStatus(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	status, err := s.accessSvc.GetBookableSpaceStatus(tenantID, spaceID)
	if err != nil {
		if errors.Is(err, access.ErrSpaceNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) getBookableSpaceUsage(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	// Return all bookings for this space as a usage summary.
	space, err := s.accessSvc.GetBookableSpace(tenantID, spaceID)
	if err != nil {
		if errors.Is(err, access.ErrSpaceNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	bookings := s.accessSvc.ListBookings(tenantID, spaceID)
	total := len(bookings)
	checkedIn := 0
	completed := 0
	cancelled := 0
	noShow := 0
	for _, b := range bookings {
		switch b.Status {
		case "checked_in":
			checkedIn++
		case "completed":
			completed++
		case "cancelled":
			cancelled++
		case "no_show":
			noShow++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"space_id":       space.ID,
		"space_name":     space.Name,
		"total_bookings": total,
		"checked_in":     checkedIn,
		"completed":      completed,
		"cancelled":      cancelled,
		"no_show":        noShow,
	})
}

func (s *server) listBookings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	spaceID := r.URL.Query().Get("space_id")
	items := s.accessSvc.ListBookings(tenantID, spaceID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) getBooking(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	bookingID := chi.URLParam(r, "bookingID")
	booking, err := s.accessSvc.GetBooking(tenantID, bookingID)
	if err != nil {
		if errors.Is(err, access.ErrBookingNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, booking)
}

func (s *server) createBooking(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID  string `json:"tenant_id"`
		SpaceID   string `json:"space_id"`
		UserID    string `json:"user_id"`
		UserName  string `json:"user_name"`
		Title     string `json:"title"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	booking, err := s.accessSvc.CreateBooking(access.CreateBookingInput{
		TenantID:  tenantID,
		SpaceID:   request.SpaceID,
		UserID:    request.UserID,
		UserName:  request.UserName,
		Title:     request.Title,
		StartTime: request.StartTime,
		EndTime:   request.EndTime,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrBookingSpaceIDRequired),
			errors.Is(err, access.ErrBookingUserIDRequired),
			errors.Is(err, access.ErrBookingStartTimeRequired),
			errors.Is(err, access.ErrBookingEndTimeRequired),
			errors.Is(err, access.ErrBookingEndBeforeStart),
			errors.Is(err, access.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, access.ErrSpaceNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrSpaceAtCapacity),
			errors.Is(err, access.ErrBookingTimeConflict):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "booking_created",
		fmt.Sprintf("booking_id=%s,space_id=%s,user_id=%s", booking.ID, booking.SpaceID, booking.UserID), "access")
	writeJSON(w, http.StatusCreated, booking)
}

func (s *server) updateBooking(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "bookingID")
	var request struct {
		TenantID  string  `json:"tenant_id"`
		Title     *string `json:"title"`
		StartTime *string `json:"start_time"`
		EndTime   *string `json:"end_time"`
		Status    *string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	booking, err := s.accessSvc.UpdateBooking(tenantID, bookingID, access.UpdateBookingInput{
		Title:     request.Title,
		StartTime: request.StartTime,
		EndTime:   request.EndTime,
		Status:    request.Status,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrBookingNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrBookingStartTimeRequired),
			errors.Is(err, access.ErrBookingEndTimeRequired),
			errors.Is(err, access.ErrBookingEndBeforeStart),
			errors.Is(err, access.ErrBookingStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "booking_updated",
		fmt.Sprintf("booking_id=%s", booking.ID), "access")
	writeJSON(w, http.StatusOK, booking)
}

func (s *server) deleteBooking(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "bookingID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	if err := s.accessSvc.DeleteBooking(tenantID, bookingID); err != nil {
		if errors.Is(err, access.ErrBookingNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "booking_deleted",
		fmt.Sprintf("booking_id=%s", bookingID), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) checkInBooking(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "bookingID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	booking, err := s.accessSvc.CheckInBooking(tenantID, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrBookingNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrBookingAlreadyCheckedIn):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "booking_checked_in",
		fmt.Sprintf("booking_id=%s,space_id=%s", booking.ID, booking.SpaceID), "access")
	writeJSON(w, http.StatusOK, booking)
}

func (s *server) checkOutBooking(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "bookingID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	booking, err := s.accessSvc.CheckOutBooking(tenantID, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrBookingNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrBookingNotCheckedIn):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "booking_checked_out",
		fmt.Sprintf("booking_id=%s,space_id=%s", booking.ID, booking.SpaceID), "access")
	writeJSON(w, http.StatusOK, booking)
}
