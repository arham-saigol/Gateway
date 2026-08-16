import { useEffect, useState } from 'preact/hooks';
import { apiRequest, setCSRFToken } from './api';
import { OverviewView } from './views/OverviewView';
import { ProvidersView } from './views/ProvidersView';
import { ModelsView } from './views/ModelsView';
import { GatewayKeysView } from './views/GatewayKeysView';
import { AnalyticsView } from './views/AnalyticsView';
import { RequestLogsView } from './views/RequestLogsView';
import { SettingsView } from './views/SettingsView';

type Tab = 'overview' | 'providers' | 'models' | 'gateway-keys' | 'analytics' | 'logs' | 'settings';

export function App() {
  const [authenticated, setAuthenticated] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [activeTab, setActiveTab] = useState<Tab>('overview');

  // Login form
  const [loginPassword, setLoginPassword] = useState('');
  const [loginError, setLoginError] = useState('');
  const [loggingIn, setLoggingIn] = useState(false);

  useEffect(() => {
    checkAuth();
  }, []);

  async function checkAuth() {
    try {
      setCheckingAuth(true);
      const res = await apiRequest<{ authenticated: boolean; csrf_token: string }>('/api/auth/me');
      if (res.authenticated) {
        setCSRFToken(res.csrf_token);
        setAuthenticated(true);
      }
    } catch {
      setAuthenticated(false);
    } finally {
      setCheckingAuth(false);
    }
  }

  async function handleLogin(e: any) {
    e.preventDefault();
    setLoggingIn(true);
    setLoginError('');
    try {
      const res = await apiRequest<{ status: string; csrf_token: string }>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ password: loginPassword }),
      });
      setCSRFToken(res.csrf_token);
      setAuthenticated(true);
      setLoginPassword('');
    } catch (err: any) {
      setLoginError(err.message || 'Invalid administrator password');
    } finally {
      setLoggingIn(false);
    }
  }

  async function handleLogout() {
    try {
      await apiRequest('/api/auth/logout', { method: 'POST' });
    } catch {}
    setAuthenticated(false);
    setCSRFToken('');
  }

  if (checkingAuth) {
    return (
      <div style={{ display: 'flex', height: '100vh', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--bg-primary)', color: 'var(--text-muted)' }}>
        Loading Arham Gateway...
      </div>
    );
  }

  if (!authenticated) {
    return (
      <div style={{ display: 'flex', minHeight: '100vh', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--bg-primary)', padding: '20px' }}>
        <div style={{ width: '100%', maxWidth: '380px', backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--radius-lg)', padding: '32px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '24px' }}>
            <div class="sidebar-logo">AG</div>
            <div>
              <h1 style={{ fontSize: '18px', fontWeight: 700, letterSpacing: '-0.3px' }}>Arham Gateway</h1>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Sign in to manage gateway & models</p>
            </div>
          </div>

          <form onSubmit={handleLogin}>
            {loginError && (
              <div style={{ padding: '10px 14px', backgroundColor: 'var(--danger-bg)', color: 'var(--danger)', borderRadius: 'var(--radius-sm)', fontSize: '13px', marginBottom: '16px' }}>
                {loginError}
              </div>
            )}

            <div class="form-group">
              <label class="form-label">Admin Password</label>
              <input
                type="password"
                class="form-input"
                required
                autoFocus
                placeholder="Enter password"
                value={loginPassword}
                onInput={(e: any) => setLoginPassword(e.target.value)}
              />
            </div>

            <button type="submit" class="btn btn-primary" style={{ width: '100%', marginTop: '12px', padding: '10px' }} disabled={loggingIn}>
              {loggingIn ? 'Authenticating...' : 'Sign In'}
            </button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div class="app-container">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="sidebar-logo">AG</div>
          <div>
            <div class="sidebar-title">Arham Gateway</div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Self-Hosted Gateway</div>
          </div>
        </div>

        <ul class="nav-list">
          <li class={`nav-item ${activeTab === 'overview' ? 'active' : ''}`} onClick={() => setActiveTab('overview')}>
            <span>📊</span> Overview
          </li>
          <li class={`nav-item ${activeTab === 'providers' ? 'active' : ''}`} onClick={() => setActiveTab('providers')}>
            <span>☁️</span> Providers & Keys
          </li>
          <li class={`nav-item ${activeTab === 'models' ? 'active' : ''}`} onClick={() => setActiveTab('models')}>
            <span>🔀</span> Models & Routing
          </li>
          <li class={`nav-item ${activeTab === 'gateway-keys' ? 'active' : ''}`} onClick={() => setActiveTab('gateway-keys')}>
            <span>🔑</span> Gateway API Keys
          </li>
          <li class={`nav-item ${activeTab === 'analytics' ? 'active' : ''}`} onClick={() => setActiveTab('analytics')}>
            <span>📈</span> Analytics
          </li>
          <li class={`nav-item ${activeTab === 'logs' ? 'active' : ''}`} onClick={() => setActiveTab('logs')}>
            <span>📋</span> Request Logs
          </li>
          <li class={`nav-item ${activeTab === 'settings' ? 'active' : ''}`} onClick={() => setActiveTab('settings')}>
            <span>⚙️</span> Settings
          </li>
        </ul>

        <div class="sidebar-footer">
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <span style={{ display: 'inline-block', width: '8px', height: '8px', borderRadius: '50%', backgroundColor: 'var(--success)' }}></span>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>Online</span>
          </div>
          <button class="btn btn-secondary btn-sm" onClick={handleLogout}>Logout</button>
        </div>
      </aside>

      <main class="main-content">
        <header class="topbar">
          <div class="topbar-title" style={{ textTransform: 'capitalize' }}>
            {activeTab.replace('-', ' ')}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <span class="badge badge-success">v1.0.0</span>
          </div>
        </header>

        <div class="content-body">
          {activeTab === 'overview' && <OverviewView />}
          {activeTab === 'providers' && <ProvidersView />}
          {activeTab === 'models' && <ModelsView />}
          {activeTab === 'gateway-keys' && <GatewayKeysView />}
          {activeTab === 'analytics' && <AnalyticsView />}
          {activeTab === 'logs' && <RequestLogsView />}
          {activeTab === 'settings' && <SettingsView />}
        </div>
      </main>
    </div>
  );
}
