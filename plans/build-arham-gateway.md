# Build Arham Gateway

## Goal

Build Arham Gateway as a fast, resource-efficient, single-user, self-hosted OpenAI-compatible model gateway for an Ubuntu GCP e2-small VM. Clients authenticate with Arham Gateway API keys and see only Arham's public model IDs; provider identities, credentials, model IDs, routing, retries, and errors remain private.

The first release supports four upstream providers—Baseten, Fireworks AI, SiliconFlow, and Novita AI—and two public model aliases:

- `deepseek-v4-flash`
- `deepseek-v4-flash-fast`

The system must remain straightforward to extend with more public models, upstream mappings, and providers.

## Product decisions and constraints

- Use one Go binary for the HTTP gateway, admin API, embedded dashboard assets, migrations, and CLI.
- Use SQLite in WAL mode; do not require Docker, Node.js at runtime, Postgres, Redis, or a separate application server.
- Build the dashboard with Preact (or an equivalently small typed SPA) and embed its production assets in the Go binary.
- Run as an unprivileged systemd service and listen on `127.0.0.1:8080` by default.
- Publish the service through a user-configured Cloudflare Tunnel. `gateway setup` installs Cloudflared if requested/missing but does not silently create or authorize a Cloudflare account or tunnel.
- Never persist prompts, responses, authorization headers, or raw request bodies. Metadata may be retained according to a configurable policy.
- Encrypt upstream provider keys at rest. Store dashboard passwords with Argon2id and gateway API keys as non-reversible hashes.
- All public responses and errors must be provider-neutral.
- Prices are editable per provider/model mapping and changes apply prospectively. Every attempt stores the exact rates used.
- Use integer/fixed-precision monetary units; never binary floating point for prices or costs.

## Public behavior

### Initial API

Implement:

- `GET /healthz` for local process health
- `GET /readyz` for database/configuration readiness
- `GET /v1/models`
- `POST /v1/chat/completions`, streaming and non-streaming

Authenticate `/v1/*` with `Authorization: Bearer <arham-key>`. `/v1/models` lists only enabled public aliases. Reject upstream provider model IDs.

For chat completions:

- Preserve the requested public alias in the response.
- Generate gateway-owned request/completion IDs; all chunks in one stream use the same gateway-owned completion ID.
- Strip upstream headers and private/provider-specific fields.
- Normalize errors to the OpenAI error envelope without provider names, model IDs, request IDs, URLs, or raw response bodies.
- Do not expose cost, routing, retries, key identity, or provider identity to API clients.
- Stream normalized SSE without application buffering, finish with `[DONE]`, and cancel upstream work when the client disconnects.

The minimum first-release request contract is:

- Required: `model`, `messages`
- Supported messages: `system`, `user`, `assistant`, and `tool` roles; text content; assistant tool calls and corresponding tool results
- Supported controls: `stream`, `stream_options.include_usage`, `temperature`, `top_p`, `max_tokens`, `max_completion_tokens`, `stop`, `frequency_penalty`, `presence_penalty`, `seed`, `response_format`, `tools`, `tool_choice`, and `parallel_tool_calls`
- If both `max_tokens` and `max_completion_tokens` are supplied, reject the request rather than guessing precedence.
- Structured text content parts may be normalized to text; image, audio, file, and other multimodal parts are rejected in the first release.
- Unknown fields and values unsupported by the public model contract receive a provider-neutral `400` error; never blindly pass arbitrary fields upstream.

Normalize responses for text, tool calls, finish reasons, and usage. Public usage uses OpenAI-compatible `prompt_tokens`, `completion_tokens`, and `total_tokens`, with cached input in `prompt_tokens_details.cached_tokens` when known. `stream_options.include_usage` controls public streaming usage output but does not stop the gateway from collecting internal usage. A provider mapping declares capabilities for fields that are not universally supported; the router skips incapable mappings before dispatch. If no enabled mapping can satisfy a valid request, return a provider-neutral unsupported-capability error. Document the exact tested contract and do not claim support for other OpenAI endpoints or parameters.

### Initial routing defaults

`deepseek-v4-flash`:

1. Fireworks AI
2. SiliconFlow
3. Novita AI
4. Baseten

`deepseek-v4-flash-fast`:

