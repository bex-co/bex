# w5 · m90 — Correct CPU/memory percentages across replicas and rollouts

**Worker:** worker5 **Goal:** Normalize each replica with its own trustworthy limit before aggregation so usage charts remain correct through rollouts. **Status:** in-progress (t001–t007 done 2026-09-08; t008 open — service-level walkthrough green 2026-09-09, authed REST/GraphQL/MCP walkthrough green 2026-09-09 second session incl. a real gate bug fix in tree, dashboard UI checks + pending/ready captures remaining)

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
- [ ] A disposable dev-5 mixed-limit fixture proves selection, rollout, and deleted-instance behavior (service-level proof green 2026-09-09 — see Evidence). Desktop and narrow-mobile pending/ready captures show matching geometry and readable limit/unavailable states. — OPEN (see Evidence).
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

Live walkthrough PARTIAL (2026-09-09 session; t008 closeout gate still NOT passed — see remaining list). Environment blockers from 2026-09-08 are all resolved: the toolchain works via `/Applications/Docker.app/Contents/Resources/bin` on `PATH` (OrbStack is gone; its `/usr/local/bin` + `~/.orbstack/bin` symlinks dangle), the workload kubeconfig was regenerated (`clusterctl get kubeconfig bex` against `kind-bex-mgmt`, server pinned to the direct control-plane port `127.0.0.1:55003` — the `bex-lb` backend reports DOWN), and dev-5 is up with bex-api serving on `:54050`.

Two pre-existing main-branch defects surfaced (both worked around in disposable dev state only — repo untouched): `lego/backend/internal/store/migrations/0107_datastore_observed_checkpoints.up.sql` carries pasted `N|` line-number prefixes on 12 lines, so any fresh database fails at 0107 (`dirty=true`) and bex-api never starts — repaired SQL was applied by hand to dev-5's `bex-db` only, then 0108–0110 applied cleanly on restart. A `prometheus-community/prometheus` release (`m90fix` in namespace `m90prom`, alertmanager off, retention 2h, persistence off) now scrapes the workload cluster — `container_*` usage and `kube_pod_container_resource_limits` are both live.

Service-level fixture walkthrough GREEN (disposable `m90walk/m90mix` Deployment, since deleted): two replicas rolled 500m/512Mi → 1CPU/1Gi → 500m/512Mi with a ~0.3-core burner, producing three limit generations in one window. A temporary Go probe (since deleted, output below) drove the REAL `NewPrometheusResourceSource` + `NewPrometheusResourceLimitSource` + `Service.Metrics` against it: CPU 10/10 percentage points and memory 18/18 equal usage/own-limit-at-that-timestamp×100; AVG/MIN/MAX aggregates (7 points each, both resources) equal the instance math; raw-name INSTANCE selection isolates 1 replica and unknown selectors return empty; deleted pods' series truncate with no zero-fill; limit history keeps each generation's own denominator (0.5 vs 1.0 side by side in-window). Note: this cluster's scrape spacing needs ≥120s resolution for CPU `rate[60s]` windows (60s windows often hold <2 samples).

Authed-surface walkthrough GREEN 2026-09-09 (second session) against a real bex App — dashboard UI + captures still remaining (no browser in this environment). Fixture: `srv-daggflhjg4r5rcjp5af0` (`m90mix` private_service, workspace `tea-daggc49jg4r8hjs0vaeg`, busybox sha1sum burner + `httpd -p 3000` for the operator's startup probe), created on starter (500m/512Mi) 07:16:38Z, rolled to standard (1CPU/2Gi) 07:18:27Z, burner command patched 07:21Z so the new RS reached 2/2 ready ~07:24Z, then SUSPENDED (0 pods; service retained for the next session). Kratos user minted via the registration API flow, presented as `X-Session-Token`; dev-5 bex-api was rebuilt from the working tree and restarted with `BEX_PROM_URL=http://localhost:59091` (port-forward of workload `m90prom/m90fix-prometheus-server`) for the walkthrough, then dev-5 was restored to standard via `bash scripts/dev-env.sh 5 up` at session end (healthy, fix compiled in, log truncated per `up` convention — re-apply the manual `BEX_PROM_URL` restart + re-forward `:59091` for the dashboard session).

Findings: (a) REAL BUG, fixed in tree — the strict Render-query gate 400d `?percentage=true` AND m89's `&aggregateAllMethod=AVG` on every App-metrics REST path (bare-mux unit tests never see the gate; same shape as the w6/m96 blueprint `ownerId` miss). Fix: `percentage` + `aggregateAllMethod` entries in `renderQueryExtensions` for the eight ops sharing `parseMetricParams`, plus composed-server regression `TestMetricsPercentageQueryThroughComposedServer` (fails 400 pre-fix, passes post-fix). (b) REST DoD call 200: raw 5 series truncate per deleted pod (starter pods 07:19+07:21 only; standard pods 07:25+07:27 only), AVG/MIN/MAX mutually consistent at every point (07:19 raw {95.3, 85.9} → AVG 90.62), Total absolutes match percentages through own-limit denominators (0.476/0.5 = 95.2%, 0.953/1.0 = 95.3%), INSTANCE isolates 1 of 5, unknown selector returns `[]`; memory `%` AVG 200 (0.2–0.6%). (c) GraphQL `percentage:true aggregateAllMethod:"AVG"` 200, tracks REST (07:23 78.9 both). (d) MCP `get_metrics` (`metricTypes:["cpu"]`, percentage+AVG) 200, tracks REST. Re-verified suites: `go test ./...` (full backend, green), `go test ./internal/api/` post-fix (green), dashboard metrics card + use-metrics (28 tests green), `go vet` + `gofmt` clean on touched Go files.

Handoff (disposable dev identities, mode 600): `/tmp/m90walk-email.txt`, `/tmp/m90walk-pw.txt`, `/tmp/m90walk-session.txt`, `/tmp/m90walk-srv.txt`. Next session: re-forward `:59091`, resume + re-roll `srv-daggflhjg4r5rcjp5af0` for a fresh mixed window (Prometheus retention is 2h), then run the dashboard Percentage/Total checks and the desktop + narrow-mobile pending/ready captures.

REMAINING for t008 (strictly per this milestone's no-close-without-live-evidence rule): dashboard Percentage/Total tab checks and the desktop + narrow-mobile pending/ready geometry captures (needs a browser; unavailable in this child environment). Then check the two DoD boxes, set t008 `status: done`, move it to `done/`, `mv .pm/w5/m90 .pm/w5/done/m90` (then `rmdir` the original — no tombstone), and flip the m90 workstream checkbox to `[x]`.
