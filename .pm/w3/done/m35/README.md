# w3 · m35 — Sandbox security hardening: per-sandbox boundary + least-privilege OpenSandbox

**Worker:** worker3 **Goal:** close the sandbox integration's known security gaps now that exec is exposed: make each sandbox an independent network and ownership domain, enforce a gVisor-compatible egress policy, and reduce the OpenSandbox lifecycle server to tenant-namespace-scoped authority. **Status:** done 2026-07-30 — deployed to production and adversarially verified on the real gVisor/Cilium substrate (40-check ownership/network/DNS-SNI/RBAC/admission/Hubble matrix, clean disposable-fixture audit). m33's post-land sandbox-exec security dependency is satisfied; the sandbox may now expand further. — **DONE**

## Tasks (in order)

| id   | title                                                                                 | est | depends_on                        |
| ---- | ------------------------------------------------------------------------------------- | --- | --------------------------------- |
| t001 | Freeze the sandbox threat model and fail-closed security contract — **DONE**          | 45m | —                                 |
| t002 | Remove sandbox same-namespace lateral access; add precise control/data-plane allows — **DONE**   | 90m | t001                              |
| t003 | Add gVisor-compatible Cilium DNS/FQDN egress enforcement and block DNS exfiltration — **DONE**   | 90m | t002                              |
| t004 | Make the public `networkPolicy` field enforced and honest — **DONE**                            | 60m | t003                              |
| t005 | Persist sandbox owner metadata and enforce owner/resource authorization — **DONE**               | 90m | t001                              |
| t006 | Remove lifecycle-server cluster-wide mutation RBAC and tighten server ingress — **DONE**         | 60m | t001                              |
| t007 | Make the hardened OpenSandbox deployment GitOps-durable and version-pinned — **DONE**            | 90m | t006                              |
| t008 | Run the adversarial same-workspace/egress/RBAC matrix on the real gVisor substrate — **DONE**    | 90m | t003, t004, t005, t007, t013, t014 |
| t009 | Render parity: security semantics across REST, GraphQL, MCP, and CLI — **DONE**                  | 45m | t008                              |
| t010 | Simplify (`$simplify` over the hardening diff) — **DONE**                                        | 30m | t009                              |
| t011 | Test coverage: generated policies, authz, RBAC, DNS, and negative reachability — **DONE**        | 60m | t009                              |
| t013 | Admission-confine the NamespaceReconciler's dynamic namespace and `bind` authority — **DONE**    | 60m | t006                              |
| t014 | Admission-enforce the sandbox Pod runtime boundary — **DONE**                                    | 45m | t006                              |
| t012 | Closeout — **DONE**                                                                              | 15m | t010, t011, t013, t014            |

## Outcome (2026-07-30)

The hardening is **deployed to production** and **adversarially verified** on the real gVisor/Cilium substrate by `scripts/verify-sandbox-isolation-live.sh` (which mints disposable Hydra/OpenFGA principals, two workspaces, and owner/member/admin identities, runs the matrix, and removes every fixture from an EXIT trap). Observed live (40 PASS / 0 FAIL, exit 0; audited clean — no disposable namespace, Pod, Hydra client, or OpenFGA tuple remained):

