# Runbook: workspace-scoped registry and static-prefix identity migration

**Status:** written 2026-08-18 (`w2/m75`). **Applies to:** every App whose Zot repository, htpasswd user, pull Secret, or static-site object prefix is still keyed on bare `metadata.name`. **Does not apply until the operator carrying [ADR074](../ADR074-workspace-scoped-artifact-identity.md) is deployed.**

This operationalizes ADR055 F2/F3's four-phase rollout. New labeled Apps already mint workspace-scoped identities. This runbook moves **existing** artifacts. It never runs itself.

## STOP — requires explicit operator authorization

**Do not execute Phase 2, 3, or 4 against live tenant data without an explicit operator (user) authorization in the change window.** Dry-run inventory (Phase 1) is read-only and may run without that authorization. Copy, tombstone, redeploy, and blob deletion all mutate tenant artifacts; a digest mismatch or a leftover baked-in image ref becomes ImagePullBackOff or an orphaned static site.

The gate is this paragraph. There is no `--force` that bypasses it. `registry-migrate` defaults to dry-run; `--apply` is the mutating switch and is still subordinate to this STOP.

## Before you start

- [ ] ADR074's operator is deployed (workspace-scoped mint + dual-read + `App.status.staticPrefix`).
- [ ] You have rehearsed Phase 2 `--apply` on a scratch App, not a tenant.
- [ ] `skopeo` and `aws` CLIs are on the operator workstation; kubeconfig context is the target cluster (`hetzner-prod` in production). Never print or commit `.env` or `*.kubeconfig` contents.
- [ ] Registry builder password and static publish credentials are in the environment (`BEX_REGISTRY_BUILDER_PASSWORD`, `BEX_STATIC_S3_*`) — names only in this document.
- [ ] You know each affected App's maintenance tolerance. Phase 3 needs a rollout so Deployments pick up the new image ref; dual-read keeps old refs working until then.

## Inventory (Phase 1 — read-only)

List Apps that still key the legacy column (no `app.bex.co/workspace` label, or labeled but still publishing/pulling the bare name). Store-managed CRs are already named `<ws>-<name>` ([ADR067](../ADR067-security-review-round12.md) finding 1); the dangerous set is never-renamed / hand-applied CRs whose `metadata.name` equals another tenant's.

```
kubectl get apps.app.bex.co -A -L app.bex.co/workspace \
  -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,WS:.metadata.labels.app\\.bex\\.co/workspace,TOMBSTONE:.metadata.annotations.app\\.bex\\.co/identity-tombstone,PREFIX:.status.staticPrefix,IMAGE:.status.image
```

Same-name collisions (the class this migration exists to kill):

```
kubectl get apps.app.bex.co -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.namespace}{"\t"}{.metadata.labels.app\.bex\.co/workspace}{"\n"}{end}' | awk -F'\t' '{c[$1]++} END{for (n in c) if (c[n]>1) print n, c[n]}'
```

Dry-run the tool against one App (mutates nothing):

```
cd lego/operator
go run ./cmd/registry-migrate \
  --app <name> --namespace <ns> --workspace <tea-id>
```

`--all --namespace <ns>` prints a plan per App in that namespace. A plan that says `tombstone: no` means another live App still keys the legacy location — copy may proceed later; do **not** tombstone that shared name.

**Rollback:** none. This phase only reads.

## Phase 2 — copy, verify, tombstone

**STOP — requires explicit operator authorization before touching live tenant data.**

Per App, after the dry-run plan looks right:

```
go run ./cmd/registry-migrate \
  --app <name> --namespace <ns> --workspace <tea-id> \
  --apply
```

The tool:

1. copies each legacy tag `A:gen-N` onto `W/A:gen-N` (digest-preserving skopeo copy) and each object under `A/` onto `W/A/` (S3 server-side copy);
2. re-reads destination digests/ETags and **aborts that App** on mismatch — the legacy location stays authoritative, no tombstone;

   S3 multipart ETags are not content MD5s (`"<md5>-<partCount>"`). A large object copied with a different part layout can fail ETag equality even when bytes match. That is the **safe** direction (no tombstone, `conflicts()` also refuses overwrite). If a dry-run shows only ETag mismatches on objects whose sizes match, inspect with `aws s3api head-object` and retry after a same-part-size copy, or skip that App until republish.

3. stamps `app.bex.co/workspace=<tea-id>`, `app.bex.co/identity-tombstone=true`, and `status.staticPrefix` when a revision is active;
4. writes marker tag `bex-tombstone` on the legacy repo and object `A/.bex-tombstone`. **No blob is deleted.**

Idempotent: re-running skips digest/ETag matches.

**Verify:** destination tags equal source digests (`skopeo inspect`); static object counts match; the App still serves (dual-read of any not-yet-rolled Deployment image ref still hits the legacy repo because blobs were not deleted).

**Rollback:** leave the tombstone in place or remove the annotation so dual-read resumes. Do **not** delete destination objects as a rollback — the legacy location is still complete. Point `status.staticPrefix` back at `A/<rev>/` only if a serve regression is proven.

## Phase 3 — tenant redeploy

**STOP — requires explicit operator authorization before touching live tenant data.**

Trigger a deploy (or wait for the next natural roll) so each migrated App's Deployment/CronJob/publish Job bakes `<registry>/W/A:gen-N` and `status.staticPrefix=W/A/<rev>/`. Dual-read plus leftover legacy blobs keep currently-running pods up during the roll.

**Verify:** `kubectl get deploy -n <ns> <name> -o jsonpath='{.spec.template.spec.containers[0].image}'` shows `W/A`; static-server hosts resolve to the recorded prefix; no ImagePullBackOff.

**Rollback:** roll the Deployment back to the previous revision. Legacy blobs are still present until Phase 4.

## Phase 4 — drop dual-read and delete legacy blobs

**STOP — requires explicit operator authorization before touching live tenant data.**

Criteria before this phase: **zero dual-read hits over 14 days** (no kubelet pull of `<registry>/A:…`, no static-server GET under `A/<rev>/`) and every migrated App is tombstoned with a rolled image ref.

Then, and only then:

- delete the legacy Zot repository `A` (or its remaining tags except after a confirmed empty catalog);
- delete the legacy S3 prefix `A/` (the `.bex-tombstone` object included);
- confirm no unlabeled sibling still owns `A`.

The operator no longer grants dual-read once the tombstone annotation is set. Removing the blobs is the last step so a missed Deployment cannot ImagePullBackOff.

**Rollback:** restore from registry/S3 backup. There is no in-place un-delete. Do not start Phase 4 until the 14-day window is clean.

## Per-phase summary

| Phase | Mutates tenant data? | Authorization | Rollback |
| --- | --- | --- | --- |
| 1 inventory / dry-run | no | not required | n/a |
| 2 copy+verify+tombstone | yes (copy + markers; **no blob delete**) | **STOP — explicit operator authorization** | annotation off; legacy still authoritative |
| 3 redeploy | yes (new image ref / prefix in status) | **STOP — explicit operator authorization** | roll back the Deployment |
| 4 drop fallback + delete legacy | yes (destructive) | **STOP — explicit operator authorization** + 14-day zero dual-read hits | restore from backup |
