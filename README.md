# Arham Gateway

Fast, resource-efficient, single-user, self-hosted OpenAI-compatible model gateway designed for Ubuntu GCP VMs and private deployments.

Clients authenticate with Arham Gateway API keys and see only Arham's public model IDs; provider identities, credentials, upstream model IDs, routing, retries, and errors remain strictly private.

## Supported Public Models

- `deepseek-v4-flash`: Primary balanced model (Fireworks AI → SiliconFlow → Novita AI → Baseten)
- `deepseek-v4-flash-fast`: Low-latency optimized alias (Baseten → Novita AI → SiliconFlow → Fireworks AI)

## Supported Upstream Providers

- **Fireworks AI**
- **SiliconFlow**
- **Novita AI**
- **Baseten**

## Features

- **Single Static Binary**: Go binary with embedded SQLite migrations, admin API, CLI, and Preact SPA dashboard.
- **Provider-Neutral Proxy**: Strips private upstream headers, models, request IDs, and provider identifiers from public API responses.
- **Resilient Multi-Provider Routing**: Configurable priority ordering, round-robin key selection within providers, automatic cooldowns, and pre-commit failover.
- **Exact Integer Accounting**: Rate snapshotting per attempt and micro-USD fixed-precision token accounting (uncached input, cached input, output).
- **Privacy & Security**: Authenticated AES-256-GCM encryption of provider secrets at rest, Argon2id password hashing, hashed gateway API keys, zero prompt/response logging.
- **Built-in Web Dashboard**: Responsive Preact SPA for live spend overview, provider keys, route reordering, rate editing, and request logs.
- **Cloudflare Tunnel Ready**: Binds to `127.0.0.1:8080` by default; publish securely without exposing open inbound VM ports.

## Quick Start

### 1. Build and Setup

```bash
# Build the binary
go build -o gateway ./cmd/gateway

# Run setup (creates master key, migrates database, and sets admin password)
sudo ./gateway setup
```

### 2. Start the Service

```bash
# Start via systemd (on Linux)
sudo gateway start

# Or run in foreground
./gateway serve
```

### 3. Open the Dashboard

Navigate to `http://127.0.0.1:8080` and log in with your dashboard admin password.

### 4. Create a Gateway API Key & Query

In the dashboard, generate a new Gateway API Key (e.g. `arham_...`), then send OpenAI-compatible chat completion requests:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer <your-arham-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "Hello Arham Gateway!"}],
    "stream": true
  }'
```

## Documentation

- [Installation Guide](docs/installation.md)
- [Cloudflare Tunnel Setup](docs/cloudflare-tunnel.md)
- [Configuration Reference](docs/configuration.md)
- [Dashboard Guide](docs/dashboard.md)
- [API Reference](docs/api-reference.md)
- [Provider Setup & Verification](docs/providers.md)
- [Metrics & Cost Formulas](docs/metrics-and-costs.md)
- [Backup & Restore Drills](docs/backup-restore.md)
- [Upgrading & Rollbacks](docs/upgrading.md)
- [Security Model](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md)

## License

MIT
