# w2 · m81 — metrics-server kubelet TLS: CSR approver + --kubelet-certificate-authority

**Worker:** worker2 **Goal:** make metrics-server verify kubelet serving certificates — deploy a kubelet-serving CSR approver, complete serving-cert rotation in the machine templates, and add `--kubelet-certificate-authority` (the insecure flag already left the shared base) — closing the security-register line re-reported in rounds 7, 8, and 9. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                                                              | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Deploy a kubelet-serving CSR approver (e.g. postfinance/kubelet-csr-approver) via deploy/gitops with tight matching rules (provider regex, node-name/IP checks)                                                                       | 45m | —          |
| t002 | Kubelet serving-cert rotation: add `rotate-server-certificates: "true"` to the CAPD templates (CAPH already sets it); verify on the mock cluster that new nodes get approved serving certs                                            | 45m | t001       |
| t003 | metrics-server: add `--kubelet-certificate-authority` to the base (insecure flag already base-absent); reconcile the local-overlay bypass; verify `kubectl top` + bex-api resource-metrics on the mock cluster (hostNetwork:10251 quirk) | 30m | t002       |
| t004 | Prod rollout: apply to hetzner-prod (approver first, template rotation on the next node rotation, then the CA flag); annotate the register lines in ADR072/ADR061/ADR063                                                              | 30m | t003       |
| t005 | Simplify (standing): run /simplify over the changed manifests/scripts                                                                                                                                                                | 20m | t004       |
| t006 | Test coverage (standing): gitops-validate/clusterapi-validate guard asserting the insecure flag stays absent and the approver manifest present                                                                                        | 30m | t004       |
| t007 | Closeout (standing): verify DoD, mark done, move milestone to done/                                                                                                                                                                  | 15m | t006       |

## Definition of done

On the mock cluster (and prod after rollout), metrics-server runs with `--kubelet-certificate-authority` and without `--kubelet-insecure-tls` anywhere; a newly scaled-up node obtains an approved kubelet serving certificate automatically (approver logs + CSR state show it); `kubectl top nodes/pods` and bex-api's resource-metrics fallback both work; a validation-script guard fails if the insecure flag reappears or the approver is absent; the ADR072 #5 / ADR061 #11 / ADR063 #9 register lines are annotated closed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 1, item 5); ADR072 finding 5, re-reported as ADR061 #11 and ADR063 #9 — plan exists only in ADR072/ADR036 prose, no `.pm` owner.
- **Goal linkage:** platform security hygiene; trustworthy resource metrics feed usage metering (ADR023) and the dashboard metrics snapshot fallback.
- **Expected outcome:** kubelet metrics scrapes are TLS-verified; a thrice-reported register line closes.
- **Why now:** cheapest close of a recurring security-register line; fix shape already known.
- **Render parity omitted:** pure platform infra; no REST/GraphQL/MCP/UI surface.
- **Repo state (verified 2026-08-18):** the insecure flag is **already absent from the shared base** — commit `815e003b` (round-10 remediation) dropped it from `deploy/gitops/base/metrics-server.yaml`; it survives only as the CAPD-local Helm-parameter patch in `deploy/gitops/overlays/local/kustomization.yaml` (lines 12–24). Still missing: `--kubelet-certificate-authority` in the base args, any kubelet-serving CSR approver (none exists in the repo), and `rotate-server-certificates` on the CAPD templates (`infra/clusterapi/overlays/local-capd/cluster.yaml`; the CAPH templates already set it — `cluster.yaml` lines 114/287/831/851/1231 + `sandbox-pool.yaml` line 204, via `kubeletExtraArgs`). New approver charts must be mirrored via `scripts/helm-artifact.sh mirror` + a `deploy/helm-artifacts.lock` row and use `project: bex-platform` (enforced by `scripts/gitops-validate.sh`).
- **Out of scope:** existing-node serving certs may need a node rotation to pick up `rotate-server-certificates` (document; never force-rotate prod nodes here — coordinate with the ADR053 immutable-template rotation); full CA rotation stays ADR036.
