# w1 · m56 — Exit deprecated platform versions and fleet migration scaffolding

**Worker:** worker1 **Goal:** Move production off end-of-life Kubernetes and CNPG's retiring in-tree Barman path, then remove one-time fleet backfills, legacy route cleanup, and Node 20 CI actions only after their live-state gates are satisfied. **Status:** in progress (t001–t009 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Production preflight: inventory deprecated state, backups, and rollback gates — **DONE** | 45m | — |
| t002 | Upgrade the CAPI Kubernetes fleet from EOL 1.31 to a supported release — **DONE** | 60m | t001 |
| t003 | Upgrade kpack and remove the Kubernetes 1.31 compatibility override — **DONE** | 45m | t002 |
| t004 | Install the Barman Cloud plugin and declare ObjectStore resources — **DONE** | 45m | t002 |
| t005 | Migrate operator-managed tenant Postgres backup and recovery to the plugin — **DONE** | 60m | t004 |
| t006 | Migrate the GitOps bex-db backup and recovery path to the plugin — **DONE** | 45m | t004 |
| t007 | Prove tenant and control-plane backup, PITR, and restore drills on the plugin — **DONE** | 60m | t005, t006 |
| t008 | Remove in-tree barmanObjectStore code, manifests, guards, and stale runbook instructions — **DONE** | 45m | t007 |
| t009 | Normalize legacy datastore names and IP allowlists; remove their migration fallbacks — **DONE** | 60m | t001 |
| t010 | Normalize build/release metadata; remove artifact-adoption and fingerprint backfills | 45m | t001 |
| t011 | Retire old Traefik datastore routes and recurring legacy load-balancer cleanup | 45m | t001 |
| t012 | Upgrade GitHub Actions to Node 24-compatible maintained majors | 30m | — |
| t013 | Simplify — run /simplify over the changed platform code | 20m | t003, t008, t009, t010, t011, t012 |
| t014 | Test coverage — upgrade, migration, restore, and absence guards | 45m | t003, t008, t009, t010, t011, t012 |
| t015 | Closeout — verify DoD, mark done, move milestone | 15m | t013, t014 |

## Definition of done

The production CAPI templates and live cluster run a Kubernetes release supported by both Kubernetes and the deployed CNPG/kpack stack, and the temporary kpack 1.31 override is absent. Tenant databases and bex-db archive WAL and take scheduled backups through the Barman Cloud plugin; fresh backup plus point-in-time restore drills recover known data for both paths before all in-tree barmanObjectStore fields and code are removed. Production inventory contains no legacy datastore name/IP shapes, unlabeled build artifacts, missing release fingerprints, old Traefik datastore routes, or legacy per-server load-balancer targets, and the corresponding ongoing migration scaffolding is gone. CI workflows use maintained Node 24-compatible action majors. GitOps validation, operator/backend tests, restore evidence, and rollout health are green.

## Source + Goal linkage

- **Source:** User-requested deprecated-code audit on 2026-07-27. It found production CAPI templates pinned to Kubernetes 1.31, CNPG 1.30 still accepting the in-tree Barman API scheduled for removal in 1.31, one-time controller/script cleanup paths, and Node 20 GitHub Actions.
- **Goal linkage:** GOAL.md #1, #4, #6, and #7: reliable stateless and Postgres hosting, safe physical-cluster lifecycle, and security maintenance on supported foundations.
- **Expected outcome:** supported cluster and CI runtimes, plugin-based restorable PostgreSQL backups before CNPG removes the old API, and controllers/workflows that reconcile only current fleet shapes instead of carrying permanent one-time migrations.
- **Why now:** Kubernetes 1.31 reached end of life on 2025-11-11, and CNPG 1.30 is the final migration window before the in-tree Barman removal planned for 1.31. Deferring either turns cleanup into an outage-prone forced upgrade.
- **Render parity task omitted:** this milestone changes platform GitOps, operator-internal migration mechanics, backups, and CI only; it does not change REST, GraphQL, MCP, or dashboard contracts.
