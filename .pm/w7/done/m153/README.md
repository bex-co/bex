# w7 · m153 — Repair analytics Secret ownership and root GitOps sync

**Worker:** worker7 **Goal:** Restore repeatable platform GitOps delivery without breaking Grafana analytics authentication. **Status:** done

**Estimate:** 120m implementation; 190m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile bootstrap and SealedSecret credential ownership — **DONE** | 45m | — |
| t002 | Make analytics bootstrap and reconciliation repeatable — **DONE** | 45m | t001 |
| t003 | Verify root sync and analytics access — **DONE** | 30m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] The migration explicitly reconciles the database role, Secret data and sealed payload; ownership no longer conflicts and no credentials appear in logs or Git.
- [x] A second bootstrap and GitOps reconciliation produces no ownership error, password mismatch or unintended credential rotation.
- [x] bex-platform-prod is Synced/Healthy; analytics authentication works; any unrelated remaining sync blocker is resolved or explicitly prevents closeout.

- [x] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

## Source + Goal linkage

- **Source:** User-approved platform-log brainstorm, 2026-09-28 UTC (September 27 America/Denver); user requested all five in w7. Read-only collection at source revision 016391810: 342 container streams, 167 pods, 22 namespaces, plus Loki platform history. Requested prior 24 hours; rotation/deleted pods limit coverage. Raw local evidence: /tmp/bex-platform-log-audit/{manifest.json,pods.json,events.json,argo.json,loki-platform.jsonl}; temporary artifacts may expire, so the observed findings are preserved below and must be revalidated at execution.
- **Observed finding:** The 2026-09-28 UTC log audit found bex-platform-prod OutOfSync/Degraded with failed sync: Resource "grafana-product-analytics" already exists and is not managed by SealedSecret. Live Secret metadata had neither an owner reference nor the managed annotation. scripts/lib/analytics-bootstrap.sh:46 applies the same Secret imperatively.
- **Goal linkage:** ADR008 reliable self-hosted Render-compatible hosting, deterministic lifecycle state and platform operability. Restore repeatable platform GitOps delivery without breaking Grafana analytics authentication.
- **Expected outcome:** Restore repeatable platform GitOps delivery without breaking Grafana analytics authentication.
- **Why now:** The ownership conflict blocks root synchronization and has already required selective child-Application updates.
- **Existing work / deduplication:** Previously observed in w7/blocked/047.md; this milestone owns the independent Secret repair, not the build-cache trial decision. Keep that note linked to this repair rather than duplicating its trial work.
- **Render parity:** Omitted: platform infrastructure/observability only, with no REST/GraphQL/MCP/UI contract change.
- **Scheduling:** approved priority 1 of five in w7. Infrastructure rollout verification follows m153's root-sync repair; isolated implementation can proceed independently. No dependency on blocked w7 datastore/sandbox walkthroughs is inferred.

## Scope and evidence limits

This is planned work, not an implemented fix. The read-only audit did not authorize implementation during collection. Revalidate the deployed revision and failure before executing these tasks. Raw logs and credentials stay out of Git; preserve only sanitized observations. Do not reopen the accepted PSL finding, duplicate the known blockeden.xyz certificate work, or infer root causes from transient etcd/storage warnings.

## Evidence (2026-09-28 UTC, worker7)

**Production-verified** (hetzner-prod, admin kubeconfig via `scripts/fetch-app-kubeconfig.sh`):

- Revalidated at 06:29Z: `bex-platform-prod` OutOfSync/Degraded, sync to `f60757162` Failed after 5 retries. The **only** failing task was `SealedSecret monitoring/grafana-product-analytics` ("already exists and is not managed by SealedSecret"); it sits in an early wave, so the child `log-shipper`/`prometheus` Application updates never applied. Live Secret: no ownerReference, no managed annotation, last written by `kubectl-client-side-apply` 2026-09-11T21:32:11Z; SealedSecret committed in `1ec6bac1e` three minutes later.
- t001 authority decision: unsealed the committed payload with `kubeseal --recovery-unseal` (key read via process substitution, nothing written to disk) and compared SHA-256 per key only — `password` and `ca.crt` both **match** the live Secret, and the CA is the same certificate as current `bex-system/bex-db-ca`. Adopting the sealed payload therefore rotates nothing.
- Migration: annotated the live Secret `sealedsecrets.bitnami.com/managed=true`. The controller logs "update suppressed, no changes in spec" and does not adopt on annotation changes (a metadata-only nudge on the SealedSecret was tried and removed), so the controller pod was deleted; on startup it adopted the Secret (ownerReference `SealedSecret`, condition Synced=True at 06:35:01Z).
- Manual root sync of `main` at 06:35Z: **Succeeded**, `bex-platform-prod` Synced/Healthy; every Argo Application Synced/Healthy (log-shipper/prometheus now applied).
- Second bootstrap: `bash scripts/product-analytics-bootstrap.sh` (new code) rc=0, "kept SealedSecret-managed … unchanged", Secret resourceVersion unchanged, SealedSecret Synced=True, root still Synced/Healthy. Both `product-analytics` and `cli-analytics` Grafana datasource health = "Database Connection OK" (before and after, last 06:38Z); a `select count(*) from product_analytics.events` through `/api/ds/query` returned 200 with one frame.

**Implemented + locally verified** (t002/t005):

- `scripts/lib/analytics-bootstrap.sh`: SealedSecret-owned Secret is authoritative — never rewritten, role password follows it, CA drift fails closed before touching the database; an unowned Secret is applied then annotated adoptable; `SEAL_TO=<file>` writes the sealed desired Secret (kubeseal). `scripts/lib/secret-install.sh` gains `secret_manifest` (apply_secret now wraps it). Runbooks `product-analytics.md` / `cli-analytics.md` updated.
- `scripts/analytics-bootstrap.test.sh` (18 checks: fresh, unowned, sealed, CA drift, reseal, no credential in argv/output), wired into `scripts/gitops-validate.sh` and the gitops workflow paths. Mutation: against the pre-change bootstrap, 7 checks fail. `gitops-validate.sh` rc=0; `onbex-default-tls-secret`, `ssh-gateway-db-role`, `disk-snapshot-secret` tests (other `secret-install.sh` callers) pass.
- t004 simplify: inline self-review of the diff (the `/simplify` skill was not invoked); no further reduction found — the three custody branches are each exercised by the test.

**Limitation / follow-up:** CNPG renews `bex-db-ca` before 2026-10-08T23:06:42Z; both sealed analytics copies then go stale. Filed `w7/060`.
