package publicapi

import (
	"context"
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
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Conn().Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("database unavailable"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

type openAIErrorResponse struct {
	Error openAIErrorDetail `json:"error"`
}

type openAIErrorDetail struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

func writeOpenAIError(w http.ResponseWriter, statusCode int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(openAIErrorResponse{
		Error: openAIErrorDetail{
			Message: message,
			Type:    errType,
			Param:   nil,
			Code:    nil,
		},
	})
}

func (h *Handler) authenticateGatewayKey(r *http.Request) (*database.GatewayKey, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, errors.New("invalid Authorization header format")
	}

	rawKey := strings.TrimSpace(parts[1])
	keyHash := auth.HashGatewayKey(rawKey)

	key, err := h.db.GetGatewayKeyByHash(keyHash)
	if err != nil {
		return nil, err
	}

	if key.Status != "active" {
		return nil, errors.New("gateway key is inactive")
	}

	return key, nil
}

type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
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
	w.WriteHeader(http.StatusOK)
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

	publicModel, err := h.db.GetPublicModel(reqBody.Model)
	if err != nil || !publicModel.Enabled {
		writeOpenAIError(w, http.StatusBadRequest, fmt.Sprintf("The model '%s' does not exist or is not enabled.", reqBody.Model), "invalid_request_error")
		return
	}

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

		attemptCtx := r.Context()
		var cancel context.CancelFunc
		if timeout := h.cfg.Timeouts.FirstResponseTimeout.Duration(); timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(r.Context(), timeout)
		} else {
			attemptCtx, cancel = context.WithCancel(r.Context())
		}

		chatResp, stats, err := adapter.ExecuteChat(attemptCtx, target.DecryptedSecret, target.UpstreamModelID, req)
		cancel()
		attemptDuration := time.Since(attemptStart)

		if err != nil {
			classification := adapter.ClassifyError(stats.HTTPStatus, err)
			lastErr = err

			var errCat *string
			if classification != providers.ErrorClassificationNone {
				cStr := string(classification)
				errCat = &cStr
			}

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
				writeOpenAIError(w, http.StatusBadRequest, "Invalid request parameters for upstream model.", "invalid_request_error")
				return
			}
			continue
		}

		var costMicro *int64
		if c, ok := accounting.CalculateAttemptCost(stats.InputTokens, stats.CachedInputTokens, stats.OutputTokens, target.InputRateSnapshot, target.CachedRateSnapshot, target.OutputRateSnapshot); ok {
			costMicro = &c
		}

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

		chatResp.ID = gatewayRequestID
		chatResp.Model = publicModel.ID
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResp)
		return
	}

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

		streamCtx, cancelStream := context.WithCancel(r.Context())
		var firstRespTimer *time.Timer
		if timeout := h.cfg.Timeouts.FirstResponseTimeout.Duration(); timeout > 0 {
			firstRespTimer = time.AfterFunc(timeout, func() {
				cancelStream()
			})
		}

		eventChan, err := adapter.StreamChat(streamCtx, target.DecryptedSecret, target.UpstreamModelID, req)
		if firstRespTimer != nil {
			firstRespTimer.Stop()
		}

		if err != nil {
			cancelStream()
			httpStatus := 0
			var httpErr *providers.HTTPStatusError
			if errors.As(err, &httpErr) {
				httpStatus = httpErr.StatusCode
			}
			classification := adapter.ClassifyError(httpStatus, err)
			if errors.Is(err, context.Canceled) && r.Context().Err() == nil {
				classification = providers.ErrorClassificationTimeout
			}
			lastErr = err

			var errCat *string
			if classification != providers.ErrorClassificationNone {
				cStr := string(classification)
				errCat = &cStr
			}

			attemptDuration := time.Since(attemptStart)
			var httpStatusPtr *int
			if httpStatus > 0 {
				httpStatusPtr = &httpStatus
			}
			_ = h.db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
				ID:                 attemptID,
				RequestID:          gatewayRequestID,
				ProviderID:         target.ProviderID,
				ProviderKeyID:      target.ProviderKeyID,
				MappingID:          target.MappingID,
				Sequence:           attemptSeq,
				Status:             "error",
				HTTPStatus:         httpStatusPtr,
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
				Stream:          true,
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
				writeOpenAIError(w, http.StatusBadRequest, "Invalid request parameters for upstream model.", "invalid_request_error")
				return
			}
			continue
		}

		if timeout := h.cfg.Timeouts.FirstResponseTimeout.Duration(); timeout > 0 {
			firstRespTimer = time.AfterFunc(timeout, func() {
				cancelStream()
			})
		}
		firstEv, ok := <-eventChan
		if firstRespTimer != nil {
			firstRespTimer.Stop()
		}

		if !ok || firstEv.Error != nil {
			cancelStream()
			var attemptErr error
			var httpStatus int
			if !ok {
				attemptErr = errors.New("upstream stream closed before emitting events")
				httpStatus = 0
			} else {
				attemptErr = firstEv.Error
				httpStatus = firstEv.HTTPStatus
			}

			classification := adapter.ClassifyError(httpStatus, attemptErr)
			if errors.Is(attemptErr, context.Canceled) && r.Context().Err() == nil {
				classification = providers.ErrorClassificationTimeout
			}
			lastErr = attemptErr

			var errCat *string
			if classification != providers.ErrorClassificationNone {
				cStr := string(classification)
				errCat = &cStr
			}

			attemptDuration := time.Since(attemptStart)
			var httpStatusPtr *int
			if httpStatus > 0 {
				httpStatusPtr = &httpStatus
			}
			_ = h.db.FinalizeAttemptAndRollup(database.RequestAttemptRecord{
				ID:                 attemptID,
				RequestID:          gatewayRequestID,
				ProviderID:         target.ProviderID,
				ProviderKeyID:      target.ProviderKeyID,
				MappingID:          target.MappingID,
				Sequence:           attemptSeq,
				Status:             "error",
				HTTPStatus:         httpStatusPtr,
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
				Stream:          true,
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
				writeOpenAIError(w, http.StatusBadRequest, "Invalid request parameters for upstream model.", "invalid_request_error")
				return
			}
			continue
		}

		defer cancelStream()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		var ttftMs *int64
		var finalUsage *providers.Usage
		var streamErr error

		if firstEv.IsFirst && firstEv.TTFT > 0 {
			ttft := firstEv.TTFT.Milliseconds()
			ttftMs = &ttft
		}
		if firstEv.Chunk != nil {
			firstEv.Chunk.ID = gatewayRequestID
			firstEv.Chunk.Model = publicModel.ID

			if firstEv.Chunk.Usage != nil {
				finalUsage = firstEv.Chunk.Usage
			}

			chunkBytes, err := json.Marshal(firstEv.Chunk)
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", string(chunkBytes))
				flusher.Flush()
			}
		}

		for ev := range eventChan {
			if ev.Error != nil {
				streamErr = ev.Error
				break
			}

			if ev.IsFirst && ev.TTFT > 0 && ttftMs == nil {
				ttft := ev.TTFT.Milliseconds()
				ttftMs = &ttft
			}

			if ev.Chunk != nil {
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

		if streamErr == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}

		attemptDuration := time.Since(attemptStart)
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

func generateID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
