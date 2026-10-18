# Hosted environment-group acceptance — 2026-10-18

## Current wire behavior

The full owned-group check ran against API image `sha256:149e5133bde2b7fb3ec3fe0cfc2db8f6256792c4b6911369676a033dde890eae` with two ready replicas. This is the initial row-backed m114 image, before the recovery corrections and migrator deletion; it proves the shared wire behavior, not final-image deployment or pre-cutover preservation.

[Counts and timings](current-wire-counts.json) record successful create, rename, variable/file reveals, clone, sparse save, unchanged-key preservation, stale-revision 409 and concurrent create/rename uniqueness (one success, one `ENV_GROUP_NAME_EXISTS` 409). REST, GraphQL and MCP return equal metadata with no secret values. In workspace bex, the group list took 0.287s, all three project-environment reads took 0.240–0.270s, and native CLI services completed in 0.847/0.818s, both exit 0.

The dashboard listed the owned canary group with two variables and one file, displayed its owner and timestamps, and saved one synthetic variable edit while preserving the other variable and file. The ready page showed the new timestamp, a successful save notification, masked untouched contents and the synthetic edited value on reveal. The group had no linked services. [Screenshot](ui-save.jpg).

[Cleanup](ui-cleanup-counts.json) independently verifies the synthetic saved value, unchanged variable/file content, DELETE 204, subsequent GET 404, listing absence and unchanged original canary/target metadata. All three groups created by this check are absent. Other sessions' resources were not deleted.

## Linked service and filters

[The linked-fixture matrix](linked-counts.json) passed 48 distinct checks on the same initial image: thirteen positive/negative cases across all seven Render list-filter keys; environment assignment, clearing and restoration; service linking/unlinking; replace-all variable updates, per-key writes/deletes and file upserts/deletes. REST, GraphQL and MCP writers/readers agree, including ordered service membership, group service links, environment scope and masked metadata.

The service Environment page loaded its ready state with the owned group's variable/file metadata. Unlink moved it from Linked (1) to Available (1); Link restored Linked (1), Available (0) and a successful notification. [Screenshot](service-linked.jpg). [Post-UI checks and cleanup](linked-ui-cleanup-counts.json) verify cross-surface membership agreement, DELETE/GET404 for the exact owned group, Free service, environment and project, and unchanged original metadata. [Read-only SQL](post-fixture-sql-counts.json) independently confirms exactly the original two groups and owners after cleanup. [Combined sanitized report](full-safe-report.json).

Render's [list API](https://api-docs.render.com/reference/list-env-groups) and [environment-group documentation](https://render.com/docs/configure-environment-variables#environment-groups) were checked on 2026-10-18 for filter keys, creation/editing, files, environment scoping and linking. The deliberate store/cutover changes are recorded in ADR013 and ADR018. No additional drift was found in these checks; this is comparison with the published contract, not a fresh Render-account execution.

## Remaining checks

A small fresh create/save/delete check must run on the deployed final image, after the snapshot comparison so test rows cannot interfere. Exact customer-data preservation remains a separate comparison against the encrypted 07:26 pre-cutover snapshot. [Protected manual run39111039281](https://github.com/bex-co/bex/actions/runs/39111039281), dispatched at main23150581, waits for the existing GitHub `production-restore` required reviewer. No preservation job has executed yet.
