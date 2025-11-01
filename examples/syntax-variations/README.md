# Syntax Variations Example

This example demonstrates **all possible syntax variations** for `depends_on` and `wait_after` in go-overlay v0.2.0+.

## All Supported Variations

### 1. No Dependencies
```toml
[[services]]
name = "base-service"
command = "/bin/sleep"
```
✅ Independent service, starts immediately

---

### 2. Single Dependency - Old Format
```toml
[[services]]
name = "service"
depends_on = "base-service"
wait_after = 2
```
✅ Simple string
✅ Global wait of 2 seconds
✅ **Original format - still fully supported**

---

### 3. Single Dependency - Array with Global Wait
```toml
[[services]]
name = "service"
depends_on = ["base-service"]
wait_after = 3
```
✅ Array with one element
✅ Global wait of 3 seconds

---

### 4. Single Dependency - Array with Wait Map
```toml
[[services]]
name = "service"
depends_on = ["base-service"]
wait_after = { base-service = 4 }
```
✅ Array with one element
✅ Specific wait of 4 seconds for base-service

---

### 5. Multiple Dependencies - Global Wait
```toml
[[services]]
name = "service"
depends_on = ["service-a", "service-b"]
wait_after = 5
```
✅ Array with multiple elements
✅ Waits 5 seconds after **each** dependency

---

### 6. Multiple Dependencies - Per Dependency Wait
```toml
[[services]]
name = "service"
depends_on = ["service-a", "service-b", "service-c"]
wait_after = { service-a = 1, service-b = 2, service-c = 3 }
```
✅ Array with multiple elements
✅ Different wait for each dependency:
  - 1 second after service-a
  - 2 seconds after service-b
  - 3 seconds after service-c

---

### 7. Complex Dependencies - Custom Wait
```toml
[[services]]
name = "service"
depends_on = ["db", "cache", "queue"]
wait_after = { db = 10, cache = 2, queue = 5 }
```
✅ Fine-grained control over each dependency
✅ Useful when services have different startup times

---

### 8. Dependency Without Wait (Default = 0)
```toml
[[services]]
name = "service"
depends_on = ["base-service"]
# wait_after is optional
```
✅ `wait_after` is optional
✅ Default = 0 (no additional wait)

---

### 9. Old Format Without Wait
```toml
[[services]]
name = "service"
depends_on = "base-service"
# wait_after is optional
```
✅ Original format without wait_after
✅ Default = 0 (no additional wait)

---

## Validation Rules

### ✅ Allowed

1. `depends_on` can be:
   - String: `"service-name"`
   - Array: `["service-a", "service-b"]`

2. `wait_after` can be:
   - Integer: `5` (seconds)
   - Map: `{ service-a = 2, service-b = 5 }`
   - Omitted (default = 0)

3. `wait_after` values: **0 to 300 seconds**

### ❌ Not Allowed

1. **Non-existent dependency**
   ```toml
   depends_on = ["non-existent-service"]  # ❌ Error
   ```

2. **Circular dependency**
   ```toml
   # service-a depends on service-b
   # service-b depends on service-a
   # ❌ Error: circular dependency
   ```

3. **Wait_after with non-dependent service**
   ```toml
   depends_on = ["service-a"]
   wait_after = { service-a = 2, service-b = 5 }  # ❌ service-b is not in depends_on
   ```

4. **Wait_after out of bounds**
   ```toml
   wait_after = 500  # ❌ Maximum is 300 seconds
   wait_after = -5   # ❌ Minimum is 0 seconds
   ```

---

## Startup Order

The services in this example start in the following order:

```
1. base-service (no dependencies)
   ↓
2. service-old-style, service-array-single, service-array-single-map, service-no-wait, service-old-no-wait
   (all depend only on base-service)
   ↓
3. service-multi-global (depends on base-service + service-old-style)
   ↓
4. service-multi-perdep (depends on base-service + service-old-style + service-array-single)
   ↓
5. service-complex (depends on service-multi-global + service-multi-perdep + service-array-single-map)
```

---

## How to Test

```bash
# Validate the configuration
go-overlay  # Will load and validate the file

# If there are errors, they will be shown immediately
# If everything is OK, services will start in the correct order
```

---

## When to Use Each Format

### Use Simple String
```toml
depends_on = "service"
wait_after = 2
```
✅ **When:** Only one dependency
✅ **Advantage:** More concise and readable

### Use Array + Global Wait
```toml
depends_on = ["service-a", "service-b"]
wait_after = 5
```
✅ **When:** Multiple dependencies with the same wait time
✅ **Advantage:** Simple and clear

### Use Array + Wait Map
```toml
depends_on = ["db", "cache", "queue"]
wait_after = { db = 10, cache = 2, queue = 5 }
```
✅ **When:** Each dependency needs a different time
✅ **Advantage:** Fine-grained control and startup time optimization

---

## Performance Tips

1. **Minimize wait_after when possible**
   - Use only the time necessary for the service to be ready

2. **Use wait_after map to optimize**
   - If cache starts in 2s but DB needs 10s, configure individually

3. **Consider health checks**
   - wait_after is a fixed time, it does not guarantee that the service is ready
   - Consider adding health checks to the services themselves

---

## Compatibility

✅ **100% backward compatible**
- All old configurations still work
- No migration needed
- Mix old and new styles freely

---

## See Also

- `examples/flask-celery-stack/` - Real example with Flask + Celery
- `examples/multi-dependency-example/` - Example with complex multiple dependencies
- `docs/MULTI-DEPENDENCIES.md` - Complete documentation
