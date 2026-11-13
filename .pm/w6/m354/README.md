# w6 · m354 — Routines finish: a cpx42 sandbox node, a 4 vCPU / 8 GiB routine template, a spread schedule and warm build caches

**Worker:** worker6 **Goal:** the eight scheduled `rt-*` routines complete instead of dying with their sandbox, and run faster, while the AX42-1 (m331) is parked until 2026-12-12. **Status:** in progress — t001 done (the sandbox pool runs one cpx42 since 2026-11-13 00:02 UTC).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Roll the sandbox pool onto cpx42 with kubelet reservations](done/t001.md) — **DONE** | 30m | — |
| t002 | [Sandbox pods tolerate a 300 s node stall](t002.md) | 15m | — |
| t003 | [An `agent-4c-8g` agent template in bex-api](t003.md) | 20m | — |
| t004 | [The routines use `agent-4c-8g` and a spread schedule](t004.md) | 15m | t003 |
| t005 | [Measure a routine on the new node](t005.md) | 1h | t001, t004 |
| t006 | [Warm build caches for the routines](t006.md) | 3h | t005 |
| t007 | [Measure again and record both in ADR105](t007.md) | 1h | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- The sandbox pool is one cpx42 with kubelet reservations, and sandbox pods tolerate a 300 s node stall.
- Every `rt-*` automation names `agent-4c-8g`, and at most two are scheduled to overlap.
- A routine run on the new node, before and after warm caches, is timed from its first turn's `started_at` to `completed_at`, and ADR105 records both beside the 2026-11-12 baseline.

## Source + Goal linkage

- **Source:** [ADR105 § Until the AX42-1 arrives](../../../docs/ADR105-agent-sandbox-hetzner-bare-metal.md). The user chose cpx42 on 2026-11-12 ("先试试 cpx42"). Baseline from production: on 2026-11-12, two of eight routines completed (3 h 34 min and 3 h 06 min); five died with their sandbox (terminated, status unavailable, out of memory) after four overlapped on a `cx33` holding 2.1 times its memory in limits.
- **Absorbs:** w9/m349 t006 (kubelet reservations, the 300 s toleration, a larger pool) and the sandbox pool's part of w1/144 (kernel 6.8.0-149).
- **External gates:** none. cpx42 costs $81.99 a month (user decision).
