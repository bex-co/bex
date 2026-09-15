# w7 · m89 — Finish production build-cache enablement

**Worker:** worker7 **Goal:** Land the corrected operator image, confirm Zot headroom, and complete the approved 48–72h production `BEX_BUILD_CACHE=registry` trial (or record an explicit hold). **Status:** **DONE 2026-09-15 on the hold branch** — the corrected image is live, capacity passes the admission budget (Zot 41.49% used, peak 44.62%, 20.03 GiB reserve above peak to 65%; expected cache growth 6–9 GiB, analytic), and the trial is **explicitly held with `BEX_BUILD_CACHE` unset in production** because its one remaining start condition is human: a named observer across the 48–72h window plus an independently verified armed rollback. t004/t005 deferred and carried as [`w7/047`](../../047.md).

**Estimate:** ~4–6h including standing closing tasks.

## Tasks (in order)

| id   | title                                                                                                      | est  | depends_on               |
| ---- | ---------------------------------------------------------------------------------------------------------- | ---- | ------------------------ |
| t001 | Unblock `deploy.yml` so an image containing m87+m88 ships — **DONE** 2026-09-15                             | 1.5h | —                        |
| t002 | Confirm production runs the post-m87/m88 digest; re-check Zot PVC vs 60%/65% budget — **DONE** 2026-09-15   | 30m  | w7/m89/t001              |
| t003 | Representative cache-size projection on largest actively rebuilt prod shape — **DONE** 2026-09-15           | 1h   | w7/m89/t002              |
| t004 | Start bounded trial: `BEX_BUILD_CACHE=registry` in prod overlay — **DEFERRED** → [`w7/047`](../../047.md)      | 45m  | w7/m89/t003              |
| t005 | Record retain-or-rollback from trial metrics — **DEFERRED** → [`w7/047`](../../047.md)                         | 30m  | w7/m89/t004              |
| t006 | Simplify — **DONE** 2026-09-15 (no code or config changed; documentation-only diff)                        | 30m  | w7/m89/t005              |
| t007 | Test coverage — **DONE** 2026-09-15 (no new code; m87/m88 coverage unchanged and green)                     | 45m  | w7/m89/t005              |
| t008 | Closeout — **DONE** 2026-09-15                                                                              | 15m  | w7/m89/t006, w7/m89/t007 |

## Definition of done

- [x] An operator/API image containing both m87 (native env invalidation) and m88 (clear-cache) is live in production — digest built from `1cb1f2d27dda`.
- [x] Zot PVC usage vs the 60% target / 65% trial admission ceiling re-checked, and a cache-size projection recorded — labelled analytic, with the reason no per-App measurement is possible (`BEX_BUILD_CACHE` is manager-wide) stated rather than glossed.
- [x] The trial **remains held with its named remaining start conditions**, and production configuration matches that decision (`BEX_BUILD_CACHE` unset). This is the second branch of the original DoD, taken deliberately.
- [x] Drill evidence lives under `docs/drills/2026-09-09-build-cache-enablement.md` (closeout section rewritten — the prior text still claimed the deployment and the measurement were pending); `w7/043` archived at `w7/done/043.md`; ADR060 D3 carries a dated enablement-status paragraph.

## Why it closed held rather than complete

The milestone owned everything technical and finished it. What is left is a 48–72 hour production observation window that needs a human on the other end of the stop rules — an agent session can neither be the observer nor independently arm the rollback, and starting the clock without one would turn a bounded trial into an unattended production change. Leaving the milestone open would not move that forward either, so the remaining two tasks are carried on the board as [`w7/047`](../../047.md), where the decision the user actually has to make is the whole content of the note.

## Source + Goal linkage

- **Source:** promoted from [w7/043](../043.md); user-approved `/pm-brainstorm for w7 for top 3 customer-impactfully work` 2026-09-09 #1; preflight in [docs/drills/2026-09-09-build-cache-enablement.md](../../../../docs/drills/2026-09-09-build-cache-enablement.md); measurements from [w7/m86](../m86/README.md).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) dependable push-to-deploy; [.pm/GOAL.md](../../../GOAL.md) #3; ADR060 D3 estate enablement.
- **Outcome:** the correctness work is live and the capacity question is answered with numbers; the warm-build win itself waits on the observed trial in `w7/047`.
- **Render parity omitted:** ops enablement / infra gate only — clearCache semantics already parity-fixed in m88; no REST/GraphQL/MCP/UI contract change.
