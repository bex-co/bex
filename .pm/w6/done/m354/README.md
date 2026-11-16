# w6 · m354 — Routines finish: a cpx42 sandbox node, a 4 vCPU / 8 GiB routine template, a spread schedule and warm build caches

**Worker:** worker6 **Goal:** the eight scheduled `rt-*` routines complete instead of dying with their sandbox, and run faster, while the AX42-1 (m331) is parked until 2026-12-12. **Status:** done (2026-11-16: seven of eight routines completed with a PR on their first full day with the cap cleared, against two on 2026-11-12; under targeted checks the median first turn is 23 min; the replay is 2,437 s → 367 s; ADR105 records it all).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Roll the sandbox pool onto cpx42 with kubelet reservations](done/t001.md) — **DONE** | 30m | — |
| t002 | [Sandbox pods tolerate a 300 s node stall](done/t002.md) — **DONE** | 15m | — |
| t003 | [An `agent-4c-8g` agent template in bex-api](done/t003.md) — **DONE** | 20m | — |
| t004 | [The routines use `agent-4c-8g` and a spread schedule](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Measure a routine on the new node](done/t005.md) — **DONE** | 1h | t001, t004 |
| t009 | [Agent sandboxes get 30Gi of ephemeral storage, and a storage eviction says so](done/t009.md) — **DONE** | 45m | t005 |
| t006 | [Warm build caches for the routines](done/t006.md) — **DONE** | 3h | t005 |
| t007 | [Measure again and record both in ADR105](done/t007.md) — **DONE** | 1h | t006 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t007 |

## Definition of done

- The sandbox pool is one cpx42 with kubelet reservations, and sandbox pods tolerate a 300 s node stall.
- Every `rt-*` automation names `agent-4c-8g`, and at most two are scheduled to overlap.
- A routine run on the new node, before and after warm caches, is timed from its first turn's `started_at` to `completed_at`, and ADR105 records both beside the 2026-11-12 baseline.

## Source + Goal linkage

- **Source:** [ADR105 § Until the AX42-1 arrives](../../../docs/ADR105-agent-sandbox-hetzner-bare-metal.md). The user chose cpx42 on 2026-11-12 ("先试试 cpx42"). Baseline from production: on 2026-11-12, two of eight routines completed (3 h 34 min and 3 h 06 min); five died with their sandbox (terminated, status unavailable, out of memory) after four overlapped on a `cx33` holding 2.1 times its memory in limits.
- **Absorbs:** w9/m349 t006 (kubelet reservations, the 300 s toleration, a larger pool) and the sandbox pool's part of w1/144 (kernel 6.8.0-149).
- **External gates:** none. cpx42 costs $81.99 a month (user decision).

## Unblock 2026-11-16 (verified evidence)

- **Affects:** t007, t008. Source: verified evidence; no user decision is needed or recorded.
- **Time gate cleared.** The routines' first full day with the spend cap cleared ended 2026-11-14 08:00 UTC, and they have run two more days since.
- **Deploy gate cleared.** deploy.yml run 43816470794 deployed `3c6654df6` to production; its `deploy` job succeeded at 06:46:40 UTC on 2026-11-16.
  - It carries `e2d9a4f7d` (w6/126): a turn keeps its first completion time, and migration 0256 repaired the turns the idle reap had re-stamped. Production `schema_migrations` reads 278, not dirty, and each routine turn's `completed_at` now equals its last transcript row's time. For example, `rt-prune-useless-tests`' 2026-11-13 CI turn reads 22:23:53 for both.
  - It also carries `16f1ff028` (w6/127, `TMPDIR=/var/tmp`). The sandboxes running at 12:45 UTC read `TMPDIR=/var/tmp`.
- **Next.** t007 tallies the routines from `agent_automation_runs` joined to their turns, records it in ADR105 and runs w6/127's check. Then t008 closes out.

