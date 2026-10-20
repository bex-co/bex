# CLI sweep 711 — active-workspace commands (live, 2026-10-19)

Session state verified unchanged after the probes.

- `bex workspace current -o json` → the canary workspace (`tea-daif693dqjvc73e7as3g`, `bex-canary`) the harness pins via env.
- `bex workspace set tea-0000000000000000000000` → `403 Forbidden` (server membership check; the CLI's rendering of it is a generic `unexpected response: 403 Forbidden`, upstream phrasing). `workspace current` and `whoami` afterwards confirm the active workspace and identity did not move.

No bug filed.
