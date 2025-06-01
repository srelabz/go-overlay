# TM Orchestrator

Go-based service orchestrator like s6-overlay for running multiple services in containers. Provides graceful shutdown, dependency management, and CLI control.

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

## CLI Commands

```bash
tm-orchestrator
tm-orchestrator list
tm-orchestrator status
tm-orchestrator restart <service-name>
tm-orchestrator install
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

## Docker Integration

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y nginx curl procps

COPY service-manager /usr/local/bin/tm-orchestrator
COPY services.toml /services.toml

RUN chmod +x /usr/local/bin/tm-orchestrator

ENTRYPOINT ["tm-orchestrator"]
```

## Development

```bash
make build
make install
make uninstall
```

## Service States

- **PENDING**: Initial state, not yet started
- **STARTING**: Currently being started
- **RUNNING**: Successfully running
- **STOPPING**: Gracefully stopping
- **STOPPED**: Successfully stopped
- **FAILED**: Failed to start or crashed

## Graceful Shutdown

TM Orchestrator handles SIGTERM, SIGINT, SIGHUP signals:
1. Cancel context to signal all services
2. Send SIGTERM to each service
3. Wait for graceful stop (configurable timeout)
4. Force kill if necessary
5. Clean up resources

## Auto-Installation

When running in daemon mode, TM Orchestrator automatically:
1. Detects if it's already in a PATH directory
2. Creates a symlink at `/usr/local/bin/tm-orchestrator`
3. Creates compatibility symlink as `entrypoint`
4. Enables CLI commands from any location

## Validation

Configuration is validated for:
- Required fields (name, command)
- Service name format (alphanumeric, dash, underscore)
- Command existence in PATH or as absolute path
- Script file existence
- Log directory existence
- Dependency existence
- Circular dependency detection
- User existence
- Reasonable timeout values

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

## Roadmap

### ✅ Implemented Features

- **Graceful Shutdown** - Complete signal handling with configurable timeouts
- **Service State Management** - Real-time state tracking and reporting
- **Configuration Validation** - Comprehensive validation with circular dependency detection
- **CLI Commands** - Remote service management via IPC (list, restart, status)
- **Configurable Timeouts** - Customizable timeout settings for all operations
- **Auto-Installation** - Automatic PATH installation for seamless Docker usage
- **Dependency Management** - Service startup ordering and waiting
- **PTY Support** - Proper log streaming with service name prefixes
- **User Switching** - Run services as different users
- **Basic Health Monitoring** - Service failure detection and system shutdown

### 🚧 Planned Features

#### High Priority
- [ ] **Health Checks** - Periodic health validation for services
  - HTTP/TCP health endpoints
  - Custom health check scripts
  - Configurable check intervals and thresholds
- [ ] **Restart Policies** - Configurable service restart behavior
  - `always`, `on-failure`, `unless-stopped` policies
  - Exponential backoff and rate limiting
  - Maximum restart attempts

#### Medium Priority
- [ ] **Resource Monitoring** - Track CPU, memory, and disk usage per service
- [ ] **Metrics Integration** - Prometheus metrics export
- [ ] **Environment Variable Substitution** - Dynamic config with env vars
- [ ] **Configuration Hot Reload** - Reload config without restart
- [ ] **Log Rotation** - Automatic log file management
- [ ] **Service Templates** - Reusable service configurations

#### Low Priority
- [ ] **Web UI** - Browser-based management interface
- [ ] **Cron-like Scheduling** - Time-based service execution
- [ ] **Backup/Restore** - Configuration and state management
- [ ] **Plugin System** - Extensible architecture for custom functionality
- [ ] **Multi-host Support** - Distributed service orchestration

### 🐛 Known Issues

- [ ] Monitor service date command not expanding properly in bash args
- [ ] Some log spacing issues in state transitions
- [ ] Error handling could be more granular in restart scenarios

### 💡 Contributions Welcome

We're actively looking for contributors to help implement these features! 
Check our [Contributing](#contributing) section for guidelines.

## License

MIT License - see LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and feature requests, please use the GitHub issue tracker.
