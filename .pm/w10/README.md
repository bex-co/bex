# w10 — Roadmap capacity (worker10)

**Worker:** worker10 Reserved 2026-07-15 as an empty workstream for future `/pm` scheduling; milestones will be added only when their scope and goal linkage are known.

## Local dev environment

Develop against `.pm/w10/dev-10/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-10` gets its own Kratos + Hydra + Mailpit (namespace `dev-10-auth`) and app namespace (`dev-10`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=10 (`dev-10/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash .pm/w10/dev-10/up.sh` — bring it up (idempotent — safe to re-run)
- `bash .pm/w10/dev-10/status.sh` — health check (processes, pods, HTTP)
- `bash .pm/w10/dev-10/down.sh` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)

`up.sh` prints the dashboard command to point at it once bex-api is running.

## Milestones

- [x] **m1** — Restore operator reconciliation under namespace-scoped Secret RBAC (6 tasks) ← from `w10/001` production incident
- [x] **m2** — Docs & code truth sweep (9 tasks) ← from `/pm-brainstorm` round 12, 2026-07-15 (this round's miners returned three false gaps traced to stale docs — ADR006:521/:285, `render-artifacts/key-value.md:27`; plus the twice-filed MCP-table backfill, `maxShutdownDelaySeconds` bex.yml drift, `gqlStr`×7, and the recurring 16-finding lint debt); coordinates with, never duplicates, `w5/013`
- [x] **m3** — Ledger & board truth sweep round 2: ADR018 backlog + FUTURE-MAYBE sync (8 tasks) ← from `/pm-brainstorm` round 13, 2026-07-15 (five ADR018 gap-backlog rows cite resolved `done/` items as open owners; ADR028/events-row pointer gaps; FUTURE-MAYBE entries 2+4 out of sync; coordinates with, never duplicates, `w5/013` and w10/m2); t008 added by round 15 (w7/done/m36 task-file drift + w4 README m23 checkbox) — done 2026-07-15 (three ADR018 backlog rows + events/PR-previews/untracked rows corrected with verified pointers; FUTURE-MAYBE w8/m7 → Done + double-delivery clause; w7/done/m36 bookkeeping synced; several cited drifts found already fixed by their own closeouts and verified as such), moved to `done/m3/`
