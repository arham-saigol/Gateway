import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { RequestRecord, RequestAttemptRecord } from '../types';

export function RequestLogsView() {
  const [requests, setRequests] = useState<RequestRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Detail drawer/modal
  const [selectedReq, setSelectedReq] = useState<RequestRecord | null>(null);
  const [attempts, setAttempts] = useState<RequestAttemptRecord[]>([]);
  const [loadingDetail, setLoadingDetail] = useState(false);

  useEffect(() => {
    loadRequests();
  }, []);

  async function loadRequests() {
    try {
      setLoading(true);
      const res = await apiRequest<RequestRecord[]>('/api/requests?limit=50');
      setRequests(res || []);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load requests');
    } finally {
      setLoading(false);
    }
  }

  async function openDetail(req: RequestRecord) {
    setSelectedReq(req);
    setLoadingDetail(true);
    try {
      const res = await apiRequest<{ request: RequestRecord; attempts: RequestAttemptRecord[] }>(`/api/requests/${req.id}`);
      setAttempts(res.attempts || []);
    } catch (err: any) {
      alert(err.message);
    } finally {
      setLoadingDetail(false);
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 600 }}>Request & Attempt Logs</h2>
          <p style={{ color: 'var(--text-secondary)', fontSize: '13px' }}>
            Metadata-only audit log of gateway requests, provider attempts, timings, and token spend.
          </p>
        </div>
        <button class="btn btn-secondary" onClick={loadRequests} disabled={loading}>
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {error && <div style={{ padding: '16px', color: 'var(--danger)', marginBottom: '16px' }}>{error}</div>}

      <div class="panel">
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Request ID</th>
                <th>Public Model</th>
                <th>Status</th>
                <th>Stream</th>
                <th>TTFT</th>
                <th>Duration</th>
                <th>Tokens (In / Out)</th>
                <th>Cost</th>
                <th>Created At</th>
                <th style={{ textAlign: 'right' }}>Details</th>
              </tr>
            </thead>
            <tbody>
              {requests.length === 0 ? (
                <tr>
                  <td colSpan={10} style={{ textAlign: 'center', padding: '32px', color: 'var(--text-muted)' }}>
                    {loading ? 'Loading requests...' : 'No request logs recorded yet.'}
                  </td>
                </tr>
              ) : (
                requests.map(r => (
                  <tr key={r.id}>
                    <td><span class="code-inline">{r.id.substring(0, 18)}...</span></td>
                    <td style={{ fontWeight: 500 }}>{r.public_model_id}</td>
                    <td>
                      <span class={`badge badge-${r.status === 'success' ? 'success' : 'danger'}`}>
                        {r.status}
                      </span>
                    </td>
                    <td>{r.stream ? 'Yes' : 'No'}</td>
                    <td>{r.ttft_ms !== undefined && r.ttft_ms !== null ? `${r.ttft_ms} ms` : '-'}</td>
                    <td>{r.total_duration_ms} ms</td>
                    <td>
                      {r.input_tokens !== undefined && r.output_tokens !== undefined ? (
                        <span>{r.input_tokens} / {r.output_tokens}</span>
                      ) : '-'}
                    </td>
                    <td style={{ fontWeight: 600 }}>{formatUSD(r.total_cost_micro_usd)}</td>
                    <td>{new Date(r.created_at).toLocaleTimeString()}</td>
                    <td style={{ textAlign: 'right' }}>
                      <button class="btn btn-secondary btn-sm" onClick={() => openDetail(r)}>
                        Attempts ({r.retry_count + 1})
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Request Attempts Detail Modal */}
      {selectedReq && (
        <div class="modal-overlay" onClick={() => setSelectedReq(null)}>
          <div class="modal-content" style={{ maxWidth: '680px' }} onClick={e => e.stopPropagation()}>
            <div class="modal-header">
              <div>
                <div class="panel-title">Request Execution Details</div>
                <div class="code-inline" style={{ marginTop: '4px', fontSize: '11px' }}>{selectedReq.id}</div>
              </div>
              <button class="btn btn-secondary btn-sm" onClick={() => setSelectedReq(null)}>✕</button>
            </div>

            <div class="modal-body">
              {loadingDetail ? (
                <div style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)' }}>Loading attempts...</div>
              ) : (
                <div>
                  <h4 style={{ fontSize: '13px', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '12px' }}>
                    Attempt History & Failover Sequence
                  </h4>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                    {attempts.map(att => (
                      <div
                        key={att.id}
                        style={{
                          backgroundColor: 'var(--bg-primary)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-sm)',
                          padding: '14px 16px',
                        }}
                      >
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                            <span style={{ fontWeight: 700, color: 'var(--accent)' }}>Attempt #{att.sequence}</span>
                            <span style={{ fontWeight: 600, textTransform: 'capitalize', color: 'var(--text-primary)' }}>
                              {att.provider_id}
                            </span>
                          </div>
                          <span class={`badge badge-${att.status === 'success' ? 'success' : 'danger'}`}>
                            {att.status} {att.http_status ? `(${att.http_status})` : ''}
                          </span>
                        </div>

                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '8px', fontSize: '12px', color: 'var(--text-secondary)' }}>
                          <div>Duration: <strong style={{ color: 'var(--text-primary)' }}>{att.duration_ms} ms</strong></div>
                          <div>TTFT: <strong style={{ color: 'var(--text-primary)' }}>{att.ttft_ms ?? '-'} ms</strong></div>
                          <div>Attempt Cost: <strong style={{ color: 'var(--text-primary)' }}>{formatUSD(att.total_cost_micro_usd)}</strong></div>
                          <div>Input Tokens: <strong style={{ color: 'var(--text-primary)' }}>{att.input_tokens ?? '-'}</strong></div>
                          <div>Cached Tokens: <strong style={{ color: 'var(--text-primary)' }}>{att.cached_input_tokens ?? '-'}</strong></div>
                          <div>Output Tokens: <strong style={{ color: 'var(--text-primary)' }}>{att.output_tokens ?? '-'}</strong></div>
                        </div>

                        {att.error_category && (
                          <div style={{ marginTop: '8px', fontSize: '12px', color: 'var(--danger)' }}>
                            Error category: {att.error_category}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div class="modal-footer">
              <button class="btn btn-primary" onClick={() => setSelectedReq(null)}>Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
