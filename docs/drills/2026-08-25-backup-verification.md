# Backup and restore verification — 2026-08-25 (production)

**Scope:** all ADR031 stores, production application cluster (`hetzner-prod`), run from an operator workstation with `.env` S3 custody but **without** `AGE_BACKUP_PRIVATE_KEY` (that key's canonical custody is GitHub Actions secrets).

**Outcome:** PASS for the CNPG estate (two live scripted restores, all archives fresh), PASS for etcd/OpenBao backup freshness and S3 inventory, **FAIL for paid KeyValue** — the verification found the store had been running with zero-to-one recovery points across all three paid instances (two distinct production defects, below), mitigated same-session. Hermetic CI gates (17/17) and the CNPG backup guard passed.

No kubeconfig, Secret value, S3 credential, OpenBao material, tenant row, or private key is recorded here. Object keys, aggregate counts, namespace names, and timestamps are non-secret acceptance metadata.

## What was verified

| store | freshness (backup side) | restore side | result |
| --- | --- | --- | --- |
| etcd | CronJob success 2026-08-25T03:15:20Z; `etcd-snapshots/etcd-20260825-031513.db.gz.age` present | `DRY_RUN=1 restore-etcd.sh` selected + downloaded the fresh object; decrypt requires the CI-custodied age key, not present locally | PASS (backup + fetch); decrypt-then-restore last proven 2026-08-04, re-proof pending the queued workflow drill |
| OpenBao | CronJob success 2026-08-25T03:45:17Z; StatefulSet 3/3 | `openbao-restore-drill.yml` dispatched (run `32814333448`) — **queued behind a downed self-hosted runner fleet** at session end | PASS (backup); live restore pending runners |
| bex-db | Nightly `plugin` backup completed 04:00Z; WAL-staleness alert quiet | **Live scripted restore**: submitted 05:53:52Z → CNPG Ready 05:55:59Z (127 s), verification query passed, teardown verified | **PASS (end to end)** |
| kratos-db | Nightly backup completed 04:15Z | **Live scripted restore**: Ready in 98 s, aggregate identity count matched captured source count, teardown verified | **PASS (end to end)** |
| hydra-db / openfga-db | Nightly backups completed 04:30Z / 04:45Z; same script shape as Kratos | not separately restored (representative Kratos restore, per 2026-07-31 precedent) | PASS (backup) |
| tenant Postgres | All 4 tenant clusters healthy; nightly backups completed 03:00Z | `DRY_RUN=1` rendered the recovery Cluster for `dpg-d9rrkoc4h4mc73edurp0`; full live tenant restore last proven 2026-08-21 (cutover step 1) | PASS |
| paid KeyValue | **BROKEN — see incident** | `DRY_RUN=1` object selection worked (and exposed the empty prefix); local decrypt blocked by key custody | **FAIL → mitigated** |

Hermetic gates re-run this session: `scripts/restore.test.sh` 17/17 ok; `scripts/cnpg-backup-guard.sh` PASS. All three Tier A pipelines (etcd ConfigMap, OpenBao ConfigMap, operator `BEX_BACKUP_AGE_PUBLIC_KEY`) were confirmed to encrypt to the **same** age recipient, so one CI-custodied private key covers all Tier A decrypts.

## Incident: paid KeyValue backups (two defects, compounding)

State at session start: `red-d9p49kdrtmes73c34ovg` **zero** objects, `red-da4086iii7bs73drbqh0` one object (2026-08-22), `red-da52hdb8ptnc73bm4uk0` **zero** objects — against a ≤24 h RPO objective.

**Defect 1 — cutover purge erased history.** The 2026-08-21/22 datastore-namespace cutover deleted each old `default`-namespace KeyValue CR after recreating it in its tenant namespace. The delete finalizer's purge Job runs `s3 rm --recursive` on `keyvalue/<id>/` — the **same prefix** the recreated instance writes to, because the id is preserved. Every pre-cutover recovery point was destroyed, including (for `red-d9p49…`) the new instance's first successful post-cutover object.

**Defect 2 — encrypt/upload UID mismatch.** w7/m85 (2026-08-21) replaced the KV encrypt stage with the operator image's `/backup-encrypt`, which wrote the ciphertext `0600` owned by the image's non-root UID. Every container in the Job drops ALL capabilities — including `DAC_OVERRIDE` — so the `amazon/aws-cli` upload stage could not read the file regardless of its own UID. AWS CLI skipped the "unreadable" file and exited 2. Every scheduled kvbak Job failed from 2026-08-23 onward (3 retries nightly, logs GC'd; diagnosed via a manually triggered Job). `BackupCronJobStale` fired correctly from 2026-08-23T05:33Z for all three instances but was not acted on.

**Mitigation (this session).** One-off Jobs cloned from each CronJob's template with `upload.securityContext.runAsUser` set to the encrypt stage's UID completed for all three instances; the bucket now holds a fresh 2026-08-25T05:54–05:55Z recovery point per instance. Jobs deleted after completion. **This does not fix the nightly pipeline** — the CronJob template still carries the broken stage until the code fix ships.

**Code fix (pending ship).** `lego/operator/cmd/backup-encrypt/main.go` now writes the ciphertext `0644` (it is age ciphertext; pod-internal readability leaks nothing), with a regression test asserting cross-UID readability. Until it ships and the operator rolls, scheduled kvbak Jobs keep failing and the alert keeps firing.

## Additional finding: self-hosted runner fleet down

Every GitHub Actions job queued from ≥05:01Z (CI for the day's push, the dispatched OpenBao drill, and any future ship of the fix). The runner hosts are operator-provisioned outside the repository; recovery requires the operator. Until then the KV fix cannot ship and the OpenBao drill cannot run.

## Follow-ups filed

[`w8/m30`](../../.pm/w8/done/m30/README.md) (promoted from the initial `w5/049` inbox note by user handoff, 2026-08-25): ship the encrypt-mode fix + roll the operator, re-verify scheduled kvbak for all three instances, decide cutover-purge semantics (suspend purge on migration deletes and/or Object Lock per ADR050 follow-up), route `BackupCronJobStale` to a human-visible channel, complete the queued OpenBao drill (which also re-proves Tier A decrypt custody), and re-drill KeyValue restore. Restoring the runner fleet is the operator-side unblock for the ship.

## Follow-up resolution — 2026-08-26 to 2026-09-01

**Outcome:** PASS. The nightly KeyValue pipeline, migration-delete protection, human-visible alert route, OpenBao restore, and post-fix KeyValue restore are all live and verified. The completed milestone record is [w8/m30](../../.pm/w8/done/m30/README.md).

- **Ship + rollout:** `4fa97ce0` shipped the `0644` ciphertext mode, `app.bex.co/preserve-backups-on-delete` guard and envtest, deliverable Alertmanager receiver, and OpenBao workflow decrypt-key wiring. Production later rolled a descendant operator image, and both surviving paid-KeyValue CronJobs reference a rolled descendant containing the `/backup-encrypt` fix.
- **Scheduled protection:** both paid KeyValues still present had four consecutive CronJob-owned successes from 2026-08-29 through 2026-09-01. Each prefix held exactly seven encrypted objects after the 2026-09-01 run (oldest 2026-08-26, newest 2026-09-01), proving keep-7 pruning. `BackupCronJobStale` returned no active series. The third incident-era instance was normally deleted after observed successes on 2026-08-28 and 2026-08-29; its CronJob and prefix were correctly removed, so the live paid estate now contains two instances.
- **Alert delivery:** SendGrid Email Activity reported four 2026-08-31 `PlatformDatabaseBackupStale` messages to the configured operator inbox as `delivered`. Both backup-staleness rules use this receiver; this distinguishes actual mailbox delivery from the prior false-positive SMTP-send counter against a no-MX placeholder.
- **OpenBao restore:** run `33007385050` succeeded on 2026-08-26. It minted a fresh encrypted snapshot, decrypted and restored it into `restore-bao-ci-20260826-1952`, verified the marker, removed the target and source marker, and ended with OpenBao 3/3. Because the 2026-08-25 inventory proved etcd/OpenBao/KeyValue share one Tier-A recipient, the successful decrypt also re-proved CI custody of the common private key; etcd's full decrypt-and-extract restore remains the 2026-08-04 proof.
- **KeyValue restore:** a non-sensitive marker was written at `2026-09-01T02:23:51Z`; scheduled Job `kvbak-red-da4086iii7bs73drbqh0-29803898` completed at `03:41:57Z` and uploaded `2026-09-01T03:39:20Z.rdb.gz.age`. Run `33467191581` selected that scheduled object, passed `DRY_RUN=1`, decrypted and checksum-validated it, verified the marker with AOF disabled, enabled/rewrote AOF, restarted and verified with AOF enabled, and removed `restore-kv-m30-20260901`. The source marker was deleted afterward; no `bex.co/restore-target=true` namespace remained and both source KeyValues were `Ready`.
- **Gates:** `make test`, `make lint`, `scripts/restore.test.sh` (17/17), `scripts/github-actions-validate.sh`, and its self-test passed. Push CI for the CI-custodied workflow passed `scripts (test)` and `gitops (render)`.
