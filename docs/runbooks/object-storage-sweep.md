# Runbook: the object storage sweep stopped completing (ObjectStorageSweepStale, ObjectStorageSweepCutOff)

**What it is:** every `bex-api` replica runs the hourly object storage sweep (`lego/backend/internal/objectstorage/sweeper.go`, [ADR112 § D8](../ADR112-object-storage.md#d8--usage-quotas-and-billing-the-bucket-is-the-truth-the-gate-is-at-write-v1-is-a-shadow-meter)): it lists each workspace's prefix in Wasabi, writes the per-bucket usage, runs the lifecycle rules ([D5](../ADR112-object-storage.md#d5--lifecycle-rules-run-in-the-sweep)), purges deleted workspaces and feeds the shadow storage meter. A replica's pass runs on its own hourly tick, and the Postgres advisory lock `object-storage-sweep` keeps two passes from overlapping. A pass has 50 minutes (`sweepPassTimeout`) and never outlives its hour; the next pass starts with the workspaces it did not reach. Board: `w21/096`.

Since w21/095 a region is listed again only after a gap of an hour per 100 objects it held (at least 45 minutes, at most 71 hours 45 minutes), because Wasabi counts what a listing reads as egress. A pass can therefore do almost nothing (`3 fresh`) and take a second, and that is healthy. Both alerts read how a pass **ended**, never how much work it did.

## The two alerts

| Alert | Fires when | It means |
| --- | --- | --- |
| `ObjectStorageSweepStale` | No pass has **completed** (reached every workspace without a failure) in three hours, for 30 minutes; or `bex-api` exports object storage but none of the sweep's series | Three hourly passes were missed or cut off or failed, or neither replica gets the lock, or the series are not stored. |
| `ObjectStorageSweepCutOff` | The 50-minute pass was cut off at its deadline at least twice in three hours, for 10 minutes | The pool is too small or a listing too slow for the workspaces the sweep has. Each cut-off leaves workspaces unswept until the next pass, which starts with them. |

A cut-off pass never counts as complete, so a sweep cut off every hour fires `ObjectStorageSweepCutOff` about two hours in and `ObjectStorageSweepStale` at three.

What is stale while it fires: the usage the quota gate and `bucket info` read (a workspace over its cap may keep writing until a pass lists its region again), the lifecycle rules (expired objects stay), the purge of a deleted workspace (its objects stay in Wasabi), and the shadow storage meter. Request-time meters, the egress meter and public reads are not affected.

## Read first: the pass line

Every pass that runs ends with one log line. Read both replicas, since either may have run it:

```bash
kubectl -n bex-system logs -l app.kubernetes.io/name=bex-api --prefix --since=6h --tail=-1 | grep 'object storage sweep'
```

```
object storage sweep: pass over N workspaces took T with 4 workers: a swept, b fresh, c failed, d not reached; listings read about B bytes
```

- **`d not reached` above 0** is a cut-off (the error line says `pass cut off: d of N workspaces not reached`). Look at `T`: near 50 minutes means the workspaces take longer than the pass has. Find the slow ones: the largest `object_storage_usage.objects` per workspace, and `bex_usage_object_storage_sweep_workspace_duration_seconds` in Prometheus. If the listings themselves are slow (Wasabi degraded), more workers will not help; see below.
- **`c failed` above 0** means that many workspaces returned an error, and each one is logged before the pass line as `object storage sweep: workspace <tea-id>: <error>` (or `... in <region>: lifecycle delete failed`). One workspace that fails every pass keeps the sweep from ever completing, so it fires `ObjectStorageSweepStale` after three hours. Fix that workspace's cause (a Wasabi error, a purge that cannot finish, a panic: `recovered panic`); the passes complete again by themselves.
- **`object storage sweep: deleted workspace <tea-id>: …`** comes from the look under purged prefixes after the workspaces ([ADR112 § D9](../ADR112-object-storage.md#d9--credentials-and-rollout-reuse-the-adr095-and-adr029-recipes)). `removed n objects that arrived after its purge` is an upload slower than the purge hold, now deleted: nothing to do. An error there is not counted in `c failed`, does not fail the pass and comes round again with the other purged workspaces. Only a workspace recorded in `object_storage_purged_workspaces` is ever looked under.
- **Only `fresh` and `swept`, no failures, but the alert fires:** the series, not the sweep, is the problem. Check that Prometheus stores them (`bex_usage_object_storage_sweep_last_complete_timestamp_seconds`; the bex-api scrape job keeps only the families its keep-list names, `TestEverySweepMetricIsKeptByTheScrapeJob`) and that both pods are scraped (`up{job="bex-api"}`).
- **No pass line on either replica for hours:** neither replica runs a pass. Look for `object storage sweep: recovered panic` or an error from the lock in the same logs, then check the lock (next section). A fleet without the Wasabi credential runs no sweep at all and exports no series; the alert is quiet there.

The Billing and metering dashboard has the same story in four panels: "Object storage sweep: pass duration by result" against the 50-minute limit, "workspaces by result", "age of the last complete pass" against the alert's three hours, and "listing bytes".

## The lock: a pass that never starts

The lock is held only while a pass runs, so a holder seen when no pass line is recent is stuck, typically a replica that died with its Postgres connection half open:

Run it on the platform database's primary (`bex-db-1` is not always it; a lock is per server):

```bash
primary=$(kubectl -n bex-system get pods -l cnpg.io/cluster=bex-db,cnpg.io/instanceRole=primary -o name)
kubectl -n bex-system exec "$primary" -c postgres -- psql -U postgres -d bex -c "
SELECT l.pid, a.state, a.state_change, a.client_addr
FROM pg_locks l JOIN pg_stat_activity a USING (pid)
WHERE l.locktype = 'advisory' AND l.objsubid = 1
  AND (l.classid::bigint << 32 | l.objid::bigint) = hashtextextended('object-storage-sweep', 0);"
```

No row is the healthy answer at rest. A row whose `state_change` is hours old is the cause: `SELECT pg_terminate_backend(<pid>)` releases the lock, and the next tick of either replica runs a pass. Ending a live pass is safe: a listing is recorded only when it finished, so nothing is half written, and the next pass starts with the workspaces it did not reach.

## What raises the pool bound

`sweepWorkers` in `lego/backend/internal/objectstorage/sweeper.go` (4, [ADR112 § D8, "A pool of 4"](../ADR112-object-storage.md#d8--usage-quotas-and-billing-the-bucket-is-the-truth-the-gate-is-at-write-v1-is-a-shadow-meter)) is how many workspaces one pass sweeps at once, and therefore the most listing requests in flight to Wasabi. It is a constant, not an environment variable ([D9](../ADR112-object-storage.md#d9--credentials-and-rollout-reuse-the-adr095-and-adr029-recipes)): raising it is a reviewed code change, with a new measurement from a pod in the production cluster, because Wasabi publishes no limit for S3 requests and the account is shared with the platform's backups.

Raise it only when the cut-off is explained by many slow workspaces (the sum of the workspaces' durations divided by the workers is close to the pass's `T`), and the listings are healthy. Do not raise it when workspaces fail, or when Wasabi answers slowly or throttles (`RequestRateLimitExceeded` in the errors): more concurrent listings make that worse. More workers do not change the bytes a pass reads either, so `ObjectStorageSweepListingHigh` is unaffected. The other levers are the ones D8 names as not done: keeping object counts incrementally on writes, so a listing is only a reconciliation, and a lower per-workspace object cap.

## After the fix

- The next pass line reads `0 failed, 0 not reached`, and "age of the last complete pass" drops to minutes.
- `ObjectStorageSweepStale` resolves at the next complete pass. `ObjectStorageSweepCutOff` is a count over three hours and resolves once fewer than two cut-offs remain in that window.

`ObjectStorageSweepListingHigh` (what the sweep's listings read, against its 10 GB a month budget) is a different signal with its own steps in ADR112 D8; the kill switch for public reads is [objectgw-kill-switch.md](objectgw-kill-switch.md).
