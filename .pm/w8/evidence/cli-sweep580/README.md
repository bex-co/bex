# CLI sweep 580 — `ea objects --local`: full client-side flight in an isolated temp dir

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. No production fixture: the whole matrix ran with `--local` inside a `mktemp` directory (removed at close). This closes the "`ea objects --local` client-side paths" Unrun note from sweep 509. The cloud verbs remain the ledger's documented `[-]` non-goal (`/v1/objects` → 404).

- **Region gate precedes local mode:** every verb with `--local` but no `--region` fails client-side with the upstream string `--region is required (or set RENDER_REGION environment variable)`, exit 1 — the recorded sweep-458 contract, unchanged. With `--region oregon --local` all verbs work without touching the server.
- **Put/get round-trip exact:** nested keys, empty files (0 bytes), and binary content all round-trip byte-identically (`cmp` clean). Overwriting a key replaces content and size. Put echoes the on-disk path; `-o json` returns `{key, region, sizeBytes, localPath}`.
- **On-disk layout is traversal-safe:** objects live under `.render/objects/<region>/<workspace-id>/` with the key URL-encoded as a single filename (`qa580%2Fsub%2Fb.txt`). Keys `../escape580.txt` and `/tmp/qa580-abs.txt` were accepted as opaque keys but encoded (`..%2F…`, `%2Ftmp%2F…`) — no file escaped the store directory, and both read back by the same key.
- **List shapes:** empty store prints `[]` in JSON and a header-only table in text; entries carry `key`, `sizeBytes`, `lastModified`; `--limit=2` truncates as documented. Keys with spaces work.
- **Delete dialects:** deleting an existing key prints `Deleted <key>`; deleting a missing or already-deleted key exits 1 with `failed to delete …: object not found`. Multi-key delete removes each in order. In non-TTY runs the confirmation does not block (deleted with stdin closed or piped) — upstream client behavior, identical in the pinned Render CLI, not a bex divergence.
- **Errors are named and client-side:** missing `--file` → cobra required-flag error; nonexistent source file → `failed to stat file: stat …: no such file or directory`; missing object on get → `object not found: <key>`, all exit 1.

## Unrun

- `--local` store shared across regions (same key under two `--region` values).
- Very large object streaming/chunking behavior in local mode.
- Cloud-mode verbs beyond the already-graded 404 non-goal.
