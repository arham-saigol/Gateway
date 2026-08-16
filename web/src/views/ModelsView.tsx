import { useEffect, useState } from 'preact/hooks';
import { apiRequest } from '../api';
import type { PublicModelWithRoutes, ProviderModelMapping } from '../types';
import { IconModels, IconChevronUp, IconChevronDown, IconEdit, IconZap } from '../components/Icons';
import { Badge, Modal, Callout, EmptyState } from '../components/UI';

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
  const [savingRate, setSavingRate] = useState(false);

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
    setSavingRate(true);

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
    } finally {
      setSavingRate(false);
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
      <div class="page-header">
        <div>
          <h1 class="page-title">Models & Routing</h1>
          <p class="page-subtitle">Configure public virtual model aliases, automatic failover routes, and token pricing.</p>
        </div>
      </div>

      <Callout>
        <strong>Zero Downtime Failover:</strong> When a request encounters a rate-limit (429), provider timeout, or internal error (5xx),
        the gateway automatically retries with the next prioritized provider in the routing chain.
      </Callout>

      {error && (
        <div style={{ padding: '14px 18px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-md)', color: 'var(--danger)', marginBottom: '20px' }}>
          {error}
        </div>
      )}

      {loading && models.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '48px', color: 'var(--text-muted)' }}>
          <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 12px' }} />
          <div>Loading models & routes...</div>
        </div>
      ) : models.length === 0 ? (
        <EmptyState
          icon={<IconModels size={28} />}
          title="No models configured"
          description="Models will appear here once initialized in the gateway."
        />
      ) : (
        models.map(m => (
          <div class="panel" key={m.id}>
            <div class="panel-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <span class="panel-title">{m.display_name}</span>
                <span class="code-inline">{m.id}</span>
              </div>
              <Badge variant="success">Active Public Alias</Badge>
            </div>

            <div style={{ padding: '18px 20px', borderBottom: '1px solid var(--border-subtle)' }}>
              <div style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: '12px' }}>
                Failover Routing Hierarchy
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {m.routes.map((route, idx) => (
                  <div key={route.id} class="priority-row">
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                      <span class={`priority-rank-badge ${idx === 0 ? 'primary' : ''}`}>
                        {idx === 0 ? 'PRIORITY 1' : `FAILOVER #${idx + 1}`}
                      </span>
                      <span style={{ fontWeight: 600, textTransform: 'capitalize', color: '#ffffff', fontSize: '13px' }}>
                        {route.provider_id}
                      </span>
                      <span class="code-inline" style={{ color: 'var(--text-secondary)' }}>
                        {route.upstream_model_id}
                      </span>
                    </div>

                    <div style={{ display: 'flex', gap: '4px' }}>
                      <button
                        class="btn btn-secondary btn-icon"
                        disabled={idx === 0}
                        onClick={() => moveRoute(m.id, idx, 'up')}
                        title="Increase priority"
                      >
                        <IconChevronUp size={14} />
                      </button>
                      <button
                        class="btn btn-secondary btn-icon"
                        disabled={idx === m.routes.length - 1}
                        onClick={() => moveRoute(m.id, idx, 'down')}
                        title="Decrease priority"
                      >
                        <IconChevronDown size={14} />
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
                    <th>Upstream Model Identifier</th>
                    <th>Input Rate ($/1M)</th>
                    <th>Cached Rate ($/1M)</th>
                    <th>Output Rate ($/1M)</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {m.mappings.map(mp => (
                    <tr key={mp.id}>
                      <td style={{ fontWeight: 600, textTransform: 'capitalize', color: 'var(--text-primary)' }}>
                        {mp.provider_id}
                      </td>
                      <td><span class="code-inline">{mp.upstream_model_id}</span></td>
                      <td style={{ fontFamily: 'var(--font-mono)' }}>${(mp.input_rate_per_m_tokens / 1000000).toFixed(4)}</td>
                      <td style={{ fontFamily: 'var(--font-mono)' }}>${(mp.cached_rate_per_m_tokens / 1000000).toFixed(4)}</td>
                      <td style={{ fontFamily: 'var(--font-mono)' }}>${(mp.output_rate_per_m_tokens / 1000000).toFixed(4)}</td>
                      <td style={{ textAlign: 'right' }}>
                        <button class="btn btn-secondary btn-sm" onClick={() => openRateModal(mp)}>
                          <IconEdit size={12} />
                          <span>Edit Rates</span>
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
      <Modal
        isOpen={showRateModal && Boolean(editingMapping)}
        onClose={() => setShowRateModal(false)}
        title={`Edit Token Rates: ${editingMapping?.provider_id || ''}`}
        footer={
          <>
            <button type="button" class="btn btn-ghost" onClick={() => setShowRateModal(false)}>
              Cancel
            </button>
            <button type="submit" form="edit-rates-form" class="btn btn-primary" disabled={savingRate}>
              {savingRate ? 'Saving...' : 'Save Rates'}
            </button>
          </>
        }
      >
        <form id="edit-rates-form" onSubmit={handleSaveRates}>
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
          <div style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '8px' }}>
            Rate adjustments apply immediately to all incoming requests and token spend calculation.
          </div>
        </form>
      </Modal>
    </div>
  );
}
