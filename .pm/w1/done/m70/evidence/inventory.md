# w1/m70 evidence — measured MCP parity inventory

Captured against `github.com/render-oss/render-mcp-server` `main@89c1f01b4527` (2026-08-17T09:15:09Z), 2026-08-18.

Both sides measured the same way: each server is built and driven through the MCP stdio handshake, and its own `tools/list` response is read. Neither side is assumed.

- upstream tools: **22**
- bex tools: **213**

## Class counts

| class | count |
| --- | --- |
| `Parity1to1` | 10 |
| `Superset` | 1 |
| `Divergent` | 8 |
| `Extension` | 194 |
| **total** | **213** |

## The 10 `Parity1to1` tools — the actual contract

These are what an agent written against Render's official MCP calls unchanged. `w1/m71` must not rename or reshape any of them.

- `get_deploy`
- `get_key_value`
- `get_postgres`
- `get_service`
- `list_key_value`
- `list_log_label_values`
- `list_logs`
- `list_postgres_instances`
- `list_workspaces`
- `query_render_postgres`

## The 1 `Superset`

- `list_deploys` — upstream's args plus bex's optional `status` and created/updated/finished time filters, keeping all three bex adapters equivalent.

## The 8 `Divergent` — share an upstream name, break its contract

| tool | departure | disposition |
| --- | --- | --- |
| `create_web_service` | missing `region` | accepted — no caller-chosen region in bex |
| `create_cron_job` | missing `region` | accepted — same |
| `create_key_value` | missing `region` | accepted — same |
| `create_postgres` | missing `region`, `diskSizeGb` | **repair** — bex spells it `diskSizeGB`; a casing bug |
| `create_static_site` | missing `autoDeploy`, `buildCommand`; requires `publishPath` | **repair** — `publishPath` is genuine, the two omissions are not |
| `get_metrics` | missing 7 args; requires renamed `resource` | **repair** — `resourceId`→`resource`, `httpLatencyQuantile`→`quantile`, `resolution`→`resolutionSeconds` |
| `list_services` | missing `includePreviews` | accepted — PR previews are a recorded non-goal |
| `trigger_deploy` | missing `clearCache` | **repair** — reaches REST and GraphQL, never wired into MCP despite ADR018's w2/m30 row |

## 3 upstream-only tools

| tool | why bex does not ship it |
| --- | --- |
| `select_workspace` | w1/m55 adopted the request-scoped `workspaceId` contract |
| `get_selected_workspace` | w1/m55 — same |
| `update_environment_variables` | bex covers the capability as `update_env_vars`; a **name** divergence, and the reason `internal/secrets/mcp.go`'s "Render's official MCP has no env-var tools" comment was stale |

## Upstream is shrinking, which matters for m71

The latest release `v0.3.0` (2026-01-14) ships 24 tools; `main` ships 22. Upstream **removed** `update_web_service`, `update_static_site`, and `update_cron_job` in [#89](https://github.com/render-oss/render-mcp-server/pull/89) (2026-07-23) — "stop exposing placeholder update tools that only return dashboard links" — and added `trigger_deploy`.

So Render's own answer to "how does an agent update a service over MCP" is currently *it does not*. That does not block m71's fold (every `set_*` is an `Extension`, so no parity attaches either way), but m71/t001 should decide the target grammar knowing upstream deliberately walked away from `update_*` rather than assuming it is the obvious destination.
