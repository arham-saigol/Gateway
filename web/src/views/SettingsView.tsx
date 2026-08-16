import { useEffect, useState } from 'preact/hooks';
import { apiRequest } from '../api';

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

    try {
      await apiRequest('/api/auth/password', {
        method: 'POST',
        body: JSON.stringify({
          old_password: oldPassword,
          new_password: newPassword,
        }),
      });

      setPassSuccess('Dashboard password updated successfully! (Other sessions have been revoked)');
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: any) {
      setPassError(err.message || 'Failed to update password');
    }
  }

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: 600 }}>Gateway Settings & Security</h2>
        <p style={{ color: 'var(--text-secondary)', fontSize: '13px' }}>
          Manage administrator credentials, service parameters, and security policies.
        </p>
      </div>

      <div class="panel" style={{ maxWidth: '600px' }}>
        <div class="panel-header">
          <span class="panel-title">Change Dashboard Password</span>
        </div>
        <div style={{ padding: '24px' }}>
          <form onSubmit={handlePasswordChange}>
            {passSuccess && (
              <div style={{ padding: '12px', backgroundColor: 'var(--success-bg)', color: 'var(--success)', borderRadius: 'var(--radius-sm)', marginBottom: '16px' }}>
                {passSuccess}
              </div>
            )}
            {passError && (
              <div style={{ padding: '12px', backgroundColor: 'var(--danger-bg)', color: 'var(--danger)', borderRadius: 'var(--radius-sm)', marginBottom: '16px' }}>
                {passError}
              </div>
            )}

            <div class="form-group">
              <label class="form-label">Current Password</label>
              <input
                type="password"
                class="form-input"
                required
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
                value={confirmPassword}
                onInput={(e: any) => setConfirmPassword(e.target.value)}
              />
            </div>

            <div style={{ marginTop: '20px' }}>
              <button type="submit" class="btn btn-primary">Update Password</button>
            </div>
          </form>
        </div>
      </div>

      <div class="panel" style={{ maxWidth: '600px' }}>
        <div class="panel-header">
          <span class="panel-title">System Runtime Parameters</span>
        </div>
        <div style={{ padding: '24px' }}>
          {settings ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--border-subtle)' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Server Binding</span>
                <strong class="code-inline">{settings.listen_addr}</strong>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--border-subtle)' }}>
                <span style={{ color: 'var(--text-secondary)' }}>First Response Timeout</span>
                <strong>{settings.first_response_timeout_seconds}s</strong>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--border-subtle)' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Stream Drain Timeout</span>
                <strong>{settings.stream_drain_timeout_seconds}s</strong>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--border-subtle)' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Max Provider Retries</span>
                <strong>{settings.max_retries}</strong>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--border-subtle)' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Key Cooldown Duration</span>
                <strong>{settings.key_cooldown_seconds}s</strong>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Detailed Log Retention</span>
                <strong>{settings.detailed_log_days} days (Rollups kept forever)</strong>
              </div>
            </div>
          ) : (
            <div style={{ color: 'var(--text-muted)' }}>Loading parameters...</div>
          )}
        </div>
      </div>
    </div>
  );
}
