# CLI sweep 517 — bogus-workspace configuration dialects

**Verdict: no bug filed.** Same isolated session and isolated CLI config (the only state mutated, restored in-sweep), HEAD build `8e57d4eaa`, 2026-10-19T09:21Z.

| probe | observed result |
| --- | --- |
| `workspace set <synthetic tea-id>` | Exit `1`, server `403 Forbidden`; the config is **not** rewritten — `workspace current` still reads `bex-canary` immediately after. |
| `workspace set <unknown name>` | Exit `1`, named client error (`no workspaces found with name …`). |
| `BEX_WORKSPACE=<synthetic tea-id> services` | Exit `1`, `you are not allowed to take this action` — the env override wins over the valid config and fails closed; it never falls back to the configured workspace. |
| `BEX_WORKSPACE=<synthetic> workspace current` | Exit `1`, `failed to list owners: 403` — same precedence. (In this sweep's first run the env leaked through a shell-function prefix and made a *legitimate* `workspace set` look broken; with a clean environment `current` reads `bex-canary` and `services` lists normally. Harness artifact, recorded so it isn't re-chased.) |
| Restore `workspace set <bex-canary id>` | Exit `0`; `current` and `services` verified healthy after. |

## Safety and unrun checks

- Only this run's isolated CLI config was written; no server state changed (a refused `set` writes nothing).
- Unrun: `workspace set` by ambiguous name (two accessible workspaces sharing a name — none exists for this identity), `RENDER_WORKSPACE` set alongside `BEX_WORKSPACE` (explicit `RENDER_*` precedence is documented bridge behavior; the hunt harness strips it for exactly that reason).
- Deploy lag means deployed revisions can precede local `main`.
