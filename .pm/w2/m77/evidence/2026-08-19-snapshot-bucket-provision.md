# Agent-session snapshot store enablement — bucket, SSE, scoped credential

**Date:** 2026-08-19 · **Cluster:** `hetzner-prod` (Secret install) + Wasabi `eu-central-2` · **Milestone:** w2/m77

## TL;DR

Dedicated snapshot bucket `bex-agent-snapshots` exists, is **not** `bex-tfstate`, and is encrypted at rest by Wasabi's platform-wide AES-256 (Wasabi does not implement `Put/GetBucketEncryption`). A bucket-scoped IAM user `bex-agent-snapshot` can PUT/GET/DELETE under `agent-snapshots/` and is denied on `bex-tfstate`, account listing, and an unrelated bucket. Values are in `.env` (gitignored) and GitHub Actions secrets; the cluster Secret is `bex-system/bex-agent-snapshot`. bex-api Deployment wiring is in git (`optional: true` secretKeyRefs). **Arm log + live hibernate→rehydrate walks wait on the deploy that consumes those env refs** (Argo `bex-operator` is auto-sync/selfHeal, so a live kubectl env patch would not stick).

## t001 — bucket + credential

- Provider/endpoint: same Wasabi contract as backups/static (`TF_STATE_ENDPOINT` / region `eu-central-2`).
- Bucket: `bex-agent-snapshots` (created 2026-08-19). Prefix: `agent-snapshots`.
- SSE: Wasabi automatic AES-256 at rest. `PutBucketEncryption` returns `MethodNotAllowed`. A probe PUT's `HeadObject` has an empty `ServerSideEncryption` header (expected on Wasabi; not SSE-S3 API).
- IAM: user `bex-agent-snapshot`, inline policy `BexAgentSnapshot` from `infra/wasabi/agent-snapshot-s3-policy.json` (bucket + objects only; no `ListAllMyBuckets`, no `bex-tfstate`).
- Access matrix (`scripts/agent-snapshot-secret.sh verify`):

```
PASS  allow  snapshot put object
PASS  sse    probe object AES256 via Wasabi automatic AES-256 at rest (no S3 SSE header)
PASS  allow  snapshot get object
PASS  allow  snapshot list prefix
PASS  deny   snapshot list tfstate bucket
PASS  deny   snapshot list account buckets
PASS  deny   snapshot list unrelated bucket
PASS  allow  snapshot delete object
```

- `.env` six `BEX_AGENT_SNAPSHOT_S3_*` names filled (values not recorded here). `scripts/gh-secrets.sh` set the six GitHub Actions secrets.
- Installer: `scripts/agent-snapshot-secret.sh {provision|verify|install}`.

## t002 — prod wiring (partial until deploy)

- Manifest: `lego/operator/config/api/deployment.yaml` — all six keys from Secret `bex-agent-snapshot`, `optional: true`. Comment documents rollback (delete/absent Secret ⇒ reclaim Terminate).
- Secret installed: `bex-system/bex-agent-snapshot` keys match the six env names (2026-08-19). Current `bex-api` pods do **not** yet list those env vars; installer skipped rollout because the live Deployment has not consumed them.
- Retention / pin quota: **keep m68 defaults** (`BEX_AGENT_SNAPSHOT_RETENTION` 168h with dirty-git doubling; `BEX_AGENT_MAX_PINNED_SANDBOXES_PER_WORKSPACE` 10). No prod override.
- Arm log `bex-api: agent-session hibernation enabled (object store bex-agent-snapshots)`: **not yet observed** — blocked on Argo applying this milestone's Deployment env.

## t003 / t004 — live walks

Not run. Require the armed bex-api (t002 arm log) so the Completer uses the snapshot store instead of Terminate. Do not lower `BEX_AGENT_SANDBOX_IDLE_TTL` cluster-wide (it would reclaim every workspace). After the deploy lands: complete a session with a distinctive uncommitted `/workspace` edit, wait the 30m grace with no editor SSH, confirm phase `hibernated` + object under `agent-snapshots/<ws>/` + pod gone, then Resume/Steer ≥3 times and capture `agent-session rehydrate: session=… resumed in …` against p50<~5s / p95<~15s.

## Rollback

Delete `bex-system/bex-agent-snapshot` (or leave the Secret absent). All six env refs are `optional: true`, so bex-api starts with the coordinates unset, `NewS3SnapshotStore` returns `(nil, nil)`, and reclaim stays Terminate (w2/m67). A **partial** Secret fails startup (`ErrPartialS3SnapshotConfig`) rather than silently disabling the tier.
