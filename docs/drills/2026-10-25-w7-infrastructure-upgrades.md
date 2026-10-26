# w7 infrastructure upgrade verification — 2026-10-25

## Outcome and limits

All three changes are shipped and have passed production acceptance: Zot GC prevention, CNPG 1.30.1 with in-place instance-manager updates, and rollback of the expired cache trial. All 13 PostgreSQL instances across nine clusters retained their Pod/container identities and postmaster start times. More than 30 continuous minutes of database and WAL observations passed, and a new platform backup completed. The dated checkpoints below distinguish the initial local evidence from subsequent production verification.

The initial production verification was blocked on authenticated cluster access: one discovered control-plane node disagreed with the local SSH pin and two others had no entry. Access was subsequently recovered by matching fresh captures against the protected CI environment's provisioned fingerprints, with strict checking throughout. The user explicitly authorized shipping and continued verification.

The existing CAPD cluster was read only: its API was reachable through the control-plane container's current published port, but all three workers were NotReady. The local drill used a disposable single-node kind cluster, `bex-w7-unblock`, with its own kubeconfig outside the repository, on Kubernetes 1.36.1. It did not change other workstreams' stacks. The later production rollout and backup are recorded separately below.

## Zot: missing metadata no longer prevents GC

The upstream fix is [project-zot/zot#4500](https://github.com/project-zot/zot/pull/4500), included in [v2.1.22](https://github.com/project-zot/zot/releases/tag/v2.1.22). The existing reviewed chart remains 0.1.121; its image override now selects:

```text
ghcr.io/project-zot/zot:v2.1.22@sha256:96cda11459ce6f8c60da3b03f03e9f67fd11b3ae2d82c4c04c92b3c608996080
```

The repeatable local acceptance check is:

```sh
python3 scripts/verify-zot-gc.py
# Negative control; this must fail with missing-metadata GC errors:
python3 scripts/verify-zot-gc.py ghcr.io/project-zot/zot:v2.1.18
```

The script uses disposable Docker containers and storage. It seeds v2.1.18 with a retained live image and a second image, deletes the second image's last manifest, then removes only the disposable fixture's metaDB while the registry is stopped. Starting the tested version rebuilds metadata from manifests, omitting the empty repository while retaining its orphan blobs. This explicitly injects a metadata rebuild; it does not claim an ordinary restart always discards a persistent metaDB.

With v2.1.18, the empty repository survived the bounded check and three `failed to get repoMeta` errors were observed. With the pinned v2.1.22 image, two successful GC cycles completed, the orphan repository disappeared, and the retained manifest plus its config and layer bytes remained readable with their original digests. The accelerated fixture uses a 30-second GC delay; production's build-safe delay is unchanged. Fixture containers and storage are cleaned up on exit.

`scripts/gitops-validate.sh` now renders the reviewed Zot chart and checks the resulting StatefulSet image. Existing source-value checks continue to cover the operator-owned config, htpasswd mounts, default anonymous denial, sizing, and PVC normalization exceptions. No production ACL or retention policy was changed.

## CNPG: NULL WAL metrics and uninterrupted PostgreSQL

The correction is [cloudnative-pg/cloudnative-pg#11207](https://github.com/cloudnative-pg/cloudnative-pg/pull/11207), released in 1.30.1. The reviewed chart changes from 0.29.0 to 0.29.1; archive SHA256 is `b53d3991fe84bcf38767e7702cae78666265427a127a26fff168ab4207d2b1df`. A temporary local OCI registry produced deterministic manifest digest `sha256:4ab30b6a4d8aa029ff551102ec2fd9835c9f16c91f73963618a970b0b10f5a67` using the same timestamp normalization as `scripts/helm-artifact.sh`. Nothing was published to GHCR locally; the release workflow mirrors the locked archive after shipping.

The operator image is pinned to:

```text
ghcr.io/cloudnative-pg/cloudnative-pg:1.30.1@sha256:923c267ec29636db3bee20f993d0ec4973fa22998e1adad37da79e4d32b5bc07
```

The chart's operator configuration sets the string `ENABLE_INSTANCE_MANAGER_INPLACE_UPDATES: "true"`. This is the upstream-supported mechanism for replacing the instance manager while adopting the running PostgreSQL process: [CNPG 1.30 upgrade documentation](https://cloudnative-pg.io/docs/1.30/installation_upgrade/#in-place-updates-of-the-instance-manager). PostgreSQL operand images and database specifications are unchanged by the production patch.

The isolated drill used two clusters in namespace `w7-cnpg-test`, a single-instance cluster and a two-instance cluster, all on PostgreSQL `18.4-system-trixie`. The baseline operator was 1.30.0 with in-place updates disabled, matching production's recorded default. It was then upgraded to the pinned 1.30.1 image while enabling in-place updates in the same Helm upgrade.

The fresh instances did not naturally expose a NULL `stats_reset`. To make the regression deterministic, only the disposable single-instance cluster's `pg_catalog.pg_stat_wal` view was temporarily replaced with the same five columns and a NULL timestamp:

```sql
CREATE OR REPLACE VIEW pg_catalog.pg_stat_wal AS
SELECT w.wal_records, w.wal_fpi, w.wal_bytes, w.wal_buffers_full,
       NULL::timestamptz AS stats_reset
FROM pg_stat_get_wal() w;
```

Results:

- Before upgrading, a metrics scrape reproduced `while collecting pg_stat_wal` / `stats_reset: converting NULL to string is unsupported` on 1.30.0.
- After upgrading, `/controller/manager version` reported 1.30.1. WAL counters were present with `stats_reset="-infinity"` and `cnpg_collector_last_collection_error 0` while SQL still returned NULL. No statistics reset was used to hide the failure.
- All three database Pod UIDs, PostgreSQL container IDs and restart counts were unchanged across the operator upgrade.
- From 2026-10-26 00:44:09 through 00:45:19 UTC, 120 successive SQL queries succeeded. Every query returned fixture value 47 and the same PostgreSQL start time, `2026-10-26 00:40:36.980599+00`.
- Deleting the disposable standby Pod after the upgrade exercised restart/recovery. Its replacement had a new UID, ran instance manager 1.30.1, rejoined as a standby, and returned `pg_is_in_recovery() = true` with fixture value 47.
- Restoring the original test view also produced normal timestamp-labelled WAL counters with no collection error.

The fixture did not configure production Barman/S3 backups. The production acceptance below separately verifies current inventory, continuous archiving, a fresh plugin backup and the required 30-minute observation. Historical counts of seven clusters and three single-instance tenant databases are not asserted as current.

The GitOps validator renders the reviewed CNPG chart and checks both the actual Deployment image and the ConfigMap's string-valued in-place update setting. It also keeps the reviewed archive and Argo digest checks.

## Cache trial 047: expired trial rolled back

The original September 17–20 activation evidence remains in the board note. It proved cache persistence and successful builds, not estate-wide performance benefit. The committed deadline was still `2026-09-20T02:34:30Z` at the beginning of this drill. Current live breaker state and original-window push latency/errors, repeat-build results and Zot storage could not be inventoried without production access; historical source numbers are not substitutes.

The implementation decision was to roll back the expired trial: remove both `BEX_BUILD_CACHE` and `BEX_BUILD_CACHE_TRIAL_DEADLINE` from the production overlay. The cache implementation remains available and opt-in. This does not claim the feature is ineffective; it avoids permanent enablement without the trial's required evidence. The user delegated the decision, and the live Deployment and gauge checks below confirm the rollback.

A future repeat trial should start only after fresh registry admission measurements and Zot GC verification. Choose its absolute deadline at rollout time, bound it to 72 hours, retain the existing latency/error breakers, and collect paired cold/warm builds plus push and storage measurements. This drill neither extends the old deadline nor starts a new trial.

## Production closeout criteria

The acceptance sequence was:

1. Reinventory CNPG clusters, current backups, operator configuration and current registry capacity. Verify deployed images match the committed digests.
2. Verify CNPG's instance-manager versions directly, PostgreSQL availability, replication and backups. Observe at least 30 minutes of WAL metrics without NULL scan errors. Pod image text alone is insufficient for an in-place update.
3. Verify at least two Zot GC cycles in the forced-rebuild fixture, then native production GC tasks, retained-image pulls and repository isolation. Preserve production's three-hour safety interval; the observed task counts below do not imply two fleet sweeps.
4. Verify the production operator Deployment no longer requests the cache trial and `bex_build_cache_enabled` is zero. Record a dated rollback result in 047.
5. Only then finish the remaining task checkboxes and move the milestones/note to `done/` through the PM workflow.

## Verification and review

- `GOWORK=off make test` from `lego/operator/`: passed, including code generation and envtest.
- `bash scripts/gitops-validate.sh`: passed. Its existing optional FGA model drift check was skipped because the FGA CLI is absent; the production manifests, locked chart renders, 123 platform alert rules and alert tests passed.
- `python3 scripts/verify-zot-gc.py`: passed with retained blob-digest checks. The v2.1.18 negative control failed with orphan storage retained and three missing-metadata errors.
- Simplify's reuse, quality and efficiency reviews completed. Applied the existing scalar guard to rendered-image captures, corrected a stale chart-rendering comment, and moved constant manifest parsing outside the GC polling loop.
- `git diff --check`: passed. Repository-wide Prettier 3.4.2 formatting was run for Markdown.

The disposable kind cluster and Docker registry fixtures were removed after evidence capture. Backup creation was verified in production; database recovery was exercised only in the isolated fixture.

## Shipping and access recovery checkpoint

The user explicitly requested “ship and continue, do not stop until you absolutely need me”. The batch was pushed to `origin/main` at `418e5ccfd`: registry `a754faaec`, CNPG `aa393fe29`, expired cache rollback `39f06eaf8`, and the shared record `418e5ccfd`. Rebased GitOps validation and pre-push gates passed.

At this checkpoint, production delivery was queued behind an older active deployment. The new manual, main-only `Verify production SSH trust` workflow uses the existing protected `production-deploy` environment and reports only `ssh-keygen` fingerprints of its provisioned known-hosts entries. No client private key or kubeconfig is exported. A network-scanned key is usable locally only after it matches this independent trusted fingerprint.

Two entries added through the initial fetch attempt's `accept-new` fallback were identified as unverified session-created records and removed from the local global known-hosts file. They were not used as authenticated pins; no newly scanned key was trusted by itself.

## Production acceptance checkpoint — 2026-10-26 UTC

Protected [SSH trust run 40256047498](https://github.com/bex-co/bex/actions/runs/40256047498) reported the provisioned host-key fingerprints. All three exact IP/fingerprint pairs matched fresh network captures; the canonical local known-hosts file was updated from that authenticated comparison, and production access succeeded with strict pin verification. No client key or kubeconfig was exported by CI.

Zot 2.1.22 started at 01:40:27 UTC. Its scheduler completed 57 per-repository GC tasks across 57 repositories between 01:40:31 and 01:55:18, with zero GC errors. Production's current GC delay and interval are both 3h and were preserved. These are 57 native repository tasks, not a claim of two entire three-hour fleet sweeps. The local old/new fixture supplies repeated-cycle orphan reclamation under the injected metadata-rebuild condition.

Two existing Apps in separate namespaces returned their exact recorded image manifest digests and config bytes; all 32 referenced blobs returned HEAD 200 with expected sizes. Cross-namespace credentials were refused with 403 in both directions, and anonymous enumeration returned 401. No tenant App or registry content was changed by these checks.

The cache rollback is live: both variables are absent, bex-operator is Synced/Healthy and `bex_build_cache_enabled` is zero. The original-window queries returned zero retained series for the cache gauge, push histogram count, push errors and Zot PVC usage; Prometheus currently retains three days. The later storage snapshot is 49,495,916,544 / 105,559,699,456 bytes (~46.9%). It is not original trial evidence. The completed 047 note preserves the historical activation evidence and explicitly records rollback.

[Reviewed-chart mirror run 40257260403](https://github.com/bex-co/bex/actions/runs/40257260403) passed and published the exact locked CNPG digest. Argo then deployed 1.30.1 with in-place updates. Current inventory is nine healthy clusters / 13 instances, including five single-instance tenant databases; every PostgreSQL operand image is explicitly pinned in its Cluster and remained unchanged. The original affected instance still returned NULL from `pg_stat_wal.stats_reset`, while the corrected exporter produced WAL series labelled `-infinity` with collection error zero. This is the natural production failure mode, without changing a view or resetting statistics.

From 02:18:55 through 02:49:32 UTC, 33 consecutive estate observations covered every instance's SQL availability, actual instance-manager version, postmaster start time, Pod UID, PostgreSQL container ID/restart count, cluster readiness and continuous archiving. All 13 ran 1.30.1, all nine clusters remained healthy, no process/container was replaced, and the bounded monitor passed after 1,828 continuous seconds. The direct WAL monitor independently passed 108 healthy scrapes over 1,804 seconds, from 02:18:07 through 02:48:11. Production Prometheus intentionally does not retain these collector/WAL series, so this check scraped the affected instance's exporter directly through a localhost port-forward. A final untruncated log check through 02:51:43 found zero NULL WAL scan errors on all 13 instances. SQL still returned NULL and the final collector error gauge remained zero.

Post-upgrade Barman plugin backup `bex-system/w7-cnpg-upgrade-zjz4h` completed from standby `bex-db-2`, starting 02:21:03 and finishing 02:22:47 UTC (`backupId: 20261026T022103`). This backup remains subject to the existing backup retention policy. All nine clusters had healthy continuous archiving; seven had earlier completed base backups and the two newer tenant clusters had none before this drill. No tenant database or registry fixture was created in production.

Argo's `cnpg-operator`, `zot` and `bex-operator` Applications were all Synced/Healthy at final acceptance. PM closeout moves m156, m158 and 047 to `w7/done/`, with all task, milestone and workstream statuses synchronized.

Both bounded observers exited successfully. Their localhost port-forwards were stopped and the temporary production kubeconfig was removed. The independently verified canonical SSH pins remain available for future strict access; no credentials are stored in this record.

The manual chart mirror reuses the existing reviewed-artifact helper. It and the regular deployment job share a queued publisher mutex (`queue: max`, `cancel-in-progress: false`), preserving the outer deployment transaction and avoiding cancellation of pending work. See [GitHub's concurrency documentation](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency).

The older deployment run 40247966232 failed during Trivy's vulnerability-database download from `mirror.gcr.io`, before the dashboard scan completed. Commit `edf98c22f` separately fixes that CI failure: use the official GHCR database first with the existing mirror as fallback, and disable the stalled remote action-cache restores. All ten scans and five CRITICAL failure gates remain. Downloading the complete official GHCR database succeeded locally; the old failed jobs were rerun in the background. New CI is not represented as green before completion and does not delay this production-verified closeout.
