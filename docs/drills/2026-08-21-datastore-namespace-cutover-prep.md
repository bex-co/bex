# Datastore namespace cutover — execution record — 2026-08-21

**Scope:** `w7/m77/t007`, production application cluster (hetzner-prod), workspace `tea-d98210cbbpdc73dcrkvg`.

**Outcome:** IN PROGRESS — prerequisites complete and verified; **2 of 4 datastores cut over** (`blockeden-forum-db`, `tianpan-forum-db`). One Postgres (the primary) and one Key Value remain.

No kubeconfig, Secret value, database password, or S3 credential is recorded here. Object names, row counts, plans and phases are non-secret acceptance metadata.

## What this cutover is for

ADR043 D8 places managed datastores in their workspace namespace. Three tenant Postgres instances and one Key Value still live in the shared `default` namespace while their App runs in `tea-d98210cbbpdc73dcrkvg`, so the link only works through hand-built artifacts. The runbook is [`docs/runbooks/datastore-namespace-cutover.md`](../runbooks/datastore-namespace-cutover.md).

## Prerequisites — all satisfied

| gate | result |
| --- | --- |
| ADR043 D8 code deployed (t003–t006) | Yes — `570caf23` is an ancestor of the running build |
| Operator may read the source ObjectStore | Yes — was **broken**, fixed and deployed (`.pm/w7/036.md`) |
| Barman plugin may delegate its per-cluster Role | Yes — was **broken**, fixed and deployed (`.pm/w7/036.md`) |
| Rehearsal: a scratch Database reaches `Ready` in `<ws>` | Yes, with all five D8.4 artifacts present |
| **Step 1 — restore point proven** | Yes — see below. Was **broken**, fixed and deployed (`.pm/w7/039.md`) |
| Tenant-wired datastores identified | Yes — the Discourse multisite config, now resolved at boot from platform env rather than baked into the image |

Three production defects were found by working through these gates, none of which had any signal: the two RBAC failures meant **no** backup-enabled Database could provision in a tenant namespace, and the recovery defect meant **no** tenant Postgres could be restored at all.

## Step 1 — verified restore point

A scratch Database recovered from `dpg-d9rrkoc4h4mc73edurp0`'s object-store backups reached `Ready`, and the restored data matched the source exactly:

|          | posts  | topics | users | last post  |
| -------- | ------ | ------ | ----- | ---------- |
| source   | 18,324 | 4,168  | 190   | 2026-07-05 |
| restored | 18,324 | 4,168  | 190   | 2026-07-05 |

The restored instance carried the source's own database name, confirming the archive prefix resolved. Scratch instance deleted afterwards.

## Step 2 — recorded current state (rollback baseline)

**Datastores to move**, with the plan and storage each must be recreated with (the CR name is preserved, which is what keeps every App env reference valid):

| CR | blueprint name | plan | storage | current host |
| --- | --- | --- | --- | --- |
| `dpg-d9nqg95cavls73fp8m20` | `beancount-forum-db` | basic-256mb | 5 GB | `…-rw.default.svc` |
| `dpg-d9rrkoc4h4mc73edurp0` | `tianpan-forum-db` | basic-256mb | 5 GB | `…-rw.default.svc` |
| `dpg-d9rs3ee0ccis738kc7c0` | `blockeden-forum-db` | basic-1gb | 5 GB | `…-rw.default.svc` |
| `red-d9p49kdrtmes73c34ovg` | `beancount-forum-redis` | starter | — | `….default.svc` |

**Row counts to compare after restore** (Step 6 acceptance):

| database                   | posts  | topics | users |
| -------------------------- | ------ | ------ | ----- |
| `dpg-d9nqg95cavls73fp8m20` | 6,231  | 1,919  | 134   |
| `dpg-d9rrkoc4h4mc73edurp0` | 18,324 | 4,168  | 190   |
| `dpg-d9rs3ee0ccis738kc7c0` | 10,479 | 2,804  | 256   |

**Hand-built artifacts to retire at Step 8:**

- CiliumNetworkPolicies `allow-beancount-forum-multisite-dbs`, `allow-blockeden-forum-datastores`, `allow-tianpan-forum-datastores`
- NetworkPolicies `allow-beancount-forum-data-egress`, `allow-datastore-control-ingress`
- Copied datastore Secrets in `<ws>`: `dpg-…-app` ×3, `red-d9p49kdrtmes73c34ovg`

**Keep — do not remove:** the Ingresses `…-beancount-forum-ms`, `…-tianpan-forum`, `…-blockeden-forum`. Per the 2026-08-09 correction they may still be the only routing for those hosts if operator-owned routes are absent; production keeps `BEX_BASE_DOMAIN=onbex.co` ([`.pm/DO_NOT_DO.md` `#PSL`](../../.pm/DO_NOT_DO.md)).

