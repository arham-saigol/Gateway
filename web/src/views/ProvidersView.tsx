import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { Provider } from '../types';

export function ProvidersView() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Modals
  const [showAddModal, setShowAddModal] = useState(false);
  const [showAdjustModal, setShowAdjustModal] = useState(false);
  const [selectedKeyId, setSelectedKeyId] = useState('');

  // Form states
  const [formProviderId, setFormProviderId] = useState('fireworks');
  const [formDisplayName, setFormDisplayName] = useState('');
  const [formSecret, setFormSecret] = useState('');
  const [formStartingBalanceUSD, setFormStartingBalanceUSD] = useState('6.00');

  const [adjustAmountUSD, setAdjustAmountUSD] = useState('5.00');
  const [adjustNote, setAdjustNote] = useState('');

  useEffect(() => {
    loadProviders();
  }, []);

  async function loadProviders() {
    try {
      setLoading(true);
      const res = await apiRequest<Provider[]>('/api/providers');
      setProviders(res || []);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load providers');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreateKey(e: any) {
    e.preventDefault();
    try {
      const startingMicro = Math.round(parseFloat(formStartingBalanceUSD) * 1000000);
      await apiRequest('/api/providers/keys', {
        method: 'POST',
        body: JSON.stringify({
          provider_id: formProviderId,
          display_name: formDisplayName,
          secret: formSecret,
          starting_balance_micro_usd: startingMicro,
        }),
      });

      setShowAddModal(false);
      setFormSecret('');
      setFormDisplayName('');
      loadProviders();
    } catch (err: any) {
      alert(err.message);
    }
  }

  async function handleToggleStatus(keyId: string, currentStatus: string) {
    const nextStatus = currentStatus === 'active' ? 'disabled' : 'active';
    try {
      await apiRequest(`/api/providers/keys/${keyId}/status`, {
        method: 'POST',
        body: JSON.stringify({ status: nextStatus }),
      });
      loadProviders();
    } catch (err: any) {
      alert(err.message);
    }
  }

  async function handleAddAdjustment(e: any) {
    e.preventDefault();
    try {
      const micro = Math.round(parseFloat(adjustAmountUSD) * 1000000);
      await apiRequest(`/api/providers/keys/${selectedKeyId}/adjust`, {
        method: 'POST',
        body: JSON.stringify({
          amount_micro_usd: micro,
          note: adjustNote,
        }),
      });
      setShowAdjustModal(false);
      setAdjustNote('');
      loadProviders();
    } catch (err: any) {
      alert(err.message);
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 600 }}>Upstream Providers & API Keys</h2>
          <p style={{ color: 'var(--text-secondary)', fontSize: '13px' }}>
            Configure and manage encrypted keys for upstream model providers.
          </p>
        </div>
        <button class="btn btn-primary" onClick={() => setShowAddModal(true)}>+ Add Provider Key</button>
      </div>

      {error && <div style={{ padding: '16px', color: 'var(--danger)', marginBottom: '16px' }}>{error}</div>}

      {loading && providers.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>Loading providers...</div>
      ) : (
        providers.map(p => (
          <div class="panel" key={p.id}>
            <div class="panel-header">
              <div>
                <span class="panel-title">{p.name}</span>
                <span class="code-inline" style={{ marginLeft: '10px' }}>id: {p.id}</span>
              </div>
              <span class={`badge ${p.enabled ? 'badge-success' : 'badge-muted'}`}>
                {p.enabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>

            <div class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Display Name</th>
                    <th>Key Prefix</th>
                    <th>Starting Balance</th>
                    <th>Status</th>
                    <th>Last Used</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {(!p.keys || p.keys.length === 0) ? (
                    <tr>
                      <td colSpan={6} style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)' }}>
                        No configured keys for {p.name}.
                      </td>
                    </tr>
                  ) : (
                    p.keys.map(k => (
                      <tr key={k.id}>
                        <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{k.display_name}</td>
                        <td><span class="code-inline">{k.key_prefix}</span></td>
                        <td>{formatUSD(k.starting_balance_micro_usd)}</td>
                        <td>
                          <span class={`badge badge-${k.status === 'active' ? 'success' : 'danger'}`}>
                            {k.status}
                          </span>
                          {k.safe_last_error && (
                            <div style={{ fontSize: '11px', color: 'var(--danger)', marginTop: '4px' }}>
                              {k.safe_last_error}
                            </div>
                          )}
                        </td>
                        <td>{k.last_used_at ? new Date(k.last_used_at).toLocaleString() : 'Never'}</td>
                        <td style={{ textAlign: 'right' }}>
                          <button
                            class="btn btn-secondary btn-sm"
                            style={{ marginRight: '6px' }}
                            onClick={() => {
                              setSelectedKeyId(k.id);
                              setShowAdjustModal(true);
                            }}
                          >
                            Adjust Balance
                          </button>
                          <button
                            class={`btn ${k.status === 'active' ? 'btn-danger' : 'btn-primary'} btn-sm`}
                            onClick={() => handleToggleStatus(k.id, k.status)}
                          >
                            {k.status === 'active' ? 'Disable' : 'Enable'}
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        ))
      )}

      {/* Add Provider Key Modal */}
      {showAddModal && (
        <div class="modal-overlay" onClick={() => setShowAddModal(false)}>
          <div class="modal-content" onClick={e => e.stopPropagation()}>
            <form onSubmit={handleCreateKey}>
              <div class="modal-header">
                <div class="panel-title">Add Provider Key</div>
                <button type="button" class="btn btn-secondary btn-sm" onClick={() => setShowAddModal(false)}>✕</button>
              </div>
              <div class="modal-body">
                <div class="form-group">
                  <label class="form-label">Provider</label>
                  <select
                    class="form-input"
                    value={formProviderId}
                    onChange={(e: any) => {
                      setFormProviderId(e.target.value);
                      if (e.target.value === 'fireworks') setFormStartingBalanceUSD('6.00');
                      else setFormStartingBalanceUSD('1.00');
                    }}
                  >
                    <option value="fireworks">Fireworks AI</option>
                    <option value="siliconflow">SiliconFlow</option>
                    <option value="novita">Novita AI</option>
                    <option value="baseten">Baseten</option>
                  </select>
                </div>

                <div class="form-group">
                  <label class="form-label">Key Display Name</label>
                  <input
                    class="form-input"
                    placeholder="e.g. Primary Fireworks Account"
                    required
                    value={formDisplayName}
                    onInput={(e: any) => setFormDisplayName(e.target.value)}
                  />
                </div>

                <div class="form-group">
                  <label class="form-label">Starting Balance (USD)</label>
                  <input
                    type="number"
                    step="0.01"
                    class="form-input"
                    required
                    value={formStartingBalanceUSD}
                    onInput={(e: any) => setFormStartingBalanceUSD(e.target.value)}
                  />
                </div>

                <div class="form-group">
                  <label class="form-label">Secret API Key</label>
                  <input
                    type="password"
                    class="form-input"
                    placeholder="fw_..."
                    required
                    value={formSecret}
                    onInput={(e: any) => setFormSecret(e.target.value)}
                  />
                  <small style={{ color: 'var(--text-muted)', fontSize: '11px', marginTop: '4px' }}>
                    Encrypted at rest with your master key. Never visible after submission.
                  </small>
                </div>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" onClick={() => setShowAddModal(false)}>Cancel</button>
                <button type="submit" class="btn btn-primary">Save Encrypted Key</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Adjust Balance Modal */}
      {showAdjustModal && (
        <div class="modal-overlay" onClick={() => setShowAdjustModal(false)}>
          <div class="modal-content" onClick={e => e.stopPropagation()}>
            <form onSubmit={handleAddAdjustment}>
              <div class="modal-header">
                <div class="panel-title">Add Balance Adjustment</div>
                <button type="button" class="btn btn-secondary btn-sm" onClick={() => setShowAdjustModal(false)}>✕</button>
              </div>
              <div class="modal-body">
                <div class="form-group">
                  <label class="form-label">Adjustment Amount (USD)</label>
                  <input
                    type="number"
                    step="0.01"
                    class="form-input"
                    placeholder="e.g. 5.00 or -2.50"
                    required
                    value={adjustAmountUSD}
                    onInput={(e: any) => setAdjustAmountUSD(e.target.value)}
                  />
                </div>
                <div class="form-group">
                  <label class="form-label">Adjustment Note / Reason</label>
                  <input
                    class="form-input"
                    placeholder="e.g. Purchased $5 promotional credits"
                    required
                    value={adjustNote}
                    onInput={(e: any) => setAdjustNote(e.target.value)}
                  />
                </div>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" onClick={() => setShowAdjustModal(false)}>Cancel</button>
                <button type="submit" class="btn btn-primary">Save Adjustment</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
