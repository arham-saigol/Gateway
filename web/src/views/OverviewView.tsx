import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { KeyBalanceSummary } from '../types';
import { IconRefresh, IconCloud, IconZap } from '../components/Icons';
import { MetricCard, Badge, CopyButton, EmptyState } from '../components/UI';

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
    return (
      <div style={{ padding: '48px', textAlign: 'center', color: 'var(--text-muted)' }}>
        <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 12px' }} />
        <div>Loading gateway metrics...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: '16px 20px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-md)', color: 'var(--danger)', marginBottom: '20px' }}>
        {error}
      </div>
    );
  }

  const successRate = data?.success_rate_24h ?? 100;
  const remainingMicro = data?.total_remaining_micro_usd ?? 0;

  return (
    <div>
      <div class="page-header">
        <div>
          <h1 class="page-title">Overview</h1>
          <p class="page-subtitle">Real-time status, health metrics, and account balances across all model providers.</p>
        </div>
        <button class="btn btn-secondary btn-sm" onClick={loadOverview} disabled={loading}>
          <IconRefresh size={13} class={loading ? 'animate-spin' : ''} />
          <span>{loading ? 'Refreshing...' : 'Refresh'}</span>
        </button>
      </div>

      <div class="stats-grid">
        <MetricCard
          label="24H Requests"
          value={(data?.requests_24h ?? 0).toLocaleString()}
          subtext="Total routed API calls"
        />

        <MetricCard
          label="24H Success Rate"
          value={`${successRate.toFixed(1)}%`}
          valueColor={successRate >= 99 ? 'var(--success)' : successRate >= 90 ? 'var(--warning)' : 'var(--danger)'}
          subtext="Completed without failure"
        />

        <MetricCard
          label="24H Avg Latency"
          value={`${(data?.avg_latency_ms_24h ?? 0).toFixed(0)} ms`}
          subtext="End-to-end response time"
        />

        <MetricCard
          label="Total Known Spend"
          value={formatUSD(data?.total_spend_micro_usd)}
          subtext="Calculated token expenditure"
        />

        <MetricCard
          label="Estimated Remaining"
          value={formatUSD(remainingMicro)}
          valueColor={remainingMicro > 0 ? '#ffffff' : 'var(--danger)'}
          subtext="Starting + adjustments - spend"
        />
      </div>

      <div class="panel">
        <div class="panel-header">
          <div class="panel-title">
            <IconCloud size={16} />
            <span>Upstream Provider Accounts & Balances</span>
          </div>
          <Badge variant="neutral" withDot={false}>
            {data?.key_balances?.length ?? 0} Keys
          </Badge>
        </div>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Provider</th>
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
                  <td colSpan={7} style={{ padding: 0 }}>
                    <EmptyState
                      icon={<IconCloud size={24} />}
                      title="No provider keys configured"
                      description="Go to Providers & Keys tab to configure your upstream AI credentials."
                    />
                  </td>
                </tr>
              ) : (
                data.key_balances.map(k => (
                  <tr key={k.provider_key_id}>
                    <td style={{ fontWeight: 600, textTransform: 'capitalize', color: 'var(--text-primary)' }}>
                      {k.provider_id}
                    </td>
                    <td>
                      <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                        <span class="code-inline">{k.key_prefix}</span>
                        <CopyButton text={k.key_prefix} size="xs" />
                      </div>
                    </td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{formatUSD(k.starting_balance_micro_usd)}</td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{formatUSD(k.adjustments_micro_usd)}</td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{formatUSD(k.known_spend_micro_usd)}</td>
                    <td style={{ fontWeight: 600, fontFamily: 'var(--font-mono)', color: k.estimated_remaining_micro_usd <= 0 ? 'var(--danger)' : '#ffffff' }}>
                      {formatUSD(k.estimated_remaining_micro_usd)}
                    </td>
                    <td>
                      <Badge variant={k.status === 'active' ? 'success' : k.status === 'exhausted' ? 'warning' : 'danger'}>
                        {k.status}
                      </Badge>
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
