package auth

import (
	"testing"
	"time"
)

func TestVerifyUserPassword(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	if err := svc.VerifyUserPassword("usr_super_admin_001", "admin123"); err != nil {
		t.Fatalf("expected correct password to verify, got %v", err)
	}
	if err := svc.VerifyUserPassword("usr_super_admin_001", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for wrong password, got %v", err)
	}
	if err := svc.VerifyUserPassword("usr_does_not_exist", "admin123"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound for unknown user, got %v", err)
	}
}

func TestVerifyUserMFACodeRejectsJunkAndAcceptsValid(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	// Before MFA is enabled there is no valid code to present.
	if err := svc.VerifyUserMFACode("usr_super_admin_001", "000000"); err != ErrInvalidMFACode {
		t.Fatalf("expected ErrInvalidMFACode before MFA enabled, got %v", err)
	}

	enrollment, err := svc.StartUserMFAEnrollment("usr_super_admin_001", "MistyPass")
	if err != nil {
		t.Fatalf("start user mfa enrollment failed: %v", err)
	}
	enableCode, err := totpCodeAt(enrollment.Secret, time.Now().UTC().Unix()/30)
	if err != nil {
		t.Fatalf("build totp enable code failed: %v", err)
	}
	if _, _, err := svc.EnableUserMFA("usr_super_admin_001", enableCode); err != nil {
		t.Fatalf("enable user mfa failed: %v", err)
	}

	// A junk code must be rejected once MFA is enabled (the original bug).
	if err := svc.VerifyUserMFACode("usr_super_admin_001", "000000"); err != ErrInvalidMFACode {
		t.Fatalf("expected ErrInvalidMFACode for junk code, got %v", err)
	}

	// The current TOTP must verify.
	validCode, err := totpCodeAt(enrollment.Secret, time.Now().UTC().Unix()/30)
	if err != nil {
		t.Fatalf("build totp verify code failed: %v", err)
	}
	if err := svc.VerifyUserMFACode("usr_super_admin_001", validCode); err != nil {
		t.Fatalf("expected valid TOTP to verify, got %v", err)
	}
}
