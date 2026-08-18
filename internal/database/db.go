package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("record not found")
)

type DB struct {
	conn *sql.DB
}

func Open(dbPath string, busyTimeoutMs int) (*DB, error) {
	if busyTimeoutMs <= 0 {
		busyTimeoutMs = 5000
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating db directory %s: %w", dir, err)
	}

	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(%d)", dbPath, busyTimeoutMs)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	conn.SetMaxOpenConns(1) // Best practice for SQLite writes
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging sqlite db: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Public Models
func (db *DB) ListPublicModels() ([]PublicModel, error) {
	rows, err := db.conn.Query("SELECT id, display_name, description, enabled, created_at FROM public_models WHERE enabled = 1 ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]PublicModel, 0)
	for rows.Next() {
		var m PublicModel
		var desc sql.NullString
		if err := rows.Scan(&m.ID, &m.DisplayName, &desc, &m.Enabled, &m.CreatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			m.Description = desc.String
		}
		models = append(models, m)
	}
	return models, nil
}

func (db *DB) GetPublicModel(id string) (*PublicModel, error) {
	var m PublicModel
	var desc sql.NullString
	err := db.conn.QueryRow("SELECT id, display_name, description, enabled, created_at FROM public_models WHERE id = ?", id).
		Scan(&m.ID, &m.DisplayName, &desc, &m.Enabled, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if desc.Valid {
		m.Description = desc.String
	}
	return &m, nil
}

// Providers & Keys
func (db *DB) ListProviders() ([]Provider, error) {
	rows, err := db.conn.Query("SELECT id, name, enabled, created_at FROM providers ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]Provider, 0)
	for rows.Next() {
		var p Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.Enabled, &p.CreatedAt); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func (db *DB) GetProvider(id string) (*Provider, error) {
	row := db.conn.QueryRow("SELECT id, name, enabled, created_at FROM providers WHERE id = ?", id)
	var p Provider
	if err := row.Scan(&p.ID, &p.Name, &p.Enabled, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (db *DB) ListProviderKeys(providerID string) ([]ProviderKey, error) {
	var query string
	var args []any
	if providerID != "" {
		query = "SELECT id, provider_id, encrypted_secret, display_name, key_prefix, starting_balance_micro_usd, status, safe_last_error, last_used_at, created_at, updated_at FROM provider_keys WHERE provider_id = ? ORDER BY created_at DESC"
		args = append(args, providerID)
	} else {
		query = "SELECT id, provider_id, encrypted_secret, display_name, key_prefix, starting_balance_micro_usd, status, safe_last_error, last_used_at, created_at, updated_at FROM provider_keys ORDER BY created_at DESC"
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]ProviderKey, 0)
	for rows.Next() {
		var k ProviderKey
		var safeErr sql.NullString
		var lastUsedTime sql.NullTime
		if err := rows.Scan(&k.ID, &k.ProviderID, &k.EncryptedSecret, &k.DisplayName, &k.KeyPrefix, &k.StartingBalanceMicroUSD, &k.Status, &safeErr, &lastUsedTime, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, err
		}
		if safeErr.Valid {
			k.SafeLastError = &safeErr.String
		}
		if lastUsedTime.Valid {
			k.LastUsedAt = &lastUsedTime.Time
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *DB) GetProviderKey(id string) (*ProviderKey, error) {
	row := db.conn.QueryRow("SELECT id, provider_id, encrypted_secret, display_name, key_prefix, starting_balance_micro_usd, status, safe_last_error, last_used_at, created_at, updated_at FROM provider_keys WHERE id = ?", id)
	var k ProviderKey
	var safeErr sql.NullString
	var lastUsedTime sql.NullTime
	if err := row.Scan(&k.ID, &k.ProviderID, &k.EncryptedSecret, &k.DisplayName, &k.KeyPrefix, &k.StartingBalanceMicroUSD, &k.Status, &safeErr, &lastUsedTime, &k.CreatedAt, &k.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if safeErr.Valid {
		k.SafeLastError = &safeErr.String
	}
	if lastUsedTime.Valid {
		k.LastUsedAt = &lastUsedTime.Time
	}
	return &k, nil
}

func (db *DB) CreateProviderKey(k ProviderKey) error {
	_, err := db.conn.Exec(`
		INSERT INTO provider_keys (id, provider_id, encrypted_secret, display_name, key_prefix, starting_balance_micro_usd, status, safe_last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, k.ID, k.ProviderID, k.EncryptedSecret, k.DisplayName, k.KeyPrefix, k.StartingBalanceMicroUSD, k.Status, k.SafeLastError, k.CreatedAt, k.UpdatedAt)
	return err
}

func (db *DB) UpdateProviderKeyStatus(id, status string, safeLastError *string) error {
	res, err := db.conn.Exec(`
		UPDATE provider_keys
		SET status = ?, safe_last_error = ?, updated_at = ?
		WHERE id = ?
	`, status, safeLastError, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) UpdateProviderKeyLastUsed(id string) error {
	now := time.Now().UTC()
	_, err := db.conn.Exec("UPDATE provider_keys SET last_used_at = ?, updated_at = ? WHERE id = ?", now, now, id)
	return err
}

func (db *DB) AddBalanceAdjustment(adj BalanceAdjustment) error {
	_, err := db.conn.Exec(`
		INSERT INTO balance_adjustments (id, provider_key_id, amount_micro_usd, note, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, adj.ID, adj.ProviderKeyID, adj.AmountMicroUSD, adj.Note, adj.CreatedAt)
	return err
}

func (db *DB) GetKeyBalanceSummaries() ([]KeyBalanceSummary, error) {
	query := `
		SELECT 
			pk.id, pk.provider_id, pk.display_name, pk.key_prefix, pk.starting_balance_micro_usd, pk.status,
			COALESCE((SELECT SUM(amount_micro_usd) FROM balance_adjustments WHERE provider_key_id = pk.id), 0) as adjustments,
			COALESCE((SELECT SUM(known_cost_micro_usd) FROM usage_rollups_daily WHERE provider_key_id = pk.id), 0) as known_spend
		FROM provider_keys pk
		ORDER BY pk.provider_id ASC, pk.created_at ASC
	`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]KeyBalanceSummary, 0)
	for rows.Next() {
		var s KeyBalanceSummary
		if err := rows.Scan(&s.ProviderKeyID, &s.ProviderID, &s.DisplayName, &s.KeyPrefix, &s.StartingBalanceMicroUSD, &s.Status, &s.AdjustmentsMicroUSD, &s.KnownSpendMicroUSD); err != nil {
			return nil, err
		}
		s.EstimatedRemainingMicroUSD = s.StartingBalanceMicroUSD + s.AdjustmentsMicroUSD - s.KnownSpendMicroUSD
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// Provider Model Mappings
func (db *DB) ListProviderModelMappings() ([]ProviderModelMapping, error) {
	rows, err := db.conn.Query(`
		SELECT id, provider_id, public_model_id, upstream_model_id,
		       supports_streaming, supports_tools,
		       input_rate_per_m_tokens, cached_rate_per_m_tokens, output_rate_per_m_tokens,
		       currency, enabled, created_at, updated_at
		FROM provider_model_mappings
		ORDER BY public_model_id ASC, provider_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mappings := make([]ProviderModelMapping, 0)
	for rows.Next() {
		var m ProviderModelMapping
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.PublicModelID, &m.UpstreamModelID,
			&m.SupportsStreaming, &m.SupportsTools,
			&m.InputRatePerMTokens, &m.CachedRatePerMTokens, &m.OutputRatePerMTokens,
			&m.Currency, &m.Enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, nil
}

func (db *DB) UpdateMappingRates(id string, inputRate, cachedRate, outputRate int64) error {
	_, err := db.conn.Exec(`
		UPDATE provider_model_mappings
		SET input_rate_per_m_tokens = ?, cached_rate_per_m_tokens = ?, output_rate_per_m_tokens = ?, updated_at = ?
		WHERE id = ?
	`, inputRate, cachedRate, outputRate, time.Now().UTC(), id)
	return err
}

// Routes
func (db *DB) GetRoutesForModel(publicModelID string) ([]RoutingEntry, error) {
	rows, err := db.conn.Query(`
		SELECT re.id, re.public_model_id, re.mapping_id, re.priority, pmm.provider_id, pmm.upstream_model_id,
		       pmm.input_rate_per_m_tokens, pmm.cached_rate_per_m_tokens, pmm.output_rate_per_m_tokens, pmm.enabled
		FROM routing_entries re
		JOIN provider_model_mappings pmm ON re.mapping_id = pmm.id
		WHERE re.public_model_id = ?
		ORDER BY re.priority ASC
	`, publicModelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]RoutingEntry, 0)
	for rows.Next() {
		var e RoutingEntry
		if err := rows.Scan(&e.ID, &e.PublicModelID, &e.MappingID, &e.Priority, &e.ProviderID, &e.UpstreamModelID,
			&e.InputRatePerMTokens, &e.CachedRatePerMTokens, &e.OutputRatePerMTokens, &e.Enabled); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) SetRoutesForModel(publicModelID string, mappingIDsInOrder []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM routing_entries WHERE public_model_id = ?", publicModelID); err != nil {
		return fmt.Errorf("clearing old routes: %w", err)
	}

	now := time.Now().UTC()
	for i, mappingID := range mappingIDsInOrder {
		id := fmt.Sprintf("route-%s-%d-%d", publicModelID, i+1, now.Unix())
		_, err := tx.Exec(`
			INSERT INTO routing_entries (id, public_model_id, mapping_id, priority, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, id, publicModelID, mappingID, i+1, now)
		if err != nil {
			return fmt.Errorf("inserting route %s at priority %d: %w", mappingID, i+1, err)
		}
	}

	return tx.Commit()
}

// Gateway Keys
func (db *DB) CreateGatewayKey(k GatewayKey) error {
	_, err := db.conn.Exec(`
		INSERT INTO gateway_keys (id, key_hash, key_prefix, name, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, k.ID, k.KeyHash, k.KeyPrefix, k.Name, k.Status, k.CreatedAt)
	return err
}

func (db *DB) GetGatewayKeyByHash(keyHash string) (*GatewayKey, error) {
	var k GatewayKey
	var lastUsed, revoked sql.NullTime
	err := db.conn.QueryRow("SELECT id, key_hash, key_prefix, name, status, created_at, last_used_at, revoked_at FROM gateway_keys WHERE key_hash = ?", keyHash).
		Scan(&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Name, &k.Status, &k.CreatedAt, &lastUsed, &revoked)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if lastUsed.Valid {
		k.LastUsedAt = &lastUsed.Time
	}
	if revoked.Valid {
		k.RevokedAt = &revoked.Time
	}
	return &k, nil
}

func (db *DB) ListGatewayKeys() ([]GatewayKey, error) {
	rows, err := db.conn.Query("SELECT id, key_hash, key_prefix, name, status, created_at, last_used_at, revoked_at FROM gateway_keys ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]GatewayKey, 0)
	for rows.Next() {
		var k GatewayKey
		var lastUsed, revoked sql.NullTime
		if err := rows.Scan(&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Name, &k.Status, &k.CreatedAt, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			k.LastUsedAt = &lastUsed.Time
		}
		if revoked.Valid {
			k.RevokedAt = &revoked.Time
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *DB) RevokeGatewayKey(id string) error {
	now := time.Now().UTC()
	_, err := db.conn.Exec("UPDATE gateway_keys SET status = 'revoked', revoked_at = ? WHERE id = ?", now, id)
	return err
}

func (db *DB) UpdateGatewayKeyLastUsed(id string) error {
	now := time.Now().UTC()
	_, err := db.conn.Exec("UPDATE gateway_keys SET last_used_at = ? WHERE id = ?", now, id)
	return err
}

// Request and Attempt Recording + Rollup
func (db *DB) FinalizeAttemptAndRollup(att RequestAttemptRecord, req RequestRecord) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert or replace request record
	_, err = tx.Exec(`
		INSERT INTO requests (
			id, public_model_id, gateway_key_id, status, error_category, stream,
			ttft_ms, total_duration_ms, input_tokens, cached_input_tokens, output_tokens,
			total_cost_micro_usd, usage_confidence, retry_count, failover_count, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			error_category = excluded.error_category,
			ttft_ms = excluded.ttft_ms,
			total_duration_ms = excluded.total_duration_ms,
			input_tokens = excluded.input_tokens,
			cached_input_tokens = excluded.cached_input_tokens,
			output_tokens = excluded.output_tokens,
			total_cost_micro_usd = excluded.total_cost_micro_usd,
			usage_confidence = excluded.usage_confidence,
			retry_count = excluded.retry_count,
			failover_count = excluded.failover_count
	`, req.ID, req.PublicModelID, req.GatewayKeyID, req.Status, req.ErrorCategory, req.Stream,
		req.TTFTMs, req.TotalDurationMs, req.InputTokens, req.CachedInputTokens, req.OutputTokens,
		req.TotalCostMicroUSD, req.UsageConfidence, req.RetryCount, req.FailoverCount, req.CreatedAt)
	if err != nil {
		return fmt.Errorf("upserting request: %w", err)
	}

	// Insert attempt record
	_, err = tx.Exec(`
		INSERT INTO request_attempts (
			id, request_id, provider_id, provider_key_id, mapping_id, sequence,
			status, http_status, error_category, ttft_ms, duration_ms,
			input_tokens, cached_input_tokens, output_tokens,
			input_rate_snapshot, cached_rate_snapshot, output_rate_snapshot,
			total_cost_micro_usd, usage_confidence, aggregated_in_rollup, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
	`, att.ID, att.RequestID, att.ProviderID, att.ProviderKeyID, att.MappingID, att.Sequence,
		att.Status, att.HTTPStatus, att.ErrorCategory, att.TTFTMs, att.DurationMs,
		att.InputTokens, att.CachedInputTokens, att.OutputTokens,
		att.InputRateSnapshot, att.CachedRateSnapshot, att.OutputRateSnapshot,
		att.TotalCostMicroUSD, att.UsageConfidence, att.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting attempt: %w", err)
	}

	// Upsert Daily Rollup
	dateUTC := att.CreatedAt.UTC().Format("2006-01-02")
	rollupID := fmt.Sprintf("roll-%s-%s-%s-%s", dateUTC, att.ProviderKeyID, att.MappingID, func() string {
		if req.GatewayKeyID != nil {
			return *req.GatewayKeyID
		}
		return "none"
	}())

	isInitial := 0
	if att.Sequence <= 1 {
		isInitial = 1
	}

	isSuccess := 0
	isFail := 0
	if att.Status == "success" {
		isSuccess = 1
	} else {
		isFail = 1
	}

	var inTokens, cachedTokens, outTokens, costMicro int64
	var unknownCost int
	if att.InputTokens != nil {
		inTokens = *att.InputTokens
	}
	if att.CachedInputTokens != nil {
		cachedTokens = *att.CachedInputTokens
	}
	if att.OutputTokens != nil {
		outTokens = *att.OutputTokens
	}
	if att.TotalCostMicroUSD != nil {
		costMicro = *att.TotalCostMicroUSD
	} else {
		unknownCost = 1
	}

	retries := 0
	if att.Sequence > 1 {
		retries = 1
	}
	failovers := 0
	if att.Sequence > 1 {
		var previousMappingID string
		err := tx.QueryRow(`
			SELECT mapping_id FROM request_attempts
			WHERE request_id = ? AND sequence = ?
		`, att.RequestID, att.Sequence-1).Scan(&previousMappingID)
		if err == nil && previousMappingID != att.MappingID {
			failovers = 1
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("reading previous attempt: %w", err)
		}
	}

	now := time.Now().UTC()

	_, err = tx.Exec(`
		INSERT INTO usage_rollups_daily (
			id, date_utc, provider_key_id, mapping_id, public_model_id, gateway_key_id,
			total_requests, successful_requests, failed_requests, retries, failovers,
			input_tokens, cached_input_tokens, output_tokens, known_cost_micro_usd, unknown_cost_attempts,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			total_requests = total_requests + excluded.total_requests,
			successful_requests = successful_requests + excluded.successful_requests,
			failed_requests = failed_requests + excluded.failed_requests,
			retries = retries + excluded.retries,
			failovers = failovers + excluded.failovers,
			input_tokens = input_tokens + excluded.input_tokens,
			cached_input_tokens = cached_input_tokens + excluded.cached_input_tokens,
			output_tokens = output_tokens + excluded.output_tokens,
			known_cost_micro_usd = known_cost_micro_usd + excluded.known_cost_micro_usd,
			unknown_cost_attempts = unknown_cost_attempts + excluded.unknown_cost_attempts,
			updated_at = excluded.updated_at
	`, rollupID, dateUTC, att.ProviderKeyID, att.MappingID, req.PublicModelID, req.GatewayKeyID,
		isInitial, isSuccess, isFail, retries, failovers,
		inTokens, cachedTokens, outTokens, costMicro, unknownCost, now)
	if err != nil {
		return fmt.Errorf("upserting daily rollup: %w", err)
	}

	return tx.Commit()
}

// Admin Credentials & Sessions
func (db *DB) GetAdminPasswordHash() (string, error) {
	var hash string
	err := db.conn.QueryRow("SELECT password_hash FROM admin_credentials WHERE id = 1").Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return hash, nil
}

func (db *DB) SetAdminPasswordHash(hash string) error {
	_, err := db.conn.Exec(`
		INSERT INTO admin_credentials (id, password_hash, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			password_hash = excluded.password_hash,
			updated_at = excluded.updated_at
	`, hash, time.Now().UTC())
	return err
}

func (db *DB) CreateAdminSession(tokenHash, csrfToken string, expiresAt time.Time) error {
	_, err := db.conn.Exec(`
		INSERT INTO admin_sessions (token_hash, csrf_token, created_at, expires_at, revoked)
		VALUES (?, ?, ?, ?, 0)
	`, tokenHash, csrfToken, time.Now().UTC(), expiresAt)
	return err
}

func (db *DB) GetAdminSession(tokenHash string) (csrfToken string, expiresAt time.Time, revoked bool, err error) {
	var revokedInt int
	err = db.conn.QueryRow("SELECT csrf_token, expires_at, revoked FROM admin_sessions WHERE token_hash = ?", tokenHash).
		Scan(&csrfToken, &expiresAt, &revokedInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", time.Time{}, false, ErrNotFound
		}
		return "", time.Time{}, false, err
	}
	return csrfToken, expiresAt, revokedInt == 1, nil
}

func (db *DB) RevokeAdminSessions() error {
	_, err := db.conn.Exec("UPDATE admin_sessions SET revoked = 1")
	return err
}

func (db *DB) RevokeAdminSession(tokenHash string) error {
	_, err := db.conn.Exec("UPDATE admin_sessions SET revoked = 1 WHERE token_hash = ?", tokenHash)
	return err
}

// Retention Cleanup: Prunes old detailed logs while preserving daily rollups and balance adjustments
func (db *DB) PruneDetailedLogs(daysToKeep int) (int64, error) {
	if daysToKeep <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -daysToKeep)
	res, err := db.conn.Exec("DELETE FROM requests WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
