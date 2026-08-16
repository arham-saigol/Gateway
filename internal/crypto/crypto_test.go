package crypto_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"arham-gateway/internal/crypto"
)

func TestMasterKeyGenerationAndLoading(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")

	// Key shouldn't exist initially
	_, err := crypto.LoadMasterKey(keyPath)
	if err == nil {
		t.Fatalf("expected error loading non-existent master key")
	}

	// Generate master key
	key, err := crypto.GenerateAndSaveMasterKey(keyPath)
	if err != nil {
		t.Fatalf("failed to generate master key: %v", err)
	}

	if len(key) != 32 {
		t.Fatalf("expected 32 bytes master key, got %d", len(key))
	}

	// Load existing master key
	loadedKey, err := crypto.LoadMasterKey(keyPath)
	if err != nil {
		t.Fatalf("failed to load master key: %v", err)
	}

	if !bytes.Equal(key, loadedKey) {
		t.Fatalf("loaded key did not match generated key")
	}
}

func TestEncryptDecryptProviderSecret(t *testing.T) {
	key, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}

	secret := "fw_3849102834019283401928340"

	ciphertext, err := crypto.Encrypt(key, secret)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if ciphertext == secret {
		t.Fatalf("ciphertext should not match plaintext")
	}

	decrypted, err := crypto.Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if decrypted != secret {
		t.Fatalf("expected %s, got %s", secret, decrypted)
	}
}

func TestTamperedCiphertextFailsDecryption(t *testing.T) {
	key, _ := crypto.GenerateRandomBytes(32)
	secret := "super-secret-provider-key"

	ciphertext, err := crypto.Encrypt(key, secret)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Tamper ciphertext
	tampered := []byte(ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = crypto.Decrypt(key, string(tampered))
	if err == nil {
		t.Fatalf("expected decryption error on tampered ciphertext, got nil")
	}
}
