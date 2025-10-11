# Go Overlay

Go-based service orchestrator inspired by s6-overlay for running multiple services in containers. Provides graceful shutdown, dependency management, and CLI control.

## Features

- ✅ **Auto-Installation**: Automatically installs itself in PATH when running in daemon mode
- ✅ **Graceful Shutdown**: Signal handling with configurable timeouts
- ✅ **Service State Management**: Real-time state tracking and reporting
- ✅ **Configuration Validation**: Comprehensive validation with circular dependency detection
- ✅ **CLI Commands**: Easy service management via IPC
- ✅ **Dependency Management**: Service startup ordering and waiting
- ✅ **PTY Support**: Proper log streaming with service name prefixes
- ✅ **User Switching**: Run services as different users
- ✅ **Health Monitoring**: Service failure detection and system shutdown on critical services

## Quick Start

### Download and Install
```bash
# Download latest release
curl -L https://github.com/srelabz/go-overlay/releases/latest/download/service-manager -o go-overlay
chmod +x go-overlay

# Auto-install in PATH (creates symlink at /go-overlay)
sudo ./go-overlay install

# Now you can use from anywhere:
go-overlay list
go-overlay status
go-overlay restart nginx
```

### Docker Usage
```dockerfile
FROM alpine:latest

# Download go-overlay directly from GitHub releases
ADD https://github.com/srelabz/go-overlay/releases/latest/download/service-manager /go-overlay
RUN chmod +x /go-overlay

# Copy your service configuration
COPY services.toml /services.toml

# Set as entrypoint
ENTRYPOINT ["/go-overlay"]
```

## CLI Commands

```bash
go-overlay                    # Start daemon
go-overlay list               # List services
go-overlay status             # Show status
go-overlay restart <service>  # Restart service
go-overlay install            # Manual installation
```

## Configuration

### Timeouts

The following timeouts are supported by the configuration and have default values in the code when not provided:

- **post_script_timeout**: time to wait after the service starts before running the `pos_script` (default: 7s)
- **service_shutdown_timeout**: maximum time for each service to exit gracefully after receiving SIGTERM, before a forced kill is applied (default: 10s)
- **global_shutdown_timeout**: maximum time for the overall shutdown to complete before forcing termination of remaining services (default: 30s)
- **dependency_wait_timeout**: maximum time to wait for a dependency to be marked as started before giving up (default: 300s)

Example timeouts configuration:

```toml
[timeouts]
post_script_timeout = 7
service_shutdown_timeout = 10
global_shutdown_timeout = 30
dependency_wait_timeout = 300
```

```toml
[[services]]
name = "nginx"
command = "/usr/sbin/nginx"
pre_script = "/scripts/pma-auth.sh"
args = ["-g", "'daemon off;'"]

[[services]]
name = "node-app"
command = "/usr/bin/npm"
args = ["--prefix", "/srv/nodejs", "run", "dev"]

[[services]]
name = "php-fpm"
# user = "www-data"
command = "/usr/local/sbin/php-fpm"
pre_script = "/scripts/php-fpm.sh"

[[services]]
name = "mariadb"
command = "/usr/bin/mysqld"
pre_script = "/scripts/mariadb.sh"
pos_script = "/scripts/is_empty.py"
args = [
  "--user=mysql",
  "--console",
  "--skip-name-resolve",
  "--skip-networking=0"
]

[[services]]
name    = "fastapi"
command = "uvicorn"
args = [
  "main:app",
  "--app-dir=/srv/python",
  "--host=0.0.0.0",
  "--port=8012",
  "--reload",
  "--log-level=debug"
]
```

## Auto-Installation

When running in daemon mode, Go Overlay automatically:
1. Detects if it's already in a PATH directory
2. Creates a symlink at `/go-overlay`
3. Enables CLI commands from any location

## Service States

- **PENDING**: Initial state, not yet started
- **STARTING**: Currently being started
- **RUNNING**: Successfully running
- **STOPPING**: Gracefully stopping
- **STOPPED**: Successfully stopped
- **FAILED**: Failed to start or crashed

## Documentation

- **[Quick Install Guide](docs/QUICK-INSTALL.md)** - Installation methods and examples
- **[CLI Commands Reference](docs/CLI-COMMANDS.md)** - Complete CLI command documentation
- **[Graceful Shutdown Testing](docs/TEST-GRACEFUL-SHUTDOWN.md)** - Testing shutdown behavior

## Roadmap

### ✅ Implemented Features

- **Graceful Shutdown** - Complete signal handling with configurable timeouts
- **Service State Management** - Real-time state tracking and reporting
- **Configuration Validation** - Comprehensive validation with circular dependency detection
- **CLI Commands** - Remote service management via IPC (list, restart, status)
- **Auto-Installation** - Automatic PATH installation for seamless Docker usage
- **Dependency Management** - Service startup ordering and waiting
- **PTY Support** - Proper log streaming with service name prefixes
- **User Switching** - Run services as different users
- **Basic Health Monitoring** - Service failure detection and system shutdown

### 🚧 Planned Features

#### High Priority
- [ ] **Health Checks** - HTTP/TCP health checks with configurable intervals
- [ ] **Restart Policies** - Automatic restart on failure with backoff strategies

#### Medium Priority
- [ ] **Resource Monitoring** - CPU/Memory usage tracking per service
- [ ] **Metrics Integration** - Prometheus metrics endpoint

#### Low Priority
- [ ] **Web UI** - Browser-based service management interface
- [ ] **Cron Scheduling** - Time-based service execution

## Examples

### Web Application Stack

```toml
[[services]]
name = "redis"
command = "/usr/bin/redis-server"
enabled = true

[[services]]
name = "db-migrate"
command = "/app/migrate"
depends_on = "redis"
wait_after = 2
enabled = true

[[services]]
name = "web"
command = "/app/server"
args = ["--port", "8080"]
depends_on = "db-migrate"
wait_after = 1
enabled = true
required = true
```

### Docker Integration

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y nginx curl procps

COPY go-overlay /go-overlay
COPY services.toml /services.toml

RUN chmod +x /go-overlay

ENTRYPOINT ["go-overlay"]
```

## Development

```bash
make build
make install
make uninstall
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
