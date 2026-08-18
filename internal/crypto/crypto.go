package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrInvalidMasterKey = errors.New("invalid master key: must be 32 bytes")
	ErrDecryptionFailed = errors.New("decryption failed: ciphertext is invalid or tampered")
)

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("reading random bytes: %w", err)
	}
	return b, nil
}

func GenerateAndSaveMasterKey(path string) ([]byte, error) {
	key, err := GenerateRandomBytes(32)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating master key directory %s: %w", dir, err)
	}

	// Write key with 0600 permissions
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, fmt.Errorf("writing master key to %s: %w", path, err)
	}

	return key, nil
}

func LoadMasterKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading master key %s: %w", path, err)
	}

	if len(data) != 32 {
		return nil, ErrInvalidMasterKey
	}

	return data, nil
}

func Encrypt(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidMasterKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func Decrypt(key []byte, ciphertextBase64 string) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidMasterKey
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM cipher: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", ErrDecryptionFailed
	}

	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}
