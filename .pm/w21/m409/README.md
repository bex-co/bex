# w21 · m409 — Admission policies bind every identity the round-25 audit named: the activator, a projected crown Secret, a platform toleration, hostPort, and the drain-guard's pod-template fields

**Worker:** worker21 **Goal:** the ValidatingAdmissionPolicies that bound the operator, the activator and drain-guard refuse the privilege paths the round-25 audit found, each with a denial test that fails without its clause, and the real flows still pass. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Confine drain-guard to an App's own Deployment and to the pod-template annotation alone | 45m | — |
| [t002](t002.md) | Close the operator-workloads policy's projected-volume, platform-toleration and hostPort gaps | 75m | — |
| [t003](t003.md) | Bind the activator ServiceAccount: Apps annotations and Deployment replicas 0 to 1 only | 75m | — |
| [t004](t004.md) | Pin the new and changed policies in gitops-validate and the docs | 30m | t001, t002, t003 |
| [t005](t005.md) | Check the operator, the activator wake and a drain still work on the local cluster | 45m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 20m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

1. `bex-drain-guard-writes` refuses a pod-template `finalizers`, `ownerReferences`, `generateName`, `name` or `namespace` change and an evacuation annotation on a Deployment that is not an App's.
2. `bex-operator-workloads` refuses a `projected` volume that sources a crown Secret, a `bex.co/platform` toleration and a container `hostPort`, and the day-to-day policy refuses the first two; the real workload projections are still admitted.
3. A policy binds the activator ServiceAccount to the two wake annotations on Apps and `spec.replicas` 0 to 1 on Deployments, and denies everything else it could write.
4. Every denial is a test that fails before its clause and passes after, `bash scripts/gitops-validate.sh` pins the clauses, and the sample App deploys and wakes on the local cluster with no policy denial in the logs.

## Source + Goal linkage

- **Source:** inbox note w21/068 (promoted 2026-11-14), from the w21/m394 round-25 audit (ADR110 § Residuals), whose hardening notes found each of these as a path to more privilege than the design intends, reachable only after a compromise of the component's own token.
- **Goal linkage:** the tenant-isolation and security-review lineage (ADR043, ADR028 to ADR110): a ValidatingAdmissionPolicy is the bound RBAC cannot express, and a component without one is trusted by default.
- **Expected outcome:** a compromised activator can touch only its two annotations and replicas 0 to 1, a compromised drain-guard only the evacuation annotation of an App, and the operator cannot project a crown Secret, tolerate the platform taint or open a host port.
- **Why now:** each gap is cheap to close while its harness (envtest with the production policy and an impersonated identity) already exists, and a gap with no test is one a later policy edit reopens unnoticed.
- **Render parity task:** omitted. Cluster admission hardening of platform identities; no REST, GraphQL, MCP or dashboard behavior changes and no customer-facing Render behavior is touched.

## Open decisions

None needing the user.
