# Provider Discovery & Upstream Adapter Specifications

This document records the exact endpoint configurations, upstream model mappings, authentication schemes, and pricing caveats verified for each supported provider.

## 1. Fireworks AI

- **Provider ID**: `fireworks`
- **Official Base URL**: `https://api.fireworks.ai/inference/v1`
- **Endpoint**: `POST https://api.fireworks.ai/inference/v1/chat/completions`
- **Auth Header**: `Authorization: Bearer <key>`
- **Upstream Model ID**: `accounts/fireworks/models/deepseek-v4-flash`
- **Starting Balance Default**: `$6.00`
- **Default Rates**:
  - Input (Uncached): `$0.14` / 1M tokens (`140,000` micro-USD)
  - Input (Cached): `$0.014` / 1M tokens (`14,000` micro-USD)
  - Output: `$0.28` / 1M tokens (`280,000` micro-USD)
- **Streaming Usage Reporting**: Final chunk includes `usage` object with `prompt_tokens_details.cached_tokens`.

---

## 2. SiliconFlow

- **Provider ID**: `siliconflow`
- **Official Base URL**: `https://api.siliconflow.cn/v1`
- **Endpoint**: `POST https://api.siliconflow.cn/v1/chat/completions`
- **Auth Header**: `Authorization: Bearer <key>`
- **Upstream Model ID**: `deepseek-ai/DeepSeek-V4-Flash`
- **Starting Balance Default**: `$1.00`
- **Default Rates**:
  - Input (Uncached): `$0.14` / 1M tokens (`140,000` micro-USD)
  - Input (Cached): `$0.014` / 1M tokens (`14,000` micro-USD)
  - Output: `$0.28` / 1M tokens (`280,000` micro-USD)
- **Streaming Usage Reporting**: Sent with final SSE event preceding `[DONE]`.

---

## 3. Novita AI

- **Provider ID**: `novita`
- **Official Base URL**: `https://api.novita.ai/v3/openai`
- **Endpoint**: `POST https://api.novita.ai/v3/openai/chat/completions`
- **Auth Header**: `Authorization: Bearer <key>`
- **Upstream Model ID**: `deepseek/deepseek-v4-flash`
- **Starting Balance Default**: `$1.00`
- **Default Rates**:
  - Input (Uncached): `$0.14` / 1M tokens (`140,000` micro-USD)
  - Input (Cached): `$0.014` / 1M tokens (`14,000` micro-USD)
  - Output: `$0.28` / 1M tokens (`280,000` micro-USD)
- **Streaming Usage Reporting**: Included in the final stream chunk when `stream_options.include_usage: true`.

---

## 4. Baseten

- **Provider ID**: `baseten`
- **Official Base URL**: `https://bridge.baseten.co/v1`
- **Endpoint**: `POST https://bridge.baseten.co/v1/chat/completions`
- **Auth Header**: `Authorization: Api-Key <key>`
- **Upstream Model ID**: `deepseek-v4-flash`
- **Starting Balance Default**: `$1.00`
- **Default Rates**:
  - Input (Uncached): `$0.14` / 1M tokens (`140,000` micro-USD)
  - Input (Cached): `$0.014` / 1M tokens (`14,000` micro-USD)
  - Output: `$0.28` / 1M tokens (`280,000` micro-USD)
- **Streaming Usage Reporting**: OpenAI-compatible chunk stream with final usage delta.

---

## Key Invariant & Account Multi-Tenancy

Each configured key in Arham Gateway represents a distinct upstream account. This ensures that balance tracking, adjustments, and account-level balance reports can be mapped 1:1 without risk of double-counting.
