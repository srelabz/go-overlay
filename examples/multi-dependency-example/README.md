# Multi-Dependency Example

This example demonstrates enhanced dependency support in go-overlay, including:

1. **Multiple dependencies** - A service can depend on several other services
2. **Custom wait times** - Configure different wait times for each dependency

## Features

### 1. Multiple Dependencies

Previously, a service could only depend on a single service using a string:

```toml
depends_on = "redis"
```

Now you can specify multiple dependencies using a list:

```toml
depends_on = ["redis", "postgres"]
```

### 2. Custom Wait Times

You can configure `wait_after` in three ways:

#### a) Global integer value (old mode - still supported)
```toml
depends_on = "redis"
wait_after = 2  # Waits 2 seconds after Redis starts
```

#### b) Dependency map (new)
```toml
depends_on = ["redis", "postgres"]
wait_after = { redis = 2, postgres = 5 }
# Waits 2 seconds after Redis and 5 seconds after PostgreSQL
```

#### c) Global value for multiple dependencies
```toml
depends_on = ["web", "celery-worker", "celery-beat"]
wait_after = 5  # Waits 5 seconds after each dependency starts
```

## Example Structure

```
redis (no dependencies)
  ↓
  ├─→ web (depends on redis + postgres)
  ├─→ celery-worker (depends on redis)
  └─→ celery-beat (depends on redis)
        ↓
        └─→ monitoring (depends on web, celery-worker, celery-beat)
```

## Validations

Go-overlay now validates:

1. ✓ All specified dependencies exist
2. ✓ There are no circular dependencies
3. ✓ Wait times are within limits (0-300 seconds)
4. ✓ Keys in the `wait_after` map correspond to declared dependencies

## Usage

```bash
# Execute the example
go-overlay --config services.toml

# List services
go-overlay list

# Check status
go-overlay status

# Restart a service
go-overlay restart web
```

## Backward Compatibility

✓ The old format is still fully supported:
```toml
depends_on = "redis"
wait_after = 2
```

✓ You can mix old and new styles in the same configuration file.
