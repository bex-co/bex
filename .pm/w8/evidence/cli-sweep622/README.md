# CLI sweep 622 — `logs` filter dialects round-out: combined types, level/direction enums, text search

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Read-only against the baseline service; no fixture, no cleanup. Continues sweep 621.

- **Combined types:** `--type app,request` streams the union (app lines shown); any combination containing `build` (`request,build`) gets the server's named `400 log type "build" must be requested on its own`.
- **Closed client enums:** `--level bogus` → `must be one of "debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"`; `--direction bogus` → `must be one of "backward", "forward"` — both client-side, exit 1, no request sent.
- **Text search:** `--text GET` matches the app's request lines; `--text qa622-definitely-absent` exits 0 with no output — an empty result, visibly distinct from an error.
- **Limit dialects:** `--limit 0` silently uses the client default (the upstream `--limit 0` lossy substitution already recorded as not-a-bug in w8/132); `--limit 100000` reaches the server and gets `400 limit must be an integer between 1 and 100`.

## Unrun

- `--start`/`--end` RFC3339 dialect edge cases through the CLI (REST arms covered in earlier eras).
