package database_test

import (
	"path/filepath"
	"testing"
	"time"

	"arham-gateway/internal/database"
)

func TestOpenAndMigrateDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gateway.db")

	db, err := database.Open(dbPath, 5000)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Migration should be idempotent
	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to re-run migrations idempotently: %v", err)
	}

	// Seed default data
	if err := db.SeedDefaults(); err != nil {
		t.Fatalf("failed to seed defaults: %v", err)
	}

	// Verify public models exist
	models, err := db.ListPublicModels()
	if err != nil {
		t.Fatalf("failed to list public models: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 public models, got %d", len(models))
	}

	foundFlash := false
	foundFast := false
	for _, m := range models {
		if m.ID == "deepseek-v4-flash" {
			foundFlash = true
		}
		if m.ID == "deepseek-v4-flash-fast" {
			foundFast = true
		}
	}

	if !foundFlash || !foundFast {
		t.Fatalf("missing expected public models: flash=%v, fast=%v", foundFlash, foundFast)
	}

	// Verify routing entries for deepseek-v4-flash
	routes, err := db.GetRoutesForModel("deepseek-v4-flash")
	if err != nil {
		t.Fatalf("failed to get routes: %v", err)
	}

	if len(routes) != 4 {
		t.Fatalf("expected 4 provider routes for deepseek-v4-flash, got %d", len(routes))
	}

	// Default order: Fireworks -> SiliconFlow -> Novita -> Baseten
	if routes[0].ProviderID != "fireworks" {
		t.Errorf("expected 1st route to be fireworks, got %s", routes[0].ProviderID)
	}
	if routes[1].ProviderID != "siliconflow" {
		t.Errorf("expected 2nd route to be siliconflow, got %s", routes[1].ProviderID)
	}
	if routes[2].ProviderID != "novita" {
		t.Errorf("expected 3rd route to be novita, got %s", routes[2].ProviderID)
	}
	if routes[3].ProviderID != "baseten" {
		t.Errorf("expected 4th route to be baseten, got %s", routes[3].ProviderID)
	}
}

