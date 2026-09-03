# Runbook: move a workspace's managed datastores into its tenant namespace

**Status:** written 2026-08-08 (`w7/m77/t001`), executed by `w7/m77/t007`. **Applies to:** any workspace whose `Database`/`KeyValue` CRs still live in the shared apps namespace (`BEX_API_NAMESPACE`, `default` on hetzner-prod) while its Apps live in `<ws>`.

## Why this exists

[ADR043 D8](../ADR043-tenant-namespace-isolation.md) moves datastores into `<ws>`. The code change fixes **new** resources only. An existing App in `<ws>` whose datastore is still in `default` stays broken — its `secretKeyRef` cannot resolve across namespaces, its egress to the datastore is denied by the tenant default-deny, and the CNPG short-name host does not resolve. This runbook is what fixes those tenants.

A datastore CR cannot be moved in place: its PVC is namespace-bound. The cutover therefore **recreates the CR under its existing name** in `<ws>` and restores its data. Preserving the name is not cosmetic — the App's injected `secretKeyRef` points at `<dpg-id>-app`, so a same-name recreate keeps every App env var valid with no App-side edit and no redeploy of the consuming service beyond the restart that picks up the new Secret.

## Before you start

- [ ] ADR043 D8's code changes are deployed (`w7/m77` t003–t006) — otherwise the recreated CR lands back in the shared namespace.
- [ ] **Provisioning in a tenant namespace actually works.** Two RBAC defects blocked it until 2026-08-19, both invisible to every test because RBAC is only exercised on a live cluster, and both found by this runbook's own rehearsal step. Do not take them on faith — prove it, which is one command:

      ```
      kubectl auth can-i get objectstores.barmancloud.cnpg.io \
        --as=system:serviceaccount:bex-system:bex-controller-manager -n <ws>   # operator reads the source ObjectStore
      kubectl auth can-i get secrets \
        --as=system:serviceaccount:cnpg-system:plugin-barman-cloud -n <ws>     # plugin can DELEGATE to its per-cluster Role
      ```

      Both must answer `yes`. The second is the non-obvious one: the plugin never reads a
      Secret itself, but Kubernetes requires it to HOLD that permission to create the
      per-cluster Role that grants it (`.pm/w7/036.md`).

- [ ] You have a **rehearsal** result: this procedure has been run end to end on a scratch workspace, not just read. Do not rehearse on a tenant.

      A scratch **Database in the target tenant namespace** is a better rehearsal than a
      scratch workspace: same blast radius (one throwaway CR), higher fidelity (the exact
      namespace, quota, and network policy the real move lands in). Assert it reaches `Ready`
      **and** that all five of these appear, because a Cluster can look healthy while its
      backup wiring silently did not land:

      | check | in `<ws>` |
      | --- | --- |
      | connection Secret | `<id>-app` |
      | per-cluster barman Role | `<id>-barman-cloud` |
      | projected ObjectStore | `bex-tenant-postgres` |
      | projected backup credential | the `BEX_DB_BACKUP_S3_SECRET` name |
      | quota charged | `count/databases.app.bex.co` incremented |

- [ ] **The cluster has headroom for one extra copy of everything you are moving.** The old and new instances run side by side from Step 5 until Step 9, and the service's own rolling update needs a second Pod on top of that. On `hetzner-prod` this exhausted the serving node group mid-cutover: the replacement Pod sat `Pending` with `0/10 nodes are available` while the autoscaler reported `3 max node group size reached`, because the build/burst pool is tainted and cannot absorb serving work.

      Raise the serving group's ceiling **before** Step 5 rather than discovering it at Step 7, and put it back after Step 9:

      ```
      kubectl -n bex-capi annotate machinedeployment bex-tenant-0 \
        cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size=3 --overwrite
      ```

      Note the ceiling you raised in the Step 10 record — it is billing until it is reverted.

- [ ] You know each affected tenant's maintenance tolerance. This procedure takes a **write outage** for the duration of the final dump + restore. For a tenant that cannot take one, see [Zero-downtime variant](#zero-downtime-variant).
- [ ] Never print or commit `.env` or `*.kubeconfig` contents.

Inventory the affected set first — datastore CRs in the shared namespace whose owning workspace has a tenant namespace:

```
kubectl -n default get databases.app.bex.co,keyvalues.app.bex.co \
  -L app.bex.co/workspace
```

Work one workspace at a time, and within a workspace one datastore at a time.

## Procedure (per datastore)

Throughout: `<ws>` is the tenant namespace, `<id>` is the CR name (`dpg-…` / `red-…`), `<svc>` is the consuming service.

### Step 1 — Capture a verified restore point

Take a backup and **prove it restores** — do not accept "a backup object exists".

- Postgres: confirm a completed base backup plus WAL continuity, then restore it into a scratch cluster and check the row counts of the tenant's largest tables against the source.
- Key Value: confirm a completed RDB snapshot and load it into a scratch Valkey.

**Rollback:** nothing has changed yet. If the restore point cannot be verified, stop — every later step depends on it.

### Step 2 — Record the exact current state

