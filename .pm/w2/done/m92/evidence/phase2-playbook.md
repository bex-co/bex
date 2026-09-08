# w2/m92 · Phase 2 ready-to-run playbook (awaiting STOP authorization)

**Status:** blocked on explicit user authorization (runbook STOP).  
**Refreshed:** 2026-09-08 — kubectl inventory matches t001 worklist (no new labeled Apps; no tombstones yet). `…-beancount-cms-v2` still `Deploying`. `BEX_REGISTRY_DUAL_READ` still unset.

## Authorization gate

Do **not** run any command in § Execute until t003 records user authorization (date + wording).

## Preconditions (before `--apply`)

1. Enable dual-read for the migration window only:

```bash
kubectl --context=hetzner-prod -n bex-system set env deploy/bex-controller-manager BEX_REGISTRY_DUAL_READ=1
kubectl --context=hetzner-prod -n bex-system rollout status deploy/bex-controller-manager --timeout=180s
```

2. Port-forward Zot + load builder password (never print/commit). `registry-migrate` now passes `--src-creds`/`--dest-creds` to skopeo (m92 fix); prior runs needed a separate `skopeo login`.

```bash
kubectl --context=hetzner-prod -n bex-registry port-forward svc/zot 15000:5000 &
# BEX_REGISTRY_BUILDER_PASSWORD from bex-build/bex-registry-push config.json auth password
kubectl config use-context hetzner-prod
```

3. Immediate pre-apply dry-run (diff vs t001):

```bash
cd lego/operator
for ns in tea-d98210cbbpdc73dcrkvg tea-da1eg9gbiuuc73bd8uag tea-da2isimlm39c739m4ofg; do
  go run ./cmd/registry-migrate --all --namespace "$ns" \
    --registry 127.0.0.1:15000 \
    --registry-user bex-builder \
    --registry-password "$BEX_REGISTRY_BUILDER_PASSWORD"
done
```

## Execute (STOP-gated)

Apply per namespace (or per App). Prefer App-by-App in the Phase-1 maintenance order; `--all` is acceptable once dry-run matches.

```bash
go run ./cmd/registry-migrate --all --namespace <ns> \
  --registry 127.0.0.1:15000 \
  --registry-user bex-builder \
  --registry-password "$BEX_REGISTRY_BUILDER_PASSWORD" \
  --apply
```

## Verify

- Every worklist App has `app.bex.co/identity-tombstone=true`
- Spot-check digests with `skopeo inspect` on a copied tag (src legacy == dst `W/A`)
- Legacy tags still listable (`LEGACY_BLOB_REMAINS`)
- Running Apps still Serving / no ImagePullBackOff burst

## Rollback (if needed)

Remove tombstone annotation (or leave it); do **not** delete destination objects. Legacy blobs remain authoritative until Phase 4.
