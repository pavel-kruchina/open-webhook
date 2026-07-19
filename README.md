# open-webhook

> [!IMPORTANT]
> ### 🤖 Using an AI coding agent (Claude Code or Codex)? &nbsp;→&nbsp; type **`/open-webhook`**
> Open this repo in your agent and run **`/open-webhook`** (Codex: `$open-webhook`). It asks whether
> you want to **test local apps** or **capture requests from the internet**, then installs, runs, and
> — if needed — deploys the service for you, no manual steps.
> See [🤖 AI agent skills](#-ai-agent-skills) for one-time setup (Claude Code auto-loads them; Codex
> needs a one-line copy).

> [!NOTE]
> **open-webhook** is an **AI-focused fork** of
> [`tarampampam/webhook-tester`](https://github.com/tarampampam/webhook-tester) by Paramtamtam.
> **All credit for the original application goes to the upstream author** — please star and support the
> [upstream project](https://github.com/tarampampam/webhook-tester). This fork keeps the original functionality and
> adds Redis-backed request/session persistence and storage & download of files uploaded via `multipart/form-data`
> (see below).

**open-webhook** is an **agent-native, self-hosted webhook inspector** with Redis persistence and first-class
multipart attachment support. It gives you unique, unguessable URLs that capture every incoming HTTP request in full
— method, headers, body, and uploaded files — and lets you read them back over a JSON API, so an AI agent can inspect
exactly what it (or a third party) sent without opening a browser.

It's a general-purpose webhook inspector, focused on the traffic AI systems depend on:

- **AI-agent tool calls** — see the outbound calls your agents make to tools and external APIs
- **Callbacks & automation triggers** — capture asynchronous callbacks from queues, jobs, and pipelines
- **Provider webhooks** — verify events delivered by third-party and model providers
- **Agent ↔ service HTTP** — observe the raw communication between agents and the services they orchestrate

Because you can customize the response **code, headers, body, and delay**, a captured URL can also **stand in for an
external tool or API** while you build an agent — simulating success, errors, or slow responses on demand.

**What this fork adds over [upstream](https://github.com/tarampampam/webhook-tester):** Redis-backed persistence
(requests and sessions survive restarts and scale across instances), and storage + download of files uploaded via
`multipart/form-data` — so multimodal payloads (documents, images, audio) are captured, not discarded. Self-host it
to keep sensitive payloads inside your own infrastructure.

> [!NOTE]
> open-webhook is a webhook **inspection and debugging** tool — **not** an AI model server or inference platform. It
> does not run, host, or proxy language models.

Consider it a self-hosted alternative to [webhook.site](https://github.com/fredsted/webhook.site),
[requestinspector.com](https://requestinspector.com/), and similar services. Built in Go with an embedded ReactJS UI
(compiled into the binary) and built-in WebSocket push for real-time updates.

<p align="center">
  <img src="https://github.com/user-attachments/assets/26e56d78-8a10-4883-9052-d18047206fda" alt="screencast" />
</p>

## How it works — with an AI agent

The fastest path is to let a coding agent (Claude Code / Codex) drive it end to end:

1. Run **`/open-webhook`**.
2. Choose **local**, **public**, or **an existing deployment**.
3. The agent starts or connects to the service (Redis + app, HTTPS if public).
4. It hands you a **webhook URL**.
5. Trigger the external webhook — point a provider at the URL, or call it yourself.
6. The agent inspects the captured request and downloads any attachments, over the JSON API.

More than a throwaway request bin: captures **persist** in Redis, uploaded **files are stored and downloadable**, and
the whole flow is **API- and agent-driven**.

## 🤖 AI agent skills

This repo ships ready-made **[Agent Skills](https://agentskills.io)** so an AI coding agent can set up
and use open-webhook for you. They live in [`.claude/skills/`](.claude/skills):

| Skill | What it does |
|-------|--------------|
| **`/open-webhook`** | **Start here.** Asks whether you want to test local apps or capture requests from the internet, then wires up the rest. |
| `/open-webhook-serve` | Runs the service locally, or deploys it to your own SSH server behind your domain and keeps it running across reboots. Written for non-technical users. |
| `/open-webhook-api` | Uses a running instance from automations via its JSON API — get a webhook URL, read captured requests, download files. |

**Claude Code** — open this repo and type `/open-webhook` (project skills load automatically; accept
the workspace-trust prompt on first use). Docs: <https://code.claude.com/docs/en/skills>.

**Codex** — Codex reads skills from `.agents/skills/`. Mirror them once, then use `$open-webhook`:

```shell
mkdir -p .agents/skills && cp -r .claude/skills/* .agents/skills/
```

Docs: <https://developers.openai.com/codex/skills>.

> [!TIP]
> Want the skills available in **every** project, not just this repo? Copy the folders into
> `~/.claude/skills/` (Claude Code) or `~/.agents/skills/` (Codex).

### 🔥 Features list

- Requests are stored in Redis; uploaded files (from `multipart/form-data`) are kept on the local filesystem
- Fully customizable response code, headers, and body for webhooks
- Option to expose your locally running instance to the global internet (via tunneling)
- Fast, built-in UI based on `ReactJS`
- Multi-architecture Docker image based on `scratch`
- Runs as an unprivileged user in Docker
- Well-tested, documented source code
- CLI health check sub-command included
- Binary view of recorded requests in UI
- Uploaded files from `multipart/form-data` requests are stored and downloadable through the UI
- Supports JSON and human-readable logging formats
- Liveness probes (`/healthz` endpoint)
- Customizable webhook responses
- Built-in WebSocket support
- Efficient in memory and CPU usage
- Free, open-source, and scalable

### 🗃 Storage

Captured requests (and sessions) are **always stored in Redis** — set the connection string with the `--redis-dsn`
flag (or the `REDIS_DSN` environment variable). Redis is required to start the app, and is also what allows running
multiple instances behind a load balancer.

> [!NOTE]
> Earlier versions offered `memory` and `fs` storage drivers selectable via `--storage-driver`. These have been
> removed: requests are now always stored in Redis, and the `--storage-driver` / `--fs-storage-dir` flags no longer
> exist.

### 📎 File uploads

When a captured request has a `multipart/form-data` body, each uploaded file is extracted and stored on the local
filesystem (not in Redis) under the directory given by the **required** `--files-dir` flag (or the `FILES_DIR`
environment variable). Only the file metadata (name, content type, size, and a random UUID) is kept alongside the
request; the request body itself stores the form structure with the file contents replaced by a short placeholder.

Files are never served directly from disk — they are downloaded **through the app** (which leaves room for adding
throttling later) at an unguessable URL:

```
{webhook-url}/{session_uuid}/files/{file_uuid}
```

Files are removed together with their session (when it is deleted or expires); a background janitor reclaims the disk
space of expired sessions. There is no independent per-file expiry.

> [!IMPORTANT]
> **Session and attachment URLs are capability URLs, not authentication.** Their unguessable UUIDs make practical
> enumeration infeasible, but that is *not* access control — **anyone who obtains a URL can access the associated
> request or file**, so treat these URLs as secrets. They leak easily: through server and proxy logs, browser
> history, screenshots, analytics, chat messages, and issue trackers. For anything exposed to the public internet,
> put the management UI and API (`/`, `/api/*`, `/docs`) behind authentication — the `/open-webhook-serve` skill and
> [`deployments/caddy/Caddyfile.public.example`](deployments/caddy/Caddyfile.public.example) do this with Caddy
> Basic Auth while keeping the capture URLs public for webhook providers.

### 📢 Pub/Sub

For WebSocket notifications, two drivers are supported for the pub/sub system: **memory** and **Redis** (configured
with the `--pubsub-driver` flag).

When running multiple instances of the app, the Redis driver is required.

### 🚀 Tunneling

Capture webhook requests from the global internet using the `ngrok` tunnel driver. Enable it by setting the
`--tunnel-driver=ngrok` flag and providing your `ngrok` authentication token with `--ngrok-auth-token`. Once enabled,
the app automatically creates the tunnel for you – no need to install or run `ngrok` manually (even using docker).

With this public URL, you can test your webhooks from external services like GitHub, GitLab, Bitbucket, and more.
You'll never miss a request!

## ⁉ FAQ

**Can I have pre-defined (static) webhook URLs (sessions) to ensure that the sent request will be captured even
without data persistence?**

Yes, simply use the `--auto-create-sessions` flag or set the `AUTO_CREATE_SESSIONS=true` environment variable. In
`v1`, you needed to define sessions during app startup to enable this functionality. However, since `v2`, all you
need to do is enable this feature. It works quite simply - if the incoming request contains a UUID-formatted prefix
(e.g., `http://app/11111111-2222-3333-4444-555555555555/...`), a session for this request will be created
automatically. All that's left for you to do is open the session in the UI
(`http://app/s/11111111-2222-3333-4444-555555555555`).

## 🧩 Installation & running

This fork is meant to be **built from source and self-hosted**. The recommended way to run it is with Docker,
alongside a Redis server.

### Build the image

```shell
git clone https://github.com/pavel-kruchina/open-webhook.git
cd open-webhook
docker build -t open-webhook .
```

The multi-stage build compiles the Go binary together with the ReactJS frontend, so you don't need a Go or Node
toolchain on the host.

### Run it

The app requires a **Redis** server (for storing requests and sessions) and a writable **files directory** (for
files uploaded via `multipart/form-data`):

```shell
# start a Redis server
docker run -d --name wh-redis redis:8-alpine

# start the app: link Redis, mount a files directory, and point --files-dir at it
docker run --rm -t -p "8080:8080/tcp" \
  --link wh-redis \
  -v "$(pwd)/wh-files:/data/files" \
  -e REDIS_DSN="redis://wh-redis:6379/0" \
  open-webhook start --files-dir /data/files
```

> [!NOTE]
> This starts the app on port `8080` (the first port in the `-p` argument is the host port, and the second is the
> application port inside the container). `--files-dir` is required and `REDIS_DSN` must point to a reachable Redis
> server.

Next, open your browser at [`localhost:8080`](http://localhost:8080) to begin inspecting your webhooks. To stop the
app, press `Ctrl+C` in the terminal where it's running.

For custom configuration options, refer to the CLI help below or run the app with the `--help` flag.

### ⚠️ A note on pre-built artifacts (upstream vs. this fork)

> [!IMPORTANT]
> This fork does **not** currently publish its own binaries, container images, or Helm release. The upstream
> project's pre-built artifacts install the **upstream** application, which does **not** include this fork's
> Redis-backed persistence or `multipart/form-data` file capture:
>
> - **Binaries** — the upstream [releases page][link_releases] serves `webhook-tester` binaries for **linux**,
>   **darwin** (macOS), and **windows** (`amd64`/`arm64`).
> - **Container images** — `ghcr.io/tarampampam/webhook-tester` ([GHCR][link_ghcr]) and
>   `tarampampam/webhook-tester` ([Docker Hub][link_docker_hub], mirror) are the **upstream** images.
> - **Helm chart** — the [ArtifactHUB chart][artifact-hub] (and the copy under `deployments/helm` in this repo)
>   default to the **upstream** image.
>
> Until fork-specific releases are published, **build from source** as shown above to get this fork's features.

[link_releases]:https://github.com/tarampampam/webhook-tester/releases
[link_ghcr]:https://github.com/users/tarampampam/packages/container/package/webhook-tester
[link_docker_hub]:https://hub.docker.com/r/tarampampam/webhook-tester/
[artifact-hub]:https://artifacthub.io/packages/helm/webhook-tester/webhook-tester

<!--GENERATED:CLI_DOCS-->
<!-- Documentation inside this block generated by github.com/urfave/cli-docs/v3; DO NOT EDIT -->
## CLI interface

open-webhook.

Usage:

```bash
$ app [GLOBAL FLAGS] [COMMAND] [COMMAND FLAGS] [ARGUMENTS...]
```

Global flags:

| Name               | Description                                 | Type   | Default value | Environment variables |
|--------------------|---------------------------------------------|--------|:-------------:|:---------------------:|
| `--log-level="…"`  | Logging level (debug/info/warn/error/fatal) | string |   `"info"`    |      `LOG_LEVEL`      |
| `--log-format="…"` | Logging format (console/json)               | string |  `"console"`  |     `LOG_FORMAT`      |

### `start` command (aliases: `s`, `server`, `serve`, `http-server`)

Start HTTP/HTTPs servers.

Usage:

```bash
$ app [GLOBAL FLAGS] start [COMMAND FLAGS] [ARGUMENTS...]
```

The following flags are supported:

| Name                          | Description                                                                                                                                                  | Type     |        Default value         |    Environment variables     |
|-------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|:----------------------------:|:----------------------------:|
| `--addr="…"`                  | IP (v4 or v6) address to listen on (0.0.0.0 to bind to all interfaces)                                                                                       | string   |         `"0.0.0.0"`          | `SERVER_ADDR`, `LISTEN_ADDR` |
| `--port="…"`                  | HTTP server port                                                                                                                                             | uint     |            `8080`            |         `HTTP_PORT`          |
| `--read-timeout="…"`          | maximum duration for reading the entire request, including the body (zero = no timeout)                                                                      | duration |            `1m0s`            |     `HTTP_READ_TIMEOUT`      |
| `--write-timeout="…"`         | maximum duration before timing out writes of the response (zero = no timeout)                                                                                | duration |            `1m0s`            |     `HTTP_WRITE_TIMEOUT`     |
| `--idle-timeout="…"`          | maximum amount of time to wait for the next request (keep-alive, zero = no timeout)                                                                          | duration |            `1m0s`            |     `HTTP_IDLE_TIMEOUT`      |
| `--session-ttl="…"`           | session TTL (time-to-live, lifetime)                                                                                                                         | duration |          `168h0m0s`          |        `SESSION_TTL`         |
| `--max-requests="…"`          | maximal number of requests to store in the storage (zero means unlimited)                                                                                    | uint     |            `128`             |        `MAX_REQUESTS`        |
| `--files-dir="…"`             | path to the directory for storing files uploaded via multipart/form-data (must exist and be writable)                                                        | string   |                              |         `FILES_DIR`          |
| `--max-request-body-size="…"` | maximal webhook request body size (in bytes), zero means unlimited                                                                                           | uint     |             `0`              |   `MAX_REQUEST_BODY_SIZE`    |
| `--auto-create-sessions`      | automatically create sessions for incoming requests                                                                                                          | bool     |           `false`            |    `AUTO_CREATE_SESSIONS`    |
| `--pubsub-driver="…"`         | pub/sub driver (memory/redis)                                                                                                                                | string   |          `"memory"`          |       `PUBSUB_DRIVER`        |
| `--tunnel-driver="…"`         | tunnel driver to expose your locally running app to the internet (ngrok, empty to disable)                                                                   | string   |                              |       `TUNNEL_DRIVER`        |
| `--ngrok-auth-token="…"`      | ngrok authentication token (required for ngrok tunnel; create a new one at https://dashboard.ngrok.com/authtokens/new)                                       | string   |                              |      `NGROK_AUTHTOKEN`       |
| `--public-url-root="…"`       | public URL root override for webhook URLs (e.g., http://webhook-tester.k8s.internal); if not set, the URL shown in the UI is based on the browser's location | string   |                              |      `PUBLIC_URL_ROOT`       |
| `--redis-dsn="…"`             | redis-like (redis, keydb) server DSN (e.g. redis://user:pwd@127.0.0.1:6379/0 or unix://user:pwd@/path/to/redis.sock?db=0)                                    | string   | `"redis://127.0.0.1:6379/0"` |         `REDIS_DSN`          |
| `--shutdown-timeout="…"`      | maximum duration for graceful shutdown                                                                                                                       | duration |            `15s`             |      `SHUTDOWN_TIMEOUT`      |
| `--use-live-frontend`         | use frontend from the local directory instead of the embedded one (useful for development)                                                                   | bool     |           `false`            |            *none*            |

### `start healthcheck` subcommand (aliases: `hc`, `health`, `check`)

Health checker for the HTTP(S) servers. Use case - docker healthcheck.

Usage:

```bash
$ app [GLOBAL FLAGS] start healthcheck [COMMAND FLAGS] [ARGUMENTS...]
```

The following flags are supported:

| Name         | Description      | Type | Default value | Environment variables |
|--------------|------------------|------|:-------------:|:---------------------:|
| `--port="…"` | HTTP server port | uint |    `8080`     |      `HTTP_PORT`      |

<!--/GENERATED:CLI_DOCS-->

## Credits

**open-webhook** is a fork of [`tarampampam/webhook-tester`](https://github.com/tarampampam/webhook-tester).
The original application was created and is maintained by Paramtamtam — please star and support the upstream project.

## License

This is open-sourced software licensed under the [MIT License][link_license].

[link_license]:https://github.com/tarampampam/webhook-tester/blob/master/LICENSE