1. Baseten
2. Novita AI
3. SiliconFlow
4. Fireworks AI

The dashboard can reorder enabled mappings by drag and drop. Persist each model's route transactionally and validate that entries are unique and valid.

Within a provider, select healthy enabled keys round-robin. Try another usable key in the same provider before advancing to the next provider. Temporary failures place keys in bounded cooldowns; authentication failures mark a key invalid until edited or re-enabled; manually disabled and estimated-exhausted keys are skipped according to configured balance policy.

Retry only failures that are plausibly transient or key-specific: connection failure, first-response timeout, HTTP 429, HTTP 502/503/504, documented temporary-capacity errors, and key authentication failures. Do not retry malformed requests, context-length failures, policy rejections, unsupported parameters, or other deterministic client errors.

Failover is permitted only before downstream response bytes are committed. Once streaming has started, forward or terminate the stream; never splice output from another provider. Record every attempt, including failed attempts and unknown billable usage.

## Provider discovery gate

Before implementing each adapter, verify against current official provider documentation and record in `docs/providers.md`:

- Exact base URL, authentication scheme, and upstream model ID for the required 0731 model/version
- Whether the model is actually available to the account/region
- Supported OpenAI request fields and streaming format
- How final streaming usage is requested and reported
- Cached-token semantics
- Error/status behavior and rate-limit headers
- Current input, cached-input, and output pricing
- Whether balance is available by API and whether it is account-level or key-level

Do not infer that all four providers are identical because they expose OpenAI-like APIs. Provider-specific details stay inside adapters. If the desired model/version is unavailable from a provider, its mapping must remain disabled rather than silently targeting a different model.

## Data model

Introduce versioned migrations for these logical records (exact table splitting may follow implementation needs):

- `settings`: non-secret application settings, retention, timeouts, and balance policy
- `admin_credentials`: Argon2id password hash and password-change metadata
- `admin_sessions`: hashed session token, creation, expiry, and revocation
- `providers`: stable provider identity and enabled state
- `provider_keys`: provider, encrypted secret, display name/prefix, configured starting balance, status, timestamps, and safe last-error category
- `balance_adjustments`: per-key signed monetary adjustment and note, forming an auditable ledger
- `public_models`: public alias, display metadata, and enabled state
- `provider_model_mappings`: provider, public model, hidden upstream model ID, capability/configuration fields, integer rates per million tokens, currency, and enabled state
- `routing_entries`: public model, provider mapping, and ordered priority
- `gateway_keys`: indexed key hash, visible prefix, name, status, created/last-used/revoked timestamps, and optional limits when added later
- `requests`: gateway request ID, public model, gateway key ID, timestamps, final outcome, normalized usage, aggregate timing/cost, retry/failover counts, and usage confidence
- `request_attempts`: request ID, provider/key/mapping IDs, sequence, status/error category, timings, normalized usage, snapshotted rates, component costs, total cost, usage confidence, and whether it has been included in durable aggregates
- `usage_rollups_daily`: durable UTC-day aggregates by provider key, provider mapping/public model, and gateway key for known cost, token counts, requests, failures, retries, failovers, and unknown-cost counts

Use foreign keys, bounded text fields, indexes for dashboard filters, UTC timestamps, and schema constraints. Never place prompt or response content in these tables. Provider keys, mappings, public models, and gateway keys referenced by history are archived/disabled rather than hard-deleted.

## Pricing, usage, and balances

Store rates as exact integer/fixed-precision USD amounts per one million tokens:

- uncached input rate
- cached-input rate
- output rate

For each attempt:

```text
uncached_input = max(input_tokens - cached_input_tokens, 0)
input_cost = uncached_input * input_rate / 1,000,000
cached_cost = cached_input_tokens * cached_rate / 1,000,000
output_cost = output_tokens * output_rate / 1,000,000
total_cost = input_cost + cached_cost + output_cost
```

Define whether provider `input_tokens` includes cached input in each adapter and normalize before applying this formula. Preserve enough precision during multiplication/division and define one consistent final rounding rule.

On dispatch, snapshot all three rates onto the attempt. Dashboard edits affect only attempts created afterward and require no restart. Historical records are never silently recalculated.

