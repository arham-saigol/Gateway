# Dashboard Guide

Arham Gateway includes an embedded Preact SPA administrative dashboard accessible at `http://127.0.0.1:8080` (or via your configured Cloudflare Tunnel domain).

## 1. Authentication & Security

- **Login**: Authenticate using the administrator password set during `gateway setup`.
- **Session Management**: Session tokens are cryptographically generated (32 bytes entropy), stored as SHA-256 hashes in SQLite, and delivered in secure `HttpOnly` `SameSite=Lax` cookies.
- **CSRF Protection**: State-changing requests (`POST`, `PUT`, `DELETE`) require a valid `X-CSRF-Token` header.
- **Password Reset**: If forgotten, reset the password using `gateway password-reset` on the server shell.

## 2. Dashboard Areas

### 📊 Overview
- **Metrics Cards**: 24-hour total requests, 24-hour success rate (%), 24-hour average latency (ms), lifetime known spend ($), and estimated remaining balance ($).
- **Accounts & Balances Table**: Breakdown of all configured provider keys, starting balance, manual adjustments, gateway spend, and remaining balance.

### ☁️ Providers & Keys
- **Add Provider Key**: Securely add API keys for Fireworks AI, SiliconFlow, Novita AI, or Baseten. Secrets are encrypted using AES-256-GCM at rest and never shown in the UI again.
- **Disable / Enable**: Toggle key availability with a single click.
- **Adjust Balance**: Record top-up credit purchases or manual balance corrections with an auditable note.

### 🔀 Models & Routing
- **Public Model Aliases**: View enabled public model aliases (`deepseek-v4-flash`, `deepseek-v4-flash-fast`).
- **Priority Reordering**: Move upstream providers up/down priority with buttons. Priority changes take effect immediately across all subsequent requests without server restart.
- **Edit Token Rates**: Modify prospective token pricing (uncached input, cached input, output) per provider/model mapping.

### 🔑 Gateway API Keys
- **Create Gateway Key**: Issue client keys with prefix `arham_...`. The secret is generated and shown once with a copy button.
- **Revoke Key**: Immediately revoke a compromised or decommissioned key.

### 📈 Analytics
- **Daily Rollup Breakdown**: View aggregate request counts (total, successful, failed), token breakdown (uncached input, cached input, output), and exact calculated costs by date and public model.

### 📋 Request Logs
- **Metadata-Only Requests**: Real-time table of recent completions with duration, TTFT, token counts, and cost.
- **Attempt History Drawer**: Click **Attempts** to see the full failover journey of a request (sequence of providers attempted, durations, HTTP codes, and snapshotted rates). Zero prompt or response contents are ever stored or displayed.

### ⚙️ Settings
- **Password Update**: Change your administrator dashboard password. Automatically revokes all other existing sessions.
- **Runtime Parameters**: Inspect active server binding, timeouts, and retention settings.
