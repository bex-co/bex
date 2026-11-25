---
name: bex-debug
description: Debug failed bex deployments by analyzing logs, metrics, and database state. Identifies errors (missing env vars, port binding, OOM, etc.) and suggests fixes. Use when deployments fail, services won't start, or users mention errors, logs, or debugging.
license: MIT
compatibility: Requires bex MCP tools or CLI
metadata:
  author: bex
  version: "1.1.0"
  category: debugging
---

# Debug bex Deployments

Analyze deployment failures using logs, metrics, and database queries. Identify root causes and apply fixes.

## When to Use This Skill

Activate this skill when:
- Deployment fails on bex
- Service won't start or keeps crashing
- User mentions errors, logs, or debugging
- Health checks are timing out
- Application errors in production
- Performance issues (slow responses)
- Database connection problems

## bex Access

Before reading logs, metrics, or database state, follow `references/bex-access.md` to establish access and verify the intended workspace. Prefer MCP when available; the CLI can provide logs and deploy status, while metrics and structured database queries require MCP.

---

## Debugging Workflow

When the failure involves configuration or secrets, read `references/environment-variables.md` before inspecting or changing service variables.
When the failure involves deploy state, rollout, health checks, restart, rollback, or shutdown, read `references/deployments.md` before diagnosing it.

### Step 1: Identify Failed Service

```
list_services()
```

If MCP isn't configured, ask whether to set it up (preferred) or continue with CLI. Then proceed.

Look for services with failed status. Get details:

```
get_service(serviceId: "<id>")
```

### Step 2: Retrieve Logs

**Build/Deploy Logs (most failures):**
```
list_logs(resource: ["<service-id>"], type: ["build"], limit: 200)
```

**Runtime Error Logs:**
```
list_logs(resource: ["<service-id>"], level: ["error"], limit: 100)
```

**Search for Specific Errors:**
```
list_logs(resource: ["<service-id>"], text: ["KeyError", "ECONNREFUSED"], limit: 50)
```

**HTTP Error Logs:**
```
list_logs(resource: ["<service-id>"], statusCode: ["500", "502", "503"], limit: 50)
```

### Step 3: Analyze Error Patterns

Match log errors against known patterns:

| Error | Log Pattern | Common Fix |
|-------|-------------|------------|
| **MISSING_ENV_VAR** | `KeyError`, `not defined` | Add to render.yaml or `update_environment_variables` |
| **PORT_BINDING** | `EADDRINUSE` | Use `0.0.0.0:$PORT` |
| **MISSING_DEPENDENCY** | `Cannot find module` | Add to package.json/requirements.txt |
| **DATABASE_CONNECTION** | `ECONNREFUSED :5432` | Check DATABASE_URL, DB status |
| **HEALTH_CHECK** | `Health check timeout` | Add /health endpoint, check port binding |
| **OUT_OF_MEMORY** | `heap out of memory`, exit 137 | Optimize memory or upgrade plan |
| **BUILD_FAILURE** | `Command failed` | Fix build command or dependencies |

Full error catalog: [references/error-patterns.md](references/error-patterns.md)

**If errors repeat across deploys:** Switch from incremental fixes to a broader sweep. Scan the codebase/config for all likely causes in that error class (related env vars, build config, dependencies, or type errors) and address them together before the next redeploy.

### Step 4: Check Metrics (Performance Issues)

For crashes, slow responses, or resource issues:

```
get_metrics(
  resourceId: "<service-id>",
  metricTypes: ["cpu_usage", "memory_usage", "memory_limit"]
)
```

```
get_metrics(
  resourceId: "<service-id>",
  metricTypes: ["http_latency"],
  httpLatencyQuantile: 0.95
)
```

Detailed metrics guide: [references/metrics-debugging.md](references/metrics-debugging.md)

### Step 5: Debug Database Issues

For database-related errors:

```
# Check database status
list_postgres_instances()

# Check connections
get_metrics(resourceId: "<postgres-id>", metricTypes: ["active_connections"])

# Query directly
query_render_postgres(
  postgresId: "<postgres-id>",
  sql: "SELECT state, count(*) FROM pg_stat_activity GROUP BY state"
)
```

Detailed database guide: [references/database-debugging.md](references/database-debugging.md)

### Step 6: Apply Fix

**For environment variables:**
```
update_environment_variables(
  serviceId: "<service-id>",
  envVars: [{"key": "MISSING_VAR", "value": "value"}]
)
```

**For code changes:**
1. Edit the source file
2. Commit and push
3. Deploy triggers automatically (if auto-deploy enabled)

### Step 7: Verify Fix

```
# Check deploy status
list_deploys(serviceId: "<service-id>", limit: 1)

# Check for new errors
list_logs(resource: ["<service-id>"], level: ["error"], limit: 20)

# Check metrics
get_metrics(resourceId: "<service-id>", metricTypes: ["http_request_count"])
```

---

## Quick Workflows

Pre-built debugging sequences for common scenarios:

| Scenario | Workflow |
|----------|----------|
| Deploy failed | `list_deploys` → `list_logs(type: build)` → fix → redeploy |
| App crashing | `list_logs(level: error)` → `get_metrics(memory)` → fix |
| App slow | `get_metrics(http_latency)` → `get_metrics(cpu)` → `query_postgres` |
| DB connection | `list_postgres` → `get_metrics(connections)` → `query_postgres` |
| Post-deploy check | `list_deploys` → `list_logs(error)` → `get_metrics` |

Detailed workflows: [references/quick-workflows.md](references/quick-workflows.md)

---

## Quick Reference

### MCP Tools

```
# Service Discovery
list_services()
get_service(serviceId: "<id>")
list_postgres_instances()

# Logs
list_logs(resource: ["<id>"], level: ["error"], limit: 100)
list_logs(resource: ["<id>"], type: ["build"], limit: 200)
list_logs(resource: ["<id>"], text: ["search"], limit: 50)

# Metrics
get_metrics(resourceId: "<id>", metricTypes: ["cpu_usage", "memory_usage"])
get_metrics(resourceId: "<id>", metricTypes: ["http_latency"], httpLatencyQuantile: 0.95)

# Database
query_render_postgres(postgresId: "<id>", sql: "SELECT ...")

# Deployments
list_deploys(serviceId: "<id>", limit: 5)

# Environment Variables
update_environment_variables(serviceId: "<id>", envVars: [{key, value}])
```

### CLI Commands (Fallback)

```bash
bex services -o json
bex logs -r <service-id> --level error -o json
bex logs -r <service-id> --tail -o text
bex deploys create <service-id> --wait
```

---

## References

- **bex access:** [references/bex-access.md](references/bex-access.md)
- **Environment variables:** [references/environment-variables.md](references/environment-variables.md)
- **Compute plans:** [references/compute-plans.md](references/compute-plans.md)
- **Deployments:** [references/deployments.md](references/deployments.md)
- **Error patterns:** [references/error-patterns.md](references/error-patterns.md)
- **Metrics debugging:** [references/metrics-debugging.md](references/metrics-debugging.md)
- **Database debugging:** [references/database-debugging.md](references/database-debugging.md)
- **Quick workflows:** [references/quick-workflows.md](references/quick-workflows.md)
- **Log analysis:** [references/log-analysis.md](references/log-analysis.md)
- **Troubleshooting:** [references/troubleshooting.md](references/troubleshooting.md)

## Related Skills

- **bex-deploy** — Deploy new applications to bex
- **bex-monitor** — Ongoing service health monitoring
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
