package database

import "time"

type Provider struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type ProviderKey struct {
	ID                     string     `json:"id"`
	ProviderID             string     `json:"provider_id"`
	EncryptedSecret        string     `json:"-"`
	DisplayName            string     `json:"display_name"`
	KeyPrefix              string     `json:"key_prefix"`
	StartingBalanceMicroUSD int64      `json:"starting_balance_micro_usd"`
	Status                 string     `json:"status"` // active, invalid, disabled, exhausted
	SafeLastError          *string    `json:"safe_last_error,omitempty"`
	LastUsedAt             *time.Time `json:"last_used_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type BalanceAdjustment struct {
	ID              string    `json:"id"`
	ProviderKeyID   string    `json:"provider_key_id"`
	AmountMicroUSD  int64     `json:"amount_micro_usd"`
	Note            string    `json:"note"`
	CreatedAt       time.Time `json:"created_at"`
}

type PublicModel struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProviderModelMapping struct {
	ID                   string    `json:"id"`
	ProviderID           string    `json:"provider_id"`
	PublicModelID        string    `json:"public_model_id"`
	UpstreamModelID      string    `json:"upstream_model_id"`
	SupportsStreaming    bool      `json:"supports_streaming"`
	SupportsTools        bool      `json:"supports_tools"`
	InputRatePerMTokens  int64     `json:"input_rate_per_m_tokens"`
	CachedRatePerMTokens int64     `json:"cached_rate_per_m_tokens"`
	OutputRatePerMTokens int64     `json:"output_rate_per_m_tokens"`
	Currency             string    `json:"currency"`
	Enabled              bool      `json:"enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type RoutingEntry struct {
	ID            string `json:"id"`
	PublicModelID string `json:"public_model_id"`
	MappingID     string `json:"mapping_id"`
	Priority      int    `json:"priority"`
	ProviderID    string `json:"provider_id"`
	UpstreamModelID string `json:"upstream_model_id"`
	Enabled       bool   `json:"enabled"`
}

type GatewayKey struct {
	ID         string     `json:"id"`
	KeyHash    string     `json:"-"`
	KeyPrefix  string     `json:"key_prefix"`
	Name       string     `json:"name"`
	Status     string     `json:"status"` // active, disabled, revoked
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type RequestRecord struct {
	ID                string     `json:"id"`
	PublicModelID     string     `json:"public_model_id"`
	GatewayKeyID      *string    `json:"gateway_key_id,omitempty"`
	Status            string     `json:"status"` // success, error, canceled
	ErrorCategory     *string    `json:"error_category,omitempty"`
	Stream            bool       `json:"stream"`
	TTFTMs            *int64     `json:"ttft_ms,omitempty"`
	TotalDurationMs   int64      `json:"total_duration_ms"`
	InputTokens       *int64     `json:"input_tokens,omitempty"`
	CachedInputTokens *int64     `json:"cached_input_tokens,omitempty"`
	OutputTokens      *int64     `json:"output_tokens,omitempty"`
	TotalCostMicroUSD *int64     `json:"total_cost_micro_usd,omitempty"`
	UsageConfidence   string     `json:"usage_confidence"`
	RetryCount        int        `json:"retry_count"`
	FailoverCount     int        `json:"failover_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

type RequestAttemptRecord struct {
	ID                 string    `json:"id"`
	RequestID          string    `json:"request_id"`
	ProviderID         string    `json:"provider_id"`
	ProviderKeyID      string    `json:"provider_key_id"`
	MappingID          string    `json:"mapping_id"`
	Sequence           int       `json:"sequence"`
	Status             string    `json:"status"`
	HTTPStatus         *int      `json:"http_status,omitempty"`
	ErrorCategory      *string   `json:"error_category,omitempty"`
	TTFTMs             *int64    `json:"ttft_ms,omitempty"`
	DurationMs         int64     `json:"duration_ms"`
	InputTokens        *int64    `json:"input_tokens,omitempty"`
	CachedInputTokens  *int64    `json:"cached_input_tokens,omitempty"`
	OutputTokens       *int64    `json:"output_tokens,omitempty"`
	InputRateSnapshot  int64     `json:"input_rate_snapshot"`
	CachedRateSnapshot int64     `json:"cached_rate_snapshot"`
	OutputRateSnapshot int64     `json:"output_rate_snapshot"`
	TotalCostMicroUSD  *int64    `json:"total_cost_micro_usd,omitempty"`
	UsageConfidence    string    `json:"usage_confidence"`
	AggregatedInRollup bool      `json:"aggregated_in_rollup"`
	CreatedAt          time.Time `json:"created_at"`
}

type KeyBalanceSummary struct {
	ProviderKeyID          string  `json:"provider_key_id"`
	ProviderID             string  `json:"provider_id"`
	DisplayName            string  `json:"display_name"`
	KeyPrefix              string  `json:"key_prefix"`
	StartingBalanceMicroUSD int64   `json:"starting_balance_micro_usd"`
	AdjustmentsMicroUSD    int64   `json:"adjustments_micro_usd"`
	KnownSpendMicroUSD     int64   `json:"known_spend_micro_usd"`
	EstimatedRemainingMicroUSD int64 `json:"estimated_remaining_micro_usd"`
	Status                 string  `json:"status"`
}
