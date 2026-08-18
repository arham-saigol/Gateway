import { useEffect, useState } from 'preact/hooks';
import { apiRequest } from '../api';
import type { GatewayKey } from '../types';
import { IconPlus, IconKey, IconShield, IconClock, IconTrash, IconZap } from '../components/Icons';
import { Badge, CopyButton, Modal, Callout, EmptyState } from '../components/UI';

export function GatewayKeysView() {
  const [keys, setKeys] = useState<GatewayKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [createdSecret, setCreatedSecret] = useState('');
  const [creatingKey, setCreatingKey] = useState(false);

  useEffect(() => {
    loadKeys();
  }, []);

  async function loadKeys() {
    try {
      setLoading(true);
      const res = await apiRequest<GatewayKey[]>('/api/gateway-keys');
      setKeys(res || []);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Failed to load gateway keys');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreateKey(e: any) {
    e.preventDefault();
    setCreatingKey(true);
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
    } finally {
      setCreatingKey(false);
    }
  }

  async function handleRevokeKey(keyId: string, name: string) {
    if (!confirm(`Are you sure you want to revoke "${name}"? Client applications using this key will immediately receive 401 Unauthorized.`)) {
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

  function handleCloseCreateModal() {
    setShowCreateModal(false);
    setCreatedSecret('');
  }

  return (
    <div>
      <div class="page-header">
        <div>
          <h1 class="page-title">Gateway API Keys</h1>
          <p class="page-subtitle">Generate client bearer keys for apps to authenticate against OpenAI-compatible endpoints.</p>
        </div>
        <button class="btn btn-primary" onClick={() => { setCreatedSecret(''); setShowCreateModal(true); }}>
          <IconPlus size={14} />
          <span>Create Gateway Key</span>
        </button>
      </div>

      <Callout>
        <strong>OpenAI-Compatible Seam:</strong> Pass your gateway keys via <span class="code-inline">Authorization: Bearer &lt;key&gt;</span> to <span class="code-inline">/v1/chat/completions</span> or <span class="code-inline">/v1/models</span>.
      </Callout>

      {error && (
        <div style={{ padding: '14px 18px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius-md)', color: 'var(--danger)', marginBottom: '20px' }}>
          {error}
        </div>
      )}

      <div class="panel">
        <div class="panel-header">
          <div class="panel-title">
            <IconKey size={16} />
            <span>Active Client API Keys</span>
          </div>
          <Badge variant="neutral" withDot={false}>
            {keys.filter(k => k.status === 'active').length} Active
          </Badge>
        </div>

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
              {loading && keys.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
                    <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 8px' }} />
                    <div>Loading keys...</div>
                  </td>
                </tr>
              ) : keys.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ padding: 0 }}>
                    <EmptyState
                      icon={<IconKey size={24} />}
                      title="No gateway keys created yet"
                      description="Create an API key to allow your applications to query models through the gateway."
                      action={
                        <button class="btn btn-primary btn-sm" onClick={() => { setCreatedSecret(''); setShowCreateModal(true); }}>
                          <IconPlus size={13} />
                          <span>Create Gateway Key</span>
                        </button>
                      }
                    />
                  </td>
                </tr>
              ) : (
                keys.map(k => (
                  <tr key={k.id}>
                    <td>
                      <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{k.name}</div>
                    </td>
                    <td>
                      <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                        <span class="code-inline">{k.key_prefix}</span>
                        <CopyButton text={k.key_prefix} size="xs" />
                      </div>
                    </td>
                    <td>
                      <Badge variant={k.status === 'active' ? 'success' : 'danger'}>
                        {k.status}
                      </Badge>
                    </td>
                    <td style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                      {new Date(k.created_at).toLocaleDateString()} {new Date(k.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
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
                      {k.status === 'active' ? (
                        <button
                          class="btn btn-danger-subtle btn-sm"
                          onClick={() => handleRevokeKey(k.id, k.name)}
                          title="Revoke key"
                        >
                          <IconTrash size={12} />
                          <span>Revoke</span>
                        </button>
                      ) : (
                        <span style={{ fontSize: '12px', color: 'var(--text-subtle)' }}>Revoked</span>
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
      <Modal
        isOpen={showCreateModal}
        onClose={handleCloseCreateModal}
        title={createdSecret ? 'Gateway Key Generated' : 'Create Gateway API Key'}
        footer={
          createdSecret ? (
            <button type="button" class="btn btn-primary" onClick={handleCloseCreateModal}>
              I have safely copied my key
            </button>
          ) : (
            <>
              <button type="button" class="btn btn-ghost" onClick={handleCloseCreateModal}>
                Cancel
              </button>
              <button type="submit" form="create-key-form" class="btn btn-primary" disabled={creatingKey}>
                {creatingKey ? 'Creating...' : 'Create Key'}
              </button>
            </>
          )
        }
      >
        {createdSecret ? (
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--success)', marginBottom: '12px', fontSize: '13px', fontWeight: 600 }}>
              <IconShield size={16} />
              <span>Key successfully created!</span>
            </div>

            <p style={{ fontSize: '12.5px', color: 'var(--text-secondary)', marginBottom: '16px', lineHeight: 1.5 }}>
              Please copy your secret API key below and store it securely. For security reasons, <strong>you will not be able to view this key again</strong>.
            </p>

            <div class="key-reveal-box">
              <div class="key-secret-text">{createdSecret}</div>
              <CopyButton text={createdSecret} label="Copy" size="sm" />
            </div>
          </div>
        ) : (
          <form id="create-key-form" onSubmit={handleCreateKey}>
            <div class="form-group">
              <label class="form-label">Key Name / Client Description</label>
              <input
                type="text"
                class="form-input"
                required
                autoFocus
                placeholder="e.g. Next.js Production App or Arham CLI"
                value={keyName}
                onInput={(e: any) => setKeyName(e.target.value)}
              />
            </div>
            <p style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Keys are generated with cryptographically secure random bytes and hashed with SHA-256 for fast, secure lookup.
            </p>
          </form>
        )}
      </Modal>
    </div>
  );
}
