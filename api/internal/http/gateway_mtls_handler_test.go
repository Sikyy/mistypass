package httpx

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayMTLSOnlyHandlerAllowsRegisterWithoutClientCertificate(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gatewayMTLSRequired(r) {
			t.Fatalf("register should not require existing mTLS certificate")
		}
		w.WriteHeader(http.StatusCreated)
	})
	handler := GatewayMTLSOnlyHandler(next)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/gateway/register", nil))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected register to pass through, got %d", rec.Code)
	}
}

func TestGatewayMTLSOnlyHandlerRequiresVerifiedClientCertificateAfterRegister(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !gatewayMTLSRequired(r) {
			t.Fatalf("expected gateway mTLS requirement marker")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := GatewayMTLSOnlyHandler(next)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected heartbeat without cert to fail, got %d", rec.Code)
	}

	leaf := &x509.Certificate{Subject: pkix.Name{CommonName: "gw_demo_001"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected heartbeat with verified cert to pass, got %d", rec.Code)
	}
}

func TestGatewayMTLSOnlyHandlerRequiresVerifiedClientCertificateForDeviceRoutes(t *testing.T) {
	handler := GatewayMTLSOnlyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !gatewayMTLSRequired(r) {
			t.Fatalf("expected gateway mTLS requirement marker for %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	leaf := &x509.Certificate{Subject: pkix.Name{CommonName: "gw_demo_001"}}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/gateway/credentials/sync"},
		{method: http.MethodPost, path: "/api/v1/gateway/audit/batch"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected %s without cert to fail, got %d", tc.path, rec.Code)
		}

		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.TLS = &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{leaf},
			VerifiedChains:   [][]*x509.Certificate{{leaf}},
		}
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected %s with verified cert to pass, got %d", tc.path, rec.Code)
		}
	}
}

func TestGatewayMTLSOnlyHandlerHidesNonGatewayRoutes(t *testing.T) {
	handler := GatewayMTLSOnlyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{
		"/api/v1/auth/me",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %s to be hidden, got %d", path, rec.Code)
		}
	}
}