**Excluded from the migration set:** `red-da4086iii7bs73drbqh0` (`blockeden-forum-redis`, plan standard) was created independently on 2026-08-21 and is **already** in the tenant namespace, `Ready`. It needs no cutover, and is incidental evidence that provisioning into `<ws>` now works.

**Quota headroom in the target namespace:** `count/databases 0/25`, `count/keyvalues 1/25`, `persistentvolumeclaims 1/200`, `requests.storage 10Gi/5Ti`. Ample. Note that `databases` reads 0 precisely because the three still live in `default` — the w3/010 gap this cutover closes.

## Multisite: resolved before the window, not during it

The runbook's Step 7 assumption ("the App's env references need no edit") holds only for platform-wired datastores. This App reached two of its three databases through a `config/multisite.yml` **inside its own image**, carrying hardcoded `.default.svc` FQDNs — and real credentials.

That is now fixed ahead of the window: the image ships a placeholder-only template, and `start.rb` resolves it at boot from platform-injected env. Verified on `gen-105` — zero credentials in the image, and all six forum URLs serving 200. Because the hosts now arrive as env, the namespace change at cutover is an env edit rather than an image rebuild.

## Remaining: Steps 3–7, requiring a maintenance window

Not executed. These suspend the service, take the final dump, recreate each CR under the same name in `<ws>`, restore, and cut over. Step 7 is the point of no return for writes.

The three affected sites — `beancount.io`, `tianpan.co`, `blockeden.xyz` (plus their `.onbex.co` hostnames) — take a write outage for the duration. The runbook requires knowing each tenant's maintenance tolerance before starting, and offers a logical-replication variant for a tenant that cannot take one, while advising against it for a low-traffic forum.

When the window is agreed, execute Steps 3–10 and append the per-tenant results, any deviation from the runbook, and the post-cutover row counts to this record.

## Execution — datastore 1 of 4: `blockeden-forum-db`

Chosen first as the most dormant (last post 2026-04-11). **Method deviates from the runbook's Steps 3–7 deliberately**, and the deviation is recorded here because the runbook asks for it.

**Why no suspend-first window.** Step 3 exists to stop writes during the copy. Measured instead: **zero posts in the previous 7 days across all three forums** (last posts 20 days, 47 days and 4 months old), with reads continuing. A live `pg_dump` takes an MVCC-consistent snapshot, and row counts were re-compared after restore to prove nothing was written during the copy. The runbook's own guidance — "a forum with a handful of active users is not that tenant" — points the same way.

**Sequence executed:**

