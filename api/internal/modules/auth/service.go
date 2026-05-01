package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidRefreshToken = errors.New("invalid refresh token")
var ErrInvalidAccessToken = errors.New("invalid access token")
var ErrUserNotFound = errors.New("user not found")
var ErrUserRoleUnsupported = errors.New("user role does not support building scope")
var ErrAdminMFAEnrollmentRequired = errors.New("admin mfa enrollment is required")
var ErrAdminMFARequired = errors.New("admin mfa code is required")
var ErrInvalidMFACode = errors.New("invalid admin mfa code")
var ErrAdminMFANotConfigured = errors.New("admin mfa is not configured")
var ErrPasswordTooWeak = errors.New("password must be at least 8 characters with uppercase, lowercase, and digit")
var ErrPasswordResetTokenInvalid = errors.New("password reset token is invalid or expired")
var ErrPasswordResetTokenExpired = errors.New("password reset token has expired")

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         User   `json:"user"`
}

type User struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name,omitempty"`
	Email                string   `json:"email"`
	Role                 string   `json:"role"`
	TenantID             string   `json:"tenant_id"`
	BuildingIDs          []string `json:"building_ids,omitempty"`
	Language             string   `json:"language,omitempty"`
	PasswordAuthEnabled  bool     `json:"password_auth_enabled"` // false = SSO-only, password login rejected
}

var ErrPasswordAuthDisabled = errors.New("password authentication is disabled for this user, use SSO")

