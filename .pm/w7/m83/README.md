# w7 · m83 — Build-plane observability + the two structural guards

**Worker:** worker7 **Goal:** the next capacity claim is read from a series instead of argued from arithmetic, and the two thin invariants the build plane silently depends on are guarded. **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on               |
| ---- | --------------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | `bex_build_queue_seconds` + `bex_builds_queued` / `bex_builds_active`               | 45m | —                        |
| t002 | Alert on build queue p95 breach / build `Pending` beyond N minutes                  | 30m | w7/m83/t001              |
| t003 | Prewarm follows the build pools: build-taint toleration + completeness guard        | 35m | —                        |
| t004 | `clusterapi-validate.sh` arithmetic guard: CA hint ≥ build request + DaemonSet floor | 45m | w7/m83/t003              |
| t005 | D6: global `BEX_MAX_ACTIVE_BUILDS` ceiling + re-count-after-create overshoot fix     | 45m | w7/m83/t001              |
| t006 | D7: check the build baseline env into the gitops path                               | 20m | w7/m83/t005              |
| t007 | Simplify the code this milestone changed                                            | 30m | w7/m83/t002, w7/m83/t004, w7/m83/t006 |
| t008 | Test coverage for the shipped behavior                                              | 40m | w7/m83/t002, w7/m83/t004, w7/m83/t006 |
| t009 | Closeout                                                                            | 15m | w7/m83/t007, w7/m83/t008 |

## Definition of done

Queue time and run time are separate series, with an alert on the former. The prewarm DaemonSet is scheduled on every node carrying the build taint, and its guard turns CI red without the toleration. The arithmetic guard turns CI red if any build pool's cluster-autoscaler hint drops below the build request plus the DaemonSet floor. Admission has a global active-build ceiling and self-corrects overshoot. The build baseline env is readable from git rather than set out of band.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.3, §3.4, §3.5 (P2/P3/P4) and §4 (the measured capacity close-out); [docs/ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md) D5/D6/D7. **Retires [`.pm/w7/024.md`](../done/024.md)**, whose branch 3 was refuted by measurement — t004 is that note's surviving residue.
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #2 (basic obs for operation) and #6 (add & remove physical machine).
- **Expected outcome:** cold-node builds stop paying the image-pull tax; a 305 MiB scheduling margin can no longer erode silently; and "should we add machines?" becomes a query rather than an argument.
- **Why now:** this milestone's own investigation is the evidence. With no queue series, the 2026-08-17 capacity question had to be settled by hand against a live cluster — and the first answer was **wrong** (it assumed cilium and friends carried meaningful memory requests; measured, they request 0). Two prior incidents in the same class (2026-08-08 wedged build, 2026-08-11 22-minute Pending) also presented as silence. Separately, D8 tainted the build pools on 2026-08-15 and the prewarm DaemonSet was left behind on the serving pool, which measurement confirmed.
- **Render parity task omitted:** yes — this is operator/infra mechanism only (Prometheus series, a DaemonSet toleration, CI guards, an admission ceiling, and gitops env). Nothing here changes a REST, GraphQL, MCP or dashboard surface.
