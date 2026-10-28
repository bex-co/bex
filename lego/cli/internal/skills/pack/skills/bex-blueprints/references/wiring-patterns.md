# Cross-Service Wiring Patterns

Read [environment-variables.md](environment-variables.md) first for current syntax discovery, secret handling, precedence, and mutation safety. When wiring an internal service address or discovery hostname, also read [private-networking.md](private-networking.md). This reference retains cross-resource wiring examples for `render.yaml`.

---

## fromDatabase

Reference a Managed Postgres database property:

```yaml
envVars:
  - key: DATABASE_URL
    fromDatabase:
      name: my-postgres
      property: connectionString
```

Available properties:

| Property | Value |
|----------|-------|
| `connectionString` | Full `postgres://` URL (most common) |
| `connectionPoolString` | Managed PgBouncer URL when connection pooling is enabled |
| `host` | Database hostname |
| `port` | Database port |
| `user` | Database user |
| `password` | Database password |
| `database` | Database name |

The `name` must match a database defined in the `databases` section of the same Blueprint, or an existing database in the workspace.

---

## fromService (Key Value)

Reference a Key Value (Redis-compatible) instance:

```yaml
envVars:
  - key: REDIS_URL
    fromService:
      name: my-redis
      type: keyvalue
      property: connectionString
```

Available properties:

| Property | Value |
|----------|-------|
| `connectionString` | Full `redis://` URL |
| `host` | Internal hostname |
| `port` | Port number |

---

## fromService (Private Service or Web Service)

Reference another service's internal address or environment variable:

```yaml
envVars:
  # Get internal hostname
  - key: AUTH_SERVICE_HOST
    fromService:
      name: auth-service
      type: pserv
      property: host

  # Get host:port
  - key: AUTH_SERVICE_URL
    fromService:
      name: auth-service
      type: pserv
      property: hostport

  # Copy an env var from another service
  - key: SHARED_SECRET
    fromService:
      name: auth-service
      type: pserv
      envVarKey: API_SECRET
```

Available properties for `pserv` and `web`:

| Property | Value |
|----------|-------|
| `host` | Internal hostname |
| `port` | Listening port |
| `hostport` | `host:port` combined |

Use `envVarKey` to copy a specific environment variable value from the other service.

---

## fromGroup

Link an entire environment group:

```yaml
envVars:
  - fromGroup: shared-secrets
```

This adds all variables from the named group to the service. The group must exist in the workspace or be defined in `envVarGroups`.

---

## Combined Example

A web service wired to Postgres, Key Value, and a private service:

```yaml
services:
  - type: web
    name: api
    runtime: node
    plan: starter
    buildCommand: npm ci
    startCommand: npm start
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: db
          property: connectionString
      - key: REDIS_URL
        fromService:
          name: cache
          type: keyvalue
          property: connectionString
      - key: AUTH_HOST
        fromService:
          name: auth
          type: pserv
          property: hostport
      - key: JWT_SECRET
        generateValue: true
      - key: STRIPE_KEY
        sync: false
      - fromGroup: shared-config

  - type: pserv
    name: auth
    runtime: node
    plan: starter
    buildCommand: npm ci
    startCommand: npm start

  - type: keyvalue
    name: cache
    plan: starter
    maxmemoryPolicy: allkeys-lru
    ipAllowList: [] # Internal access only

databases:
  - name: db
    plan: basic-256mb

envVarGroups:
  - name: shared-config
    envVars:
      - key: LOG_LEVEL
        value: info
```

---

## Edge Cases

- **fromService/fromDatabase can reference services outside the Blueprint** but the referenced service must already exist in the workspace.
