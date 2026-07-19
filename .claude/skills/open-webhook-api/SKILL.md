---
name: open-webhook-api
description: >-
  Use a running open-webhook instance from automations via its JSON API — create a webhook URL to
  hand to a user or a third-party service, then read back the captured requests (method, headers,
  base64 body, uploaded files) or download files. Invoke whenever you need to programmatically get a
  webhook endpoint or inspect what was sent to one. Includes the Swagger link and a curl cheatsheet.
allowed-tools: Read, Bash
---

# open-webhook API cheatsheet

## Base URL

Use the instance saved in your memory (`open-webhook-instance`). If none is saved, run `/open-webhook`
to set one up, or ask the user for it. Set it once per session:

```bash
BASE=<instance base url>   # e.g. https://hooks.example.com or http://localhost:8080
```

- Interactive docs (Swagger UI): `{BASE}/docs`
- Full machine-readable spec (fetch when you need an endpoint not covered here): `{BASE}/openapi.json`

## Concepts

- A **session** = one webhook endpoint, identified by a UUID. Its capture URL is `{BASE}/{session_uuid}`.
- Send **any** method / path / body to `{BASE}/{session_uuid}` (or `.../{session_uuid}/anything`) and
  it is recorded.
- Sessions auto-expire (default TTL ~7 days) and keep the most recent ~128 requests.
- Request bodies are returned **base64-encoded**. Files from `multipart/form-data` are extracted and
  downloaded separately.

## Key endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/session` | Create a session → returns its `uuid` |
| `GET` | `/api/session/{sid}` | Get session options |
| `GET` | `/api/session/{sid}/requests` | List captured requests (newest first) + files |
| `GET` | `/api/session/{sid}/requests/{rid}` | One captured request (full payload) |
| `GET` | `/api/session/{sid}/requests/subscribe` | WebSocket — real-time push of new requests |
| `DELETE` | `/api/session/{sid}/requests` | Clear all captured requests |
| `GET` | `/{sid}/files/{fid}` | Download an extracted file |
| `*` | `/{sid}` | The **capture URL** — send webhooks / test requests here |
| `GET` | `/api/settings` | Service limits (max requests, body size, session TTL) |

## Recipes

### Get a webhook URL to hand out
```bash
SID=$(curl -s -X POST "$BASE/api/session" \
  -H 'Content-Type: application/json' \
  -d '{"status_code":200,"headers":[],"delay":0,"response_body_base64":""}' | jq -r '.uuid')
echo "Give this URL to the user / service: $BASE/$SID"
```
You can append any path, e.g. `$BASE/$SID/payment/callback`. To make the URL **simulate a specific
tool or API**, set `status_code`, `headers`, `delay` (≤ 30s), and a base64 `response_body_base64` when
creating the session.

### Read what was captured
```bash
# list (newest first)
curl -s "$BASE/api/session/$SID/requests" \
  | jq '.[] | {method, url, captured_at_unix_milli, files: [.files[].name]}'

# decode the newest request body (it is base64-encoded)
curl -s "$BASE/api/session/$SID/requests" | jq -r '.[0].request_payload_base64' | base64 -d
```

### Download an uploaded file (multipart/form-data)
```bash
FID=$(curl -s "$BASE/api/session/$SID/requests" | jq -r '.[0].files[0].uuid')
curl -s "$BASE/$SID/files/$FID" -o ./downloaded-file
```
Both the `session_uuid` and the `file_uuid` are required (unguessable), so files cannot be enumerated.

### Real-time instead of polling (optional)
Open a WebSocket to `wss://<host>/api/session/{sid}/requests/subscribe` to receive a push for each new
request (event carries metadata; then fetch the full request for the payload).

## Need an endpoint not listed here?
Fetch the full spec: `curl -s "$BASE/openapi.json"`, or open `{BASE}/docs`.
