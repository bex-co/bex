# CLI sweep 1186 — `notifyOnFail` speaks Render's exact enum (live, 2026-10-20)

Two disposable free web services created and deleted (204, verified 404). No new bug filed.

- The valid values are `default | notify | ignore` (per the field's contract in [rest.go](/Volumes/nvme4tbfish/projects/bex8/lego/backend/internal/apps/rest.go:112)): each PATCHes 200 and reads back applied.
- Plausible-but-wrong guesses (`all`, `failed_only`), a bogus string, and boolean `true` all → 400. The enum admits only Render's vocabulary, with no coercion.
