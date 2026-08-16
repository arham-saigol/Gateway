package adminapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"arham-gateway/internal/auth"
	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
)

type Handler struct {
	db        *database.DB
	masterKey []byte
	cfg       *config.Config
	mux       *http.ServeMux
}

func NewHandler(db *database.DB, masterKey []byte, cfg *config.Config) http.Handler {
	h := &Handler{
		db:        db,
		masterKey: masterKey,
		cfg:       cfg,
		mux:       http.NewServeMux(),
	}

	// Auth Endpoints (Public or Semi-public)
	h.mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	h.mux.HandleFunc("POST /api/auth/logout", h.authMiddleware(h.handleLogout))
	h.mux.HandleFunc("GET /api/auth/me", h.authMiddleware(h.handleMe))
	h.mux.HandleFunc("POST /api/auth/password", h.authMiddleware(h.handlePasswordChange))

	// Overview & Analytics
	h.mux.HandleFunc("GET /api/overview", h.authMiddleware(h.handleOverview))
	h.mux.HandleFunc("GET /api/analytics", h.authMiddleware(h.handleAnalytics))

	// Providers & Keys
	h.mux.HandleFunc("GET /api/providers", h.authMiddleware(h.handleListProviders))
	h.mux.HandleFunc("POST /api/providers/keys", h.authMiddleware(h.handleCreateProviderKey))
	h.mux.HandleFunc("POST /api/providers/keys/{id}/status", h.authMiddleware(h.handleUpdateKeyStatus))
	h.mux.HandleFunc("POST /api/providers/keys/{id}/adjust", h.authMiddleware(h.handleAddKeyAdjustment))

	// Models & Routing
	h.mux.HandleFunc("GET /api/models", h.authMiddleware(h.handleListModels))
	h.mux.HandleFunc("POST /api/models/{id}/routes", h.authMiddleware(h.handleSetModelRoutes))
	h.mux.HandleFunc("POST /api/mappings/{id}/rates", h.authMiddleware(h.handleUpdateMappingRates))

	// Gateway Keys
	h.mux.HandleFunc("GET /api/gateway-keys", h.authMiddleware(h.handleListGatewayKeys))
	h.mux.HandleFunc("POST /api/gateway-keys", h.authMiddleware(h.handleCreateGatewayKey))
	h.mux.HandleFunc("POST /api/gateway-keys/{id}/revoke", h.authMiddleware(h.handleRevokeGatewayKey))

	// Request Logs
	h.mux.HandleFunc("GET /api/requests", h.authMiddleware(h.handleListRequests))
	h.mux.HandleFunc("GET /api/requests/{id}", h.authMiddleware(h.handleGetRequestDetail))

	// Settings
	h.mux.HandleFunc("GET /api/settings", h.authMiddleware(h.handleGetSettings))

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type authContextKey struct{}

func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("arham_session")
		if err != nil || cookie.Value == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		tokenHash := auth.HashSessionToken(cookie.Value)
		csrfToken, expiresAt, revoked, err := h.db.GetAdminSession(tokenHash)
		if err != nil || revoked || time.Now().UTC().After(expiresAt) {
			http.Error(w, `{"error":"session expired or revoked"}`, http.StatusUnauthorized)
			return
		}

		// CSRF check on state-changing methods
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			clientCSRF := r.Header.Get("X-CSRF-Token")
			if !auth.ValidateCSRFToken(csrfToken, clientCSRF) {
				http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
				return
			}
		}

		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func generateRandomID(prefix string) string {
	b := make([]byte, 10)
	_, _ = io.ReadFull(rand.Reader, b)
	return prefix + hex.EncodeToString(b)
}

