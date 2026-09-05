# go-overlay

**Init system and process supervisor for Docker containers.** Run multiple services
in a single container with startup ordering, health checks, restart policies,
graceful shutdown and PID 1 zombie reaping. One static Go binary, one TOML file, no
runtime dependencies.

[![CI](https://github.com/corebunker/go-overlay/actions/workflows/ci.yml/badge.svg)](https://github.com/corebunker/go-overlay/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/corebunker/go-overlay?sort=semver)](https://github.com/corebunker/go-overlay/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)

Drop it in as your container `ENTRYPOINT` and it supervises everything else: an API,
a worker, a database, a reverse proxy, migrations that must finish first. It is a
lightweight alternative to s6-overlay and supervisord, and it replaces tini when you
need more than signal forwarding.

## Quick Start

```dockerfile
FROM debian:bookworm-slim

ADD https://github.com/corebunker/go-overlay/releases/latest/download/go-overlay /go-overlay
RUN chmod 0755 /go-overlay

COPY services.toml /services.toml
ENTRYPOINT ["/go-overlay"]
```

Every release also publishes `go-overlay.sha256`. To verify the download:

```dockerfile
ARG BINARY_SHA256=<sha256 from the release page>
RUN echo "${BINARY_SHA256}  /go-overlay" | sha256sum -c -
```

```toml
# services.toml
[[services]]
name = "app"
command = "/app/server"
required = true
restart = "on-failure"

[services.health_check]
endpoint = "http://localhost:8080/health"
interval = 30
```

```bash
docker build -t myapp .
docker run myapp
```

## Features

- **Dependencies**: `depends_on` + `wait_after` for startup ordering
- **One-shot Jobs**: `oneshot = true` for migrations and init scripts
- **Health Checks**: HTTP endpoint or command-based monitoring
- **Restart Policies**: `never`, `on-failure`, `always` with max attempts
- **Environment**: Inline `env`, `env_file`, runtime overrides via `GO_OVERLAY_ENABLE_*`
- **Graceful Shutdown**: SIGTERM → wait → SIGKILL of the whole process group, with configurable timeouts
- **Pre/Post Scripts**: Run scripts before/after service lifecycle
- **User Switching**: Run services as specific users via setuid/setgid
- **PID 1 Ready**: Reaps orphaned processes when running as PID 1
- **CLI Management**: `list`, `status`, `restart` via IPC

## Why go-overlay

A Docker image runs one command. The moment you need a second process next to it, an
API plus a worker, a web server plus a background job, a migration that must finish
before the app boots, you are writing a shell script as PID 1. Shell scripts do not
forward signals correctly, do not reap zombies, do not restart a crashed process, and
do not know that the API should wait for Postgres.

go-overlay is that missing piece, in one static binary:

- **Correct PID 1 behavior**: reaps orphaned processes so long-lived containers do not
  leak zombies, and forwards termination to the whole process group.
- **Startup ordering**: `depends_on` plus `wait_after` for services that must come up
  in a specific order.
- **One-shot jobs**: run database migrations or seed scripts to completion before the
  services that depend on them start.
- **Health checks**: HTTP endpoint or shell command, with retries and restart on
  failure.
- **Graceful shutdown**: SIGTERM, wait, then SIGKILL, with per-service and global
  timeouts, so a `docker stop` does not corrupt state.
- **Runtime control**: `go-overlay list`, `status` and `restart <service>` inside the
  running container.

## Comparison

| | go-overlay | s6-overlay | supervisord | tini |
|---|---|---|---|---|
| Added to the image | one static binary (~8 MB) | s6 suite + execline | Python runtime + package | one small binary |
| Configuration | single TOML file | service directories and scripts | INI file | none |
| Supervises many services | yes | yes | yes | no, single process |
| Startup ordering | `depends_on`, `wait_after` | dependency files | `priority` | n/a |
| One-shot jobs | `oneshot = true` | oneshot service type | no | n/a |
| Built-in health checks | HTTP and command | no | no, needs event listeners | n/a |
| Restart policies | never, on-failure, always, `max_restarts` | yes | yes | n/a |
| Reaps orphan processes as PID 1 | yes | yes | yes | yes |
| Graceful shutdown timeouts | per service and global | yes | yes | signal forwarding only |
| Runtime CLI | `list`, `status`, `restart` | `s6-svc` | `supervisorctl` | n/a |
| Language | Go | C and execline | Python | C |

Pick s6-overlay when you want a battle-tested supervision suite and do not mind its
learning curve. Pick tini when you truly have a single process. Pick go-overlay when
you want several services described in one readable file, with no interpreter added
to the image.

## Use Cases

- **Full stack in one container**: API, frontend and Caddy or Nginx behind one image,
  the way the `examples/` directory shows.
- **Migrations before boot**: run `migrate up` as a one-shot job and let the API wait
  for it.
- **Legacy applications**: an app that expects cron, a log shipper or a queue worker
  alongside it.
- **Development and CI images**: reproduce a small production topology without
  docker-compose or Kubernetes.
- **Edge and on-premise**: single-host deployments where a full orchestrator is more
  machinery than the workload deserves.

## CLI

```bash
go-overlay                       # Start supervisor (reads /services.toml)
go-overlay --config /app/s.toml  # Start with a different config file
go-overlay --debug               # Start with environment dump (secrets redacted)
go-overlay list                  # List services with status, PID, uptime
go-overlay status                # Show system summary
go-overlay restart <service>     # Restart a service
go-overlay install               # Install CLI to /usr/local/bin/
```

The supervisor exits with status `1` when a `required` service fails, so container
orchestrators see the failure. A signal-driven shutdown exits with `0`.

The CLI talks to the supervisor over a Unix socket at `/run/go-overlay.sock`
(falling back to `/tmp/go-overlay.sock` when `/run` is not writable). The socket is
created with mode `0600`, so only the user running the supervisor can control it.
Override the location with `GO_OVERLAY_SOCKET`.

## Configuration

### Timeouts

```toml
[timeouts]
post_script_timeout = 7        # pos_script max duration (default: 7)
service_shutdown_timeout = 10   # SIGTERM → SIGKILL per service (default: 10)
global_shutdown_timeout = 30    # Total shutdown timeout (default: 30)
dependency_wait_timeout = 300   # Max wait for dependencies (default: 300)
```

### Service Fields

```toml
[[services]]
name = "api"                            # Required. Unique identifier
command = "/app/server"                 # Required. Executable path
args = ["--port", "8080"]               # Command arguments
enabled = true                          # Start this service (default: true)
required = false                        # Shutdown system on failure (default: false)
oneshot = false                         # Run once, ready after exit 0 (default: false)
depends_on = ["db", "redis"]            # Wait for these services
wait_after = 3                          # Seconds after deps ready (or map: { db = 5, redis = 2 })
user = "appuser"                        # Run as this user (requires root; no shell involved)
log_file = "/var/log/api.log"           # Redirect stdout/stderr to a file instead of the PTY
pre_script = "/scripts/init.sh"         # Run before start
pos_script = "/scripts/cleanup.sh"      # Run after start
env = { KEY = "value" }                 # Inline env vars
env_file = "/app/.env"                  # Load from .env file
restart = "never"                       # never | on-failure | always
restart_delay = 1                       # Seconds between restarts (default: 1)
max_restarts = 0                        # 0 = unlimited (default: 0)

[services.health_check]
endpoint = "http://localhost:8080/health"  # HTTP check (2xx/3xx = healthy)
command = "pg_isready"                     # OR command check (exit 0 = healthy)
interval = 30                              # Seconds between checks (default: 30)
retries = 3                                # Failures before unhealthy (default: 3)
timeout = 5                                # Per-check timeout (default: 5)
start_delay = 10                           # Delay before first check (default: 10)
```

## Service Selection via ENV

```bash
GO_OVERLAY_ONLY_SERVICES="backend,redis" go-overlay     # Only these services
GO_OVERLAY_ENABLE_FASTAPI_BACKEND=true go-overlay        # Enable specific service
GO_OVERLAY_DISABLE_CADDY_FRONTEND=true go-overlay        # Disable specific service
```

Service names are uppercased with non-alphanumeric chars replaced by `_`.

## Supervisor Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `GO_OVERLAY_SOCKET` | `/run/go-overlay.sock` | IPC socket path used by the CLI |
| `GO_OVERLAY_AUTO_INSTALL` | unset (off) | Symlink the binary into `/usr/local/bin` at startup |
| `GO_OVERLAY_ONLY_SERVICES` | unset | Comma-separated allowlist of services to start |

Auto-install is opt-in. Run `go-overlay install` explicitly, or set
`GO_OVERLAY_AUTO_INSTALL=true` to restore the previous startup behavior.

## Complete Example

```toml
[timeouts]
service_shutdown_timeout = 10
global_shutdown_timeout = 30

[[services]]
name = "postgres"
command = "postgres"
args = ["-D", "/var/lib/postgresql/data"]
required = true

[[services]]
name = "migrate"
command = "/app/migrate"
args = ["up"]
depends_on = ["postgres"]
wait_after = 3
oneshot = true

[[services]]
name = "redis"
command = "redis-server"
required = true

[[services]]
name = "api"
command = "/app/server"
depends_on = ["postgres", "redis", "migrate"]
wait_after = { postgres = 3, redis = 1 }
required = true
restart = "on-failure"
max_restarts = 5
env = { DATABASE_URL = "postgres://localhost/app" }

[services.health_check]
endpoint = "http://localhost:8080/health"
interval = 30

[[services]]
name = "worker"
command = "/app/worker"
depends_on = ["redis"]
restart = "always"

[[services]]
name = "caddy"
command = "caddy"
args = ["run", "--config", "/etc/caddy/Caddyfile"]
depends_on = ["api"]
wait_after = 3
required = true
```

## Examples

Production-ready stacks in `examples/`:

| Stack | Use Case |
|-------|----------|
| [FastAPI + React + Caddy](./examples/fastapi-react-stack/) | Python REST APIs, ML backends |
| [Express + React + Caddy](./examples/express-react-stack/) | JavaScript full-stack |
| [Bun + React + Caddy](./examples/bun-react-stack/) | Max performance, native TypeScript |
| [Next.js Standalone](./examples/nextjs-standalone/) | SSR/SSG, SEO-focused apps |
| [Production Stack](./examples/production-stack/) | Full stack with PostgreSQL + Redis |

## Development

```bash
mise exec -- invoke go.build       # Build binary
mise exec -- invoke go.test        # Run tests
mise exec -- invoke quality.fmt    # Format code
mise exec -- invoke --list         # All tasks
```

## FAQ

### How do I run multiple processes in a Docker container?

Set go-overlay as the `ENTRYPOINT` and describe each process in `/services.toml`. It
starts them, keeps them alive, streams their logs with a prefix per service, and stops
them cleanly when the container is asked to shut down. See [Quick Start](#quick-start).

### Is running more than one service per container bad practice?

The one-process-per-container rule exists because Docker only supervises PID 1. When
several processes genuinely belong to the same unit of deployment, a supervisor that
handles signals, restarts and reaping gives you the same guarantees inside a single
image. That is what go-overlay provides.

### Do I still need tini or an init process?

No. When go-overlay runs as PID 1 it reaps orphaned processes itself, so you do not
need `docker run --init` or tini in the image.

### How is this different from s6-overlay?

Same job, different trade-offs. s6-overlay is a mature supervision suite configured
through service directories and execline scripts. go-overlay is a single Go binary
configured by one TOML file, with health checks and dependency ordering built in. See
the [comparison table](#comparison).

### How do I wait for a database before starting the API?

Declare the dependency and, if the service needs a grace period after the dependency
reports ready, add `wait_after`:

```toml
[[services]]
name = "api"
depends_on = ["postgres"]
wait_after = { postgres = 3 }
```

### How do I run database migrations before the application?

Mark the migration as a one-shot job. Dependents wait until it exits successfully:

```toml
[[services]]
name = "migrate"
command = "/app/migrate"
args = ["up"]
oneshot = true
```

### What happens when a service crashes?

It follows the service `restart` policy: `never`, `on-failure` or `always`, bounded by
`max_restarts` and spaced by `restart_delay`. If the service is marked `required`, the
supervisor shuts everything down and exits with status 1 so your orchestrator restarts
the container.

### Can services run as a non-root user?

Yes. Set `user = "appuser"` and the process is started with that user's uid, gid and
supplementary groups. The supervisor itself must run as root to switch users.

### How do I inspect services inside a running container?

```bash
docker exec -it mycontainer go-overlay list
docker exec -it mycontainer go-overlay status
docker exec -it mycontainer go-overlay restart api
```

### Does it work outside Docker?

Yes. It is a plain Linux binary, so it also supervises processes on a VM or a
bare-metal host, but graceful shutdown and reaping are designed with containers in
mind.

## Contributing

Issues and pull requests are welcome. The development workflow, the CI pipeline and
the release process are documented in [docs/CI-CD-PIPELINE.md](./docs/CI-CD-PIPELINE.md).

## License

MIT License - see [LICENSE](./LICENSE) file.

---

**Keywords**: docker process supervisor, container init system, run multiple processes
in one docker container, s6-overlay alternative, supervisord alternative, tini
alternative, PID 1 zombie reaping, container service manager, docker entrypoint
multiple services, graceful shutdown, health checks, service dependencies, Go, TOML.
