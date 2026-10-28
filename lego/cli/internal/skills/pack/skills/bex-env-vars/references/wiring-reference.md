# Blueprint environment variable wiring

Read [environment-variables.md](environment-variables.md) first for current syntax discovery, secret handling, precedence, and mutation safety. This reference retains task-specific cross-resource wiring examples. Pair it with the **bex-blueprints** skill for service structure, previews, and validation.

## `fromDatabase` — database properties

Reference a database defined in the same Blueprint (name must match the resource’s `name`).

Available **properties** (use the subset your app needs):

- `connectionString`
- `connectionPoolString` (when managed PgBouncer is enabled)
- `host`
- `port`
- `user`
- `password`
- `database`

```yaml
databases:
  - name: mydb
    databaseName: myapp
    user: myuser

services:
  - type: web
    name: api
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: mydb
          property: connectionString
      - key: DATABASE_POOL_URL
        fromDatabase:
          name: mydb
          property: connectionPoolString
      - key: PGHOST
        fromDatabase:
          name: mydb
          property: host
      - key: PGPORT
        fromDatabase:
          name: mydb
          property: port
      - key: PGUSER
        fromDatabase:
          name: mydb
          property: user
      - key: PGPASSWORD
        fromDatabase:
          name: mydb
          property: password
      - key: PGDATABASE
        fromDatabase:
          name: mydb
          property: database
```

## `fromService` — Key Value

Use `type: keyvalue` with one of: `connectionString`, `host`, or `port`. If an application needs `host:port`, combine the separate `host` and `port` values.

```yaml
services:
  - type: keyvalue
    name: cache
    ipAllowList: []

  - type: web
    name: api
    envVars:
      - key: REDIS_URL
        fromService:
          name: cache
          type: keyvalue
          property: connectionString
      - key: REDIS_HOST
        fromService:
          name: cache
          type: keyvalue
          property: host
      - key: REDIS_PORT
        fromService:
          name: cache
          type: keyvalue
          property: port
```

## `fromService` — private service or web service

For **private services** (`pserv`) or **web** services, typical properties:

- `host` — internal hostname
- `hostport` — `host:port` combined

Alternatively, reference a **specific env var** exposed by the other service with `envVarKey` (when supported for that link type—confirm in Dashboard-generated Blueprints for your service types).

```yaml
services:
  - type: pserv
    name: internal-api
    runtime: docker
    # ...

  - type: web
    name: gateway
    envVars:
      - key: INTERNAL_API_HOST
        fromService:
          name: internal-api
          type: pserv
          property: host
      - key: INTERNAL_API_HOSTPORT
        fromService:
          name: internal-api
          type: pserv
          property: hostport
```

```yaml
  - key: UPSTREAM_TOKEN
    fromService:
      name: internal-api
      type: web
      envVarKey: SERVICE_TOKEN
```

(Adjust `type` to match the target service: `web`, `pserv`, etc.)

## `fromGroup` — environment group

Links **all** variables from the named group. Group must exist (or be created in the same Blueprint workflow, depending on your org’s pattern).

```yaml
services:
  - type: web
    name: app
    envVars:
      - fromGroup: shared-config
      - key: SERVICE_SPECIFIC
        value: only-on-app
```

## Edge cases

- **`fromService` / `fromDatabase` outside the Blueprint file** — You may reference resources that are **not** declared in `render.yaml` **only if** they **already exist** in the same bex **workspace** (same team/account context) and names match. Otherwise provision fails at apply time.

## Minimal multi-pattern example

```yaml
services:
  - type: web
    name: full-example
    runtime: node
    buildCommand: npm install && npm run build
    startCommand: npm start
    envVars:
      - fromGroup: baseline
      - key: PUBLIC_API_URL
        value: https://api.example.com
      - key: WEBHOOK_SIGNING_SECRET
        generateValue: true
      - key: THIRD_PARTY_API_KEY
        sync: false
      - key: DATABASE_URL
        fromDatabase:
          name: prod-db
          property: connectionString
      - key: REDIS_URL
        fromService:
          name: sessions-kv
          type: keyvalue
          property: connectionString
```
