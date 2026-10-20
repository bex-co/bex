# CLI sweep 836 — CLI workspace selection surface (live, 2026-10-20)

Read-only CLI probes. No new bug filed.

- `bex workspace` exposes `current`/`set` (no `list` subcommand — upstream pinned shape).
- `workspace set tea-000…` (synthetic) → `Error: unexpected response: 403 Forbidden` — the foreign-workspace refusal reaches the CLI as a clean error, and the active workspace is unchanged afterwards (`workspace current` still reports `bex-canary`, also pinned by the harness's `BEX_WORKSPACE`).
