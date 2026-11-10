# w22 — project workstream (worker22)

**Worker:** worker22. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [ ] **m311** — [Sandbox groups, as on Render: a default group per workspace, a concurrency limit, and 429 at the limit](blocked/m311/README.md) (12 tasks) ← from docs/ADR104 gaps 2, 12, 14 — **BLOCKED (the production deploy of `88a13216f`: scheduled deploys run at 19:00, 00:00 and 05:00 UTC, next 2026-11-10 19:00 UTC; then run `isolation-matrix.yml` on production, record the run id, flip the ADR018 and checklist rows, close out)**; t001–t006, t008–t010 and t012 done and shipped; t007 and t011 remain
- [ ] **m312** — [Docker and Compose in a sandbox: a measured spike on a throwaway gVisor node](blocked/m312/README.md) (10 tasks) ← from docs/ADR104 gap 7 — **BLOCKED (a throwaway Kubernetes node with gVisor and Cilium, from the user or infra: t005's isolation tests and t007's final option recommendation cannot run without one; billable, so it needs an owner and a teardown date)**; t003 and t006 done and shipped, t001/t002/t004 half done on a local VM (ADR104 D6 "D6 spike results")
- [ ] **m313** — [Sandbox storage and memory meters: a suspended sandbox pays snapshot storage, as on Render](blocked/m313/README.md) (11 tasks) ← from docs/ADR104 gaps 8, 14 — **BLOCKED (t006's production check needs, from the user or infra: the deploy of this change; the two Zot users with read and delete rights and bex-api's credential and `BEX_SANDBOX_SNAPSHOT_REGISTRY_*` env, filed as w22/006; `scripts/stripe-billing-setup.py` run on the test and live Stripe accounts and then `BEX_STRIPE_SANDBOX_STORAGE_METER=1`; then a suspended sandbox to compare with the registry)**; t001–t005 and t007–t010 done and shipped, t006 and t011 remain
- [ ] **m314** — [Sandbox CPU billed on use, as on Render: a gVisor accuracy spike, then the cutover](blocked/m314/README.md) (9 tasks) ← from docs/ADR104 gap 8; depends on m313 — **BLOCKED (t001's spike needs production: a sandbox to load and Prometheus to read, from the user or infra; QA login is broken, w32/m284; every other task depends on its verdict)**; thresholds recorded before any measurement, nothing built

## Inbox

- [x] [001](done/001.md) — Pin sandbox `sh -c`, the list `limit` cap and the 40 GB filesystem with tests
- [ ] [002](002.md) — Meter sandbox egress, then alert on a workspace whose allow-all sandboxes egress far above its plan
- [ ] [003](003.md) — Sandbox groups: the optional project environment on "+ New > Sandbox Group"
- [ ] [004](004.md) — Sandbox group list and paging: where bex's answers differ from Render's error vocabulary
- [ ] [005](005.md) — Agent-session sandboxes: decide whether their retained image is billed storage
- [ ] [006](006.md) — Provision the storage meter's registry users as operator-managed Zot credentials
- [ ] [007](007.md) — Unparameterized lists return the full set where Render returns 20: record the divergence in ADR018 with the user's sign-off
- [ ] [008](008.md) — Enforce the 40 GB writable filesystem on sandbox pods, after checking kubelet's accounting under gVisor
