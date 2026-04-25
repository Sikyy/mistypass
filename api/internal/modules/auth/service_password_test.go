package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerifyPasswordDoesNotTrimWhitespace(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}

	if !verifyPassword(hash, "admin123") {
		t.Fatalf("expected exact password to verify")
	}
	if verifyPassword(hash, " admin123 ") {
		t.Fatalf("expected whitespace-variant password to fail verification")
	}
}
