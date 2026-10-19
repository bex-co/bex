# CLI sweep 560 — workspace selection dialects and env/config precedence

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T14:1x–14:20Z. Only the loop's own `/tmp/qa501/cli.yaml` was mutated, and it was restored to `bex-canary` before the sweep closed.

Two invocation modes were compared: the loop wrapper (`BEX_WORKSPACE=tea-daif693dqjvc73e7as3g` exported) and the bare binary with `env -u BEX_WORKSPACE` against the same isolated config.

- **Failure dialects leave state intact:** `workspace set no-such-workspace-qa560` → client-side `no workspaces found with name …`, exit 1; uppercase `BEX` and partial `canary` likewise (name matching is exact and case-sensitive); empty name → `multiple workspaces found with name ; please specify workspace id`. A synthetic `tea-000…` id instead reaches the server (`RetrieveOwner`) and comes back as the client's raw `unexpected response: 403 Forbidden`. Traced in the pinned client (`pkg/tui/views/workspacelist.go` `SelectWorkspace`): `tea-`/`usr-` prefixes take the server-get path, anything else the client-side `ListOwners` name match (0 → none-found, >1 → multiple; empty string lists all, hence "multiple"). The server's uniform 403 for foreign-or-nonexistent owner ids is the non-enumerating answer; the blunt wording is the pinned client's generic non-2xx rendering, not a bex message. After every failed set, `workspace current` still read `bex-canary`.
- **Happy paths:** set by exact name (`bex` → `Workspace set to: bex (tea-d98210cbbpdc73dcrkvg)`) and by id both persist; with no env, `services` then scoped to that workspace (13 rows in `bex`, read-only).
- **Precedence:** with config set to `bex`, the env-pinned wrapper's `workspace current` still reported `bex-canary` — `BEX_WORKSPACE` wins over the persisted config. Config was restored to `tea-daif693dqjvc73e7as3g` by id and re-verified under both modes.

**Baseline changed mid-loop, not by this sweep:** `services` in `bex-canary` grew 8 → 10 with `qa-20261019-loop2-static` (13:53Z) and `qa-20261019-loop2-web` (14:02Z) — a second QA loop's fixtures, observed read-only and left untouched.

## Unrun

- `workspace set` for a real foreign (non-member) workspace id: indistinguishable from synthetic by design (same 403); no foreign id was available to name.
- Interactive (TTY) workspace picker behavior.
