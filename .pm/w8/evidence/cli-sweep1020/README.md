# CLI sweep 1020 — service list filters (live, 2026-10-20)

Read-only probes against the workspace service list. No fixture created. No new bug filed.

- `type=web_service` → 200; a bogus type → coded 400 enumerating the five service types.
- The `name` filter is exact-match: `name=hello` returns `[]` even though `hello-go` exists (consistent with the exact-name lookups used for cleanup verification throughout this loop).
- `suspended=suspended` filter → 200; a malformed `createdBefore` date → 400.