- **Per-sandbox boundary (t002):** every sandbox namespace carries only `default-deny`; two concurrently running sandboxes in the same `<ws>-sandbox` namespace cannot reach each other's Pod IP, Service, execd, egress API, or user ports; sandbox→hosting/lifecycle-API/Kubernetes-API/node/kubelet/cloud-metadata all denied.
- **gVisor + Cilium egress (t003):** attacker probes ran inside real gVisor Pods (`4.19.0-gvisor`); Cilium, not the incompatible OpenSandbox iptables sidecar, denied unapproved DNS, unique DNS-exfiltration queries, unapproved public FQDNs, approved-destination-by-literal-IP/wrong-SNI, and private Kubernetes-API-with-approved-SNI, while explicitly approved FQDN+SNI destinations worked.
- **Truthful `networkPolicy` (t004):** `deny-all` round-trips and is enforced; `allow-all` is refused with a named `SANDBOX_NETWORK_POLICY_UNSUPPORTED` 4xx consistently across REST/GraphQL/MCP/CLI; the durable read fails closed unless the stored metadata is exactly `deny-all`.
- **Durable ownership (t005):** owner isolation fails closed and `can_manage` is the sole override; owner/admin exec succeeds while cross-owner and cross-workspace exec hide existence (non-enumerating 404).
- **Least-privilege lifecycle (t006/t007):** the server ServiceAccount's cluster-wide ClusterRole is read-only (`get/list/watch`); full CRUD + `pods/exec` exist only via namespace-scoped RoleBindings inside each `<ws>-sandbox`; only the exact `bex-api` Cilium identity may reach the lifecycle port (8077); the server image is digest-pinned and GitOps-durable; the lifecycle-server DNS egress is name-filtered to its fixed tenant resolver (exfiltration denied).
- **Admission confinement (t013/t014):** an impersonated bex-api attempt to bind a Secret role into a platform namespace, delete/relabel a platform namespace, downgrade a sandbox namespace's Pod Security labels, or install an arbitrary sandbox allow policy is denied while the exact canonical tenant projection is admitted; an impersonated lifecycle component's runc/no-gVisor Pod (and a `nodeName` pool jump) is denied while the exact gVisor Pod shape is admitted.
- **Hubble attribution:** the final Hubble step attributes policy drops to the attacker sandbox Pod and the unique denied DNS-exfiltration query.

Late-found defect, fixed before closeout: the checked-in `bex-sandbox-pods` ValidatingAdmissionPolicy's CEL `matchConditions` on `namespaceObject` did not select a controller-submitted Pod in a `regime=sandbox` namespace, so a controller identity with valid namespace-scoped Pod authority could submit an ordinary runc Pod and bypass the claimed gVisor boundary. Fixed (commit `329a6ec5`) by replacing the CEL namespace match with the admission API's native `spec.matchConstraints.namespaceSelector`; the live matrix now proves the deny/allow behavior. The verifier's own Hubble query also combined `--from-pod` with `--namespace` (rejected by the Hubble CLI); fixed to use `--from-pod` alone.

`t010` simplify: two behavior-preserving changes were applied — a `defaultDenyNetworkPolicyName` constant so the deny-policy name and the prune desired-set seed cannot drift and silently delete the structural default-deny, and `workspaceID` converted from a free function to a `*Service` method. Declined (security reasons): routing `isWorkspaceAdmin` through `core.AuthorizeOn` (would change the fail-closed posture of the verified admin-override path), `ownerID` hoisting (marginal and on the owner-equality path), and the `gqlInt`/`gqlStr` → `gqlutil` graduation (low value, expands shared-package surface on a verified milestone). No simplification broadened an allow, weakened ownership, or restored cluster-wide mutation authority.

Earlier hardening commits: `b8d1ca95` (network/owner/RBAC/admission hardening), `9dc7dc4c`/`f3535ec2`/`b0dc18a6` (CI + OpenSandbox control-plane health/port fixes), `329a6ec5` (the admission selector fix). Pre-closeout staging notes are in `evidence/2026-07-29-handoff.md`.

## Definition of done

