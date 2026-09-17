# w7 · m148 — Reliable local cluster bring-up

**Worker:** worker7 **Goal:** Publish a usable local substrate without concurrent provisioning or misleading success. **Status:** blocked (t001-t006 done; t007 closeout held on a healthy local run)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Serialize local cluster mutations — **DONE** | 40m | — |
| t002 | Publish validated kubeconfig atomically — **DONE** | 50m | w7/m148/t001 |
| t003 | Fail clearly on unmet substrate prerequisites — **DONE** | 45m | w7/m148/t002 |
| t004 | Verify the installed substrate before reporting success — **DONE** | 45m | w7/m148/t003 |
| t005 | Simplify — **DONE** | 25m | w7/m148/t001, w7/m148/t002, w7/m148/t003, w7/m148/t004 |
| t006 | Test coverage — **DONE** | 45m | w7/m148/t001, w7/m148/t002, w7/m148/t003, w7/m148/t004 |
| t007 | Closeout | 15m | w7/m148/t005, w7/m148/t006 |

## Definition of done

- [x] A second invocation performs no provisioning or scale mutation while the first owns the lock.
- [x] Normal exit and interruption release only the invoking process's lock; stale ownership has a safe recovery path.
- [x] Failure before publication leaves the prior shared kubeconfig byte-identical.
- [x] Published configuration reaches the intended local cluster with TLS verification enabled.
- [x] Concurrent refresh cannot publish partial or stale configuration; credential contents are never printed.
- [x] Required failures exit nonzero and prevent dependent stages from running.
- [x] Healthy bootstrap still reaches CNI installation and later readiness checks in the correct order.
- [x] No recovery touches production or silently destroys another workstream's resources.
- [x] The script never prints its successful up message after a required check fails.
- [ ] A healthy local run and controlled failed prerequisite have recorded results.
- [ ] Report any local environment rebuilt; do not claim the script installs the bex operator or App CRD if that remains a separate workflow.

## Source + Goal linkage

- **Source:** Approved w7 brainstorm item 1 via `$pm all for w7`; `.pm/w9/blocked/060.md`, `.pm/w10/003.md`, and current `scripts/mock-cluster.sh` / `scripts/dev-env.sh`.
- **Goal linkage:** ADR008 hosting reliability: make local acceptance evidence dependable.
- **Expected outcome:** Concurrent bring-up cannot interleave mutations; failed generation preserves the prior kubeconfig; success means required substrate checks passed.
- **Why now:** Local cluster failures block verification in several queues. w1/043 installed missing components and w1/m72 unified dev harnesses; neither closed publication, concurrency, and failure-reporting gaps.
- **Render parity:** Omitted: local infrastructure and harness behavior only; no REST/GraphQL/MCP/UI contract change.
- **Sizing:** 180m implementation; 265m including standing closing tasks, 7 tasks.

## BLOCKED 2026-09-17 — needs a healthy CAPD environment

Every implementable task (t001-t006) is done, verified, and CI-gated. One acceptance line cannot be satisfied here: **"A healthy local run ... recorded"**. The controlled-failure half *is* recorded, and it is strong evidence — a real bring-up refused to publish and left the shared kubeconfig byte-identical.

The gate is the environment, not the code. The local CAPD cluster has rotted: `kubectl --context kind-bex-mgmt get machines` shows three machines `Running` and **20 days old**, while the Docker containers backing them are gone, so `bex-lb` publishes no 6443 port and `cluster/bex` sits `Provisioned`/Ready=False forever. Waiting cannot fix that — CAPI's state is stale, not slow. Two bring-up attempts confirmed it.

Recovery is a reprovision, which is pre-approved by `CLAUDE.md` but is a 15-25 minute destructive operation on a shared local cluster and should not be left half-finished by a drain:

```sh
kubectl --context kind-bex-mgmt delete cluster bex --wait=false
bash scripts/mock-cluster.sh
```

Once that reports its success banner, t007 closeout can run and this milestone moves to `done/`.
