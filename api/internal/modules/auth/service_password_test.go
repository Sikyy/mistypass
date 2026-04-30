package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"", true},
		{"short", true},
		{"alllowercase1", true},  // no uppercase
		{"ALLUPPERCASE1", true},  // no lowercase
		{"NoDigitsHere", true},   // no digit
		{"Abcdefg1", false},      // exactly 8, meets all
		{"MyP@ssw0rd", false},    // strong
		{"12345678", true},       // all digits
	}
	for _, tt := range tests {
		err := ValidatePasswordPolicy(tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePasswordPolicy(%q) = %v, wantErr=%v", tt.password, err, tt.wantErr)
		}
	}
}

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
