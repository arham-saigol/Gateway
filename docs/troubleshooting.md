# Troubleshooting & Diagnostics

This guide covers common operational issues and diagnostic procedures for Arham Gateway.

---

## 1. Checking Service & Process Health

### Check Service Status
```bash
gateway status
```

### View Live Logs via Journald
```bash
sudo journalctl -u gateway -f -n 100
```

### Verify Readiness Endpoint
```bash
curl -i http://127.0.0.1:8080/readyz
```

---

## 2. Common Issues & Solutions

### A. "Master key not found at /etc/arham-gateway/master.key"
- **Cause**: The master key was not generated or has incorrect file permissions.
- **Fix**: Run `sudo gateway setup` or verify file ownership and mode `0600`:
  ```bash
  sudo ls -l /etc/arham-gateway/master.key
  sudo chmod 0600 /etc/arham-gateway/master.key
  ```

### B. Upstream HTTP 401 / Authentication Failure
- **Behavior**: The gateway automatically classifies the key as `invalid` and removes it from active routing to protect throughput.
- **Fix**: Go to the **Providers & Keys** dashboard view, inspect the error category on the key, and re-add or update the key with valid upstream credentials.

### C. Upstream HTTP 429 / Rate Limited
- **Behavior**: The gateway automatically places the key into a temporary cooldown (default 30s) and fails over to the next healthy key or provider in the route.
- **Fix**: If rate limits persist, add additional keys for the same provider to distribute traffic across accounts via round-robin.

### D. Cloudflare Tunnel Returns 502 Bad Gateway
- **Cause**: The gateway service is stopped or cloudflared cannot connect to `http://localhost:8080`.
- **Fix**:
  1. Check if the gateway is running: `gateway status`.
  2. In Cloudflare Tunnel configuration, verify the service URL is set to `http://localhost:8080` (or `http://127.0.0.1:8080`) with `HTTP2` enabled.

### E. Forgotten Admin Dashboard Password
- **Fix**: Run `sudo gateway password-reset` on the server terminal to securely reset the password without server downtime.
