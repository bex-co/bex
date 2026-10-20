# CLI sweep 823 — periodic status: pin, w8/135, evidence backlog (live, 2026-10-20)

Status sweep; nothing created.

- Production pin unchanged: `9d7ac8afa` → `ca30db3a3646`. Newer `origin/main` has `a8c215fdf feat(domains)… (w5/m137)` but it is not pinned, so production behavior is the pin's.
- w8/135 retest: `POST /registrycredentials` (bex body) still → 400 `invalid request body at /registry`, as expected until a pin passes the fix `410b9ae7b`. The finding stays open.
- Local tree: 129 uncommitted `cli-sweep*` evidence paths plus new [w8/137](../../137.md)/[w8/138](../../138.md) and the w8 README lines — all awaiting a future explicit ship. An untracked `lego/backend/internal/apps/static_rules.go` from another worker (likely the w8/137 fix in flight) was left untouched.
