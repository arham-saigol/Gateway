package publicapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"arham-gateway/internal/accounting"
	"arham-gateway/internal/auth"
	"arham-gateway/internal/config"
	"arham-gateway/internal/database"
	"arham-gateway/internal/providers"
	"arham-gateway/internal/routing"
)

type Handler struct {
	db     *database.DB
	router *routing.Router
	cfg    *config.Config
	mux    *http.ServeMux
}

func NewHandler(db *database.DB, router *routing.Router, cfg *config.Config) http.Handler {
	h := &Handler{
		db:     db,
		router: router,
		cfg:    cfg,
		mux:    http.NewServeMux(),
	}

	h.mux.HandleFunc("GET /healthz", h.handleHealthz)
	h.mux.HandleFunc("GET /readyz", h.handleReadyz)
	h.mux.HandleFunc("GET /v1/models", h.handleListModels)
	h.mux.HandleFunc("POST /v1/chat/completions", h.handleChatCompletions)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Conn().Ping(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "database unavailable"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (h *Handler) authenticateGatewayKey(r *http.Request) (*database.GatewayKey, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, errors.New("invalid Authorization format, expected Bearer <token>")
	}

	rawKey := strings.TrimSpace(parts[1])
	if rawKey == "" {
		return nil, errors.New("empty bearer token")
	}

	keyHash := auth.HashGatewayKey(rawKey)
	gwKey, err := h.db.GetGatewayKeyByHash(keyHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, errors.New("invalid or revoked API key")
		}
		return nil, err
	}

	if gwKey.Status != "active" {
		return nil, errors.New("API key is inactive or revoked")
	}

	_ = h.db.UpdateGatewayKeyLastUsed(gwKey.ID)
	return gwKey, nil
}

func writeOpenAIError(w http.ResponseWriter, statusCode int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	})
}

func generateID(prefix string) string {
	b := make([]byte, 12)
	_, _ = io.ReadFull(rand.Reader, b)
	return prefix + hex.EncodeToString(b)
}

func (h *Handler) handleListModels(w http.ResponseWriter, r *http.Request) {
	if _, err := h.authenticateGatewayKey(r); err != nil {
		writeOpenAIError(w, http.StatusUnauthorized, "Incorrect API key provided.", "invalid_request_error")
		return
	}

	models, err := h.db.ListPublicModels()
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "Failed to retrieve models.", "api_error")
		return
	}

	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	var data []modelEntry
	for _, m := range models {
		data = append(data, modelEntry{
			ID:      m.ID,
			Object:  "model",
			Created: m.CreatedAt.Unix(),
			OwnedBy: "arham",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   data,
	})
}

func (h *Handler) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	gwKey, err := h.authenticateGatewayKey(r)
	if err != nil {
		writeOpenAIError(w, http.StatusUnauthorized, "Incorrect API key provided.", "invalid_request_error")
		return
	}

	var reqBody providers.ChatRequest
	bodyReader := io.LimitReader(r.Body, h.cfg.Server.MaxRequestBodyBytes)
	if err := json.NewDecoder(bodyReader).Decode(&reqBody); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "Invalid JSON body.", "invalid_request_error")
		return
	}

	if reqBody.Model == "" {
		writeOpenAIError(w, http.StatusBadRequest, "Missing required field: model.", "invalid_request_error")
		return
	}

	if len(reqBody.Messages) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "Missing required field: messages.", "invalid_request_error")
		return
	}

	// Validate public model alias
	publicModel, err := h.db.GetPublicModel(reqBody.Model)
	if err != nil || !publicModel.Enabled {
		writeOpenAIError(w, http.StatusBadRequest, fmt.Sprintf("The model '%s' does not exist or is not enabled.", reqBody.Model), "invalid_request_error")
		return
	}

	// Reject if both max_tokens and max_completion_tokens are provided
	if reqBody.MaxTokens != nil && reqBody.MaxCompletionTokens != nil {
		writeOpenAIError(w, http.StatusBadRequest, "Cannot specify both max_tokens and max_completion_tokens.", "invalid_request_error")
		return
	}

	gatewayRequestID := generateID("chatcmpl-arham-")
	reqStartTime := time.Now()

	if reqBody.Stream {
		h.handleStreamingChat(w, r, gwKey, publicModel, &reqBody, gatewayRequestID, reqStartTime)
	} else {
		h.handleNonStreamingChat(w, r, gwKey, publicModel, &reqBody, gatewayRequestID, reqStartTime)
	}
}

