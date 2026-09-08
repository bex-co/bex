# w2/m92 · Phase 3 ready-to-run playbook (awaiting STOP authorization)

**Status:** blocked on explicit user authorization (runbook STOP) and on Phase 2 completion.  
**Order:** from Phase 1 maintenance tolerance (`.pm/w2/m92/evidence/phase1-inventory.md`).  
**Precondition note (2026-09-08):** `…-beancount-cms-v2` still `Deploying` — new pod Pending (Insufficient cpu/memory; node group at max). Old pod still Running/serving on scoped `gen-460`. Details: `beancount-cms-v2-deploying.md`. Skip Phase-3 restart until Ready or capacity is addressed.

## Authorization gate

Do **not** redeploy tenant Apps until t004 records user authorization (date + wording). Prefer authorizing Phase 2 and Phase 3 in the same change window.

## Rollout order

1. `tea-da1eg9…/…-market-size` (Failed — confirm only)
2. `tea-da2isi…/…-tianpan-v4-web` (already scoped image)
3. `…-beancount-forum`
4. `…-block-eden-mono`
5. `…-eden-cms-v2`
6. `…-beancount-cms-v2` — **wait until Ready** (was Deploying at inventory)
7. `…-agentmarketcap-1` — **must cut over** (only live legacy image ref)

## Per-App procedure

```bash
CTX=hetzner-prod
NS=<ns>
APP=<name>

# Trigger roll (annotation bump or explicit restart)
kubectl --context=$CTX -n "$NS" rollout restart deploy/"$APP"
kubectl --context=$CTX -n "$NS" rollout status deploy/"$APP" --timeout=300s

# Image must be W/A (path contains workspace/)
kubectl --context=$CTX -n "$NS" get deploy "$APP" \
  -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'

# No ImagePullBackOff
kubectl --context=$CTX -n "$NS" get po -l app.kubernetes.io/name="$APP" \
  -o custom-columns=NAME:.metadata.name,READY:.status.containerStatuses[0].ready,REASON:.status.containerStatuses[0].state.waiting.reason

# App phase
kubectl --context=$CTX -n "$NS" get app "$APP" -o jsonpath='phase={.status.phase} image={.status.image}{"\n"}'
```

For Apps already on scoped `status.image`, restart is confirm-only. For `agentmarketcap-1`, ensure post-Phase-2 operator reconcile has written the scoped ref (or patch/redeploy via normal bex deploy path) before declaring healthy.

## Inventory gate (DoD)

After all rolls:

```bash
KUBECONFIG=… bash scripts/verify-workspace-scoped-identity.sh
# Plus: every labeled App has identity-tombstone=true and image refs are W/A
```

Zero labeled Apps on legacy-named repos/users/Secrets or legacy static prefixes.

## Rollback

`kubectl rollout undo deploy/<app> -n <ns>` — legacy blobs still present until Phase 4.
