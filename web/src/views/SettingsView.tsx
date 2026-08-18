import { useEffect, useState } from 'preact/hooks';
import { apiRequest } from '../api';
import { IconLock, IconShield, IconZap, IconCheck } from '../components/Icons';

interface SettingsData {
  listen_addr: string;
  first_response_timeout_seconds: number;
  stream_drain_timeout_seconds: number;
  detailed_log_days: number;
  max_retries: number;
  key_cooldown_seconds: number;
}

export function SettingsView() {
  const [settings, setSettings] = useState<SettingsData | null>(null);

  // Password change
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passSuccess, setPassSuccess] = useState('');
  const [passError, setPassError] = useState('');
  const [savingPassword, setSavingPassword] = useState(false);

  useEffect(() => {
    loadSettings();
  }, []);

  async function loadSettings() {
    try {
      const res = await apiRequest<SettingsData>('/api/settings');
      setSettings(res);
    } catch {}
  }

  async function handlePasswordChange(e: any) {
    e.preventDefault();
    setPassSuccess('');
    setPassError('');

    if (newPassword !== confirmPassword) {
      setPassError('New passwords do not match');
      return;
    }

    setSavingPassword(true);
    try {
      await apiRequest('/api/auth/password', {
        method: 'POST',
        body: JSON.stringify({
          old_password: oldPassword,
          new_password: newPassword,
        }),
      });

      setPassSuccess('Dashboard password updated successfully! (All other active sessions have been revoked)');
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: any) {
      setPassError(err.message || 'Failed to update password');
    } finally {
      setSavingPassword(false);
    }
  }

  return (
    <div>
      <div class="page-header">
        <div>
          <h1 class="page-title">Settings & Security</h1>
          <p class="page-subtitle">Manage administrator credentials, service parameters, and security policies.</p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '24px', alignItems: 'start' }}>
        {/* Security & Password */}
        <div class="panel">
          <div class="panel-header">
            <div class="panel-title">
              <IconLock size={16} />
              <span>Change Master Password</span>
            </div>
          </div>
          <div class="panel-body">
            <form onSubmit={handlePasswordChange}>
              {passSuccess && (
                <div style={{ padding: '10px 14px', backgroundColor: 'var(--success-bg)', border: '1px solid var(--success-border)', color: 'var(--success)', borderRadius: 'var(--radius-sm)', marginBottom: '16px', fontSize: '12.5px', display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <IconCheck size={14} />
                  <span>{passSuccess}</span>
                </div>
              )}
              {passError && (
                <div style={{ padding: '10px 14px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', color: 'var(--danger)', borderRadius: 'var(--radius-sm)', marginBottom: '16px', fontSize: '12.5px' }}>
                  {passError}
                </div>
              )}

              <div class="form-group">
                <label class="form-label">Current Master Password</label>
                <input
                  type="password"
                  class="form-input"
                  required
                  placeholder="Enter current password"
                  value={oldPassword}
                  onInput={(e: any) => setOldPassword(e.target.value)}
                />
              </div>

              <div class="form-group">
                <label class="form-label">New Password</label>
                <input
                  type="password"
                  class="form-input"
                  required
                  placeholder="Enter new password"
                  value={newPassword}
                  onInput={(e: any) => setNewPassword(e.target.value)}
                />
              </div>

              <div class="form-group">
                <label class="form-label">Confirm New Password</label>
                <input
                  type="password"
                  class="form-input"
                  required
                  placeholder="Re-enter new password"
                  value={confirmPassword}
                  onInput={(e: any) => setConfirmPassword(e.target.value)}
                />
              </div>

              <div style={{ marginTop: '20px' }}>
                <button type="submit" class="btn btn-primary" disabled={savingPassword}>
                  {savingPassword ? 'Updating Password...' : 'Update Password'}
                </button>
              </div>
            </form>
          </div>
        </div>

        {/* Runtime Parameters */}
        <div class="panel">
          <div class="panel-header">
            <div class="panel-title">
              <IconShield size={16} />
              <span>System Runtime Parameters</span>
            </div>
          </div>
          <div class="panel-body">
            {settings ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>Server Binding Address</div>
                    <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>TCP interface and port</div>
                  </div>
                  <strong class="code-inline">{settings.listen_addr}</strong>
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>First Response Timeout</div>
                    <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>Max wait time before failover</div>
                  </div>
                  <span class="code-inline">{settings.first_response_timeout_seconds}s</span>
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>Stream Drain Timeout</div>
                    <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>Max inactive stream duration</div>
                  </div>
                  <span class="code-inline">{settings.stream_drain_timeout_seconds}s</span>
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>Max Provider Retries</div>
                    <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>Failover attempts per request</div>
                  </div>
                  <span class="code-inline">{settings.max_retries} attempts</span>
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>Key Cooldown Period</div>
                    <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>Temporary backoff after 429</div>
                  </div>
                  <span class="code-inline">{settings.key_cooldown_seconds}s</span>
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>Request Log Retention</div>
                    <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>Detailed attempts retention</div>
                  </div>
                  <span class="code-inline">{settings.detailed_log_days} days</span>
                </div>
              </div>
            ) : (
              <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '24px' }}>
                <IconZap size={20} class="text-muted" style={{ animation: 'spin 1.5s linear infinite', margin: '0 auto 8px' }} />
                <div>Loading parameters...</div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
