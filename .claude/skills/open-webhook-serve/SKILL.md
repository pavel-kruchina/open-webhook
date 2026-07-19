---
name: open-webhook-serve
description: >-
  Install and run the open-webhook service — either LOCALLY on this machine (a localhost URL for
  testing local apps) or HOSTED on a server over SSH behind the user's own domain, kept running across
  reboots. Invoke when the user wants to run, host, deploy, publish, or "make open-webhook available".
  Written for non-technical users: it explains in plain language what SSH access and a domain are and
  asks for them, then does all the technical work. Saves the resulting instance URL to memory for
  later automations.
allowed-tools: Read, Write, Bash
---

# Run / host open-webhook

Two ways to run it. Pick based on what the user told the router (`/open-webhook`), or ask.

- **Local** — runs on this computer. Fast, private, URL is `http://localhost:8080`. Good for testing
  apps on this machine. Cannot receive requests from the public internet.
- **Hosted** — runs on an always-on server behind the user's domain with HTTPS. Good for receiving
  webhooks from third-party services. Survives reboots.

The app always needs a **Redis** server (stores captured requests) and a **files directory** (stores
uploaded files). Both are handled below. Run the app with `--auto-create-sessions` so any UUID URL
works without pre-creating a session.

---

## Local mode

Requirement: Docker installed on this machine. From the repo root:

```bash
docker build -t open-webhook .        # builds Go + React; no toolchain needed on the host

# Redis (kept running across reboots)
docker run -d --name wh-redis --restart unless-stopped redis:8-alpine

# the app (kept running across reboots via --restart)
mkdir -p "$PWD/wh-files"
docker run -d --name open-webhook --restart unless-stopped \
  -p 8080:8080 --link wh-redis \
  -v "$PWD/wh-files:/data/files" \
  -e REDIS_DSN="redis://wh-redis:6379/0" \
  open-webhook start --files-dir /data/files --auto-create-sessions
```

Verify: `curl -fsS http://localhost:8080/healthz` returns `200`. Base URL = `http://localhost:8080`.

> The repo also has `make up` for a dev/watch setup. For a persistent local instance prefer the
> `docker run --restart unless-stopped` commands above.

Then **save it to memory** (see below) and tell the user their local URL.

---

## Hosted mode (SSH server + domain) — plain-language guide

**You (the agent) do all the technical work.** From the user you only need three things. Explain each
simply and ask for them:

1. **A server you can SSH into.** This is a small always-on computer on the internet (a "cloud VM" or
   "VPS" — e.g. from AWS, DigitalOcean, Hetzner, or any host). I need:
   - its **address** (an IP like `203.0.113.10`, or a hostname),
   - the **username** to log in as (often `ubuntu` or `root`),
   - and **how to log in**: either the path to an **SSH key file** (e.g. `~/.ssh/id_ed25519`) or a
     password.

   > Don't have one yet? Any provider's cheapest Linux (Ubuntu) VM is enough. Create it, and during
   > setup add your SSH key or note the password.

2. **A domain name** you own (e.g. `hooks.yourcompany.com`), with its DNS **A record pointed at the
   server's IP**. This gives you a clean HTTPS URL. If it isn't pointed yet, tell the user exactly
   what to add at their DNS provider: an `A` record `hooks.example.com → <server IP>`. HTTPS is set
   up automatically once DNS points at the server.

   > No domain? You can still run over plain IP (`http://<ip>`), but HTTPS needs a domain.

Confirm you have all of this before proceeding. **Never print the user's private key contents.**

### Deploy steps (agent runs these)

Let `IP`, `USER`, `SSHOPT` (the `-i <key>` / connection options), and `DOMAIN` be the user's values.

1. **Build locally and copy the image to the server** (the server needs no Go/Node toolchain):
   ```bash
   docker build -t open-webhook:deploy .
   docker save open-webhook:deploy | gzip > /tmp/ow.tar.gz
   scp $SSHOPT /tmp/ow.tar.gz $USER@$IP:/tmp/
   ```
2. **On the server**, ensure Docker exists
   (`command -v docker || curl -fsSL https://get.docker.com | sh`), then load the image and run three
   containers on a shared `wh` network, **all with `--restart unless-stopped`** so they return after a
   reboot:
   - `redis:8-alpine` (storage);
   - `open-webhook:deploy` (the app, internal only) with
     `-e REDIS_DSN=redis://redis:6379/0 -e FILES_DIR=/data/files -v /opt/open-webhook/files:/data/files`
     and args `start --auto-create-sessions`;
   - `caddy:2-alpine` on ports 80/443, reverse-proxying to `open-webhook:8080`, with a one-line
     Caddyfile `{DOMAIN} { reverse_proxy open-webhook:8080 }` — Caddy fetches a free HTTPS
     certificate automatically once DNS points at the server.
3. **Verify**: `curl -fsS https://$DOMAIN/healthz` returns `200` (or `http://$IP/healthz` if no
   domain).

> If this repo already contains a proven runbook for a specific deployment (e.g. a `deployments/`
> folder or a machine/runbook doc), follow it instead of the outline above. Either way, keep Redis +
> Caddy + the files volume persistent so captured data and the TLS certificate survive redeploys.

**Persistence:** `--restart unless-stopped` restarts the containers on boot as long as the Docker
service itself starts on boot (default on standard installs; run `systemctl enable docker` if unsure).

### Quick alternative — a public URL without a server (ngrok)

If the user just needs a temporary public URL and has an ngrok token, run the **local** setup above
but add `--tunnel-driver=ngrok --ngrok-auth-token=<token>` to the `start` command. The app creates the
tunnel itself and logs the public URL. Note: the URL is ephemeral and changes between runs.

---

## Save the instance URL to memory (both modes)

Once the instance answers on `/healthz`, **save its base URL to your persistent memory** so future
automations use it without asking. Create a memory entry named `open-webhook-instance` and add its
pointer to `MEMORY.md`. Include:

- the **base URL** (e.g. `https://hooks.example.com` or `http://localhost:8080`),
- for hosted: the **server IP/host** and **domain** (so it can be updated later),
- a note that it is reachable at `{BASE}/healthz`.

Do **not** store passwords or private keys in memory.

Then hand off to `/open-webhook-api` to create webhook URLs and read captured requests.
