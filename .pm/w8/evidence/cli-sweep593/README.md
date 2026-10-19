# CLI sweep 593 — deploy rollback journey: v1 → v2 → rollback to v1, with dialects

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixture `qa593-web` (`srv-dbb980ibpdss73ci0ml0`, free `gcr.io/google-samples/hello-app`), deleted at close: REST 404, URL 404, zero `qa593` names. Rollback has no pinned-CLI verb, so the journey is the REST boundary (`POST /v1/services/{id}/rollback`) with the CLI for create/deploy/list.

- **Journey:** v1 (`hello-app:1.0`) live serving `Version: 1.0.0` → `deploys create --image …:2.0` → `Version: 2.0.0` → `POST rollback {deployId: <v1 deploy>}` → `201` with a fresh deploy row (`trigger: "rollback"`, `rollbackOf: <v1 deploy id>`, image ref restored to `1.0`) → public URL back to `Version: 1.0.0` in seconds. The v2 row settled to `deactivated`.
- **Dialects:** nonexistent deploy → `404 deploy not found`; missing `deployId` → `400 invalid request body at /deployId`; synthetic service → `404 not found`; rollback to the currently live deploy → `409 conflict: deploy "…" is already live — nothing to roll back to`. Rolling back to a `deactivated` (superseded) deploy is accepted — that is the normal case above.

## Unrun

- Rollback to a canceled or failed deploy (refused or accepted?).
- Rollback on a Git-backed service (commit restoration, not just image ref).
- CLI-surface rollback, if a future pin grows the verb.
