# Runbook: env-group OpenBao workspace-prefixed path migration

**Status:** written 2026-08-19 (`w2/m80`). **Applies to:** every environment group still fully resident at the shared legacy OpenBao tenant (`default`) — created before this milestone, or created since but never visited by an earlier `MigratePaths` pass.

Ordinary traffic already converges a group toward its workspace-prefixed home: every write (`SetEnvGroupVars`, `RenameEnvGroup`, `LinkService`, …) lands only under the group's own workspace tenant, and a read falls back to the legacy tenant only when the workspace copy is still empty. This runbook is for an operator who wants to finish the move explicitly — draining the legacy tenant on a schedule — rather than waiting for every group to be touched by ordinary use.

`MigratePaths` (`lego/backend/internal/envgroups/migrate.go`) is invoked once at `bex-api` startup via an environment variable, exactly like the existing `BEX_ENV_GROUP_NAME_CLAIM_AUDIT` backfill it sits next to in `cmd/api/main.go`. There is no separate CLI binary.

## STOP — requires explicit operator authorization

**Do not set `BEX_ENV_GROUP_PATH_MIGRATION=apply` against a production deployment without an explicit operator (user) authorization for that change window.** `dry-run` is read-only and may be run at any time. `apply` copies every unmigrated group's meta/env/files/revision to its workspace tenant and then rewrites the legacy tenant's copy to a locator, deleting the legacy env/files/revision — a mutation of every OpenBao path an unmigrated group occupies.

The gate is this paragraph. There is no flag that bypasses it. The env var is unset by default (byte-identical to pre-`w2/m80` behavior aside from the ordinary lazy convergence above).

## Before you start

- [ ] You have a recent OpenBao backup/snapshot per [docs/ADR031-platform-data-backup.md](../ADR031-platform-data-backup.md) — `apply` is not itself destructive of content (legacy env/files/revision are deleted only after the workspace-tenant copy is verified), but a backup is still cheap insurance before any bulk write.
- [ ] `BEX_OPENBAO_URL` (and Kubernetes-auth wiring) resolves from wherever `bex-api` will run the migration — same requirement as the name-claim audit.
- [ ] You have rehearsed both modes against a scratch/dev store, not production, first.

## Dry-run (read-only)

Set the variable for one `bex-api` process invocation and inspect the startup log line; it never writes:

```
BEX_ENV_GROUP_PATH_MIGRATION=dry-run <bex-api invocation>
```

Look for:

```
bex-api: environment-group path migration mode=dry-run scanned=<N> migrated=<N> alreadyMigrated=<N> skippedNoWorkspace=<N> failed=[]
```

- `scanned` — legacy-tenant groups examined this pass (a permanent legacy locator from an earlier `apply` is recognized and excluded before this count, so a fully-migrated store reports `scanned=0`).
- `migrated` — groups that _would_ move to their own workspace tenant.
- `alreadyMigrated` — a group whose workspace tenant already holds full content (a prior `apply` copied it but did not finish tombstoning the legacy entry, e.g. a crash mid-run); `apply` would only finish the tombstone, never re-copy.
- `skippedNoWorkspace` — a group with no resolvable owning workspace (store-off era, or one core.DefaultTenant-attributed group where the "workspace" already **is** the legacy tenant); left alone, exactly as `readMeta`'s own lazy attribution would leave it.
- `failed` — per-id `"<gid>: <error>"` entries; `apply` stops migrating that one id and continues with the rest.

**Rollback:** none. This mode only reads.

## Apply

**STOP — requires explicit operator authorization before touching live tenant data** (see above).

```
BEX_ENV_GROUP_PATH_MIGRATION=apply <bex-api invocation>
```

Per unmigrated group, the pass:

1. copies meta, env, files, and the CAS revision row to the group's own workspace tenant (`tenants/<workspace>/env-groups/<id>/...`);
2. re-reads the workspace-tenant meta and verifies name + workspace match the legacy source **before** touching the legacy copy — a mismatch aborts that one group (recorded in `failed`) and leaves the legacy tenant untouched;
3. replaces the legacy tenant's meta with a thin locator (`{"workspace": "<id>", "locator": "1", "tombstoned": "1"}`) — the same shape `writeMetaLocator` publishes for every group created directly under a workspace tenant, so a bare-gid lookup (`GetEnvGroup`, the SSH/Blueprint seams) keeps resolving it;
4. deletes the legacy tenant's env, files, and revision copies (now superseded).

Idempotent: a re-run finds only locators for every already-migrated group (excluded before `scanned` is even incremented) and does no further work. A group whose workspace tenant already holds a full copy (a partial prior run) has its tombstone finished without any content re-copy.

**Verify**

- The startup log line's `migrated` + `alreadyMigrated` should sum to the prior dry-run's `scanned` for the affected ids (allowing for ordinary lazy convergence between the two runs).
- Spot-check one migrated group end to end: `GetEnvGroup`/`GetEnvGroupVar` through the normal API surface for a caller in that group's workspace should return unchanged values.
- `failed` should be empty; investigate any entry before re-running (its legacy copy is left exactly as it was — safe to retry after fixing the underlying store issue).

**Rollback:** there is no "undo" verb — the workspace tenant's copy is the group's new source of truth. Because the legacy env/files/revision are deleted only after verification against the workspace-tenant copy succeeds, the workspace tenant always holds a complete, valid copy before anything legacy is removed; a regression is a bug in the _reading_ code path, not a data-loss risk from this migration. If a specific group's migrated copy is ever suspect, restore both the legacy and workspace-tenant paths for that one id from an OpenBao backup and re-run `apply` for it.

## Per-mode summary

| Mode | Mutates tenant data? | Authorization | Rollback |
| --- | --- | --- | --- |
| `dry-run` | no | not required | n/a |
| `apply` | yes (copy + tombstone + delete legacy content) | **STOP — explicit operator authorization** | restore from OpenBao backup for any suspect group; re-run `apply` |
