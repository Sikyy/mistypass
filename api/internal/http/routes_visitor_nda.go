package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

// ---------------------------------------------------------------------------
// Visitor NDA — tenant template + guest signing + check-in enforcement
// ---------------------------------------------------------------------------

const stateKeyVisitorNDA = "module_visitor_nda"

type visitorNDATemplate struct {
	TenantID  string `json:"tenant_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Version   int    `json:"version"`
	Required  bool   `json:"required"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (s *server) visitorNDATemplateForTenant(tenantID string) visitorNDATemplate {
	s.visitorNDAMu.RLock()
	defer s.visitorNDAMu.RUnlock()
	if template, ok := s.visitorNDATemplates[tenantID]; ok {
		return template
	}
	return visitorNDATemplate{TenantID: tenantID}
}

// GET /api/v1/visitor-nda/template
func (s *server) getVisitorNDATemplate(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.visitorNDATemplateForTenant(tenantID))
}

// PUT /api/v1/visitor-nda/template
func (s *server) updateVisitorNDATemplate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string  `json:"tenant_id"`
		Title    *string `json:"title"`
		Body     *string `json:"body"`
		Required *bool   `json:"required"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	s.visitorNDAMu.Lock()
	template, exists := s.visitorNDATemplates[tenantID]
	if !exists {
		template = visitorNDATemplate{TenantID: tenantID}
	}
	contentChanged := false
	if request.Title != nil && strings.TrimSpace(*request.Title) != template.Title {
		template.Title = strings.TrimSpace(*request.Title)
		contentChanged = true
	}
	if request.Body != nil && *request.Body != template.Body {
		template.Body = *request.Body
		contentChanged = true
	}
	if request.Required != nil {
		template.Required = *request.Required
	}
	if contentChanged {
		template.Version++
	}
	template.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.visitorNDATemplates[tenantID] = template
	s.persistVisitorNDATemplatesLocked()
	s.visitorNDAMu.Unlock()

	s.appendAuditLog(r, tenantID, "visitor_nda_template_updated",
		fmt.Sprintf("version=%d,required=%v", template.Version, template.Required), "visitors")
	writeJSON(w, http.StatusOK, template)
}

// POST /api/v1/guests/{guestID}/nda/sign
func (s *server) signGuestNDA(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID         string `json:"tenant_id"`
		SignerName       string `json:"signer_name"`
		SignatureDataURL string `json:"signature_data_url"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	template := s.visitorNDATemplateForTenant(tenantID)
	if template.Version == 0 {
		writeError(w, http.StatusConflict, "no visitor NDA template is configured for this tenant")
		return
	}

	guestID := chi.URLParam(r, "guestID")
	guest, err := s.accessSvc.SignGuestNDA(tenantID, guestID, access.GuestNDAInput{
		SignerName:       request.SignerName,
		SignatureDataURL: request.SignatureDataURL,
		TemplateVersion:  template.Version,
	})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrGuestNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, access.ErrGuestNDASignerRequired), errors.Is(err, access.ErrGuestNDASignatureInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "guest_nda_signed",
		fmt.Sprintf("guest_id=%s,template_version=%d,signature_hash=%s", guest.ID, guest.NDATemplateVersion, guest.NDASignatureHash), "visitors")
	writeJSON(w, http.StatusOK, guest)
}

// guestNDACheckInBlocked reports whether checking in the guest must be rejected
// because the tenant requires a signed NDA and the guest has not signed.
func (s *server) guestNDACheckInBlocked(tenantID, guestID string) bool {
	template := s.visitorNDATemplateForTenant(tenantID)
	if !template.Required {
		return false
	}
	guest, err := s.accessSvc.GetGuest(tenantID, guestID)
	if err != nil {
		// Unknown guest: let the status-update path return its own 404.
		return false
	}
	return strings.TrimSpace(guest.NDASignedAt) == ""
}

// --- persistence ---

type visitorNDAStateSnapshot struct {
	Templates []visitorNDATemplate `json:"templates"`
}

// persistVisitorNDATemplatesLocked saves templates; caller holds visitorNDAMu.
func (s *server) persistVisitorNDATemplatesLocked() {
	if s.stateStore == nil {
		return
	}
	templates := make([]visitorNDATemplate, 0, len(s.visitorNDATemplates))
	for _, template := range s.visitorNDATemplates {
		templates = append(templates, template)
	}
	_ = s.stateStore.Save(stateKeyVisitorNDA, visitorNDAStateSnapshot{Templates: templates})
}

func (s *server) restoreVisitorNDATemplatesFromState() {
	if s.stateStore == nil {
		return
	}
	var snapshot visitorNDAStateSnapshot
	found, err := s.stateStore.Load(stateKeyVisitorNDA, &snapshot)
	if err != nil || !found {
		return
	}
	s.visitorNDAMu.Lock()
	defer s.visitorNDAMu.Unlock()
	for _, template := range snapshot.Templates {
		s.visitorNDATemplates[template.TenantID] = template
	}
}
