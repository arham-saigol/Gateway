export interface ProviderKey {
  id: string;
  key_prefix: string;
  starting_balance_micro_usd: number;
  status: 'active' | 'disabled' | 'invalid' | 'exhausted';
  safe_last_error?: string;
  last_used_at?: string;
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
  upstream_model_id: string;
  input_rate_per_m_tokens: number;
  cached_rate_per_m_tokens: number;
  output_rate_per_m_tokens: number;
}

export interface RoutingEntry {
  id: string;
  mapping_id: string;
  provider_id: string;
  upstream_model_id: string;
}

export interface PublicModelWithRoutes {
  id: string;
  display_name: string;
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
}

export interface RequestRecord {
  id: string;
  public_model_id: string;
  status: 'success' | 'error' | 'canceled';
  stream: boolean;
  ttft_ms?: number;
  total_duration_ms: number;
  input_tokens?: number;
  output_tokens?: number;
  total_cost_micro_usd?: number;
  retry_count: number;
  created_at: string;
}

export interface RequestAttemptRecord {
  id: string;
  provider_id: string;
  sequence: number;
  status: string;
  http_status?: number;
  error_category?: string;
  ttft_ms?: number;
  duration_ms: number;
  input_tokens?: number;
  cached_input_tokens?: number;
  output_tokens?: number;
  total_cost_micro_usd?: number;
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
