# Metrics & Cost Calculation Formulas

This document details how Arham Gateway measures latencies, calculates exact monetary costs, handles token caching, and maintains balance ledgers.

## 1. Latency & Throughput Metrics

Arham Gateway measures all latencies in-process using monotonic clock measurements (`time.Since` / `time.Duration`), avoiding wall-clock skew.

- **TTFT (Time to First Token)**: Duration from the gateway receiving the initial client HTTP request to the arrival of the first meaningful token delta (text content or tool call) from the upstream provider.
- **Generation Duration**: `Total Duration - TTFT`.
- **Generation Throughput**: `Output Tokens / Generation Duration (seconds)`.
- **Effective TPS**: `Output Tokens / Total Request Duration (seconds)`.
- **End-to-End Latency**: Total elapsed time from client connection arrival to final HTTP response commit.

---

## 2. Exact Integer Cost Formula

All prices and token calculations use exact integer arithmetic in **micro-USD** ($1.00 USD = `1,000,000` micro-USD).

Rates are stored as integer micro-USD per 1,000,000 tokens (`input_rate_per_m_tokens`, `cached_rate_per_m_tokens`, `output_rate_per_m_tokens`).

### Formula

```text
uncached_input = max(input_tokens - cached_input_tokens, 0)

input_cost_micro   = (uncached_input * input_rate_per_m + 500,000) / 1,000,000
cached_cost_micro  = (cached_input_tokens * cached_rate_per_m + 500,000) / 1,000,000
output_cost_micro  = (output_tokens * output_rate_per_m + 500,000) / 1,000,000

total_cost_micro   = input_cost_micro + cached_cost_micro + output_cost_micro
```

### Rate Snapshotting

When an attempt is dispatched:
1. The mapping's current rates are captured directly onto the `request_attempts` row (`input_rate_snapshot`, `cached_rate_snapshot`, `output_rate_snapshot`).
2. If rates are later edited in the dashboard, future attempts reflect the new rates, while all historical attempt costs remain completely immutable.

---

## 3. Account Balance Reconciliations

For every provider key:

```text
Estimated Remaining Balance = Starting Balance + SUM(Manual Adjustments) - SUM(Known Gateway Spend)
```

- **Durable Rollups**: When an attempt completes, its cost and token counts are aggregated into `usage_rollups_daily` inside the same SQLite transaction.
- **Prune Immunity**: Even when detailed request and attempt rows are pruned after the retention period (e.g. 30 days), daily rollups and balance adjustments remain permanent. Lifetime spend and remaining balances can never drift or be corrupted by log cleanup.
