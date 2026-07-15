# w7 · m32 — Service inbound `ipAllowList`: web services + static sites

**Worker:** worker7 **Goal:** Render's service-level `ipAllowList` (`[{cidrBlock, description}]` on webServiceDetails + staticSiteDetails, POST+PATCH) exists for bex: inbound HTTP to a web service or static site is restricted to the listed CIDRs via a Traefik middleware, settable on every surface. **Status:** todo

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's semantics (empty list, deny behavior, scope)  | 30m | —          |
| t002 | CRD field + Traefik `ipAllowList` middleware on the app Ingress | 60m | t001       |
| t003 | Static-site coverage (shared static-server host routes)        | 45m | t002       |
| t004 | REST/GraphQL/MCP with the structured `{cidrBlock, description}` shape | 45m | t002       |
| t005 | Reconcile the PG/KV flat `[]string` divergence (decide + flag) | 30m | t004       |
| t006 | Dashboard: Networking section on the service page              | 40m | t004       |
| t007 | Render parity                                                  | 30m | t003, t005, t006 |
| t008 | Simplify                                                       | 30m | t007       |
| t009 | Test coverage                                                  | 45m | t007       |
| t010 | Closeout                                                       | 15m | t009       |

## Definition of done

A web service (and a static site) with an `ipAllowList` set answers only from listed CIDRs (out-of-list requests get the captured deny behavior) on platform and custom hosts alike; an empty/absent list means allow-all (per capture); the field round-trips as `[{cidrBlock, description}]` on REST/GraphQL/MCP create+PATCH+read; the dashboard Networking section edits it; private services and cron jobs reject it per the spec's placement. Verified live against the mock cluster from two source IPs (or forged `X-Forwarded-For` only if the capture says Render honors it — do not fake the test otherwise; use in-cluster vs external vantage points).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 7, 2026-07-14 — systematic field-diff of Render's pinned OpenAPI: `ipAllowList: [{cidrBlock, description}]` on `webServiceDetailsPOST/PATCH` + `staticSiteDetailsPOST/PATCH` (NOT privateServiceDetails — internal-only already). Zero hits in `lego/backend/internal/apps/` beyond the bex.yml datastore pass-through.
- **Goal linkage:** Render parity + w7's network-access-control charter — this is w7/m5 (Key Value ipAllowList) applied to the HTTP plane; the Traefik middleware mechanism is proven there and in managed Postgres.
- **Expected outcome:** tenants can IP-restrict staging/admin services; the last network-access parity gap closes; one fewer w7/m30 allowlist entry.
- **Why now:** spec-verified gap on a proven mechanism; w7 at two open milestones. Render parity task included — all-surface change. t005 exists because Render's shape here is richer than the flat `[]string` bex consciously chose for PG/KV — that fork must be decided (adopt structured everywhere vs document the split), not silently shipped.
