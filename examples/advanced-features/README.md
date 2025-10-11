# Advanced Features Example

This example showcases several of `go-overlay`'s advanced configuration options, including running scripts before and after a service starts, running a service as a specific user, and redirecting logs to a file.

## `services.toml`

```toml
[[services]]
name = "advanced-service"
command = "/usr/bin/tail"
args = ["-f", "/var/log/app.log"]
user = "nobody"

# Scripts to run before and after the main command
pre_script = "/scripts/setup.sh"
pos_script = "/scripts/teardown.sh"

# If this is enabled, logs will go to the file instead of stdout
# log_file = "/var/log/app.log"
```

## Setup

For this example, we need to create the scripts that will be used by `pre_script` and `pos_script`.

1.  **Create a `scripts` directory:**
    ```bash
    mkdir -p scripts
    ```

2.  **Create `/scripts/setup.sh`:**
    This script will run before the main service command. It creates a log file and writes to it.

    ```bash
    cat <<'EOF' > scripts/setup.sh
    #!/bin/sh
    echo "--- Running pre-start script ---"
    touch /var/log/app.log
    chown nobody:nobody /var/log/app.log
    echo "Setup complete. Tailing log file..."
    EOF
    ```

3.  **Create `/scripts/teardown.sh`:**
    This script will run after the main service command has started.

    ```bash
    cat <<'EOF' > scripts/teardown.sh
    #!/bin/sh
    echo "--- Running post-start script ---"
    echo "Service process has started. This script is now running."
    echo "Writing a log entry..."
    echo "$(date): Log entry from post-start script" >> /var/log/app.log
    EOF
    ```

4.  **Make the scripts executable:**
    ```bash
    chmod +x scripts/*.sh
    ```

## `Dockerfile`
To make this work in Docker, you'll need to copy the scripts directory. You would add this to your `Dockerfile`:

```dockerfile
# ... (your existing Dockerfile lines)
COPY scripts/ /scripts/
RUN chmod +x /scripts/*.sh
# ...
```

### Breakdown

- **`user = "nobody"`**: The `advanced-service` will be executed as the `nobody` user instead of `root`. This is a good security practice.
- **`pre_script = "/scripts/setup.sh"`**: Before running the `tail -f` command, `go-overlay` will execute this script. This is useful for setup tasks like creating directories, setting permissions, or running database migrations.
- **`pos_script = "/scripts/teardown.sh"`**: Shortly after the `tail -f` command starts, `go-overlay` will execute this script. This is useful for follow-up or notification tasks.
- **`log_file` (commented out)**: If you were to uncomment the `log_file` line, `go-overlay` would no longer capture the stdout/stderr of the service. Instead, it would expect the service to write its own logs to the specified file. `go-overlay` would then tail this file for you.

## How to Run
After creating the scripts and updating your `Dockerfile`, you can run this example using the standard procedure. The output will show the messages from the pre- and post-start scripts executing in order.
