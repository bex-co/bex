# CLI sweep 534 — m113 regression recheck: `services` list in the large workspace

**Verdict: no bug; the m113 fix holds.** Same isolated session, read-only cross-check in the `bex` workspace (`tea-…kvg`, named explicitly per-command; the sweep's home workspace remains `bex-canary` and nothing was created or mutated anywhere), 2026-10-19T10:20Z.

Two consecutive `bex services -o json` runs in the workspace whose list used to exceed the edge timeout (110s→502, 125s→524 before m113): **0.898s** and **1.166s**, exit `0`, 9 rows — in line with m113's own acceptance numbers (0.811/0.875s). No timeout, no truncation.

- Deploy lag note: production runs image `8e57d4eaa121`; this is a live-behavior recheck of an already-closed milestone, recorded because the loop passes this workspace only on rechecks.
