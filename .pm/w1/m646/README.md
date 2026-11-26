# w1 · m646 — A rollback's confirm dialog lists what the rollback changes, as Render's does

**Worker:** worker1 **Goal:** before a rollback, the dashboard compares the live deploy's configuration with the target deploy's, as Render's `rollbackComparison` does. It names each changed command, health check path, instance count, plan, env var key, secret file and env group, and never shows a value. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's rollback comparison](t001.md) | 30m | — |
| t002 | [GraphQL read: the live and target releases' configuration compared](t002.md) | 2h | t001 |
| t003 | [The confirm dialog in Render's words](t003.md) | 1h45m | t002 |
| t004 | [Cron jobs, image-backed services and static sites](t004.md) | 45m | t003 |
| t005 | [Production walk on a disposable service](t005.md) | 30m | t004 |
| t006 | [Render parity](t006.md) | 20m | t005 |
| t007 | [Simplify](t007.md) | 20m | t006 |
| t008 | [Test coverage](t008.md) | 45m | t006 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

1. On production, take a disposable web service in `bex-canary` whose env var keys, one secret file and health check path changed after an earlier deploy. Opening Rollback on that deploy lists each change in Render's sentences, and no value appears in the dialog or in the GraphQL response.
2. Rollback on a deploy with nothing changed since says "No configuration changes since this deploy."
3. A target whose release record was reclaimed or predates snapshots shows the explicit unavailable state rather than an empty diff. A live deploy without a record shows Render's "Unable to load configuration for the current live deploy…" warning.
4. The loading frame matches the ready dialog at 1440 and 390 px.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4` (2026-11-25), item 3. On the user's request it was checked in a headed browser the same day and filed to w1.
  - **Render's read** is in Render's public dashboard bundle (`server-*.js`):
    - the query is `rollbackComparison(serverId, deployId, ownerID) { currentLiveDeploy { serviceConfigSnapshot } deploy(id) { serviceConfigSnapshot } serverPlans }`;
    - the `ServiceConfigSnapshot` fields are `dockerCommand`, `runCommand`, `preDeployCommand`, `healthCheckPath`, `instances`, `plan`, `envGroups`, `deletedEnvGroupCount` and `envVarChanges`;
    - `envVarChanges` holds `createdVarKeys`, `deletedVarKeys`, `updatedVarKeys`, `createdFileKeys`, `deletedFileKeys` and `updatedFileKeys`.
  - **Render's dialog** is in `DeployContainer-*.js`:
    - title and lead: "Confirm Rollback" and "Rolling back reverts certain configuration changes.";
    - rows: Start Command, Pre-Deploy Command, Docker Command, Health Check Path, Environment Variables, Secret Files;
    - secret files: "Secret file … was added after this deploy. The rollback will ignore it.", "…will restore the file." and "…will revert to the original contents.";
    - env groups: "The rollback will unlink environment group …";
    - plan: "Your current plan has lower CPU and memory allocations than the plan originally used for this deploy…";
    - empty and error states: "No configuration changes since this deploy." and "Unable to load configuration for the current live deploy…";
    - button: "Rollback to this deploy".
  - **No feature flag:** neither chunk imports Render's `featureflags` hooks, so every Render workspace has this dialog.
  - **bex today:** checked on production on 2026-11-25, on bex's own `metrics-grafana`; the dialog was opened and cancelled. It says "Roll back to this deploy? The service will run this deploy's image (built from …) and retained configuration. Saved settings stay unchanged and apply with the next standard deploy. Auto-deploy will be turned off. Older deploys without retained configuration restore only the image." It shows no comparison (`dashboard/src/features/deploys/components/deploy-actions.tsx`, `services.eventsRollbackConfirm*` in `dashboard/src/features/services/locales/en.ts`).
  - **The data exists:** [ADR004 § Per-release configuration snapshots (w1/m152)](../../../docs/ADR004-app-deployment.md#per-release-configuration-snapshots-w1m152) keeps a record (`ReleaseRecordName`, `<app>-podtemplate-rN`) and per-source env and file copies for 20 generations. w4/m145's `undeployedChanges` already compares saved sources with the active release's record.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). This extends ADR018's Rollback row (`docs/ADR018/deploys.md`).
- **Expected outcome:** someone rolling back sees what reverts before confirming, as on Render, instead of discovering it from the next deploy's behavior.
- **Why now:** rollback restores retained configuration since w5/m107, so it does change configuration. The dialog is the only place to say what changes, and today it does not.
- **Render parity included:** GraphQL and dashboard change. Render has no REST or MCP comparison, and the parity task confirms that REST and MCP rollback are unchanged.
