# Publishing via Cloudflare Tunnel

Arham Gateway binds by default to loopback (`127.0.0.1:8080`). To expose the gateway publicly over HTTPS without opening inbound firewall ports on your GCP VM, use Cloudflare Tunnel.

## Why Cloudflare Tunnel?

1. **Zero Open Ports**: No public IP or inbound firewall rules needed on GCP.
2. **Automatic TLS / HTTPS**: Valid SSL certificates handled automatically by Cloudflare.
3. **SSE Support**: Seamless real-time streaming with zero application buffer lag.
4. **Access Control**: Optional Cloudflare Access rules (e.g. SSO, MFA) on the `/admin` path while keeping `/v1/*` open for API keys.

## Step 1: Install Cloudflared on Ubuntu

```bash
curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i cloudflared.deb
```

## Step 2: Create a Cloudflare Tunnel

In the [Cloudflare Zero Trust Dashboard](https://one.dash.cloudflare.com/):

1. Go to **Networks** → **Tunnels** → **Create a Tunnel**.
2. Select **Cloudflared** as the connector type and name your tunnel (e.g. `arham-gateway-vm`).
3. Copy the installation token command and run it on your VM:
   ```bash
   sudo cloudflared service install <your-tunnel-token>
   ```

## Step 3: Configure Public Hostname Route

In the Cloudflare dashboard under your tunnel's **Public Hostnames** tab:

- **Public Hostname**: `gateway.yourdomain.com` (or `api.yourdomain.com`)
- **Service**:
  - **Type**: `HTTP`
  - **URL**: `http://localhost:8080` (or `http://127.0.0.1:8080`)
- Under **Additional application settings** → **HTTP Settings**:
  - **No TLS Verify**: Enabled (since local connection is plain HTTP).
  - **HTTP2 / Chunked Transfer**: Enabled (essential for SSE streaming).

## Step 4: Verify Public Access

### 1. Test Healthz
```bash
curl https://gateway.yourdomain.com/healthz
# Response: {"status":"ok"}
```

### 2. Test Streaming Chat Completion
```bash
curl https://gateway.yourdomain.com/v1/chat/completions \
  -H "Authorization: Bearer <your-arham-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "Tell me a joke"}],
    "stream": true
  }'
```
