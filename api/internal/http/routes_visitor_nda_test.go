package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

const ndaTestSignature = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=="

func ndaTestRouter(t *testing.T) (http.Handler, string) {
	t.Helper()
	router, _, err := NewRouter(config.Config{JWTSecret: "visitor-nda-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return router, referenceAPILogin(t, router, "organization.admin@mistypass.local")
}

func ndaPutTemplate(t *testing.T, router http.Handler, token, body string) *struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Version  int    `json:"version"`
	Required bool   `json:"required"`
} {
	t.Helper()
	rec := referenceAPIRequest(t, router, http.MethodPut, "/api/v1/visitor-nda/template", token, []byte(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected template put 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	out := &struct {
		Title    string `json:"title"`
		Body     string `json:"body"`
		Version  int    `json:"version"`
		Required bool   `json:"required"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode template: %v", err)
	}
	return out
}

func ndaCreateGuest(t *testing.T, router http.Handler, token string) string {
	t.Helper()
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"NDA Visitor","phone":"0812 555 0001","host_name":"Host"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected guest create 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created access.Guest
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode guest: %v", err)
	}
	return created.ID
}

func TestVisitorNDATemplateLifecycle(t *testing.T) {
	store := &incidentAlertTestStore{}
	router, _, err := NewRouter(config.Config{JWTSecret: "visitor-nda-template-test", EnableDemoUsers: true}, store)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Default: version 0, not required.
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/visitor-nda/template?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected template get 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var defaults struct {
		Version  int  `json:"version"`
		Required bool `json:"required"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &defaults)
	if defaults.Version != 0 || defaults.Required {
		t.Fatalf("expected default template v0 not required, got %+v", defaults)
	}

	// PUT body+title -> version 1.
	updated := ndaPutTemplate(t, router, token, `{"tenant_id":"tenant_demo_jakarta","title":"Visitor NDA","body":"Keep secrets secret.","required":true}`)
	if updated.Version != 1 || !updated.Required || updated.Title != "Visitor NDA" {
		t.Fatalf("expected v1 required template, got %+v", updated)
	}

	// Toggling required only must not bump the version.
	toggled := ndaPutTemplate(t, router, token, `{"tenant_id":"tenant_demo_jakarta","required":false}`)
	if toggled.Version != 1 || toggled.Required {
		t.Fatalf("expected required-only toggle to keep v1, got %+v", toggled)
	}

	// Restart with the same state store restores the template.
	restored, _, err := NewRouter(config.Config{JWTSecret: "visitor-nda-template-test", EnableDemoUsers: true}, store)
	if err != nil {
		t.Fatalf("restored router: %v", err)
	}
	rToken := referenceAPILogin(t, restored, "organization.admin@mistypass.local")
	rGet := referenceAPIRequest(t, restored, http.MethodGet, "/api/v1/visitor-nda/template?tenant_id=tenant_demo_jakarta", rToken, nil)
	var restoredTemplate struct {
		Title   string `json:"title"`
		Version int    `json:"version"`
	}
	_ = json.Unmarshal(rGet.Body.Bytes(), &restoredTemplate)
	if rGet.Code != http.StatusOK || restoredTemplate.Version != 1 || restoredTemplate.Title != "Visitor NDA" {
		t.Fatalf("expected template to survive restart, got %d %+v", rGet.Code, restoredTemplate)
	}
}

func TestVisitorNDASignFlow(t *testing.T) {
	router, token := ndaTestRouter(t)
	guestID := ndaCreateGuest(t, router, token)

	// Signing before any template is configured -> 409.
	early := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/"+guestID+"/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signer_name":"NDA Visitor","signature_data_url":"`+ndaTestSignature+`"}`))
	if early.Code != http.StatusConflict {
		t.Fatalf("expected 409 signing without template, got %d body=%s", early.Code, early.Body.String())
	}

	ndaPutTemplate(t, router, token, `{"tenant_id":"tenant_demo_jakarta","title":"Visitor NDA","body":"Keep secrets.","required":true}`)

	// Valid sign.
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/"+guestID+"/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signer_name":"NDA Visitor","signature_data_url":"`+ndaTestSignature+`"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected sign 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var signed access.Guest
	if err := json.Unmarshal(rec.Body.Bytes(), &signed); err != nil {
		t.Fatalf("decode signed guest: %v", err)
	}
	if signed.NDASignedAt == "" || signed.NDASignerName != "NDA Visitor" || signed.NDATemplateVersion != 1 || signed.NDASignatureHash == "" {
		t.Fatalf("expected NDA fields set, got %+v", signed)
	}

	// Validation errors.
	noSigner := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/"+guestID+"/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signature_data_url":"`+ndaTestSignature+`"}`))
	if noSigner.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 missing signer, got %d", noSigner.Code)
	}
	badSig := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/"+guestID+"/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signer_name":"X","signature_data_url":"data:text/plain;base64,QUJD"}`))
	if badSig.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 non-image signature, got %d", badSig.Code)
	}
	huge := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/"+guestID+"/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signer_name":"X","signature_data_url":"data:image/png;base64,`+strings.Repeat("A", 70*1024)+`"}`))
	if huge.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 oversize signature, got %d", huge.Code)
	}
	missing := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/gst_does_not_exist/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signer_name":"X","signature_data_url":"`+ndaTestSignature+`"}`))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404 unknown guest, got %d", missing.Code)
	}
}

