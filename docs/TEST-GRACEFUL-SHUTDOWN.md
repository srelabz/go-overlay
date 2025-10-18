# Graceful Shutdown Testing - Go Overlay

This directory contains everything needed to test the Go Overlay graceful shutdown system.

## Test Services

The test container includes:

1. **nginx** - Web server on port 80
   - Pre-script: `/scripts/nginx-pre.sh`
   - Post-script: `/scripts/nginx-post.sh`

2. **test-service** - Test service that depends on nginx
   - Dependency: `nginx`
   - Waits 3 seconds after nginx starts

3. **monitor** - Monitoring service
   - Dependency: `test-service` 
   - Waits 2 seconds after test-service starts

4. **logger** - Independent logging service
   - No dependencies

## How to Test

### Interactive Test
```bash
./test-container.sh
```
- Starts the container interactively
- Press `Ctrl+C` to test graceful shutdown
- Nginx will be available at `http://localhost:8080`

### Automated Test
```bash
./test-graceful-shutdown.sh
```
- Runs complete test automatically
- Verifies if nginx is responding
- Sends SIGTERM to test graceful shutdown
- Shows process logs

### Manual Test

1. **Build image:**
   ```bash
   docker build -t go-overlay-test .
   ```

2. **Run container:**
   ```bash
   docker run --rm -p 8080:80 --name tm-test go-overlay-test
   ```

3. **Test nginx (in another terminal):**
   ```bash
   curl http://localhost:8080
   curl http://localhost:8080/health
   ```

4. **Test graceful shutdown:**
   ```bash
   docker kill --signal=SIGTERM tm-test
   ```

## What to Observe

During graceful shutdown, you should see:

1. **Signal reception:** Message indicating SIGTERM was received
2. **Stop order:** Services being stopped in the correct order
3. **Timeouts:** Services that don't respond being force-terminated after 10s
4. **Cleanup:** PTYs being closed and resources freed
5. **Finalization:** "Graceful shutdown completed" message

## File Structure

- `Dockerfile` - Test container configuration
- `services.toml` - Services configuration
- `test-container.sh` - Interactive test script
- `test-graceful-shutdown.sh` - Automated test script
- `go-overlay` - Compiled orchestrator binary

## Expected Logs

```
Go Overlay - Version: v0.1.0
[INFO] Loading services from /services.toml
[INFO] | === PRE-SCRIPT START --- [SERVICE: nginx] === |
Pre-script for nginx executed
Nginx pre-script completed
[INFO] | === PRE-SCRIPT END --- [SERVICE: nginx] === |
[INFO] Starting service: nginx
[INFO] Service nginx started successfully (PID: 123)
[nginx] nginx: [alert] low worker processes
...
```

During shutdown:
```
[INFO] Received signal: terminated
[INFO] Initiating graceful shutdown...
[INFO] Starting graceful shutdown process...
[INFO] Gracefully stopping service: nginx
[INFO] Service nginx stopped gracefully
[INFO] All services stopped gracefully
[INFO] Graceful shutdown completed
```
