package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

// validateGuestDoorIDs checks that every requested guest door belongs to the
// tenant — and, when buildingID is provided, to that building — so a guest QR
// access token can never be provisioned against doors outside the caller's
// scope. It returns the trimmed list, or a non-empty message for a 400.
func (s *server) validateGuestDoorIDs(tenantID, buildingID string, doorIDs []string) ([]string, string) {
	if len(doorIDs) == 0 {
		return nil, ""
	}
	doorBuildings := make(map[string]string)
	for _, door := range s.spaceSvc.ListDoors(tenantID) {
		doorBuildings[door.ID] = door.BuildingID
	}
	cleaned := make([]string, 0, len(doorIDs))
	for _, doorID := range doorIDs {
		doorID = strings.TrimSpace(doorID)
		if doorID == "" {
			continue
		}
		building, ok := doorBuildings[doorID]
		if !ok {
			return nil, "door " + doorID + " does not belong to this tenant"
		}
		if buildingID != "" && building != buildingID {
			return nil, "door " + doorID + " does not belong to this place"
		}
		cleaned = append(cleaned, doorID)
	}
	return cleaned, ""
}

func (s *server) listGuests(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListGuests(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) getGuest(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	guestID := chi.URLParam(r, "guestID")
	guest, err := s.accessSvc.GetGuest(tenantID, guestID)
	if err != nil {
		if errors.Is(err, access.ErrGuestNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, guest)
}

func (s *server) createGuest(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID         string   `json:"tenant_id"`
		BuildingID       string   `json:"building_id"`
		Name             string   `json:"name"`
		Email            string   `json:"email"`
		Phone            string   `json:"phone"`
		Company          string   `json:"company"`
		Purpose          string   `json:"purpose"`
		HostName         string   `json:"host_name"`
		HostEmail        string   `json:"host_email"`
		HostPhone        string   `json:"host_phone"`
		IDDocumentType   string   `json:"id_document_type"`
		IDDocumentNumber string   `json:"id_document_number"`
		ExpectedAt       string   `json:"expected_at"`
		NotifyHost       bool     `json:"notify_host"`
		DoorIDs          []string `json:"door_ids"`
		AccessTTLHours   int      `json:"access_ttl_hours"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	doorIDs, doorErrMsg := s.validateGuestDoorIDs(tenantID, request.BuildingID, request.DoorIDs)
	if doorErrMsg != "" {
		writeError(w, http.StatusBadRequest, doorErrMsg)
		return
	}

	guest, err := s.accessSvc.CreateGuest(access.CreateGuestInput{
		TenantID:         tenantID,
		BuildingID:       request.BuildingID,
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
		DoorIDs:          doorIDs,
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
		fmt.Sprintf("guest_id=%s,name=%s,phone=%s,host=%s", guest.ID, guest.Name, guest.Phone, guest.HostName), "access")
	writeJSON(w, http.StatusCreated, guest)
}

func (s *server) updateGuestStatus(w http.ResponseWriter, r *http.Request) {
	guestID := chi.URLParam(r, "guestID")
	var request struct {
		TenantID string `json:"tenant_id"`
		Status   string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
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
		fmt.Sprintf("guest_id=%s,status=%s", guest.ID, guest.Status), "access")
	writeJSON(w, http.StatusOK, guest)
}

func (s *server) deleteGuest(w http.ResponseWriter, r *http.Request) {
	guestID := chi.URLParam(r, "guestID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	if err := s.accessSvc.DeleteGuest(tenantID, guestID); err != nil {
		if errors.Is(err, access.ErrGuestNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "guest_deleted",
		fmt.Sprintf("guest_id=%s", guestID), "access")
	w.WriteHeader(http.StatusNoContent)
}
