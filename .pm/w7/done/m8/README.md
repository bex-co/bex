# w7 · m8 — Tenant registry authn/z (close the unauthenticated Zot hole)

**Worker:** worker7 **Goal:** The in-cluster Zot registry no longer accepts unauthenticated access from tenant-controlled build code: pushes require a credential tenant `RUN` steps cannot read, reads follow a documented policy, and a CI guard fails if auth is ever removed — closing cross-tenant image enumeration, pull (source disclosure), and tag-overwrite poisoning. **Status:** done

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on       | status       |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------------- | ------------ |
| t001 | Design the registry credential scheme (htpasswd + accessControl; shared vs per-App creds; read policy) | 30m | —                | — **DONE** |
| t002 | Gitops: enable Zot auth — htpasswd Secret + accessControl config, pin the chart version                | 45m | t001             | — **DONE** |
| t003 | Operator: build Job authenticates its push (docker-config Secret in pod fs, new env knob)              | 45m | t001             | — **DONE** |
| t004 | Tenant pulls under the t001 read policy — imagePullSecrets or documented residual                      | 30m | t001             | — **DONE** |
| t005 | Verification: live registry-auth probes + `gitops-validate.sh` CI guard                                | 45m | t002, t003, t004 | — **DONE** |
| t006 | Simplify — `/simplify` over the code/manifests this milestone changed                                  | 20m | t005             | — **DONE** |
| t007 | Test coverage — meaningful tests for the registry-auth behavior this milestone shipped                 | 30m | t005             | — **DONE** |
| t008 | Closeout — DoD verified, milestone moved to `done/`                                                    | 15m | t007             | — **DONE** |

## Definition of done

On a live cluster with the shipped config: an unauthenticated `curl` to Zot's `/v2/_catalog` and an unauthenticated push are both refused (401/403) per the t001 policy (write is always authenticated; read per the documented decision); a normal build-from-git App still builds, pushes, deploys, and serves end-to-end; `scripts/gitops-validate.sh` fails if the Zot values lose their auth/accessControl config. The registry credential is never readable from tenant Dockerfile/CNB `RUN` steps (pod-filesystem mount only — never a build-arg or a declared BuildKit secret).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w7` (2026-07-12). Verified 2026-07-12: `deploy/gitops/base/zot.yaml:16` ships "local/dev defaults" with no auth configured; `lego/operator/internal/build/build.go:184` pushes with `registry.insecure=true` and no credential; `deploy/gitops/base/network-policies.yaml:64-68` allows build-labeled pods — which execute tenant-authored Dockerfile/CNB steps in the pod's network namespace — to reach Zot on :5000. So any tenant build can enumerate (`/v2/_catalog`), pull, or push over any tenant's images today.
- **Goal linkage:** GOAL.md V0 #7 (security review); cross-tenant isolation — the same class m1/m4 closed at the network layer, still open at the registry layer — plus supply-chain integrity complementing w6/006 tenant-image signing.
- **Expected outcome:** tenant build code loses read/write access to other tenants' images; the poisoned-image path (overwrite a tag → a fresh autoscaled node pulls the poisoned layer) is closed; a CI guard prevents silent regression.
- **Why now:** the last verified cross-tenant hole on the board; w6/006's opt-in signing is weak insurance while anyone can push unauthenticated. Sequence: before real tenants, like the rest of w7.
- **Render parity: omitted** — registry authentication is internal platform/operator mechanism with no REST/GraphQL/MCP/UI surface change. Closing tasks are Simplify → Test coverage → Closeout only.
