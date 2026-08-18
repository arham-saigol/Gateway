package auth_test

import (
	"strings"
	"testing"

	"arham-gateway/internal/auth"
)

func TestArgon2idPasswordHashingAndVerification(t *testing.T) {
	password := "SecureAdminPass123!#"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected $argon2id$ prefix, got %s", hash)
	}

	// Verify correct password
	match, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("verification error: %v", err)
	}
	if !match {
		t.Fatalf("expected valid password to match")
	}

	// Verify incorrect password
	match, err = auth.VerifyPassword("WrongPassword!", hash)
	if err != nil {
		t.Fatalf("verification error: %v", err)
	}
	if match {
		t.Fatalf("expected wrong password to fail")
	}
}

func TestGatewayAPIKeyGenerationAndHashing(t *testing.T) {
	rawKey, prefix, hash, err := auth.GenerateGatewayKey("test-client")
	if err != nil {
		t.Fatalf("failed to generate gateway key: %v", err)
	}

	if !strings.HasPrefix(rawKey, "arham_") {
		t.Fatalf("expected key prefix arham_, got %s", rawKey)
	}

	if !strings.HasPrefix(prefix, "arham_") {
		t.Fatalf("expected visible prefix arham_, got %s", prefix)
	}

	if len(hash) != 64 { // hex-encoded sha256
		t.Fatalf("expected 64 hex characters sha256 hash, got %d", len(hash))
	}

	// Verify hash computation from raw key
	computedHash := auth.HashGatewayKey(rawKey)
	if computedHash != hash {
		t.Fatalf("computed hash %s did not match generated hash %s", computedHash, hash)
	}
}

func TestSessionTokenGeneration(t *testing.T) {
	rawToken, hashedToken, err := auth.GenerateSessionToken()
	if err != nil {
		t.Fatalf("failed to generate session token: %v", err)
	}

	if len(rawToken) < 32 {
		t.Fatalf("raw token too short: %d", len(rawToken))
	}

	computedHash := auth.HashSessionToken(rawToken)
	if computedHash != hashedToken {
		t.Fatalf("session token hash mismatch")
	}
}

func TestCSRFTokenVerification(t *testing.T) {
	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("failed to generate CSRF token: %v", err)
	}

	if !auth.ValidateCSRFToken(csrfToken, csrfToken) {
		t.Fatalf("expected matching CSRF tokens to validate")
	}

	if auth.ValidateCSRFToken(csrfToken, "invalid-token") {
		t.Fatalf("expected mismatched CSRF tokens to fail validation")
	}
}
