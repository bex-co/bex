---
name: bex-monitor
description: Monitor bex services in real-time. Check health, performance metrics, logs, and resource usage. Use when users want to check service status, view metrics, monitor performance, or verify deployments are healthy.
license: MIT
compatibility: Requires bex MCP tools or CLI
metadata:
  author: bex
  version: "1.0.0"
  category: monitoring
---

# Monitor bex Services

Real-time monitoring of bex services including health checks, performance metrics, and logs.

## When to Use This Skill

Activate this skill when users want to:
- Check if services are healthy
- View performance metrics
- Monitor logs
- Verify a deployment is working
- Investigate slow performance
- Check database health

## bex Access

Before reading service state, logs, metrics, or database health, follow `references/bex-access.md` to establish access and verify the intended workspace. Prefer MCP when available; the CLI can provide service status and logs, while metrics and database queries require MCP.

Before interpreting deploy or health-check state, read `references/deployments.md`.

---

## Quick Health Check

Run these 5 checks to assess service health:

```
# 1. Check service status
list_services()

# 2. Check latest deploy
list_deploys(serviceId: "<service-id>", limit: 1)

# 3. Check for errors
list_logs(resource: ["<service-id>"], level: ["error"], limit: 20)

# 4. Check resource usage
get_metrics(resourceId: "<service-id>", metricTypes: ["cpu_usage", "memory_usage"])

# 5. Check latency
get_metrics(resourceId: "<service-id>", metricTypes: ["http_latency"], httpLatencyQuantile: 0.95)
```

---

## Service Health

### Check Status

```
list_services()
```

```
get_service(serviceId: "<id>")
```

### Check Deployments

```
list_deploys(serviceId: "<service-id>", limit: 5)
```

Follow `references/deployments.md` to interpret asynchronous and terminal states, identify the specific deploy, and verify the workload after it becomes live.

### Check Errors

```
list_logs(resource: ["<service-id>"], level: ["error"], limit: 50)
```

```
list_logs(resource: ["<service-id>"], statusCode: ["500", "502", "503"], limit: 50)
```

---

## Performance Metrics

### CPU & Memory

```
get_metrics(
  resourceId: "<service-id>",
  metricTypes: ["cpu_usage", "memory_usage", "cpu_limit", "memory_limit"]
)
```

| Metric | Healthy | Warning | Critical |
|--------|---------|---------|----------|
| CPU | <70% | 70-85% | >85% |
| Memory | <80% | 80-90% | >90% |

### HTTP Latency

```
get_metrics(
  resourceId: "<service-id>",
  metricTypes: ["http_latency"],
  httpLatencyQuantile: 0.95
)
```

| p95 Latency | Status |
|-------------|--------|
| <200ms | Excellent |
| 200-500ms | Good |
| 500ms-1s | Concerning |
| >1s | Problem |

### Request Count

```
get_metrics(
  resourceId: "<service-id>",
  metricTypes: ["http_request_count"]
)
```

### Filter by Endpoint

```
get_metrics(
  resourceId: "<service-id>",
  metricTypes: ["http_latency"],
  httpPath: "/api/users"
)
```

Detailed metrics guide: [references/metrics-guide.md](references/metrics-guide.md)

---

## Database Monitoring

### PostgreSQL Status

```
list_postgres_instances()
get_postgres(postgresId: "<postgres-id>")
```

### Connection Count

```
get_metrics(resourceId: "<postgres-id>", metricTypes: ["active_connections"])
```

### Query Database

```
query_render_postgres(
  postgresId: "<postgres-id>",
  sql: "SELECT state, count(*) FROM pg_stat_activity GROUP BY state"
)
```

### Find Slow Queries

```
query_render_postgres(
  postgresId: "<postgres-id>",
  sql: "SELECT query, mean_exec_time FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10"
)
```

### Key-Value Store

```
list_key_value()
get_key_value(keyValueId: "<kv-id>")
```

---

## Log Monitoring

### Recent Logs

```
list_logs(resource: ["<service-id>"], limit: 100)
```

### Error Logs

```
list_logs(resource: ["<service-id>"], level: ["error"], limit: 50)
```

### Search Logs

```
list_logs(resource: ["<service-id>"], text: ["timeout", "error"], limit: 50)
```

### Filter by Time

```
list_logs(
  resource: ["<service-id>"],
  startTime: "2024-01-15T10:00:00Z",
  endTime: "2024-01-15T11:00:00Z"
)
```

### Stream Logs (CLI)

```bash
bex logs -r <service-id> --tail -o text
```

---

## Quick Reference

### MCP Tools

```
# Services
list_services()
get_service(serviceId: "<id>")
list_deploys(serviceId: "<id>", limit: 5)

# Logs
list_logs(resource: ["<id>"], level: ["error"], limit: 100)
list_logs(resource: ["<id>"], text: ["search"], limit: 50)

# Metrics
get_metrics(resourceId: "<id>", metricTypes: ["cpu_usage", "memory_usage"])
get_metrics(resourceId: "<id>", metricTypes: ["http_latency"], httpLatencyQuantile: 0.95)
get_metrics(resourceId: "<id>", metricTypes: ["http_request_count"])

# Database
list_postgres_instances()
get_postgres(postgresId: "<id>")
query_render_postgres(postgresId: "<id>", sql: "SELECT ...")
get_metrics(resourceId: "<postgres-id>", metricTypes: ["active_connections"])

# Key-Value
list_key_value()
get_key_value(keyValueId: "<id>")
```

### CLI Commands (Fallback)

Use these if MCP tools are unavailable:

```bash
# Service status
bex services -o json
bex services instances <service-id>

# Deployments
bex deploys list <service-id> -o json

# Logs
bex logs -r <service-id> --tail -o text          # Stream logs
bex logs -r <service-id> --level error -o json   # Error logs
bex logs -r <service-id> --type deploy -o json   # Build logs

# Database
bex psql <database-id>                           # Connect to PostgreSQL

# SSH for live debugging
bex ssh <service-id>
```

### Healthy Service Indicators

| Indicator | Healthy | Warning | Critical |
|-----------|---------|---------|----------|
| Error Rate | <0.1% | 0.1-1% | >1% |
| p95 Latency | <500ms | 500ms-2s | >2s |
| CPU Usage | <70% | 70-90% | >90% |
| Memory Usage | <80% | 80-95% | >95% |

---

## References

- **bex access:** [references/bex-access.md](references/bex-access.md)
- **Compute plans:** [references/compute-plans.md](references/compute-plans.md)
- **Deployments:** [references/deployments.md](references/deployments.md)
- **Metrics guide:** [references/metrics-guide.md](references/metrics-guide.md)

## Related Skills

- **bex-deploy** — Deploy new applications to bex
- **bex-debug** — Diagnose and fix deployment failures
- **bex-mcp** — MCP server setup and capability discovery

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