func (h *Handler) handleNonStreamingChat(w http.ResponseWriter, r *http.Request, gwKey *database.GatewayKey, publicModel *database.PublicModel, req *providers.ChatRequest, gatewayRequestID string, reqStartTime time.Time) {
	maxRetries := h.cfg.Routing.MaxRetriesPerRequest
	attemptedKeys := make(map[string]bool)
	var lastErr error

	for attemptSeq := 1; attemptSeq <= maxRetries+1; attemptSeq++ {
		target, err := h.router.SelectNextTarget(publicModel.ID, attemptedKeys)
		if err != nil {
			lastErr = err
			break
		}
		attemptedKeys[target.ProviderKeyID] = true

		adapter, ok := h.router.GetAdapter(target.ProviderID)
		if !ok {
			lastErr = fmt.Errorf("adapter for provider %s not found", target.ProviderID)
			continue
		}

		attemptID := generateID("att-")
		attemptStart := time.Now()

		chatResp, stats, err := adapter.ExecuteChat(r.Context(), target.DecryptedSecret, target.UpstreamModelID, req)
		attemptDuration := time.Since(attemptStart)

		if err != nil {
			classification := adapter.ClassifyError(stats.HTTPStatus, err)
			lastErr = err

			var errCat *string
			if classification != providers.ErrorClassificationNone {
				cStr := string(classification)
				errCat = &cStr
			}

			// Record failed attempt
			_ = h.db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
				ID:                 attemptID,
				RequestID:          gatewayRequestID,
				ProviderID:         target.ProviderID,
				ProviderKeyID:      target.ProviderKeyID,
				MappingID:          target.MappingID,
				Sequence:           attemptSeq,
				Status:             "error",
				HTTPStatus:         &stats.HTTPStatus,
				ErrorCategory:      errCat,
				DurationMs:         attemptDuration.Milliseconds(),
				InputRateSnapshot:  target.InputRateSnapshot,
				CachedRateSnapshot: target.CachedRateSnapshot,
				OutputRateSnapshot: target.OutputRateSnapshot,
				UsageConfidence:    "unavailable",
				CreatedAt:          attemptStart.UTC(),
			}, database.RequestRecord{
				ID:              gatewayRequestID,
				PublicModelID:   publicModel.ID,
				GatewayKeyID:    &gwKey.ID,
				Status:          "error",
				ErrorCategory:   errCat,
				Stream:          false,
				TotalDurationMs: time.Since(reqStartTime).Milliseconds(),
				UsageConfidence: "unavailable",
				RetryCount:      attemptSeq - 1,
				FailoverCount:   attemptSeq - 1,
				CreatedAt:       reqStartTime.UTC(),
			})

			if classification == providers.ErrorClassificationAuthInvalid {
				_ = h.router.MarkKeyAuthInvalid(target.ProviderKeyID, "Authentication failure with upstream provider")
			} else if classification.IsRetryable() {
				h.router.MarkKeyCooldown(target.ProviderKeyID, h.cfg.Routing.KeyCooldownDuration.Duration())
			}

			if !classification.IsRetryable() {
				// Non-retryable error: fail immediately
				writeOpenAIError(w, http.StatusBadRequest, "Invalid request parameters for upstream model.", "invalid_request_error")
				return
			}

			// Retry with next key/provider
			continue
		}

		// Success! Calculate exact integer cost
		var costMicro *int64
		if c, ok := accounting.CalculateAttemptCost(stats.InputTokens, stats.CachedInputTokens, stats.OutputTokens, target.InputRateSnapshot, target.CachedRateSnapshot, target.OutputRateSnapshot); ok {
			costMicro = &c
		}

		// Record successful attempt and request
		_ = h.db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
			ID:                 attemptID,
			RequestID:          gatewayRequestID,
			ProviderID:         target.ProviderID,
			ProviderKeyID:      target.ProviderKeyID,
			MappingID:          target.MappingID,
			Sequence:           attemptSeq,
			Status:             "success",
			HTTPStatus:         &stats.HTTPStatus,
			DurationMs:         attemptDuration.Milliseconds(),
			InputTokens:        &stats.InputTokens,
			CachedInputTokens:  &stats.CachedInputTokens,
			OutputTokens:       &stats.OutputTokens,
			InputRateSnapshot:  target.InputRateSnapshot,
			CachedRateSnapshot: target.CachedRateSnapshot,
			OutputRateSnapshot: target.OutputRateSnapshot,
			TotalCostMicroUSD:  costMicro,
			UsageConfidence:    stats.UsageConfidence,
			CreatedAt:          attemptStart.UTC(),
		}, database.RequestRecord{
			ID:                gatewayRequestID,
			PublicModelID:     publicModel.ID,
			GatewayKeyID:      &gwKey.ID,
			Status:            "success",
			Stream:            false,
			TotalDurationMs:   time.Since(reqStartTime).Milliseconds(),
			InputTokens:       &stats.InputTokens,
			CachedInputTokens: &stats.CachedInputTokens,
			OutputTokens:      &stats.OutputTokens,
			TotalCostMicroUSD: costMicro,
			UsageConfidence:   stats.UsageConfidence,
			RetryCount:        attemptSeq - 1,
			FailoverCount:     attemptSeq - 1,
			CreatedAt:         reqStartTime.UTC(),
		})

		// Normalize response: replace upstream ID with gateway ID and model with public alias
		chatResp.ID = gatewayRequestID
		chatResp.Model = publicModel.ID

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResp)
		return
	}

	// All attempts failed
	writeOpenAIError(w, http.StatusBadGateway, fmt.Sprintf("All upstream provider attempts failed: %v", lastErr), "api_error")
}

