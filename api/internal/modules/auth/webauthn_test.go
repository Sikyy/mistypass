package auth

import (
	"testing"
	"time"
)

func TestNewWebAuthnEngineNilWhenNoRPID(t *testing.T) {
	engine, err := NewWebAuthnEngine("MistyPass", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine != nil {
		t.Fatalf("expected nil engine when RPID is empty")
	}
}

func TestNewWebAuthnEngineCreated(t *testing.T) {
	engine, err := NewWebAuthnEngine("MistyPass", "localhost", []string{"http://localhost:5173"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine == nil {
		t.Fatalf("expected engine to be created")
	}
}

func TestWebAuthnEngineBeginRegistrationWithoutConfig(t *testing.T) {
	var engine *WebAuthnEngine
	_, err := engine.BeginRegistration(User{ID: "user1", Email: "a@b.com"})
	if err != ErrWebAuthnNotConfigured {
		t.Fatalf("expected ErrWebAuthnNotConfigured, got %v", err)
	}
}

func TestWebAuthnEngineBeginLoginNoCredentials(t *testing.T) {
	engine, _ := NewWebAuthnEngine("MistyPass", "localhost", []string{"http://localhost:5173"})
	_, err := engine.BeginLogin(User{ID: "user1", Email: "a@b.com"})
	if err != ErrWebAuthnNoCredentials {
		t.Fatalf("expected ErrWebAuthnNoCredentials, got %v", err)
	}
}

func TestWebAuthnEngineSetAndListCredentials(t *testing.T) {
	engine, _ := NewWebAuthnEngine("MistyPass", "localhost", []string{"http://localhost:5173"})

	creds := []WebAuthnCredential{
		{ID: "cred1", UserID: "user1", DisplayName: "MacBook", CreatedAt: time.Now()},
		{ID: "cred2", UserID: "user1", DisplayName: "YubiKey", CreatedAt: time.Now()},
	}
	engine.SetCredentials("user1", creds)

	listed := engine.ListCredentials("user1")
	if len(listed) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(listed))
	}
	if listed[0].DisplayName != "MacBook" {
		t.Errorf("expected MacBook, got %s", listed[0].DisplayName)
	}
}

func TestWebAuthnEngineDeleteCredential(t *testing.T) {
	engine, _ := NewWebAuthnEngine("MistyPass", "localhost", []string{"http://localhost:5173"})

	engine.SetCredentials("user1", []WebAuthnCredential{
		{ID: "cred1", UserID: "user1", DisplayName: "MacBook"},
		{ID: "cred2", UserID: "user1", DisplayName: "YubiKey"},
	})

	err := engine.DeleteCredential("user1", "cred1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listed := engine.ListCredentials("user1")
	if len(listed) != 1 {
		t.Fatalf("expected 1 credential after delete, got %d", len(listed))
	}
	if listed[0].ID != "cred2" {
		t.Errorf("expected cred2, got %s", listed[0].ID)
	}

	err = engine.DeleteCredential("user1", "nonexistent")
	if err != ErrWebAuthnCredentialNotFound {
		t.Fatalf("expected ErrWebAuthnCredentialNotFound, got %v", err)
	}
}

func TestWebAuthnEngineHasCredentials(t *testing.T) {
	engine, _ := NewWebAuthnEngine("MistyPass", "localhost", []string{"http://localhost:5173"})

	if engine.HasCredentials("user1") {
		t.Errorf("expected no credentials initially")
	}

	engine.SetCredentials("user1", []WebAuthnCredential{
		{ID: "cred1", UserID: "user1"},
	})

	if !engine.HasCredentials("user1") {
		t.Errorf("expected credentials after set")
	}
}

func TestWebAuthnSessionExpiry(t *testing.T) {
	engine, _ := NewWebAuthnEngine("MistyPass", "localhost", []string{"http://localhost:5173"})

	// Manually add an expired session
	engine.mu.Lock()
	engine.sessions["user1:register"] = webAuthnSession{
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	engine.mu.Unlock()

	// Finish should fail with session expired
	_, err := engine.FinishRegistration(User{ID: "user1", Email: "a@b.com"}, "test", nil)
	if err != ErrWebAuthnSessionExpired {
		t.Fatalf("expected ErrWebAuthnSessionExpired, got %v", err)
	}
}

func TestWebAuthnNilEngineSafety(t *testing.T) {
	var engine *WebAuthnEngine

	if engine.HasCredentials("x") {
		t.Error("nil engine should return false for HasCredentials")
	}
	if creds := engine.ListCredentials("x"); creds != nil {
		t.Error("nil engine should return nil for ListCredentials")
	}
	if err := engine.DeleteCredential("x", "y"); err != ErrWebAuthnNotConfigured {
		t.Errorf("nil engine DeleteCredential should return ErrWebAuthnNotConfigured, got %v", err)
	}

	engine.SetCredentials("x", nil) // should not panic
	engine.SetPersistence(nil)      // should not panic
}
