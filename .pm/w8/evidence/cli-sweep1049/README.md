# CLI sweep 1049 — CLI workspace selection scopes lists (live, 2026-10-20)

Read-only CLI probes; nothing created or mutated in either workspace. No new bug filed.

- With `BEX_WORKSPACE` pointed at the user's other workspace, `bex services -o json` lists that workspace's services (8, including its older QA fixtures) — CLI list scoping follows the selected workspace exactly as the REST list does (sweep 1024/1025), while the isolated config's default remains `bex-canary`.
