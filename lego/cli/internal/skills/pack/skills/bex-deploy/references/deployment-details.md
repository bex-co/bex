# Deployment Details

Use this reference for service discovery, configuration patterns, quick commands, and common issues.

## Service Discovery

**List all services:**
```
list_services()
```
Returns all services with IDs, names, types, and status.

**Get specific service details:**
```
get_service(serviceId: "<id>")
```
Returns full configuration including environment variables and build/start commands.

**List PostgreSQL databases:**
```
list_postgres_instances()
```

**List Key-Value stores:**
```
list_key_value()
```

## Configuration Details

### Environment Variables

Follow [environment-variables.md](environment-variables.md) for the current configuration and mutation workflow. Deployment-specific configuration notes continue in [configuration-guide.md](configuration-guide.md).

### Port Binding

**CRITICAL:** Web services must bind to `0.0.0.0:$PORT` (NOT `localhost`). bex sets the `PORT` environment variable.

**Node.js Example:**
```javascript
const PORT = process.env.PORT || 3000;
app.listen(PORT, '0.0.0.0', () => {
  console.log(`Server running on port ${PORT}`);
});
```

**Python Example:**
```python
import os

port = int(os.environ.get('PORT', 5000))
app.run(host='0.0.0.0', port=port)
```

**Go Example:**
```go
port := os.Getenv("PORT")
if port == "" {
    port = "3000"
}
http.ListenAndServe(":"+port, handler)
```

### Compute plans

Select a compute plan appropriate to the resource and workload. Use [compute-plans.md](compute-plans.md) for current Plan IDs, defaults, availability, specifications, and pricing guidance.

### Build Commands

**Use non-interactive flags to prevent build hangs:**
- npm: `npm ci`
- yarn: `yarn install --frozen-lockfile`
- pnpm: `pnpm install --frozen-lockfile`
- bun: `bun install --frozen-lockfile`
- pip: `pip install -r requirements.txt`
- uv: `uv sync`
- apt: `apt-get install -y <package>`
- bundler: `bundle install --jobs=4 --retry=3`

### Database Connections

When services connect to databases in the same bex account, use `fromDatabase` references for internal URLs.

### Health Checks

Follow [deployments.md](deployments.md) for current health-check behavior and endpoint guidance.

## Quick Reference

### MCP Tools (Preferred)

Follow [bex-access.md](bex-access.md) and inspect the tools currently available instead of relying on a fixed tool catalog.

### CLI Commands
```bash
# Validate Blueprint
bex blueprints validate

# List services
bex services -o json

# View deployment logs
bex logs -r <service-id> -o json

# Create deployment
bex deploys create <service-id> --wait
```

### Templates by Framework
- Node.js Express: [../assets/node-express.yaml](../assets/node-express.yaml)
- Next.js + Postgres: [../assets/nextjs-postgres.yaml](../assets/nextjs-postgres.yaml)
- Django + Worker: [../assets/python-django.yaml](../assets/python-django.yaml)
- Static Site: [../assets/static-site.yaml](../assets/static-site.yaml)
- Go API: [../assets/go-api.yaml](../assets/go-api.yaml)
- Docker: [../assets/docker.yaml](../assets/docker.yaml)

### Documentation
- Current Blueprint authoring and validation workflow: [blueprints.md](blueprints.md)
- Service types explained: [service-types.md](service-types.md)
- Runtime options: [runtimes.md](runtimes.md)
- Configuration guide: [configuration-guide.md](configuration-guide.md)

## Common Issues

**Issue:** Deployment fails with port binding error

**Solution:** Ensure app binds to `0.0.0.0:$PORT` (see Port Binding section above)

---

**Issue:** Build hangs or times out

**Solution:** Use non-interactive build commands (see Build Commands section above)

---

**Issue:** Missing environment variables in Dashboard

**Solution:** Follow [environment-variables.md](environment-variables.md), identify the intended source of truth, and add the missing keys without exposing secret values.

---

**Issue:** Database connection fails

**Solution:** Use `fromDatabase` references for internal connection strings.

---

**Issue:** Static site shows 404 for routes

**Solution:** Add rewrite rules to render.yaml for SPA routing:
```yaml
routes:
  - type: rewrite
    source: /*
    destination: /index.html
```

For more detailed troubleshooting, see the debug skill or [configuration-guide.md](configuration-guide.md).
