-- Schema migration 001: Initial schema for Arham Gateway

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_credentials (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    token_hash TEXT PRIMARY KEY,
    csrf_token TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_keys (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES providers(id),
    encrypted_secret TEXT NOT NULL,
    display_name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    starting_balance_micro_usd INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    safe_last_error TEXT,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS balance_adjustments (
    id TEXT PRIMARY KEY,
    provider_key_id TEXT NOT NULL REFERENCES provider_keys(id),
    amount_micro_usd INTEGER NOT NULL,
    note TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS public_models (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_model_mappings (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES providers(id),
    public_model_id TEXT NOT NULL REFERENCES public_models(id),
    upstream_model_id TEXT NOT NULL,
    supports_streaming INTEGER NOT NULL DEFAULT 1,
    supports_tools INTEGER NOT NULL DEFAULT 1,
    input_rate_per_m_tokens INTEGER NOT NULL,
    cached_rate_per_m_tokens INTEGER NOT NULL,
    output_rate_per_m_tokens INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS routing_entries (
    id TEXT PRIMARY KEY,
    public_model_id TEXT NOT NULL REFERENCES public_models(id),
    mapping_id TEXT NOT NULL REFERENCES provider_model_mappings(id),
    priority INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL,
    UNIQUE(public_model_id, mapping_id),
    UNIQUE(public_model_id, priority)
);

CREATE TABLE IF NOT EXISTS gateway_keys (
    id TEXT PRIMARY KEY,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    revoked_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    public_model_id TEXT NOT NULL,
    gateway_key_id TEXT REFERENCES gateway_keys(id),
    status TEXT NOT NULL,
    error_category TEXT,
    stream INTEGER NOT NULL,
    ttft_ms INTEGER,
    total_duration_ms INTEGER NOT NULL,
    input_tokens INTEGER,
    cached_input_tokens INTEGER,
    output_tokens INTEGER,
    total_cost_micro_usd INTEGER,
    usage_confidence TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    failover_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS request_attempts (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    provider_id TEXT NOT NULL,
    provider_key_id TEXT NOT NULL REFERENCES provider_keys(id),
    mapping_id TEXT NOT NULL REFERENCES provider_model_mappings(id),
    sequence INTEGER NOT NULL,
    status TEXT NOT NULL,
    http_status INTEGER,
    error_category TEXT,
    ttft_ms INTEGER,
    duration_ms INTEGER NOT NULL,
    input_tokens INTEGER,
    cached_input_tokens INTEGER,
    output_tokens INTEGER,
    input_rate_snapshot INTEGER NOT NULL,
    cached_rate_snapshot INTEGER NOT NULL,
    output_rate_snapshot INTEGER NOT NULL,
    total_cost_micro_usd INTEGER,
    usage_confidence TEXT NOT NULL,
    aggregated_in_rollup INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS usage_rollups_daily (
    id TEXT PRIMARY KEY,
    date_utc TEXT NOT NULL,
    provider_key_id TEXT NOT NULL REFERENCES provider_keys(id),
    mapping_id TEXT NOT NULL REFERENCES provider_model_mappings(id),
    public_model_id TEXT NOT NULL,
    gateway_key_id TEXT,
    total_requests INTEGER NOT NULL DEFAULT 0,
    successful_requests INTEGER NOT NULL DEFAULT 0,
    failed_requests INTEGER NOT NULL DEFAULT 0,
    retries INTEGER NOT NULL DEFAULT 0,
    failovers INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    cached_input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    known_cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    unknown_cost_attempts INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE(date_utc, provider_key_id, mapping_id, gateway_key_id)
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_requests_public_model ON requests(public_model_id);
CREATE INDEX IF NOT EXISTS idx_requests_gateway_key ON requests(gateway_key_id);
CREATE INDEX IF NOT EXISTS idx_request_attempts_request_id ON request_attempts(request_id);
CREATE INDEX IF NOT EXISTS idx_request_attempts_provider_key ON request_attempts(provider_key_id);
CREATE INDEX IF NOT EXISTS idx_usage_rollups_date ON usage_rollups_daily(date_utc);
CREATE INDEX IF NOT EXISTS idx_usage_rollups_key ON usage_rollups_daily(provider_key_id);
