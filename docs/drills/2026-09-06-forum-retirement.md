# Forum legacy-image retirement — 2026-09-06

Status: completed. Three database restores verified; clean image preserved; five inactive ReplicaSets and the exact legacy credential-bearing artifact retired. Existing missing upload files were investigated in w7/042 (closed 2026-09-06); restore remains deferred until `nvme4tbfish` or another matching archive is available — see `.pm/FUTURE-MAYBE.md` and `w7/done/042.md`.

## Scope and initial state

The user requested a safe migration/backup before closing w7/023 and w7/038. The forum already runs the clean gen-112 image on one Ready replica in workspace `tea-d98210cbbpdc73dcrkvg`. Five historical ReplicaSets reference gen-54 and have zero desired/current replicas. They hold deployment history, not database storage. No traffic cutover or database relocation is needed to retire them.

The single forum process serves beancount, tianpan and blockeden using three managed PostgreSQL databases. Existing managed databases and their PVCs remain in the tenant namespace. Current App and Deployment configurations were captured privately. The three origin `/forum/site.json` requests returned HTTP 200 with their respective Host headers.

The retirement target is exactly the legacy repository `tea-d98210cbbpdc73dcrkvg-beancount-forum`, gen-54 manifest `sha256:f9f1e8de18d77743c78d73bafbe05faa73e64647f5f40681f92c362f2248c029`, and its credential-bearing layer `sha256:55be548f91cf6a1ccebb07de8203d7cafa5074275b41c438e2dcefe8563e991b`. Earlier verification established that the two embedded passwords are invalid; current retained images have placeholder-only templates. This session does not restore those passwords or retain the unsafe image as the rollback target.

## Backup method and safety correction

Each logical export uses the application role and a read-only repeatable-read snapshot shared by the table-count observer and `pg_dump --snapshot`. This gives an exact comparison against the restored dump while production remains writable; see PostgreSQL's [snapshot synchronization](https://www.postgresql.org/docs/18/functions-admin.html#FUNCTIONS-SNAPSHOT-SYNCHRONIZATION) and [pg_dump snapshot option](https://www.postgresql.org/docs/18/app-pgdump.html). Exports omit ownership/ACL commands; restore recreates application ownership explicitly and verifies every public table's owner.

The first attempt ran `pg_dump` inside the basic-256mb database container while its fresh Barman backup was also running. It exited 137, and the database container recorded `OOMKilled` at `2026-09-07T02:46:19Z`. The associated Barman attempt failed. The database recovered Ready and all three origin site checks returned 200 afterwards. The incomplete dump is not a recovery point. The operation was changed to a separate local client container, forwarding PostgreSQL connections instead of running the dump client inside the production container. The first database's Barman backup was rerun alone and completed. No resource-limit change or serving-image rollout was made.

Restoration uses a separate local PostgreSQL 18.4 container with networking disabled and no published ports, matching the CNPG image digest `sha256:42708a75345b7a48fdd9257b071830783a97fd228529196b6313187a7198e185`. Required extensions are created before the dump is restored under the application role. Full schema/data restore, all public-table row counts and table ownership are checked. No email sender, background worker or forum application is started against the restored data.

## Upload-file limitation discovered before retirement

The live forum Pod has no volumes. Both `/var/www/discourse/public/uploads` and `/var/discourse/shared/uploads` contain zero files. There are 29 custom upload references in the databases: 11 beancount, 8 tianpan, 10 blockeden. Their direct origin requests returned 25 HTTP 500 and 4 HTTP 404 responses; public download attempts returned HTTP 403. Built-in image assets were excluded from these counts. Scanning all 34 gen-54 image layers found no files under either upload path, so retaining that image cannot recover them.

These failures were recorded before any old ReplicaSet/image deletion or application rollout. A database backup preserves the references, not the missing file bytes. Follow-up investigation (`w7/done/042.md`) confirmed historical routing to `/Volumes/nvme4tbfish/discourse_docker/data/…` and prior Render upload disks; the volume is not currently attached, so restore + durable `spec.disk` stays in `.pm/FUTURE-MAYBE.md`. This operation must not claim a complete historical upload backup.

## Recovery materials

Private artifacts live outside the repository in `/Users/tianpan/.local/share/bex/backups/forum-retirement-20260906/`, with directory mode 0700 and files created under umask 077. They include valid logical dumps and checksums, same-snapshot table counts, restore results, current configuration, upload-failure inventory and a clean gen-112 OCI image copy. Do not commit the database contents, upload URLs, credentials or raw configurations.

The current database PVCs and production backup archives are retained. Existing clean revision history is retained. An ordinary deployment recovery should use the clean saved gen-112 image and current configuration. Restoring a database dump is a disaster-recovery action requiring quiesced writes and an explicit choice of recovery point; never replay these preflight dumps over a live database as part of ordinary image rollback.

## Final verification

| Database | Bytes | Tables restored and matched | SHA-256 |
| --- | --: | --: | --- |
| `dpg-d9nqg95cavls73fp8m20` | 52681966 | 292 | `09d9446d03c9db43bdf65ee3e3d20d4b03b549bbe4adb13dc428cb3d9ca7daa3` |
| `dpg-d9rrkoc4h4mc73edurp0` | 132513651 | 295 | `9a08325ce1073049e574dc15834dd7264f2e5ef54b99862d561a0c0fb5c4f7fa` |
| `dpg-d9rs3ee0ccis738kc7c0` | 81484170 | 295 | `4159dac6abdc4f06eee0a5337a88f8467d95288c903b8638652ddae965320cb6` |

All **882 public tables** restored with exact same-snapshot row counts and application-role ownership. All three fresh Barman backups completed; the first database's successful backup has the `-retire-20260906-solo` suffix, the others `-retire-20260906`. The failed first attempt remains distinguishable in Backup history.

The offline gen-112 OCI layout preserves manifest `sha256:a9df34a83a2bd3356f55b8aecff128044bb0e96b4c9f759c632c7477f3ea6742`; all 37 stored content objects passed SHA-256 verification. Temporary registry authentication material was removed from the backup directory.

Before mutation, rechecked all App, Pod, Deployment, StatefulSet, DaemonSet, Job and CronJob specs for the old image: none referenced it. Deleted exactly the five gen-54 ReplicaSets with UID and resourceVersion preconditions after verifying zero desired/current replicas and the expected Deployment owner. Six clean-image ReplicaSets remain, including the active one. Other legacy tags and all current scoped repository tags were unchanged.

The gen-54 manifest deletion returned HTTP 202. The deployed Zot also accepted targeted deletion of the now-unreferenced credential layer (HTTP 202), so there was no need to alter registry-wide garbage-collection settings or wait for the hourly sweep. The versioned [Zot handler](https://github.com/project-zot/zot/blob/v2.1.18/pkg/api/routes.go) documents this endpoint and rejects referenced blobs. Follow-up checks using the forum's own pull credential returned HTTP 404 for the tag, manifest, layer HEAD and layer GET. A temporary read-only registry-volume inspection found **zero files with the credential-layer digest anywhere on that volume**. This verifies the local registry copy's removal, not deletion of unknown external historical copies.

Post-retirement, the unchanged gen-112 Deployment remained 1/1 Ready; all three origin site APIs returned HTTP 200. No App spec, Deployment template, Secret, database PVC, registry ACL or global registry configuration was changed. Temporary inspection pods were deleted. Scratch restore/client containers and local port forwards are removed after recording verification; the restricted backup files remain with the user.