Save, for the tenant's own records and for rollback:

- the datastore CR YAML (`kubectl -n default get database <id> -o yaml`),
- the connection Secret's keys (names only — never the values),
- the consuming App's env references (`kubectl -n <ws> get app <svc> -o yaml`),
- which manual workaround artifacts exist for this tenant (see [Manual artifacts](#manual-artifacts)).

**Rollback:** none needed; this step only reads.

### Step 3 — Quiesce writes

Suspend the consuming service so no writes land during the copy. Suspension keeps the Service, Ingress, and TLS in place (`docs/ADR007-restart-suspend-and-resume.md`), so resume is just scaling back.

Confirm the datastore has no remaining client connections before continuing.

**Rollback:** resume the service. The old datastore is still live and authoritative — this is the last fully reversible step.

### Step 4 — Final dump

Take the final consistent export from the old instance, _after_ writes have stopped. This is the copy that carries the data — Step 1's backup is the safety net, not the transfer.

For Postgres, dump as the **application role**, not as a superuser. Restoring a superuser-owned dump leaves tables owned by `postgres` and the app user without privileges — that is exactly the `PG::InsufficientPrivilege` failure the 2026-08-08 incident hit during manual recovery, and it is avoidable here.

**Rollback:** resume the service; nothing downstream has been created.

### Step 5 — Create the new CR in `<ws>` under the same name

Create the `Database`/`KeyValue` with the **identical** `metadata.name` in `<ws>`, same plan and storage as recorded in Step 2. Wait for it to reach a ready state and confirm:

- the operator created the connection Secret in `<ws>`,
- the ObjectStore and backup credential are present in `<ws>` (ADR043 D8.4),
- the namespace `ResourceQuota` accepted it,
- for Postgres, the CNPG pods are healthy — a stall here is almost certainly a missing D8.3 network allow, not a datastore problem.
- **for Postgres, the new cluster is actually archiving.** This is the check the whole step turns on, and readiness does not imply it:

      ```
      kubectl -n <ws> get cluster.postgresql.cnpg.io <id> \
        -o jsonpath='{.status.conditions[?(@.type=="ContinuousArchiving")].status}{"\n"}'
      ```

      It must read `True`. `False` here means the new cluster's archive prefix collides with
      the old one's — the two share a name, and before the `w7/040` fix bex derived the barman
      `serverName` from the CR name alone. barman fails closed with
      `WAL archive check failed … Expected empty archive`, so the migrated database serves
      normally while backing up **nothing** and accumulating WAL until its volume fills.
      On a build that predates the fix, repoint it explicitly:

      ```
      kubectl -n <ws> patch database.app.bex.co <id> --subresource=status --type=merge \
        -p '{"status":{"backupServerName":"<ws>-<id>"}}'
      # a status-only write does not trigger a reconcile — touch the owned Cluster to drive one
      kubectl -n <ws> annotate cluster.postgresql.cnpg.io <id> bex.co/nudge="$(date +%s)" --overwrite
      ```

      Then take a base backup and confirm it reaches `completed` before you go on. A database
      whose archive you have not seen written is a database with no restore point.

**Rollback:** delete the new CR. The old one is untouched and still holds the data; resume the service.

### Step 6 — Restore into the new instance

Load Step 4's dump. Then verify against the source, not against expectations: compare row counts for the tenant's largest tables, and confirm object ownership is the application role.

**Rollback:** delete the new CR and resume the service against the old one.

### Step 7 — Cut the service over

Trigger a redeploy of `<svc>` so its pods pick up the `<ws>` connection Secret. Because the CR name is unchanged, the App's env references need no edit.

> **Only true for datastores the platform wired.** An App reaches a platform-injected datastore through `secretKeyRef` env, which is name-based and survives the move. Any datastore the tenant wired **itself** — a connection string baked into an image, a config file, a hand-written env value — carries the namespace in its FQDN (`<id>-rw.<old-ns>.svc.cluster.local`) and breaks the moment the move lands. Find those before Step 3, not after Step 7.
>
> Real example (`w7/m77`): a Discourse multisite App reached its primary database through platform env, and two further databases through a `config/multisite.yml` **inside its own image**. Nothing in the App CR referenced them; only a hand-written CiliumNetworkPolicy hinted they existed. Grep the tenant's image and config for `.svc.cluster.local` and for each datastore id, and fold the required edit into the same window.

Verify from inside the pod, in this order — each check is the one that was invisible in the incident:

1. the pod starts (no `CreateContainerConfigError`) — the Secret resolved in-namespace;
2. it connects to Postgres and to Valkey — the network path is allowed;
3. the hostname in its env resolves — no `could not translate host name`;
4. the site serves real content, not just a health check.

**Rollback:** this is the point of no return for _writes_ — any write after cutover exists only in the new instance. To roll back, quiesce again, dump the new instance, restore into the old one, and redeploy. Keep the old CR until Step 9 so this stays possible.

### Step 8 — Remove the manual workaround artifacts

For tenants repaired by hand on 2026-08-08, remove the hand-built reconcile artifacts and confirm nothing regresses. Their removal **is** the proof the platform path works — leaving them in place hides whether the fix actually took.

<a id="manual-artifacts"></a>

| Artifact | Where | Notes |
| --- | --- | --- |
| Copied `dpg-<id>-app` / `red-<id>` Secrets, with hand-patched FQDN `host` | `<ws>` | superseded by the operator-created Secret |
| Hand-written `CiliumNetworkPolicy` (forum → keyvalue :6379, cnpg :5432) | `<ws>` | superseded by `allow-same-namespace` |
| Hand-made Traefik `Ingress` for `<svc>.onbex.co` | `<ws>` | **KEEP — do not remove.** See the note below |

> **Correction (2026-08-09, from `w7/m78/t001`; posture update 2026-08-18).** Removing the hand-made Ingresses was deferred to `w7/m78` on the assumption that a fix there would make the platform recreate them. **It will not** recreate them for services that have no effective public host. Production **keeps** `BEX_BASE_DOMAIN=onbex.co` (shared hosting enabled; cross-tenant cookie risk accepted before open signup — [`.pm/DO_NOT_DO.md` `#PSL`](../../.pm/DO_NOT_DO.md)); the earlier "unset the base domain" remediation is not authorized. A service still ends up with no platform Ingress when `renderSubdomainPolicy` is disabled and it has no custom domains (or an installation has no base domain configured).
>
> Those Ingresses may still be the **only** thing routing `tianpan-forum` and `blockeden-forum` if those services lack a healthy operator-owned route — removing them without verifying platform/custom routing takes the forums offline. Sibling-cookie exposure on `onbex.co` is the accepted `#PSL` residual. Resolving hand-made Ingress debt is a product/ops decision (attach real custom domains, or rely on the restored platform subdomain), not a cleanup step that unsets `BEX_BASE_DOMAIN`.

**Rollback:** re-apply the artifact from the Step 2 record.

### Step 9 — Retire the old datastore

Only after the tenant has served correctly for an agreed soak period. Delete the old CR and let the finalizer tear down its Cluster/StatefulSet, PVCs, Secrets, Services, and backup prefix (`w7/m12`).

> **Confirm the new database's archive prefix differs from the old one's first.** The purge Job deletes S3 **prefixes**, recursively. Before the `w7/040` fix it derived them from the bare CR name — which both instances share — so this step would have deleted the _live_ database's archive along with the retired one's. Compare them and refuse to proceed if they match:
>
> ```
> kubectl -n default get database.app.bex.co <id> -o jsonpath='{.status.backupServerName}{"\n"}'
> kubectl -n <ws>    get database.app.bex.co <id> -o jsonpath='{.status.backupServerName}{"\n"}'
> ```

> **KeyValue is different — its backup prefix NEVER differs across a cutover.** `keyvalue/<id>/` is keyed by the CR name alone (ids are globally unique, so namespaces never collide — but a migration reuses the id by design), and the 2026-08-21/22 cutover's unguarded deletes purged the recreated instances' recovery points along with the retired CRs' (see [2026-08-25-backup-verification.md](../drills/2026-08-25-backup-verification.md)). **Annotate the old KeyValue CR before deleting it** so the finalizer skips the prefix purge; every other teardown (CronJob, Jobs, StatefulSet, PVCs, Secrets) still runs:
>
> ```
> kubectl -n default annotate keyvalue.app.bex.co <id> app.bex.co/preserve-backups-on-delete=true
> kubectl -n default delete keyvalue.app.bex.co <id>
> ```
>
> Never carry this annotation on a real tenant delete — purge-on-delete is the ADR021/ADR031 privacy contract.

**Rollback:** none — this is irreversible. The Step 1 restore point is the only remaining recourse, so do not run this step until the soak has passed.

### Step 10 — Record it

Write a drill record under `docs/drills/` naming what was done per tenant, what deviated from this runbook, and the verified outcome. Include any autoscaler ceiling raised in _Before you start_, and revert it once the retired instances are gone.

## Zero-downtime variant

For a tenant that cannot take a write outage, replace Steps 3–7 with logical replication: create the new instance, replicate from the old one, wait for the lag to reach zero, then cut over with only a brief pause for the final flush.

This is genuinely more complex and has more failure modes than a maintenance window. Prefer the window unless the tenant's constraint is real — a forum with a handful of active users is not that tenant.

## Failure modes to expect

| Symptom during cutover | Almost certainly |
| --- | --- |
| CNPG pods stall in bootstrap in `<ws>` | a missing D8.3 allow — the kube-apiserver egress or `cnpg-system` ingress. Same shape as w7/m33 |
| New CR is rejected on create | the namespace `ResourceQuota` — the datastore dimensions bite now (ADR043 D8, `w7/m77/t006`) |
| Operator errors reading/writing the connection Secret | Secret access still on the cached client — ADR043 D8.2 |
| Restored tables owned by `postgres`, app user denied | dumped as superuser — redo Step 4 as the application role |
| Backups silently absent on the new instance | ObjectStore or credential missing in `<ws>` — ADR043 D8.4 |
