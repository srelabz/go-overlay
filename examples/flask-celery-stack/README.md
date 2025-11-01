# Flask + Celery Stack Example

This example demonstrates a typical Flask application setup with Celery for asynchronous task processing.

## Architecture

```
Redis
  ↓
  ├─→ Web (Gunicorn + Flask)
  ├─→ Celery Worker
  └─→ Celery Beat
```

## Services

1. **Redis** - Message broker for Celery
2. **Web** - Flask web server using Gunicorn
3. **Celery Worker** - Asynchronous task processor
4. **Celery Beat** - Periodic task scheduler

## Configuration

All services wait for Redis to start and then wait for 2 seconds before starting:

```toml
depends_on = ["redis"]
wait_after = { redis = 2 }
```

## Prerequisites

```bash
# Install Redis
sudo apt-get install redis-server

# Create appuser user
sudo useradd -m -s /bin/bash appuser

# Install Python dependencies
pip install flask gunicorn celery redis
```

## Usage

```bash
# Run the stack
go-overlay

# List services
go-overlay list

# Check status
go-overlay status

# Restart a specific service
go-overlay restart celery-worker
```

## Notes

- Redis is marked as `required = true`, so if it fails, the entire system will shut down.
- Celery workers are `required = false`, allowing the system to continue functioning without them.
- All Flask/Celery services run as `appuser` for security.
