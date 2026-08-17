package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GenericOpenAIAdapter struct {
	id               string
	name             string
	baseURL          string
	authHeaderPrefix string
	client           *http.Client
}

func NewGenericOpenAIAdapter(id, name, baseURL, authHeaderPrefix string) *GenericOpenAIAdapter {
	return &GenericOpenAIAdapter{
		id:               id,
		name:             name,
		baseURL:          strings.TrimRight(baseURL, "/"),
		authHeaderPrefix: authHeaderPrefix,
		client: &http.Client{
			Timeout: 0, // No blanket timeout; use context for request cancellation & streaming
		},
	}
}

func (a *GenericOpenAIAdapter) ID() string {
	return a.id
}

func (a *GenericOpenAIAdapter) Name() string {
	return a.name
}

type HTTPStatusError struct {
	StatusCode int
	Message    string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("upstream stream error (status %d): %s", e.StatusCode, e.Message)
}

func (a *GenericOpenAIAdapter) ClassifyError(statusCode int, err error) ErrorClassification {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorClassificationTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ErrorClassificationNone
	}

	if statusCode == 0 && err != nil {
		var httpErr *HTTPStatusError
		if errors.As(err, &httpErr) {
			statusCode = httpErr.StatusCode
		}
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorClassificationAuthInvalid
	case http.StatusTooManyRequests:
		return ErrorClassificationRateLimited
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ErrorClassificationTransient
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrorClassificationBadRequest
	default:
		if statusCode >= 500 {
			return ErrorClassificationTransient
		}
		if statusCode >= 400 && statusCode < 500 {
			return ErrorClassificationBadRequest
		}
		if statusCode == 0 && err != nil {
			return ErrorClassificationTransient
		}
		return ErrorClassificationNone
	}
}

func (a *GenericOpenAIAdapter) getEndpointURL() string {
	if strings.HasSuffix(a.baseURL, "/chat/completions") {
		return a.baseURL
	}
	return a.baseURL + "/chat/completions"
}

func (a *GenericOpenAIAdapter) ExecuteChat(ctx context.Context, key string, upstreamModel string, req *ChatRequest) (*ChatResponse, *AttemptStats, error) {
	startTime := time.Now()

	// Clone request and set upstream model
	reqClone := *req
	reqClone.Model = upstreamModel
	reqClone.Stream = false

	bodyBytes, err := json.Marshal(reqClone)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.getEndpointURL(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("creating http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", a.authHeaderPrefix+" "+key)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &AttemptStats{
			Duration:   time.Since(startTime),
			HTTPStatus: 0,
		}, err
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &AttemptStats{
			Duration:   duration,
			HTTPStatus: resp.StatusCode,
		}, fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, &AttemptStats{
			Duration:   duration,
			HTTPStatus: resp.StatusCode,
		}, fmt.Errorf("decoding response: %w", err)
	}

	stats := &AttemptStats{
		Duration:        duration,
		HTTPStatus:      resp.StatusCode,
		UsageConfidence: "unavailable",
	}

	if chatResp.Usage != nil {
		stats.InputTokens = chatResp.Usage.PromptTokens
		stats.OutputTokens = chatResp.Usage.CompletionTokens
		if chatResp.Usage.PromptTokensDetails != nil {
			stats.CachedInputTokens = chatResp.Usage.PromptTokensDetails.CachedTokens
		}
		stats.UsageConfidence = "provider_reported"
	}

	return &chatResp, stats, nil
}

func (a *GenericOpenAIAdapter) StreamChat(ctx context.Context, key string, upstreamModel string, req *ChatRequest) (<-chan StreamEvent, error) {
	reqClone := *req
	reqClone.Model = upstreamModel
	reqClone.Stream = true
	if reqClone.StreamOptions == nil {
		reqClone.StreamOptions = &StreamOptions{IncludeUsage: true}
	} else {
		reqClone.StreamOptions.IncludeUsage = true
	}

	bodyBytes, err := json.Marshal(reqClone)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.getEndpointURL(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", a.authHeaderPrefix+" "+key)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	eventChan := make(chan StreamEvent, 64)

	go func() {
		defer close(eventChan)
		defer resp.Body.Close()

		startTime := time.Now()
		var firstTokenSeen bool
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 10*1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				eventChan <- StreamEvent{Error: ctx.Err(), HTTPStatus: resp.StatusCode}
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			dataContent := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataContent == "[DONE]" {
				return
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
				// Skip non-json SSE lines or comments
				continue
			}

			var ttft time.Duration
			isFirst := false
			if !firstTokenSeen {
				hasContent := false
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" || len(c.Delta.ToolCalls) > 0 {
						hasContent = true
						break
					}
				}
				if hasContent {
					firstTokenSeen = true
					isFirst = true
					ttft = time.Since(startTime)
				}
			}

			select {
			case <-ctx.Done():
				return
			case eventChan <- StreamEvent{
				Chunk:      &chunk,
				HTTPStatus: resp.StatusCode,
				TTFT:       ttft,
				IsFirst:    isFirst,
			}:
			}
		}

		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			select {
			case <-ctx.Done():
				return
			case eventChan <- StreamEvent{Error: err, HTTPStatus: resp.StatusCode}:
			}
		}
	}()

	return eventChan, nil
}

// Built-in adapter factories
func NewFireworksAdapter() ProviderAdapter {
	return NewGenericOpenAIAdapter("fireworks", "Fireworks AI", "https://api.fireworks.ai/inference/v1", "Bearer")
}

func NewSiliconFlowAdapter() ProviderAdapter {
	return NewGenericOpenAIAdapter("siliconflow", "SiliconFlow", "https://api.siliconflow.cn/v1", "Bearer")
}

func NewNovitaAdapter() ProviderAdapter {
	return NewGenericOpenAIAdapter("novita", "Novita AI", "https://api.novita.ai/v3/openai", "Bearer")
}

func NewBasetenAdapter() ProviderAdapter {
	return NewGenericOpenAIAdapter("baseten", "Baseten", "https://bridge.baseten.co/v1", "Api-Key")
}
