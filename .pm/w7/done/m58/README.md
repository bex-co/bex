# w7 · m58 — Platform-pod hardening parity + universal CI guard

**Worker:** worker7 **Goal:** bring the one unhardened platform component (static-server) up to the securityContext baseline every sibling already carries, and add a CI structural guard so no future platform workload can ship without it — because `bex-system` is deliberately privileged-PSS (egress-meter needs BPF caps) and therefore has no namespace PodSecurity backstop. **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |            |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Harden the static-server Deployment to platform-pod parity                                | 45m | —          | — **DONE** |
| t002 | Audit + align remaining platform workloads; build the documented exemption list           | 45m | —          | — **DONE** |
| t003 | Universal `gitops-validate` guard: every platform workload carries the hardened context   | 60m | t001, t002 | — **DONE** |
| t004 | Simplify                                                                                   | 20m | t003       | — **DONE** |
| t005 | Test coverage: guard goes red on any non-exempt unhardened platform workload              | 30m | t003       | — **DONE** |
| t006 | Closeout                                                                                   | 15m | t005       | — **DONE** |

## Outcome (2026-07-30)

Every platform `Deployment`/`DaemonSet` under `lego/operator/config` now carries the hardened baseline (pod `runAsNonRoot:true` + `seccompProfile: RuntimeDefault`; each container `allowPrivilegeEscalation:false` + `readOnlyRootFilesystem:true` + `capabilities.drop:[ALL]`), enforced by a universal, enumerated, fail-closed CI guard.

**Platform-pod hardening matrix (final):**

| Workload | Disposition |
| --- | --- |
| bex-api, ssh-gateway, manager, activator | already hardened (unchanged) |
| **static-server** | **hardened** (t001) — was the only workload with no securityContext at all; writes no temp files so `readOnlyRootFilesystem` needs no emptyDir |
| **pg-sni-proxy** | **hardened** (t002) — added pod `runAsNonRoot`+`seccompProfile` and container non-root uid 65532 (hostPort binding is kubelet-side, does not need root) |
| **kv-sni-proxy** | **hardened** (t002) — added pod `seccompProfile`+`runAsNonRoot` (already ran as non-root uid 65532 at the container level) |
| **egress-meter** | **EXEMPT** (sole entry) — deliberately privileged: `hostNetwork:true` + `runAsNonRoot:false` + `capabilities.add:[BPF,NET_ADMIN,PERFMON,SYS_ADMIN,SYS_RESOURCE]` for post-policy L3 metering |

**DoD scoping correction:** the source note assumed only pg-sni-proxy was under-baseline and that kv-sni-proxy already had `seccompProfile` — but **both** sni-proxies lacked `seccompProfile`; both were brought to full parity.

Guard: `scripts/gitops-validate.sh` reads the single-sourced invariant `scripts/lib/platform-pod-hardening-guard.yq` (so guard and self-test can't drift) and fails on any non-exempt workload below baseline — proven red by reverting static-server (`FAIL: ...below the securityContext hardening baseline (w7/m58): bex-static-server`). `scripts/gitops-hardening-guard-test.sh` is a durable self-test (no-securityContext / each-field-missing / exemption-scoped), wired into `.github/workflows/gitops.yml` alongside the guard.

## Definition of done

- `lego/operator/config/staticserver/deployment.yaml` carries pod-level `runAsNonRoot: true` + `seccompProfile: RuntimeDefault` and container-level `readOnlyRootFilesystem: true` + `allowPrivilegeEscalation: false` + `capabilities.drop: [ALL]`, matching bex-api / ssh-gateway / manager / activator; the static-server still serves static content and reads S3 under the read-only rootfs (any writable temp path provided via an `emptyDir`, not a writable rootfs).
- Every `Deployment`/`DaemonSet` under `lego/operator/config` is either at the hardened baseline or on a documented exemption list; the sole legitimate exemption is egress-meter (privileged for BPF/NET_ADMIN + hostNetwork). pg-sni-proxy's missing `runAsNonRoot`/`seccompProfile` (present on kv-sni-proxy) is either closed or explicitly exempted with a recorded reason.
- `scripts/gitops-validate.sh` fails when any non-exempt platform workload drops below the hardened set — proven by temporarily reverting the static-server fix and observing the guard go red — and CI enforces it via `.github/workflows/gitops.yml`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w7` 2026-07-30 — code sweep of `lego/operator/config/*/deployment.yaml` found static-server the only platform Deployment with no `securityContext` (bex-api, ssh-gateway, manager, activator, pg-sni-proxy, kv-sni-proxy all hardened; egress-meter deliberately privileged). `scripts/gitops-validate.sh` guards egress-meter caps + the opensandbox lifecycle-server hardening but has **no universal platform-pod hardening assertion** — which is why static-server slipped through.
- **Goal linkage:** GOAL.md V0 #7 (security review). Extends **m2** — which hardened *tenant* Apps (PSS baseline + pod securityContext defaults), not bex's own component Deployments — to the platform's pods, and mirrors **m7/m30**'s CI-structural-guard charter (invariant enforced in CI, not left to review).
- **Expected outcome:** the internet-facing static-server no longer runs as an unhardened root-capable container; a future unhardened platform pod fails CI instead of silently shipping.
- **Why now:** `bex-system` is intentionally privileged-PSS (egress-meter's BPF/NET_ADMIN caps + hostNetwork), so there is **no** namespace PodSecurity backstop — the per-pod securityContext is the only control, and the one component missing it serves tenant-controlled static content on the public edge. The gap is latent today; the guard makes the fix permanent.
- **Render parity:** **omitted** — pure platform manifest/infra + CI change, no REST/GraphQL/MCP/UI surface (the m30/m7/m33 precedent). Not a duplicate of m2 (tenant workloads) or m57 (billing/static-publish/namespace *planes* + secret custody — not platform-pod runtime hardening).
