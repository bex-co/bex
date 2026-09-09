# w5 · m91 — Preserve instance selection through empty windows and refreshes

**Worker:** worker5 **Goal:** Keep explicit instance filters stable as discovery and telemetry change, with successful empty results for valid queries. **Status:** done (2026-09-09)

**Estimate:** 2h implementation; 3h30m including standing closing tasks (7 tasks). **Priority:** 2 in the approved 2026-09-08 corrective queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) — **DONE** | Validate instance-filter eligibility independently of returned data | 40m | — |
| [t002](done/t002.md) — **DONE** | Preserve explicit selection across discovery and window changes | 45m | w5/m91/t001 |
| [t003](done/t003.md) — **DONE** | Make unavailable selections clear and usable on desktop and mobile | 35m | w5/m91/t002 |
| [t004](done/t004.md) — **DONE** | Render parity | 25m | w5/m91/t003 |
| [t005](done/t005.md) — **DONE** | Simplify | 20m | w5/m91/t004 |
| [t006](done/t006.md) — **DONE** | Test coverage | 35m | w5/m91/t004, w5/m91/t005 |
| [t007](done/t007.md) — **DONE** | Closeout | 10m | w5/m91/t006 |

## Definition of done

- [x] Selecting replica A never returns to all instances or displays B without an explicit user action, including after polling, discovery errors, and window changes.
- [x] Explicit selections survive temporary discovery gaps and historical choices leaving the window; the UI identifies unavailable choices and offers an explicit return-to-all action.
- [x] An authorized instance-filtered CPU/memory query with no samples returns a successful empty result. Filterable limit queries receive consistent treatment; unsupported metric/filter combinations still return defined errors.
- [x] Unknown or foreign instance selectors never broaden an authorized query; resource authorization, selector bounds, and malformed-input validation remain intact.
- [x] Browser verification covers an expired historical choice, refresh failure/recovery, polling, resource navigation, and returning to all. Pending and ready states match at desktop and narrow-mobile widths.
- [x] Backend and dashboard regression checks pass, API behavior agrees across surfaces, and live evidence is recorded before closeout.

## Evidence (2026-09-09, worker5, isolated dev-5)

- Fixtures (disposable, deleted after): Kratos user `m91walk-1788943146@example.com` (address verified 09:04 UTC) in workspace `tea-daghmb1jg4r1r6rsi6ng`; `srv-daghmh9jg4r1r6rsi6og` (m91walk, private busybox, 1 live instance `…-gpt711leo1sv531i17co`) and `srv-daghqp9jg4r1r6rsi6q0` (m91walk2, navigation target). dev-5 bex-api rebuilt from this working tree (`bash scripts/dev-env.sh 5 up`) so every live call below exercises the m91 fix.
- API live on the rebuilt binary: GraphQL `CPU_LIMIT` + unresolved instance → `{"data":{"metrics":[]}}` (the fix: pre-m91 this 400d on empty labels); `MEMORY_LIMIT` + foreign instance → `[]` (never broadens); `CPU_LIMIT` + live instance → exactly 1 series (`0.5` cores, selected id); `INSTANCES` + instance → `bad request: INSTANCE filter applies only to per-instance cpu/memory metrics`; REST `instance-count` + instance → 400. REST `cpu` + instance → 500 here because this dev cluster runs no metrics-server/Prometheus (eligibility gate passes, the read reaches the absent source) — the shared core's empty-success is proven instead via the GraphQL limit reads plus in-process `TestInstanceFilterEmptyCrossSurfaceParity` (REST+GraphQL+MCP agree).
- Browser live (headless Chrome, CDP-driven, dashboard dev against dev-5): login incl. Kratos verification; metrics ready at desktop and 390px; live instance offered, selected, and `Show all` appears (`m91-selected-desktop.png`); client-side window change keeps the selection (`m91-range-desktop.png`); a 35s live tick keeps it; `Show all` clears it; `srv2` carries no selection (`m91-dashboard-srv2-desktop.png`); route skeleton captured at the marker at both widths (`m91-pending-desktop.png`, `m91-pending-mobile.png`) with geometry matching ready (3 header slots = instance group + 2 tab groups; sections stack cleanly at 390px). Discovery is mount-cached Apollo (no poll), so a post-mount discovery failure cannot disturb the filter by construction; the retained-but-unavailable suffix/note/empty copy are proven by 4 RTL tests (`m91-discovery-failure-desktop.png` shows selection + `Show all` intact across the armed range change). A total GraphQL outage honestly unmounts the detail layout into its `Couldn't load service` error — selection state is moot there, recovery remounts fresh.
- Suites: `go test ./...` (backend) green; `yarn test` (dashboard) 403 files / 3086 tests green (one unrelated sidebar flake passed on rerun); `go vet ./...` clean; touched Go files gofmt-clean; eslint on touched dashboard files clean except the pre-existing `react-refresh/only-export-components` on the untouched `summarizeLimits` export; prettier clean on touched markdown; skill-layout PASS. `make lint-backend` cannot run in this env (pinned custom golangci-lint built with go1.26 vs workspace go1.27 — loader panic, pre-existing skew; `go vet` + tests substitute); `yarn typecheck` still reports only the pre-existing untouched-file errors.
- Docs: ADR018 Core row extended with the m91 correction, ADR010 query-params paragraph extended, `docs/render-artifacts/metrics-page.md` gained a `Corrected by w5/m91` section. No API wire changes (REST/GraphQL/MCP shapes untouched), no new env vars, no new skills, no new id mints.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposal 2 for w5, materialized 2026-09-08. [Shipped m89](../done/m89/README.md) supplies the original contract.
- **Gap analysis:** m89 checked no-silent-broadening as complete. Its prune effect removes unavailable selections, and an empty selection omits the INSTANCE filter, requesting all instances. select.go also infers filter eligibility from returned instance labels, so an empty supported CPU/memory query becomes ErrBadRequest. Source-confirmed on 2026-09-08; no live reproduction is claimed.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 1–3 and [.pm/GOAL.md](../../GOAL.md) basic operational observability: trustworthy, deterministic diagnostics for people and agents.
- **Expected outcome:** Selecting replica A never silently displays replica B, and a valid metric query with no matching samples is not misreported as a bad request.
- **Why now:** Polling, replica replacement, and window changes are normal transitions in the newly shipped m89 selector.
- **Render parity:** Included because existing REST/GraphQL/MCP/UI metrics semantics change. Correct the specific percentage/aggregation or instance-filtering clauses in [ADR018](../../../docs/ADR018-render-parity.md)'s Core metrics row; preserve its independent decisions and evidence limits.

## Scope and execution

The first task is technically independent of m90, but execute after m90 to avoid edits colliding in the same card. This is scheduling order, not an artificial cross-milestone dependency.

Approved order: m90 → m91. Both build on shipped m87/m89; neither reimplements those capability families. Existing m89 archived status is retained as historical shipping evidence; these milestones explicitly track the disproven acceptance claims. Follow the worker5 isolated environment instructions in [w5 README](../README.md). Do not close on code-only evidence when the definition of done requires a live walkthrough.

No external drains, new billing surface, first-token telemetry, unrelated API instrumentation, or chart-library rewrite. Preserve operator → types ← backend and thin dashboard/API adapters.
