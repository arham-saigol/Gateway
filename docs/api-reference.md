# Public API Reference

Arham Gateway exposes a 100% OpenAI-compatible HTTP REST API for chat completions and model listing.

## Base URL & Authentication

- **Base URL**: `http://127.0.0.1:8080` (or `https://gateway.yourdomain.com`)
- **Authentication**: `Authorization: Bearer <your-arham-key>`

All `/v1/*` endpoints require a valid, active Arham Gateway API key (`arham_...`).

---

## 1. List Models

`GET /v1/models`

Returns the list of enabled public model aliases. Provider identities and upstream model IDs are completely omitted.

### Request Example
```bash
curl http://127.0.0.1:8080/v1/models \
  -H "Authorization: Bearer arham_a1b2c3d4..."
```

### Response Example
```json
{
  "object": "list",
  "data": [
    {
      "id": "deepseek-v4-flash",
      "object": "model",
      "created": 1723824000,
      "owned_by": "arham"
    },
    {
      "id": "deepseek-v4-flash-fast",
      "object": "model",
      "created": 1723824000,
      "owned_by": "arham"
    }
  ]
}
```

---

## 2. Create Chat Completion

`POST /v1/chat/completions`

### Supported Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | **Yes** | Public alias (`deepseek-v4-flash` or `deepseek-v4-flash-fast`) |
| `messages` | array | **Yes** | List of message objects with roles `system`, `user`, `assistant`, or `tool` |
| `stream` | boolean | No | Whether to stream back partial progress via SSE (`false` by default) |
| `stream_options` | object | No | `{ "include_usage": true }` to include token usage in streaming |
| `temperature` | number | No | Sampling temperature between 0 and 2 |
| `top_p` | number | No | Nucleus sampling probability |
| `max_tokens` | integer | No | Maximum tokens to generate (mutually exclusive with `max_completion_tokens`) |
| `max_completion_tokens` | integer | No | Upper bound on completion tokens |
| `stop` | string / array | No | Up to 4 sequences where generation stops |
| `frequency_penalty` | number | No | Frequency penalty between -2.0 and 2.0 |
| `presence_penalty` | number | No | Presence penalty between -2.0 and 2.0 |
| `seed` | integer | No | Deterministic sampling seed |
| `response_format` | object | No | Output format (e.g. `{ "type": "json_object" }`) |
| `tools` | array | No | Function tool definitions |
| `tool_choice` | string / object | No | Tool selection control (`"auto"`, `"none"`, `"required"`, or specific function) |
| `parallel_tool_calls`| boolean | No | Enable parallel tool calling |

### Non-Streaming Request Example
```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer arham_a1b2c3d4..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing in one sentence."}
    ],
    "temperature": 0.7
  }'
```

### Non-Streaming Response Example
```json
{
  "id": "chatcmpl-arham-9a8b7c6d5e4f",
  "object": "chat.completion",
  "created": 1723824000,
  "model": "deepseek-v4-flash",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Quantum computing harnesses the strange properties of quantum mechanics—such as superposition and entanglement—to process complex information exponentially faster than classical computers for specific problems."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 28,
    "completion_tokens": 31,
    "total_tokens": 59,
    "prompt_tokens_details": {
      "cached_tokens": 12
    }
  }
}
```

### Streaming Request Example
```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer arham_a1b2c3d4..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash-fast",
    "messages": [{"role": "user", "content": "Count from 1 to 5"}],
    "stream": true
  }'
```

### Streaming SSE Output
```text
data: {"id":"chatcmpl-arham-9a8b7c6d5e4f","object":"chat.completion.chunk","created":1723824000,"model":"deepseek-v4-flash-fast","choices":[{"index":0,"delta":{"role":"assistant","content":"1"},"finish_reason":null}]}

data: {"id":"chatcmpl-arham-9a8b7c6d5e4f","object":"chat.completion.chunk","created":1723824000,"model":"deepseek-v4-flash-fast","choices":[{"index":0,"delta":{"content":", 2, 3, 4, 5"},"finish_reason":"stop"}],"usage":{"prompt_tokens":14,"completion_tokens":10,"total_tokens":24}}

data: [DONE]
```

---

## 3. Health & Readiness Endpoints

- `GET /healthz`: Returns `{"status":"ok"}` (HTTP 200) for local process liveness.
- `GET /readyz`: Returns `{"status":"ready"}` (HTTP 200) when SQLite database is connected and accessible.
