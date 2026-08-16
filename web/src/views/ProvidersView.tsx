import { useEffect, useState } from 'preact/hooks';
import { apiRequest, formatUSD } from '../api';
import type { Provider } from '../types';
import { IconPlus, IconCloud, IconAlertTriangle, IconClock, IconZap } from '../components/Icons';
import { Badge, CopyButton, Modal, Callout, EmptyState } from '../components/UI';

export function ProvidersView() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Modals
  const [showAddModal, setShowAddModal] = useState(false);
  const [showAdjustModal, setShowAdjustModal] = useState(false);
  const [selectedKeyId, setSelectedKeyId] = useState('');
  const [selectedKeyName, setSelectedKeyName] = useState('');

  // Form states
  const [formProviderId, setFormProviderId] = useState('fireworks');
  const [formDisplayName, setFormDisplayName] = useState('');
  const [formSecret, setFormSecret] = useState('');
  const [formStartingBalanceUSD, setFormStartingBalanceUSD] = useState('6.00');

  const [adjustAmountUSD, setAdjustAmountUSD] = useState('5.00');
  const [adjustNote, setAdjustNote] = useState('');
  const [submitting, setSubmitting] = useState(false);

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
    setSubmitting(true);
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
    } finally {
      setSubmitting(false);
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
    setSubmitting(true);
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
    } finally {
      setSubmitting(false);
    }
  }

  function openAdjustModal(keyId: string, keyName: string) {
    setSelectedKeyId(keyId);
    setSelectedKeyName(keyName);
    setAdjustAmountUSD('5.00');
    setAdjustNote('');
    setShowAdjustModal(true);
  }

  return (
    <div>
      <div class="page-header">
        <div>
          <h1 class="page-title">Providers & Keys</h1>
          <p class="page-subtitle">Configure encrypted credentials and balance tracking for upstream LLM providers.</p>
        </div>
        <button class="btn btn-primary" onClick={() => setShowAddModal(true)}>
          <IconPlus size={14} />
          <span>Add Provider Key</span>
        </button>
      </div>

      <Callout>
        <strong>AES-GCM Encryption:</strong> All provider keys are encrypted with your master key at rest before touching SQLite.
        Keys are never decrypted or returned to the browser after creation.
      </Callout>

      {error && (
        <div style={{ padding: '14px 18px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-md)', color: 'var(--danger)', marginBottom: '20px' }}>
          {error}
        </div>
      )}

      {loading && providers.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '48px', color: 'var(--text-muted)' }}>
          <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 12px' }} />
          <div>Loading providers...</div>
        </div>
      ) : providers.length === 0 ? (
        <EmptyState
          icon={<IconCloud size={28} />}
          title="No providers registered"
          description="Register upstream AI providers to route inference requests."
          action={
            <button class="btn btn-primary" onClick={() => setShowAddModal(true)}>
              <IconPlus size={14} />
              <span>Add First Provider Key</span>
            </button>
          }
        />
      ) : (
        providers.map(p => (
          <div class="panel" key={p.id}>
            <div class="panel-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <span class="panel-title">{p.name}</span>
                <span class="code-inline">{p.id}</span>
              </div>
              <Badge variant={p.enabled ? 'success' : 'muted'}>
                {p.enabled ? 'Enabled' : 'Disabled'}
              </Badge>
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
                      <td colSpan={6} style={{ padding: '24px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
                        No configured keys for {p.name}. Click "Add Provider Key" above to add one.
                      </td>
                    </tr>
                  ) : (
                    p.keys.map(k => (
                      <tr key={k.id}>
                        <td>
                          <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{k.display_name}</div>
                          {k.safe_last_error && (
                            <div style={{ display: 'flex', alignItems: 'center', gap: '4px', color: 'var(--danger)', fontSize: '11.5px', marginTop: '3px' }}>
                              <IconAlertTriangle size={12} />
                              <span>{k.safe_last_error}</span>
                            </div>
                          )}
                        </td>
                        <td>
                          <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                            <span class="code-inline">{k.key_prefix}</span>
                            <CopyButton text={k.key_prefix} size="xs" />
                          </div>
                        </td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{formatUSD(k.starting_balance_micro_usd)}</td>
                        <td>
                          <Badge variant={k.status === 'active' ? 'success' : k.status === 'exhausted' ? 'warning' : 'danger'}>
                            {k.status}
                          </Badge>
                        </td>
                        <td style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                          {k.last_used_at ? (
                            <div style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
                              <IconClock size={12} />
                              <span>{new Date(k.last_used_at).toLocaleDateString()} {new Date(k.last_used_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
                            </div>
                          ) : (
                            'Never'
                          )}
                        </td>
                        <td style={{ textAlign: 'right' }}>
                          <div style={{ display: 'inline-flex', gap: '6px', justifyContent: 'flex-end' }}>
                            <button
                              class="btn btn-secondary btn-sm"
                              onClick={() => openAdjustModal(k.id, k.display_name)}
                              title="Adjust starting balance or add credit"
                            >
                              Adjust Balance
                            </button>
                            <button
                              class={`btn btn-sm ${k.status === 'active' ? 'btn-danger-subtle' : 'btn-secondary'}`}
                              onClick={() => handleToggleStatus(k.id, k.status)}
                            >
                              {k.status === 'active' ? 'Disable' : 'Enable'}
                            </button>
                          </div>
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
      <Modal
        isOpen={showAddModal}
        onClose={() => setShowAddModal(false)}
        title="Add Upstream Provider Key"
        footer={
          <>
            <button type="button" class="btn btn-ghost" onClick={() => setShowAddModal(false)}>
              Cancel
            </button>
            <button type="submit" form="add-key-form" class="btn btn-primary" disabled={submitting}>
              {submitting ? 'Saving...' : 'Save Encrypted Key'}
            </button>
          </>
        }
      >
        <form id="add-key-form" onSubmit={handleCreateKey}>
          <div class="form-group">
            <label class="form-label">Provider</label>
            <select
              class="form-select"
              value={formProviderId}
              onChange={(e: any) => setFormProviderId(e.target.value)}
            >
              <option value="fireworks">Fireworks AI</option>
              <option value="together">Together AI</option>
              <option value="groq">Groq</option>
              <option value="openrouter">OpenRouter</option>
              <option value="deepinfra">DeepInfra</option>
              <option value="openai">OpenAI</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Display Name / Label</label>
            <input
              type="text"
              class="form-input"
              required
              placeholder="e.g. Primary Fireworks Key"
              value={formDisplayName}
              onInput={(e: any) => setFormDisplayName(e.target.value)}
            />
          </div>

          <div class="form-group">
            <label class="form-label">Upstream Secret API Key</label>
            <input
              type="password"
              class="form-input"
              required
              placeholder="fw_... or sk-..."
              value={formSecret}
              onInput={(e: any) => setFormSecret(e.target.value)}
            />
          </div>

          <div class="form-group">
            <label class="form-label">
              <span>Initial Free / Funded Balance (USD)</span>
              <span style={{ fontSize: '11px', color: 'var(--text-subtle)' }}>Default: $6.00</span>
            </label>
            <input
              type="number"
              step="0.01"
              class="form-input"
              required
              value={formStartingBalanceUSD}
              onInput={(e: any) => setFormStartingBalanceUSD(e.target.value)}
            />
          </div>
        </form>
      </Modal>

      {/* Adjust Balance Modal */}
      <Modal
        isOpen={showAdjustModal}
        onClose={() => setShowAdjustModal(false)}
        title={`Adjust Balance: ${selectedKeyName}`}
        footer={
          <>
            <button type="button" class="btn btn-ghost" onClick={() => setShowAdjustModal(false)}>
              Cancel
            </button>
            <button type="submit" form="adjust-balance-form" class="btn btn-primary" disabled={submitting}>
              {submitting ? 'Applying...' : 'Apply Adjustment'}
            </button>
          </>
        }
      >
        <form id="adjust-balance-form" onSubmit={handleAddAdjustment}>
          <div class="form-group">
            <label class="form-label">Adjustment Amount in USD</label>
            <input
              type="number"
              step="0.01"
              class="form-input"
              required
              placeholder="e.g. 5.00 or -2.50"
              value={adjustAmountUSD}
              onInput={(e: any) => setAdjustAmountUSD(e.target.value)}
            />
            <span style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '4px' }}>
              Positive values add funds/promotional credits. Negative values reduce starting balance.
            </span>
          </div>

          <div class="form-group">
            <label class="form-label">Note / Rationale</label>
            <input
              type="text"
              class="form-input"
              placeholder="e.g. Monthly free credit renewal"
              value={adjustNote}
              onInput={(e: any) => setAdjustNote(e.target.value)}
            />
          </div>
        </form>
      </Modal>
    </div>
  );
}
