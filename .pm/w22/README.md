# w22 — project workstream (worker22)

**Worker:** worker22. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [ ] **m311** — [Sandbox groups, as on Render: a default group per workspace, a concurrency limit, and 429 at the limit](m311/README.md) (12 tasks) ← from docs/ADR104 gaps 2, 12, 14
- [ ] **m312** — [Docker and Compose in a sandbox: a measured spike on a throwaway gVisor node](m312/README.md) (10 tasks) ← from docs/ADR104 gap 7
- [ ] **m313** — [Sandbox storage and memory meters: a suspended sandbox pays snapshot storage, as on Render](m313/README.md) (11 tasks) ← from docs/ADR104 gaps 8, 14
- [ ] **m314** — [Sandbox CPU billed on use, as on Render: a gVisor accuracy spike, then the cutover](m314/README.md) (9 tasks) ← from docs/ADR104 gap 8; depends on m313

## Inbox

- [ ] [001](001.md) — Pin sandbox `bash -c`, the list `limit` cap and the 40 GB filesystem with tests
- [ ] [002](002.md) — Meter sandbox egress, then alert on a workspace whose allow-all sandboxes egress far above its plan
- [ ] [003](003.md) — Sandbox groups: the optional project environment on "+ New > Sandbox Group"
