import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { DailyStat } from '../types';
import { IconRefresh, IconAnalytics, IconZap } from '../components/Icons';
import { MetricCard, Badge, EmptyState } from '../components/UI';

export function AnalyticsView() {
  const [stats, setStats] = useState<DailyStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    loadAnalytics();
  }, []);

  async function loadAnalytics() {
    try {
      setLoading(true);
      const res = await apiRequest<{ daily_stats: DailyStat[] }>('/api/analytics');
      setStats(res.daily_stats || []);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load analytics');
    } finally {
      setLoading(false);
    }
  }

  const totalRequests = stats.reduce((acc, s) => acc + s.total_requests, 0);
  const totalTokens = stats.reduce((acc, s) => acc + s.input_tokens + s.output_tokens, 0);
  const totalCost = stats.reduce((acc, s) => acc + s.cost_micro_usd, 0);

  return (
    <div>
      <div class="page-header">
        <div>
          <h1 class="page-title">Analytics</h1>
          <p class="page-subtitle">Durable daily rollups, token volume aggregates, and exact model cost accounting.</p>
        </div>
        <button class="btn btn-secondary btn-sm" onClick={loadAnalytics} disabled={loading}>
          <IconRefresh size={13} class={loading ? 'animate-spin' : ''} />
          <span>{loading ? 'Refreshing...' : 'Refresh'}</span>
        </button>
      </div>

      {stats.length > 0 && (
        <div class="stats-grid">
          <MetricCard
            label="Aggregated Requests"
            value={totalRequests.toLocaleString()}
            subtext="Across recorded history"
          />
          <MetricCard
            label="Total Tokens Processed"
            value={totalTokens.toLocaleString()}
            subtext="Prompt + Output tokens"
          />
          <MetricCard
            label="Total Recorded Spend"
            value={formatUSD(totalCost)}
            subtext="Exact token pricing sum"
          />
        </div>
      )}

      {error && (
        <div style={{ padding: '14px 18px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-md)', color: 'var(--danger)', marginBottom: '20px' }}>
          {error}
        </div>
      )}

      <div class="panel">
        <div class="panel-header">
          <div class="panel-title">
            <IconAnalytics size={16} />
            <span>Daily Model Rollup Records</span>
          </div>
          <Badge variant="neutral" withDot={false}>
            {stats.length} Days Recorded
          </Badge>
        </div>

        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Date (UTC)</th>
                <th>Public Model</th>
                <th>Total Requests</th>
                <th>Successful</th>
                <th>Failed</th>
                <th>Input Tokens</th>
                <th>Cached Tokens</th>
                <th>Output Tokens</th>
                <th>Exact Cost</th>
              </tr>
            </thead>
            <tbody>
              {loading && stats.length === 0 ? (
                <tr>
                  <td colSpan={9} style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
                    <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 8px' }} />
                    <div>Loading analytics rollups...</div>
                  </td>
                </tr>
              ) : stats.length === 0 ? (
                <tr>
                  <td colSpan={9} style={{ padding: 0 }}>
                    <EmptyState
                      icon={<IconAnalytics size={24} />}
                      title="No traffic recorded yet"
                      description="Daily analytics will automatically compute and display here once requests pass through the gateway."
                    />
                  </td>
                </tr>
              ) : (
                stats.map((s, idx) => (
                  <tr key={idx}>
                    <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{s.date_utc}</td>
                    <td><span class="code-inline">{s.public_model_id}</span></td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{s.total_requests.toLocaleString()}</td>
                    <td>
                      <Badge variant="success" withDot={false}>
                        {s.success_requests.toLocaleString()}
                      </Badge>
                    </td>
                    <td>
                      {s.failed_requests > 0 ? (
                        <Badge variant="danger" withDot={false}>
                          {s.failed_requests.toLocaleString()}
                        </Badge>
                      ) : (
                        <span style={{ color: 'var(--text-subtle)', fontSize: '12px' }}>0</span>
                      )}
                    </td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{s.input_tokens.toLocaleString()}</td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{s.cached_input_tokens.toLocaleString()}</td>
                    <td style={{ fontFamily: 'var(--font-mono)' }}>{s.output_tokens.toLocaleString()}</td>
                    <td style={{ fontWeight: 600, fontFamily: 'var(--font-mono)', color: '#ffffff' }}>
                      {formatUSD(s.cost_micro_usd)}
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
