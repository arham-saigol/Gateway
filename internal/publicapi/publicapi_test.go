package publicapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arham-gateway/internal/auth"
	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
	"arham-gateway/internal/providers"
	"arham-gateway/internal/publicapi"
	"arham-gateway/internal/routing"
)

func setupTestEnvironment(t *testing.T) (*database.DB, []byte, string, *routing.Router, http.Handler) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gateway.db")

	db, err := database.Open(dbPath, 5000)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	_ = db.Migrate()
	_ = db.SeedDefaults()

	masterKey, _ := crypto.GenerateRandomBytes(32)

	// Create a Gateway API Key
	rawKey, prefix, hash, _ := auth.GenerateGatewayKey("test-client")
	_ = db.CreateGatewayKey(database.GatewayKey{
		ID:        "gw-key-1",
		KeyHash:   hash,
		KeyPrefix: prefix,
		Name:      "test-client",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	})

	cfg := config.DefaultConfig()
	router := routing.NewRouter(db, masterKey, &cfg)

	handler := publicapi.NewHandler(db, router, &cfg)
	return db, masterKey, rawKey, router, handler
}

func TestHealthAndReadiness(t *testing.T) {
	_, _, _, _, handler := setupTestEnvironment(t)

	// GET /healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected /healthz to return 200, got %d", rec.Code)
	}

	// GET /readyz
	reqReady := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recReady := httptest.NewRecorder()
	handler.ServeHTTP(recReady, reqReady)

	if recReady.Code != http.StatusOK {
		t.Errorf("expected /readyz to return 200, got %d", recReady.Code)
	}
}

func TestListModelsOnlyExposesPublicAliases(t *testing.T) {
	_, _, rawKey, _, handler := setupTestEnvironment(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected exactly 2 models, got %d", len(resp.Data))
	}

	for _, m := range resp.Data {
		if m.ID != "deepseek-v4-flash" && m.ID != "deepseek-v4-flash-fast" {
			t.Errorf("leaked non-public model ID: %s", m.ID)
		}
		if m.OwnedBy != "arham" {
			t.Errorf("expected owned_by 'arham', got '%s'", m.OwnedBy)
		}
	}
}

func TestChatCompletionsAuthenticationAndValidation(t *testing.T) {
	_, _, rawKey, _, handler := setupTestEnvironment(t)

	// 1. Missing auth
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on missing auth, got %d", rec.Code)
	}

	// 2. Reject upstream model ID directly
	reqBadModel := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"model":"accounts/fireworks/models/deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)))
	reqBadModel.Header.Set("Authorization", "Bearer "+rawKey)
	recBadModel := httptest.NewRecorder()
	handler.ServeHTTP(recBadModel, reqBadModel)
	if recBadModel.Code != http.StatusBadRequest && recBadModel.Code != http.StatusNotFound {
		t.Errorf("expected 400/404 for upstream model ID, got %d", recBadModel.Code)
	}

	// 3. Reject conflicting max_tokens and max_completion_tokens
	maxTokens := 100
	maxCompletion := 100
	conflictBody, _ := json.Marshal(map[string]any{
		"model":                 "deepseek-v4-flash",
		"messages":              []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens":            maxTokens,
		"max_completion_tokens": maxCompletion,
	})
	reqConflict := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(conflictBody))
	reqConflict.Header.Set("Authorization", "Bearer "+rawKey)
	recConflict := httptest.NewRecorder()
	handler.ServeHTTP(recConflict, reqConflict)
	if recConflict.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when both max_tokens and max_completion_tokens are provided, got %d", recConflict.Code)
	}
}

