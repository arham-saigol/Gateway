# Configuration Reference

Arham Gateway loads its settings from `/etc/arham-gateway/config.toml` (or `./config.toml` when running in local development mode).

## Sample `config.toml`

```toml
[server]
# Local listener binding address
listen_addr = "127.0.0.1:8080"
trusted_proxies = ["127.0.0.1"]
max_request_body_bytes = 10485760 # 10MB
read_timeout = "30s"
write_timeout = "0s" # 0s for unbounded streaming completions
idle_timeout = "120s"

[database]
# Path to SQLite database file
path = "/var/lib/arham-gateway/gateway.db"
busy_timeout_ms = 5000
max_open_conns = 1
max_idle_conns = 1

[security]
# Path to 32-byte master encryption key
master_key_path = "/etc/arham-gateway/master.key"
admin_session_expiry = "24h"
cookie_secure = false # Set true if accessing directly over HTTPS without local proxy

[timeouts]
first_response_timeout = "30s"
stream_drain_timeout = "15s"
upstream_dial_timeout = "10s"

[routing]
# Number of retry/failover attempts before returning error
max_retries_per_request = 3
# Cooldown duration for temporarily failing keys (rate limit / 5xx)
key_cooldown_duration = "30s"
# Balance warning threshold in micro-USD (0 = warn at $0.00)
warning_balance_threshold_micro_usd = 0

[retention]
# Detailed request/attempt log retention (days)
detailed_log_days = 30
# Background pruning ticker interval
cleanup_interval = "1h"
```

## Section Details

### `[server]`
- `listen_addr`: IP and port for the HTTP listener. Default is `127.0.0.1:8080` (loopback only).
- `write_timeout`: Maximum duration before timing out writes of the response. Must be `"0s"` to support long-lived AI streams without arbitrary disconnections.

### `[security]`
- `master_key_path`: Absolute path to the master encryption key used to encrypt provider secrets with AES-256-GCM.
- `cookie_secure`: Controls the `Secure` flag on admin session cookies.

### `[routing]`
- `max_retries_per_request`: How many times the gateway will try alternative keys or providers on transient failures before downstream response bytes are committed.
- `key_cooldown_duration`: Period for which a key returning 429 or 5xx is temporarily removed from selection.

### `[retention]`
- `detailed_log_days`: Number of days to keep detailed per-request attempt rows. Note: Daily rollups (`usage_rollups_daily`) and balance adjustment records are **never deleted**, preserving lifetime spend and balance totals across prunes.
