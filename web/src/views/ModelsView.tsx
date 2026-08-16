import { useEffect, useState } from 'preact/hooks';
import { apiRequest } from '../api';
import type { PublicModelWithRoutes, ProviderModelMapping } from '../types';

export function ModelsView() {
  const [models, setModels] = useState<PublicModelWithRoutes[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Rate edit modal
  const [showRateModal, setShowRateModal] = useState(false);
  const [editingMapping, setEditingMapping] = useState<ProviderModelMapping | null>(null);
  const [inputRatePerMUSD, setInputRatePerMUSD] = useState('');
  const [cachedRatePerMUSD, setCachedRatePerMUSD] = useState('');
  const [outputRatePerMUSD, setOutputRatePerMUSD] = useState('');

  useEffect(() => {
    loadModels();
  }, []);

  async function loadModels() {
    try {
      setLoading(true);
      const res = await apiRequest<PublicModelWithRoutes[]>('/api/models');
      setModels(res || []);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load models');
    } finally {
      setLoading(false);
    }
  }

  async function moveRoute(modelId: string, currentIndex: number, direction: 'up' | 'down') {
    const model = models.find(m => m.id === modelId);
    if (!model) return;

    const newIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1;
    if (newIndex < 0 || newIndex >= model.routes.length) return;

    const newRoutes = [...model.routes];
    const [moved] = newRoutes.splice(currentIndex, 1);
    newRoutes.splice(newIndex, 0, moved);

    const mappingIDs = newRoutes.map(r => r.mapping_id);

    try {
      await apiRequest(`/api/models/${modelId}/routes`, {
        method: 'POST',
        body: JSON.stringify({ mapping_ids: mappingIDs }),
      });
      loadModels();
    } catch (err: any) {
      alert(err.message);
    }
  }

  async function handleSaveRates(e: any) {
    e.preventDefault();
    if (!editingMapping) return;

    try {
      const inRateMicro = Math.round(parseFloat(inputRatePerMUSD) * 1000000);
      const cachedRateMicro = Math.round(parseFloat(cachedRatePerMUSD) * 1000000);
      const outRateMicro = Math.round(parseFloat(outputRatePerMUSD) * 1000000);

      await apiRequest(`/api/mappings/${editingMapping.id}/rates`, {
        method: 'POST',
        body: JSON.stringify({
          input_rate: inRateMicro,
          cached_rate: cachedRateMicro,
          output_rate: outRateMicro,
        }),
      });

      setShowRateModal(false);
      setEditingMapping(null);
      loadModels();
    } catch (err: any) {
      alert(err.message);
    }
  }

  function openRateModal(mapping: ProviderModelMapping) {
    setEditingMapping(mapping);
    setInputRatePerMUSD((mapping.input_rate_per_m_tokens / 1000000).toFixed(4));
    setCachedRatePerMUSD((mapping.cached_rate_per_m_tokens / 1000000).toFixed(4));
    setOutputRatePerMUSD((mapping.output_rate_per_m_tokens / 1000000).toFixed(4));
    setShowRateModal(true);
  }

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: 600 }}>Public Models & Routing Priorities</h2>
        <p style={{ color: 'var(--text-secondary)', fontSize: '13px' }}>
          Manage gateway-exposed aliases, upstream provider mappings, and prioritized failover order.
        </p>
      </div>

      {error && <div style={{ padding: '16px', color: 'var(--danger)', marginBottom: '16px' }}>{error}</div>}

      {loading && models.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>Loading model routes...</div>
      ) : (
        models.map(m => (
          <div class="panel" key={m.id}>
            <div class="panel-header">
              <div>
                <span class="panel-title">{m.display_name}</span>
                <span class="code-inline" style={{ marginLeft: '10px' }}>{m.id}</span>
              </div>
              <span class="badge badge-success">Active Public Alias</span>
            </div>

            <div style={{ padding: '20px 24px 8px 24px', borderBottom: '1px solid var(--border-subtle)' }}>
              <h4 style={{ fontSize: '13px', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '12px' }}>
                Ordered Provider Priority & Failover Route
              </h4>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {m.routes.map((route, idx) => (
                  <div
                    key={route.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '10px 16px',
                      backgroundColor: 'var(--bg-primary)',
                      borderRadius: 'var(--radius-sm)',
                      border: '1px solid var(--border-subtle)',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
                      <span style={{ fontWeight: 700, color: 'var(--accent)', minWidth: '24px' }}>#{idx + 1}</span>
                      <span style={{ fontWeight: 600, textTransform: 'capitalize', color: 'var(--text-primary)' }}>
                        {route.provider_id}
                      </span>
                      <span class="code-inline" style={{ color: 'var(--text-muted)' }}>
                        {route.upstream_model_id}
                      </span>
                    </div>

                    <div style={{ display: 'flex', gap: '4px' }}>
                      <button
                        class="btn btn-secondary btn-sm"
                        disabled={idx === 0}
                        onClick={() => moveRoute(m.id, idx, 'up')}
                        title="Move up priority"
                      >
                        ▲ Up
                      </button>
                      <button
                        class="btn btn-secondary btn-sm"
                        disabled={idx === m.routes.length - 1}
                        onClick={() => moveRoute(m.id, idx, 'down')}
                        title="Move down priority"
                      >
                        ▼ Down
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Provider</th>
                    <th>Hidden Upstream Model ID</th>
                    <th>Input Rate ($/M)</th>
                    <th>Cached Rate ($/M)</th>
                    <th>Output Rate ($/M)</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {m.mappings.map(mp => (
                    <tr key={mp.id}>
                      <td style={{ fontWeight: 600, textTransform: 'capitalize' }}>{mp.provider_id}</td>
                      <td><span class="code-inline">{mp.upstream_model_id}</span></td>
                      <td>${(mp.input_rate_per_m_tokens / 1000000).toFixed(4)}</td>
                      <td>${(mp.cached_rate_per_m_tokens / 1000000).toFixed(4)}</td>
                      <td>${(mp.output_rate_per_m_tokens / 1000000).toFixed(4)}</td>
                      <td style={{ textAlign: 'right' }}>
                        <button class="btn btn-secondary btn-sm" onClick={() => openRateModal(mp)}>
                          Edit Rates
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        ))
      )}

      {/* Edit Rates Modal */}
      {showRateModal && editingMapping && (
        <div class="modal-overlay" onClick={() => setShowRateModal(false)}>
          <div class="modal-content" onClick={e => e.stopPropagation()}>
            <form onSubmit={handleSaveRates}>
              <div class="modal-header">
                <div class="panel-title">Edit Token Rates ({editingMapping.provider_id})</div>
                <button type="button" class="btn btn-secondary btn-sm" onClick={() => setShowRateModal(false)}>✕</button>
              </div>
              <div class="modal-body">
                <div class="form-group">
                  <label class="form-label">Uncached Input Rate ($ per 1 Million tokens)</label>
                  <input
                    type="number"
                    step="0.0001"
                    class="form-input"
                    required
                    value={inputRatePerMUSD}
                    onInput={(e: any) => setInputRatePerMUSD(e.target.value)}
                  />
                </div>
                <div class="form-group">
                  <label class="form-label">Cached Input Rate ($ per 1 Million tokens)</label>
                  <input
                    type="number"
                    step="0.0001"
                    class="form-input"
                    required
                    value={cachedRatePerMUSD}
                    onInput={(e: any) => setCachedRatePerMUSD(e.target.value)}
                  />
                </div>
                <div class="form-group">
                  <label class="form-label">Output Rate ($ per 1 Million tokens)</label>
                  <input
                    type="number"
                    step="0.0001"
                    class="form-input"
                    required
                    value={outputRatePerMUSD}
                    onInput={(e: any) => setOutputRatePerMUSD(e.target.value)}
                  />
                </div>
                <small style={{ color: 'var(--text-muted)', fontSize: '11px' }}>
                  Changes apply prospectively to future attempts. Historical attempt records remain untouched.
                </small>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" onClick={() => setShowRateModal(false)}>Cancel</button>
                <button type="submit" class="btn btn-primary">Save Rates</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