func TestChatCompletionsNonStreamingExecution(t *testing.T) {
	db, masterKey, rawKey, router, handler := setupTestEnvironment(t)

	// Create mock upstream
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "up-secret-id",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "accounts/fireworks/models/deepseek-v4-flash",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Normalized assistant response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     20,
				"completion_tokens": 10,
				"total_tokens":      30,
			},
		})
	}))
	defer mockUpstream.Close()

	// Register custom adapter for fireworks pointing to mockUpstream
	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockUpstream.URL, "Bearer", 0))

	// Add a fireworks key to DB
	encSecret, _ := crypto.Encrypt(masterKey, "mock-fw-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-fw-mock",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "Fireworks Mock",
		KeyPrefix:               "fw_mock...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Hello!"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Preserves public model alias
	if resp.Model != "deepseek-v4-flash" {
		t.Errorf("expected model 'deepseek-v4-flash', got '%s'", resp.Model)
	}
	// Gateway-owned ID prefix
	if !strings.HasPrefix(resp.ID, "chatcmpl-arham-") {
		t.Errorf("expected gateway-owned ID prefix chatcmpl-arham-, got '%s'", resp.ID)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "Normalized assistant response" {
		t.Errorf("unexpected content: %+v", resp.Choices)
	}
	if resp.Usage.PromptTokens != 20 || resp.Usage.CompletionTokens != 10 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}

	// Verify last_used_at was updated on gateway key and provider key
	gwKeys, err := db.ListGatewayKeys()
	if err != nil || len(gwKeys) == 0 || gwKeys[0].LastUsedAt == nil {
		t.Errorf("expected gateway key last_used_at to be updated, got %+v", gwKeys)
	}
	pKeys, err := db.ListProviderKeys("fireworks")
	if err != nil || len(pKeys) == 0 || pKeys[0].LastUsedAt == nil {
		t.Errorf("expected provider key last_used_at to be updated, got %+v", pKeys)
	}
}

func TestChatCompletionsStreamingExecution(t *testing.T) {
	db, masterKey, rawKey, router, handler := setupTestEnvironment(t)

	// Create mock streaming upstream
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		w.Write([]byte("data: {\"id\":\"up-chunk-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Streamed\"},\"finish_reason\":null}]}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: {\"id\":\"up-chunk-2\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" reply\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":2,\"total_tokens\":14}}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer mockUpstream.Close()

	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockUpstream.URL, "Bearer", 0))

	encSecret, _ := crypto.Encrypt(masterKey, "mock-fw-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-fw-mock-stream",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "Fireworks Stream Mock",
		KeyPrefix:               "fw_mock...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Stream me"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	scanner := bufio.NewScanner(rec.Body)
	var chunks []string
	var seenDone bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				seenDone = true
			} else {
				chunks = append(chunks, data)
			}
		}
	}

	if !seenDone {
		t.Errorf("streaming response missing [DONE]")
	}
	if len(chunks) == 0 {
		t.Fatalf("no chunks received")
	}

	// Verify all chunks have same gateway-owned completion ID
	var commonID string
	for _, raw := range chunks {
		var chunk map[string]any
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			t.Fatalf("invalid chunk json: %v", err)
		}
		id := chunk["id"].(string)
		if !strings.HasPrefix(id, "chatcmpl-arham-") {
			t.Errorf("chunk leaked non-gateway ID: %s", id)
		}
		if commonID == "" {
			commonID = id
		} else if id != commonID {
			t.Errorf("chunk ID changed across stream: was %s, now %s", commonID, id)
		}
		if chunk["model"].(string) != "deepseek-v4-flash" {
			t.Errorf("chunk leaked model ID: %v", chunk["model"])
		}
	}

	// Verify last_used_at was updated on gateway key and provider key
	gwKeys, err := db.ListGatewayKeys()
	if err != nil || len(gwKeys) == 0 || gwKeys[0].LastUsedAt == nil {
		t.Errorf("expected gateway key last_used_at to be updated for stream, got %+v", gwKeys)
	}
	pKeys, err := db.ListProviderKeys("fireworks")
	if err != nil || len(pKeys) == 0 || pKeys[0].LastUsedAt == nil {
		t.Errorf("expected provider key last_used_at to be updated for stream, got %+v", pKeys)
	}
}

