# CLI sweep 596 — w8/133 + w8/134 re-verification on the new production pin: both still open

**Verdict: no new filing; both notes confirmed still live.** Same isolated QA session, `bex-canary`, CLI rebuilt at `1a49cb806` (production pin `ebfefea261db`), 2026-10-19. Failed POSTs only — a blueprint list check confirms nothing was created by these probes.

- **w8/133 (`POST /blueprints/generate`):** bodies `not-json`, empty, and `[1,2]` still return the bare `500 {"error":"internal error","id":"internal_error"}`. A wrong-type-but-object body (`{"yaml": 42}`) reaches validation and gets the named `400 … select at least one resource…`, so the gap is exactly the filed one: unparseable/empty/non-object payloads.
- **w8/134 (`POST /blueprints`):** with `branch` supplied, all three fetch-failure classes still return the same bare 500: nonexistent repo, nonexistent branch on a real repo, and a traversal path (`../escape.yaml`). (Omitting `branch` gets the named `400 repo and branch are required` — that part of the boundary is honest.)
- No source fix has landed for either: both handlers are unchanged at the pulled HEAD. The notes stand as filed; no duplicate created.

## Unrun

- Re-check after the next production pin moves past a fix for either note.
