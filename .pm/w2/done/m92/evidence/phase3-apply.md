# w2/m92 · Phase 3 redeploy / verify

**Date:** 2026-09-08 · **Cluster:** `hetzner-prod`  
**Auth:** recorded in `t004.md` (same `/goal implement` change-window as t003)

## Cutover — `…-agentmarketcap-1` (only live legacy ref)

1. Patched `status.image` + `status.artifactImage` to scoped  
   `…/W/A:gen-230@sha256:07d198c2…` (digest-preserving; same blob as legacy).
2. Annotated to nudge reconcile; operator converged Deployment to scoped path.
3. Rollout succeeded; App `phase=Running`, Ready `1/1` on scoped image.

Patching only `status.image` was insufficient — `reusableArtifactImage` restored the legacy `artifactImage`. Both fields required.

## Confirm — already-scoped Apps

| App | Action | Result |
| --- | --- | --- |
| `…-tianpan-v4-web` | rollout restart | Running on scoped image |
| `…-market-size` | confirm-only (Failed, no Deploy) | tombstoned |
| `…-beancount-forum` / `…-block-eden-mono` / `…-eden-cms-v2` | restart attempted | **cluster capacity** (Pending / ProgressDeadlineExceeded); rolled back with `rollout undo` — Deployments remained **scoped**; Apps returned Running (eden briefly Deploying with available pod) |
| `…-beancount-cms-v2` | no forced restart (pre-existing capacity Pending) | scoped image; Building/Deploying unrelated to identity |

## Identity gate

`IDENTITY_GATE_OK`: every labeled App has `identity-tombstone=true` and a scoped `status.image` (or empty Failed). All live Deployments use `…/W/A:…` paths. Unlabeled `hello-static` out of scope. **Zero blob deletes.**

## Notes

- Confirm restarts on a full node group are optional when Deployments already bake `W/A`; capacity made them harmful — undo restored health without legacy regression.
- `BEX_REGISTRY_DUAL_READ=1` remains on for the Phase-4 clean window.
