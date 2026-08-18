# Security & Privacy Architecture

This document details the security model, encryption standards, threat boundaries, and privacy guarantees implemented in Arham Gateway.

---

## 1. Zero Prompt & Response Persistence

- **Zero Content Logging**: The gateway **never** writes prompts, messages, tool calls, or completion text to disk, application logs, or the database.
- **Metadata Only**: Only bounded operational metadata (model alias, token counts, request duration, TTFT, calculated costs, and HTTP status codes) are retained.
- **Redacted Logging**: The application structured logger filters and redacts authorization headers, API keys, passwords, bearer tokens, and URLs before writing to stdout / journald.

---

## 2. Cryptographic Secret Protection

### Upstream Provider API Keys
- Encrypted using **AES-256-GCM** with authenticated 12-byte cryptographically secure random nonces (`crypto/rand`).
- The 32-byte master encryption key is kept separately outside the database at `/etc/arham-gateway/master.key` (mode `0600`).
- The web dashboard and admin API never echo or return provider secrets after initial submission.

### Dashboard Administrator Password
- Hashed with **Argon2id** (`golang.org/x/crypto/argon2`) using memory-hard parameters ($64\text{MB}$ RAM, 3 iterations, 2 threads, 16-byte random salt).
- Constant-time secret verification (`crypto/subtle.ConstantTimeCompare`) prevents timing side-channel attacks.

### Gateway API Keys
- Client tokens (`arham_...`) are generated with 192 bits of cryptographic entropy.
- Keys are hashed with **SHA-256** in SQLite.
- The raw plaintext secret is shown only once at creation time and cannot be recovered from the database.

---

## 3. Network & Proxy Hardening

- **Loopback Default**: Default binding is `127.0.0.1:8080`.
- **Cloudflare Tunnel Publishing**: Enables full SSL/TLS encryption in transit without exposing open inbound TCP ports on your GCP VM firewall.
- **CSRF Protection**: All administrative mutations (`POST`, `PUT`, `DELETE`) require a cryptographically verified `X-CSRF-Token` header matching the active session.
- **Session Security**: Session tokens are transmitted exclusively in `HttpOnly`, `SameSite=Lax` cookies.
