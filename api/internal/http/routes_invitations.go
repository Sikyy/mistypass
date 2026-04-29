package httpx

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

func (s *server) listInvitations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	items := s.accessSvc.ListUserInvitationDeliveries(tenantID, "")

	if statusFilter != "" {
		filtered := make([]access.UserInvitationDelivery, 0, len(items))
		for _, item := range items {
			if item.Status == statusFilter {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].QueuedAt.After(items[j].QueuedAt)
	})

	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getInvitation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	delivery, err := s.accessSvc.GetInvitationDelivery(tenantID, chi.URLParam(r, "deliveryID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, delivery)
}

func (s *server) cancelInvitation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	deliveryID := chi.URLParam(r, "deliveryID")
	delivery, err := s.accessSvc.CancelInvitationDelivery(tenantID, deliveryID)
	if err != nil {
		if strings.Contains(err.Error(), "cannot cancel") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusNotFound, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "invitation_cancelled",
		fmt.Sprintf("delivery_id=%s,user_id=%s,email=%s", delivery.ID, delivery.UserID, delivery.Email),
		"access",
	)
	writeJSON(w, http.StatusOK, delivery)
}

func (s *server) resendInvitation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	deliveryID := chi.URLParam(r, "deliveryID")
	newDelivery, user, err := s.accessSvc.ResendInvitationDelivery(tenantID, deliveryID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	responseDelivery := newDelivery
	s.appendAuditLog(r, tenantID, "invitation_resent",
		fmt.Sprintf("delivery_id=%s,original_delivery_id=%s,user_id=%s,email=%s", newDelivery.ID, deliveryID, newDelivery.UserID, newDelivery.Email),
		"access",
	)
	if dispatched, ok := s.dispatchUserInvitationEmail(r, newDelivery, user); ok {
		responseDelivery = dispatched
	}
	writeJSON(w, http.StatusAccepted, responseDelivery)
}