Normalize usage as input, cached-input, and output tokens with source/confidence: `provider_reported`, `gateway_estimated`, or `unavailable`. Prefer provider-reported final usage. Do not fabricate cached tokens. The first release must not estimate tokens unless an exact, tested tokenizer for the upstream model is available; otherwise mark usage unavailable. If a failed attempt has no usage, mark cost unknown rather than zero. Keep unknown cost distinct from a known zero.

Finalize an attempt and upsert its `usage_rollups_daily` contribution in one SQLite transaction, guarded so an attempt can be aggregated only once. Request-log retention may delete old `requests` and `request_attempts`, but never deletes balance adjustments or durable rollups. Lifetime spend/balance totals and long-range aggregate analytics come from rollups, so deleting detailed logs cannot reduce spend or increase remaining balance. Recent request drill-down continues to use retained request/attempt rows.

For each provider key, present separately:

- configured starting balance
- manual adjustments
- gateway-estimated spend
- estimated remaining balance
- provider-reported balance and synchronization time, only where officially supported

Default starting balances are Baseten $1, Fireworks AI $6, Novita AI $1, and SiliconFlow $1. Clearly label estimates. The deployment invariant is that only one configured API key comes from any one upstream provider account; each additional key for a provider represents a distinct account. This allows an officially reported account-level balance to be associated with that key without double-counting. State this invariant in the add-key form and documentation because the gateway generally cannot verify account identity. External usage can still prevent estimates from matching provider billing.

## Metrics

Measure with a monotonic clock in-process and store durations, not wall-clock differences:

- TTFT: gateway receipt to first meaningful output content
- generation throughput: output tokens / first-content-to-final-content duration
- effective TPS: output tokens / gateway-receipt-to-completion duration
- end-to-end latency
- upstream attempt latency

Request streaming from the upstream even when the client requests `stream: false`, then aggregate normalized chunks into a bounded in-memory non-streaming response. This keeps TTFT comparable for streaming and non-streaming clients. Adapters must correctly aggregate text, tool-call deltas, finish reasons, and final usage. If a provider cannot stream for a valid request shape, use its non-streaming mode and mark TTFT and generation throughput unavailable rather than substituting full-response latency. Enforce configurable maximum aggregated response bytes and tokens.

Dashboard aggregations include request volume, in-flight requests, success/error/rate-limit rates, p50/p95/p99 TTFT and latency, throughput, effective TPS, token counts, estimated/known cost, unknown-cost attempts, retries, failovers, and provider/key health. Filters include date range, public model, provider, provider key, gateway key, and outcome. Keep SQL queries bounded and indexed.

## Security and privacy

- Generate a random master encryption key during setup, store it outside SQLite as `/etc/arham-gateway/master.key`, owned by the dedicated gateway service account with mode `0600` inside a root-controlled directory, and use authenticated encryption for provider secrets. Root may access it administratively, but unrelated users and services may not.
- Store random gateway API keys only as an indexed cryptographic hash; display the secret once at creation and retain only a short non-secret prefix.
- Use Argon2id for the dashboard password, secure HttpOnly SameSite cookies, CSRF protection on mutations, session expiry/revocation, login throttling, generic auth failures, and constant-time secret comparisons where applicable.
- Bind to loopback by default. Reject unsafe proxy assumptions; configure trusted proxy handling explicitly for Cloudflare/local deployment.
- Apply request-body limits, header/read timeouts, upstream connection limits, and graceful shutdown without imposing a total timeout that kills healthy long model streams.
- Redact secrets and upstream bodies from application logs and errors. Audit all metadata fields for accidental prompt fragments.
- Add standard dashboard security headers and prevent dashboard assets/API responses from being cached where inappropriate.
- Document backup handling: the database and master key are both required; either alone must not disclose provider secrets.

## CLI and service management

Build these user commands:

- `gateway setup`
- `gateway start`
- `gateway stop`
- `gateway restart`
- `gateway status`
- `gateway password-reset`

Also implement `gateway serve` as the foreground server process used only by systemd and advanced diagnostics. It loads configuration, opens and migrates SQLite, validates the master key and readiness prerequisites, starts the HTTP server, and handles termination signals with graceful draining. The installed unit uses `ExecStart=/usr/local/bin/gateway serve`; management commands call systemd and must not daemonize or launch a second unmanaged server.

