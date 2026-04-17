package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
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

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	TenantID    string   `json:"tenant_id"`
	BuildingIDs []string `json:"building_ids,omitempty"`
}

type tokenClaims struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	TenantID    string   `json:"tenant_id,omitempty"`
	BuildingIDs []string `json:"building_ids,omitempty"`
	TokenType   string   `json:"token_type"`
	jwt.RegisteredClaims
}

type userRecord struct {
	User         User
	PasswordHash []byte
}

type refreshSession struct {
	UserID    string
	ExpiresAt time.Time
}

type Service struct {
	mu              sync.RWMutex
	signingKey      []byte
	issuer          string
	accessTTL       time.Duration
	refreshTTL      time.Duration
	usersByEmail    map[string]userRecord
	usersByID       map[string]User
	refreshSessions map[string]refreshSession
	revokedAccess   map[string]time.Time
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
		signingKey:      []byte(nextSecret),
		issuer:          nextIssuer,
		accessTTL:       nextAccessTTL,
		refreshTTL:      nextRefreshTTL,
		usersByEmail:    usersByEmail,
		usersByID:       usersByID,
		refreshSessions: make(map[string]refreshSession),
		revokedAccess:   make(map[string]time.Time),
	}
}

func (s *Service) Login(request LoginRequest) (LoginResponse, error) {
	email := normalizeEmail(request.Email)
	password := strings.TrimSpace(request.Password)
	if email == "" || password == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())

	record, exists := s.usersByEmail[email]
	if !exists || !verifyPassword(record.PasswordHash, password) {
		return LoginResponse{}, ErrInvalidCredentials
	}

	return s.issueTokenPairLocked(record.User)
}

func (s *Service) LoginByTrustedIdentity(email string) (LoginResponse, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())

	record, exists := s.usersByEmail[nextEmail]
	if !exists {
		return LoginResponse{}, ErrInvalidCredentials
	}

	return s.issueTokenPairLocked(record.User)
}

