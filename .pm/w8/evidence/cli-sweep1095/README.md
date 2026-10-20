# CLI sweep 1095 — dryRun runs the same validation (live, 2026-10-20)

One real disposable service created with `dryRun=false` and deleted (204). No new bug filed.

- `dryRun=true` with a bogus plan → 400: the dry path runs the real validation stack, not a shallow echo. `dryRun=false` → 201 (a real create, cleaned up). The invalid dry-run left nothing behind.
