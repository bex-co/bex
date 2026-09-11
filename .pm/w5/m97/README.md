# w5 · m97 — Product adoption v2: creation-surface attribution

**Worker:** worker5 **Goal:** every recorded resource creation carries a closed `surface` label — dashboard, CLI, MCP, direct API, or Blueprint — so the Product adoption board can show the agent-versus-human creation split ADR008's thesis makes the headline metric, with `unknown` as an honest bucket rather than an inference. **Status:** todo — **gate satisfied 2026-09-10:** the product-adoption v1 work (migrations `0114`/`0115`, recorders, `docs/runbooks/product-analytics.md`) shipped as `a99e13750` ("feat(observability): add inventory-first product adoption analytics") and was pinned for deploy by `ba00f3c98`. It was never board-tracked (brainstorm proposal 1 was not materialized); t001 must still confirm the migrations are live in the target environment before altering the table.

## Tasks (in order)

| id   | title                                                                                               | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Closed `surface` column with down migration; carried from request context into the success recorder | 45m | —          |
| t002 | Derive CLI, dashboard, MCP, API, and Blueprint from existing request facts; unit-test each          | 45m | t001       |
| t003 | View column, "Creations by surface" and "Agent-driven share" panels, SQL tests, runbook definitions | 45m | t002       |
| t004 | Simplify                                                                                            | 20m | t003       |
| t005 | Test coverage                                                                                       | 40m | t003       |
| t006 | Closeout                                                                                            | 10m | t005       |

## Definition of done

- `product_activity_events.surface` exists with a `CHECK` over the closed set `{dashboard, cli, mcp, api, blueprint, unknown}`, default `unknown`; legacy-backfill rows stay `unknown`; down migration verified in the full-chain migration test.
- On dev-5, one creation each via the dashboard, a tree-built `bex` CLI, MCP `create_web_service`, a direct REST call with an API key, and a Blueprint sync land with the expected surface; a request with no recognizable facts lands `unknown`.
- The Product adoption board's collapsed Adoption detail row shows "Creations by surface" (mutually exclusive, summable) and an "Agent-driven share" stat defined as the MCP share of successful creations in range; the reader's column allowlist includes `surface`; `scripts/test_product_analytics.py` executes both new queries.
- `docs/runbooks/product-analytics.md` defines `surface` and its derivation rules, and no longer lists "exact dashboard/CLI/MCP attribution" under Known gaps.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 5, from `docs/runbooks/product-analytics.md` § Known gaps ("exact dashboard/CLI/MCP attribution … not collected by this first version").
- **Facts verified 2026-09-10:** `core.ProductActivity` (`lego/backend/internal/core/product_activity.go`) carries `ActorType ∈ {human, machine, system, unknown}` derived from `IdentityFrom(ctx)` (`Method == "session"` / `Human` → human; `"oauth2"` → machine) but no surface; `product_activity_events` has no surface column (`store/migrations/0114_product_analytics.up.sql`). The API layer already classifies every request into the closed set `rest | graphql | mcp | auth | internal` for metrics (`api/httpmetrics.go:76-83`, middleware 203-233) but does not put it on the context. The upstream CLI sends `User-Agent: render-cli/<version> …` (`render-oss/cli` `pkg/cfg/cfg.go:75`). Recorder call sites: `apps/service.go:2145` (App create tail, also used by Blueprint), `apps/domains.go:959`, `core/audit.go:538,653` (Postgres / Key Value).
- **Goal linkage:** ADR008 — "AI agents as first-class users"; the split between agent-driven and human-driven creations is the direct measurement of that thesis. ADR088 product-analytics visibility.
- **Expected outcome:** operators read what share of new services, datastores, and domains are created by agents through MCP, by the CLI, by the dashboard, or by Blueprints — per day and by audience — without inferring from actor type.
- **Why now:** it must follow v1 (schema, reader role, board) and should land before the 90-day retention window fills with rows that can never be attributed; adding the column early keeps the `unknown` wedge to the legacy backfill only.
- **Render parity omitted:** ops-only analytics; no tenant REST/GraphQL/MCP/dashboard contract changes. The recorder remains best-effort and bounded (never turns a completed create into an error).
