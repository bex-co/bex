# CLI sweep 1236 — project/environment placement through the CLI and TUI (live, 2026-10-20)

Owned project `qa-20261020-53921b-proj` (`prj-dbbgq519a20c7385rb30`) with inline environments `production`/`staging`, created over REST because the CLI has no project create. One free web service was placed with `bex services create … --environment-id <staging>`. Both were deleted (service 204 → 404, project → 404 with both environments gone). No new bug filed.

- `bex projects -o json` and `bex environments <prj> -o json` return the project and both environments (Render's `serviceIds`/`databasesIds`/`redisIds`/`envGroupIds`/`protectedStatus`/`ipAllowList` shape; the environment default list carries Bex's `0.0.0.0/0` + `::/0` pair).
- The create response and later GET carry `environmentId` = staging, and staging's `serviceIds` lists the service. `bex services -e <staging>` lists exactly it, `-e <production>` lists none, and `-e <both>` lists it. An unknown environment id on create → exit 1 `404 (NOT_FOUND): resolving environment: environment not found`.
- TUI (PTY): Projects → project → environment list (two rows) → staging shows the service with the Project/Environment columns filled.
- Not findings: an empty result prints `null` with `-o json` while REST answers `[]` (the pinned client marshals a nil slice; same class as sweep 1212). A malformed `-e` value lists nothing rather than refusing; Render's validation for it is unverified.
