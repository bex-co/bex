# w1 · m1 — Reliability: fix config drift + back up etcd

**Worker:** worker1 **Goal:** Make the currently-live single-node deployment correct and recoverable — the operator must propagate operator-level config changes to running Apps, and App state in etcd must survive a node rebuild. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Drop the Reconcile early-return so desired state always applies — **DONE** | 25m | — |
| t002 | Requeue all Apps on operator startup / config change | 30m | t001 |
| t003 | etcd snapshot CronJob → Wasabi | 30m | — |
| t004 | Snapshot retention + documented restore | 20m | t003 |

## Definition of done

Flipping an operator-level config (e.g. `BEX_CLUSTER_ISSUER`) re-reconciles every running App with no manual nudge; a daily etcd snapshot lands in Wasabi and a documented restore onto a fresh node recovers the `App` objects.

## Source

Converted from `.tmp/009-operator-reconcile-drift.md` and `.tmp/007-etcd-snapshot-backup.md`.
