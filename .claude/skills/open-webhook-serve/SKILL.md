---
name: open-webhook-serve
description: >-
  Install and run the open-webhook service — LOCALLY on this machine (a localhost URL for testing
  local apps), on a PRIVATE trusted network, or PUBLICLY on a server over SSH behind the user's own
  domain with HTTPS, kept running across reboots. Invoke when the user wants to run, host, deploy,
  publish, or "make open-webhook available". Written for non-technical users: it explains SSH access
  and domains in plain language and applies safe defaults (body-size limit, authenticated management
  UI/API, bounded retention) for public deployments. Saves the resulting instance URL to memory.
allowed-tools: Read, Write, Bash
---

# Run / host open-webhook

Pick the deployment tier that matches what the user needs. **Security defaults get stricter as
exposure grows** — do not carry local-dev convenience settings onto a public server.

| Tier | Where | Auth on management UI/API | Auto-create sessions | Body limit |
|------|-------|---------------------------|----------------------|------------|
| **Local dev** | this machine, `localhost` | none (not exposed) | ON (convenient) | unlimited OK |
| **Private / trusted network** | LAN or VPN only | optional | ON or OFF | set a limit |
| **Public internet** | server + domain, HTTPS | **required (Basic Auth)** | **OFF** | **50 MB default** |

The app always needs a **Redis** server (stores captured requests) and a **files directory** (stores
uploaded files). Verified flag names (do not invent others): `--files-dir`, `--redis-dsn`,
`--auto-create-sessions`, `--max-request-body-size` (bytes; `0` = unlimited), `--session-ttl`
(default `168h`), `--max-requests` (default `128`). There is **no built-in authentication flag** —
management protection is done at the reverse proxy (Caddy).

---

## Tier 1 — Local development (keep it simple)

Requirement: Docker. From the repo root:

```bash
docker build -t open-webhook .        # builds Go + React; no toolchain needed on the host
docker run -d --name wh-redis --restart unless-stopped redis:8-alpine
mkdir -p "$PWD/wh-files"
docker run -d --name open-webhook --restart unless-stopped \
  -p 8080:8080 --link wh-redis \
  -v "$PWD/wh-files:/data/files" \
  -e REDIS_DSN="redis://wh-redis:6379/0" \
  open-webhook start --files-dir /data/files --auto-create-sessions
```

Verify: `curl -fsS http://localhost:8080/healthz` → `200`. Base URL = `http://localhost:8080`.
Auto-create is fine here because nothing outside your machine can reach it. Then **save it to memory**
(see the last section) and tell the user their local URL.

> The repo also has `make up` for a dev/watch setup. For a persistent local instance prefer the
> `docker run --restart unless-stopped` commands above.

---

## Tier 2 — Private / trusted network

Same as local, but reachable by trusted colleagues on a LAN or VPN. Apply two changes:

- **Set a body-size limit** even here (e.g. 50 MB): add `--max-request-body-size 52428800`.
- Decide on `--auto-create-sessions`: keep it ON only if every user on the network is trusted;
  otherwise omit it and create sessions via the API.

Do **not** port-forward this to the public internet without the Tier 3 protections.

---

## Tier 3 — Public internet (SSH server + domain)

**You (the agent) do all the technical work.** From the user you only need three things — explain each
simply and ask for them:

1. **A server you can SSH into.** A small always-on computer on the internet (a "cloud VM" / "VPS" —
   AWS, DigitalOcean, Hetzner, etc.). I need its **address** (IP or hostname), the **login username**
   (often `ubuntu` or `root`), and **how to log in** (path to an **SSH key file** like
   `~/.ssh/id_ed25519`, or a password).
   > No server yet? Any provider's cheapest Ubuntu VM works. Create it and add your SSH key.
2. **A domain name** you own (e.g. `hooks.yourcompany.com`) with a DNS **A record pointed at the
   server's IP**. This gives a clean HTTPS URL; HTTPS is issued automatically once DNS points at the
   server. Tell the user exactly what to add: an `A` record `hooks.example.com → <server IP>`.
   > No domain? You can run over plain IP (`http://<ip>`), but HTTPS and a friendly URL need a domain.