func (s *Service) LoginByTrustedUser(user User) (LoginResponse, error) {
	nextUserID := strings.TrimSpace(user.ID)
	nextEmail := normalizeEmail(user.Email)
	nextRole := strings.ToLower(strings.TrimSpace(user.Role))
	nextTenantID := strings.TrimSpace(user.TenantID)
	nextBuildingIDs := uniqueNormalizedIDs(user.BuildingIDs)

	if nextUserID == "" || nextEmail == "" || nextRole == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	trustedUser := User{
		ID:          nextUserID,
		Email:       nextEmail,
		Role:        nextRole,
		TenantID:    nextTenantID,
		BuildingIDs: nextBuildingIDs,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())

	s.usersByID[trustedUser.ID] = trustedUser
	record, exists := s.usersByEmail[nextEmail]
	if exists {
		record.User = trustedUser
		s.usersByEmail[nextEmail] = record
	} else {
		s.usersByEmail[nextEmail] = userRecord{
			User:         trustedUser,
			PasswordHash: nil,
		}
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	session, exists := s.refreshSessions[claims.ID]
	if !exists || session.UserID != claims.UserID || session.ExpiresAt.Before(now) {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	delete(s.refreshSessions, claims.ID)

	user, exists := s.usersByID[claims.UserID]
	if !exists {
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	session, exists := s.refreshSessions[claims.ID]
	if !exists || session.UserID != claims.UserID || !session.ExpiresAt.After(now) {
		return ErrInvalidRefreshToken
	}
	delete(s.refreshSessions, claims.ID)
	return nil
}

func (s *Service) RevokeRefreshTokensByUserEmail(email string) int {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return 0
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	record, exists := s.usersByEmail[nextEmail]
	if !exists {
		return 0
	}

	revoked := 0
	for sessionID, session := range s.refreshSessions {
		if session.UserID != record.User.ID {
			continue
		}
		delete(s.refreshSessions, sessionID)
		revoked++
	}
	return revoked
}

func (s *Service) DowngradeTrustedUserToLeastPrivilegeByEmail(email string, fallbackTenantID string) (User, User, bool) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return User{}, User{}, false
	}
	nextFallbackTenantID := strings.TrimSpace(fallbackTenantID)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(time.Now().UTC())

	record, exists := s.usersByEmail[nextEmail]
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

	record.User = after
	s.usersByEmail[nextEmail] = record
	s.usersByID[after.ID] = after

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
	s.revokedAccess[claims.ID] = claims.ExpiresAt.Time

	return nil
}

func (s *Service) VerifyAccessToken(accessToken string) (User, error) {
	claims, err := s.parseTokenClaims(strings.TrimSpace(accessToken), "access")
	if err != nil {
		return User{}, ErrInvalidAccessToken
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	if expiresAt, revoked := s.revokedAccess[claims.ID]; revoked && expiresAt.After(now) {
		return User{}, ErrInvalidAccessToken
	}

	user, exists := s.usersByID[claims.UserID]
	if exists {
		return user, nil
	}

	return User{
		ID:          claims.UserID,
		Email:       claims.Email,
		Role:        claims.Role,
		TenantID:    claims.TenantID,
		BuildingIDs: append([]string(nil), claims.BuildingIDs...),
	}, nil
}

func (s *Service) GetUserByID(userID string) (User, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, ErrUserNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.usersByID[nextUserID]
	if !exists {
		return User{}, ErrUserNotFound
	}

	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, nil
}

func (s *Service) UpdateUserBuildingScope(userID string, buildingIDs []string) (User, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return User{}, ErrUserNotFound
	}

	nextBuildingIDs := uniqueNormalizedIDs(buildingIDs)

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.usersByID[nextUserID]
	if !exists {
		return User{}, ErrUserNotFound
	}

	if strings.ToLower(strings.TrimSpace(user.Role)) != "building_admin" {
		return User{}, ErrUserRoleUnsupported
	}

	user.BuildingIDs = nextBuildingIDs
	s.usersByID[nextUserID] = user

	emailKey := normalizeEmail(user.Email)
	record, exists := s.usersByEmail[emailKey]
	if exists {
		record.User = user
		s.usersByEmail[emailKey] = record
	}

	user.BuildingIDs = append([]string(nil), user.BuildingIDs...)
	return user, nil
}

func (s *Service) issueTokenPairLocked(user User) (LoginResponse, error) {
	accessToken, _, err := s.signToken(user, "access", s.accessTTL)
	if err != nil {
		return LoginResponse{}, err
	}

	refreshToken, refreshClaims, err := s.signToken(user, "refresh", s.refreshTTL)
	if err != nil {
		return LoginResponse{}, err
	}

	s.refreshSessions[refreshClaims.ID] = refreshSession{
		UserID:    user.ID,
		ExpiresAt: refreshClaims.ExpiresAt.Time,
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
		Email:       user.Email,
		Role:        user.Role,
		TenantID:    user.TenantID,
		BuildingIDs: append([]string(nil), user.BuildingIDs...),
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
				Email:    "superadmin@mistypass.local",
				Role:     "super_admin",
				TenantID: "",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_tenant_admin_jkt_001",
				Email:    "tenant.admin@sudirman.co",
				Role:     "tenant_admin",
				TenantID: "tenant_demo_jakarta",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_operator_jkt_001",
				Email:    "ops.jkt.01@mistypass.local",
				Role:     "operator",
				TenantID: "tenant_demo_jakarta",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:          "usr_building_admin_jkt_001",
				Email:       "building.admin.sudirman@mistypass.local",
				Role:        "building_admin",
				TenantID:    "tenant_demo_jakarta",
				BuildingIDs: []string{"building_demo_001"},
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_tenant_admin_fct_001",
				Email:    "tenant.admin@factory.local",
				Role:     "tenant_admin",
				TenantID: "tenant_demo_factory",
			},
			Password: "admin123",
		},
		{
			User: User{
				ID:       "usr_resident_jkt_001",
				Email:    "resident.jakarta@mistypass.local",
				Role:     "resident",
				TenantID: "tenant_demo_jakarta",
			},
			Password: "admin123",
		},
	}
	result := make([]userRecord, 0, len(seed))
	for i := range seed {
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
	nextPlain := strings.TrimSpace(plain)
	if len(passwordHash) == 0 || nextPlain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword(passwordHash, []byte(nextPlain)) == nil
}

func ephemeralJWTSecret() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err == nil {
		return fmt.Sprintf("%x", raw)
	}
	// fallback to timestamp-based secret when system randomness is unavailable.
	return fmt.Sprintf("mistypass-ephemeral-%d", time.Now().UnixNano())
}
