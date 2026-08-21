# Datastore namespace cutover — pre-window record — 2026-08-21

**Scope:** `w7/m77/t007`, production application cluster (hetzner-prod), workspace `tea-d98210cbbpdc73dcrkvg`.

**Outcome:** PREPARATION COMPLETE — every runbook prerequisite and every non-destructive step is done and verified. Steps 3–7 (the write outage) have **not** been executed; they need an agreed maintenance window because they are irreversible for writes and affect three live forums.

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

**Keep — do not remove:** the Ingresses `…-beancount-forum-ms`, `…-tianpan-forum`, `…-blockeden-forum`. Per the 2026-08-09 correction they are the only routing for those hosts; production runs with `BEX_BASE_DOMAIN` unset by deliberate security decision.

**Excluded from the migration set:** `red-da4086iii7bs73drbqh0` (`blockeden-forum-redis`, plan standard) was created independently on 2026-08-21 and is **already** in the tenant namespace, `Ready`. It needs no cutover, and is incidental evidence that provisioning into `<ws>` now works.

**Quota headroom in the target namespace:** `count/databases 0/25`, `count/keyvalues 1/25`, `persistentvolumeclaims 1/200`, `requests.storage 10Gi/5Ti`. Ample. Note that `databases` reads 0 precisely because the three still live in `default` — the w3/010 gap this cutover closes.

## Multisite: resolved before the window, not during it

The runbook's Step 7 assumption ("the App's env references need no edit") holds only for platform-wired datastores. This App reached two of its three databases through a `config/multisite.yml` **inside its own image**, carrying hardcoded `.default.svc` FQDNs — and real credentials.

That is now fixed ahead of the window: the image ships a placeholder-only template, and `start.rb` resolves it at boot from platform-injected env. Verified on `gen-105` — zero credentials in the image, and all six forum URLs serving 200. Because the hosts now arrive as env, the namespace change at cutover is an env edit rather than an image rebuild.

## Remaining: Steps 3–7, requiring a maintenance window

Not executed. These suspend the service, take the final dump, recreate each CR under the same name in `<ws>`, restore, and cut over. Step 7 is the point of no return for writes.

The three affected sites — `beancount.io`, `tianpan.co`, `blockeden.xyz` (plus their `.onbex.co` hostnames) — take a write outage for the duration. The runbook requires knowing each tenant's maintenance tolerance before starting, and offers a logical-replication variant for a tenant that cannot take one, while advising against it for a low-traffic forum.

When the window is agreed, execute Steps 3–10 and append the per-tenant results, any deviation from the runbook, and the post-cutover row counts to this record.
