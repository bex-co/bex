# Scripted all-store backup and restore drill — 2026-07-31

**Scope:** w7/m69, production application cluster

**Outcome:** PASS — each ADR031 backup class completed a fresh backup followed by recovery through its committed `scripts/restore-*.sh` program. All recoveries used throwaway targets; no script modified a source store or offered an in-place mode. Marker freshness passed for OpenBao, `bex-db`, tenant Postgres, and KeyValue. Every target and source drill resource was removed.

**Versions:** Kubernetes v1.34.9; etcd `registry.k8s.io/etcd:3.6.5-0`; OpenBao `quay.io/openbao/openbao:2.5.5`; CloudNativePG 1.30.0; Barman Cloud plugin/sidecar 0.13.0; PostgreSQL 18.4 and 16; `valkey/valkey:8-alpine`; AWS CLI 2.22.35.

The drill ran during the evening of July 31 in the operator timezone; Kubernetes and workflow timestamps below are UTC on 2026-08-01. No kubeconfig, Secret data, S3 credential, OpenBao key/token, Valkey password, identity id/email, customer row/value, or private key is recorded here. Marker strings, restore namespace names, object keys, aggregate counts, and image versions are non-secret acceptance metadata.

## Safety and test gates

All four scripts source `scripts/lib/restore.sh`. A Kubernetes recovery requires a new `restore-*` namespace and an exact `--confirm` match. The namespace is labeled `bex.co/restore-target=true`; teardown refuses an unlabeled namespace and waits until the labeled namespace is absent. `restore-etcd.sh` extraction never writes Kubernetes; its separate `--apply-dir` mode safety-validates every reviewed JSON file and requires a `restore-*` kube context. `restore-openbao.sh` rejects `--live`/same-instance, and `restore-keyvalue.sh` has no `--in-place` path.

Before live mutation, production `DRY_RUN=1` passed for all four scripts. The Postgres preview rendered `recoveryTarget.targetTime: 2026-08-01T09:18:40Z`, proving the optional PITR path without creating a resource. CI run `30694420655` passed ShellCheck and nine hermetic assertions: environment-only credential mapping, latest RFC3339 snapshot selection, all four dry-run paths with zero fake Kubernetes/S3 mutations, corrupt gzip rejection, bad resource-id rejection, and non-throwaway namespace rejection.

## etcd

The production `kube-system/etcd-backup` CronJob completed a fresh Job from `09:17:19Z` to `09:17:32Z`; its selected object was `etcd-snapshots/etcd-20260801-091727.db.gz`. `DRY_RUN=1 scripts/restore-etcd.sh` then:

1. downloaded and passed gzip plus `etcdutl snapshot status` integrity checks;
2. restored the snapshot into a local Docker data directory and started a one-member throwaway etcd;
3. extracted and sanitized exactly the four current App CRs (one in `default`, three in the active tenant namespace); and
4. removed the local etcd container, restored data, and review-output directory.

The scripted restore ran from `09:17:52Z` to `09:18:12Z` (20 seconds). The stale deleted App present in the earlier nightly snapshot was absent, which also proved the script selected the fresh object. Applying manifests was deliberately not part of this healthy-cluster drill; the extracted JSON was structurally validated and then deleted.

## OpenBao

OpenBao's Shamir material is intentionally held only in GitHub Actions secrets, so the committed manual workflow ran `restore-openbao.sh` without exporting secret values. Final workflow run `30694423134` passed in 1 minute 54 seconds.

The workflow wrote a disposable `tenants/` marker without printing its path contents, then triggered `secrets/openbao-backup`. The fresh backup completed from `09:47:18Z` to `09:47:27Z`; the script selected `openbao-snapshots/openbao-20260801-094722.snap.gz`.

The script created `restore-bao-m69-final/openbao-0` with a new PVC, initialized/unsealed only that fresh target, used the fresh root token for the force-restore API, restarted, and unsealed with the original three Shamir keys. Snapshot-force through marker verification ran from `09:48:01Z` to `09:48:34Z` (33 seconds). It then deleted the namespace by `09:48:52Z`. The workflow deleted the live marker and proved a subsequent read returned 404, deleted the one-shot backup Job, and found the source StatefulSet 3/3 Ready.

The first workflow attempt failed safely before target creation because the environment-only S3 aliases were not mapped when dotenv loading was disabled. Commit `e9b702d5` moved aliasing outside that branch and added a hermetic regression assertion. A second run passed; commit `54e169be` added positive live-marker deletion verification, and the final run above re-earned the complete drill.

