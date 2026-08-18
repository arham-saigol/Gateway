package providers

import (
	"context"
	"time"
)

type ErrorClassification string

const (
	ErrorClassificationNone        ErrorClassification = "none"
	ErrorClassificationAuthInvalid ErrorClassification = "auth_invalid"
	ErrorClassificationRateLimited ErrorClassification = "rate_limited"
	ErrorClassificationTransient   ErrorClassification = "transient"
	ErrorClassificationBadRequest  ErrorClassification = "bad_request"
	ErrorClassificationTimeout     ErrorClassification = "timeout"
)

func (e ErrorClassification) IsRetryable() bool {
	switch e {
	case ErrorClassificationRateLimited, ErrorClassificationTransient, ErrorClassificationTimeout, ErrorClassificationAuthInvalid:
		return true
	default:
		return false
	}
}

type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolCall struct {
	Index    *int             `json:"index,omitempty"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function,omitempty"`
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type ResponseFormat struct {
	Type string `json:"type,omitempty"`
}

type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []ChatMessage   `json:"messages"`
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Stop                any             `json:"stop,omitempty"` // string or []string
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	Seed                *int64          `json:"seed,omitempty"`
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`
	Tools               []any           `json:"tools,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool           `json:"parallel_tool_calls,omitempty"`
}

type PromptTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
}

type Usage struct {
	PromptTokens        int64                `json:"prompt_tokens"`
	CompletionTokens    int64                `json:"completion_tokens"`
	TotalTokens         int64                `json:"total_tokens"`
	PromptTokensDetails *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type ChoiceMessage struct {
	Role      string     `json:"role"`
	Content   any        `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Choice struct {
	Index        int           `json:"index"`
	Message      ChoiceMessage `json:"message"`
	FinishReason *string       `json:"finish_reason"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type ChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
}

type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

type AttemptStats struct {
	Duration          time.Duration
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	UsageConfidence   string // provider_reported, gateway_estimated, unavailable
	HTTPStatus        int
}

type StreamEvent struct {
	Chunk      *ChatCompletionChunk
	Error      error
	HTTPStatus int
	IsFirst    bool
}

type ProviderAdapter interface {
	ID() string
	ExecuteChat(ctx context.Context, key string, upstreamModel string, req *ChatRequest) (*ChatResponse, *AttemptStats, error)
	StreamChat(ctx context.Context, key string, upstreamModel string, req *ChatRequest) (<-chan StreamEvent, error)
	ClassifyError(statusCode int, err error) ErrorClassification
}
