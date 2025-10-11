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

## Configuration (`services.toml`)

`go-overlay` uses a `services.toml` file to define the services it should manage.

### Global Timeouts

You can specify global timeouts in a `[timeouts]` block. These are the defaults implemented in the code:

```toml
[timeouts]
# Time to wait after a service starts before running its `pos_script`.
post_script_timeout = 7

# Max time for a service to shut down gracefully before being killed.
service_shutdown_timeout = 10

# Max time for the entire shutdown sequence to complete.
global_shutdown_timeout = 30

# Max time to wait for a dependency to start.
dependency_wait_timeout = 300
```

### Service Definition

Each service is defined in a `[[services]]` block. Supported fields below reflect the current implementation:

```toml
[[services]]
# A unique name for the service. Used for logging and CLI commands. (Required)
name = "my-app"

# The command to execute. (Required)
command = "/usr/local/bin/my-app-binary"

# A list of arguments to pass to the command. (Optional)
args = ["--config", "/etc/my-app.conf", "--verbose"]

# If provided, go-overlay will tail this file instead of attaching a PTY. (Optional)
# log_file = "/var/log/my-app.log"

# A shell script to execute before starting the main command. (Optional)
pre_script = "/scripts/setup-app.sh"

# A shell script to execute after the service is considered started (runs after post_script_timeout). (Optional)
pos_script = "/scripts/notify-startup.sh"

# Name of a dependency that must be started before this service. (Optional)
depends_on = "database"

# Extra delay (in seconds) after dependency is up, before starting this service. (Optional)
wait_after = 5

# If omitted, defaults to true. (Optional)
enabled = true

# If true, a failure of this service triggers a graceful shutdown of all services. (Optional, default: false)
required = false

# Run the service as a specific user (uses `su`). (Optional)
user = "www-data"
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

## Examples

We provide a dedicated `examples/` directory with practical configurations:

- **[Simple Web Server](./examples/simple-web-server/README.md)**: Minimal config to run a single service.
- **[Web Stack with Dependencies](./examples/web-stack-deps/README.md)**: Control startup order using `depends_on`.
- **[Background Worker](./examples/background-worker/README.md)**: Use `required` for critical vs. non-critical services.
- **[Advanced Features](./examples/advanced-features/README.md)**: `pre_script`, `pos_script`, `user`, and `log_file` usage.

## Development

This project uses `invoke` with `mise` for task management.

```bash
# List available tasks
mise exec -- invoke --list

# Build the Go binary for your local OS
mise exec -- invoke go.build

# Install/uninstall the binary
mise exec -- invoke install
mise exec -- invoke uninstall

# Build the Docker image
mise exec -- invoke docker.build

# Run tests
mise exec -- invoke go.test
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
