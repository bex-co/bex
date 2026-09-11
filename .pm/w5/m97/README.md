# w5 · m97 — Product adoption v2: creation-surface attribution

**Worker:** worker5 **Goal:** every recorded resource creation carries a closed `surface` label — dashboard, CLI, MCP, direct API, or Blueprint — so the Product adoption board can show the agent-versus-human creation split ADR008's thesis makes the headline metric, with `unknown` as an honest bucket rather than an inference. **Status:** todo (t001-t005 done; t006 closeout pending live verification)

## Tasks (in order)

| id   | title                                                                                               | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ---------- |
| [t001](done/t001.md) — **DONE** | Closed `surface` column with down migration; carried from request context into the success recorder | 45m | —          |
| [t002](done/t002.md) — **DONE** | Derive CLI, dashboard, MCP, API, and Blueprint from existing request facts; unit-test each          | 45m | t001       |
| [t003](done/t003.md) — **DONE** | View column, "Creations by surface" and "Agent-driven share" panels, SQL tests, runbook definitions | 45m | t002       |
| [t004](done/t004.md) — **DONE** | Simplify                                                                                            | 20m | t003       |
| [t005](done/t005.md) — **DONE** | Test coverage                                                                                       | 40m | t003       |
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

## Evidence (2026-09-11)

**Two axes, not one.** `actor_type` (v1) records whether a human or a machine credential authenticated; it cannot distinguish a dashboard click from a CLI invocation, and an agent holding a human's OAuth grant still reads as human. `surface` is the axis ADR008's agent-native thesis is actually measured on. A test pins that the two do not collapse into each other.

**Derivation is resolved late, on purpose.** The edge records only what a request carries on arrival (transport + User-Agent); identity is authenticated further in, so the surface is computed at record time when both halves exist. The carrier is its own **unconditional** middleware rather than the metrics one, which returns the handler untouched when metrics are disabled — correct for telemetry, quietly wrong for attribution.

**Blueprint attribution sits at the one shared choke point.** `deployParsedStack` is where initial create, manual sync, and the webhook auto-sync worker all converge, and it keys off the request's `BlueprintID`, so a direct `render.yaml` deploy (MCP, `scripts/app-apply.sh`) correctly keeps the surface it arrived on. Verified by tracing all three paths, not inferred.

**Review found three things worth fixing, all fixed.**

1. **The agent share was silently deflated.** Every row predating this migration defaults to `unknown`, and the panel divided by all of them — producing a confident, wrong, low number on exactly the board whose stated discipline is "gaps are labelled, never synthetic zeros". Fixed with a `surface_started_at` watermark, following the same pattern `0114`/`0115` already use for their own collections: a range reaching before attribution began now reads **null** rather than being scored. `test_agent_share_ignores_creations_predating_surface_collection` pins it by inserting legacy rows and asserting the score does not move.
2. **A comment claimed a guarantee the code did not provide** — the same failure class as m94's. `httpmetrics.go` asserted that reusing `publicSurface` meant "the two axes cannot drift", while the derivation switched on its own copies of the same literals. Renaming either would have turned every dashboard creation into `unknown` with a green build. The transport vocabulary now lives in one place (`core.Transport*`) which the API layer aliases, and `RequestOriginMiddleware` — which had **zero** test coverage — now has one driving real paths through the real middleware.
3. **The append-only guard I first wrote could not catch the defect it existed for.** It derived the "previous" view generation by shrinking the current file, so a mid-list insertion was inherited by both sides and passed — precisely the mistake m94's first attempt made. Replaced with a pinned released-column prefix, then mutation-proven: inserting `surface` mid-list now fails `test_view_columns_are_append_only`. Also split the migration's inline `CHECK` into `NOT VALID` + `VALIDATE`, so the validating scan does not hold `ACCESS EXCLUSIVE`.

**One gap documented rather than papered over.** A Blueprint apply that creates a managed Postgres or Key Value records **no** creation event at all — those two paths never reach the recorder. Pre-existing since v1, but m97 makes it readable as a false conclusion ("nobody provisions Postgres from a manifest"), so both the panel description and the runbook now say the `blueprint` slice is incomplete for datastores, and the fix is filed as `w5/056` rather than widened into this milestone: adding those events changes existing creation counts and deserves its own before/after note.

**Also corrected:** the runbook's deploy order still named only `0114/0115` and never told the operator to re-run the bootstrap when the projection changes — following it as written would have deployed both panels against a view with no `surface` column. Collapsed-row panels (more than half this board) were never overlap-checked by the geometry test; they are now.

**Gates.** `make lint` 0 issues across all four modules; `cd lego/backend && go test ./...` **63 packages green, exit 0** against a disposable PostgreSQL 17; `python3 -m unittest scripts.test_product_analytics` 31 tests green; `scripts/gitops-validate.sh` exit 0; prettier clean.

**Honest notes.** `TestCompleterDoubleFinalizeObservesOnce` (agent sessions) failed once during a full-suite run and then passed in isolation three times and in the package three times; this milestone touches no file in that package, so it is recorded as a pre-existing flake, not a regression, and not investigated further here. Unrelated gofmt churn in `namespaces.go`/`datastore_lifecycle_facts_test.go` (local Go 1.27 vs whatever last formatted them) was reverted rather than bundled in.

**Not verified live.** Both panels rendering at obs.bex.co is owed after deploy (t006), along with confirming the bootstrap re-run applies the new projection.
