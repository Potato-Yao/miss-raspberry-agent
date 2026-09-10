# API Reference

HTTP API for `miss-raspberry-agent`. The server listens on `HTTP_ADDR` (default `:8080`) and
speaks JSON. All request/response bodies are UTF-8 JSON.

## Authentication

Every `/api/v1/*` endpoint requires a bearer token:

```http
Authorization: Bearer <API_TOKEN>
```

`API_TOKEN` is configured through the environment (see `.env.example`) and is required at
startup. Missing or invalid tokens receive `401 Unauthorized`. Comparison is constant-time.

`GET /healthz` is public.

## Errors

All errors share one shape:

```json
{ "error": "human-readable message" }
```

| Status | Meaning |
| --- | --- |
| `400 Bad Request` | Malformed JSON, missing required field, or invalid field value |
| `401 Unauthorized` | Missing or invalid bearer token |
| `422 Unprocessable Entity` | Well-formed request but unsupported value (e.g. unknown platform) |
| `500 Internal Server Error` | Unexpected server error |

## Endpoints

### `GET /healthz`

**Description:** Liveness check. No authentication required.

**Request:** No body, no parameters.

**Response `200 OK`**

```json
{ "status": "ok" }
```

**curl**

```bash
curl http://127.0.0.1:8080/healthz
```

### `POST /api/v1/agents/main/messages`

**Description:** Submits a message for the main agent to produce and send. The request is
**queued**: the handler validates it and pushes it onto the main agent's todo list, then returns
immediately. The agent processes the queue asynchronously.

**Request body**

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `platform` | string | yes | Message platform. Currently only `qq` is supported. |
| `target_id` | string | yes | Target identifier on that platform. For `qq`, a numeric user QQ id. |
| `content` | string | yes | The message content the agent should work from. Must be non-blank. |
| `context` | string | no | Extra context to help the agent craft a better reply (e.g. who the user is, prior events). |

> **Platform note:** `qq` currently maps to a **private** QQ chat, so `target_id` is the
> recipient's user QQ number. Group delivery is not exposed through this endpoint yet.

**Response `202 Accepted`**

| Field | Type | Description |
| --- | --- | --- |
| `id` | string | Id of the created todo item. The agent removes it from the queue once handled. |
| `status` | string | Always `queued`. |

```json
{
  "id": "item-1",
  "status": "queued"
}
```

**Error responses:** `400` (malformed body / missing or invalid field), `401` (bad token),
`422` (unsupported `platform`).

**curl**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/agents/main/messages \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"qq","target_id":"10001","content":"你好","context":"初次打招呼"}'
```

## Related configuration

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Address the HTTP API listens on |
| `API_TOKEN` | — (required) | Bearer token for `/api/v1/*` |
