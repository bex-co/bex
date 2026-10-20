# CLI sweep 1217 — suspended list filter tracks live state (live, 2026-10-20)

One disposable free web service created, suspended, and deleted (204, verified 404). No new bug filed.

- After suspend, the fixture appears under `?suspended=suspended` and not under `?suspended=not_suspended`: list filters read the same live state the verbs write. A bogus suspended value → 400 (the enum is validated at the query layer, per the filter's contract in [rest.go](/Volumes/nvme4tbfish/projects/bex8/lego/backend/internal/apps/rest.go:695)).