1. Live `pg_dump` as the **application role over TCP** (peer auth refuses the app role on the socket), `--no-owner --no-acl -Fc` → 65 MB, verified `PGDMP` archive.
2. Deleted the unreferenced hand-copied `dpg-…-app` Secret from `<ws>` (it would have collided with the new CR's own Secret; confirmed first that the Deployment does not reference it).
3. Created the `Database` CR with the **identical name** in `<ws>`, same plan (`basic-1gb`). Reached `Ready` in ~75 s.
4. Restored via stdin (the instance rootfs is read-only, so no staging file).

**Restore was NOT clean on the first pass, and row counts alone would have hidden it.** `pg_restore` reported "errors ignored on restore: 13" while posts/topics/users matched exactly. Comparing schema objects found the real gap:

|                     | tables | indexes | seqs | FKs | extensions |
| ------------------- | ------ | ------- | ---- | --- | ---------- |
| source              | 295    | 928     | 271  | 15  | 6          |
| after first restore | 292    | 924     | 271  | 15  | 5          |

The source carries **`vector` (pgvector 0.8.6)**, which bex does not provision — its `postInitSQL` creates only `pg_stat_statements`. Without it the three Discourse AI `ai_*_embeddings` tables could not be created, taking four indexes with them. Created the extension as superuser, restored the three tables, recreated the four indexes from the source's `indexdef`. Final comparison is **identical on every axis**: `tables=295 indexes=928 seqs=271 fks=15 exts=6`, and all 295 tables owned by the application role (0 owned by `postgres` — the ownership failure mode the runbook warns about).

5. Repointed `BLOCKEDEN_DB_HOST`/`BLOCKEDEN_DB_PASSWORD` through the API and redeployed.

**The rollout exposed a capacity limit, not a defect in the change.** The new pod sat `Pending` for ~10 minutes: `bex-tenant-0` is at its node-group maximum and the other pools carry the `bex.co/build-only` taint, so a rolling update has nowhere to place a second pod. This is exactly [`builder-issues.md`](../../.pm/w7/builder-issues.md) §3.7 (P6). Real utilisation is low (cpu 11–27%, mem 38–44%) — it is requests-bound. Resolved by removing the old pod so the pending one could bind.

**Outage: ~100 seconds** (19:31:29Z → 19:33:01Z), reads only, no writes to lose.

**Verification after cutover:**

- All six forum URLs return 200 with correct titles.
- `start.rb` logged `multisite: wrote config/multisite.yml (2 extra site(s): tianpan, blockeden)`.
- The pod's rendered config points blockeden at `…-rw.tea-d98210cbbpdc73dcrkvg.svc`, while tianpan still points at `default` — the expected mid-migration state.
- The relocated database shows 4 active connections from the application pod, proving traffic actually landed on it.

**Rollback still available:** the old `dpg-d9rs3ee0ccis738kc7c0` in `default` is untouched and retains its data. Reverting is two env vars and a redeploy. Step 9 (retiring it) is deferred until a soak period passes.

### Carry-forward for the remaining three

- Create the extension set **before** restoring: compare `pg_extension` between source and destination first. bex provisions only `pg_stat_statements`; anything the tenant added by hand (here `vector`) must be recreated or the restore silently drops the dependent tables.
- Compare **schema objects**, not just row counts. Row counts matched perfectly while three tables were missing.
- Dump as the application role over **TCP**, not the socket.
- Expect the rollout to need the old pod removed first until P6 is addressed.

## Execution — datastore 2 of 4: `tianpan-forum-db`

Same method. Final state matches the source on every axis: `tables=295 idx=928 seqs=271 fks=15 exts=6 posts=18324 topics=4168 users=190`, all 295 tables owned by the application role.

This one took three attempts, and each failure is worth recording because none of them was visible from row counts alone.

**A truncated dump that looked valid.** The first dump was taken while the source instance was being restarted by kubelet (below), so `pg_dump` was cut off mid-stream. The file still had a correct `PGDMP` header and a plausible 73 MB size — only `pg_restore` revealed it, with `could not read from input file: end of file` after loading 44 of 295 tables. A complete dump of the same database is **111 MB**. Header-and-size checks are not integrity checks; the reliable one is `pg_restore -l | grep -c "TABLE DATA"` against the source's table count (295).

**Streaming large files through `kubectl exec` silently truncates.** Retrieving the 111 MB dump via `exec … cat >` produced 109,105,147 bytes locally against 111,070,519 in the pod — byte counts matched between local and the _next_ hop, so only an md5 against the **source** caught it. `kubectl cp` (tar-based) transferred it intact, verified by identical md5 at all three locations. Use `kubectl cp`, and checksum against the origin.

**The source database restarted mid-migration, and it was not caused by this work.** The `postgres` container of `dpg-d9rrkoc4h4mc73edurp0` had been failing its readiness probe intermittently for **16 hours** (`Readiness probe failed: Get "https://…:8000/readyz": context deadline exceeded`, x10 over 16h) before kubelet finally restarted it. During the shutdown the database reported `FATAL: the database system is shutting down`, which is what truncated the first dump. It recovered on its own (`2/2 Running`, `posts=18324` intact) and the site stayed up throughout. The instance-manager HTTP endpoint on :8000 — which serves `/readyz` and the metrics the collector scrapes — was timing out on TLS handshakes; worth its own investigation, since the same symptom would eventually restart any instance.

**Also relearned:** `DROP SCHEMA public CASCADE` drops `pg_stat_statements` along with everything else, so re-create the full extension set after a reset, not just the ones the restore needs.

### A self-inflicted outage worth recording honestly

The rollout again needed the old pod removed for the new one to schedule (P6, as with datastore 1). This time the new pod was **not yet Ready** when the old one was deleted, and both were briefly unavailable: `beancount.io` served **503 for roughly 5 minutes**.

The correct sequence is to wait for the replacement to report `Ready` and only then free the old pod — or, better, to stop working around P6 by hand. It resolved without intervention once the pending pod bound.

**Separately, the repo-triggered build path now fails.** Restoring the service's `repo` binding (finding `.pm/w7/038.md`) makes bex attempt a clone, and the clone has no credential:

```
remote: Invalid username or token. Password authentication is not supported for Git operations.
fatal: Authentication failed for 'https://github.com/bex-co/discourse_docker/'
```

A private repo needs a GitHub App installation token, and restoring `repo` through the API did not attach a git connection. Deploys still work through the explicit `POST /v1/services/{id}/deploys` with a `commitId`, which is how both images were built. The service is therefore back to "autoDeploy claims a behaviour it cannot perform" — the same defect `038` describes, now with a different cause. **Recorded as a regression introduced by this work**; either attach the connection or clear `repo` again.

## Execution — datastore 3 of 4: `red-d9p49kdrtmes73c34ovg` (Valkey)

Straightforward and already complete by the time the primary was reached: the `<ws>` CR is `Ready`, its `uri` Secret is owned by the KeyValue CR and resolves in-namespace (`red-…tea-d98210cbbpdc73dcrkvg.svc:6379`), and the App reads it through `secretKeyRef` — so no env edit was needed. The `default` instance has served no client since.

## Execution — datastore 4 of 4: `beancount-forum-db` (the primary, `beancount.io`)

The highest-risk one, and the only cutover in this drill with **zero data loss and no unplanned downtime**.

Applying the carry-forward list worked exactly as written:

| check | source | restored |
| --- | --- | --- |
| tables / indexes / sequences / FKs | 292 / 924 / 271 / 15 | **identical** |
| extensions | 5 (`hstore pg_stat_statements pg_trgm plpgsql unaccent`) | **identical** — created before the restore |
| posts / topics / users | 6231 / 1919 / 134 | **identical** |
| table ownership | — | all 292 owned by the application role |

`pg_restore -l | grep -c "TABLE DATA"` gave 292 against the source's 292 tables before the transfer; `kubectl cp` moved the 44,040,246-byte dump with md5 `ef9bd805…` identical at source, laptop, and destination. The four `pg_restore` errors were all `COMMENT ON EXTENSION` (extensions are owned by superuser) and touch no data.

Drift was measured rather than assumed: max ids and last-post timestamps were identical between source and copy at cutover time (last post 2026-08-01), so the window cost nothing.

**The rollout was clean this time.** `maxUnavailable: 25%` of one replica rounds _down_ to zero, so the Deployment brings the replacement up before retiring the old pod — the P6 workaround from datastores 1 and 2 was never needed once there was somewhere for the new pod to land. `beancount.io/forum` served 200 on every poll except a single 502 at the instant the endpoint swapped.

### Capacity was the real obstacle, not the data

The replacement Pod sat `Pending` for ten minutes: `0/10 nodes are available: 1 Insufficient cpu, 2 Insufficient memory, 8 node(s) had untolerated taint(s)`, with the autoscaler declining to help — `pod didn't trigger scale-up: 1 node(s) had untolerated taint(s), 3 max node group size reached`. Running the old and new instance of every datastore side by side had filled the serving pool (`bex-tenant-0` at max 2, both nodes ≥99% CPU / ≥81% memory), and `bex-tenant-burst` is tainted for builds and cannot absorb serving Pods.

Raising `bex-tenant-0`'s autoscaler ceiling to 3 gave the rollout somewhere to go. **That ceiling is still raised** and must go back to 2 once the retired instances are deleted — it is billing until then. The runbook now carries this as a precondition rather than a discovery.

An attempt to free capacity by hibernating the two idle old clusters (`cnpg.io/hibernation: "on"`) is worth recording as a dead end: the bex Database controller owns the CNPG Cluster and rewrote the annotation away within a reconcile. That is the m84 convergence behaviour working as designed, but it means **there is no operator-facing way to pause a bex-managed Postgres** — no `spec.suspend` on the `Database` CR.

### The finding this cutover produced: silent backup loss on a same-name migration

Verifying the archive after cutover — not merely that the CR was `Ready` — surfaced the defect written up as [`.pm/w7/040.md`](../../.pm/w7/040.md): all three migrated Postgres clusters read `ContinuousArchiving=False` with `WAL archive check failed … Expected empty archive`. Because bex derived the barman `serverName` from the CR name alone, each new cluster collided with its old same-named twin's archive prefix, and barman fails closed. Three production databases had been serving with **no archived WAL and no base backup**, WAL piling up at 1.3 GB / 913 MB / 353 MB against 9.8 GB volumes.

Worse, the _next_ step of this runbook would have compounded it: the delete-time purge Job removes S3 prefixes recursively keyed on the same bare name, so retiring the old instance would have erased the live one's archive too. Defect 1 masked defect 2 — the collision that broke archiving is the only reason the purge would have found nothing to destroy.

Mitigated in place by repointing all three at workspace-scoped prefixes through `status.backupServerName` (`tea-d98210cbbpdc73dcrkvg-dpg-…`), which required nudging the owned Cluster because a status-only write does not trigger the generation-based `Database` watch. All three then reported `ContinuousArchiving=True`, a fresh base backup completed on each, and WAL began draining. The fix in the repo derives exactly those names, so the deploy is a no-op for these three.

## Final state

|  | old (`default`) | new (`<ws>`) |
| --- | --- | --- |
| `dpg-d9nqg95cavls73fp8m20` (beancount) | idle, 0 clients | **live**, archiving, backed up |
| `dpg-d9rrkoc4h4mc73edurp0` (tianpan) | idle, 0 clients | **live**, archiving, backed up |
| `dpg-d9rs3ee0ccis738kc7c0` (blockeden) | idle, 0 clients | **live**, archiving, backed up |
| `red-d9p49kdrtmes73c34ovg` | idle | **live** |

All three forums serve 200 with real content from the new instances (`beancount.io`, `tianpan.co`, `blockeden.xyz`). Steps 1–8 are complete for all four datastores.

## Open — deliberately not done

- **Step 9 (retire the old instances) is held for a soak.** It is irreversible: the purge takes the S3 archive with it, so the old instances are today the only independent copy of each forum. They are idle and cost only their reserved capacity. Retiring them is also what releases the raised autoscaler ceiling, so the two should be done together.
- **The `repo` binding regression** from datastore 2 is unresolved: `autoDeploy` is set on a service whose clone has no credential. Either attach a git connection or clear `repo`.
- ~~**No alert covers `ContinuousArchiving=False`.**~~ Closed in the same pass: `bex_datastore_wal_archiving` plus the `DatabaseNotArchivingWAL` alert now page on a Ready database that is archiving nothing. Not yet deployed — it ships with the `w7/040` fix.

## Step 8 — hand-built artifacts removed

Removal is the proof, so it was done incrementally with the three forums polled after each batch. Everything was captured to a local backup first (outside the repo — the Secrets carry credentials).

| artifact | verdict |
| --- | --- |
| CNP `allow-blockeden-forum-datastores`, `allow-tianpan-forum-datastores` | removed — debris for Apps that no longer exist |
| Secrets `red-d9rrkoc4h4mc73edurpg`, `red-d9rs3ee0ccis738kc7cg` | removed — connection Secrets minted for two Key Value instances that were never provisioned (Discourse multisite shares one Valkey), referenced by nothing |
| Secret `beancount-multisite-yml` | removed — inert; the image now assembles `multisite.yml` from platform env |
| Secrets `reg-pull-…-blockeden-forum`, `reg-pull-…-tianpan-forum` | removed — pull credentials for deleted Apps |
| CNP `allow-beancount-forum-multisite-dbs`, NP `allow-beancount-forum-data-egress` | removed — both allowed egress into `default`, which co-location makes unnecessary; `allow-same-namespace` now carries the traffic |
| NP `allow-datastore-control-ingress` | **KEPT — it is not a hand-built artifact.** It appears in all 20 tenant namespaces, so it is the platform's own ADR043 D8.3 policy. Checked before deleting, because the name reads exactly like the hand-made ones beside it |
| The three Ingresses and their TLS Secrets | **KEPT**, per the 2026-08-09 correction — they are the only routing |

After the last removal, all three sites still served real content (30 topics each) with live connections on all three databases (5 / 4 / 2 clients) — through the platform's own policy, which is what Step 8 exists to demonstrate.

### One residual the adoption left behind

The primary's `dpg-…-app` Secret was a hand-copy that CNPG **adopted** rather than replaced: it bootstrapped the new cluster from the existing credentials, so the password worked, but it never rewrote the object. It therefore carried no owner reference, none of the `cnpg.io/*` labels, and — the part that mattered — `uri`, `jdbc-uri`, `fqdn-uri` and `fqdn-jdbc-uri` all still named `.default`. Only `host` had been corrected, which is the single key the App reads, so nothing failed; any consumer reading a URI would have been sent to the old namespace.

Aligned with its two correctly-owned siblings: the four URI keys rewritten to the tenant namespace, the CNPG labels restored, and the controller owner reference set to the live Cluster so deletion garbage-collects it like any other. Verified the Secret survived and all three sites stayed up.

### Final state, verified

- No hand-made NetworkPolicy or CiliumNetworkPolicy remains — only the six platform policies plus the two operator-owned Key Value backup policies.
- No datastore Secret without an owner remains.
- All five datastore CRs are `Ready` in `<ws>`; the quota charges `count/databases.app.bex.co = 3`, `count/keyvalues.app.bex.co = 2` — the `w3/010` gap, closed by observation.
- All three Postgres clusters report `ContinuousArchiving=True` under workspace-scoped archive prefixes.