func (h *Handler) handleStreamingChat(w http.ResponseWriter, r *http.Request, gwKey *database.GatewayKey, publicModel *database.PublicModel, req *providers.ChatRequest, gatewayRequestID string, reqStartTime time.Time) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeOpenAIError(w, http.StatusInternalServerError, "Streaming unsupported by server.", "api_error")
		return
	}

	maxRetries := h.cfg.Routing.MaxRetriesPerRequest
	attemptedKeys := make(map[string]bool)
	var lastErr error

	for attemptSeq := 1; attemptSeq <= maxRetries+1; attemptSeq++ {
		target, err := h.router.SelectNextTarget(publicModel.ID, attemptedKeys)
		if err != nil {
			lastErr = err
			break
		}
		attemptedKeys[target.ProviderKeyID] = true

		adapter, ok := h.router.GetAdapter(target.ProviderID)
		if !ok {
			lastErr = fmt.Errorf("adapter for provider %s not found", target.ProviderID)
			continue
		}

		attemptID := generateID("att-")
		attemptStart := time.Now()

		eventChan, err := adapter.StreamChat(r.Context(), target.DecryptedSecret, target.UpstreamModelID, req)
		if err != nil {
			classification := adapter.ClassifyError(0, err)
			lastErr = err

			if classification == providers.ErrorClassificationAuthInvalid {
				_ = h.router.MarkKeyAuthInvalid(target.ProviderKeyID, "Authentication failure with upstream provider")
			} else if classification.IsRetryable() {
				h.router.MarkKeyCooldown(target.ProviderKeyID, h.cfg.Routing.KeyCooldownDuration.Duration())
			}

			if !classification.IsRetryable() {
				writeOpenAIError(w, http.StatusBadRequest, "Invalid request parameters for upstream model.", "invalid_request_error")
				return
			}
			continue
		}

		// Stream established
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		var ttftMs *int64
		var finalUsage *providers.Usage
		var streamErr error

		for ev := range eventChan {
			if ev.Error != nil {
				streamErr = ev.Error
				break
			}

			if ev.IsFirst && ev.TTFT > 0 {
				ttft := ev.TTFT.Milliseconds()
				ttftMs = &ttft
			}

			if ev.Chunk != nil {
				// Normalize chunk
				ev.Chunk.ID = gatewayRequestID
				ev.Chunk.Model = publicModel.ID

				if ev.Chunk.Usage != nil {
					finalUsage = ev.Chunk.Usage
				}

				chunkBytes, err := json.Marshal(ev.Chunk)
				if err == nil {
					_, _ = fmt.Fprintf(w, "data: %s\n\n", string(chunkBytes))
					flusher.Flush()
				}
			}
		}

		// Send [DONE] only if stream completed without error
		if streamErr == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}

		attemptDuration := time.Since(attemptStart)

		// Finalize accounting
		var inTokens, cachedTokens, outTokens int64
		usageConf := "unavailable"
		var costMicro *int64

		if finalUsage != nil {
			inTokens = finalUsage.PromptTokens
			outTokens = finalUsage.CompletionTokens
			if finalUsage.PromptTokensDetails != nil {
				cachedTokens = finalUsage.PromptTokensDetails.CachedTokens
			}
			usageConf = "provider_reported"
			if c, ok := accounting.CalculateAttemptCost(inTokens, cachedTokens, outTokens, target.InputRateSnapshot, target.CachedRateSnapshot, target.OutputRateSnapshot); ok {
				costMicro = &c
			}
		}

		status := "success"
		var errCat *string
		if streamErr != nil {
			status = "error"
			c := string(adapter.ClassifyError(0, streamErr))
			errCat = &c
		}

		var inPtr, cachedPtr, outPtr *int64
		if usageConf == "provider_reported" {
			inPtr = &inTokens
			cachedPtr = &cachedTokens
			outPtr = &outTokens
		}

		_ = h.db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
			ID:                 attemptID,
			RequestID:          gatewayRequestID,
			ProviderID:         target.ProviderID,
			ProviderKeyID:      target.ProviderKeyID,
			MappingID:          target.MappingID,
			Sequence:           attemptSeq,
			Status:             status,
			ErrorCategory:      errCat,
			TTFTMs:             ttftMs,
			DurationMs:         attemptDuration.Milliseconds(),
			InputTokens:        inPtr,
			CachedInputTokens:  cachedPtr,
			OutputTokens:       outPtr,
			InputRateSnapshot:  target.InputRateSnapshot,
			CachedRateSnapshot: target.CachedRateSnapshot,
			OutputRateSnapshot: target.OutputRateSnapshot,
			TotalCostMicroUSD:  costMicro,
			UsageConfidence:    usageConf,
			CreatedAt:          attemptStart.UTC(),
		}, database.RequestRecord{
			ID:                gatewayRequestID,
			PublicModelID:     publicModel.ID,
			GatewayKeyID:      &gwKey.ID,
			Status:            status,
			ErrorCategory:     errCat,
			Stream:            true,
			TTFTMs:            ttftMs,
			TotalDurationMs:   time.Since(reqStartTime).Milliseconds(),
			InputTokens:       inPtr,
			CachedInputTokens: cachedPtr,
			OutputTokens:      outPtr,
			TotalCostMicroUSD: costMicro,
			UsageConfidence:   usageConf,
			RetryCount:        attemptSeq - 1,
			FailoverCount:     attemptSeq - 1,
			CreatedAt:         reqStartTime.UTC(),
		})

		return
	}

	writeOpenAIError(w, http.StatusBadGateway, fmt.Sprintf("All upstream streaming attempts failed: %v", lastErr), "api_error")
}