func TestChatCompletionsNullAndMultimodalContent(t *testing.T) {
	db, masterKey, rawKey, router, handler := setupTestEnvironment(t)

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "up-secret-id",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "accounts/fireworks/models/deepseek-v4-flash",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Received complex content successfully",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     15,
				"completion_tokens": 5,
				"total_tokens":      20,
			},
		})
	}))
	defer mockUpstream.Close()

	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockUpstream.URL, "Bearer", 0))

	encSecret, _ := crypto.Encrypt(masterKey, "mock-fw-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-fw-mock-complex",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "Fireworks Complex Mock",
		KeyPrefix:               "fw_mock...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	// Test body with content: null (tool call message) and array content (multimodal)
	body := `{
		"model": "deepseek-v4-flash",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Describe image"}]},
			{"role": "assistant", "content": null, "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "lookup", "arguments": "{}"}}]},
			{"role": "tool", "tool_call_id": "call_1", "content": "Result"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for null/multimodal content, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStreamingErrorDoesNotEmitDone(t *testing.T) {
	db, masterKey, rawKey, router, handler := setupTestEnvironment(t)

	// Mock upstream that sends a chunk then breaks with non-200/malformed stream
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		w.Write([]byte("data: {\"id\":\"up-chunk-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Chunk 1\"},\"finish_reason\":null}]}\n\n"))
		flusher.Flush()

		// Hijack / close connection abruptly to simulate upstream error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer mockUpstream.Close()

	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockUpstream.URL, "Bearer", 0))

	encSecret, _ := crypto.Encrypt(masterKey, "mock-fw-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-fw-mock-stream-err",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "Fireworks Stream Err Mock",
		KeyPrefix:               "fw_mock...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Stream me"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Since connection died midway, [DONE] should not be present
	out := rec.Body.String()
	if strings.Contains(out, "data: [DONE]") {
		t.Errorf("expected stream with upstream error NOT to emit [DONE], got: %s", out)
	}
}

func TestStreamingFailoverOnInitialStreamError(t *testing.T) {
	db, masterKey, rawKey, router, handler := setupTestEnvironment(t)

	// Provider 1 (fireworks) returns HTTP 500 error on stream initiation
	mockFw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fireworks internal error"}`))
	}))
	defer mockFw.Close()

	// Provider 2 (siliconflow) returns valid stream
	mockSf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"id\":\"sf-chunk-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Failover succeeded\"},\"finish_reason\":\"stop\"}]}\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer mockSf.Close()

	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockFw.URL, "Bearer", 0))
	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("siliconflow", "SiliconFlow", mockSf.URL, "Bearer", 0))

	// Create fireworks key (pri 1) and siliconflow key (pri 2)
	encFw, _ := crypto.Encrypt(masterKey, "mock-fw-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-fw-fail",
		ProviderID:              "fireworks",
		EncryptedSecret:         encFw,
		DisplayName:             "Fireworks Failing",
		KeyPrefix:               "fw_fail...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	encSf, _ := crypto.Encrypt(masterKey, "mock-sf-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-sf-ok",
		ProviderID:              "siliconflow",
		EncryptedSecret:         encSf,
		DisplayName:             "SiliconFlow Working",
		KeyPrefix:               "sf_ok...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on failover, got %d: %s", rec.Code, rec.Body.String())
	}

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "Failover succeeded") {
		t.Errorf("expected stream to contain content from provider 2, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Errorf("expected stream to contain [DONE], got: %s", bodyStr)
	}
}

func TestFirstResponseTimeoutEnforced(t *testing.T) {
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
	rawKey, prefix, hash, _ := auth.GenerateGatewayKey("test-client")
	_ = db.CreateGatewayKey(database.GatewayKey{
		ID:        "gw-key-timeout",
		KeyHash:   hash,
		KeyPrefix: prefix,
		Name:      "test-client",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	})

	// Configure a tight 100ms first response timeout
	cfg := config.DefaultConfig()
	cfg.Timeouts.FirstResponseTimeout = config.Duration(100 * time.Millisecond)

	// Provider 1 hangs for 500ms (exceeding 100ms timeout)
	mockHanging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer mockHanging.Close()

	// Provider 2 responds immediately
	mockFast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "up-fast-id",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "accounts/siliconflow/models/deepseek-v4-flash",
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "Fast provider response"},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer mockFast.Close()

	router := routing.NewRouter(db, masterKey, &cfg)
	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockHanging.URL, "Bearer", 0))
	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("siliconflow", "SiliconFlow", mockFast.URL, "Bearer", 0))

	encFw, _ := crypto.Encrypt(masterKey, "mock-fw-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-fw-hang",
		ProviderID:              "fireworks",
		EncryptedSecret:         encFw,
		DisplayName:             "Fireworks Hanging",
		KeyPrefix:               "fw_hang...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	encSf, _ := crypto.Encrypt(masterKey, "mock-sf-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-sf-fast",
		ProviderID:              "siliconflow",
		EncryptedSecret:         encSf,
		DisplayName:             "SiliconFlow Fast",
		KeyPrefix:               "sf_fast...",
		StartingBalanceMicroUSD: 6000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	handler := publicapi.NewHandler(db, router, &cfg)

	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Timeout test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after failover from hanging provider, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "Fast provider response") {
		t.Errorf("expected response from fast provider, got: %s", rec.Body.String())
	}
}

