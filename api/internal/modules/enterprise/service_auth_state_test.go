package enterprise

import (
	"testing"
	"time"
)

func TestStartAndConsumeAuthStateTokenSuccess(t *testing.T) {
	svc := NewService()

	stateToken, err := svc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		"jit.employee@sudirman.co",
		"https://admin.mistypass.local/enterprise/callback",
		2*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected state token creation to succeed: %v", err)
	}
	if stateToken.Token == "" {
		t.Fatalf("expected non-empty state token")
	}
	if stateToken.Provider != "oidc" {
		t.Fatalf("unexpected provider: %s", stateToken.Provider)
	}

	consumed, err := svc.ConsumeAuthStateToken(stateToken.Token, "oidc")
	if err != nil {
		t.Fatalf("expected state token consume to succeed: %v", err)
	}
	if consumed.Token != stateToken.Token {
		t.Fatalf("unexpected consumed token: got=%s want=%s", consumed.Token, stateToken.Token)
	}

	_, err = svc.ConsumeAuthStateToken(stateToken.Token, "oidc")
	if err == nil {
		t.Fatalf("expected state token to be one-time consumable")
	}
	if err != ErrAuthStateTokenNotFound {
		t.Fatalf("unexpected consume error: %v", err)
	}
}

func TestConsumeAuthStateTokenProviderMismatch(t *testing.T) {
	svc := NewService()

	stateToken, err := svc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"saml",
		"jit.employee@sudirman.co",
		"https://admin.mistypass.local/enterprise/callback",
		2*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected state token creation to succeed: %v", err)
	}

	_, err = svc.ConsumeAuthStateToken(stateToken.Token, "oidc")
	if err == nil {
		t.Fatalf("expected provider mismatch to fail")
	}
	if err != ErrAuthStateProviderMismatch {
		t.Fatalf("unexpected provider mismatch error: %v", err)
	}
}

func TestConsumeAuthStateTokenExpired(t *testing.T) {
	svc := NewService()

	stateToken, err := svc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		"jit.employee@sudirman.co",
		"http://localhost:5173/enterprise/callback",
		1*time.Nanosecond,
	)
	if err != nil {
		t.Fatalf("expected state token creation to succeed: %v", err)
	}
	if stateToken.Token == "" {
		t.Fatalf("expected non-empty token")
	}
	time.Sleep(1 * time.Millisecond)

	_, err = svc.ConsumeAuthStateToken(stateToken.Token, "oidc")
	if err == nil {
		t.Fatalf("expected expired token consume to fail")
	}
	if err != ErrAuthStateTokenNotFound {
		t.Fatalf("unexpected expired token error: %v", err)
	}
}

func TestStartAuthStateTokenInvalidRedirectURI(t *testing.T) {
	svc := NewService()

	_, err := svc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		"jit.employee@sudirman.co",
		"javascript:alert(1)",
		1*time.Minute,
	)
	if err == nil {
		t.Fatalf("expected invalid redirect uri to fail")
	}
	if err != ErrInvalidRedirectURI {
		t.Fatalf("unexpected redirect uri error: %v", err)
	}
}