// ----------------- Auth Handlers -----------------

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Password == "" {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	hash, err := h.db.GetAdminPasswordHash()
	if err != nil {
		http.Error(w, `{"error":"admin password not configured"}`, http.StatusServiceUnavailable)
		return
	}

	match, err := auth.VerifyPassword(body.Password, hash)
	if err != nil || !match {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	rawToken, hashedToken, err := auth.GenerateSessionToken()
	if err != nil {
		http.Error(w, `{"error":"generating session"}`, http.StatusInternalServerError)
		return
	}

	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		http.Error(w, `{"error":"generating csrf token"}`, http.StatusInternalServerError)
		return
	}

	expiry := time.Now().UTC().Add(h.cfg.Security.AdminSessionExpiry.Duration())
	if err := h.db.CreateAdminSession(hashedToken, csrfToken, expiry); err != nil {
		http.Error(w, `{"error":"storing session"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "arham_session",
		Value:    rawToken,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		Secure:   h.cfg.Security.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"csrf_token": csrfToken,
		"expires_at": expiry,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("arham_session")
	if err == nil && cookie.Value != "" {
		tokenHash := auth.HashSessionToken(cookie.Value)
		_ = h.db.RevokeAdminSession(tokenHash)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "arham_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("arham_session")
	tokenHash := auth.HashSessionToken(cookie.Value)
	csrfToken, expiresAt, _, _ := h.db.GetAdminSession(tokenHash)

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrf_token":    csrfToken,
		"expires_at":    expiresAt,
	})
}

func (h *Handler) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OldPassword == "" || body.NewPassword == "" {
		http.Error(w, `{"error":"invalid password payload"}`, http.StatusBadRequest)
		return
	}

	hash, err := h.db.GetAdminPasswordHash()
	if err != nil {
		http.Error(w, `{"error":"admin credentials error"}`, http.StatusInternalServerError)
		return
	}

	match, err := auth.VerifyPassword(body.OldPassword, hash)
	if err != nil || !match {
		http.Error(w, `{"error":"current password incorrect"}`, http.StatusUnauthorized)
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		http.Error(w, `{"error":"hashing new password"}`, http.StatusInternalServerError)
		return
	}

	if err := h.db.SetAdminPasswordHash(newHash); err != nil {
		http.Error(w, `{"error":"saving password hash"}`, http.StatusInternalServerError)
		return
	}

	// Revoke all existing sessions
	_ = h.db.RevokeAdminSessions()

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated, all sessions revoked"})
}

// ----------------- Overview & Analytics Handlers -----------------

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.db.GetKeyBalanceSummaries()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	var totalSpendMicro, totalRemainingMicro int64
	for _, s := range summaries {
		totalSpendMicro += s.KnownSpendMicroUSD
		totalRemainingMicro += s.EstimatedRemainingMicroUSD
	}

	// Calculate 24h stats
	var total24hRequests, success24hRequests int64
	var avgLatency24h float64
	_ = h.db.Conn().QueryRow(`
		SELECT COUNT(1), COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0), COALESCE(AVG(total_duration_ms), 0)
		FROM requests
		WHERE created_at >= datetime('now', '-1 day')
	`).Scan(&total24hRequests, &success24hRequests, &avgLatency24h)

	successRate := 100.0
	if total24hRequests > 0 {
		successRate = float64(success24hRequests) / float64(total24hRequests) * 100.0
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_spend_micro_usd":     totalSpendMicro,
		"total_remaining_micro_usd": totalRemainingMicro,
		"requests_24h":              total24hRequests,
		"success_rate_24h":          successRate,
		"avg_latency_ms_24h":        avgLatency24h,
		"key_balances":              summaries,
	})
}

func (h *Handler) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Conn().Query(`
		SELECT date_utc, public_model_id, SUM(total_requests), SUM(successful_requests), SUM(failed_requests),
		       SUM(input_tokens), SUM(cached_input_tokens), SUM(output_tokens), SUM(known_cost_micro_usd)
		FROM usage_rollups_daily
		GROUP BY date_utc, public_model_id
		ORDER BY date_utc ASC
	`)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type dailyModelStat struct {
		DateUTC           string `json:"date_utc"`
		PublicModelID     string `json:"public_model_id"`
		TotalRequests     int64  `json:"total_requests"`
		SuccessRequests   int64  `json:"success_requests"`
		FailedRequests    int64  `json:"failed_requests"`
		InputTokens       int64  `json:"input_tokens"`
		CachedInputTokens int64  `json:"cached_input_tokens"`
		OutputTokens      int64  `json:"output_tokens"`
		CostMicroUSD      int64  `json:"cost_micro_usd"`
	}

	stats := make([]dailyModelStat, 0)
	for rows.Next() {
		var s dailyModelStat
		if err := rows.Scan(&s.DateUTC, &s.PublicModelID, &s.TotalRequests, &s.SuccessRequests, &s.FailedRequests,
			&s.InputTokens, &s.CachedInputTokens, &s.OutputTokens, &s.CostMicroUSD); err != nil {
			continue
		}
		stats = append(stats, s)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"daily_stats": stats,
	})
}

// ----------------- Providers & Keys Handlers -----------------

func (h *Handler) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providersList, err := h.db.ListProviders()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	keysList, err := h.db.ListProviderKeys("")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	type providerWithKeys struct {
		database.Provider
		Keys []database.ProviderKey `json:"keys"`
	}

	keysByProvider := make(map[string][]database.ProviderKey)
	for _, k := range keysList {
		keysByProvider[k.ProviderID] = append(keysByProvider[k.ProviderID], k)
	}

	result := make([]providerWithKeys, 0)
	for _, p := range providersList {
		kList := keysByProvider[p.ID]
		if kList == nil {
			kList = make([]database.ProviderKey, 0)
		}
		result = append(result, providerWithKeys{
			Provider: p,
			Keys:     kList,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleCreateProviderKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProviderID              string `json:"provider_id"`
		DisplayName             string `json:"display_name"`
		Secret                  string `json:"secret"`
		StartingBalanceMicroUSD int64  `json:"starting_balance_micro_usd"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ProviderID == "" || body.Secret == "" || body.DisplayName == "" {
		http.Error(w, `{"error":"invalid provider key payload"}`, http.StatusBadRequest)
		return
	}

	encSecret, err := crypto.Encrypt(h.masterKey, body.Secret)
	if err != nil {
		http.Error(w, `{"error":"encrypting provider secret"}`, http.StatusInternalServerError)
		return
	}

	prefix := body.Secret
	if len(prefix) > 8 {
		prefix = prefix[:8] + "..."
	}

	now := time.Now().UTC()
	keyID := generateRandomID("pkey-")

	k := database.ProviderKey{
		ID:                      keyID,
		ProviderID:              body.ProviderID,
		EncryptedSecret:         encSecret,
		DisplayName:             body.DisplayName,
		KeyPrefix:               prefix,
		StartingBalanceMicroUSD: body.StartingBalanceMicroUSD,
		Status:                  "active",
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if err := h.db.CreateProviderKey(k); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, k)
}

func (h *Handler) handleUpdateKeyStatus(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("id")
	var body struct {
		Status string `json:"status"` // active, disabled
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Status != "active" && body.Status != "disabled") {
		http.Error(w, `{"error":"invalid status"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateProviderKeyStatus(keyID, body.Status, nil); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) handleAddKeyAdjustment(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("id")
	var body struct {
		AmountMicroUSD int64  `json:"amount_micro_usd"`
		Note           string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Note == "" {
		http.Error(w, `{"error":"invalid adjustment payload"}`, http.StatusBadRequest)
		return
	}

	adj := database.BalanceAdjustment{
		ID:             generateRandomID("adj-"),
		ProviderKeyID:  keyID,
		AmountMicroUSD: body.AmountMicroUSD,
		Note:           body.Note,
		CreatedAt:      time.Now().UTC(),
	}

	if err := h.db.AddBalanceAdjustment(adj); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, adj)
}

// ----------------- Models & Routing Handlers -----------------

func (h *Handler) handleListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.db.ListPublicModels()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	mappings, err := h.db.ListProviderModelMappings()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	type modelWithRoutes struct {
		database.PublicModel
		Routes   []database.RoutingEntry       `json:"routes"`
		Mappings []database.ProviderModelMapping `json:"mappings"`
	}

	result := make([]modelWithRoutes, 0)
	for _, m := range models {
		routes, _ := h.db.GetRoutesForModel(m.ID)
		var modelMappings []database.ProviderModelMapping
		for _, mp := range mappings {
			if mp.PublicModelID == m.ID {
				modelMappings = append(modelMappings, mp)
			}
		}

		result = append(result, modelWithRoutes{
			PublicModel: m,
			Routes:      routes,
			Mappings:    modelMappings,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleSetModelRoutes(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	var body struct {
		MappingIDs []string `json:"mapping_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.MappingIDs) == 0 {
		http.Error(w, `{"error":"invalid mapping ids"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.SetRoutesForModel(modelID, body.MappingIDs); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "routes updated"})
}

func (h *Handler) handleUpdateMappingRates(w http.ResponseWriter, r *http.Request) {
	mappingID := r.PathValue("id")
	var body struct {
		InputRate  int64 `json:"input_rate"`
		CachedRate int64 `json:"cached_rate"`
		OutputRate int64 `json:"output_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InputRate < 0 || body.CachedRate < 0 || body.OutputRate < 0 {
		http.Error(w, `{"error":"invalid rate values"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateMappingRates(mappingID, body.InputRate, body.CachedRate, body.OutputRate); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rates updated"})
}

// ----------------- Gateway Keys Handlers -----------------

func (h *Handler) handleListGatewayKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.db.ListGatewayKeys()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (h *Handler) handleCreateGatewayKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, `{"error":"key name required"}`, http.StatusBadRequest)
		return
	}

	rawKey, prefix, hash, err := auth.GenerateGatewayKey(body.Name)
	if err != nil {
		http.Error(w, `{"error":"generating key"}`, http.StatusInternalServerError)
		return
	}

	gwKey := database.GatewayKey{
		ID:        generateRandomID("gkey-"),
		KeyHash:   hash,
		KeyPrefix: prefix,
		Name:      body.Name,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}

	if err := h.db.CreateGatewayKey(gwKey); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         gwKey.ID,
		"key":        rawKey, // returned only once
		"prefix":     gwKey.KeyPrefix,
		"name":       gwKey.Name,
		"created_at": gwKey.CreatedAt,
	})
}

func (h *Handler) handleRevokeGatewayKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("id")
	if err := h.db.RevokeGatewayKey(keyID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// ----------------- Request Logs Handlers -----------------

func (h *Handler) handleListRequests(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	rows, err := h.db.Conn().Query(`
		SELECT id, public_model_id, gateway_key_id, status, error_category, stream,
		       ttft_ms, total_duration_ms, input_tokens, cached_input_tokens, output_tokens,
		       total_cost_micro_usd, usage_confidence, retry_count, failover_count, created_at
		FROM requests
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	reqs := make([]database.RequestRecord, 0)
	for rows.Next() {
		var req database.RequestRecord
		if err := rows.Scan(&req.ID, &req.PublicModelID, &req.GatewayKeyID, &req.Status, &req.ErrorCategory, &req.Stream,
			&req.TTFTMs, &req.TotalDurationMs, &req.InputTokens, &req.CachedInputTokens, &req.OutputTokens,
			&req.TotalCostMicroUSD, &req.UsageConfidence, &req.RetryCount, &req.FailoverCount, &req.CreatedAt); err != nil {
			continue
		}
		reqs = append(reqs, req)
	}

	writeJSON(w, http.StatusOK, reqs)
}

func (h *Handler) handleGetRequestDetail(w http.ResponseWriter, r *http.Request) {
	reqID := r.PathValue("id")

	var req database.RequestRecord
	err := h.db.Conn().QueryRow(`
		SELECT id, public_model_id, gateway_key_id, status, error_category, stream,
		       ttft_ms, total_duration_ms, input_tokens, cached_input_tokens, output_tokens,
		       total_cost_micro_usd, usage_confidence, retry_count, failover_count, created_at
		FROM requests
		WHERE id = ?
	`, reqID).Scan(&req.ID, &req.PublicModelID, &req.GatewayKeyID, &req.Status, &req.ErrorCategory, &req.Stream,
		&req.TTFTMs, &req.TotalDurationMs, &req.InputTokens, &req.CachedInputTokens, &req.OutputTokens,
		&req.TotalCostMicroUSD, &req.UsageConfidence, &req.RetryCount, &req.FailoverCount, &req.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"request not found"}`, http.StatusNotFound)
		return
	}

	// Fetch attempts
	rows, err := h.db.Conn().Query(`
		SELECT id, request_id, provider_id, provider_key_id, mapping_id, sequence,
		       status, http_status, error_category, ttft_ms, duration_ms,
		       input_tokens, cached_input_tokens, output_tokens,
		       input_rate_snapshot, cached_rate_snapshot, output_rate_snapshot,
		       total_cost_micro_usd, usage_confidence, aggregated_in_rollup, created_at
		FROM request_attempts
		WHERE request_id = ?
		ORDER BY sequence ASC
	`, reqID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	attempts := make([]database.RequestAttemptRecord, 0)
	for rows.Next() {
		var att database.RequestAttemptRecord
		if err := rows.Scan(&att.ID, &att.RequestID, &att.ProviderID, &att.ProviderKeyID, &att.MappingID, &att.Sequence,
			&att.Status, &att.HTTPStatus, &att.ErrorCategory, &att.TTFTMs, &att.DurationMs,
			&att.InputTokens, &att.CachedInputTokens, &att.OutputTokens,
			&att.InputRateSnapshot, &att.CachedRateSnapshot, &att.OutputRateSnapshot,
			&att.TotalCostMicroUSD, &att.UsageConfidence, &att.AggregatedInRollup, &att.CreatedAt); err != nil {
			continue
		}
		attempts = append(attempts, att)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request":  req,
		"attempts": attempts,
	})
}

// ----------------- Settings Handlers -----------------

func (h *Handler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"listen_addr":                     h.cfg.Server.ListenAddr,
		"first_response_timeout_seconds": h.cfg.Timeouts.FirstResponseTimeout.Duration().Seconds(),
		"stream_drain_timeout_seconds":   h.cfg.Timeouts.StreamDrainTimeout.Duration().Seconds(),
		"detailed_log_days":              h.cfg.Retention.DetailedLogDays,
		"max_retries":                     h.cfg.Routing.MaxRetriesPerRequest,
		"key_cooldown_seconds":            h.cfg.Routing.KeyCooldownDuration.Duration().Seconds(),
	})
}
