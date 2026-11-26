# Render's rollback confirmation: what the rollback changes

Captured 2026-11-26 for w1/m646 from Render's public dashboard bundle (read without sign-in), and checked the same day in a headed browser signed in to a Render Pro workspace: a rollback confirmation was opened on a web service and cancelled. Nothing that acts was clicked. Names, ids, commands and env var keys from that workspace are not recorded here.

**Evidence.** Render's dashboard bundle: the `server-*.js` chunk holds the query (`rollbackComparison`, `fragment snapshotFields on ServiceConfigSnapshot`), `DeployContainer-*.js` the page section (`Confirm Rollback`), `Diff-*.js` the diff rows, and `event-timeline-utils-*.js` the env-group set difference. The hashes change with every Render deploy, so find the chunks by name from `https://dashboard.render.com/`'s `assets/index-*.js` ([metrics-page.md § Status-code label](metrics-page.md#status-code-label-w3080-2026-11-20) describes the method). Neither chunk imports Render's `featureflags` hooks: every Render workspace has this page.

## Not a modal: a page section on the deploy's `/rollback` route

- The deploy list's and the deploy page's **Rollback** control is a link to `/<type>/<service id>/deploys/<deploy id>/rollback` (`urls-*.js`: the deploy path plus `rollback`).
- That route is the deploy page itself. The deploy header stays, without its Rollback control, and the section below it replaces the log viewer.
- A deploy that cannot be rolled back to (`rollbackSupportStatus` other than `ROLLBACK_SUPPORTED`) redirects to the deploy page, replacing the history entry.
- The section is a block titled **Confirm Rollback**, lead "Rolling back reverts certain configuration changes." plus an inline link "Learn more." to `/docs/rollbacks`. Then the comparison, then the notices, then the buttons **Rollback to this deploy** and **Cancel** (secondary). Cancel goes back to the deploy page. While the mutation runs, both are disabled and the first shows a spinner. A failure shows an inline error (`Error:` and the message) under the buttons.
- Success opens the new rollback deploy's page.
- Render's cron deploy page (`CronJobDeployContainer-*.js`) has no rollback route.

## The read

```graphql
query rollbackComparison(
  $serverId: String!
  $deployId: String!
  $ownerID: String
) {
  currentLiveDeploy(serverId: $serverId) {
    id
    serviceConfigSnapshot {
      ...snapshotFields
    }
  }
  deploy(id: $deployId) {
    id
    serviceConfigSnapshot {
      ...snapshotFields
    }
  }
  serverPlans(ownerID: $ownerID) {
    ...planFields
  }
}
fragment snapshotFields on ServiceConfigSnapshot {
  deletedEnvGroupCount
  dockerCommand
  envGroups {
    id
    name
  }
  envVarChanges {
    createdVarKeys
    deletedVarKeys
    updatedVarKeys
    createdFileKeys
    deletedFileKeys
    updatedFileKeys
  }
  healthCheckPath
  instances
  plan {
    ...planFields
  }
  runCommand
  preDeployCommand
}
```

The page reads `current` (the live deploy's snapshot) and `old` (the target deploy's). Only `old.envVarChanges` is read: Render's server reports the env var and secret-file changes relative to now on the target's snapshot.

## The comparison, row by row

Each row appears only when its condition holds, in this order. A scalar row is a diff whose `-` line is the live value and whose `+` line is the target's, each `none` when empty (`Diff-*.js`: `-` is `<del>` labelled "Removed", `+` is `<ins>` labelled "Added").

| Row | Condition | Shows |
| --- | --- | --- |
| **Start Command** | `current.runCommand !== old.runCommand` | `- <live>` / `+ <target>` |
| **Pre-Deploy Command** | `preDeployCommand` differs | as above |
| **Docker Command** | `dockerCommand` differs | as above |
| **Health Check Path** | `healthCheckPath` differs | as above |
| **Instances** | `instances` differs | as above |
| **Environment Variables** | any of `createdVarKeys`, `deletedVarKeys`, `updatedVarKeys` is non-empty | a KEY / VALUE table (column headers `Key`, `Value`), one row per key sorted by key: a created key on the `+` line, a deleted key on the `-` line, an updated key on both. Every value is `••••••••••••`. |
| **Environment Group** | a group was added or removed since, or `old.deletedEnvGroupCount !== 0`, or `old.envGroups` is non-empty | the sentences below |
| **Secret Files** | any file key list is non-empty | one sentence per file, below |
| **Plan** | the service's current plan's price (looked up in `serverPlans` by name) is lower than `old.plan`'s price | a warning, below |

Commands, the health check path and the instance count show their values. Env var values and secret file contents are never shown.

**Orientation of `envVarChanges`.** Render's server is not observable, so the orientation comes from the sentences: a created file "was added after this deploy", a deleted one "was deleted after this deploy". The same object carries the env var lists, so a created env var is one added after this deploy, and the env var table marks it `+`, while the scalar rows put the live value on `-`. bex follows Render's code as written.

**Secret Files**, one sentence per key:

- created: "Secret file `<name>` was added after this deploy. The rollback will ignore it."
- deleted: "Secret file `<name>` was deleted after this deploy. The rollback will restore the file."
- updated: "Secret file `<name>` was changed after this deploy. The rollback will revert to the original contents."

**Environment Group.** `added` is the live snapshot's groups minus the target's (by id), `removed` the target's minus the live one's. A web service, private service or background worker:

- added: "The rollback will unlink environment group **`<name>`** which was added to the service after this deploy."
- removed: "The rollback will link environment group **`<name>`** which was removed from the service after this deploy."
- `deletedEnvGroupCount` 1: "This deploy was linked to **1** environment group that has since been deleted. It cannot be restored."
- `deletedEnvGroupCount` N > 1: "This deploy was linked to **N** environment groups that have since been deleted. They cannot be restored."
- the target has any group: "The rollback will use current keys and values for environment groups linked to this deploy."

A static site has its own wording:

- added: "The rollback will not use environment group **`<name>`** which was added to the site after this build."
- removed: "The rollback will use environment group **`<name>`** which was removed from the site after this build."
- 1 deleted: "This build used **1** environment group that has since been deleted. The rollback will use its original values."
- N deleted: "This build used **N** environment groups that have since been deleted. The rollback will use their original values."
- the target has any group: "The rollback will use the original values for environment groups used for this build."

**Plan**: "Your current plan has lower CPU and memory allocations than the plan originally used for this deploy. You may need to switch to a different plan for the rollback to succeed."

## States

- **Loading** (no data yet): a centered spinner in a frame of `min-h-[9.25rem]`.
- **Unavailable**: on a query error, and whenever **either** snapshot (live or target) is missing: "Unable to load configuration for the current live deploy. Rolling back to this deploy could cause unanticipated configuration changes." Render has one sentence for both cases. It has no separate state for a target whose configuration is gone.
- **Empty** (no row's condition holds): "No configuration changes since this deploy."

## Notices below the block

- A static site: an informative notice "Changes made to **redirects, rewrites, and headers** will not be rolled back."
- Auto-deploy on: a warning titled "Auto-Deploy", "will be disabled for this service to prevent accidental deploys."

## What Render documents

[render.com/docs/rollbacks](https://render.com/docs/rollbacks) "What's rolled back?", read 2026-11-26: start command, health check path, Docker command, build artifact, instance count and environment variables match the target deploy. A registry-hosted Docker image and environment groups match it partly ("Rolling back does _not_ modify any values in an environment group", "rolling back might modify _which_ environment groups are applied to the service", "If the target deploy included an environment group that has since been deleted, the rollback proceeds without it"). Disks, compute plan, custom domains and static site redirects, rewrites and headers use the current configuration. The page says nothing about secret files or about what the confirmation lists ("On the confirmation page, click _Rollback to this deploy_").

## bex (w1/m646)

- GraphQL `rollbackComparison(serviceId, deployId)` answers `currentLiveDeploy` and `deploy`, each `{ id serviceConfigSnapshot }`, with Render's snapshot fields, and `currentPlanLower` in place of `serverPlans`: bex compares the plans server-side, by their place in the compute ladder (`types/tiers`, cheapest first). A snapshot is null when its release record is missing or predates record version 1. The dashboard then shows Render's unavailable sentence.
- A static site's snapshot (w1/m660) is the environment its build read: the env var keys, secret file names and env groups of the record the release kept when it was adopted, with no commands, health check path, instance count or plan. A static Rollback or Restart serves the files of the release whose publication it copied, so its snapshot is that build's, found through the `StaticPrefix` of the release selection its record's adopted spec holds. The dashboard words the Environment Group row for a static site as Render's does, from the deploy page's `/static` route.
- The live snapshot is the serving release's record. The target's env var and secret-file key lists compare the service's own env and file copies of the two releases, values compared server-side. A key or file name leaves the server, never a value. Env groups come from each record's group refs; a group no longer in the workspace counts as deleted.
- The read needs the deploy's read relation (`can_view_activity`) and, because it names env var keys and secret files, the same access the service's Environment page reads them with (`can_view_sensitive`, and Admin in a protected environment). Every role holding `can_view_sensitive` can also roll back. A role that can roll back but not read the environment gets the unavailable sentence and can still confirm.
- The dashboard serves the page at `/services/<id>/deploys/<deploy id>/rollback` and `/static/<id>/deploys/<deploy id>/rollback`. Render's `/web/…/rollback` links reach it through the alias routes.
