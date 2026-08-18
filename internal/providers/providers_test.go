package providers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"arham-gateway/internal/providers"
)

func TestProviderAdapterExecuteChat(t *testing.T) {
	// Create a mock OpenAI-compatible upstream server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key-123" {
			http.Error(w, `{"error":{"message":"invalid api key"}}`, http.StatusUnauthorized)
			return
		}

		var req providers.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":{"message":"bad json"}}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "upstream-chatcmpl-999",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from mock upstream!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     15,
				"completion_tokens": 8,
				"total_tokens":      23,
				"prompt_tokens_details": map[string]any{
					"cached_tokens": 5,
				},
			},
		})
	}))
	defer server.Close()

	adapter := providers.NewGenericOpenAIAdapter("fireworks", server.URL, "Bearer", 0)

	req := &providers.ChatRequest{
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hello world"},
		},
	}

	resp, stats, err := adapter.ExecuteChat(context.Background(), "test-key-123", req.Model, req)
	if err != nil {
		t.Fatalf("unexpected error executing chat: %v", err)
	}

	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "Hello from mock upstream!" {
		t.Fatalf("unexpected response content: %+v", resp.Choices)
	}

	if stats.InputTokens != 15 || stats.CachedInputTokens != 5 || stats.OutputTokens != 8 {
		t.Errorf("unexpected usage stats: in=%d, cached=%d, out=%d", stats.InputTokens, stats.CachedInputTokens, stats.OutputTokens)
	}
	if stats.UsageConfidence != "provider_reported" {
		t.Errorf("expected provider_reported confidence, got %s", stats.UsageConfidence)
	}
}

func TestProviderAdapterStreamingChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		chunk1 := `{"id":"up-1","object":"chat.completion.chunk","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`
		chunk2 := `{"id":"up-1","object":"chat.completion.chunk","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`

		w.Write([]byte("data: " + chunk1 + "\n\n"))
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)

		w.Write([]byte("data: " + chunk2 + "\n\n"))
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)

		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	adapter := providers.NewGenericOpenAIAdapter("fireworks", server.URL, "Bearer", 0)

	req := &providers.ChatRequest{
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
		Stream: true,
	}

	events, err := adapter.StreamChat(context.Background(), "test-key-123", req.Model, req)
	if err != nil {
		t.Fatalf("unexpected stream start error: %v", err)
	}

	var text strings.Builder
	var finalUsage *providers.Usage
	for ev := range events {
		if ev.Error != nil {
			t.Fatalf("stream event error: %v", ev.Error)
		}
		if ev.Chunk != nil {
			for _, choice := range ev.Chunk.Choices {
				text.WriteString(choice.Delta.Content)
			}
			if ev.Chunk.Usage != nil {
				finalUsage = ev.Chunk.Usage
			}
		}
	}

	if text.String() != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", text.String())
	}
	if finalUsage == nil || finalUsage.CompletionTokens != 2 {
		t.Errorf("expected final usage completion_tokens=2, got %+v", finalUsage)
	}
}

func TestClassifyError(t *testing.T) {
	adapter := providers.NewGenericOpenAIAdapter("fireworks", "https://api.fireworks.ai", "Bearer", 0)

	if c := adapter.ClassifyError(401, nil); c != providers.ErrorClassificationAuthInvalid {
		t.Errorf("expected 401 to be AuthInvalid, got %v", c)
	}
	if c := adapter.ClassifyError(429, nil); c != providers.ErrorClassificationRateLimited {
		t.Errorf("expected 429 to be RateLimited, got %v", c)
	}
	if c := adapter.ClassifyError(503, nil); c != providers.ErrorClassificationTransient {
		t.Errorf("expected 503 to be Transient, got %v", c)
	}
	if c := adapter.ClassifyError(400, nil); c != providers.ErrorClassificationBadRequest {
		t.Errorf("expected 400 to be BadRequest (non-retryable), got %v", c)
	}
}

func TestProviderAdapterStreamingChatCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		for i := 0; i < 100; i++ {
			chunk := `{"id":"up-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"token"}}]}`
			_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	adapter := providers.NewGenericOpenAIAdapter("fireworks", server.URL, "Bearer", 0)

	req := &providers.ChatRequest{
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
		Stream: true,
	}

	events, err := adapter.StreamChat(ctx, "test-key-123", req.Model, req)
	if err != nil {
		t.Fatalf("unexpected stream start error: %v", err)
	}

	// Read one event and cancel context immediately
	<-events
	cancel()

	// Ensure channel closes within a reasonable timeout rather than blocking indefinitely
	done := make(chan struct{})
	go func() {
		for range events {
			// Drain remaining buffered events if any
		}
		close(done)
	}()

	select {
	case <-done:
		// Succeeded in exiting cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("StreamChat goroutine leaked or hung on context cancellation")
	}
}

func TestProviderAdapterStreamingChatUnexpectedEOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		chunk := `{"id":"up-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hi"}}]}`
		_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
		flusher.Flush()
		// Prematurely close response without sending data: [DONE]
	}))
	defer server.Close()

	adapter := providers.NewGenericOpenAIAdapter("fireworks", server.URL, "Bearer", 0)

	req := &providers.ChatRequest{
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
		Stream: true,
	}

	events, err := adapter.StreamChat(context.Background(), "test-key-123", req.Model, req)
	if err != nil {
		t.Fatalf("unexpected stream start error: %v", err)
	}

	var hasErr bool
	for ev := range events {
		if ev.Error != nil {
			hasErr = true
			break
		}
	}

	if !hasErr {
		t.Fatalf("expected stream to report error on unexpected EOF before [DONE]")
	}
}

func TestProviderAdapterDialTimeout(t *testing.T) {
	// Using a non-routable IP from RFC 5737 test range
	dialTimeout := 100 * time.Millisecond
	adapter := providers.NewGenericOpenAIAdapter("fireworks", "http://198.51.100.1:81", "Bearer", dialTimeout)

	req := &providers.ChatRequest{
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
	}

	start := time.Now()
	_, _, err := adapter.ExecuteChat(context.Background(), "test-key", req.Model, req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected connection to non-routable address to fail")
	}

	// Dial timeout was 100ms; should fail much quicker than default 30s
	if elapsed > 2*time.Second {
		t.Fatalf("expected dial timeout to bound connection attempt (took %v, expected < 2s)", elapsed)
	}
}

