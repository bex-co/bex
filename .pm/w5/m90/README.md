# w5 · m90 — Correct CPU/memory percentages across replicas and rollouts

**Worker:** worker5 **Goal:** Normalize each replica with its own trustworthy limit before aggregation so usage charts remain correct through rollouts. **Status:** in-progress (t001–t007 done 2026-09-08; t008 open — dev-5 live walkthrough blocked, no cluster toolchain in this environment)

**Estimate:** 3h30m implementation; 5h including standing closing tasks (8 tasks). **Priority:** 1 in the approved 2026-09-08 corrective queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Normalize each instance before replica aggregation — **DONE** | 60m | — |
| [t002](done/t002.md) | Associate retained usage with trustworthy instance limits — **DONE** | 60m | w5/m90/t001 |
| [t003](done/t003.md) | Expose consistent percentage semantics through API adapters and hooks — **DONE** | 45m | w5/m90/t002 |
| [t004](done/t004.md) | Render truthful limits and unavailable percentage states — **DONE** | 45m | w5/m90/t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 25m | w5/m90/t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 20m | w5/m90/t005 |
| [t007](done/t007.md) | Test coverage — **DONE** (except the live-walkthrough sub-item, blocked — see Evidence) | 35m | w5/m90/t005, w5/m90/t006 |
| [t008](t008.md) | Closeout | 10m | w5/m90/t007 |

## Definition of done

- [x] For replicas using 0.4/0.5 CPU and 0.5/1 CPU at the same timestamp, raw percentages are 80% and 50%; MIN is 50%, MAX is 80%, AVG is 65%. Equivalent memory cases pass.
- [x] Normalization precedes replica aggregation; selection still precedes aggregation. Total mode preserves absolute samples and missing values are never filled with fabricated zeroes.
- [x] Historical points never inherit a different instance’s limit or an unproven current limit. Missing, zero, or otherwise untrustworthy denominators omit the affected percentages with an explicit unavailable explanation; trustworthy retained history remains usable.
- [x] REST, GraphQL, MCP, and dashboard agree on the supported percentage contract, defaults, units, and errors. Any bex extension or upstream evidence limit is documented.
- [ ] A disposable dev-5 mixed-limit fixture proves selection, rollout, and deleted-instance behavior. Desktop and narrow-mobile pending/ready captures show matching geometry and readable limit/unavailable states. — BLOCKED (see Evidence).
- [ ] Affected backend/dashboard checks pass; live evidence and any source limitations are recorded before closeout. — checks pass; live evidence outstanding.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposal 1 for w5, materialized 2026-09-08. [Shipped m89](../done/m89/README.md) supplies the original contract.
- **Gap analysis:** m89 checked mixed-limit pairing as complete, but application-metrics-card.tsx still divides every point by latestValue of an aggregate limit. service.go resourceLimitMetric returns current pod limits only. This is source-review evidence from 2026-09-08, not a live reproduction. This milestone corrects that specific incomplete criterion; it does not reimplement m89 selection or m87 identity.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 1–3 and [.pm/GOAL.md](../../GOAL.md) basic operational observability: trustworthy, deterministic diagnostics for people and agents.
- **Expected outcome:** A user or agent can identify a saturated replica without misleading percentages or historical samples inheriting another replica’s limit.
- **Why now:** m89 exposes replica aggregation directly, making the pre-existing single-denominator shortcut more consequential.
- **Render parity:** Included because existing REST/GraphQL/MCP/UI metrics semantics change. Correct the specific percentage/aggregation or instance-filtering clauses in [ADR018](../../../docs/ADR018-render-parity.md)'s Core metrics row; preserve its independent decisions and evidence limits.

## Scope and execution

Historical limit availability is a verify-first implementation decision. Reuse available telemetry, preserve truthful values, and explicitly omit unsupported history; do not create a new telemetry warehouse or fabricate historical limits.

Approved order: m90 → m91. Both build on shipped m87/m89; neither reimplements those capability families. Existing m89 archived status is retained as historical shipping evidence; these milestones explicitly track the disproven acceptance claims. Follow the worker5 isolated environment instructions in [w5 README](../README.md). Do not close on code-only evidence when the definition of done requires a live walkthrough.

No external drains, new billing surface, first-token telemetry, unrelated API instrumentation, or chart-library rewrite. Preserve operator → types ← backend and thin dashboard/API adapters.

## Evidence (2026-09-08, worker5 session)

Verify-first decision (t002): per-instance limit history IS retained telemetry — the `kube-state-metrics` job in `deploy/gitops/base/prometheus.yaml` scrapes `kube_pod_container_resource_limits` cluster-wide, so no new telemetry was built; percentage mode joins each usage sample against the limit observed at that timestamp and omits untrustworthy denominators.

Recorded results (all observed in-session):

- `cd lego/backend && go test ./...` — entire backend suite green.
- `cd lego/backend && go test ./internal/metrics/ ./internal/api/` — green, including 9 new w5/m90 regressions in `lego/backend/internal/metrics/percentage_test.go`: mixed-limit 80/50 + MIN/MAX/AVG 50/80/65 (CPU), memory equivalent, Total-mode absolutes, mid-window rollout keeping the old denominator (80 then 40), deleted/predated/zero-denominator omission, limit-source error surfacing, limit PromQL shape, REST/GraphQL/MCP parity (65 AVG on all three), and a stub-Prometheus end-to-end through the real `NewPrometheusResourceSource` + `NewPrometheusResourceLimitSource` (80/50 raw, select-then-aggregate, 65 AVG).
- `cd dashboard && yarn test` — 402 files / 3077 tests green, including rewritten `application-metrics-card` tests (server percentages rendered as-is, `Limits vary`, `Percentages unavailable` vs `No data in range`) and new `use-metrics` percentage-variable tests.
- `cd dashboard && yarn typecheck` — no errors under `src/features/metrics/` (remaining errors are pre-existing `outboundIps`/`EnvGroupView` mismatches in untouched files).
- Schema codegen: `TestDumpGraphQLSchema` + `SCHEMA_JSON=... yarn codegen` produces exactly the one-line `percentage?: InputMaybe<Scalars['Boolean']['input']>` hunk in `MetricsQueryInput` — the hand-splice has zero drift; `definitions.ts` unchanged (input variables carry no field list).
- `go vet` on `internal/metrics`, `internal/api`, `cmd/api` — clean. `gofmt` clean on all touched Go files. Prettier clean on touched docs; skill layout validates.
- No new environment variables (`BEX_PROM_URL` reused for both usage and limit history); no new resource ids (none needed); no operator changes (`make test` untouched per "whichever touched").

BLOCKED — live walkthrough (t008 closeout gate, NOT passed): `bash scripts/dev-env.sh 5 status` reports everything down, and no cluster toolchain is executable from this environment (`kubectl`/`docker` resolve in `/usr/local/bin` listings but fail at exec even by absolute path; `kind` present but no cluster; no browser binaries for pending/ready captures). Per the milestone's own rule ("Do not close on code-only evidence when the definition of done requires a live walkthrough"), t008 stays open: the next session with a working dev-5 must run the disposable mixed-limit fixture (selection, rollout, deleted-instance), the desktop + narrow-mobile pending/ready comparison, then move `m90/` to `w5/done/m90/` and flip the workstream checkbox.
