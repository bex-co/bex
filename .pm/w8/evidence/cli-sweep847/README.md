# CLI sweep 847 — CLI restart + deploy create/cancel (live, 2026-10-20)

Two disposable free web services created, exercised via CLI, deleted (204s, then GETs 404). No new bug filed.

- `bex restart <id>` → `<id> restarted successfully`.
- `bex deploys create <id>` without `--confirm` prompts interactively and dies cleanly on EOF (`Error: EOF`, nothing created) — the pinned CLI's confirmation discipline; with `--confirm -o json` it returns the new `dep-…` (`status: created`, `trigger: api`).
- `bex deploys cancel <svc> <dep> --confirm` → `Deploy dep-… successfully canceled`.
