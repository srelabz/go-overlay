# TM Orchestrator, a Task Manager for containerized environments

A modern and lightweight task orchestrator designed for containerized environments. Built to simplify and enhance the functionality provided by tools like Supervisor, S6-Overlay, and OpenRC, it delivers a streamlined approach to managing processes, dependencies, and service orchestration in Docker containers.

## Features
- Simplified Orchestration: Easy-to-use configuration with a single TOML file for defining services, dependencies, and pre-scripts.
- Lightweight: Minimal resource usage and a single static binary for execution.
- Dependency Management: Supports service dependencies and delay-based execution for better startup control.
- Log Aggregation: Captures logs from stdout, stderr, or specified log files with service-level prefixes.
- Pre-Script Execution: Run custom scripts before starting services for initialization tasks.

## Configuration

### Example `services.toml`
```toml
[[services]]
name = "salt-master"
command = "/usr/bin/salt-master"
args = ["--log-level=debug"]
log_file = "/var/log/salt/master"           # Optional
pre_script = "/scripts/init-salt-master.sh" # Optional

[[services]]
name = "salt-api"
command = "/usr/bin/salt-api"
args = ["-l", "debug"]
depends_on = "salt-master"                  # Optional
wait_after = 10                             # Optional
pre_script = "/scripts/init-salt-api.sh"    # Optional
```

## Install on Dockerfile
```Dockerfile
COPY services.toml /services.toml
RUN curl -sSL https://raw.githubusercontent.com/tarcisiomiranda/tm-overlay/main/install.sh | sh
ENTRYPOINT ["/entrypoint"]
```

## Develompent Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/tarcisiomiranda/tm-overlay.git
   cd tm-overlay
   ```
2. Build the binary:
   ***x64***
   ```bash
   VERSION=$(git describe --tags --always)
   CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-X main.version=$VERSION" -o service-manager .
   ```
   ***ARM***
   ```bash
   GOOS=linux GOARCH=arm GOARM=7 go build -o entrypoint main.go
   ```
3. Run the application:
   ```bash
   ./entrypoint
   ```

## Usage
1. Define your services in a `services.toml` file.
2. (Optional) Add pre-scripts and dependencies as needed.
3. Run the binary to start managing your services.

## Logs
- Logs are prefixed with the service name for easy identification.
- If a `log_file` is specified in `services.toml`, the application tails the file and displays the logs in real-time.

## License
This project is licensed under the MIT License.
