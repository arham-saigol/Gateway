package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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

func NewGenericOpenAIAdapter(id, name, baseURL, authHeaderPrefix string, dialTimeout time.Duration) *GenericOpenAIAdapter {
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &GenericOpenAIAdapter{
		id:               id,
		name:             name,
		baseURL:          strings.TrimRight(baseURL, "/"),
		authHeaderPrefix: authHeaderPrefix,
		client: &http.Client{
			Transport: transport,
			Timeout:   0, // No blanket timeout; use context for request cancellation & streaming
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

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return ErrorClassificationAuthInvalid
	case statusCode == http.StatusTooManyRequests:
		return ErrorClassificationRateLimited
	case statusCode >= 500:
		return ErrorClassificationTransient
	case statusCode >= 400:
		return ErrorClassificationBadRequest
	case statusCode == 0 && err != nil:
		return ErrorClassificationTransient
	default:
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
	streamStart := time.Now()

	reqClone := *req
	reqClone.Model = upstreamModel
	reqClone.Stream = true
	reqClone.StreamOptions = &StreamOptions{IncludeUsage: true}

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

		var firstTokenSeen bool
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 10*1024*1024)

		var doneSeen bool
		for scanner.Scan() {
			select {
			case <-ctx.Done():
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
				doneSeen = true
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
					ttft = time.Since(streamStart)
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
			return
		}

		if !doneSeen && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case eventChan <- StreamEvent{Error: io.ErrUnexpectedEOF, HTTPStatus: resp.StatusCode}:
			}
		}
	}()

	return eventChan, nil
}

// Built-in adapter factories
func NewFireworksAdapter(dialTimeout time.Duration) ProviderAdapter {
	return NewGenericOpenAIAdapter("fireworks", "Fireworks AI", "https://api.fireworks.ai/inference/v1", "Bearer", dialTimeout)
}

func NewSiliconFlowAdapter(dialTimeout time.Duration) ProviderAdapter {
	return NewGenericOpenAIAdapter("siliconflow", "SiliconFlow", "https://api.siliconflow.cn/v1", "Bearer", dialTimeout)
}

func NewNovitaAdapter(dialTimeout time.Duration) ProviderAdapter {
	return NewGenericOpenAIAdapter("novita", "Novita AI", "https://api.novita.ai/v3/openai", "Bearer", dialTimeout)
}

func NewBasetenAdapter(dialTimeout time.Duration) ProviderAdapter {
	return NewGenericOpenAIAdapter("baseten", "Baseten", "https://bridge.baseten.co/v1", "Api-Key", dialTimeout)
}
