# w2 · m59 — P0 operator isolation: contain untrusted execution

**Worker:** worker2 **Goal:** Make build, static-publish, and pre-deploy execution a proven untrusted boundary, and make tenant-image verification select the workloads it is meant to protect. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the adversarial isolation contract | 45m | — |
| t002 | Move untrusted execution out of `bex-system` | 60m | t001 |
| t003 | Disable workload API tokens and scope identities | 30m | t002 |
| t004 | Enforce build-boundary network and placement policy | 60m | t002 |
| t005 | Remove the BuildKit process/credential co-tenancy gap | 60m | t003, t004 |
| t006 | Make tenant-image admission select and fail closed | 45m | t001 |
| t007 | Simplify the isolation implementation | 30m | t005, t006 |
| t008 | Complete adversarial and regression coverage | 60m | t007 |
| t009 | Close out m59 | 15m | t008 |

## Definition of done

Rendered production manifests and a live adversarial test prove that build, publish, and pre-deploy Pods have no Kubernetes API token; cannot reach metadata, kubelet, platform services, other tenant services, or other registry repositories; cannot extract registry/signing credentials from a malicious Dockerfile; and can reach only the explicitly required build destinations. When tenant signing is enabled, an unsigned tenant image is denied by a ready, fail-closed webhook that actually selects tenant Pods.

## Source + Goal linkage

- **Source:** `docs/ADR039-operator-audit-and-platform-reuse.md` O-01 and O-02, which supersede the absolute credential-isolation claim in ADR022/ADR034. This explicitly reopens the affected parts of completed `w7/m8`, `w7/m11`, and `w1/m5` with new code/manifests evidence rather than duplicating their original scope.
- **Goal linkage:** ADR008's secure multi-tenant Render alternative and `GOAL.md` #5/#7 (multi-tenancy and security review).
- **Expected outcome:** Tenant-controlled build steps and images execute inside a narrow, test-proven boundary; enabling image verification cannot silently bypass every workload.
- **Why now:** O-01 is Critical and O-02 is a latent High activation failure. Both block real multi-tenant source builds. Render parity closing is omitted because this milestone changes only operator/GitOps security mechanisms and intentionally preserves every REST/GraphQL/MCP/UI contract.
