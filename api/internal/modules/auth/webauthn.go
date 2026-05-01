package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// preReadBody reads the HTTP request body and returns a new request with a
// re-readable body. This allows callers to read the body before acquiring a
// lock, preventing slow clients from holding the lock.
func preReadBody(r *http.Request) (*http.Request, error) {
	if r == nil || r.Body == nil {
		return r, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	return r, nil
}

var ErrWebAuthnNotConfigured = errors.New("webauthn is not configured")
var ErrWebAuthnCredentialNotFound = errors.New("webauthn credential not found")
var ErrWebAuthnSessionExpired = errors.New("webauthn challenge expired or not found")
var ErrWebAuthnNoCredentials = errors.New("user has no webauthn credentials")

// WebAuthnCredential is a stored passkey credential for a user.
type WebAuthnCredential struct {
	ID              string    `json:"id"`               // credential ID (base64url)
	UserID          string    `json:"user_id"`
	PublicKey       []byte    `json:"public_key"`       // COSE public key
	AttestationType string    `json:"attestation_type"` // "none", "packed", etc.
	AAGUID          string    `json:"aaguid"`           // authenticator ID
	SignCount       uint32    `json:"sign_count"`       // replay counter
	DisplayName     string    `json:"display_name"`     // "MacBook Touch ID", "YubiKey 5"
	CreatedAt       time.Time `json:"created_at"`
}

// webAuthnUser wraps auth.User to satisfy the webauthn.User interface.
type webAuthnUser struct {
	user        User
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte {
	return []byte(u.user.ID)
}

func (u *webAuthnUser) WebAuthnName() string {
	return u.user.Email
}

func (u *webAuthnUser) WebAuthnDisplayName() string {
	if u.user.Name != "" {
		return u.user.Name
	}
	return u.user.Email
}

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

// webAuthnSession stores a pending challenge with expiry.
type webAuthnSession struct {
	Data      webauthn.SessionData
	ExpiresAt time.Time
}

const webAuthnSessionTTL = 5 * time.Minute
const webAuthnVerifiedTTL = 2 * time.Minute

// webAuthnVerifiedSession stores proof that a user passed WebAuthn, pending MFA.
type webAuthnVerifiedSession struct {
	UserEmail string
	ExpiresAt time.Time
}

// WebAuthnEngine holds the webauthn library instance and in-memory challenge/credential stores.
type WebAuthnEngine struct {
	mu               sync.RWMutex
	wa               *webauthn.WebAuthn
	sessions         map[string]webAuthnSession      // keyed by userID + purpose ("register" / "login")
	credentials      map[string][]WebAuthnCredential  // keyed by userID
	verifiedSessions map[string]webAuthnVerifiedSession // keyed by token
	persistence      Persistence
}

// NewWebAuthnEngine creates a WebAuthn engine from config values. Returns nil if rpID is empty.
func NewWebAuthnEngine(rpDisplayName, rpID string, rpOrigins []string) (*WebAuthnEngine, error) {
	if strings.TrimSpace(rpID) == "" {
		return nil, nil
	}
	displayName := rpDisplayName
	if displayName == "" {
		displayName = "MistyPass"
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: displayName,
		RPID:          rpID,
		RPOrigins:     rpOrigins,
	})
	if err != nil {
		return nil, err
	}
	return &WebAuthnEngine{
		wa:               wa,
		sessions:         make(map[string]webAuthnSession),
		credentials:      make(map[string][]WebAuthnCredential),
		verifiedSessions: make(map[string]webAuthnVerifiedSession),
	}, nil
}

func (e *WebAuthnEngine) sessionKey(userID, purpose string) string {
	return userID + ":" + purpose
}

func (e *WebAuthnEngine) cleanupExpiredSessions(now time.Time) {
	for key, sess := range e.sessions {
		if sess.ExpiresAt.Before(now) {
			delete(e.sessions, key)
		}
	}
	for token, vs := range e.verifiedSessions {
		if vs.ExpiresAt.Before(now) {
			delete(e.verifiedSessions, token)
		}
	}
}

// CreateVerifiedSession stores a short-lived token proving WebAuthn succeeded for a user.
// Used when MFA is still required after passkey verification.
func (e *WebAuthnEngine) CreateVerifiedSession(email string) (string, error) {
	if e == nil {
		return "", ErrWebAuthnNotConfigured
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)

	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now().UTC()
	e.cleanupExpiredSessions(now)
	e.verifiedSessions[token] = webAuthnVerifiedSession{
		UserEmail: email,
		ExpiresAt: now.Add(webAuthnVerifiedTTL),
	}
	return token, nil
}

// ConsumeVerifiedSession validates and removes a verified session token.
// Returns the user email if valid.
func (e *WebAuthnEngine) ConsumeVerifiedSession(token string) (string, error) {
	if e == nil {
		return "", ErrWebAuthnNotConfigured
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now().UTC()
	e.cleanupExpiredSessions(now)

	vs, exists := e.verifiedSessions[token]
	if !exists {
		return "", ErrWebAuthnSessionExpired
	}
	delete(e.verifiedSessions, token)
	return vs.UserEmail, nil
}

// toLibCredentials converts stored credentials to the library type.
func toLibCredentials(creds []WebAuthnCredential) []webauthn.Credential {
	out := make([]webauthn.Credential, len(creds))
	for i, c := range creds {
		raw, _ := base64.RawURLEncoding.DecodeString(c.ID)
		out[i] = webauthn.Credential{
			ID:              raw,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Authenticator: webauthn.Authenticator{
				SignCount: c.SignCount,
			},
		}
	}
	return out
}

// BeginRegistration starts the passkey registration ceremony.
func (e *WebAuthnEngine) BeginRegistration(user User) (any, error) {
	if e == nil || e.wa == nil {
		return nil, ErrWebAuthnNotConfigured
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.cleanupExpiredSessions(time.Now().UTC())

	e.ensureCredentialsLoaded(user.ID)

	wUser := &webAuthnUser{
		user:        user,
		credentials: toLibCredentials(e.credentials[user.ID]),
	}

	creation, session, err := e.wa.BeginRegistration(wUser)
	if err != nil {
		return nil, err
	}

	e.sessions[e.sessionKey(user.ID, "register")] = webAuthnSession{
		Data:      *session,
		ExpiresAt: time.Now().UTC().Add(webAuthnSessionTTL),
	}

	return creation, nil
}

// FinishRegistration completes registration and stores the credential.
func (e *WebAuthnEngine) FinishRegistration(user User, displayName string, r *http.Request) (*WebAuthnCredential, error) {
	if e == nil || e.wa == nil {
		return nil, ErrWebAuthnNotConfigured
	}

	// Pre-read HTTP body before acquiring lock to avoid blocking on slow clients
	r, err := preReadBody(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now().UTC()
	e.cleanupExpiredSessions(now)

	e.ensureCredentialsLoaded(user.ID)

	key := e.sessionKey(user.ID, "register")
	sess, exists := e.sessions[key]
	if !exists {
		return nil, ErrWebAuthnSessionExpired
	}
	delete(e.sessions, key)

	wUser := &webAuthnUser{
		user:        user,
		credentials: toLibCredentials(e.credentials[user.ID]),
	}

	cred, err := e.wa.FinishRegistration(wUser, sess.Data, r)
	if err != nil {
		return nil, err
	}

	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	aaguid := base64.RawURLEncoding.EncodeToString(cred.Authenticator.AAGUID)
	nextDisplayName := strings.TrimSpace(displayName)
	if nextDisplayName == "" {
		nextDisplayName = "Passkey"
	}

	stored := WebAuthnCredential{
		ID:              credID,
		UserID:          user.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          aaguid,
		SignCount:       cred.Authenticator.SignCount,
		DisplayName:     nextDisplayName,
		CreatedAt:       now,
	}

	e.credentials[user.ID] = append(e.credentials[user.ID], stored)
	if e.persistence != nil {
		if err := e.persistence.UpsertWebAuthnCredential(stored); err != nil {
			// Roll back in-memory state on persistence failure
			creds := e.credentials[user.ID]
			e.credentials[user.ID] = creds[:len(creds)-1]
			return nil, fmt.Errorf("failed to persist credential: %w", err)
		}
	}
	return &stored, nil
}

// BeginLogin starts the passkey login ceremony.
func (e *WebAuthnEngine) BeginLogin(user User) (any, error) {
	if e == nil || e.wa == nil {
		return nil, ErrWebAuthnNotConfigured
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.cleanupExpiredSessions(time.Now().UTC())

	e.ensureCredentialsLoaded(user.ID)

	creds := e.credentials[user.ID]
	if len(creds) == 0 {
		return nil, ErrWebAuthnNoCredentials
	}

	wUser := &webAuthnUser{
		user:        user,
		credentials: toLibCredentials(creds),
	}

	assertion, session, err := e.wa.BeginLogin(wUser)
	if err != nil {
		return nil, err
	}

	e.sessions[e.sessionKey(user.ID, "login")] = webAuthnSession{
		Data:      *session,
		ExpiresAt: time.Now().UTC().Add(webAuthnSessionTTL),
	}

	return assertion, nil
}

// FinishLogin completes the login ceremony, verifies signature and sign count.
func (e *WebAuthnEngine) FinishLogin(user User, r *http.Request) error {
	if e == nil || e.wa == nil {
		return ErrWebAuthnNotConfigured
	}

	// Pre-read HTTP body before acquiring lock to avoid blocking on slow clients
	r, err := preReadBody(r)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now().UTC()
	e.cleanupExpiredSessions(now)

	e.ensureCredentialsLoaded(user.ID)

	key := e.sessionKey(user.ID, "login")
	sess, exists := e.sessions[key]
	if !exists {
		return ErrWebAuthnSessionExpired
	}
	delete(e.sessions, key)

	wUser := &webAuthnUser{
		user:        user,
		credentials: toLibCredentials(e.credentials[user.ID]),
	}

	cred, err := e.wa.FinishLogin(wUser, sess.Data, r)
	if err != nil {
		return err
	}

	// Update sign count on the matching stored credential.
	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	for i, c := range e.credentials[user.ID] {
		if c.ID == credID {
			e.credentials[user.ID][i].SignCount = cred.Authenticator.SignCount
			if e.persistence != nil {
				if persistErr := e.persistence.UpsertWebAuthnCredential(e.credentials[user.ID][i]); persistErr != nil {
					return fmt.Errorf("failed to persist sign count: %w", persistErr)
				}
			}
			break
		}
	}

	return nil
}

// ListCredentials returns the user's registered passkeys.
func (e *WebAuthnEngine) ListCredentials(userID string) []WebAuthnCredential {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureCredentialsLoaded(userID)
	creds := e.credentials[userID]
	out := make([]WebAuthnCredential, len(creds))
	copy(out, creds)
	return out
}

// DeleteCredential removes a passkey by credential ID.
func (e *WebAuthnEngine) DeleteCredential(userID, credentialID string) error {
	if e == nil {
		return ErrWebAuthnNotConfigured
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.ensureCredentialsLoaded(userID)
	creds := e.credentials[userID]
	for i, c := range creds {
		if c.ID == credentialID {
			if e.persistence != nil {
				if err := e.persistence.DeleteWebAuthnCredential(credentialID); err != nil {
					return fmt.Errorf("failed to delete credential: %w", err)
				}
			}
			e.credentials[userID] = append(creds[:i], creds[i+1:]...)
			return nil
		}
	}
	return ErrWebAuthnCredentialNotFound
}

// HasCredentials returns whether the user has any registered passkeys.
func (e *WebAuthnEngine) HasCredentials(userID string) bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureCredentialsLoaded(userID)
	return len(e.credentials[userID]) > 0
}

// SetCredentials bulk-loads credentials (e.g. from persistence on startup).
func (e *WebAuthnEngine) SetCredentials(userID string, creds []WebAuthnCredential) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.credentials[userID] = creds
}

// SetPersistence attaches a persistence store. After this, credential mutations
// are automatically persisted and credentials for a user are loaded on first access.
func (e *WebAuthnEngine) SetPersistence(p Persistence) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.persistence = p
}

// ensureCredentialsLoaded loads credentials from persistence if not already cached.
func (e *WebAuthnEngine) ensureCredentialsLoaded(userID string) {
	if e.persistence == nil {
		return
	}
	if _, cached := e.credentials[userID]; cached {
		return
	}
	creds, err := e.persistence.FindWebAuthnCredentialsByUserID(userID)
	if err != nil || creds == nil {
		return
	}
	e.credentials[userID] = creds
}
