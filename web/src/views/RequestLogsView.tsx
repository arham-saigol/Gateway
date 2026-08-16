import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { RequestRecord, RequestAttemptRecord } from '../types';
import { IconLogs, IconRefresh, IconZap, IconActivity, IconAlertTriangle } from '../components/Icons';
import { Badge, CopyButton, Modal, SearchInput, EmptyState } from '../components/UI';

export function RequestLogsView() {
  const [requests, setRequests] = useState<RequestRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [searchQuery, setSearchQuery] = useState('');

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

  const filteredRequests = requests.filter(r => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return (
      r.id.toLowerCase().includes(q) ||
      r.public_model_id.toLowerCase().includes(q) ||
      r.status.toLowerCase().includes(q)
    );
  });

  return (
    <div>
      <div class="page-header">
        <div>
          <h1 class="page-title">Request Logs</h1>
          <p class="page-subtitle">Metadata-only audit log with timings, attempt failovers, token metrics, and cost calculations.</p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <SearchInput
            value={searchQuery}
            onInput={setSearchQuery}
            placeholder="Filter requests or models..."
          />
          <button class="btn btn-secondary btn-sm" onClick={loadRequests} disabled={loading}>
            <IconRefresh size={13} class={loading ? 'animate-spin' : ''} />
            <span>{loading ? 'Refreshing...' : 'Refresh'}</span>
          </button>
        </div>
      </div>

      {error && (
        <div style={{ padding: '14px 18px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-md)', color: 'var(--danger)', marginBottom: '20px' }}>
          {error}
        </div>
      )}

      <div class="panel">
        <div class="panel-header">
          <div class="panel-title">
            <IconLogs size={16} />
            <span>Recent Gateway Requests</span>
          </div>
          <Badge variant="neutral" withDot={false}>
            Showing {filteredRequests.length} of {requests.length}
          </Badge>
        </div>

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
                <th>Time</th>
                <th style={{ textAlign: 'right' }}>Failover & Attempts</th>
              </tr>
            </thead>
            <tbody>
              {loading && requests.length === 0 ? (
                <tr>
                  <td colSpan={10} style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
                    <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 8px' }} />
                    <div>Loading request logs...</div>
                  </td>
                </tr>
              ) : filteredRequests.length === 0 ? (
                <tr>
                  <td colSpan={10} style={{ padding: 0 }}>
                    <EmptyState
                      icon={<IconLogs size={24} />}
                      title={searchQuery ? 'No requests match your filter' : 'No requests recorded yet'}
                      description={searchQuery ? 'Try searching for a different ID or model name.' : 'Send a chat completion to your gateway to view real-time traffic here.'}
                    />
                  </td>
                </tr>
              ) : (
                filteredRequests.map(r => (
                  <tr key={r.id}>
                    <td>
                      <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                        <span class="code-inline">{r.id.substring(0, 16)}...</span>
                        <CopyButton text={r.id} size="xs" />
                      </div>
                    </td>
                    <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{r.public_model_id}</td>
                    <td>
                      <Badge variant={r.status === 'success' ? 'success' : 'danger'}>
                        {r.status}
                      </Badge>
                    </td>
                    <td>
                      <span style={{ fontSize: '12px', color: r.stream ? 'var(--text-primary)' : 'var(--text-muted)' }}>
                        {r.stream ? 'Stream' : 'Unary'}
                      </span>
                    </td>
                    <td style={{ fontFamily: 'var(--font-mono)', fontSize: '12px' }}>
                      {r.ttft_ms !== undefined && r.ttft_ms !== null ? `${r.ttft_ms} ms` : '-'}
                    </td>
                    <td style={{ fontFamily: 'var(--font-mono)', fontSize: '12px' }}>
                      {r.total_duration_ms} ms
                    </td>
                    <td style={{ fontFamily: 'var(--font-mono)', fontSize: '12px' }}>
                      {r.input_tokens !== undefined && r.output_tokens !== undefined ? (
                        <span>{r.input_tokens} / {r.output_tokens}</span>
                      ) : '-'}
                    </td>
                    <td style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: '#ffffff' }}>
                      {formatUSD(r.total_cost_micro_usd)}
                    </td>
                    <td style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                      {new Date(r.created_at).toLocaleTimeString()}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button class="btn btn-secondary btn-sm" onClick={() => openDetail(r)}>
                        <IconActivity size={12} />
                        <span>{r.retry_count > 0 ? `${r.retry_count + 1} Attempts` : '1 Attempt'}</span>
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
      <Modal
        isOpen={Boolean(selectedReq)}
        onClose={() => setSelectedReq(null)}
        title="Request Execution & Failover Details"
        maxWidth="660px"
        footer={
          <button type="button" class="btn btn-primary" onClick={() => setSelectedReq(null)}>
            Done
          </button>
        }
      >
        {selectedReq && (
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 14px', backgroundColor: 'var(--bg-canvas)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--radius-md)', marginBottom: '16px' }}>
              <div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.04em' }}>Request ID</div>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: '12.5px', color: '#ffffff', marginTop: '2px' }}>{selectedReq.id}</div>
              </div>
              <CopyButton text={selectedReq.id} label="Copy ID" size="xs" />
            </div>

            {loadingDetail ? (
              <div style={{ textAlign: 'center', padding: '32px', color: 'var(--text-muted)' }}>
                <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 8px' }} />
                <div>Loading attempt timeline...</div>
              </div>
            ) : attempts.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)' }}>No attempt records found for this request.</div>
            ) : (
              <div>
                <div style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: '10px' }}>
                  Execution Timeline & Failover Waterfall ({attempts.length} attempts)
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                  {attempts.map(att => (
                    <div
                      key={att.id}
                      style={{
                        backgroundColor: 'var(--bg-canvas)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: 'var(--radius-md)',
                        padding: '14px 16px',
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                          <span class="priority-rank-badge">
                            ATTEMPT #{att.sequence}
                          </span>
                          <span style={{ fontWeight: 600, textTransform: 'capitalize', color: '#ffffff', fontSize: '13px' }}>
                            {att.provider_id}
                          </span>
                        </div>
                        <Badge variant={att.status === 'success' ? 'success' : 'danger'}>
                          {att.status} {att.http_status ? `(${att.http_status})` : ''}
                        </Badge>
                      </div>

                      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '10px', fontSize: '12px', color: 'var(--text-secondary)' }}>
                        <div>Duration: <strong style={{ color: '#ffffff', fontFamily: 'var(--font-mono)' }}>{att.duration_ms} ms</strong></div>
                        <div>TTFT: <strong style={{ color: '#ffffff', fontFamily: 'var(--font-mono)' }}>{att.ttft_ms ?? '-'} ms</strong></div>
                        <div>Cost: <strong style={{ color: '#ffffff', fontFamily: 'var(--font-mono)' }}>{formatUSD(att.total_cost_micro_usd)}</strong></div>
                        <div>Input Tokens: <strong style={{ color: '#ffffff', fontFamily: 'var(--font-mono)' }}>{att.input_tokens ?? '-'}</strong></div>
                        <div>Cached Tokens: <strong style={{ color: '#ffffff', fontFamily: 'var(--font-mono)' }}>{att.cached_input_tokens ?? '-'}</strong></div>
                        <div>Output Tokens: <strong style={{ color: '#ffffff', fontFamily: 'var(--font-mono)' }}>{att.output_tokens ?? '-'}</strong></div>
                      </div>

                      {att.error_category && (
                        <div style={{ marginTop: '10px', padding: '8px 10px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-xs)', fontSize: '11.5px', color: 'var(--danger)', display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <IconAlertTriangle size={13} />
                          <span>Error: {att.error_category}</span>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}
