# w7 · m39 — Per-App registry credentials: production hardening

**Worker:** worker7 **Goal:** The per-App Zot pull-credential feature (w7/m36) is operable in production: a compromised credential can be rotated without deleting the App, delete-time reclamation is truthful and regression-guarded, bulk App churn no longer fails reconciles on htpasswd write contention, and the operator-owned Zot config can't silently drift from the deployed registry. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Rotation path: on-demand credential re-issue (Secret + htpasswd + ACL atomically) + runbook  | 45m | —                      |
| t002 | Fix the GC-comment drift: truthful comment + delete-leak regression test                     | 30m | —                      |
| t003 | Replace the 5-retry hard failure with conflict-requeue backoff; test concurrent reconciles   | 40m | —                      |
| t004 | De-risk `baseZotConfig` drift: retention knob + single-source/drift guard                    | 40m | —                      |
| t005 | Simplify                                                                                     | 20m | t001, t002, t003, t004 |
| t006 | Test coverage                                                                                | 30m | t001, t002, t003, t004 |
| t007 | Closeout                                                                                     | 15m | t006                   |

## Definition of done

A compromised per-App credential can be rotated without deleting the App: after the documented rotation action, the old password is rejected by Zot, the new one is accepted, and the App's pods still pull (verified via `scripts/verify-per-app-registry-isolation.sh` or an envtest equivalent). Deleting an App provably reclaims its `reg-pull-<name>` Secret, htpasswd entry, and ACL entry — the misleading "garbage-collected via owner references" comment is gone and a regression test guards the explicit delete. N concurrent App creates no longer exhaust the htpasswd optimistic-lock retries into a reconcile failure (conflict ⇒ requeue/backoff, tested). The Zot tag-retention count is configurable (new env var mirrored into CLAUDE.md's table + `.env.example`/`.env.template`), and the operator-rendered Zot config's contract with the deployed registry (mount path, port) is guarded against silent drift.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 15, 2026-07-15 — mechanical-consistency mining over the freshly shipped 57f146d2..441d6040 range; all four findings verified by reading `lego/operator/internal/registry/creds.go`: no rotation path (`:84-120` — password minted once, re-derived forever), GC comment claims owner-reference collection that doesn't exist and can't (cross-namespace) (`:136` vs the explicit delete at `app_controller.go:2306`), singleton-Secret write contention with a hard `for range 5` cap that fails the reconcile (`:153-303`), and the hardcoded `baseZotConfig` JSON literal owning policy (`mostRecentlyPushedCount: 5`) and contract values (mount path, port) with no guard (`:477-520`).
- **Goal linkage:** tenant isolation (docs/ADR022-tenant-isolation.md § Per-App pull credentials) — credential lifecycle is the operational half w7/m36 didn't ship; w7's charter is exactly this hardening.
- **Expected outcome:** the per-App credential feature survives production incidents (rotate on compromise), refactors (no leak trap), and scale (bulk onboarding) without operator surgery.
- **Why now:** the code is one day old — harden before it calcifies; the misleading GC comment is an active trap for the next controller refactor; w7 has only two open milestones.
- **Render parity:** omitted — operator-internal mechanism (registry credential plumbing), no REST/GraphQL/MCP/UI surface change.