## CloudNativePG archives

`scripts/restore-postgres.sh` exercised all three ObjectStore contracts. Each run copied its source ObjectStore plus referenced Secret streams into a new namespace, generated a one-instance recovery Cluster with no backup/WAL-writer plugin, waited for CNPG Ready, suppressed query text/results, and deleted the whole namespace.

| Archive class | Fresh backup | Scripted recovery | Verification and cleanup |
| --- | --- | --- | --- |
| `bex-system/bex-db`, server `bex-db` | `bex-db-m69-base`; `09:18:39Z`–`09:18:42Z`; backup id `20260801T091839` | `restore-pg-m69-bex`; submitted `09:19:08Z`, Ready `09:21:16Z` (128 seconds) | A dedicated marker row restored exactly once; target namespace absent; marker schema removed from the live source; source stayed 2/2 Ready |
| `default/bex-tenant-postgres`, server `dpg-m69drill000000000000` (PostgreSQL 16) | source Ready `09:23:36Z`; backup `09:24:51Z`–`09:25:01Z` | `restore-pg-m69-tenant`; submitted `09:25:26Z`, Ready `09:27:05Z` (99 seconds) | Marker restored exactly once; target namespace absent; source deletion/finalizer completed in 56 seconds; delete-audit 4/4; object prefix count zero |
| `auth/auth-dbs`, server `kratos-db-pg18` | `kratos-db-m69-base`; `09:28:54Z`–`09:28:59Z`; backup id `20260801T092854` | `restore-pg-m69-auth`; submitted `09:29:17Z`, Ready `09:30:27Z` (70 seconds) | Recovered aggregate identity count exactly matched the captured source count without recording it; target namespace absent; source stayed 2/2 Ready |

The tenant Database existed solely for the drill. Its normal operator finalizer removed the CNPG Cluster and backup prefix; `scripts/delete-audit.sh --db` passed every Kubernetes check, and an independent S3 API listing returned zero objects under its server prefix. The fresh `bex-db` and Kratos Backup CRs remain as production recovery inventory under their normal ObjectStore retention policies.

## KeyValue

The paid `default/red-m69drill` source reached Ready at `09:31:23Z`, stored a known marker, and completed a coherent `valkey-cli --rdb` backup from `09:31:36Z` to `09:33:18Z`. The selected object was `keyvalue/red-m69drill/2026-08-01T09:32:19Z.rdb.gz`; the real dry run passed gzip and `valkey-check-rdb` validation before any namespace existed.

The first actual attempt exposed a script portability issue: `kubectl exec -i` created a zero-byte `/data/dump.rdb` on this API-server/client combination. The script's mandatory non-empty check failed before Valkey startup and retained the labeled namespace for diagnosis. The PVC was writable and contained no AOF. The fix switched only the binary transfer to tar-backed `kubectl cp`, kept the empty-volume and non-empty-RDB guards, and deleted the failed namespace through the script's own teardown gate before retrying.

The clean rerun seeded a fresh PVC, verified the exact marker with `aof_enabled:0`, enabled AOF, waited for the initial rewrite, ran `BGREWRITEAOF`, rolled to the AOF-on configuration, and verified the marker again with `aof_enabled:1`. The script's load/rewrite/restart interval was `09:37:11Z`–`09:37:44Z` (33 seconds); namespace teardown also completed successfully.

Deleting the source invoked its normal backup purge finalizer from `09:38:18Z` to `09:39:24Z` (66 seconds). `scripts/delete-audit.sh --kv` passed all 11 Kubernetes checks, and a separate credentialed S3 listing returned zero objects under `keyvalue/red-m69drill/`.

## Final state

Final production checks returned:

- zero namespaces labeled `bex.co/restore-target=true`;
- zero NotReady nodes, unhealthy/unsynced Argo applications, or unavailable bex Deployments;
- OpenBao 3/3 Ready;
- zero tenant Database or KeyValue drill CRs;
- zero objects under the deleted tenant Postgres and KeyValue prefixes.

All ad-hoc work in the drill was limited to controlled marker writes and fresh backup triggers. Every recovery, verification, and recovery-target teardown was performed by the committed script for that store; the only mid-drill recovery intervention (the KeyValue transfer issue) was folded into the script and the entire target run was repeated cleanly.
