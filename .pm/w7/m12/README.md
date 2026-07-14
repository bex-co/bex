# w7 · m12 — Delete really deletes: purge orphaned tenant artifacts on service/Postgres/Key Value deletion

**Worker:** worker7 **Goal:** deleting a service, a managed Postgres, or a managed Key Value destroys _all_ of its tenant data and platform artifacts — not just the CR and its ownerRef children. **Status:** todo

The delete **verbs** shipped long ago (w2/m4: REST/GraphQL/MCP; w5/m14: dashboard danger-zone — all ✅ in `docs/ADR018-render-parity.md`), but a code survey (2026-07-14) found the teardown leaks tenant credentials and data on every delete:

| Orphan | Left behind by | Why the cascade misses it |
| --- | --- | --- |
| OpenBao env vars + secret files (and the materialized `<app>-env` Secret) | service delete | `apps.Service.Delete` never calls the secrets feature; only the **workspace** purger (`secrets.WorkspacePurger`) reaches OpenBao |
| Build Jobs + kpack Images in `BEX_BUILD_NAMESPACE` | service delete | cross-namespace ownerReferences are invalid in k8s |
| Built tenant images in Zot | service delete | nothing ever deletes registry manifests |
| Static-site objects under `<appID>/…` in the S3 bucket | static_site delete | `publish.go` only `--delete`s within a revision prefix |
| cert-manager TLS Secrets for custom hosts | service delete | documented orphan (`apps/service.go` comment, ADR006) |
| Valkey StatefulSet PVCs | Key Value delete | StatefulSet PVCs are not GC'd with the StatefulSet by default |
| CNPG object-store backups/WAL | Postgres delete | barman object-store data outlives the Cluster CR |

## Tasks (in order)

| id   | title                                                                           | est | depends_on                               |
| ---- | ------------------------------------------------------------------------------- | --- | ---------------------------------------- |
| t001 | Service delete: purge per-app OpenBao env vars + secret files                    | 45m | —                                        |
| t002 | App finalizer: delete-time teardown hook + build-artifact GC in build namespace  | 60m | —                                        |
| t003 | App delete: remove the app's built images from Zot (manifest delete + GC)        | 60m | t002                                     |
| t004 | Static-site delete: purge the app's object-store prefix                          | 45m | t002                                     |
| t005 | App delete: remove orphaned cert-manager TLS Secrets for app hosts               | 30m | t002                                     |
| t006 | Key Value delete: PVCs go with the StatefulSet (retention policy)                | 30m | —                                        |
| t007 | Postgres delete: decide + implement CNPG object-store backup retention           | 45m | —                                        |
| t008 | Delete-cascade acceptance: zero-leftover audit across all three types            | 45m | t001, t003, t004, t005, t006, t007       |
| t009 | Render parity: delete semantics across REST/GraphQL/MCP/UI                       | 30m | t008                                     |
| t010 | Simplify the changed code                                                        | 30m | t009                                     |
| t011 | Test coverage for delete-time teardown                                           | 45m | t009                                     |
| t012 | Closeout                                                                         | 15m | t010, t011                               |

## Definition of done

On the local mock cluster, after deleting (a) a repo-built service with env vars, a custom host, and at least one completed build, (b) a static site, (c) a managed Postgres, and (d) a managed Key Value — each via its public delete verb — an audit finds **zero leftovers**: no build Jobs or kpack Images for the app in `BEX_BUILD_NAMESPACE`, no repo for the app in Zot, no objects under the app's static-site S3 prefix, no OpenBao env-var/secret-file entries and no `<app>-env` or host TLS Secrets in the apps namespace, no PVCs from the Key Value's StatefulSet, and CNPG object-store backups handled per the recorded retention decision. The t008 acceptance script proves this end-to-end and is repeatable.

## Source + Goal linkage

- **Source:** `/pm service deletion across all service, db, key value types for w7` (2026-07-14) + same-day code survey of the delete paths (`lego/backend/internal/{apps,postgres,keyvalue}/service.go`, `lego/operator/internal/controller/*_controller.go`). Gap analysis vs existing work: the verbs are done (w2/m4, w5/m14) — this milestone is exclusively the delete-time **teardown** those milestones left leaking; no surface/verb work is repeated.
- **Goal linkage:** GOAL.md V0 #1 ("… Suspend. Delete. Create.") — delete only counts as done when it destroys the data; and w7's security-hardening charter (ADR028 hygiene): tenant credentials (OpenBao secrets, TLS keys) and tenant data (images, static content, PVCs, backups) persisting after deletion is a data-retention hole, and unbounded orphan accumulation (registry blobs, S3 objects, PVC disk) is a storage-abuse vector against the dense bin-pack economics.
- **Expected outcome:** deleting any of the three resource types leaves no tenant credential, data artifact, or storage residue anywhere on the platform; the acceptance audit is a standing regression harness for future resource types.
- **Why now:** every delete in dev already orphans artifacts today; the holes compound silently and are far cheaper to close before real tenants exist than to retro-clean after (same "before real tenants" sequencing as w7/m1–m2). w2/m4's delete-cascade acceptance only checked ownerRef children, so nothing currently guards against regression here.
- **Render parity included** (t009): delete is a tenant-facing verb on all four surfaces; Render documents deletion as irreversible data destruction — bex must match that semantics, and the ledger's delete rows need re-verifying once teardown actually destroys data.
