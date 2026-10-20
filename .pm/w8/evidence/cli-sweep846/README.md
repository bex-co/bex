# CLI sweep 846 — CLI projects/environments/deploys/workspaces reads (live, 2026-10-20)

Read-only CLI probes. No new bug filed.

- `bex projects -o json` → `null` (canary currently has no projects; the CLI's empty-list-as-`null` rendering already recorded in sweeps 656/696/708).
- `bex environments prj-000…` → server-coded 404 `NOT_FOUND: project not found` surfaced verbatim.
- `bex deploys list srv-daif…` → the baseline's deploy history with full commit metadata (`id`, `message`, `createdAt` — the w2/m42/w9/001 commit-provenance contract through the CLI).
- `bex workspaces -o json` → all three account workspaces.