`setup` is idempotent where safe and:

1. Checks supported Ubuntu/systemd environment and privileges.
2. Installs/copies the gateway binary.
3. Creates an unprivileged service account and directories.
4. Creates `/etc/arham-gateway/config.toml` and `/etc/arham-gateway/master.key` with restrictive ownership/modes.
5. Creates `/var/lib/arham-gateway/gateway.db` through migrations.
6. Securely prompts twice for the dashboard password without command-line arguments or terminal echo.
7. Installs and enables a hardened systemd unit, but does not unexpectedly expose the listener.
8. Offers to install Cloudflared from its official supported source if missing.
9. Prints local verification and Cloudflare Tunnel setup instructions.

`start`, `stop`, and `restart` wrap systemd with useful errors. `status` combines service state with local health/readiness. `password-reset` prompts securely, updates the Argon2id hash, and revokes existing admin sessions.

Use this target layout:

```text
/usr/local/bin/gateway
/etc/arham-gateway/config.toml
/etc/arham-gateway/master.key
/var/lib/arham-gateway/gateway.db
```

Prefer journald over a separate log-file subsystem. Add `gateway logs`, `doctor`, `backup`, and `restore` only after the core commands are complete.

## Dashboard

Create these authenticated areas:

- Overview: spend, estimated balances, traffic, success rate, latency, and health
- Providers: encrypted key CRUD, status, balance defaults/adjustments, and safe validation
- Models & Routing: public aliases, hidden mappings/prices, enablement, and drag/drop priorities
- Gateway Keys: create-once display, disable, revoke, and usage
- Analytics: cost, token, TTFT, latency, generation throughput, and effective TPS charts/tables
- Request Logs: metadata-only requests with expandable attempts and filters
- Settings: password, retention, timeouts, and balance policy

The server is authoritative for authorization, validation, routing, money, and aggregation. The SPA must not receive raw provider secrets after submission.

## Intended repository organization

Use a conventional, discoverable layout and keep domain packages internal to the binary. Adjust names if Go conventions discovered during implementation make a simpler layout, but preserve these boundaries:

```text
cmd/gateway/                 CLI and server entry point
internal/app/                startup, shutdown, and dependency wiring
internal/config/             config loading and validation
internal/database/           SQLite connection, migrations, and queries
internal/auth/               admin sessions, passwords, CSRF, and API keys
internal/crypto/             provider-secret authenticated encryption
internal/publicapi/          /v1 handlers and provider-neutral wire types
internal/adminapi/           authenticated dashboard API
internal/providers/          adapter contract and provider implementations
internal/routing/            route/key selection, health, retries, cooldowns
internal/accounting/         token normalization, exact costs, balances
internal/analytics/          bounded queries and metric aggregation
internal/service/            setup/systemd/cloudflared integration
web/                         dashboard source and build configuration
web/dist/                    generated assets embedded by Go
migrations/                  ordered SQL migrations embedded by Go
test/                        cross-boundary integration/load fixtures only
packaging/systemd/           service unit/template
scripts/                     reproducible build/release helpers
docs/                        end-user setup, use, and operations docs
```

Prefer package-level tests beside code and black-box tests at HTTP/CLI boundaries. Avoid repository abstractions, plugin frameworks, event buses, or generated query layers unless measurement or repeated code demonstrates a need.

## Implementation sequence

### Phase 1: repository and executable foundation

- Create the Go module, command entry point, structured configuration, embedded migrations, SQLite connection policy, structured redacted logging, build/version metadata, and signal-driven graceful shutdown.
- Create the dashboard workspace and reproducible production asset embedding. Node tooling is build-time only.
- Add CI commands for Go tests/static analysis, dashboard typecheck/test/build, migration checks, and secret scanning.

### Phase 2: security and administration foundation

- Implement master-key loading, provider-secret encryption, password setup/reset, admin sessions, CSRF, gateway-key generation/hash authentication, and audit-safe errors.
- Implement provider key, public model, mapping/price, and route CRUD APIs with validation.
- Seed providers, two public aliases, default balances, and default route order without inventing upstream model IDs or prices.

### Phase 3: proxy and adapter seam

