# w2/m92 · Pre-existing: beancount-cms-v2 stuck Deploying

**Date:** 2026-09-08 · **Cluster:** `hetzner-prod` · **Mode:** read-only diagnosis (no mutate)

## Finding

`tea-d98210…/…-beancount-cms-v2` remains `phase=Deploying` with:

- Serving image already **scoped** `…/W/A:gen-460@sha256:88498f…`
- Deploy condition `Progressing=False` / `ProgressDeadlineExceeded`
- Old RS pod **Running** `1/1` (still serving)
- New RS pod **Pending** 38m+: cannot schedule

## Scheduler / autoscaler

```
0/10 nodes available: 2 Insufficient cpu, 2 Insufficient memory,
8 node(s) had untolerated taint(s).
cluster-autoscaler: max node group size reached
```

Pod requests `cpu: 1` (+ ephemeral-storage 4Gi). Not an identity/registry issue.

## Impact on m92

- Phase 2 copy of historical legacy tags is independent of this rollout.
- Phase 3 for this App: confirm-only once Ready; **do not** restart while Pending/ProgressDeadlineExceeded unless capacity is fixed first.
- Live legacy cutover remains only `…-agentmarketcap-1`.

## Mutations

**None** this note.
