# CLI sweep 904 — owners pagination + w8/139 spot-check (live, 2026-10-20)

Read-only probes; nothing created. No new bug filed.

- w8/139 still reproduces at pin `3ede47d3c5a9`: REST KV create with `maxmemoryPolicy: "allkeys-lru"` → 400 (no fix yet).
- `GET /owners?limit=abc` → 400 `invalid query parameter "limit"`; `limit=1` + cursor → a different second page — the account's three workspaces page with the same keyset discipline as services (sweep 859).
