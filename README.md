# open-webhook

> [!NOTE]
> **open-webhook** is a fork of [`tarampampam/webhook-tester`](https://github.com/tarampampam/webhook-tester) by
> Paramtamtam. All credit for the original application goes to the upstream author. This fork stores captured
> requests in Redis and adds storage & download of files uploaded via `multipart/form-data` (see below).

This application allows you to test and debug webhooks and HTTP requests using unique, randomly generated URLs. You
can customize the response code, `Content-Type` HTTP header, response content, and even set a delay for responses.

Consider it a free and self-hosted alternative to [webhook.site](https://github.com/fredsted/webhook.site),
[requestinspector.com](https://requestinspector.com/), and similar services.

<p align="center">
  <img src="https://github.com/user-attachments/assets/26e56d78-8a10-4883-9052-d18047206fda" alt="screencast" />
</p>

Built with Go for high performance, this application includes a lightweight UI (written in `ReactJS`) that’s compiled
into the binary, so no additional assets are required. WebSocket support provides real-time webhook notifications in
the UI - no need for third-party solutions like `pusher.com`!

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

Both UUIDs must be known to download a file, so they cannot be enumerated by brute force. Files are removed together
with their session (when it is deleted or expires); a background janitor reclaims the disk space of expired sessions.

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

## 🧩 Installation

Download the latest binary for your architecture from the [releases page][link_releases]. For example, to install
on an **amd64** system (e.g., Debian, Ubuntu):

[link_releases]:https://github.com/tarampampam/webhook-tester/releases

```shell
curl -SsL -o ./webhook-tester https://github.com/tarampampam/webhook-tester/releases/latest/download/webhook-tester-linux-amd64
chmod +x ./webhook-tester
./webhook-tester start
```

> [!TIP]
> Each release includes binaries for **linux**, **darwin** (macOS) and **windows** (`amd64` and `arm64` architectures).
> You can download the binary for your system from the [releases page][link_releases] (section `Assets`). And - yes,
> all what you need is just download and run single binary file.

Alternatively, you can use the Docker image:

| Registry                               | Image                                |
|----------------------------------------|--------------------------------------|
| [GitHub Container Registry][link_ghcr] | `ghcr.io/tarampampam/webhook-tester` |
| [Docker Hub][link_docker_hub] (mirror) | `tarampampam/webhook-tester`         |

> [!NOTE]
> It’s recommended to avoid using the `latest` tag, as **major** upgrades may include breaking changes.
> Instead, use specific tags in `X.Y.Z` format for version consistency.

To install it on Kubernetes (K8s), please use the Helm chart from [ArtifactHUB][artifact-hub].

[artifact-hub]:https://artifacthub.io/packages/helm/webhook-tester/webhook-tester

## ⚙ Usage

The app requires a **Redis** server (for storing requests) and a writable **files directory** (for uploaded files),
so the easiest way to run it is with the Docker image alongside Redis:

```shell
# start a Redis server
docker run -d --name wh-redis redis:8-alpine

# start the app: link Redis, mount a files directory, and point --files-dir at it
docker run --rm -t -p "8080:8080/tcp" \
  --link wh-redis \
  -v "$(pwd)/wh-files:/data/files" \
  -e REDIS_DSN="redis://wh-redis:6379/0" \
  ghcr.io/tarampampam/webhook-tester:2 start --files-dir /data/files
```

> [!NOTE]
> This starts the app on port `8080` (the first port in the `-p` argument is the host port, and the second is the
> application port inside the container). `--files-dir` is required and `REDIS_DSN` must point to a reachable Redis
> server.

Next, open your browser at [`localhost:8080`](http://localhost:8080) to begin testing your webhooks. To stop the app, press `Ctrl+C` in
the terminal where it's running.

For custom configuration options, refer to the CLI help below or execute the app with the `--help` flag.

[link_ghcr]:https://github.com/users/tarampampam/packages/container/package/webhook-tester
[link_docker_hub]:https://hub.docker.com/r/tarampampam/webhook-tester/

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