- Define the smallest adapter boundary needed to translate a normalized chat request, execute stream/non-stream calls, normalize chunks/usage/errors, and classify retries.
- Implement one provider end to end first, with contract tests against a local fake upstream. Add the other adapters only after completing the provider discovery gate.
- Implement public API authentication, model resolution, provider-neutral responses, cancellation, SSE flushing, and metadata-only request/attempt recording.

### Phase 4: routing and resilience

- Implement ordered mapping selection, per-provider healthy-key round-robin, cooldowns, deterministic retry classification, pre-commit failover, attempt limits, and concurrency-safe health state.
- Ensure cancellation and shutdown release upstream bodies/connections and no retry occurs after downstream commit.

### Phase 5: accounting and analytics

- Implement normalized usage, exact cost calculation, immutable rate snapshots, idempotent durable daily rollups, balance ledger and estimates, retention cleanup that preserves lifetime accounting, bounded aggregations, and metrics definitions.
- Add provider-reported balance synchronization only for providers with an official suitable API; keep it optional and clearly scoped.

### Phase 6: dashboard

- Build the seven dashboard areas against the admin API, with accessible controls, secure secret forms, route drag/drop plus keyboard controls, explicit estimate/unknown labels, responsive tables, and clear empty/error/loading states.

### Phase 7: CLI, packaging, operations, and end-user documentation

- Implement setup and service commands, systemd hardening, release artifacts/checksums, and migration-on-start with safe failure behavior.
- Write the complete end-user documentation set under `docs/`, including installation, initial setup, dashboard configuration, API use, Cloudflare Tunnel setup, upgrades, backup/restore, security, and troubleshooting. Treat the docs as part of the feature: every command and example must match the shipped binary.
- Exercise the documented installation from a release artifact on a fresh supported Ubuntu VM. Have a tester follow only the docs, then validate dashboard access plus tunnel-based streaming and non-streaming requests.

### Phase 8: hardening and release verification

- Threat-model auth, proxying, secret storage, logs, backups, and admin endpoints.
- Load-test representative long-lived streaming traffic on an actual e2-small while measuring RSS, CPU, open files, database contention, disconnect cleanup, and latency overhead. Fix unbounded behavior; publish measured capacity rather than an unsupported request-count claim.
- Test crash/restart behavior, migration rollback strategy, disk-full behavior, expired sessions, key revocation, malformed SSE, slow upstreams, and graceful shutdown with live streams.

## Verification and acceptance criteria

### Automated behavior

- Public model listing contains only the two enabled Arham aliases.
- Requests using hidden upstream IDs are rejected.
- Contract tests cover every supported chat request field, text/tool-call response normalization, streaming chunk identity and `[DONE]`, usage output, unsupported fields, and incapable-route filtering.
- Streaming and non-streaming completions preserve the Arham alias and contain no provider identifiers in bodies, headers, IDs, fingerprints, or normalized errors.
- Non-streaming clients receive correctly aggregated text/tool-call responses from streaming upstreams, with TTFT measured at the first meaningful upstream content; non-stream-capable attempts mark TTFT/throughput unavailable.
- Gateway keys authenticate, can be revoked immediately, and are never recoverable from storage.
- Provider secrets are not readable from SQLite and decrypt only with the separate master key.
- Default routing order is correct for both aliases; priority changes apply without restart.
- All usable keys in one provider are attempted according to policy before moving on; non-retryable failures do not fail over.
- No failover occurs after downstream bytes are committed.
- Client disconnect cancels upstream work and does not leak goroutines/connections.
- Price arithmetic is exact for independently calculated fixtures, including cached input and rounding boundaries.
- Editing a rate affects future attempts only; existing attempt costs and snapshots remain unchanged.
- Failed attempts with missing usage show unknown cost, not zero.
- Balance summaries reconcile exactly from starting balance, adjustments, and known gateway spend.
- Attempt finalization contributes to daily rollups exactly once across retries/crash recovery; deleting expired detailed logs does not change lifetime spend, remaining balances, or retained rollup analytics.
- Logs and database fixtures contain no prompt/response content or secrets.
- Dashboard login, CSRF, expiry, throttling, password reset, and session revocation work as specified.
- Migrations apply from an empty database and from every released schema fixture.

### Operational behavior

