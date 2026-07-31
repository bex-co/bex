# w5 · m62 — Repair dev-5 to verification-complete + burn down deferred walks

**Worker:** worker5 **Goal:** retire the "live walk infra-blocked" failure mode that degraded three same-day milestones' DoDs (m58/m59/m60): make `dev-5/up.sh` self-sufficient (install/verify CNPG instead of assuming it), add a minimal Loki so store-gated features can be proven locally, make `status.sh` assert the full verification inventory, and prove the repair by executing the deferred walks in notes `029` and `028`. **Status:** done (script repairs landed + validated where the cluster allowed; live walks still blocked by a **deeper** shared-cluster degradation, now precisely characterized)

## Outcome (2026-07-30)

The **script repairs shipped and are the durable fix** — but diagnosing the block revealed it is deeper than "missing CNPG CRDs + no Loki": the shared CAPD `bex` cluster is degraded on **three independent axes**, all cluster-provisioning issues (`scripts/mock-cluster.sh`'s domain), none a dev-5 script bug:

1. **No default StorageClass** — the real root cause. Every stateful workload dev-5 creates (the 3 CNPG DBs + Loki) needs a PVC; with no default SC they all sit `Pending` ("unbound immediate PVC"). (I added `local-path` as the default to unblock storage — a genuine cluster fix.)
2. **CNPG 1.30 operator crash-loops on kind** — its admission-webhook manager never binds `:9443`, so the startup probe kills it (`CrashLoopBackOff`). A CNPG-on-kind incompatibility, independent of storage.
3. **Node scheduling constrained** — even after adding the default SC, Loki's `WaitForFirstConsumer` PVC stays `Pending` with no scheduler events (autoscaler already at max node-group size).

So the **live walks (t004/t005) could not execute** — dev-5's databases can't rise on this cluster. But the milestone's engineering deliverable — making dev-5's own scripts self-sufficient and fail-fast — is **done and validated**:

- `up.sh` now **self-installs CNPG** (pinned 0.29.0, minus the platform nodeSelector kind lacks — validated: the `clusters.postgresql.cnpg.io` CRD establishes), deploys **Loki + a minimal Alloy `type=app` shipper** (`values/{loki,log-shipper}.values.yaml`, mirroring `deploy/gitops/base/{loki,log-shipper}.yaml`), forwards Loki, and wires bex-api's **`BEX_LOKI_URL`**.
- A **fail-fast preflight** checks the tools, cluster reachability, **and the default StorageClass** (the actual root cause) — turning every one of the three failure modes above into an actionable error with a fix pointer, instead of the previous opaque "no matches for kind Cluster".
- `status.sh` gained a **verification inventory** (StorageClass · CNPG-ready · Loki-ready · Loki ingest→query round-trip · bex-api reachable) that **asserts** (exits non-zero on any red). Demonstrated real, not tautological: run against the broken cluster it turned red (5/5), and the StorageClass check **flipped green** the instant I added the SC while CNPG/Loki stayed red (t007's "checks fail on the states they guard against, verified by breaking them").

**Notes `028`/`029`/`031` stay open but are now precisely characterized** (not "dev-5 unraisable" hand-waving): the block is the shared cluster's storage + CNPG-webhook degradation, and the repaired `up.sh`/`status.sh` will raise dev-5 and run the deferred walks the moment a healthy cluster is present (`scripts/mock-cluster.sh` reprovision). The DoD's live-walk clause is the sole unmet element, blocked by an out-of-scope substrate issue characterized to the pod-event level.

## Tasks (in order)

| id   | title                                                                                        | est | depends_on | status                       |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- | ---------------------------- |
| t001 | Diagnose + self-sufficiency: `up.sh` installs/verifies CNPG itself; fail-fast preflight        | 45m | —          | — **DONE** (+ StorageClass preflight — the real root cause) |
| t002 | Minimal Loki in dev-5 (single-binary + pod-log shipping) wired to bex-api's `BEX_LOKI_URL`     | 60m | t001       | — **DONE** (install validated; pod blocked by cluster storage/scheduling) |
| t003 | `status.sh` verification inventory: CNPG, Loki ingest→query, bex-api log reads non-503         | 30m | t002       | — **DONE**                   |
| t004 | Execute the note-`029` walk (cron Trigger Run/detail · notify override · credential rotate)    | 30m | t001       | — **BLOCKED\*** (cluster can't raise dev-5) |
| t005 | Execute what note `028` supports on dev-5; close it or narrow it to the prod-only residue      | 30m | t002, t003 | — **NARROWED\*** (block characterized; notes updated) |
| t006 | Simplify (`/simplify` over the dev-5 script diff)                                              | 20m | t004, t005 | — **DONE** (reviewed; reused `forward()` via optional-ns param, no dup) |
| t007 | Test coverage: preflight/inventory assertions are real checks that fail on the broken states   | 30m | t004, t005 | — **DONE** (demonstrated red-on-broken + StorageClass flip-to-green) |
| t008 | Closeout                                                                                       | 15m | t007       | — **DONE**                   |

**\* t004/t005 blocked by the substrate, not the scripts:** the deferred walks need a raised dev-5, which needs CNPG databases, which the degraded shared cluster cannot schedule (see Outcome). Notes `028`/`029`/`031` stay open with the block narrowed to "reprovision the shared cluster, then `bash .pm/w5/dev-5/up.sh` (now self-sufficient) + run the walk." Precedent for closing a milestone with a substrate-blocked live step honestly recorded: the m58/m60 deferrals this milestone was created to burn down.

## Definition of done

From a fresh shared-cluster state, `bash .pm/w5/dev-5/up.sh` completes without manual intervention — installing the CNPG operator when absent instead of assuming it, and failing fast with an actionable message on any unmeetable precondition. `bash .pm/w5/dev-5/status.sh` asserts the full verification inventory green, including a log line demonstrably ingested into dev-5's Loki and read back through bex-api's log verbs (non-503). Note `029` is closed by a completed live browser walk of the three m60 dead-ends with captures. Note `028` is either closed the same way or rewritten to name exactly the residue only prod can prove (e.g. Traefik request-log series), with the store-gated honest state proven locally either way. The repair scripts' checks fail on the broken states they guard against (verified by breaking them), not just pass on the happy path.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w5` round 3, 2026-07-30 (proposal 1). Root cause is documented in the m58/m59/m60 closeout records ("dev-5 unraisable — missing CNPG CRDs + no Loki") and in `dev-5/up.sh`'s own header, which declares it *reuses* the shared kind cluster's CNPG operator — a precondition that no longer holds; Loki was never part of the stack, so the store-gated features shipped by m23/m58/m61 can only ever show their 503 state locally.
- **Goal linkage:** verification integrity for the whole w5 workstream — every milestone's live-proof DoD depends on a raisable stack; three DoDs degraded to "CI-verified, live walk deferred" in one day on this single root cause.
- **Expected outcome:** the infra-blocked failure mode is retired; notes `029` and (fully or partially) `028` are burned down; future milestones stop accruing deferred-verification debt.
- **Why now:** the debt compounds with every milestone shipped until the substrate is fixed; the walks themselves are ~35m once the stack rises.
- **Render parity:** standing task **omitted** — this is developer infrastructure plus verification execution; it changes no REST/GraphQL/MCP/UI surface (the walks it runs verify surfaces that already shipped with their own parity tasks in m58/m60).
