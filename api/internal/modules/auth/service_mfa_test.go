package auth

import (
	"strings"
	"testing"
	"time"
)

type stubAuthPersistence struct {
	usersByEmail map[string]userRecord
	usersByID    map[string]userRecord
	adminMFA     map[string]AdminMFAPersistenceState
}

func newStubAuthPersistence() *stubAuthPersistence {
	return &stubAuthPersistence{
		usersByEmail: map[string]userRecord{},
		usersByID:    map[string]userRecord{},
		adminMFA:     map[string]AdminMFAPersistenceState{},
	}
}

func (s *stubAuthPersistence) UpsertAuthUser(user User, passwordHash []byte) error {
	record := userRecord{
		User:         user,
		PasswordHash: cloneBytes(passwordHash),
	}
	s.usersByID[user.ID] = record
	s.usersByEmail[normalizeEmail(user.Email)] = record
	return nil
}

func (s *stubAuthPersistence) FindAuthUserByEmail(email string) (User, []byte, bool, error) {
	record, exists := s.usersByEmail[normalizeEmail(email)]
	if !exists {
		return User{}, nil, false, nil
	}
	return record.User, cloneBytes(record.PasswordHash), true, nil
}

func (s *stubAuthPersistence) FindAuthUserByID(userID string) (User, []byte, bool, error) {
	record, exists := s.usersByID[strings.TrimSpace(userID)]
	if !exists {
		return User{}, nil, false, nil
	}
	return record.User, cloneBytes(record.PasswordHash), true, nil
}

func (s *stubAuthPersistence) UpsertAuthRefreshSession(string, string, time.Time) error {
	return nil
}

func (s *stubAuthPersistence) FindAuthRefreshSession(string) (string, time.Time, bool, error) {
	return "", time.Time{}, false, nil
}

func (s *stubAuthPersistence) DeleteAuthRefreshSession(string) error {
	return nil
}

func (s *stubAuthPersistence) DeleteAuthRefreshSessionsByUserID(string) (int, error) {
	return 0, nil
}

func (s *stubAuthPersistence) UpsertAuthRevokedAccessToken(string, time.Time) error {
	return nil
}

func (s *stubAuthPersistence) IsAuthAccessTokenRevoked(string, time.Time) (bool, error) {
	return false, nil
}

func (s *stubAuthPersistence) UpsertAuthAdminMFAState(userID string, state AdminMFAPersistenceState) error {
	s.adminMFA[strings.TrimSpace(userID)] = state
	return nil
}

func (s *stubAuthPersistence) FindAuthAdminMFAState(userID string) (AdminMFAPersistenceState, bool, error) {
	state, exists := s.adminMFA[strings.TrimSpace(userID)]
	if !exists {
		return AdminMFAPersistenceState{}, false, nil
	}
	return state, true, nil
}

func TestAdminMFALoginFlow(t *testing.T) {
	svc := NewService("", "", 0, 0, true)
	svc.SetAdminMFARequired(true)

	_, err := svc.Login(LoginRequest{
		Email:    "superadmin@mistypass.local",
		Password: "admin123",
	})
	if err != ErrAdminMFAEnrollmentRequired {
		t.Fatalf("expected mfa enrollment required before login, got %v", err)
	}

	enrollment, err := svc.StartAdminMFAEnrollment("usr_super_admin_001", "MistyPass")
	if err != nil {
		t.Fatalf("start admin mfa enrollment failed: %v", err)
	}
	if enrollment.Secret == "" || enrollment.OTPAuthURL == "" {
		t.Fatalf("unexpected empty enrollment payload: %+v", enrollment)
	}

	enableCode, err := totpCodeAt(enrollment.Secret, time.Now().UTC().Unix()/30)
	if err != nil {
		t.Fatalf("build totp enable code failed: %v", err)
	}
	status, recoveryCodes, err := svc.EnableAdminMFA("usr_super_admin_001", enableCode)
	if err != nil {
		t.Fatalf("enable admin mfa failed: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected enabled mfa status")
	}
	if recoveryCodes == nil || len(recoveryCodes.Codes) != 8 {
		t.Fatalf("expected 8 recovery codes, got %v", recoveryCodes)
	}

	_, err = svc.Login(LoginRequest{
		Email:    "superadmin@mistypass.local",
		Password: "admin123",
	})
	if err != ErrAdminMFARequired {
		t.Fatalf("expected mfa code required error, got %v", err)
	}

	_, err = svc.Login(LoginRequest{
		Email:    "superadmin@mistypass.local",
		Password: "admin123",
		MFACode:  "000000",
	})
	if err != ErrInvalidMFACode {
		t.Fatalf("expected invalid mfa code error, got %v", err)
	}

	loginCode, err := totpCodeAt(enrollment.Secret, time.Now().UTC().Unix()/30)
	if err != nil {
		t.Fatalf("build totp login code failed: %v", err)
	}
	response, err := svc.Login(LoginRequest{
		Email:    "superadmin@mistypass.local",
		Password: "admin123",
		MFACode:  loginCode,
	})
	if err != nil {
		t.Fatalf("expected admin login with mfa success: %v", err)
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		t.Fatalf("expected non-empty token pair")
	}
}

func TestAdminMFARejectsNonAdminUser(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	_, err := svc.StartAdminMFAEnrollment("usr_resident_jkt_001", "MistyPass")
	if err != ErrUserRoleUnsupported {
		t.Fatalf("expected non-admin mfa setup to be forbidden, got %v", err)
	}
}

func TestAdminMFAStatePersistsAcrossServiceRecreation(t *testing.T) {
	store := newStubAuthPersistence()

	svc := NewService("", "", 0, 0, true)
	svc.SetAdminMFARequired(true)
	if err := svc.SetPersistence(store); err != nil {
		t.Fatalf("set persistence failed: %v", err)
	}

	enrollment, err := svc.StartAdminMFAEnrollment("usr_super_admin_001", "MistyPass")
	if err != nil {
		t.Fatalf("start admin mfa enrollment failed: %v", err)
	}
	enableCode, err := totpCodeAt(enrollment.Secret, time.Now().UTC().Unix()/30)
	if err != nil {
		t.Fatalf("build totp enable code failed: %v", err)
	}
	if _, _, err := svc.EnableAdminMFA("usr_super_admin_001", enableCode); err != nil {
		t.Fatalf("enable admin mfa failed: %v", err)
	}

	restarted := NewService("", "", 0, 0, true)
	restarted.SetAdminMFARequired(true)
	if err := restarted.SetPersistence(store); err != nil {
		t.Fatalf("set persistence on restarted service failed: %v", err)
	}

	status, err := restarted.GetAdminMFAStatus("usr_super_admin_001")
	if err != nil {
		t.Fatalf("get admin mfa status failed after restart: %v", err)
	}
	if !status.Enabled || status.Pending {
		t.Fatalf("expected enabled non-pending mfa status after restart, got %+v", status)
	}

	loginCode, err := totpCodeAt(enrollment.Secret, time.Now().UTC().Unix()/30)
	if err != nil {
		t.Fatalf("build totp login code failed: %v", err)
	}
	response, err := restarted.Login(LoginRequest{
		Email:    "superadmin@mistypass.local",
		Password: "admin123",
		MFACode:  loginCode,
	})
	if err != nil {
		t.Fatalf("expected persisted mfa login success after restart: %v", err)
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		t.Fatalf("expected non-empty token pair after restart")
	}
}
