# w7 · m75 — CI & infrastructure supply-chain pinning

**Worker:** worker7 **Goal:** eliminate the one injectable CI input and stop every privileged CI/infra path from consuming a mutable, unverified upstream artifact. **Status:** done

## Tasks (in order)

| id   | title                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------ | ---- | ---------- |
| t001 | snapshot.yml — stop kubernetes_version shell injection — **DONE**  | 30m  | —          |
| t002 | gVisor — pin release + verify checksum — **DONE**                  | 30m  | —          |
| t003 | Argo CD install manifest — pin to a tagged release — **DONE**      | 20m  | —          |
| t004 | Argo CD Applications — pin exact chart versions — **DONE**         | 30m  | —          |
| t005 | k3s DR installer — pin version + verify checksum — **DONE**        | 30m  | —          |
| t006 | Simplify — **DONE**                                                | 20m  | t001–t005  |
| t007 | Test coverage — **DONE**                                           | 30m  | t006       |
| t008 | Closeout — **DONE**                                                | 10m  | t007       |

## Definition of done

No privileged CI/infra path consumes a mutable, unverified, or injectable upstream artifact: the snapshot workflow validates its `kubernetes_version` input and passes it via env (no shell interpolation); gVisor, the Argo CD install manifest, and the k3s installer are pinned to explicit versions and integrity-checked where feasible; the four Argo CD Applications use exact chart versions with a reviewed bump process. w7/m65's action SHA-pinning is orthogonal and already done — this milestone covers runtime-fetched/interpolated artifacts, not action versions.

## Source + Goal linkage

- **Source:** codex-security scan findings #16 (medium, snapshot.yml command injection), #18 (gVisor), #19 (Argo CD manifest), #20 (Argo wildcard revisions), #27 (k3s installer) — validated against HEAD.
- **Goal linkage:** Security pillar — CI/CD + supply-chain integrity (ADR028; ADR016/ADR001 GitOps boundaries).
- **Expected outcome:** a compromised mutable upstream tag/URL or a quote-bearing workflow input can no longer reach a privileged shell or a production cluster.
- **Why now:** #16 is the most immediately exploitable item in the supply-chain set (expression-injection into a `HCLOUD_TOKEN`-bearing shell) and the pinning items are cheap, high-confidence hardening.
- **Render parity omitted:** CI/infra integrity; no REST/GraphQL/MCP/UI surface change.

## Ship record — DONE 2026-08-01

Shipped as `29031c59` (deployed → GitOps pin `804d0fce`). Five fixes: snapshot.yml funnels `kubernetes_version` through a `KUBERNETES_VERSION` env + a strict `x.y.z` regex guard before it touches a `run:` script, closing the HCLOUD_TOKEN shell-injection (#16); deploy.yml applies Argo CD at the pinned `v3.4.6` manifest instead of the mutable `stable` branch (#19); the four Argo Applications (loki `6.55.0`, prometheus `25.27.0`, metrics-server `3.12.2`, alloy/log-shipper `1.3.1`) use exact chart versions instead of semver wildcards (#20); new sandbox nodes install gVisor from a pinned `release-20260622.0` and verify both binaries against the published `.sha256` (#18); the DR bootstrap installs a pinned k3s (`v1.34.9+k3s1` via `INSTALL_K3S_VERSION`, new `k3s_version` var) instead of the mutable stable channel (#27). Each pinned version is the verified current-latest in its track (what `stable`/wildcards already resolved to), frozen to an immutable ref. CI: clusterapi validate ✅, gitops render ✅, terraform validate ✅, all three test suites ✅, deploy ✅.