3. **A username + password for the management UI/API** (you'll set up Basic Auth). Pick any; the
   password is hashed, never stored in plaintext on the server.

Confirm you have all of this before proceeding. **Never print the user's private key.**

### Safe public defaults (apply these)

```bash
OW_MAX_BODY=52428800   # 50 MB request/upload cap. Raise/lower as the user wants.
OW_TTL=48h             # captured data + files expire after this. App default is 168h (7 days).
```

Run the app **without** `--auto-create-sessions` (so random UUID hits cannot spawn sessions) and
**with** the body limit and TTL:

```
open-webhook start --files-dir /data/files \
  --max-request-body-size ${OW_MAX_BODY} --session-ttl ${OW_TTL}
```

`--max-requests` stays at its default of 128 per session (a per-session cap; leave unless the user
asks).

### Deploy steps (agent runs these)

Let `IP`, `USER`, `SSHOPT`, `DOMAIN` be the user's values.

1. **Build locally, copy the image** (server needs no Go/Node toolchain):
   ```bash
   docker build -t open-webhook:deploy .
   docker save open-webhook:deploy | gzip > /tmp/ow.tar.gz
   scp $SSHOPT /tmp/ow.tar.gz $USER@$IP:/tmp/
   ```
2. **Generate the Basic Auth hash** (do this once; substitute the user's password):
   ```bash
   docker run --rm caddy:2-alpine caddy hash-password --plaintext 'THE_PASSWORD'
   ```
3. **On the server**, ensure Docker exists
   (`command -v docker || curl -fsSL https://get.docker.com | sh`), write the Caddyfile (see
   `deployments/caddy/Caddyfile.public.example` in this repo), then run three containers on a shared
   `wh` network, **all `--restart unless-stopped`**:
   - `redis:8-alpine`;
   - the app, internal only:
     `-e REDIS_DSN=redis://redis:6379/0 -e FILES_DIR=/data/files -v /opt/open-webhook/files:/data/files`
     with args `start --max-request-body-size ${OW_MAX_BODY} --session-ttl ${OW_TTL}` (no
     `--auto-create-sessions`);
   - `caddy:2-alpine` on ports 80/443, mounting the Caddyfile and passing
     `-e OW_DOMAIN=$DOMAIN -e OW_ADMIN_USER=<user> -e OW_ADMIN_HASH='<hash from step 2>'`, plus
     volumes `caddy_data:/data caddy_config:/config` so the TLS cert persists.
4. **Verify**:
   - `curl -fsS https://$DOMAIN/healthz` → `200` (public probe).
   - `curl -s -o /dev/null -w '%{http_code}' https://$DOMAIN/api/settings` → `401` (management API is
     protected). With `-u user:pass` it returns `200`.

### What is public vs authenticated (public tier)

| Route | Access | Why |
|-------|--------|-----|
| `POST/ANY /{session_uuid}` and `/{session_uuid}/...` | **public** | webhook capture — external providers must reach it |
| `GET /{session_uuid}/files/{file_uuid}` | **public (capability URL)** | file download by unguessable URL; treat the URL as a secret |
| `/healthz`, `/ready` | **public** | uptime probes, no secrets |
| `/` and `/s/*` (management UI) | **authenticated** | Basic Auth |
| `/api/*` (create sessions, **read captured data**, delete) | **authenticated** | Basic Auth |
| `/docs`, `/openapi.json` | **authenticated** | Basic Auth |

- **Session creation is NOT public**: it happens only through the authenticated `/api/session`, and
  `--auto-create-sessions` is off, so an attacker cannot mint sessions by hitting random UUIDs.
- **Max request/upload size**: `OW_MAX_BODY` (50 MB default), enforced by the app.
- **Retention/cleanup**: sessions (and their captured requests) expire after `OW_TTL`; each session
  keeps its most recent 128 requests; **uploaded files are deleted only when their session is deleted
  or expires** (a janitor sweeps roughly once a minute). There is **no independent per-file TTL**.

### ⚠️ Disk, retention, and exposure warning — tell the user

- **Disk grows with uploads.** Worst case ≈ (number of live sessions) × 128 requests × `OW_MAX_BODY`.
  With auth + auto-create off, sessions are gated, but monitor free disk on the server and lower
  `OW_MAX_BODY` / `OW_TTL` if needed.
- **Files persist until the session expires** (`OW_TTL`), not per download. A shorter TTL reclaims
  disk sooner.
- **Anyone with a capture or file URL can use it** — those URLs are capability secrets (see the
  README's file-uploads note). Don't post them in public logs, tickets, or chats.
- Exposing any capture service to the internet invites junk/abusive traffic to your capture URLs; the
  Basic Auth wall keeps captured *data* and the UI private, but the capture endpoints themselves are
  open by design.

### Limitation (be honest, do not oversell)

The application has **no native authentication, per-session tokens, rate limiting, or per-file
expiry**. All management protection here comes from the Caddy reverse proxy (Basic Auth). Capture
endpoints and file-download capability URLs remain publicly reachable — that is required for webhook
providers and is mitigated only by unguessable UUIDs, not by access control. If you need
authenticated capture, per-URL revocation, rate limiting, or automatic file expiry, that requires
application changes and is **not** provided today. Say this plainly rather than implying the instance
is fully locked down.

---

## Save the instance URL to memory (all tiers)

Once the instance answers on `/healthz`, **save its base URL to your persistent memory** so future
automations use it without asking. Create a memory entry named `open-webhook-instance`, add its
pointer to `MEMORY.md`, and include: the **base URL**; for public, the **server IP/host**, **domain**,
and that the **management API requires Basic Auth** (record the username; do **not** store the
password or private key in memory). Then hand off to `/open-webhook-api` to create webhook URLs and
read captured requests.
