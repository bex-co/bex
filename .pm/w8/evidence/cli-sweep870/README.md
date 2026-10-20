# CLI sweep 870 — static-site `publishPath` PATCH + start-command inapplicability (live, 2026-10-20)

Disposable static site created via REST (`type: static_site` + `serviceDetails.publishPath`, the ADR029 spelling — no `runtime` field), patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"serviceDetails":{"publishPath":"dist"}}` → 200, read back `dist`.
- `envSpecificDetails.startCommand` on a static site → named 400 `start command is not applicable to a static_site` (the `settings_check.go` type gate refusing, not silently storing, a command a file-serving type can never run).