func TestProviderKeyAndBalanceLedger(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gateway.db")

	db, err := database.Open(dbPath, 5000)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	_ = db.Migrate()
	_ = db.SeedDefaults()

	now := time.Now().UTC()
	keyID := "key-fw-1"
	err = db.CreateProviderKey(database.ProviderKey{
		ID:                      keyID,
		ProviderID:              "fireworks",
		EncryptedSecret:         "enc-secret-xyz",
		DisplayName:             "Fireworks Main",
		KeyPrefix:               "fw_1234...",
		StartingBalanceMicroUSD: 6000000, // $6.00
		Status:                  "active",
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	if err != nil {
		t.Fatalf("failed to create provider key: %v", err)
	}

	// Add manual balance adjustment (+ $5.00)
	adjID := "adj-1"
	err = db.AddBalanceAdjustment(database.BalanceAdjustment{
		ID:             adjID,
		ProviderKeyID:  keyID,
		AmountMicroUSD: 5000000,
		Note:           "Added $5 test credits",
		CreatedAt:      now,
	})
	if err != nil {
		t.Fatalf("failed to add balance adjustment: %v", err)
	}

	// Finalize an attempt costing $1.50 (1,500,000 micro-USD)
	costMicro := int64(1500000)
	inTokens := int64(1000)
	outTokens := int64(200)
	reqID := "req-1"
	attID := "att-1"

	err = db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
		ID:                 attID,
		RequestID:          reqID,
		ProviderID:         "fireworks",
		ProviderKeyID:      keyID,
		MappingID:          "map-fw-flash",
		Sequence:           1,
		Status:             "success",
		DurationMs:         120,
		InputTokens:        &inTokens,
		OutputTokens:       &outTokens,
		InputRateSnapshot:  140000,
		CachedRateSnapshot: 14000,
		OutputRateSnapshot: 280000,
		TotalCostMicroUSD:  &costMicro,
		UsageConfidence:    "provider_reported",
		CreatedAt:          now,
	}, database.RequestRecord{
		ID:                reqID,
		PublicModelID:     "deepseek-v4-flash",
		Status:            "success",
		Stream:            false,
		TotalDurationMs:   120,
		InputTokens:       &inTokens,
		OutputTokens:      &outTokens,
		TotalCostMicroUSD: &costMicro,
		UsageConfidence:   "provider_reported",
		RetryCount:        0,
		FailoverCount:     0,
		CreatedAt:         now,
	})
	if err != nil {
		t.Fatalf("failed to finalize attempt and rollup: %v", err)
	}

	summaries, err := db.GetKeyBalanceSummaries()
	if err != nil {
		t.Fatalf("failed to get balance summaries: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	s := summaries[0]
	if s.StartingBalanceMicroUSD != 6000000 {
		t.Errorf("expected starting balance $6, got %d", s.StartingBalanceMicroUSD)
	}
	if s.AdjustmentsMicroUSD != 5000000 {
		t.Errorf("expected adjustments $5, got %d", s.AdjustmentsMicroUSD)
	}
	if s.KnownSpendMicroUSD != 1500000 {
		t.Errorf("expected spend $1.50, got %d", s.KnownSpendMicroUSD)
	}
	// 6 + 5 - 1.5 = 9.5 ($9.50 = 9500000 micro-USD)
	if s.EstimatedRemainingMicroUSD != 9500000 {
		t.Errorf("expected remaining $9.50, got %d", s.EstimatedRemainingMicroUSD)
	}

	// Test Detailed Logs Pruning: Pruning requests/attempts must NOT affect balance summary or spend!
	affected, err := db.PruneDetailedLogs(0) // keep 0 days
	if err != nil {
		t.Fatalf("failed to prune logs: %v", err)
	}
	_ = affected

	summariesAfterPrune, err := db.GetKeyBalanceSummaries()
	if err != nil {
		t.Fatalf("failed to get balance summaries after prune: %v", err)
	}
	if summariesAfterPrune[0].EstimatedRemainingMicroUSD != 9500000 {
		t.Fatalf("spend/remaining balance changed after log pruning! Expected 9500000, got %d", summariesAfterPrune[0].EstimatedRemainingMicroUSD)
	}
}

func TestRollupSequenceRetryTotalRequests(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gateway.db")

	db, err := database.Open(dbPath, 5000)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	_ = db.Migrate()
	_ = db.SeedDefaults()

	now := time.Now().UTC()
	keyID := "key-fw-retry-test"
	err = db.CreateProviderKey(database.ProviderKey{
		ID:                      keyID,
		ProviderID:              "fireworks",
		EncryptedSecret:         "enc-secret-xyz",
		DisplayName:             "Fireworks Retry Test",
		KeyPrefix:               "fw_retry...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	if err != nil {
		t.Fatalf("failed to create provider key: %v", err)
	}

	_ = db.CreateGatewayKey(database.GatewayKey{
		ID:        "gw-key-1",
		KeyHash:   "test-hash-retry",
		KeyPrefix: "arham_test...",
		Name:      "test-client",
		Status:    "active",
		CreatedAt: now,
	})

	reqID := "req-retry-1"
	gwKeyID := "gw-key-1"
	errCat := "transient"
	http500 := 500
	http200 := 200

	// Attempt 1: Failed
	err = db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
		ID:                 "att-1",
		RequestID:          reqID,
		ProviderID:         "fireworks",
		ProviderKeyID:      keyID,
		MappingID:          "map-fw-flash",
		Sequence:           1,
		Status:             "error",
		HTTPStatus:         &http500,
		ErrorCategory:      &errCat,
		DurationMs:         100,
		InputRateSnapshot:  140000,
		CachedRateSnapshot: 14000,
		OutputRateSnapshot: 280000,
		UsageConfidence:    "unavailable",
		CreatedAt:          now,
	}, database.RequestRecord{
		ID:              reqID,
		PublicModelID:   "deepseek-v4-flash",
		GatewayKeyID:    &gwKeyID,
		Status:          "error",
		ErrorCategory:   &errCat,
		Stream:          false,
		TotalDurationMs: 100,
		UsageConfidence: "unavailable",
		RetryCount:      0,
		FailoverCount:   0,
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("failed to record attempt 1: %v", err)
	}

	// Attempt 2: Succeeded (same key retry)
	inTokens := int64(100)
	outTokens := int64(50)
	costMicro := int64(28000)
	err = db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
		ID:                 "att-2",
		RequestID:          reqID,
		ProviderID:         "fireworks",
		ProviderKeyID:      keyID,
		MappingID:          "map-fw-flash",
		Sequence:           2,
		Status:             "success",
		HTTPStatus:         &http200,
		DurationMs:         150,
		InputTokens:        &inTokens,
		OutputTokens:       &outTokens,
		InputRateSnapshot:  140000,
		CachedRateSnapshot: 14000,
		OutputRateSnapshot: 280000,
		TotalCostMicroUSD:  &costMicro,
		UsageConfidence:    "provider_reported",
		CreatedAt:          now,
	}, database.RequestRecord{
		ID:                reqID,
		PublicModelID:     "deepseek-v4-flash",
		GatewayKeyID:      &gwKeyID,
		Status:            "success",
		Stream:            false,
		TotalDurationMs:   250,
		InputTokens:       &inTokens,
		OutputTokens:      &outTokens,
		TotalCostMicroUSD: &costMicro,
		UsageConfidence:   "provider_reported",
		RetryCount:        1,
		FailoverCount:     0,
		CreatedAt:         now,
	})
	if err != nil {
		t.Fatalf("failed to record attempt 2: %v", err)
	}

	dateUTC := now.Format("2006-01-02")
	var totalReqs, successReqs, failReqs, retries, failovers int
	err = db.Conn().QueryRow(`
		SELECT total_requests, successful_requests, failed_requests, retries, failovers
		FROM usage_rollups_daily
		WHERE date_utc = ? AND provider_key_id = ?
	`, dateUTC, keyID).Scan(&totalReqs, &successReqs, &failReqs, &retries, &failovers)
	if err != nil {
		t.Fatalf("failed to query daily rollup: %v", err)
	}

	if totalReqs != 1 {
		t.Errorf("expected total_requests = 1, got %d", totalReqs)
	}
	if successReqs != 1 {
		t.Errorf("expected successful_requests = 1, got %d", successReqs)
	}
	if failReqs != 1 {
		t.Errorf("expected failed_requests = 1, got %d", failReqs)
	}
	if retries != 1 {
		t.Errorf("expected retries = 1, got %d", retries)
	}
	if failovers != 0 {
		t.Errorf("expected failovers = 0 for a retry on the same route, got %d", failovers)
	}

	err = db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
		ID: "att-3", RequestID: reqID, ProviderID: "siliconflow", ProviderKeyID: keyID,
		MappingID: "map-sf-flash", Sequence: 3, Status: "success", HTTPStatus: &http200,
		UsageConfidence: "unavailable", CreatedAt: now,
	}, database.RequestRecord{
		ID: reqID, PublicModelID: "deepseek-v4-flash", GatewayKeyID: &gwKeyID,
		Status: "success", UsageConfidence: "unavailable", RetryCount: 2, FailoverCount: 1, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("failed to record failover attempt: %v", err)
	}
	if err := db.Conn().QueryRow(`
		SELECT COALESCE(SUM(failovers), 0) FROM usage_rollups_daily
		WHERE date_utc = ? AND provider_key_id = ?
	`, dateUTC, keyID).Scan(&failovers); err != nil {
		t.Fatal(err)
	}
	if failovers != 1 {
		t.Errorf("expected one route failover, got %d", failovers)
	}
}
