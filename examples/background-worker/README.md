# Background Worker and `required` Flag Example

This example shows how to use the `required` flag to control `go-overlay`'s behavior when a service fails.

## `services.toml`

```toml
# A critical web service that must always be running.
[[services]]
name = "web-server"
command = "python3"
args = ["-m", "http.server", "8080"]
enabled = true
required = true # If this fails, the container will shut down.

# A background worker that is allowed to fail without stopping the container.
[[services]]
name = "worker"
command = "sh"
args = ["-c", "echo 'Worker starting...'; sleep 5; echo 'Worker has an error!'; exit 1"]
enabled = true
required = false # If this fails, other services keep running.
```

### Breakdown

- **`web-server` service**:
    - This is a standard Python web server.
    - **`required = true`**: This tells `go-overlay` that this is a critical service. If it were to crash, `go-overlay` would begin a graceful shutdown of all other running services and then exit.
- **`worker` service**:
    - This is a script designed to fail. It runs for 5 seconds and then exits with a non-zero exit code (`exit 1`).
    - **`required = false`**: This is important. Because the worker is not required, its failure will **not** cause the `web-server` to shut down. The container will continue running.

## How to Run

1.  **Copy the `services.toml`:**

    ```bash
    cp examples/background-worker/services.toml .
    ```

2.  **Build and Run:**

    ```bash
    mise exec -- invoke docker.build
    mise exec -- invoke docker.run --port=8080
    ```

3.  **Observe the Behavior:**
    1.  Both `web-server` and `worker` will start.
    2.  The `worker` will print "Worker starting...".
    3.  After 5 seconds, the `worker` will print "Worker has an error!" and exit.
    4.  `go-overlay` will log that the `worker` service has failed.
    5.  **Crucially, the `web-server` will continue running.** You can verify this by navigating to `http://localhost:8080` in your browser.
    6.  If you use another terminal to run `mise exec -- invoke docker.exec go-overlay list`, you will see the `worker` in the `FAILED` state while `web-server` is `RUNNING`.

This example demonstrates how you can run non-critical or auxiliary tasks alongside your main application services.
