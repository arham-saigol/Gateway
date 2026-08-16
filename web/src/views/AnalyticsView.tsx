import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { DailyStat } from '../types';

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

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: 600 }}>Usage & Cost Analytics</h2>
        <p style={{ color: 'var(--text-secondary)', fontSize: '13px' }}>
          Durable daily rollup aggregates by public model, token volume, and exact calculated spend.
        </p>
      </div>

      {error && <div style={{ padding: '16px', color: 'var(--danger)', marginBottom: '16px' }}>{error}</div>}

      <div class="panel">
        <div class="panel-header">
          <span class="panel-title">Daily Model Aggregates</span>
          <button class="btn btn-secondary btn-sm" onClick={loadAnalytics} disabled={loading}>
            {loading ? 'Refreshing...' : 'Refresh'}
          </button>
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
              {stats.length === 0 ? (
                <tr>
                  <td colSpan={9} style={{ textAlign: 'center', padding: '32px', color: 'var(--text-muted)' }}>
                    {loading ? 'Loading analytics...' : 'No recorded traffic yet.'}
                  </td>
                </tr>
              ) : (
                stats.map((s, idx) => (
                  <tr key={idx}>
                    <td style={{ fontWeight: 600 }}>{s.date_utc}</td>
                    <td><span class="code-inline">{s.public_model_id}</span></td>
                    <td>{s.total_requests}</td>
                    <td style={{ color: 'var(--success)' }}>{s.success_requests}</td>
                    <td style={{ color: s.failed_requests > 0 ? 'var(--danger)' : 'var(--text-muted)' }}>{s.failed_requests}</td>
                    <td>{s.input_tokens.toLocaleString()}</td>
                    <td>{s.cached_input_tokens.toLocaleString()}</td>
                    <td>{s.output_tokens.toLocaleString()}</td>
                    <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{formatUSD(s.cost_micro_usd)}</td>
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
