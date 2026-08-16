import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { KeyBalanceSummary } from '../types';

interface OverviewData {
  total_spend_micro_usd: number;
  total_remaining_micro_usd: number;
  requests_24h: number;
  success_rate_24h: number;
  avg_latency_ms_24h: number;
  key_balances: KeyBalanceSummary[];
}

export function OverviewView() {
  const [data, setData] = useState<OverviewData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    loadOverview();
  }, []);

  async function loadOverview() {
    try {
      setLoading(true);
      const res = await apiRequest<OverviewData>('/api/overview');
      setData(res);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load overview data');
    } finally {
      setLoading(false);
    }
  }

  if (loading && !data) {
    return <div style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>Loading gateway metrics...</div>;
  }

  if (error) {
    return <div style={{ padding: '20px', color: 'var(--danger)' }}>{error}</div>;
  }

  return (
    <div>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-label">24h Requests</div>
          <div class="stat-value">{data?.requests_24h ?? 0}</div>
          <div class="stat-sub">Across all public models</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">24h Success Rate</div>
          <div class="stat-value" style={{ color: (data?.success_rate_24h ?? 100) >= 99 ? 'var(--success)' : 'var(--warning)' }}>
            {(data?.success_rate_24h ?? 100).toFixed(1)}%
          </div>
          <div class="stat-sub">Completed without unhandled error</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">24h Avg Latency</div>
          <div class="stat-value">{(data?.avg_latency_ms_24h ?? 0).toFixed(0)} ms</div>
          <div class="stat-sub">End-to-end request duration</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Total Known Spend</div>
          <div class="stat-value">{formatUSD(data?.total_spend_micro_usd)}</div>
          <div class="stat-sub">Exact calculated token spend</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Estimated Remaining</div>
          <div class="stat-value" style={{ color: (data?.total_remaining_micro_usd ?? 0) > 0 ? 'var(--text-primary)' : 'var(--danger)' }}>
            {formatUSD(data?.total_remaining_micro_usd)}
          </div>
          <div class="stat-sub">Starting + adjustments - spend</div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div class="panel-title">Provider Accounts & Balances</div>
          <button class="btn btn-secondary btn-sm" onClick={loadOverview} disabled={loading}>
            {loading ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Provider</th>
                <th>Key Display Name</th>
                <th>Prefix</th>
                <th>Starting Balance</th>
                <th>Adjustments</th>
                <th>Gateway Spend</th>
                <th>Estimated Remaining</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {(!data?.key_balances || data.key_balances.length === 0) ? (
                <tr>
                  <td colSpan={8} style={{ textAlign: 'center', padding: '32px', color: 'var(--text-muted)' }}>
                    No provider keys configured yet. Go to Providers to add an upstream key.
                  </td>
                </tr>
              ) : (
                data.key_balances.map(k => (
                  <tr key={k.provider_key_id}>
                    <td style={{ fontWeight: 600, textTransform: 'capitalize', color: 'var(--text-primary)' }}>{k.provider_id}</td>
                    <td>{k.display_name}</td>
                    <td><span class="code-inline">{k.key_prefix}</span></td>
                    <td>{formatUSD(k.starting_balance_micro_usd)}</td>
                    <td>{formatUSD(k.adjustments_micro_usd)}</td>
                    <td>{formatUSD(k.known_spend_micro_usd)}</td>
                    <td style={{ fontWeight: 600, color: k.estimated_remaining_micro_usd <= 0 ? 'var(--danger)' : 'var(--text-primary)' }}>
                      {formatUSD(k.estimated_remaining_micro_usd)}
                    </td>
                    <td>
                      <span class={`badge badge-${k.status === 'active' ? 'success' : 'danger'}`}>
                        {k.status}
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
