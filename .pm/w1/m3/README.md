# w1 · m3 — Elastic substrate: bin-pack + autoscale

**Worker:** worker1 **Goal:** Make provisioning track aggregate demand (Σ running tiers), not be hand-driven: pack pods onto the fullest node for density, and add/remove machines reactively. This is bex's auto-allocator. **Status:** implemented, simplified + verified on the CAPD mock (2026-07-09, `scripts/verify-elastic.sh` exit 0 twice: scale-up, MostAllocated pack, scale-down) — open: t008 closeout only, ship-gated on the Hetzner leg (next `/ship` triggers app-cluster.yml: overlay re-apply → control-plane rollout + autoscaler install)

## Tasks (in order)

| id   | title                                                                         | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Tier → pod requests/limits — **DONE**                                         | —   | —          |
| t002 | KubeScheduler `MostAllocated` profile (density) — **DONE**                    | 30m | t001       |
| t003 | Wire cluster-autoscaler (clusterapi provider) — **DONE**                      | 30m | —          |
| t004 | Autoscaler min/max on the `bex-worker-0` MachineDeployment — **DONE**         | 20m | t003       |
| t005 | Verify pack + scale-up + scale-down end-to-end — **DONE** (CAPD mock)         | 25m | t002,t004  |
| t006 | Simplify — `/simplify` over what this milestone changed — **DONE**            | 20m | t005       |
| t007 | Test coverage — script-level checks for the elastic behavior — **DONE**       | 20m | t005       |
| t008 | Closeout — verify DoD holds, then move the milestone to `done/`               | 10m | t007       |

## Definition of done

A pending pod (no room) triggers a new Hetzner node; pods land on the most-allocated node; an emptied node scales down. Machine count tracks Σ demand with no manual `scale`.

## Current state (2026-07-09, implemented)

- **t002 done**: `MostAllocated` scheduler config in BOTH overlays — CAPH `KubeadmControlPlane` (kubeadm map-form extraArgs + `files[]` + `extraVolumes`) and the local `KubeadmControlPlaneTemplate` (CAPI v1beta2 list-form). Verified live: the mock's kube-scheduler runs `--config=/etc/kubernetes/scheduler-config.yaml`; the pack probe chose the fuller worker. **Lesson:** the local ClusterClass `podSecurityStandard` patch used `add /spec/…/files` — RFC 6902 `add` on an existing member REPLACES it, which silently dropped the scheduler file and broke a control-plane join (etcd wedged; mock recreated per the recreate-don't-repair rule). The patch now appends via `/files/-`.
- **t003 done — decision changed from the plan**: NOT the Argo Application. Argo runs on the app cluster; the CAPI objects the autoscaler scales live on the mgmt/infra cluster, and bridging with an Argo app would seal a mgmt-cluster kubeconfig into the app cluster — reversing the docs/infra-credentials.md one-way trust chain. Instead the chart (9.58.0, `clusterAPIMode: kubeconfig-incluster`) installs on the MGMT cluster beside CAPI, reusing the CAPI-generated `bex-kubeconfig` secret (zero new creds): one installer `scripts/install-autoscaler.sh` (owns the chart pin; called by `.github/workflows/app-cluster.yml` in prod and `scripts/mock-cluster.sh` locally), shared values `infra/clusterapi/autoscaler-values.yaml` (incl. `rbac.additionalRules` for the `infrastructure.cluster.x-k8s.io` group — the chart's default RBAC misses infra machine templates); the values-file header is the decision record.
- **t004 done**: CAPH `bex-worker-0` min 0 / max 3 + cx33 capacity hints (4 CPU/8G, Hetzner API) for scale-from-zero; local topology worker-0 min 1 / max 5. `replicas` removed in both — with the annotations present, CAPI's defaulting webhook seeds a fresh create at min-size and `kubectl apply` re-runs never stomp the autoscaler's count.
- **t005 done (mock) / t008 open (prod)**: `scripts/verify-elastic.sh` passed end-to-end on the CAPD mock (workers 1→2 on a Pending pod; probe packed onto the 9350m worker over the 6550m one; drain → back to 1). The Hetzner leg rides the next `/ship`: app-cluster.yml re-applies the overlay (⚠ scheduler-config change = KCP rollout — CAPH replaces the single control-plane machine, brief API blip; apps ride the LB) and installs the autoscaler. t008 closes only after that runs green.

## Source + Goal linkage

- **Source:** converted from `.tmp/002-binpack-scheduler.md` and `.tmp/004-cluster-autoscaler.md` (t001 already shipped — see Current state). Retrofitted to current `/pm` conventions 2026-07-08; Closeout (t008) added 2026-07-09.
- **Render parity closing task: omitted.** Pure infra/gitops manifests — no REST/GraphQL/MCP/UI surface change. Render keeps machine provisioning internal (no user-facing API for it); the user-facing Render autoscaling surface (`PUT /services/{id}/autoscaling`) is tracked separately as `w1/008`, gated on this milestone.
- **Goal linkage:** V0 roadmap #6 (add & remove physical machine); the density economics that make tier pricing and free-tier sleep (m4, shipped) pay off.
- **Expected outcome:** node count follows real demand — a pending pod self-provisions a Hetzner machine, an emptied node is removed; no human runs `mock-cluster.sh scale N` in anger.
- **Why now:** a hand-scaled single-node pool is both a capacity ceiling and a cost floor; m4's shipped "sleep = free" only saves money if drained nodes actually scale down. The replica-semantics contract is settled (scale verb, w2/m12; sleep/suspend override the effective count without rewriting `spec.replicas`) — the autoscaler moves **nodes**, never `spec.replicas`.

## Notes

- The work here is gitops/infra yaml (no Go), so the standing closing tasks apply in the m13 style: `/simplify` runs over the changed manifests; test coverage means scripted, asserting verification (extend the t005 script) or an explicit n/a closure — never tautological checks.