- Two concurrently running sandboxes in the same `<ws>-sandbox` namespace cannot reach each other's Pod IP, Service, execd, egress API, or user ports; cross-tenant and sandbox→hosting isolation still hold.
- gVisor remains the runtime and Cilium, not the incompatible OpenSandbox iptables sidecar, enforces fail-closed DNS/FQDN egress. Unapproved DNS names, public-IP traffic without approved TLS SNI, private/rebound targets, Pod/Service CIDRs, node/kubelet, Kubernetes API, lifecycle server, bex platform services, and cloud metadata are denied; explicitly approved FQDN+SNI destinations work.
- `networkPolicy` is never accepted-and-echoed without enforcement: supported values round-trip and affect policy, unsupported values fail with a named 4xx consistently across REST/GraphQL/MCP/CLI.
- Every sandbox is stamped with durable owner/workspace metadata; list/get/exec/lifecycle operations enforce the documented owner or explicit workspace-admin rule instead of trusting only namespace membership.
- The OpenSandbox server ServiceAccount cannot create/update/delete workload resources outside provisioned `<ws>-sandbox` namespaces. Any cluster-wide informer grant is read-only, and only the bex-api Pod may reach the lifecycle port.
- The NamespaceReconciler's unavoidable cluster-scoped namespace/RoleBinding authority is API-server-confined to canonical managed tenant namespaces and exact regime-specific subjects; it cannot delete/relabel platform namespaces or bind tenant roles there.
- Every Pod admitted in a sandbox namespace is API-server-required to retain the gVisor RuntimeClass, sandbox-node placement, sandbox identity label, and disabled ServiceAccount token; a compromised lifecycle component cannot fall back to runc.
- The server, config, RuntimeClass, and policy resources are GitOps-durable; OpenSandbox server/execd/egress images are deliberately version/digest pinned and the checked-in deployment reproduces the validated state.
- A repeatable adversarial verification script and automated regressions fail on every pre-fix path above. Backend tests, GitOps validation, and relevant cluster checks pass. m33 exec has already landed; no further sandbox capability expansion may proceed until this milestone closes.

## Source + Goal linkage

- **Source:** user-requested PM handoff following the 2026-07-28 review of OpenSandbox's [Kubernetes network-isolation guidance](../../../docs/ADR042-sandbox-cluster-substrate.md) against bex's live integration. The review found a trust-domain mismatch (`allow-same-namespace` makes every sandbox in a workspace mutually reachable), an accepted-but-unenforced `networkPolicy`, unrestricted DNS as an exfiltration channel, gVisor incompatibility with OpenSandbox's iptables sidecar, residual cluster-wide lifecycle-server mutation RBAC, a template-only gVisor claim that compromised controllers could bypass with a runc Pod, and owner metadata that is returned but not persisted/enforced.
- **Goal linkage:** pillar 5 hosted execution environments ([ADR008](../../../docs/ADR008-vision.md)) and tenant isolation ([ADR043](../../../docs/ADR043-tenant-namespace-isolation.md)). Running untrusted agent-generated code is only a product capability if the sandbox boundary survives malicious code and a compromised lifecycle component.
- **Expected outcome:** each sandbox becomes an independently testable security domain; the only allowed data/control paths are explicit, egress is enforceable under gVisor, and an OpenSandbox compromise is contained to tenant sandbox namespaces rather than the cluster.
- **Why now:** m33 command execution landed concurrently while this hardening was being prepared. That makes the audited gaps reachable attacker primitives rather than latent substrate weaknesses, so m35 is now an urgent production gate before any further sandbox expansion.
- **Render parity:** INCLUDED because `networkPolicy`, sandbox visibility/ownership, lifecycle errors, and CLI behavior are tenant-facing REST/GraphQL/MCP/CLI contracts. Security tightening must not produce contradictory surface semantics.
- **Gap analysis:** m32 covered cross-tenant namespace scoping and recorded RBAC/GitOps follow-ups; m33 covers exec transport/auth tokens. Neither covers same-workspace sandbox-to-sandbox isolation, DNS exfiltration, gVisor-compatible FQDN policy, truthful `networkPolicy`, durable per-sandbox ownership, or API-server confinement for the NamespaceReconciler's dynamic `bind` authority. m35 owns those gaps and absorbs m32's RBAC/GitOps security follow-ups without duplicating exec work.
