package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

func (s *server) appListBookableSpaces(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	items := s.accessSvc.ListBookableSpaces(user.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) appGetBookableSpaceStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	spaceID := chi.URLParam(r, "spaceID")
	status, err := s.accessSvc.GetBookableSpaceStatus(user.TenantID, spaceID)
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

func (s *server) appListBookings(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	spaceID := r.URL.Query().Get("space_id")
	items := s.accessSvc.ListBookings(user.TenantID, spaceID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) appCreateBooking(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	var request struct {
		SpaceID   string `json:"space_id"`
		Title     string `json:"title"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	booking, err := s.accessSvc.CreateBooking(access.CreateBookingInput{
		TenantID:  user.TenantID,
		SpaceID:   request.SpaceID,
		UserID:    user.ID,
		UserName:  user.Name,
		Title:     request.Title,
		StartTime: request.StartTime,
		EndTime:   request.EndTime,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrBookingSpaceIDRequired),
			errors.Is(err, access.ErrBookingStartTimeRequired),
			errors.Is(err, access.ErrBookingEndTimeRequired),
			errors.Is(err, access.ErrBookingEndBeforeStart):
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

	s.appendAuditLog(r, user.TenantID, "booking_created",
		fmt.Sprintf("booking_id=%s,space_id=%s,user=%s", booking.ID, booking.SpaceID, user.Email), "access")
	writeJSON(w, http.StatusCreated, booking)
}

func (s *server) appCancelBooking(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	bookingID := chi.URLParam(r, "bookingID")

	booking, err := s.accessSvc.GetBooking(user.TenantID, bookingID)
	if err != nil {
		if errors.Is(err, access.ErrBookingNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if booking.UserID != user.ID {
		writeError(w, http.StatusForbidden, "cannot cancel another user's booking")
		return
	}

	cancelled := "cancelled"
	updated, err := s.accessSvc.UpdateBooking(user.TenantID, bookingID, access.UpdateBookingInput{
		Status: &cancelled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.appendAuditLog(r, user.TenantID, "booking_cancelled",
		fmt.Sprintf("booking_id=%s,space_id=%s", bookingID, updated.SpaceID), "access")
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) appCheckInBooking(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	bookingID := chi.URLParam(r, "bookingID")

	booking, err := s.accessSvc.CheckInBooking(user.TenantID, bookingID)
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

	s.appendAuditLog(r, user.TenantID, "booking_checked_in",
		fmt.Sprintf("booking_id=%s,space_id=%s,user=%s,time=%s", booking.ID, booking.SpaceID, user.Email, time.Now().UTC().Format(time.RFC3339)), "access")
	writeJSON(w, http.StatusOK, booking)
}

func (s *server) appCheckOutBooking(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	bookingID := chi.URLParam(r, "bookingID")

	booking, err := s.accessSvc.CheckOutBooking(user.TenantID, bookingID)
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

	s.appendAuditLog(r, user.TenantID, "booking_checked_out",
		fmt.Sprintf("booking_id=%s,space_id=%s,user=%s", booking.ID, booking.SpaceID, user.Email), "access")
	writeJSON(w, http.StatusOK, booking)
}
