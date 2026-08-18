import { useEffect, useState } from 'preact/hooks';
import { apiRequest, setCSRFToken } from './api';
import { OverviewView } from './views/OverviewView';
import { ProvidersView } from './views/ProvidersView';
import { ModelsView } from './views/ModelsView';
import { GatewayKeysView } from './views/GatewayKeysView';
import { AnalyticsView } from './views/AnalyticsView';
import { RequestLogsView } from './views/RequestLogsView';
import { SettingsView } from './views/SettingsView';
import {
  IconOverview,
  IconCloud,
  IconModels,
  IconKey,
  IconAnalytics,
  IconLogs,
  IconSettings,
  IconLogOut,
  IconZap,
  IconShield,
} from './components/Icons';
import { Badge } from './components/UI';

type Tab = 'overview' | 'providers' | 'models' | 'gateway-keys' | 'analytics' | 'logs' | 'settings';

interface TabConfig {
  id: Tab;
  label: string;
  icon: any;
}

const TABS: TabConfig[] = [
  { id: 'overview', label: 'Overview', icon: IconOverview },
  { id: 'providers', label: 'Providers & Keys', icon: IconCloud },
  { id: 'models', label: 'Models & Routing', icon: IconModels },
  { id: 'gateway-keys', label: 'Gateway API Keys', icon: IconKey },
  { id: 'analytics', label: 'Analytics', icon: IconAnalytics },
  { id: 'logs', label: 'Request Logs', icon: IconLogs },
  { id: 'settings', label: 'Settings', icon: IconSettings },
];

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
      <div style={{ display: 'flex', height: '100vh', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--bg-canvas)', color: 'var(--text-muted)', gap: '10px' }}>
        <IconZap size={20} class="text-white" />
        <span style={{ fontSize: '13px', fontWeight: 500 }}>Connecting to Arham Gateway...</span>
      </div>
    );
  }

  if (!authenticated) {
    return (
      <div style={{ display: 'flex', minHeight: '100vh', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--bg-canvas)', padding: '20px' }}>
        <div style={{ width: '100%', maxWidth: '380px', backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border-strong)', borderRadius: 'var(--radius-xl)', padding: '32px', boxShadow: 'var(--shadow-lg)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '24px' }}>
            <div class="workspace-icon" style={{ width: '36px', height: '36px', borderRadius: '8px', fontSize: '15px' }}>
              <IconZap size={20} />
            </div>
            <div>
              <h1 style={{ fontSize: '16px', fontWeight: 600, letterSpacing: '-0.02em', color: '#ffffff' }}>Arham Gateway</h1>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Self-Hosted AI Model Proxy</p>
            </div>
          </div>

          <form onSubmit={handleLogin}>
            {loginError && (
              <div style={{ padding: '10px 12px', backgroundColor: 'var(--danger-bg)', border: '1px solid var(--danger-border)', color: 'var(--danger)', borderRadius: 'var(--radius-sm)', fontSize: '12.5px', marginBottom: '16px' }}>
                {loginError}
              </div>
            )}

            <div class="form-group">
              <label class="form-label">
                <span>Master Password</span>
                <IconShield size={13} class="text-muted" />
              </label>
              <input
                type="password"
                class="form-input"
                required
                autoFocus
                placeholder="Enter admin password"
                value={loginPassword}
                onInput={(e: any) => setLoginPassword(e.target.value)}
              />
            </div>

            <button type="submit" class="btn btn-primary" style={{ width: '100%', marginTop: '12px', padding: '9px 14px' }} disabled={loggingIn}>
              {loggingIn ? 'Authenticating...' : 'Sign In'}
            </button>
          </form>

          <div style={{ marginTop: '24px', paddingTop: '16px', borderTop: '1px solid var(--border-subtle)', textAlign: 'center', fontSize: '11.5px', color: 'var(--text-subtle)' }}>
            Encrypted SQLite · Zero-Trust Key Storage
          </div>
        </div>
      </div>
    );
  }

  const currentTab = TABS.find(t => t.id === activeTab) || TABS[0];

  return (
    <div class="app-container">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="workspace-badge">
            <div class="workspace-icon">
              <IconZap size={15} />
            </div>
            <div class="workspace-info">
              <div class="workspace-name">Arham Gateway</div>
              <div class="workspace-env">
                <span>production</span>
                <span>•</span>
                <span>v1.0.0</span>
              </div>
            </div>
          </div>
        </div>

        <div class="nav-section">Dashboard</div>
        <ul class="nav-list">
          {TABS.map(tab => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <li
                key={tab.id}
                class={`nav-item ${isActive ? 'active' : ''}`}
                onClick={() => setActiveTab(tab.id)}
              >
                <Icon size={16} />
                <span>{tab.label}</span>
              </li>
            );
          })}
        </ul>

        <div class="sidebar-footer">
          <div class="status-indicator">
            <span class="status-dot-pulse" />
            <span>Online</span>
          </div>
          <button class="btn btn-ghost btn-sm" onClick={handleLogout} title="Sign Out">
            <IconLogOut size={14} />
            <span>Logout</span>
          </button>
        </div>
      </aside>

      <main class="main-content">
        <header class="topbar">
          <div class="breadcrumbs">
            <div class="breadcrumb-root">
              <IconZap size={14} />
              <span>Gateway</span>
            </div>
            <span class="breadcrumb-separator">/</span>
            <div class="breadcrumb-current">{currentTab.label}</div>
          </div>

          <div class="topbar-actions">
            <Badge variant="success" withDot>Ready</Badge>
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
