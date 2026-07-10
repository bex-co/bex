# w7 · m2 — Tenant workload hardening: Pod Security baseline + quotas + token hygiene

**Worker:** worker7 **Goal:** A hostile or compromised tenant workload can't escalate to the node or carry cluster credentials: Pod Security `baseline` enforced on the apps namespace, operator-set securityContext defaults, no ServiceAccount tokens in tenant pods, aggregate namespace quotas, and resource-bounded build Jobs. **Status:** done

## Tasks (in order)

| id   | title                                                                                              | est | depends_on   |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Enforce PSS `baseline` on the apps namespace                                                        | 30m | w7/m1/t001   | — **DONE** |
| t002 | Operator pod-spec defaults: drop capabilities, RuntimeDefault seccomp, no SA token automount        | 45m | t001         | — **DONE** |
| t003 | ResourceQuota + LimitRange on the apps namespace; resource limits on BuildKit Jobs                  | 45m | —            | — **DONE** |
| t004 | Verification: hostile spec rejected, no SA token, build limits, samples still Ready                 | 30m | t002, t003   | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                         | 20m | t004         | — **DONE** |
| t006 | Test coverage — meaningful tests for securityContext defaults + build-Job limits                    | 30m | t004         | — **DONE** |
| t007 | Closeout — DoD verified, milestone moved to `done/`                                                 | 15m | t006         | — **DONE** |

## Definition of done

On a cluster with the apps namespace configured: a privileged / hostPath / hostNetwork tenant pod spec is **rejected at admission**; a reconciled tenant pod has no ServiceAccount token mounted, `RuntimeDefault` seccomp, and dropped capabilities; the apps namespace carries a ResourceQuota + LimitRange; BuildKit Jobs carry cpu/mem limits; `examples/whoami-app.yaml` and `examples/hello-go` still reach Ready unchanged.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-09, take 2). Verified 2026-07-09: no ResourceQuota/LimitRange or `pod-security.kubernetes.io` labels anywhere in `lego/` or `deploy/gitops/`; no `automountServiceAccountToken` in the operator (tenant pods get SA tokens); `lego/operator/internal/build/build.go` sets deadline/backoff/TTL but no `Resources`.
- **Goal linkage:** GOAL.md V0 #7 (security review); pairs with m1 as the namespace-tier of the DO_NOT_DO isolation ladder (m1 = network axis, m2 = runtime/credential axis).
- **Expected outcome:** privilege escalation and credential theft from a tenant container are blocked at admission and at the pod spec; a runaway tenant or build can't starve the node.
- **Why now:** cheapest before real tenants exist (w1/m9); `baseline` (not `restricted`) is deliberate — tenant images may run as root, and forcing `runAsNonRoot` would break arbitrary user images (recorded in the m1 ADR).
- **Render parity: omitted** — pure mechanism (admission labels, operator pod-spec defaults, quotas); no REST/GraphQL/MCP/UI surface change.