- A release installs on a clean supported Ubuntu system and runs `gateway serve` under the dedicated unprivileged systemd user.
- The service account can read its `0600` master key, while an unrelated local user cannot.
- Listener defaults to loopback; no inbound gateway port must be opened for Cloudflare Tunnel.
- `setup`, service commands, status, and password reset are safe and actionable; repeated management commands never create an unmanaged duplicate server.
- Restart and graceful stop preserve database integrity and handle active streams according to a documented drain deadline.
- A database-plus-master-key backup restores successfully; either artifact alone does not reveal provider secrets.
- Representative e2-small load testing shows bounded memory, goroutine, connection, and SQLite growth. Record the tested workload and results in release documentation.

## End-user documentation deliverables

As part of implementation—not as planning artifacts—create a concise `README.md` and complete setup/operations documentation under `docs/`. At minimum ship:

- `README.md`: what Arham Gateway is, supported models/endpoints, requirements, a minimal quick start, and links into `docs/`
- `docs/installation.md`: supported Ubuntu/GCP requirements, release download/checksum verification, `gateway setup`, filesystem/service details, and uninstall steps
- `docs/cloudflare-tunnel.md`: dashboard-created tunnel instructions, hostname-to-`http://localhost:8080` mapping, connector service command, SSE verification, token safety, and optional Cloudflare Access
- `docs/configuration.md`: config file reference, defaults, environment overrides if supported, timeouts/cooldowns/retention, loopback binding, and restart requirements
- `docs/dashboard.md`: first login, providers/keys, balances, editable pricing, model mappings, drag/drop routing, gateway keys, analytics, logs, and password reset
- `docs/api-reference.md`: authentication, `/v1/models`, `/v1/chat/completions`, streaming examples, supported fields, model aliases, normalized errors, privacy behavior, and `curl` examples
- `docs/providers.md`: verified provider-specific setup requirements, exact model mapping guidance, pricing/usage caveats, and links to dated official sources; never include secrets
- `docs/metrics-and-costs.md`: TTFT/throughput/effective-TPS definitions, exact cost formula, cached-token handling, immutable rate snapshots, estimated versus reported balances, and unknown usage
- `docs/backup-restore.md`: SQLite-safe backup, master-key requirement, restore drill, ownership/modes, and disaster cases
- `docs/upgrading.md`: backup-first upgrade, binary replacement, migrations, rollback limits, health verification, and release notes
- `docs/security.md`: threat boundaries, secret storage, prompt non-persistence, gateway-key handling, network exposure, Cloudflare considerations, and incident rotation steps
- `docs/troubleshooting.md`: systemd/journald commands, readiness failures, tunnel/SSE issues, invalid or throttled provider keys, database/disk problems, and safe diagnostics

Generate CLI help from the actual command definitions where practical, but keep narrative procedures hand-written and tested. All examples must use Arham public model IDs and must not expose provider model IDs on the public API. Documentation must clearly distinguish steps performed automatically by `gateway setup` from the Cloudflare account/tunnel steps the user performs manually.

Documentation acceptance criteria:

- A new user can install and publish the gateway on a clean supported Ubuntu VM by following only `README.md` and `docs/installation.md`/`docs/cloudflare-tunnel.md`.
- Every documented command is exercised in CI where possible and in the fresh-VM release test otherwise.
- API examples pass against the release candidate, including one streaming request.
- Configuration names/defaults are checked against the implementation to prevent drift.
- Security-sensitive examples use placeholders and do not encourage secrets in shell history.
- Docs are updated in the same change whenever public API behavior, setup, configuration, routing semantics, metrics, cost rules, or operational procedures change.

## Open questions that must be resolved before production release

These do not block foundational work, but they affect production configuration or behavior:

1. Confirm the exact required “DeepSeek V4 Flash 0731” upstream model/version and IDs on all four providers.
2. Confirm official prices and cached-token semantics for every provider/model mapping.
3. Validate the default warning-only behavior for an estimated balance at or below zero. Estimated exhaustion does not disable routing unless the administrator explicitly enables that policy.
4. Set retry count, first-response timeout, key cooldowns, stream drain deadline, and log-retention defaults from provider behavior and e2-small tests.
5. Decide the production hostname layout (one hostname with `/admin`, or separate dashboard/API hostnames). Both may route to the same loopback service.
