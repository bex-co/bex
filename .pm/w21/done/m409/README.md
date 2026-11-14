# w21 · m409 — Admission policies bind every identity the round-25 audit named: the activator, a projected crown Secret, a platform toleration, hostPort, and the drain-guard's pod-template fields

**Worker:** worker21 **Goal:** the ValidatingAdmissionPolicies that bound the operator, the activator and drain-guard refuse the privilege paths the round-25 audit found, each with a denial test that fails without its clause, and the real flows still pass. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Confine drain-guard to an App's own Deployment and to the pod-template annotation alone — **DONE** | 45m | — |
| [t002](done/t002.md) | Close the operator-workloads policy's projected-volume, platform-toleration and hostPort gaps — **DONE** | 75m | — |
| [t003](done/t003.md) | Bind the activator ServiceAccount: Apps annotations and Deployment replicas 0 to 1 only — **DONE** | 75m | — |
| [t004](done/t004.md) | Pin the new and changed policies in gitops-validate and the docs — **DONE** | 30m | t001, t002, t003 |
| [t005](done/t005.md) | Run the operator's, the activator's and drain-guard's real writes against the production policies — **DONE** | 45m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

1. `bex-drain-guard-writes` refuses a pod-template `finalizers`, `ownerReferences`, `generateName`, `name` or `namespace` change and an evacuation annotation on a Deployment that is not an App's.
2. `bex-operator-workloads` refuses a `projected` volume that sources a crown Secret, a `bex.co/platform` toleration and a container `hostPort`, and the day-to-day policy refuses the first two; the real workload projections are still admitted.
3. A policy binds the activator ServiceAccount to the two wake annotations on Apps and `spec.replicas` 0 to 1 on Deployments, and denies everything else it could write.
4. Every denial is a test that fails before its clause and passes after, `bash scripts/gitops-validate.sh` pins the clauses, and the operator's, the activator's and drain-guard's own code passes the policies against a real API server. The live-cluster run of the same flows, which needs `scripts/mock-cluster.sh` and was not possible on the machine this ran on, is w21/079.

## Source + Goal linkage

- **Source:** inbox note w21/068 (promoted 2026-11-14), from the w21/m394 round-25 audit (ADR110 § Residuals), whose hardening notes found each of these as a path to more privilege than the design intends, reachable only after a compromise of the component's own token.
- **Goal linkage:** the tenant-isolation and security-review lineage (ADR043, ADR028 to ADR110): a ValidatingAdmissionPolicy is the bound RBAC cannot express, and a component without one is trusted by default.
- **Expected outcome:** a compromised activator can touch only its two annotations and replicas 0 to 1, a compromised drain-guard only the evacuation annotation of an App, and the operator cannot project a crown Secret, tolerate the platform taint or open a host port.
- **Why now:** each gap is cheap to close while its harness (envtest with the production policy and an impersonated identity) already exists, and a gap with no test is one a later policy edit reopens unnoticed.
- **Render parity task:** omitted. Cluster admission hardening of platform identities; no REST, GraphQL, MCP or dashboard behavior changes and no customer-facing Render behavior is touched.

## Open decisions

None needing the user.

## Findings

### What the audit's gaps were, and what closes each (t001 to t003)

| Gap | Policy | Closed by | Denial test (fails without the clause) |
| --- | --- | --- | --- |
| A pod template's `finalizers`, `ownerReferences`, `generateName`, `name` and `namespace` unchecked (they are copied onto every pod); the evacuation annotation allowed on any Deployment | `bex-drain-guard-writes` | five template-metadata equalities and the selector's `app.bex.co/app` label equal to the Deployment's name, the shape drain-guard's own selection already requires | six new rows in `TestAdmissionPolicyConfinesDrainGuard`, all allowed before the change |
| A `projected` volume sourcing a crown Secret | `bex-operator-workloads`, `bex-operator-daytoday-workloads` (the build Job policy allows only `emptyDir` and `secret` volumes) | the volume rule reads `projected.sources[].secret` | `projected-crown-secret` and `-beside-another` in the shared table, against both identities |
| A toleration of `bex.co/platform` (the operator never authors one; the platform charts do) | all three | the key joins the refused set | `platform-toleration` and `-exists`, against the operator, the day-to-day credential and, through `bex-build`, any Job author |
| A container `hostPort` | all three | a rule over containers, init containers and ephemeral containers | `host-port` and `init-host-port` in the same table |
| No policy bound the activator | new `bex-activator-writes` | an App may change only `app.bex.co/last-active` and `app.bex.co/wake-started` (UTC RFC 3339); a Deployment only `spec.replicas` from none or zero to one; nothing else, create and delete included | 26 cases in `cmd/activator/admission_test.go`, each clause mutated: always-true, apps unrestricted, any replica value, a scale-down, a changed template, any timestamp |

The cases are one table (`admissionPrivilegePaths`) run against every identity. Before this the day-to-day policy and `bex-build-job-shape` had no behavior test at all: the day-to-day credential is `system:serviceaccount:kube-system:bex-operator`, which the spec never impersonated, and `bex-build-job-shape` is masked for the operator by `bex-operator-workloads`, so it is exercised through an author only it binds, in a `bex-build` namespace. `scripts/gitops-validate.sh` pins the five objects and each clause, counting the three that repeat per policy (2 projected, 3 platform, 9 hostPort lines) and the DELETE short-circuit count (8 to 9), and it had pinned neither the drain-guard nor the activator policy before; each pin was shown to fail with its clause removed.

### Real flows (t005)

The operator's real producers, the activator's `wakeApp` and drain-guard's `evacuate` each run against an API server that holds the production policy, as the confined identity; `wakeApp` logs a refused write instead of returning it, so the test reads the annotations and replicas back. A policy that refuses the real replicas write or the real timestamp format fails the wake test. The live cluster could not be brought up (no `kind` or `clusterctl`, helm artifact downloads answering 504), so that run is [079](../../079.md).

### Simplify (t006)

Reviewed by hand, not by agents: the diff is policy YAML, three tests and a shell pin, and it follows the files' existing idioms. The host-port rule appears once per policy on purpose, since a ValidatingAdmissionPolicy stands alone; the privilege-path cases are written once and shared. One slip found and fixed on the way: the activator test's readiness probe wrote to a Deployment that did not exist, so it never saw a denial.

### Test coverage (t007)

Every clause has a test that fails when it is removed; the mutations were run, not assumed: the three operator policies' projected, platform and hostPort clauses one at a time (each caught at the exact case), the activator's six clauses (the first run missed a scale-down, so `replicas 3 down to 1` was added), the drain-guard selector and template-metadata clauses, and each `gitops-validate.sh` pin.

