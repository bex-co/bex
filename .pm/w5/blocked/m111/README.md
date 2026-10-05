# w5 · m111 — `instance_seconds` stops metering terminated pods through Prometheus's 5-minute lookback

**Worker:** worker5 **Goal:** A pod that ends inside the metered hour is billed for its life only, not ~285 s more, for Apps and datastores. **Status:** blocked — t001–t004, t006–t008 done 2026-10-05; t005 needs the deployed bex-api and a rolled-up hour; t009 closeout follows

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | End the promtool fixtures the way production series end — **DONE** | 30m | — |
| t002 | Liveness-gate the presence query — **DONE** | 45m | t001 |
| t003 | Run the promtool test in backend CI — **DONE** | 30m | t002 |
| t004 | Correct the comments and ADR023's claim — **DONE** | 15m | t002 |
| t005 | Replay a rollout inside one hour live | 30m | t003, t004 |
| t006 | Render parity — **DONE** | 15m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006, t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- promtool fixtures whose series simply end read: a 5-minute pod 300 ± 15; a rollout its lifetime plus co-run seconds; a full hour 3600; two replicas 7200. This holds on promtool 2.55.1 and 3.x.
- The test runs in backend CI.
- A live replay of one rollout inside an hour reads lifetime + overlap ± one step, not ~285 s more.
- Comments and ADR023 describe the liveness-gated query.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `e2510371c` (w4/m173 t001). The subquery's instant selector looks back 5 minutes at each 15 s step, and cAdvisor series get no staleness markers in production (w4/m110). So every pod that ends inside the window is metered for ~285 s after its last sample. Replayed in promtool 3.14.0: a 5-minute pod reads 585 s, and only the test fixture's explicit `stale` marker makes it 300 s. `instance_seconds` feeds the Stripe meters `instance_seconds.<kind>.<tier>` (`lego/backend/internal/billing/stripe.go:421-434`); paid tiers are priced per second (`pricing/pricing.go:56-58`).
- **Goal linkage:** ADR023 usage metering; ADR040 Stripe meter export; ADR030 pricing.
- **Expected outcome:** Rollouts, scale-downs and pod replacements stop adding ~285 s per terminated pod to paid instance-seconds.
- **Why now:** Billing correctness. w4/m173's DoD "Rollout" bullet will fail its replay until this lands, and its t007 (meter deleted resources) would add the same tail to every deleted resource.
- **Render parity included:** usage numbers change on REST/GraphQL/MCP and the dashboard Billing page.
- **Gap analysis vs w4/m173 (blocked, worker4):** m173's open tasks are t002 (historical correction decision), t007 (meter resources deleted before their hour's rollup, and align the rollup tick) and t006 (closeout). None covers the lookback tail. This milestone owns the query; m173 keeps enumeration, rollup timing and historical repair, and its t002 paid-exposure read should include this overcount. A pointer was added to m173's README.

## Evidence — 2026-10-05

- Reproduced in Prometheus's engine: with series ending as cAdvisor's do (no staleness marker), the w4/m173 query billed a 5-minute pod 570 s and a rollout's old pod ~270 s extra (promtool 3.14.0 and 2.54.1).
- Fixed: `instanceSecondsQuery` gates each step on a sample within 44 s (`count_over_time(…[44s])` inside the per-pod `max`), so a terminated pod stops counting within 44 s of its last sample; gaps up to 44 s (kubelet housekeeping spacing, a missed scrape) lose nothing.
- Verified: fixtures for short life, sidecars, replicas, rollout overlap, one/two/three missed scrapes and an overlapping container restart give identical values on promtool 2.54.1 and 3.14.0; the backend CI job now installs promtool 2.55.1 so the test runs instead of skipping. Mutation checks: the old query, a 29 s window and a 59 s window each fail.
- Not verified: the ~34 s stored-sample spacing is modeled from kubelet/cAdvisor source. t005's live read below also checks it with `max(max_over_time((time() - timestamp(container_memory_working_set_bytes{container!=""}))[1h:15s]))` ≤ 44.

## Blocked — t005 live replay (after deploy)

After the deploy that includes this milestone, on owned Free `qa-` fixtures in `bex-canary` (QA session per `/qa-find-bugs`):

1. Keep one web service alive across a full hour and trigger one deploy mid-hour; after that hour rolls up (`GET /v1/usage?ownerId=…`), `instance_seconds` reads its lifetime + the rollout overlap within 45 s, not +~285 s (w4/m173's "Rollout" DoD bullet).
2. Read the stored-sample spacing query above against the live Prometheus; if it exceeds 44 s, widen `instanceLivenessSeconds` and its fixtures.
3. Coordinate with w4/m173 t007 (deleted-resource enumeration, rollup tick alignment): a fixture deleted before its hour rolls up won't have a row until t007 lands. Record evidence here, then run t009.