func TestVisitorNDACheckInEnforcement(t *testing.T) {
	router, token := ndaTestRouter(t)
	ndaPutTemplate(t, router, token, `{"tenant_id":"tenant_demo_jakarta","title":"Visitor NDA","body":"Keep secrets.","required":true}`)
	guestID := ndaCreateGuest(t, router, token)

	// Unsigned check-in blocked on the reference route.
	checkin := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"checked_in"}`)
	blocked := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/guests/"+guestID+"/status", token, checkin)
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), "nda_required") {
		t.Fatalf("expected 409 nda_required, got %d body=%s", blocked.Code, blocked.Body.String())
	}

	// Non-check-in transitions are unaffected.
	cancel := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/guests/"+guestID+"/status", token, []byte(`{"tenant_id":"tenant_demo_jakarta","status":"cancelled"}`))
	if cancel.Code != http.StatusOK {
		t.Fatalf("expected cancel 200 despite NDA, got %d body=%s", cancel.Code, cancel.Body.String())
	}

	// Mobile place route also blocks.
	appToken := referenceAPILogin(t, router, "building.admin.sudirman@mistypass.local")
	appGuestBody := []byte(`{"name":"App NDA Visitor","phone":"0812 555 0002","host_name":"Host"}`)
	appCreate := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/places/building_demo_001/guests", appToken, appGuestBody)
	if appCreate.Code != http.StatusCreated {
		t.Fatalf("expected app guest create 201, got %d body=%s", appCreate.Code, appCreate.Body.String())
	}
	var appGuest access.Guest
	_ = json.Unmarshal(appCreate.Body.Bytes(), &appGuest)
	appBlocked := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/app/places/building_demo_001/guests/"+appGuest.ID, appToken, []byte(`{"status":"checked_in"}`))
	if appBlocked.Code != http.StatusConflict || !strings.Contains(appBlocked.Body.String(), "nda_required") {
		t.Fatalf("expected app route 409 nda_required, got %d body=%s", appBlocked.Code, appBlocked.Body.String())
	}

	// Sign, then check-in succeeds.
	sign := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests/"+appGuest.ID+"/nda/sign", token, []byte(`{"tenant_id":"tenant_demo_jakarta","signer_name":"App NDA Visitor","signature_data_url":"`+ndaTestSignature+`"}`))
	if sign.Code != http.StatusOK {
		t.Fatalf("expected sign 200, got %d body=%s", sign.Code, sign.Body.String())
	}
	after := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/app/places/building_demo_001/guests/"+appGuest.ID, appToken, []byte(`{"status":"checked_in"}`))
	if after.Code != http.StatusOK {
		t.Fatalf("expected check-in 200 after signing, got %d body=%s", after.Code, after.Body.String())
	}
}

func TestVisitorNDANotRequiredAllowsCheckIn(t *testing.T) {
	router, token := ndaTestRouter(t)
	ndaPutTemplate(t, router, token, `{"tenant_id":"tenant_demo_jakarta","title":"Visitor NDA","body":"Keep secrets.","required":false}`)
	guestID := ndaCreateGuest(t, router, token)

	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/guests/"+guestID+"/status", token, []byte(`{"tenant_id":"tenant_demo_jakarta","status":"checked_in"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected unsigned check-in 200 when not required, got %d body=%s", rec.Code, rec.Body.String())
	}
}
