# `deploy_ended` and `build_ended` events have no Render `reason` object, so a crashed deploy never reports `failure.nonZeroExit`

Why: Render's service events give every ended deploy and build a structured `details.reason` (`buildDeployEndReason`). It has four members:

- `failure`: a `failureReason` with `nonZeroExit`, `earlyExit`, `oomKilled`, `evicted`/`evictionReason`, `timedOutSeconds`/`timedOutReason` or `unhealthy`;
- `buildFailed {id}`;
- `newDeploy {id}`, for a deploy that was superseded;
- `newBuild {id}`.

Tools read that object to tell a crash from an OOM, a timeout or a superseding deploy. bex sends only its own `failureReason` sentence and `fullDeployStatus` on failed deploys (w1/m138 extras). There is no `reason`, so any client written against Render's schema sees nothing. Cron-run events already carry Render's `reason.nonZeroExit` (ADR038, `events/rest.go:137`); deploys and builds do not.

Severity: **minor** (the human-readable cause is present; the machine-readable one is missing). Attribution: backend events REST/GraphQL/MCP (`lego/backend/internal/events`). Estimate: ~60m. Source: `/qa-find-bugs-cli` loop sweep 3093, 2026-11-02 07:24 UTC, pin `91ff7059c946`, workspace `bex-canary`.

## Evidence

Fresh Free web service `qa-20261102-3093-crash` (`srv-dbk3jd8rv49s73eq444g`, express-hello-world with start command `node nope.js`):

```text
07:23:32 → 07:24:03  deploy dep-dbk3jd8rv49s73eq4450: update_in_progress → update_failed   (w4/454 fix: ~40 s, not 15 min)
GET /v1/services/{id}/events?limit=3
deploy_ended  details keys: commitId, commitMessage, deployId, deployStatus="failed", failureReason="container exited shortly after start and is restarting repeatedly (last exit code 1) — …", finishedAt, fullDeployStatus="update_failed", startedAt, triggeredByUser
              details.reason → absent
build_ended   {"deployId": "dep-…", "status": "succeeded"}   (no reason)
```

Render's schema (pinned client `pkg/client/events/events_gen.go`): `DeployEndedEvent{deployId, deployStatus, reason BuildDeployEndReason, status}`, `BuildDeployEndReason{buildFailed, failure *FailureReason, newBuild, newDeploy}`, `FailureReason{earlyExit, evicted, evictionReason, nonZeroExit, oomKilled, timedOutReason, timedOutSeconds, unhealthy}`. On Render this deploy would carry `reason.failure.nonZeroExit: 1` (and `earlyExit: true`).

## Fix

- Map the reconciler's existing deploy-failure facts into a Render `reason` object on `deploy_ended`, on REST, GraphQL and MCP alike:
  - crash-loop exit code → `failure.nonZeroExit` + `earlyExit`;
  - OOM → `oomKilled`;
  - health-check timeout → `timedOutSeconds`/`unhealthy`;
  - eviction → `evicted`;
  - superseded deploy → `newDeploy.id`;
  - build failure → `buildFailed.id`.
- Do the same for `build_ended`.
- Reuse the cron-run reason vocabulary (`store.EventReason*`) rather than parsing `failureReason` text.
- Keep `failureReason`/`fullDeployStatus` as the recorded extras.

## Acceptance

A start-command crash, a superseded deploy and a failed build each read back `details.reason` in Render's shape. On a crash that is `failure.nonZeroExit` equal to the exit code; on a superseded deploy, `newDeploy.id` names the newer deploy; on a failed build, `buildFailed.id`. A succeeded deploy carries an empty reason or none, as Render does.

Dedupe: searched `.pm`/`docs` for `buildFailed`, `newDeploy`, `nonZeroExit`, `deploy_ended … reason`. Only cron runs (ADR038) and the w1/m138 text extras exist; w4/131, 161 and 089 cover status and text, not the structured reason.

Cleanup: fixture deleted by ID (`GET` 404).
