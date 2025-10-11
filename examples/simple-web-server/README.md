# Simple Web Server Example

This example demonstrates the most basic usage of `go-overlay`: running a single, simple service.

## `services.toml`

```toml
[[services]]
name = "web-server"
command = "python3"
args = ["-m", "http.server", "8080"]
enabled = true
required = true
```

### Breakdown

- **`name`**: "web-server" - A descriptive name for our service. This is what you'll see in the logs and use for CLI commands like `restart`.
- **`command`**: "python3" - The executable to run.
- **`args`**: `["-m", "http.server", "8080"]` - A list of arguments to pass to the command. This will start a simple web server on port 8080.
- **`enabled`**: `true` - The service will be started by `go-overlay`.
- **`required`**: `true` - If this service fails, `go-overlay` will initiate a graceful shutdown of all services.

## How to Run

1.  **Copy the files:**
    Copy this `services.toml` file to the root of your `go-overlay` project.

    ```bash
    cp examples/simple-web-server/services.toml .
    ```

2.  **Build the Docker image:**
    Use the `invoke` task to build the image. This will use the `services.toml` you just copied.

    ```bash
    mise exec -- invoke docker.build
    ```

3.  **Run the container:**
    Use the `invoke` task to run the container in the foreground.

    ```bash
    mise exec -- invoke docker.run --port=8080
    ```

4.  **Verify:**
    Open your browser and navigate to `http://localhost:8080`. You should see a directory listing served by Python's HTTP server. The logs from the service will be streamed to your terminal, prefixed with `[web-server]`.
