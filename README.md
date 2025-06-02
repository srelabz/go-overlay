# Go Supervisor

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
curl -L https://github.com/tarcisiomiranda/go-entrypoint/releases/latest/download/service-manager -o go-supervisor
chmod +x go-supervisor

# Auto-install in PATH (creates symlink at /go-supervisor)
sudo ./go-supervisor install

# Now you can use from anywhere:
go-supervisor list
go-supervisor status
go-supervisor restart nginx
```

### Docker Usage
```dockerfile
FROM alpine:latest

# Download go-supervisor directly from GitHub releases
ADD https://github.com/tarcisiomiranda/go-entrypoint/releases/latest/download/service-manager /go-supervisor
RUN chmod +x /go-supervisor

# Copy your service configuration
COPY services.toml /services.toml

# Set as entrypoint
ENTRYPOINT ["/go-supervisor"]
```

## CLI Commands

```bash
go-supervisor                    # Start daemon
go-supervisor list               # List services
go-supervisor status             # Show status
go-supervisor restart <service>  # Restart service
go-supervisor install            # Manual installation
```

## Configuration

```toml
[timeouts]
post_script_timeout = 5
service_shutdown_timeout = 15
global_shutdown_timeout = 45
dependency_wait_timeout = 120

[[services]]
name = "nginx"
command = "/usr/sbin/nginx"
args = ["-g", "daemon off;"]
enabled = true
required = false

[[services]]
name = "app"
command = "/usr/local/bin/myapp"
depends_on = "nginx"
wait_after = 3
enabled = true
required = true
user = "appuser"

[[services]]
name = "worker"
command = "/usr/local/bin/worker"
pre_script = "/scripts/setup-worker.sh"
pos_script = "/scripts/post-worker.sh"
log_file = "/var/log/worker.log"
enabled = true
```

## Auto-Installation

When running in daemon mode, Go Supervisor automatically:
1. Detects if it's already in a PATH directory
2. Creates a symlink at `/go-supervisor`
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

COPY go-supervisor /go-supervisor
COPY services.toml /services.toml

RUN chmod +x /go-supervisor

ENTRYPOINT ["go-supervisor"]
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
