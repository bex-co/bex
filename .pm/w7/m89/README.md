# w7 · m89 — Finish production build-cache enablement

**Worker:** worker7 **Goal:** Land the corrected operator image, confirm Zot headroom, and complete the approved 48–72h production `BEX_BUILD_CACHE=registry` trial (or record an explicit hold). **Status:** todo

**Estimate:** ~4–6h including standing closing tasks.

## Tasks (in order)

| id   | title                                                                                                                          | est  | depends_on      |
| ---- | ------------------------------------------------------------------------------------------------------------------------------ | ---- | --------------- |
| t001 | Unblock `deploy.yml` so an image containing m87+m88 ships                                                                      | 1.5h | —               |
| t002 | Confirm production runs the post-m87/m88 digest; re-check Zot PVC vs 60%/65% budget                                            | 30m  | w7/m89/t001     |
| t003 | Representative cache-size projection on largest actively rebuilt prod shape (≥2 GC intervals)                                  | 1h   | w7/m89/t002     |
| t004 | Start bounded trial: `BEX_BUILD_CACHE=registry` in prod overlay; 48h observe / 72h hard stop + rollback armed                  | 45m  | w7/m89/t003     |
| t005 | Record retain-or-rollback from trial metrics (PVC, push p95, stale/clear correctness)                                          | 30m  | w7/m89/t004     |
| t006 | Simplify                                                                                                                       | 30m  | w7/m89/t005     |
| t007 | Test coverage                                                                                                                  | 45m  | w7/m89/t005     |
| t008 | Closeout                                                                                                                       | 15m  | w7/m89/t006, w7/m89/t007 |

## Definition of done

- An operator/API image containing both m87 (native env invalidation) and m88 (clear-cache) is live in production.
- Zot PVC usage vs the 60% target / 65% trial admission ceiling is re-checked; a representative cache-size projection for the largest actively rebuilt production workload is recorded (or the trial is explicitly held with the missing measurement named).
- The approved 48–72h trial either completes with a dated retain-or-rollback decision, or remains held with named remaining start conditions — production configuration matches that decision.
- Drill evidence lives under `docs/drills/` and `w7/043` is archived as the source note.

## Source + Goal linkage

- **Source:** promoted from [w7/043](../done/043.md); user-approved `/pm-brainstorm for w7 for top 3 customer-impactfully work` 2026-09-09 #1; preflight in [docs/drills/2026-09-09-build-cache-enablement.md](../../../docs/drills/2026-09-09-build-cache-enablement.md); measurements from [w7/m86](../done/m86/README.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) dependable push-to-deploy; [.pm/GOAL.md](../../GOAL.md) #3; ADR060 D3 estate enablement.
- **Expected outcome:** Tenants feel faster warm builds (Dockerfile 179s→38s class), or an honest hold with concrete thresholds — the shipped cache feature is no longer permanently dark.
- **Why now:** Correctness blockers closed; Sep 9 approval expanded the recommend-only note into deploy-unblock + size budget + trial; largest measured customer-visible latency win on the board.
- **Render parity omitted:** ops enablement / infra gate only — clearCache semantics already parity-fixed in m88; no REST/GraphQL/MCP/UI contract change.