type tokenClaims struct {
	UserID      string   `json:"user_id"`
	Name        string   `json:"name,omitempty"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	TenantID    string   `json:"tenant_id,omitempty"`
	BuildingIDs []string `json:"building_ids,omitempty"`
	Language    string   `json:"language,omitempty"`
	TokenType   string   `json:"token_type"`
	jwt.RegisteredClaims
}

type userRecord struct {
	User         User
	PasswordHash []byte
}

type refreshSession struct {
	UserID      string
	ExpiresAt   time.Time
	IPAddress   string
	UserAgent   string
	LoginMethod string // "password", "sso", "webauthn"
	CreatedAt   time.Time
}

// LoginSession is the public view of an active session.
type LoginSession struct {
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	LoginMethod string    `json:"login_method,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// SessionMetadata carries request context for session tracking.
type SessionMetadata struct {
	IPAddress   string
	UserAgent   string
	LoginMethod string
}

type adminMFAState struct {
	Secret        string
	PendingSecret string
	Enabled       bool
	RecoveryCodes []mfaRecoveryCode // one-time backup codes
	UpdatedAt     time.Time
}

type mfaRecoveryCode struct {
	CodeHash string // bcrypt hash of the code
	Used     bool
}

type AdminMFAStatus struct {
	UserID    string     `json:"user_id"`
	Enabled   bool       `json:"enabled"`
	Pending   bool       `json:"pending"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type AdminMFARecoveryCodes struct {
	UserID string   `json:"user_id"`
	Codes  []string `json:"codes"` // plaintext codes, shown only once
}

type AdminMFAEnrollment struct {
	UserID     string `json:"user_id"`
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type AdminMFAPersistenceState struct {
	Secret        string
	PendingSecret string
	Enabled       bool
	RecoveryCodes []MFARecoveryCodePersistence `json:"recovery_codes,omitempty"`
	UpdatedAt     time.Time
}

type MFARecoveryCodePersistence struct {
	CodeHash string `json:"code_hash"`
	Used     bool   `json:"used"`
}

type Persistence interface {
	UpsertAuthUser(user User, passwordHash []byte) error
	FindAuthUserByEmail(email string) (User, []byte, bool, error)
	FindAuthUserByID(userID string) (User, []byte, bool, error)
	UpsertAuthRefreshSession(sessionID, userID string, expiresAt time.Time) error
	FindAuthRefreshSession(sessionID string) (string, time.Time, bool, error)
	DeleteAuthRefreshSession(sessionID string) error
	DeleteAuthRefreshSessionsByUserID(userID string) (int, error)
	UpsertAuthRevokedAccessToken(tokenID string, expiresAt time.Time) error
	IsAuthAccessTokenRevoked(tokenID string, now time.Time) (bool, error)
	UpsertAuthAdminMFAState(userID string, state AdminMFAPersistenceState) error
	FindAuthAdminMFAState(userID string) (AdminMFAPersistenceState, bool, error)
	UpsertWebAuthnCredential(cred WebAuthnCredential) error
	FindWebAuthnCredentialsByUserID(userID string) ([]WebAuthnCredential, error)
	DeleteWebAuthnCredential(credentialID string) error
}

type VolatileStore interface {
	UpsertAuthRefreshSession(sessionID, userID string, expiresAt time.Time) error
	FindAuthRefreshSession(sessionID string) (string, time.Time, bool, error)
	DeleteAuthRefreshSession(sessionID string) error
	DeleteAuthRefreshSessionsByUserID(userID string) (int, error)
	UpsertAuthRevokedAccessToken(tokenID string, expiresAt time.Time) error
	IsAuthAccessTokenRevoked(tokenID string, now time.Time) (bool, error)
}

type passwordResetToken struct {
	Email     string
	ExpiresAt time.Time
}

type Service struct {
	mu                  sync.RWMutex
	signingKey          []byte
	issuer              string
	accessTTL           time.Duration
	refreshTTL          time.Duration
	adminMFARequired    bool
	adminMFA            map[string]adminMFAState
	persistence         Persistence
	volatileStore       VolatileStore
	usersByEmail        map[string]userRecord
	usersByID           map[string]User
	refreshSessions     map[string]refreshSession
	revokedAccess       map[string]time.Time
	passwordResetTokens map[string]passwordResetToken
}

const defaultJWTIssuer = "mistypass-api"
const defaultAccessTTL = time.Hour
const defaultRefreshTTL = 7 * 24 * time.Hour

func NewService(secret, issuer string, accessTTL, refreshTTL time.Duration, enableDemoUsers bool) *Service {
	nextSecret := strings.TrimSpace(secret)
	if nextSecret == "" {
		nextSecret = ephemeralJWTSecret()
	}
	nextIssuer := strings.TrimSpace(issuer)
	if nextIssuer == "" {
		nextIssuer = defaultJWTIssuer
	}
	nextAccessTTL := accessTTL
	if nextAccessTTL <= 0 {
		nextAccessTTL = defaultAccessTTL
	}
	nextRefreshTTL := refreshTTL
	if nextRefreshTTL <= 0 {
		nextRefreshTTL = defaultRefreshTTL
	}

	users := []userRecord{}
	if enableDemoUsers {
		users = buildDemoUsers()
	}

	usersByEmail := make(map[string]userRecord, len(users))
	usersByID := make(map[string]User, len(users))
	for i := range users {
		email := normalizeEmail(users[i].User.Email)
		usersByEmail[email] = users[i]
		usersByID[users[i].User.ID] = users[i].User
	}

	return &Service{
		signingKey:          []byte(nextSecret),
		issuer:              nextIssuer,
		accessTTL:           nextAccessTTL,
		refreshTTL:          nextRefreshTTL,
		adminMFA:            make(map[string]adminMFAState),
		usersByEmail:        usersByEmail,
		usersByID:           usersByID,
		refreshSessions:     make(map[string]refreshSession),
		revokedAccess:       make(map[string]time.Time),
		passwordResetTokens: make(map[string]passwordResetToken),
	}
}

func (s *Service) SetPersistence(store Persistence) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if store == nil {
		s.persistence = nil
		return nil
	}

	for _, record := range s.usersByEmail {
		if err := store.UpsertAuthUser(record.User, record.PasswordHash); err != nil {
			return err
		}
	}
	if s.volatileStore == nil {
		for sessionID, session := range s.refreshSessions {
			if err := store.UpsertAuthRefreshSession(sessionID, session.UserID, session.ExpiresAt); err != nil {
				return err
			}
		}
		for tokenID, expiresAt := range s.revokedAccess {
			if err := store.UpsertAuthRevokedAccessToken(tokenID, expiresAt); err != nil {
				return err
			}
		}
	}
	for userID, state := range s.adminMFA {
		if err := store.UpsertAuthAdminMFAState(userID, adminMFAStateForPersistence(state)); err != nil {
			return err
		}
	}
	s.persistence = store
	return nil
}

func (s *Service) SetVolatileStore(store VolatileStore) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if store == nil {
		s.volatileStore = nil
		return nil
	}

	for sessionID, session := range s.refreshSessions {
		if err := store.UpsertAuthRefreshSession(sessionID, session.UserID, session.ExpiresAt); err != nil {
			return err
		}
	}
	for tokenID, expiresAt := range s.revokedAccess {
		if err := store.UpsertAuthRevokedAccessToken(tokenID, expiresAt); err != nil {
			return err
		}
	}
	s.volatileStore = store
	return nil
}

func (s *Service) SetAdminMFARequired(required bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adminMFARequired = required
}

func (s *Service) Login(request LoginRequest, meta ...SessionMetadata) (LoginResponse, error) {
	email := normalizeEmail(request.Email)
	password := strings.TrimSpace(request.Password)
	if email == "" || password == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	record, exists, err := s.findUserByEmail(email)
	if err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if !exists || !verifyPassword(record.PasswordHash, password) {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if !record.User.PasswordAuthEnabled {
		return LoginResponse{}, ErrPasswordAuthDisabled
	}
	if err := s.enforceAdminMFA(record.User, request.MFACode); err != nil {
		return LoginResponse{}, err
	}

	m := SessionMetadata{LoginMethod: "password"}
	if len(meta) > 0 {
		m = meta[0]
		if m.LoginMethod == "" {
			m.LoginMethod = "password"
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())
	return s.issueTokenPairWithMetadataLocked(record.User, m)
}

func (s *Service) LoginByTrustedIdentity(email string, meta ...SessionMetadata) (LoginResponse, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	record, exists, err := s.findUserByEmail(nextEmail)
	if err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if !exists {
		return LoginResponse{}, ErrInvalidCredentials
	}

	m := SessionMetadata{LoginMethod: "sso"}
	if len(meta) > 0 {
		m = meta[0]
		if m.LoginMethod == "" {
			m.LoginMethod = "sso"
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())
	return s.issueTokenPairWithMetadataLocked(record.User, m)
}

// LoginByWebAuthn issues a token pair after WebAuthn authentication succeeds.
// WebAuthn replaces the password but not MFA — TOTP is still enforced if enabled.
func (s *Service) LoginByWebAuthn(email, mfaCode string, meta ...SessionMetadata) (LoginResponse, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	record, exists, err := s.findUserByEmail(nextEmail)
	if err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if !exists {
		return LoginResponse{}, ErrInvalidCredentials
	}

	if err := s.enforceAdminMFA(record.User, mfaCode); err != nil {
		return LoginResponse{}, err
	}

	m := SessionMetadata{LoginMethod: "webauthn"}
	if len(meta) > 0 {
		m = meta[0]
		if m.LoginMethod == "" {
			m.LoginMethod = "webauthn"
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())
	return s.issueTokenPairWithMetadataLocked(record.User, m)
}

func (s *Service) LoginByTrustedUser(user User) (LoginResponse, error) {
	nextUserID := strings.TrimSpace(user.ID)
	nextName := strings.TrimSpace(user.Name)
	nextEmail := normalizeEmail(user.Email)
	nextRole := strings.ToLower(strings.TrimSpace(user.Role))
	nextTenantID := strings.TrimSpace(user.TenantID)
	nextBuildingIDs := uniqueNormalizedIDs(user.BuildingIDs)
	nextLanguage := normalizeLanguage(user.Language)

	if nextUserID == "" || nextEmail == "" || nextRole == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	trustedUser := User{
		ID:          nextUserID,
		Name:        nextName,
		Email:       nextEmail,
		Role:        nextRole,
		TenantID:    nextTenantID,
		BuildingIDs: nextBuildingIDs,
		Language:    nextLanguage,
	}

	passwordHash := []byte(nil)
	record, exists, err := s.findUserByEmail(nextEmail)
	if err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if exists {
		passwordHash = record.PasswordHash
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())
	if err := s.persistUserLocked(trustedUser, passwordHash); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}

	return s.issueTokenPairLocked(trustedUser)
}

func (s *Service) Refresh(request RefreshRequest) (LoginResponse, error) {
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	claims, err := s.parseTokenClaims(refreshToken, "refresh")
	if err != nil {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	now := time.Now().UTC()
	session, exists, err := s.findRefreshSession(claims.ID)
	if err != nil {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if !exists || session.UserID != claims.UserID || session.ExpiresAt.Before(now) {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	user, exists, err := s.findUserByID(claims.UserID)
	if err != nil {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if !exists {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	session, exists, err = s.findRefreshSessionLocked(claims.ID)
	if err != nil {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if !exists || session.UserID != claims.UserID || session.ExpiresAt.Before(now) {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if err := s.deleteRefreshSessionLocked(claims.ID); err != nil {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	return s.issueTokenPairLocked(user)
}

func (s *Service) RevokeRefreshToken(refreshToken string) error {
	nextRefreshToken := strings.TrimSpace(refreshToken)
	if nextRefreshToken == "" {
		return ErrInvalidRefreshToken
	}

	claims, err := s.parseTokenClaims(nextRefreshToken, "refresh")
	if err != nil {
		return ErrInvalidRefreshToken
	}

	now := time.Now().UTC()
	session, exists, err := s.findRefreshSession(claims.ID)
	if err != nil {
		return ErrInvalidRefreshToken
	}
	if !exists || session.UserID != claims.UserID || !session.ExpiresAt.After(now) {
		return ErrInvalidRefreshToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	session, exists, err = s.findRefreshSessionLocked(claims.ID)
	if err != nil {
		return ErrInvalidRefreshToken
	}
	if !exists || session.UserID != claims.UserID || !session.ExpiresAt.After(now) {
		return ErrInvalidRefreshToken
	}
	if err := s.deleteRefreshSessionLocked(claims.ID); err != nil {
		return ErrInvalidRefreshToken
	}
	return nil
}

func (s *Service) RevokeRefreshTokensByUserEmail(email string) int {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return 0
	}

	now := time.Now().UTC()
	record, exists, err := s.findUserByEmail(nextEmail)
	if err != nil {
		return 0
	}
	if !exists {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	revoked := 0
	localRevoked := 0
	if s.volatileStore != nil {
		volatileRevoked, volatileErr := s.volatileStore.DeleteAuthRefreshSessionsByUserID(record.User.ID)
		if volatileErr == nil && volatileRevoked > revoked {
			revoked = volatileRevoked
		}
	} else if s.persistence != nil {
		persistedRevoked, persistErr := s.persistence.DeleteAuthRefreshSessionsByUserID(record.User.ID)
		if persistErr == nil && persistedRevoked > revoked {
			revoked = persistedRevoked
		}
	}
	for sessionID, session := range s.refreshSessions {
		if session.UserID != record.User.ID {
			continue
		}
		delete(s.refreshSessions, sessionID)
		localRevoked++
	}
	if localRevoked > revoked {
		revoked = localRevoked
	}
	return revoked
}

func (s *Service) DowngradeTrustedUserToLeastPrivilegeByEmail(email string, fallbackTenantID string) (User, User, bool) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return User{}, User{}, false
	}
	nextFallbackTenantID := strings.TrimSpace(fallbackTenantID)
	record, exists, err := s.findUserByEmail(nextEmail)
	if err != nil {
		return User{}, User{}, false
	}
	if !exists {
		return User{}, User{}, false
	}

	before := record.User
	before.BuildingIDs = append([]string(nil), before.BuildingIDs...)

	after := before
	after.Role = "resident"
	after.BuildingIDs = nil
	if strings.TrimSpace(after.TenantID) == "" && nextFallbackTenantID != "" {
		after.TenantID = nextFallbackTenantID
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())
	if err := s.persistUserLocked(after, record.PasswordHash); err != nil {
		return User{}, User{}, false
	}

	return before, after, true
}

func (s *Service) Me(accessToken string) (User, error) {
	return s.VerifyAccessToken(accessToken)
}

func (s *Service) Logout(accessToken string) error {
	claims, err := s.parseTokenClaims(strings.TrimSpace(accessToken), "access")
	if err != nil {
		return ErrInvalidAccessToken
	}

	if claims.ID == "" || claims.ExpiresAt == nil {
		return ErrInvalidAccessToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())
	if err := s.revokeAccessTokenLocked(claims.ID, claims.ExpiresAt.Time); err != nil {
		return ErrInvalidAccessToken
	}

	return nil
}

func (s *Service) VerifyAccessToken(accessToken string) (User, error) {
	claims, err := s.parseTokenClaims(strings.TrimSpace(accessToken), "access")
	if err != nil {
		return User{}, ErrInvalidAccessToken
	}

	now := time.Now().UTC()
	revoked, err := s.isAccessTokenRevoked(claims.ID, now)
	if err != nil {
		return User{}, ErrInvalidAccessToken
	}
	if revoked {
		return User{}, ErrInvalidAccessToken
	}

	user, exists, err := s.findUserByID(claims.UserID)
	if err != nil {
		return User{}, ErrInvalidAccessToken
	}
	if exists {
		return user, nil
	}

	return User{
		ID:          claims.UserID,
		Name:        claims.Name,
		Email:       claims.Email,
		Role:        claims.Role,
		TenantID:    claims.TenantID,
		BuildingIDs: append([]string(nil), claims.BuildingIDs...),
		Language:    claims.Language,
	}, nil
}

func (s *Service) GetUserByID(userID string) (User, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, ErrUserNotFound
	}

	user, exists, err := s.findUserByID(nextUserID)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	if !exists {
		return User{}, ErrUserNotFound
	}

	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, nil
}

// ListActiveSessions returns all non-expired refresh sessions for a user.
func (s *Service) ListActiveSessions(userID string) []LoginSession {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return nil
	}

	now := time.Now().UTC()
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []LoginSession
	for sessionID, session := range s.refreshSessions {
		if session.UserID != nextUserID || session.ExpiresAt.Before(now) {
			continue
		}
		sessions = append(sessions, LoginSession{
			SessionID:   sessionID,
			UserID:      session.UserID,
			IPAddress:   session.IPAddress,
			UserAgent:   session.UserAgent,
			LoginMethod: session.LoginMethod,
			CreatedAt:   session.CreatedAt,
			ExpiresAt:   session.ExpiresAt,
		})
	}
	return sessions
}

// RevokeSession revokes a specific refresh session for a user.
func (s *Service) RevokeSession(userID, sessionID string) error {
	nextUserID := strings.TrimSpace(userID)
	nextSessionID := strings.TrimSpace(sessionID)
	if nextUserID == "" || nextSessionID == "" {
		return ErrInvalidRefreshToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.refreshSessions[nextSessionID]
	if !exists || session.UserID != nextUserID {
		return ErrInvalidRefreshToken
	}

	return s.deleteRefreshSessionLocked(nextSessionID)
}

const passwordResetTokenTTL = 15 * time.Minute

// RequestPasswordReset generates a one-time reset token for the given email.
// Returns the token string. The caller is responsible for delivering it (e.g. via email).
// Returns empty string (no error) if the user doesn't exist to avoid email enumeration.
func (s *Service) RequestPasswordReset(email string) (string, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return "", nil
	}

	_, exists, err := s.findUserByEmail(nextEmail)
	if err != nil || !exists {
		return "", nil // silent — don't reveal whether the user exists
	}

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := fmt.Sprintf("prt_%x", buf)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Cleanup expired tokens
	now := time.Now().UTC()
	for k, t := range s.passwordResetTokens {
		if t.ExpiresAt.Before(now) {
			delete(s.passwordResetTokens, k)
		}
	}

	s.passwordResetTokens[token] = passwordResetToken{
		Email:     nextEmail,
		ExpiresAt: now.Add(passwordResetTokenTTL),
	}
	return token, nil
}

// ConfirmPasswordReset validates the token and sets a new password.
func (s *Service) ConfirmPasswordReset(token, newPassword string) error {
	nextToken := strings.TrimSpace(token)
	if nextToken == "" {
		return ErrPasswordResetTokenInvalid
	}

	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	prt, exists := s.passwordResetTokens[nextToken]
	if !exists {
		return ErrPasswordResetTokenInvalid
	}
	delete(s.passwordResetTokens, nextToken)

	if prt.ExpiresAt.Before(now) {
		return ErrPasswordResetTokenExpired
	}

	record, exists, err := s.findUserByEmailLocked(prt.Email)
	if err != nil || !exists {
		return ErrUserNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.persistUserLocked(record.User, hash)
}

// CreateUser creates a new user with email and password (self-signup).
func (s *Service) CreateUser(email, name, password string) (User, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return User{}, errors.New("email is required")
	}
	if err := ValidatePasswordPolicy(password); err != nil {
		return User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	userID, err := randomTokenID(8)
	if err != nil {
		return User{}, err
	}
	user := User{
		ID:                  "usr_" + userID,
		Name:                strings.TrimSpace(name),
		Email:               nextEmail,
		Role:                "resident",
		PasswordAuthEnabled: true,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Check existence under lock to prevent TOCTOU race
	if _, exists, _ := s.findUserByEmailLocked(nextEmail); exists {
		return User{}, errors.New("user already exists")
	}
	if err := s.persistUserLocked(user, hash); err != nil {
		return User{}, err
	}
	return user, nil
}

// ChangePassword validates the current password and sets a new one.
func (s *Service) ChangePassword(userID, currentPassword, newPassword string) error {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return ErrUserNotFound
	}
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}

	// Find user outside lock for bcrypt verification (acceptable if slightly stale)
	user, exists, err := s.findUserByID(nextUserID)
	if err != nil || !exists {
		return ErrUserNotFound
	}
	record, recordExists, _ := s.findUserByEmail(user.Email)
	if !recordExists {
		return ErrUserNotFound
	}
	if !verifyPassword(record.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Re-verify user exists under lock
	lockedRecord, lockedExists, _ := s.findUserByEmailLocked(user.Email)
	if !lockedExists {
		return ErrUserNotFound
	}
	return s.persistUserLocked(lockedRecord.User, hash)
}

func (s *Service) FindUserByEmail(email string) (User, error) {
	record, exists, err := s.findUserByEmail(email)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	if !exists {
		return User{}, ErrUserNotFound
	}
	return record.User, nil
}

func (s *Service) UpdateUserBuildingScope(userID string, buildingIDs []string) (User, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, ErrUserNotFound
	}

	nextBuildingIDs := uniqueNormalizedIDs(buildingIDs)

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	if !exists {
		return User{}, ErrUserNotFound
	}

	if strings.ToLower(strings.TrimSpace(user.Role)) != "building_admin" {
		return User{}, ErrUserRoleUnsupported
	}

	user.BuildingIDs = nextBuildingIDs
	passwordHash := []byte(nil)
	record, exists, err := s.findUserByEmailLocked(user.Email)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	if exists {
		passwordHash = record.PasswordHash
	}
	if err := s.persistUserLocked(user, passwordHash); err != nil {
		return User{}, ErrUserNotFound
	}

	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, nil
}

func (s *Service) UpdateUserPasswordAuth(userID string, enabled bool) (User, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, ErrUserNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return User{}, ErrUserNotFound
	}

	user.PasswordAuthEnabled = enabled
	passwordHash := []byte(nil)
	record, exists, err := s.findUserByEmailLocked(user.Email)
	if err == nil && exists {
		passwordHash = record.PasswordHash
	}
	if err := s.persistUserLocked(user, passwordHash); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *Service) UpdateUserProfile(userID, name, language string) (User, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, ErrUserNotFound
	}
	nextName := strings.TrimSpace(name)
	nextLanguage := normalizeLanguage(language)

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	if !exists {
		return User{}, ErrUserNotFound
	}

	user.Name = nextName
	user.Language = nextLanguage

	passwordHash := []byte(nil)
	record, exists, err := s.findUserByEmailLocked(user.Email)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	if exists {
		passwordHash = record.PasswordHash
	}
	if err := s.persistUserLocked(user, passwordHash); err != nil {
		return User{}, ErrUserNotFound
	}

	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, nil
}

func (s *Service) GetAdminMFAStatus(userID string) (AdminMFAStatus, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAStatus{}, ErrUserNotFound
	}

	user, exists, err := s.findUserByID(nextUserID)
	if err != nil || !exists {
		return AdminMFAStatus{}, ErrUserNotFound
	}
	if !isAdminRole(user.Role) {
		return AdminMFAStatus{}, ErrUserRoleUnsupported
	}

	state, _, err := s.findAdminMFAState(nextUserID)
	if err != nil {
		return AdminMFAStatus{}, err
	}
	status := AdminMFAStatus{
		UserID:  nextUserID,
		Enabled: state.Enabled && strings.TrimSpace(state.Secret) != "",
		Pending: strings.TrimSpace(state.PendingSecret) != "",
	}
	if !state.UpdatedAt.IsZero() {
		updated := state.UpdatedAt.UTC()
		status.UpdatedAt = &updated
	}
	return status, nil
}

func (s *Service) StartAdminMFAEnrollment(userID, issuer string) (AdminMFAEnrollment, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAEnrollment{}, ErrUserNotFound
	}
	nextIssuer := strings.TrimSpace(issuer)
	if nextIssuer == "" {
		nextIssuer = "MistyPass"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return AdminMFAEnrollment{}, ErrUserNotFound
	}
	if !isAdminRole(user.Role) {
		return AdminMFAEnrollment{}, ErrUserRoleUnsupported
	}

	secret, err := generateTOTPSecret(20)
	if err != nil {
		return AdminMFAEnrollment{}, err
	}
	state := s.adminMFA[nextUserID]
	state.PendingSecret = secret
	state.UpdatedAt = time.Now().UTC()
	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return AdminMFAEnrollment{}, err
	}

	return AdminMFAEnrollment{
		UserID:     nextUserID,
		Secret:     secret,
		OTPAuthURL: buildOTPAuthURL(nextIssuer, user.Email, secret),
	}, nil
}

func (s *Service) EnableAdminMFA(userID, code string) (AdminMFAStatus, *AdminMFARecoveryCodes, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAStatus{}, nil, ErrUserNotFound
	}
	nextCode := strings.TrimSpace(code)
	if nextCode == "" {
		return AdminMFAStatus{}, nil, ErrAdminMFARequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return AdminMFAStatus{}, nil, ErrUserNotFound
	}
	if !isAdminRole(user.Role) {
		return AdminMFAStatus{}, nil, ErrUserRoleUnsupported
	}

	state, exists, err := s.findAdminMFAStateLocked(nextUserID)
	if err != nil {
		return AdminMFAStatus{}, nil, err
	}
	if !exists || strings.TrimSpace(state.PendingSecret) == "" {
		return AdminMFAStatus{}, nil, ErrAdminMFANotConfigured
	}
	if !verifyTOTPCode(state.PendingSecret, nextCode, time.Now().UTC()) {
		return AdminMFAStatus{}, nil, ErrInvalidMFACode
	}

	state.Secret = state.PendingSecret
	state.PendingSecret = ""
	state.Enabled = true
	state.UpdatedAt = time.Now().UTC()

	// Generate one-time recovery codes
	plaintextCodes, hashedCodes, err := generateRecoveryCodes(8)
	if err != nil {
		return AdminMFAStatus{}, nil, err
	}
	state.RecoveryCodes = hashedCodes

	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return AdminMFAStatus{}, nil, err
	}

	return AdminMFAStatus{
		UserID:    nextUserID,
		Enabled:   true,
		Pending:   false,
		UpdatedAt: timePointer(state.UpdatedAt),
	}, &AdminMFARecoveryCodes{
		UserID: nextUserID,
		Codes:  plaintextCodes,
	}, nil
}

// RegenerateAdminMFARecoveryCodes generates a new set of recovery codes, invalidating old ones.
func (s *Service) RegenerateAdminMFARecoveryCodes(userID string) (*AdminMFARecoveryCodes, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return nil, ErrUserNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists, err := s.findAdminMFAStateLocked(nextUserID)
	if err != nil {
		return nil, err
	}
	if !exists || !state.Enabled {
		return nil, ErrAdminMFANotConfigured
	}

	plaintextCodes, hashedCodes, err := generateRecoveryCodes(8)
	if err != nil {
		return nil, err
	}
	state.RecoveryCodes = hashedCodes
	state.UpdatedAt = time.Now().UTC()

	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return nil, err
	}

	return &AdminMFARecoveryCodes{
		UserID: nextUserID,
		Codes:  plaintextCodes,
	}, nil
}

func (s *Service) DisableAdminMFA(userID string) (AdminMFAStatus, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAStatus{}, ErrUserNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return AdminMFAStatus{}, ErrUserNotFound
	}
	if !isAdminRole(user.Role) {
		return AdminMFAStatus{}, ErrUserRoleUnsupported
	}

	state, _, err := s.findAdminMFAStateLocked(nextUserID)
	if err != nil {
		return AdminMFAStatus{}, err
	}
	state.Enabled = false
	state.Secret = ""
	state.PendingSecret = ""
	state.UpdatedAt = time.Now().UTC()
	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return AdminMFAStatus{}, err
	}

	return AdminMFAStatus{
		UserID:    nextUserID,
		Enabled:   false,
		Pending:   false,
		UpdatedAt: timePointer(state.UpdatedAt),
	}, nil
}

// --- User-level MFA (self-service for all authenticated users) ---

func (s *Service) GetUserMFAStatus(userID string) (AdminMFAStatus, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAStatus{}, ErrUserNotFound
	}

	_, exists, err := s.findUserByID(nextUserID)
	if err != nil || !exists {
		return AdminMFAStatus{}, ErrUserNotFound
	}

	state, _, err := s.findAdminMFAState(nextUserID)
	if err != nil {
		return AdminMFAStatus{}, err
	}
	status := AdminMFAStatus{
		UserID:  nextUserID,
		Enabled: state.Enabled && strings.TrimSpace(state.Secret) != "",
		Pending: strings.TrimSpace(state.PendingSecret) != "",
	}
	if !state.UpdatedAt.IsZero() {
		updated := state.UpdatedAt.UTC()
		status.UpdatedAt = &updated
	}
	return status, nil
}

func (s *Service) StartUserMFAEnrollment(userID, issuer string) (AdminMFAEnrollment, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAEnrollment{}, ErrUserNotFound
	}
	nextIssuer := strings.TrimSpace(issuer)
	if nextIssuer == "" {
		nextIssuer = "MistyPass"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return AdminMFAEnrollment{}, ErrUserNotFound
	}

	secret, err := generateTOTPSecret(20)
	if err != nil {
		return AdminMFAEnrollment{}, err
	}
	state := s.adminMFA[nextUserID]
	state.PendingSecret = secret
	state.UpdatedAt = time.Now().UTC()
	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return AdminMFAEnrollment{}, err
	}

	return AdminMFAEnrollment{
		UserID:     nextUserID,
		Secret:     secret,
		OTPAuthURL: buildOTPAuthURL(nextIssuer, user.Email, secret),
	}, nil
}

func (s *Service) EnableUserMFA(userID, code string) (AdminMFAStatus, *AdminMFARecoveryCodes, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAStatus{}, nil, ErrUserNotFound
	}
	nextCode := strings.TrimSpace(code)
	if nextCode == "" {
		return AdminMFAStatus{}, nil, ErrAdminMFARequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return AdminMFAStatus{}, nil, ErrUserNotFound
	}

	state, exists, err := s.findAdminMFAStateLocked(nextUserID)
	if err != nil {
		return AdminMFAStatus{}, nil, err
	}
	if !exists || strings.TrimSpace(state.PendingSecret) == "" {
		return AdminMFAStatus{}, nil, ErrAdminMFANotConfigured
	}
	if !verifyTOTPCode(state.PendingSecret, nextCode, time.Now().UTC()) {
		return AdminMFAStatus{}, nil, ErrInvalidMFACode
	}

	state.Secret = state.PendingSecret
	state.PendingSecret = ""
	state.Enabled = true
	state.UpdatedAt = time.Now().UTC()

	// Generate one-time recovery codes
	plaintextCodes, hashedCodes, err := generateRecoveryCodes(8)
	if err != nil {
		return AdminMFAStatus{}, nil, err
	}
	state.RecoveryCodes = hashedCodes

	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return AdminMFAStatus{}, nil, err
	}

	return AdminMFAStatus{
		UserID:    nextUserID,
		Enabled:   true,
		Pending:   false,
		UpdatedAt: timePointer(state.UpdatedAt),
	}, &AdminMFARecoveryCodes{
		UserID: nextUserID,
		Codes:  plaintextCodes,
	}, nil
}

func (s *Service) DisableUserMFA(userID string) (AdminMFAStatus, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return AdminMFAStatus{}, ErrUserNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists, err := s.findUserByIDLocked(nextUserID)
	if err != nil || !exists {
		return AdminMFAStatus{}, ErrUserNotFound
	}

	state, _, err := s.findAdminMFAStateLocked(nextUserID)
	if err != nil {
		return AdminMFAStatus{}, err
	}
	state.Enabled = false
	state.Secret = ""
	state.PendingSecret = ""
	state.UpdatedAt = time.Now().UTC()
	if err := s.persistAdminMFAStateLocked(nextUserID, state); err != nil {
		return AdminMFAStatus{}, err
	}

	return AdminMFAStatus{
		UserID:    nextUserID,
		Enabled:   false,
		Pending:   false,
		UpdatedAt: timePointer(state.UpdatedAt),
	}, nil
}

func (s *Service) issueTokenPairLocked(user User) (LoginResponse, error) {
	return s.issueTokenPairWithMetadataLocked(user, SessionMetadata{})
}

func (s *Service) issueTokenPairWithMetadataLocked(user User, meta SessionMetadata) (LoginResponse, error) {
	accessToken, _, err := s.signToken(user, "access", s.accessTTL)
	if err != nil {
		return LoginResponse{}, err
	}

	refreshToken, refreshClaims, err := s.signToken(user, "refresh", s.refreshTTL)
	if err != nil {
		return LoginResponse{}, err
	}

	now := time.Now().UTC()
	if err := s.upsertRefreshSessionLocked(refreshClaims.ID, refreshSession{
		UserID:      user.ID,
		ExpiresAt:   refreshClaims.ExpiresAt.Time,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
		LoginMethod: meta.LoginMethod,
		CreatedAt:   now,
	}); err != nil {
		return LoginResponse{}, err
	}

	return LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		User:         user,
	}, nil
}

func (s *Service) signToken(user User, tokenType string, ttl time.Duration) (string, tokenClaims, error) {
	jti, err := randomTokenID(12)
	if err != nil {
		return "", tokenClaims{}, err
	}

	now := time.Now().UTC()
	claims := tokenClaims{
		UserID:      user.ID,
		Name:        strings.TrimSpace(user.Name),
		Email:       user.Email,
		Role:        user.Role,
		TenantID:    user.TenantID,
		BuildingIDs: append([]string(nil), user.BuildingIDs...),
		Language:    normalizeLanguage(user.Language),
		TokenType:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   user.ID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", tokenClaims{}, err
	}

	return signedToken, claims, nil
}

func (s *Service) parseTokenClaims(rawToken, expectedType string) (tokenClaims, error) {
	if rawToken == "" {
		return tokenClaims{}, errors.New("empty token")
	}

	claims := tokenClaims{}
	parsed, err := jwt.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.signingKey, nil
	})
	if err != nil || !parsed.Valid {
		return tokenClaims{}, errors.New("invalid token")
	}

	if claims.TokenType != expectedType {
		return tokenClaims{}, errors.New("invalid token type")
	}
	if claims.Issuer != s.issuer {
		return tokenClaims{}, errors.New("invalid token issuer")
	}
	if claims.UserID == "" || claims.ID == "" {
		return tokenClaims{}, errors.New("invalid token claims")
	}

	return claims, nil
}

func (s *Service) findUserByEmail(email string) (userRecord, bool, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return userRecord{}, false, nil
	}

	s.mu.RLock()
	persistence := s.persistence
	s.mu.RUnlock()

	if persistence != nil {
		user, passwordHash, exists, err := persistence.FindAuthUserByEmail(nextEmail)
		if err != nil {
			return userRecord{}, false, err
		}
		if exists {
			normalizedUser, ok := normalizeUser(user)
			if ok {
				record := userRecord{
					User:         normalizedUser,
					PasswordHash: cloneBytes(passwordHash),
				}
				s.mu.Lock()
				s.cacheUserLocked(normalizedUser, passwordHash)
				s.mu.Unlock()
				return record, true, nil
			}
		}
	}

	s.mu.RLock()
	record, exists := s.usersByEmail[nextEmail]
	s.mu.RUnlock()
	if !exists {
		return userRecord{}, false, nil
	}

	record.User.BuildingIDs = append([]string(nil), record.User.BuildingIDs...)
	record.PasswordHash = cloneBytes(record.PasswordHash)
	return record, true, nil
}

func (s *Service) findUserByID(userID string) (User, bool, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, false, nil
	}

	s.mu.RLock()
	persistence := s.persistence
	s.mu.RUnlock()

	if persistence != nil {
		user, passwordHash, exists, err := persistence.FindAuthUserByID(nextUserID)
		if err != nil {
			return User{}, false, err
		}
		if exists {
			normalizedUser, ok := normalizeUser(user)
			if ok {
				s.mu.Lock()
				s.cacheUserLocked(normalizedUser, passwordHash)
				s.mu.Unlock()
				normalizedUser.BuildingIDs = append([]string(nil), normalizedUser.BuildingIDs...)
				return normalizedUser, true, nil
			}
		}
	}

	s.mu.RLock()
	user, exists := s.usersByID[nextUserID]
	s.mu.RUnlock()
	if !exists {
		return User{}, false, nil
	}

	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, true, nil
}

func (s *Service) findRefreshSession(sessionID string) (refreshSession, bool, error) {
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return refreshSession{}, false, nil
	}

	s.mu.RLock()
	volatileStore := s.volatileStore
	persistence := s.persistence
	s.mu.RUnlock()

	if volatileStore != nil {
		userID, expiresAt, exists, err := volatileStore.FindAuthRefreshSession(nextSessionID)
		if err != nil {
			return refreshSession{}, false, err
		}
		if exists {
			session := refreshSession{
				UserID:    strings.TrimSpace(userID),
				ExpiresAt: expiresAt.UTC(),
			}
			if session.UserID != "" && !session.ExpiresAt.IsZero() {
				s.mu.Lock()
				s.refreshSessions[nextSessionID] = session
				s.mu.Unlock()
				return session, true, nil
			}
		}
	} else if persistence != nil {
		userID, expiresAt, exists, err := persistence.FindAuthRefreshSession(nextSessionID)
		if err != nil {
			return refreshSession{}, false, err
		}
		if exists {
			session := refreshSession{
				UserID:    strings.TrimSpace(userID),
				ExpiresAt: expiresAt.UTC(),
			}
			if session.UserID != "" && !session.ExpiresAt.IsZero() {
				s.mu.Lock()
				s.refreshSessions[nextSessionID] = session
				s.mu.Unlock()
				return session, true, nil
			}
		}
	}

	s.mu.RLock()
	session, exists := s.refreshSessions[nextSessionID]
	s.mu.RUnlock()
	if !exists {
		return refreshSession{}, false, nil
	}

	session.UserID = strings.TrimSpace(session.UserID)
	session.ExpiresAt = session.ExpiresAt.UTC()
	return session, true, nil
}

func (s *Service) isAccessTokenRevoked(tokenID string, now time.Time) (bool, error) {
	nextTokenID := strings.TrimSpace(tokenID)
	if nextTokenID == "" {
		return false, nil
	}

	s.mu.RLock()
	volatileStore := s.volatileStore
	persistence := s.persistence
	s.mu.RUnlock()

	if volatileStore != nil {
		revoked, err := volatileStore.IsAuthAccessTokenRevoked(nextTokenID, now)
		if err != nil {
			return false, err
		}
		if revoked {
			return true, nil
		}
	} else if persistence != nil {
		revoked, err := persistence.IsAuthAccessTokenRevoked(nextTokenID, now)
		if err != nil {
			return false, err
		}
		if revoked {
			return true, nil
		}
	}

	s.mu.RLock()
	expiresAt, exists := s.revokedAccess[nextTokenID]
	s.mu.RUnlock()
	if !exists || !expiresAt.After(now) {
		return false, nil
	}
	return true, nil
}

func (s *Service) findAdminMFAState(userID string) (adminMFAState, bool, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return adminMFAState{}, false, nil
	}

	s.mu.RLock()
	state, exists := s.adminMFA[nextUserID]
	persistence := s.persistence
	s.mu.RUnlock()
	if exists {
		return state, true, nil
	}
	if persistence == nil {
		return adminMFAState{}, false, nil
	}

	persisted, found, err := persistence.FindAuthAdminMFAState(nextUserID)
	if err != nil {
		return adminMFAState{}, false, err
	}
	if !found {
		return adminMFAState{}, false, nil
	}

	state = adminMFAStateFromPersistence(persisted)
	s.mu.Lock()
	if cached, cachedExists := s.adminMFA[nextUserID]; cachedExists {
		state = cached
	} else {
		s.adminMFA[nextUserID] = state
	}
	s.mu.Unlock()
	return state, true, nil
}

func (s *Service) enforceAdminMFA(user User, mfaCode string) error {
	state, exists, err := s.findAdminMFAState(strings.TrimSpace(user.ID))
	if err != nil {
		return err
	}

	if isAdminRole(user.Role) {
		s.mu.RLock()
		required := s.adminMFARequired
		s.mu.RUnlock()

		if !exists || !state.Enabled || strings.TrimSpace(state.Secret) == "" {
			if required {
				return ErrAdminMFAEnrollmentRequired
			}
			return nil
		}
	} else {
		if !exists || !state.Enabled || strings.TrimSpace(state.Secret) == "" {
			return nil
		}
	}

	nextCode := strings.TrimSpace(mfaCode)
	if nextCode == "" {
		return ErrAdminMFARequired
	}
	// Try TOTP first (no state mutation needed)
	if verifyTOTPCode(state.Secret, nextCode, time.Now().UTC()) {
		return nil
	}
	// Try recovery code as fallback — acquire lock for state mutation
	s.mu.Lock()
	defer s.mu.Unlock()
	// Re-read state under lock to avoid TOCTOU race
	state, exists, err = s.findAdminMFAStateLocked(strings.TrimSpace(user.ID))
	if err != nil || !exists || !state.Enabled {
		return ErrInvalidMFACode
	}
	updatedCodes, matched := verifyAndConsumeRecoveryCode(state.RecoveryCodes, nextCode)
	if matched {
		state.RecoveryCodes = updatedCodes
		state.UpdatedAt = time.Now().UTC()
		if err := s.persistAdminMFAStateLocked(user.ID, state); err != nil {
			return err
		}
		return nil
	}
	return ErrInvalidMFACode
}

func (s *Service) findUserByEmailLocked(email string) (userRecord, bool, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return userRecord{}, false, nil
	}

	if s.persistence != nil {
		user, passwordHash, exists, err := s.persistence.FindAuthUserByEmail(nextEmail)
		if err != nil {
			return userRecord{}, false, err
		}
		if exists {
			normalizedUser, ok := normalizeUser(user)
			if ok {
				s.cacheUserLocked(normalizedUser, passwordHash)
				return userRecord{
					User:         normalizedUser,
					PasswordHash: cloneBytes(passwordHash),
				}, true, nil
			}
		}
	}

	record, exists := s.usersByEmail[nextEmail]
	if !exists {
		return userRecord{}, false, nil
	}
	record.User.BuildingIDs = append([]string(nil), record.User.BuildingIDs...)
	record.PasswordHash = cloneBytes(record.PasswordHash)
	return record, true, nil
}

func (s *Service) findUserByIDLocked(userID string) (User, bool, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, false, nil
	}

	if s.persistence != nil {
		user, passwordHash, exists, err := s.persistence.FindAuthUserByID(nextUserID)
		if err != nil {
			return User{}, false, err
		}
		if exists {
			normalizedUser, ok := normalizeUser(user)
			if ok {
				s.cacheUserLocked(normalizedUser, passwordHash)
				normalizedUser.BuildingIDs = append([]string(nil), normalizedUser.BuildingIDs...)
				return normalizedUser, true, nil
			}
		}
	}

	user, exists := s.usersByID[nextUserID]
	if !exists {
		return User{}, false, nil
	}
	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, true, nil
}

func (s *Service) persistUserLocked(user User, passwordHash []byte) error {
	normalizedUser, ok := normalizeUser(user)
	if !ok {
		return errors.New("user id/email/role are required")
	}
	nextPasswordHash := cloneBytes(passwordHash)
	if s.persistence != nil {
		if err := s.persistence.UpsertAuthUser(normalizedUser, nextPasswordHash); err != nil {
			return err
		}
	}
	s.cacheUserLocked(normalizedUser, nextPasswordHash)
	return nil
}

func (s *Service) cacheUserLocked(user User, passwordHash []byte) {
	nextEmail := normalizeEmail(user.Email)
	if existing, exists := s.usersByID[user.ID]; exists {
		oldEmail := normalizeEmail(existing.Email)
		if oldEmail != "" && oldEmail != nextEmail {
			delete(s.usersByEmail, oldEmail)
		}
	}

	cachedUser := user
	cachedUser.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	// Backward compat: users with password hash but no explicit PasswordAuthEnabled
	// default to enabled (pre-existing users before this field was added)
	if len(passwordHash) > 0 && !cachedUser.PasswordAuthEnabled {
		cachedUser.PasswordAuthEnabled = true
	}
	s.usersByID[user.ID] = cachedUser
	s.usersByEmail[nextEmail] = userRecord{
		User:         cachedUser,
		PasswordHash: cloneBytes(passwordHash),
	}
}

func (s *Service) upsertRefreshSessionLocked(sessionID string, session refreshSession) error {
	nextSessionID := strings.TrimSpace(sessionID)
	nextUserID := strings.TrimSpace(session.UserID)
	if nextSessionID == "" || nextUserID == "" || session.ExpiresAt.IsZero() {
		return errors.New("invalid refresh session")
	}
	nextSession := refreshSession{
		UserID:      nextUserID,
		ExpiresAt:   session.ExpiresAt.UTC(),
		IPAddress:   session.IPAddress,
		UserAgent:   session.UserAgent,
		LoginMethod: session.LoginMethod,
		CreatedAt:   session.CreatedAt,
	}

	if s.volatileStore != nil {
		if err := s.volatileStore.UpsertAuthRefreshSession(nextSessionID, nextSession.UserID, nextSession.ExpiresAt); err != nil {
			return err
		}
	} else if s.persistence != nil {
		if err := s.persistence.UpsertAuthRefreshSession(nextSessionID, nextSession.UserID, nextSession.ExpiresAt); err != nil {
			return err
		}
	}
	s.refreshSessions[nextSessionID] = nextSession
	return nil
}

func (s *Service) findRefreshSessionLocked(sessionID string) (refreshSession, bool, error) {
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return refreshSession{}, false, nil
	}

	if s.volatileStore != nil {
		userID, expiresAt, exists, err := s.volatileStore.FindAuthRefreshSession(nextSessionID)
		if err != nil {
			return refreshSession{}, false, err
		}
		if exists {
			session := refreshSession{
				UserID:    strings.TrimSpace(userID),
				ExpiresAt: expiresAt.UTC(),
			}
			if session.UserID != "" && !session.ExpiresAt.IsZero() {
				s.refreshSessions[nextSessionID] = session
				return session, true, nil
			}
		}
	} else if s.persistence != nil {
		userID, expiresAt, exists, err := s.persistence.FindAuthRefreshSession(nextSessionID)
		if err != nil {
			return refreshSession{}, false, err
		}
		if exists {
			session := refreshSession{
				UserID:    strings.TrimSpace(userID),
				ExpiresAt: expiresAt.UTC(),
			}
			if session.UserID != "" && !session.ExpiresAt.IsZero() {
				s.refreshSessions[nextSessionID] = session
				return session, true, nil
			}
		}
	}

	session, exists := s.refreshSessions[nextSessionID]
	if !exists {
		return refreshSession{}, false, nil
	}
	session.UserID = strings.TrimSpace(session.UserID)
	session.ExpiresAt = session.ExpiresAt.UTC()
	return session, true, nil
}

func (s *Service) deleteRefreshSessionLocked(sessionID string) error {
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return nil
	}

	if s.volatileStore != nil {
		if err := s.volatileStore.DeleteAuthRefreshSession(nextSessionID); err != nil {
			return err
		}
	} else if s.persistence != nil {
		if err := s.persistence.DeleteAuthRefreshSession(nextSessionID); err != nil {
			return err
		}
	}
	delete(s.refreshSessions, nextSessionID)
	return nil
}

func (s *Service) revokeAccessTokenLocked(tokenID string, expiresAt time.Time) error {
	nextTokenID := strings.TrimSpace(tokenID)
	if nextTokenID == "" || expiresAt.IsZero() {
		return errors.New("invalid access token")
	}
	nextExpiresAt := expiresAt.UTC()

	if s.volatileStore != nil {
		if err := s.volatileStore.UpsertAuthRevokedAccessToken(nextTokenID, nextExpiresAt); err != nil {
			return err
		}
	} else if s.persistence != nil {
		if err := s.persistence.UpsertAuthRevokedAccessToken(nextTokenID, nextExpiresAt); err != nil {
			return err
		}
	}
	s.revokedAccess[nextTokenID] = nextExpiresAt
	return nil
}

func (s *Service) isAccessTokenRevokedLocked(tokenID string, now time.Time) (bool, error) {
	nextTokenID := strings.TrimSpace(tokenID)
	if nextTokenID == "" {
		return false, nil
	}

	if s.volatileStore != nil {
		revoked, err := s.volatileStore.IsAuthAccessTokenRevoked(nextTokenID, now)
		if err != nil {
			return false, err
		}
		if revoked {
			return true, nil
		}
	} else if s.persistence != nil {
		revoked, err := s.persistence.IsAuthAccessTokenRevoked(nextTokenID, now)
		if err != nil {
			return false, err
		}
		if revoked {
			return true, nil
		}
	}

	expiresAt, exists := s.revokedAccess[nextTokenID]
	if !exists {
		return false, nil
	}
	if !expiresAt.After(now) {
		delete(s.revokedAccess, nextTokenID)
		return false, nil
	}
	return true, nil
}

func (s *Service) cleanupExpiredLocked(now time.Time) {
	for jti, expiresAt := range s.revokedAccess {
		if !expiresAt.After(now) {
			delete(s.revokedAccess, jti)
		}
	}
	for jti, session := range s.refreshSessions {
		if !session.ExpiresAt.After(now) {
			delete(s.refreshSessions, jti)
		}
	}
}

func (s *Service) findAdminMFAStateLocked(userID string) (adminMFAState, bool, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return adminMFAState{}, false, nil
	}

	state, exists := s.adminMFA[nextUserID]
	if exists {
		return state, true, nil
	}
	if s.persistence == nil {
		return adminMFAState{}, false, nil
	}

	persisted, found, err := s.persistence.FindAuthAdminMFAState(nextUserID)
	if err != nil {
		return adminMFAState{}, false, err
	}
	if !found {
		return adminMFAState{}, false, nil
	}

	state = adminMFAStateFromPersistence(persisted)
	s.adminMFA[nextUserID] = state
	return state, true, nil
}

func (s *Service) persistAdminMFAStateLocked(userID string, state adminMFAState) error {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return ErrUserNotFound
	}
	state.Secret = strings.TrimSpace(state.Secret)
	state.PendingSecret = strings.TrimSpace(state.PendingSecret)
	state.UpdatedAt = state.UpdatedAt.UTC()
	if s.persistence != nil {
		if err := s.persistence.UpsertAuthAdminMFAState(nextUserID, adminMFAStateForPersistence(state)); err != nil {
			return err
		}
	}
	s.adminMFA[nextUserID] = state
	return nil
}

func (s *Service) enforceAdminMFALocked(user User, mfaCode string) error {
	if !isAdminRole(user.Role) {
		return nil
	}
	state, exists, err := s.findAdminMFAStateLocked(strings.TrimSpace(user.ID))
	if err != nil {
		return err
	}
	if !exists || !state.Enabled || strings.TrimSpace(state.Secret) == "" {
		if s.adminMFARequired {
			return ErrAdminMFAEnrollmentRequired
		}
		return nil
	}
	nextCode := strings.TrimSpace(mfaCode)
	if nextCode == "" {
		return ErrAdminMFARequired
	}
	if verifyTOTPCode(state.Secret, nextCode, time.Now().UTC()) {
		return nil
	}
	updatedCodes, matched := verifyAndConsumeRecoveryCode(state.RecoveryCodes, nextCode)
	if matched {
		state.RecoveryCodes = updatedCodes
		state.UpdatedAt = time.Now().UTC()
		if err := s.persistAdminMFAStateLocked(user.ID, state); err != nil {
			return err
		}
		return nil
	}
	return ErrInvalidMFACode
}

func isAdminRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "super_admin", "tenant_admin":
		return true
	default:
		return false
	}
}

func adminMFAStateForPersistence(state adminMFAState) AdminMFAPersistenceState {
	codes := make([]MFARecoveryCodePersistence, 0, len(state.RecoveryCodes))
	for _, c := range state.RecoveryCodes {
		codes = append(codes, MFARecoveryCodePersistence{CodeHash: c.CodeHash, Used: c.Used})
	}
	return AdminMFAPersistenceState{
		Secret:        strings.TrimSpace(state.Secret),
		PendingSecret: strings.TrimSpace(state.PendingSecret),
		Enabled:       state.Enabled,
		RecoveryCodes: codes,
		UpdatedAt:     state.UpdatedAt.UTC(),
	}
}

func adminMFAStateFromPersistence(state AdminMFAPersistenceState) adminMFAState {
	codes := make([]mfaRecoveryCode, 0, len(state.RecoveryCodes))
	for _, c := range state.RecoveryCodes {
		codes = append(codes, mfaRecoveryCode{CodeHash: c.CodeHash, Used: c.Used})
	}
	return adminMFAState{
		Secret:        strings.TrimSpace(state.Secret),
		PendingSecret: strings.TrimSpace(state.PendingSecret),
		Enabled:       state.Enabled,
		RecoveryCodes: codes,
		UpdatedAt:     state.UpdatedAt.UTC(),
	}
}

func generateTOTPSecret(byteLen int) (string, error) {
	size := byteLen
	if size <= 0 {
		size = 20
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	return encoder.EncodeToString(raw), nil
}

func buildOTPAuthURL(issuer, email, secret string) string {
	nextIssuer := strings.TrimSpace(issuer)
	if nextIssuer == "" {
		nextIssuer = "MistyPass"
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		nextEmail = "admin"
	}
	label := url.PathEscape(nextIssuer + ":" + nextEmail)
	return "otpauth://totp/" + label + "?secret=" + url.QueryEscape(secret) +
		"&issuer=" + url.QueryEscape(nextIssuer) +
		"&algorithm=SHA1&digits=6&period=30"
}

func verifyTOTPCode(secret, code string, now time.Time) bool {
	nextCode := strings.TrimSpace(code)
	if len(nextCode) != 6 {
		return false
	}
	for _, ch := range nextCode {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	counter := now.UTC().Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		generated, err := totpCodeAt(secret, counter+offset)
		if err != nil {
			return false
		}
		if generated == nextCode {
			return true
		}
	}
	return false
}

func totpCodeAt(secret string, counter int64) (string, error) {
	nextSecret := strings.ToUpper(strings.TrimSpace(secret))
	if nextSecret == "" {
		return "", errors.New("empty totp secret")
	}
	decoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	secretBytes, err := decoder.DecodeString(nextSecret)
	if err != nil {
		return "", err
	}

	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, uint64(counter))

	hash := hmac.New(sha1.New, secretBytes)
	if _, err := hash.Write(counterBytes); err != nil {
		return "", err
	}
	sum := hash.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := int32(sum[offset]&0x7f)<<24 |
		int32(sum[offset+1])<<16 |
		int32(sum[offset+2])<<8 |
		int32(sum[offset+3])
	otp := int(binaryCode % 1000000)
	return fmt.Sprintf("%06d", otp), nil
}

func timePointer(value time.Time) *time.Time {
	next := value.UTC()
	return &next
}

func normalizeUser(user User) (User, bool) {
	nextUser := User{
		ID:                  strings.TrimSpace(user.ID),
		Name:                strings.TrimSpace(user.Name),
		Email:               normalizeEmail(user.Email),
		Role:                strings.ToLower(strings.TrimSpace(user.Role)),
		TenantID:            strings.TrimSpace(user.TenantID),
		BuildingIDs:         uniqueNormalizedIDs(user.BuildingIDs),
		Language:            normalizeLanguage(user.Language),
		PasswordAuthEnabled: user.PasswordAuthEnabled,
	}
	if nextUser.ID == "" || nextUser.Email == "" || nextUser.Role == "" {
		return User{}, false
	}
	return nextUser, true
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func randomTokenID(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeLanguage(language string) string {
	nextLanguage := strings.TrimSpace(language)
	switch nextLanguage {
	case "en-US", "id-ID", "zh-CN":
		return nextLanguage
	default:
		return ""
	}
}

func uniqueNormalizedIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for i := range values {
		value := strings.TrimSpace(values[i])
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}

	return items
}

func buildDemoUsers() []userRecord {
	seed := []struct {
		User     User
		Password string
	}{
		{
			User: User{
				ID:       "usr_super_admin_001",
				Name:     "Super Admin",
				Email:    "superadmin@mistypass.local",
				Role:     "super_admin",
				TenantID: "",
				Language: "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_tenant_admin_jkt_001",
				Name:     "Jakarta Tenant Admin",
				Email:    "tenant.admin@sudirman.co",
				Role:     "tenant_admin",
				TenantID: "tenant_demo_jakarta",
				Language: "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_organization_admin_jkt_001",
				Name:     "Organization Admin",
				Email:    "organization.admin@mistypass.local",
				Role:     "tenant_admin",
				TenantID: "tenant_demo_jakarta",
				Language: "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_operator_jkt_001",
				Name:     "Jakarta Operator",
				Email:    "ops.jkt.01@mistypass.local",
				Role:     "operator",
				TenantID: "tenant_demo_jakarta",
				Language: "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:          "usr_building_admin_jkt_001",
				Name:        "Sudirman Building Admin",
				Email:       "building.admin.sudirman@mistypass.local",
				Role:        "building_admin",
				TenantID:    "tenant_demo_jakarta",
				BuildingIDs: []string{"building_demo_001"},
				Language:    "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:          "usr_place_admin_sudirman_001",
				Name:        "Sudirman Place Admin",
				Email:       "place.admin.sudirman@mistypass.local",
				Role:        "building_admin",
				TenantID:    "tenant_demo_jakarta",
				BuildingIDs: []string{"building_demo_001"},
				Language:    "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_tenant_admin_fct_001",
				Name:     "Factory Tenant Admin",
				Email:    "tenant.admin@factory.local",
				Role:     "tenant_admin",
				TenantID: "tenant_demo_factory",
				Language: "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_resident_jkt_001",
				Name:     "Jakarta Resident",
				Email:    "resident.jakarta@mistypass.local",
				Role:     "resident",
				TenantID: "tenant_demo_jakarta",
				Language: "en-US",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_resident_siky_001",
				Name:     "Siky",
				Email:    "siky",
				Role:     "resident",
				TenantID: "tenant_demo_jakarta",
				Language: "zh-CN",
			},
			Password: "65552588",
		},
	}
	result := make([]userRecord, 0, len(seed))
	for i := range seed {
		seed[i].User.PasswordAuthEnabled = true // demo users always have password auth
		hash, err := bcrypt.GenerateFromPassword([]byte(seed[i].Password), bcrypt.DefaultCost)
		if err != nil {
			panic(fmt.Sprintf("hash demo user password failed for %s: %v", seed[i].User.Email, err))
		}
		result = append(result, userRecord{
			User:         seed[i].User,
			PasswordHash: hash,
		})
	}
	return result
}

func verifyPassword(passwordHash []byte, plain string) bool {
	if len(passwordHash) == 0 || plain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword(passwordHash, []byte(plain)) == nil
}

// generateRecoveryCodes creates n one-time recovery codes and their bcrypt hashes.
func generateRecoveryCodes(n int) (plaintext []string, hashed []mfaRecoveryCode, err error) {
	plaintext = make([]string, 0, n)
	hashed = make([]mfaRecoveryCode, 0, n)
	for i := 0; i < n; i++ {
		raw := make([]byte, 4)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		code := fmt.Sprintf("%08X", raw)
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		plaintext = append(plaintext, code)
		hashed = append(hashed, mfaRecoveryCode{CodeHash: string(hash), Used: false})
	}
	return plaintext, hashed, nil
}

// verifyAndConsumeRecoveryCode checks if the code matches any unused recovery code.
// If matched, marks it as used and returns true.
func verifyAndConsumeRecoveryCode(codes []mfaRecoveryCode, code string) ([]mfaRecoveryCode, bool) {
	for i := range codes {
		if codes[i].Used {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(codes[i].CodeHash), []byte(code)) == nil {
			codes[i].Used = true
			return codes, true
		}
	}
	return codes, false
}

// ValidatePasswordPolicy checks that a password meets minimum security requirements:
// at least 8 characters, at least 1 uppercase letter, 1 lowercase letter, and 1 digit.
func ValidatePasswordPolicy(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooWeak
	}
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return ErrPasswordTooWeak
	}
	return nil
}

func ephemeralJWTSecret() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Sprintf("generate ephemeral jwt secret failed: %v", err))
	}
	return fmt.Sprintf("%x", raw)
}
