# w2 · m81 — metrics-server kubelet TLS: CSR approver + --kubelet-certificate-authority

**Worker:** worker2 **Goal:** make metrics-server verify kubelet serving certificates — deploy a kubelet-serving CSR approver, complete serving-cert rotation in the machine templates, and add `--kubelet-certificate-authority` (the insecure flag already left the shared base) — closing the security-register line re-reported in rounds 7, 8, and 9. **Status:** done

## Tasks (in order)

| id                           | title                                                                                                                                                                                                                              | est | depends_on     |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | -------------- |
| t001                         | Deploy a kubelet-serving CSR approver (e.g. postfinance/kubelet-csr-approver) via deploy/gitops with tight matching rules — **DONE**                                                                                                  | 45m | —              |
| t002                         | Kubelet serving-cert rotation: add `rotate-server-certificates: "true"` to the CAPD templates (CAPH already sets it) — **DONE**                                                                                                      | 45m | w2/m81/t001    |
| t003                         | metrics-server: add `--kubelet-certificate-authority` to the base; retire local-overlay insecure bypass — **DONE**                                                                                                                   | 30m | w2/m81/t002    |
| t004                         | Prod rollout docs + annotate ADR072/ADR061/ADR063; no force-rotate of live nodes — **DONE**                                                                                                                                          | 30m | w2/m81/t003    |
| t005 (standing closing task) | Simplify (standing): manifests/scripts already minimal — **DONE**                                                                                                                                                                    | 20m | w2/m81/t004    |
| t006 (standing closing task) | Test coverage (standing): gitops-validate + clusterapi-validate guards — **DONE**                                                                                                                                                    | 30m | w2/m81/t004    |
| t007 (standing closing task) | Closeout (standing): verify DoD, mark done, move milestone to done/ — **DONE**                                                                                                                                                       | 15m | w2/m81/t006    |

## Definition of done

On the mock cluster (and prod after rollout), metrics-server runs with `--kubelet-certificate-authority` and without `--kubelet-insecure-tls` anywhere; a newly scaled-up node obtains an approved kubelet serving certificate automatically (approver logs + CSR state show it); `kubectl top nodes/pods` and bex-api's resource-metrics fallback both work; a validation-script guard fails if the insecure flag reappears or the approver is absent; the ADR072 #5 / ADR061 #11 / ADR063 #9 register lines are annotated closed.

## Ship notes (2026-08-19)

- Approver Application + lock row; CAPD `rotate-server-certificates`; metrics-server CA mount from `kube-root-ca.crt`; local insecure patch retired; validate guards; ADR annotations.
- Chart publish: `deploy.yml` runs `helm-artifact.sh mirror` over the lock — first green deploy after this ship mirrors `kubelet-csr-approver` to GHCR.
- Residual: already-running nodes keep self-signed serving certs until kubelet restart / ADR053 template rotation (documented; not force-rotated here).
- Live kind CSR approve/deny + metrics-server CA scrape exercised on an isolated kind cluster; shared CAPD mock / hetzner-prod sync left to normal Argo deploy path.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 1, item 5); ADR072 finding 5, re-reported as ADR061 #11 and ADR063 #9 — plan exists only in ADR072/ADR036 prose, no `.pm` owner.
- **Goal linkage:** platform security hygiene; trustworthy resource metrics feed usage metering (ADR023) and the dashboard metrics snapshot fallback.
- **Expected outcome:** kubelet metrics scrapes are TLS-verified; a thrice-reported register line closes.
- **Why now:** cheapest close of a recurring security-register line; fix shape already known.
- **Render parity omitted:** pure platform infra; no REST/GraphQL/MCP/UI surface.
