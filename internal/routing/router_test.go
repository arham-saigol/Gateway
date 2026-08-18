package routing_test

import (
	"path/filepath"
	"testing"
	"time"

	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
	"arham-gateway/internal/routing"
)

func TestRouterRoundRobinAndCooldown(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gateway.db")

	db, err := database.Open(dbPath, 5000)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	_ = db.Migrate()
	_ = db.SeedDefaults()

	masterKey, _ := crypto.GenerateRandomBytes(32)

	// Create 2 keys for fireworks
	now := time.Now().UTC()
	encSecret1, _ := crypto.Encrypt(masterKey, "fw-secret-1")
	encSecret2, _ := crypto.Encrypt(masterKey, "fw-secret-2")

	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "fw-key-1",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret1,
		DisplayName:             "Fireworks Key 1",
		KeyPrefix:               "fw_1...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "fw-key-2",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret2,
		DisplayName:             "Fireworks Key 2",
		KeyPrefix:               "fw_2...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               now,
		UpdatedAt:               now,
	})

	cfg := config.DefaultConfig()
	router := routing.NewRouter(db, masterKey, &cfg)

	// Select key 1
	target1, err := router.SelectNextTarget("deepseek-v4-flash", nil)
	if err != nil {
		t.Fatalf("failed to select target: %v", err)
	}
	if target1.ProviderKeyID != "fw-key-1" {
		t.Errorf("expected fw-key-1, got %s", target1.ProviderKeyID)
	}
	if target1.DecryptedSecret != "fw-secret-1" {
		t.Errorf("expected decrypted secret fw-secret-1, got %s", target1.DecryptedSecret)
	}

	// Next selection should round-robin to key 2
	target2, err := router.SelectNextTarget("deepseek-v4-flash", nil)
	if err != nil {
		t.Fatalf("failed to select target 2: %v", err)
	}
	if target2.ProviderKeyID != "fw-key-2" {
		t.Errorf("expected fw-key-2, got %s", target2.ProviderKeyID)
	}

	// Put key 1 on cooldown
	router.MarkKeyCooldown("fw-key-1", 10*time.Second)

	// Now key 1 is in cooldown, selecting next should give key 2
	target3, err := router.SelectNextTarget("deepseek-v4-flash", nil)
	if err != nil {
		t.Fatalf("failed to select target 3: %v", err)
	}
	if target3.ProviderKeyID != "fw-key-2" {
		t.Errorf("expected fw-key-2 because key 1 is in cooldown, got %s", target3.ProviderKeyID)
	}

	// Mark key 2 auth invalid (e.g. 401 response)
	err = router.MarkKeyAuthInvalid("fw-key-2", "invalid API key")
	if err != nil {
		t.Fatalf("failed to mark key invalid: %v", err)
	}

	// Now fw-key-2 is marked invalid in DB and fw-key-1 is in cooldown -> router should advance to 2nd priority provider (siliconflow)
	// Since siliconflow has no keys yet, it should return an informative error
	_, err = router.SelectNextTarget("deepseek-v4-flash", nil)
	if err == nil {
		t.Fatalf("expected error when no active healthy keys are available")
	}
}

func TestRouterSkipsUndecryptableKey(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "gateway.db"), 5000)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.Migrate()
	_ = db.SeedDefaults()

	masterKey, _ := crypto.GenerateRandomBytes(32)
	validSecret, _ := crypto.Encrypt(masterKey, "valid-secret")
	now := time.Now().UTC()
	for _, key := range []database.ProviderKey{
		{ID: "corrupt", ProviderID: "fireworks", EncryptedSecret: "not-ciphertext", DisplayName: "Corrupt", KeyPrefix: "bad...", Status: "active", CreatedAt: now.Add(time.Second), UpdatedAt: now},
		{ID: "valid", ProviderID: "fireworks", EncryptedSecret: validSecret, DisplayName: "Valid", KeyPrefix: "good...", Status: "active", CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.CreateProviderKey(key); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.DefaultConfig()
	target, err := routing.NewRouter(db, masterKey, &cfg).SelectNextTarget("deepseek-v4-flash", nil)
	if err != nil {
		t.Fatal(err)
	}
	if target.ProviderKeyID != "valid" || target.DecryptedSecret != "valid-secret" {
		t.Fatalf("expected valid fallback key, got %#v", target)
	}
	corrupt, err := db.GetProviderKey("corrupt")
	if err != nil || corrupt.Status != "invalid" {
		t.Fatalf("expected corrupt key to be marked invalid, got %#v, %v", corrupt, err)
	}
}
