export interface ProviderKey {
  id: string;
  provider_id: string;
  display_name: string;
  key_prefix: string;
  starting_balance_micro_usd: number;
  status: 'active' | 'disabled' | 'invalid' | 'exhausted';
  safe_last_error?: string;
  last_used_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Provider {
  id: string;
  name: string;
  enabled: boolean;
  keys: ProviderKey[];
}

export interface KeyBalanceSummary {
  provider_key_id: string;
  provider_id: string;
  display_name: string;
  key_prefix: string;
  starting_balance_micro_usd: number;
  adjustments_micro_usd: number;
  known_spend_micro_usd: number;
  estimated_remaining_micro_usd: number;
  status: string;
}

export interface ProviderModelMapping {
  id: string;
  provider_id: string;
  public_model_id: string;
  upstream_model_id: string;
  supports_streaming: boolean;
  supports_tools: boolean;
  input_rate_per_m_tokens: number;
  cached_rate_per_m_tokens: number;
  output_rate_per_m_tokens: number;
  currency: string;
  enabled: boolean;
}

export interface RoutingEntry {
  id: string;
  public_model_id: string;
  mapping_id: string;
  priority: number;
  provider_id: string;
  upstream_model_id: string;
  enabled: boolean;
}

export interface PublicModelWithRoutes {
  id: string;
  display_name: string;
  description: string;
  enabled: boolean;
  routes: RoutingEntry[];
  mappings: ProviderModelMapping[];
}

export interface GatewayKey {
  id: string;
  key_prefix: string;
  name: string;
  status: 'active' | 'disabled' | 'revoked';
  created_at: string;
  last_used_at?: string;
  revoked_at?: string;
}

export interface RequestRecord {
  id: string;
  public_model_id: string;
  gateway_key_id?: string;
  status: 'success' | 'error' | 'canceled';
  error_category?: string;
  stream: boolean;
  ttft_ms?: number;
  total_duration_ms: number;
  input_tokens?: number;
  cached_input_tokens?: number;
  output_tokens?: number;
  total_cost_micro_usd?: number;
  usage_confidence: string;
  retry_count: number;
  failover_count: number;
  created_at: string;
}

export interface RequestAttemptRecord {
  id: string;
  request_id: string;
  provider_id: string;
  provider_key_id: string;
  mapping_id: string;
  sequence: number;
  status: string;
  http_status?: number;
  error_category?: string;
  ttft_ms?: number;
  duration_ms: number;
  input_tokens?: number;
  cached_input_tokens?: number;
  output_tokens?: number;
  input_rate_snapshot: number;
  cached_rate_snapshot: number;
  output_rate_snapshot: number;
  total_cost_micro_usd?: number;
  usage_confidence: string;
  created_at: string;
}

export interface DailyStat {
  date_utc: string;
  public_model_id: string;
  total_requests: number;
  success_requests: number;
  failed_requests: number;
  input_tokens: number;
  cached_input_tokens: number;
  output_tokens: number;
  cost_micro_usd: number;
}