func TestChatCompletionClientCancellation(t *testing.T) {
	db, masterKey, rawKey, _, _ := setupTestEnvironment(t)
	cfg := config.DefaultConfig()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":123,"model":"qwen-2.5-72b-instruct","choices":[{"index":0,"message":{"role":"assistant","content":"Response"}}]}`))
	}))
	defer mockServer.Close()

	router := routing.NewRouter(db, masterKey, &cfg)
	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockServer.URL, "Bearer", 0))

	encSecret, _ := crypto.Encrypt(masterKey, "mock-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-cancel-test",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "Test Key",
		KeyPrefix:               "fw_test...",
		StartingBalanceMicroUSD: 5000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	handler := publicapi.NewHandler(db, router, &cfg)

	ctx, cancel := context.WithCancel(context.Background())
	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Cancel test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body))).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	// Cancel context quickly while request is processing
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	handler.ServeHTTP(rec, req)

	// Verify request was recorded as canceled
	var recordedStatus string
	err := db.Conn().QueryRow("SELECT status FROM requests WHERE public_model_id = 'deepseek-v4-flash' ORDER BY created_at DESC LIMIT 1").Scan(&recordedStatus)
	if err != nil {
		t.Fatalf("failed to query request record: %v", err)
	}
	if recordedStatus != "canceled" {
		t.Errorf("expected request status 'canceled', got %s", recordedStatus)
	}
}

func TestStreamingChatCompletionClientCancellation(t *testing.T) {
	db, masterKey, rawKey, _, _ := setupTestEnvironment(t)
	cfg := config.DefaultConfig()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 5; i++ {
			_, _ = fmt.Fprintf(w, "data: {\"id\":\"chunk-%d\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"chunk \"}}]}\n\n", i)
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	router := routing.NewRouter(db, masterKey, &cfg)
	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockServer.URL, "Bearer", 0))

	encSecret, _ := crypto.Encrypt(masterKey, "mock-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-stream-cancel",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "Test Key",
		KeyPrefix:               "fw_stream...",
		StartingBalanceMicroUSD: 5000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	handler := publicapi.NewHandler(db, router, &cfg)

	ctx, cancel := context.WithCancel(context.Background())
	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"Stream cancel test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body))).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	// Cancel stream after receiving first chunk
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	handler.ServeHTTP(rec, req)

	var recordedStatus string
	err := db.Conn().QueryRow("SELECT status FROM requests WHERE public_model_id = 'deepseek-v4-flash' ORDER BY created_at DESC LIMIT 1").Scan(&recordedStatus)
	if err != nil {
		t.Fatalf("failed to query streaming request record: %v", err)
	}
	if recordedStatus != "canceled" {
		t.Errorf("expected streaming request status 'canceled', got %s", recordedStatus)
	}
}

func TestStreamingTTFTMeasuredFromRequestStart(t *testing.T) {
	db, masterKey, rawKey, router, handler := setupTestEnvironment(t)

	// Server simulates network / header delay of 50ms before returning first chunk
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		_, _ = fmt.Fprintf(w, "data: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"First token\"}}]}\n\n")
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	router.RegisterAdapter(providers.NewGenericOpenAIAdapter("fireworks", "Fireworks AI", mockServer.URL, "Bearer", 0))

	encSecret, _ := crypto.Encrypt(masterKey, "mock-key")
	_ = db.CreateProviderKey(database.ProviderKey{
		ID:                      "key-ttft-test",
		ProviderID:              "fireworks",
		EncryptedSecret:         encSecret,
		DisplayName:             "TTFT Key",
		KeyPrefix:               "fw_ttft...",
		StartingBalanceMicroUSD: 5000000,
		Status:                  "active",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	})

	body := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"TTFT test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var reqTTFT *int64
	var attTTFT *int64
	err := db.Conn().QueryRow("SELECT r.ttft_ms, a.ttft_ms FROM requests r JOIN request_attempts a ON r.id = a.request_id WHERE r.public_model_id = 'deepseek-v4-flash' ORDER BY r.created_at DESC LIMIT 1").Scan(&reqTTFT, &attTTFT)
	if err != nil {
		t.Fatalf("failed to query TTFT: %v", err)
	}

	if reqTTFT == nil || *reqTTFT < 30 {
		t.Errorf("expected request TTFT to be at least ~50ms (measured from request arrival), got %v", reqTTFT)
	}
	if attTTFT == nil || *attTTFT < 30 {
		t.Errorf("expected attempt TTFT to be at least ~50ms (measured from attempt start), got %v", attTTFT)
	}
}

