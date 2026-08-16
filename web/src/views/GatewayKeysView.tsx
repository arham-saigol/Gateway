import { useEffect, useState } from 'preact/hooks';
import { apiRequest } from '../api';
import type { GatewayKey } from '../types';

export function GatewayKeysView() {
  const [keys, setKeys] = useState<GatewayKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [createdSecret, setCreatedSecret] = useState('');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    loadKeys();
  }, []);

  async function loadKeys() {
    try {
      setLoading(true);
      const res = await apiRequest<GatewayKey[]>('/api/gateway-keys');
      setKeys(res);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load gateway keys');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreateKey(e: any) {
    e.preventDefault();
    try {
      const res = await apiRequest<{ id: string; key: string; prefix: string; name: string }>('/api/gateway-keys', {
        method: 'POST',
        body: JSON.stringify({ name: keyName }),
      });
      setCreatedSecret(res.key);
      setKeyName('');
      loadKeys();
    } catch (err: any) {
      alert(err.message);
    }
  }

  async function handleRevokeKey(keyId: string) {
    if (!confirm('Are you sure you want to revoke this API key? Applications using it will immediately be denied access.')) {
      return;
    }

    try {
      await apiRequest(`/api/gateway-keys/${keyId}/revoke`, {
        method: 'POST',
      });
      loadKeys();
    } catch (err: any) {
      alert(err.message);
    }
  }

  function handleCopy() {
    navigator.clipboard.writeText(createdSecret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 600 }}>Gateway API Keys</h2>
          <p style={{ color: 'var(--text-secondary)', fontSize: '13px' }}>
            Issue client authentication keys used to access public OpenAI-compatible endpoints.
          </p>
        </div>
        <button class="btn btn-primary" onClick={() => { setCreatedSecret(''); setShowCreateModal(true); }}>
          + Create Gateway Key
        </button>
      </div>

      {error && <div style={{ padding: '16px', color: 'var(--danger)', marginBottom: '16px' }}>{error}</div>}

      <div class="panel">
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Key Name</th>
                <th>Prefix</th>
                <th>Status</th>
                <th>Created At</th>
                <th>Last Used</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {keys.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: '32px', color: 'var(--text-muted)' }}>
                    {loading ? 'Loading gateway keys...' : 'No gateway keys issued yet. Click Create Gateway Key above.'}
                  </td>
                </tr>
              ) : (
                keys.map(k => (
                  <tr key={k.id}>
                    <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{k.name}</td>
                    <td><span class="code-inline">{k.key_prefix}</span></td>
                    <td>
                      <span class={`badge badge-${k.status === 'active' ? 'success' : 'danger'}`}>
                        {k.status}
                      </span>
                    </td>
                    <td>{new Date(k.created_at).toLocaleString()}</td>
                    <td>{k.last_used_at ? new Date(k.last_used_at).toLocaleString() : 'Never'}</td>
                    <td style={{ textAlign: 'right' }}>
                      {k.status === 'active' && (
                        <button class="btn btn-danger btn-sm" onClick={() => handleRevokeKey(k.id)}>
                          Revoke
                        </button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Create Gateway Key Modal */}
      {showCreateModal && (
        <div class="modal-overlay" onClick={() => setShowCreateModal(false)}>
          <div class="modal-content" onClick={e => e.stopPropagation()}>
            {createdSecret ? (
              <div>
                <div class="modal-header">
                  <div class="panel-title" style={{ color: 'var(--success)' }}>Key Created Successfully</div>
                  <button type="button" class="btn btn-secondary btn-sm" onClick={() => setShowCreateModal(false)}>✕</button>
                </div>
                <div class="modal-body">
                  <p style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
                    Please copy this secret key now. For security, it will <strong>never be shown again</strong>.
                  </p>

                  <div class="key-reveal-box">
                    <span class="code-inline" style={{ fontSize: '13px', wordBreak: 'break-all' }}>
                      {createdSecret}
                    </span>
                    <button class="btn btn-secondary btn-sm" onClick={handleCopy}>
                      {copied ? 'Copied!' : 'Copy'}
                    </button>
                  </div>
                </div>
                <div class="modal-footer">
                  <button class="btn btn-primary" onClick={() => setShowCreateModal(false)}>Done</button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleCreateKey}>
                <div class="modal-header">
                  <div class="panel-title">Create Gateway Key</div>
                  <button type="button" class="btn btn-secondary btn-sm" onClick={() => setShowCreateModal(false)}>✕</button>
                </div>
                <div class="modal-body">
                  <div class="form-group">
                    <label class="form-label">Client / Description Name</label>
                    <input
                      class="form-input"
                      placeholder="e.g. VSCode Extension or Web App Client"
                      required
                      value={keyName}
                      onInput={(e: any) => setKeyName(e.target.value)}
                    />
                  </div>
                </div>
                <div class="modal-footer">
                  <button type="button" class="btn btn-secondary" onClick={() => setShowCreateModal(false)}>Cancel</button>
                  <button type="submit" class="btn btn-primary">Generate Secret Key</button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
