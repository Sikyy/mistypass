package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func rolloutTestServer(t *testing.T) (*server, gateway.GatewayFirmware) {
	t.Helper()
	svc := gateway.NewService()
	fw, err := svc.CreateFirmware(gateway.CreateFirmwareInput{
		TenantID: "tenant_demo_jakarta", Version: "1.4.0",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: strings.Repeat("b", 128),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &server{gatewaySvc: svc, cfg: config.Config{UploadStorageDir: t.TempDir(), UploadSigningKey: "k"}}, fw
}

func rolloutTestReq(method, target string, body any) *http.Request {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, target, bytes.NewReader(b))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	return withGatewayMQTTUser(r, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
}

func withRolloutIDParam(r *http.Request, id string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add("rolloutID", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func TestCreateAndGetRolloutEndpoint(t *testing.T) {
	s, fw := rolloutTestServer(t)
	rec := httptest.NewRecorder()
	s.createGatewayRollout(rec, rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID,
		"target":      map[string]any{"kind": "all"},
		"phases":      []map[string]any{{"percentage": 100}},
	}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created gateway.GatewayRollout
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" || created.State != "active" {
		t.Fatalf("unexpected created rollout: %+v", created)
	}

	dRec := httptest.NewRecorder()
	dReq := withRolloutIDParam(rolloutTestReq(http.MethodGet, "/api/v1/gateways/rollouts/"+created.ID+"?tenant_id=tenant_demo_jakarta", nil), created.ID)
	s.getGatewayRollout(dRec, dReq)
	if dRec.Code != http.StatusOK {
		t.Fatalf("detail expected 200, got %d body=%s", dRec.Code, dRec.Body.String())
	}
	var detail struct {
		Rollout  gateway.GatewayRollout         `json:"rollout"`
		Gateways []gateway.RolloutGatewayStatus `json:"gateways"`
	}
	_ = json.Unmarshal(dRec.Body.Bytes(), &detail)
	if detail.Rollout.ID != created.ID || len(detail.Gateways) == 0 {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	badRec := httptest.NewRecorder()
	s.createGatewayRollout(badRec, rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": "fw_nope", "target": map[string]any{"kind": "all"}, "phases": []map[string]any{{"percentage": 100}},
	}))
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("unknown firmware expected 404, got %d", badRec.Code)
	}
}

func TestRolloutAbortEndpoint(t *testing.T) {
	s, fw := rolloutTestServer(t)
	rec := httptest.NewRecorder()
	s.createGatewayRollout(rec, rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID, "target": map[string]any{"kind": "all"}, "phases": []map[string]any{{"percentage": 100}},
	}))
	var created gateway.GatewayRollout
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	aRec := httptest.NewRecorder()
	aReq := withRolloutIDParam(rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts/"+created.ID+"/abort?tenant_id=tenant_demo_jakarta", nil), created.ID)
	s.abortGatewayRollout(aRec, aReq)
	if aRec.Code != http.StatusOK {
		t.Fatalf("abort expected 200, got %d body=%s", aRec.Code, aRec.Body.String())
	}
	var aborted gateway.GatewayRollout
	_ = json.Unmarshal(aRec.Body.Bytes(), &aborted)
	if aborted.State != "failed" {
		t.Fatalf("want failed, got %s", aborted.State)
	}
}

func TestRolloutActionConflictAndNotFound(t *testing.T) {
	s, fw := rolloutTestServer(t)
	rec := httptest.NewRecorder()
	s.createGatewayRollout(rec, rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID, "target": map[string]any{"kind": "all"}, "phases": []map[string]any{{"percentage": 100}},
	}))
	var created gateway.GatewayRollout
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	abort := func() *httptest.ResponseRecorder {
		ar := httptest.NewRecorder()
		s.abortGatewayRollout(ar, withRolloutIDParam(rolloutTestReq(http.MethodPost, "/x?tenant_id=tenant_demo_jakarta", nil), created.ID))
		return ar
	}
	abort()                                              // → failed
	if got := abort(); got.Code != http.StatusConflict { // abort on a failed rollout → 409
		t.Fatalf("conflict expected 409, got %d body=%s", got.Code, got.Body.String())
	}

	nf := httptest.NewRecorder()
	s.getGatewayRollout(nf, withRolloutIDParam(rolloutTestReq(http.MethodGet, "/x?tenant_id=tenant_demo_jakarta", nil), "rollout_nope"))
	if nf.Code != http.StatusNotFound {
		t.Fatalf("unknown rollout expected 404, got %d", nf.Code)
	}

	lr := httptest.NewRecorder()
	s.listGatewayRollouts(lr, rolloutTestReq(http.MethodGet, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", nil))
	if lr.Code != http.StatusOK || !strings.Contains(lr.Body.String(), created.ID) {
		t.Fatalf("list expected 200 containing %s, got %d %s", created.ID, lr.Code, lr.Body.String())
	}
}
