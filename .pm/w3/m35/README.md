# w3 · m35 — Sandbox security hardening: per-sandbox boundary + least-privilege OpenSandbox

**Worker:** worker3 **Goal:** close the sandbox integration's known security gaps now that exec is exposed: make each sandbox an independent network and ownership domain, enforce a gVisor-compatible egress policy, and reduce the OpenSandbox lifecycle server to tenant-namespace-scoped authority. **Status:** todo (t001 done; implementation staged, production adversarial evidence pending)

## Tasks (in order)

| id   | title                                                                                 | est | depends_on                   |
| ---- | ------------------------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | Freeze the sandbox threat model and fail-closed security contract — **DONE**          | 45m | —                            |
| t002 | Remove sandbox same-namespace lateral access; add precise control/data-plane allows   | 90m | t001                         |
| t003 | Add gVisor-compatible Cilium DNS/FQDN egress enforcement and block DNS exfiltration   | 90m | t002                         |
| t004 | Make the public `networkPolicy` field enforced and honest                            | 60m | t003                         |
| t005 | Persist sandbox owner metadata and enforce owner/resource authorization               | 90m | t001                         |
| t006 | Remove lifecycle-server cluster-wide mutation RBAC and tighten server ingress         | 60m | t001                         |
| t007 | Make the hardened OpenSandbox deployment GitOps-durable and version-pinned            | 90m | t006                         |
| t008 | Run the adversarial same-workspace/egress/RBAC matrix on the real gVisor substrate    | 90m | t003, t004, t005, t007, t013, t014 |
| t009 | Render parity: security semantics across REST, GraphQL, MCP, and CLI                  | 45m | t008                         |
| t010 | Simplify (`$simplify` over the hardening diff)                                        | 30m | t009                         |
| t011 | Test coverage: generated policies, authz, RBAC, DNS, and negative reachability        | 60m | t009                         |
| t013 | Admission-confine the NamespaceReconciler's dynamic namespace and `bind` authority    | 60m | t006                         |
| t014 | Admission-enforce the sandbox Pod runtime boundary                                    | 45m | t006                         |
| t012 | Closeout                                                                              | 15m | t010, t011, t013, t014       |

## Current gate

The implementation and static regression suite are staged as of 2026-07-29: backend tests/build, GitOps rendering/guards, shell syntax, OpenSandbox config parsing, and API-server CEL compilation pass. Durable reads now require the complete owner/workspace/sandbox-regime/enforced-policy stamp; the lifecycle server has name-filtered DNS only to its fixed tenant resolver and the controller has no DNS egress; digest write-back rejects stale cross-commit images. The adversarial verifier now fails closed before creating fixtures unless the hardening resources, least-privilege grants, immutable workloads, admission policies, Cilium agent-local Hubble socket, the root platform Application, and both OpenSandbox Argo applications are healthy. Its explicit preflight-only mode cannot create API or Kubernetes fixtures even if the cluster becomes ready while an audit is running.

A read-only production audit on 2026-07-29 confirmed the substrate prerequisite (9/9 Ready nodes, one Ready sandbox node, `gvisor`/`runsc`, and Cilium 1.19.5 at 9/9 with L7 proxy enabled), but also confirmed that the hardening is **not deployed**: all 15 sandbox namespaces retain `allow-same-namespace`, raw `allow-dns-egress`, and the Secret-bearing operator RoleBinding; only the older node/metadata CCNP exists; neither admission policy exists; the lifecycle server retains cluster-wide mutation verbs and a mutable/manual image; and Argo has no lifecycle-server Application. A safe verifier preflight exited on the first missing CCNP before any fixture or API creation. Tasks t002-t014 therefore remain `todo`. Closing the milestone requires an authorized production GitOps rollout, reconciliation of all existing sandbox namespaces, four pre-provisioned owner/member/admin test identities, the full live matrix, then parity/simplification/closeout. No production apply was performed as part of this implementation pass.

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
