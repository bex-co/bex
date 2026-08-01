# Paid KeyValue backup and AOF-aware restore drill — 2026-07-31

**Scope:** w7/m68, production application cluster

**Outcome:** PASS — a paid throwaway KeyValue produced eight coherent RDB snapshots; the eighth run pruned the oldest object and left exactly the newest seven. The newest object passed RDB checksum validation, restored the known marker from object storage into a fresh PVC with AOF disabled, and retained the marker after AOF enablement, rewrite, and restart. Deleting the source removed its CronJob, retained Jobs and Pods, workload, credentials, PVC/PV, and complete object-store prefix.

**Versions:** Kubernetes v1.34.9; `valkey/valkey:8-alpine`; AWS CLI 2.22.35; operator image `ghcr.io/bex-co/bex-operator@sha256:aca80f973e54c4c2394d6de96ae7f9a83961ab767e01fbbe2c75f70499d9952d`.

The drill ran during the evening of July 31 in the operator timezone; Kubernetes timestamps below are UTC on 2026-08-01. The source id, marker, object keys, and image digests are non-secret acceptance metadata. No kubeconfig, S3 credential, Valkey password, customer value, token, or private key is recorded here.

## Rollout findings

The first fresh-volume start found that the official Valkey entrypoint tried to `chown` the PVC as root after the platform had dropped `CAP_CHOWN`. The shipped correction runs the image as its built-in `999:1000` account and gives the Pod `fsGroup: 1000`; replacing only the failed Pod brought the same PVC Ready with zero restarts. The first pre-acceptance backup then proved snapshot and gzip success but found AWS CLI trying to write config below its non-writable `/root`; the shipped correction sets only its config home to `/tmp` and disables EC2 metadata lookup. The failed Job and Pods were deleted, and the eight-run acceptance count restarted from one.

Commits `1e0534d2` and `a81b47a1` pin both regressions in unit/envtest coverage. The final current-main dispatch, including a concurrent billing commit, passed every repository test, image build/sign/CVE gate, and Argo rollout in GitHub Actions run `30690782098` at `08:16:21Z`.

## Backup and retention evidence

The source `default/red-m68drill` used plan `starter`, Valkey 8, and journal-snapshot persistence. It reached Ready at `07:34:10Z` after the runtime-identity rollout, then stored and read back `w7:m68:marker = w7-m68-marker-20260801` at `07:34:40Z`. Its deterministic CronJob schedule was `29 3 * * *` in `Etc/UTC`.

All eight accepted Jobs ran sequentially. Every `snapshot` init container completed `valkey-cli --rdb` successfully, every gzip stage completed, and every upload Job completed once without a failed Pod.

| Job            | Started     | Completed   | Object timestamp   |
| -------------- | ----------- | ----------- | ------------------ |
| `m68-kvbak-01` | `08:16:40Z` | `08:18:23Z` | `08:17:20Z.rdb.gz` |
| `m68-kvbak-02` | `08:18:41Z` | `08:20:21Z` | `08:19:23Z.rdb.gz` |
| `m68-kvbak-03` | `08:20:35Z` | `08:22:18Z` | `08:21:19Z.rdb.gz` |
| `m68-kvbak-04` | `08:22:32Z` | `08:24:12Z` | `08:23:11Z.rdb.gz` |
| `m68-kvbak-05` | `08:24:31Z` | `08:26:10Z` | `08:25:10Z.rdb.gz` |
| `m68-kvbak-06` | `08:26:21Z` | `08:27:59Z` | `08:26:59Z.rdb.gz` |
| `m68-kvbak-07` | `08:28:08Z` | `08:29:49Z` | `08:28:50Z.rdb.gz` |
| `m68-kvbak-08` | `08:30:21Z` | `08:32:28Z` | `08:31:02Z.rdb.gz` |

The eighth uploader logged deletion of `08:17:20Z.rdb.gz`. An independent S3 listing then returned exactly seven lexicographically ordered objects, from `08:19:23Z.rdb.gz` through `08:31:02Z.rdb.gz`. Downloading the newest object from `s3://bex-tfstate/keyvalue/red-m68drill/` and running the Valkey image's `valkey-check-rdb` reported `Checksum OK`, one key, and zero expired keys.

## Fresh-PVC restore and AOF transition

The selected gzip was seeded as `/data/dump.rdb` onto a new 1 Gi `hcloud-volumes` PVC at `08:36:11Z`; the loader proved that no `appendonlydir` existed before or after seeding. A throwaway StatefulSet started once with `appendonly no`. At `08:36:57Z`, it returned the exact marker and reported `aof_enabled:0`, proving that the RDB—not a surviving AOF—supplied the data.

The drill then enabled AOF through `CONFIG SET`, waited for the initial rewrite, explicitly ran `BGREWRITEAOF`, and waited for completion. The StatefulSet template changed to the managed `appendonly yes` setting, rolled to a new Pod, and at `08:37:43Z` returned the exact marker again with `aof_enabled:1` and three files under `appendonlydir`.

Cleanup deleted the restore StatefulSet, Service, Secret, PVC, bound PV, local snapshot, and Docker validation volume by `08:38:19Z`. Every named restore resource was observed absent.

## Alert and source-deletion evidence

Live Prometheus returned one `kube_cronjob_created` series and `kube_cronjob_spec_suspend=0` for `default/kvbak-red-m68drill`. The `BackupCronJobStale` rule reported `health=ok`, `state=inactive`, and no last error; its promtool fixture separately proves the >26-hour never-successful case fires while suspended and recent cases remain quiet.

Before source deletion, the CronJob controller retained the configured three successful backup Jobs and Pods. Deletion was requested at `08:39:20Z`. The finalizer removed the CronJob and those Jobs/Pods, ran `purge-kv-red-m68drill-ac8511cb` successfully, and released the KeyValue at `08:40:31Z` (71 seconds).

`scripts/delete-audit.sh --kv red-m68drill` passed all 11 Kubernetes checks. The source StatefulSet, Service, both Secrets, PVC, bound PV, CronJob, backup/purge Jobs, and Pods were absent. A separate credentialed S3 API listing returned zero objects below `keyvalue/red-m68drill/` at `08:43:24Z`. Final platform checks found zero NotReady nodes, zero unhealthy/unsynced Argo applications, zero unavailable bex Deployments, and zero remaining KeyValues.
