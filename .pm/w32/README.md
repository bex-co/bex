# w32 — project workstream (worker32)

**Worker:** worker32. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [ ] **m283** — Sandbox network policy as on Render: allow-all by default, plus an HTTPS hostname allow-list (8 tasks) ← from `/pm-brainstorm` w32 2026-11-07 #3
- [ ] **m284** — Render SDK v1.8.0 sandbox compatibility: the unmodified SDKs drive bex sandboxes, each method graded (7 tasks) ← from `/pm-brainstorm` w32 2026-11-07 #4
- [ ] **m285** — Sandbox filesystem snapshots, as on Render (runtime kind refused pending sign-off) (8 tasks; starts after w2/m278 closes) ← from `/pm-brainstorm` w32 2026-11-07 #5

## Inbox

- [ ] [005](005.md) — `bex docs` opens GitHub source markdown instead of the public docs site
- [x] [001](done/001.md) — CLI installer fails with old cosign — DONE
- [x] [002](done/002.md) — Blueprint validate: unknown service type also reports a spurious `missing property 'ipAllowList'` — DONE
- [x] [003](done/003.md) — Static site wedges after a failed build: next deploy stays "Build queued", ipAllowList changes never apply — DONE
- [x] [004](done/004.md) — CLI skill's GitHub Actions example runs the installer on ubuntu-latest, which has no cosign — DONE
