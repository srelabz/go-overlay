# Web Stack with Dependencies Example

This example demonstrates how to use the `depends_on` and `wait_after` options to control the startup order of services.

## `services.toml`

```toml
# Simulate a database that takes time to start up
[[services]]
name = "database"
command = "sh"
args = ["-c", "echo 'Booting database...'; sleep 5; echo 'Database is ready!'"]
enabled = true
required = true

# A web application that depends on the database
[[services]]
name = "web-app"
command = "sh"
args = ["-c", "echo 'Web app connecting to database...'; echo 'Web app started successfully!'"]
enabled = true
required = true
depends_on = "database" # This service will wait for 'database' to start
wait_after = 2          # Wait 2 extra seconds after 'database' starts
```

### Breakdown

- **`database` service**:
    - This is a simple shell script that simulates a database starting up by printing a message and then sleeping for 5 seconds.
- **`web-app` service**:
    - **`depends_on = "database"`**: This is the key feature. `go-overlay` will not start the `web-app` service until after the `database` service has started (i.e., its main process has been launched).
    - **`wait_after = 2`**: This adds an additional, fixed delay. After `go-overlay` starts the `database` service, it will wait an extra 2 seconds before it proceeds to start the `web-app` service. This is useful for services that need a moment to initialize even after their process has started.

## How to Run

1.  **Copy the `services.toml`:**

    ```bash
    cp examples/web-stack-deps/services.toml .
    ```

2.  **Build and Run:**

    ```bash
    mise exec -- invoke docker.build
    mise exec -- invoke docker.run
    ```

3.  **Observe the Logs:**
    You will see the following sequence in your terminal logs:
    1.  `go-overlay` starts the `database` service.
    2.  The `database` service prints "Booting database...".
    3.  `go-overlay` waits for the `database` process to run, then waits an additional 2 seconds (due to `wait_after`).
    4.  After the wait, `go-overlay` starts the `web-app` service.
    5.  The `web-app` service prints "Web app connecting to database...".
    6.  After a total of 5 seconds from its start, the `database` service prints "Database is ready!".

This demonstrates how `go-overlay` can manage complex startup sequences, ensuring that services start in the correct order.
