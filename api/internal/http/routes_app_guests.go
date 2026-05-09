package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

// GET /api/v1/app/places/{placeId}/guests
func (s *server) appAdminListGuests(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	items := s.accessSvc.ListGuests(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/v1/app/places/{placeId}/guests
func (s *server) appAdminCreateGuest(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	var request struct {
		Name             string `json:"name"`
		Email            string `json:"email"`
		Phone            string `json:"phone"`
		Company          string `json:"company"`
		Purpose          string `json:"purpose"`
		HostName         string `json:"host_name"`
		HostEmail        string `json:"host_email"`
		HostPhone        string `json:"host_phone"`
		IDDocumentType   string `json:"id_document_type"`
		IDDocumentNumber string `json:"id_document_number"`
		ExpectedAt       string `json:"expected_at"`
		NotifyHost       bool   `json:"notify_host"`
		AccessTTLHours   int    `json:"access_ttl_hours"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	guest, err := s.accessSvc.CreateGuest(access.CreateGuestInput{
		TenantID:         tenantID,
		BuildingID:       placeID,
		Name:             request.Name,
		Email:            request.Email,
		Phone:            request.Phone,
		Company:          request.Company,
		Purpose:          request.Purpose,
		HostName:         request.HostName,
		HostEmail:        request.HostEmail,
		HostPhone:        request.HostPhone,
		IDDocumentType:   request.IDDocumentType,
		IDDocumentNumber: request.IDDocumentNumber,
		ExpectedAt:       request.ExpectedAt,
		NotifyHost:       request.NotifyHost,
		AccessTTLHours:   request.AccessTTLHours,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrGuestNameRequired),
			errors.Is(err, access.ErrGuestPhoneRequired),
			errors.Is(err, access.ErrGuestHostRequired),
			errors.Is(err, access.ErrGuestIDDocumentTypeInvalid),
			errors.Is(err, access.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "guest_created",
		fmt.Sprintf("guest_id=%s,name=%s,host=%s,user=%s", guest.ID, guest.Name, guest.HostName, user.Email), "access")
	writeJSON(w, http.StatusCreated, guest)
}

// PATCH /api/v1/app/places/{placeId}/guests/{guestId}
func (s *server) appAdminUpdateGuestStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	guestID := chi.URLParam(r, "guestId")
	tenantID := user.TenantID

	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	guest, err := s.accessSvc.UpdateGuestStatus(tenantID, guestID, request.Status)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrGuestNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrGuestStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "guest_status_updated",
		fmt.Sprintf("guest_id=%s,status=%s,user=%s", guest.ID, guest.Status, user.Email), "access")
	writeJSON(w, http.StatusOK, guest)
}

// DELETE /api/v1/app/places/{placeId}/guests/{guestId}
func (s *server) appAdminDeleteGuest(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	guestID := chi.URLParam(r, "guestId")
	tenantID := user.TenantID

	if err := s.accessSvc.DeleteGuest(tenantID, guestID); err != nil {
		if errors.Is(err, access.ErrGuestNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "guest_deleted",
		fmt.Sprintf("guest_id=%s,user=%s", guestID, user.Email), "access")
	w.WriteHeader(http.StatusNoContent)
}
