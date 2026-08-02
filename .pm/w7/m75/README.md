# w7 · m75 — CI & infrastructure supply-chain pinning

**Worker:** worker7 **Goal:** eliminate the one injectable CI input and stop every privileged CI/infra path from consuming a mutable, unverified upstream artifact. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------ | ---- | ---------- |
| t001 | snapshot.yml — stop kubernetes_version shell injection             | 30m  | —          |
| t002 | gVisor — pin release + verify checksum                             | 30m  | —          |
| t003 | Argo CD install manifest — pin to a tagged release                 | 20m  | —          |
| t004 | Argo CD Applications — pin exact chart versions                    | 30m  | —          |
| t005 | k3s DR installer — pin version + verify checksum                   | 30m  | —          |
| t006 | Simplify                                                          | 20m  | t001–t005  |
| t007 | Test coverage                                                    | 30m  | t006       |
| t008 | Closeout                                                         | 10m  | t007       |

## Definition of done

No privileged CI/infra path consumes a mutable, unverified, or injectable upstream artifact: the snapshot workflow validates its `kubernetes_version` input and passes it via env (no shell interpolation); gVisor, the Argo CD install manifest, and the k3s installer are pinned to explicit versions and integrity-checked where feasible; the four Argo CD Applications use exact chart versions with a reviewed bump process. w7/m65's action SHA-pinning is orthogonal and already done — this milestone covers runtime-fetched/interpolated artifacts, not action versions.

## Source + Goal linkage

- **Source:** codex-security scan findings #16 (medium, snapshot.yml command injection), #18 (gVisor), #19 (Argo CD manifest), #20 (Argo wildcard revisions), #27 (k3s installer) — validated against HEAD.
- **Goal linkage:** Security pillar — CI/CD + supply-chain integrity (ADR028; ADR016/ADR001 GitOps boundaries).
- **Expected outcome:** a compromised mutable upstream tag/URL or a quote-bearing workflow input can no longer reach a privileged shell or a production cluster.
- **Why now:** #16 is the most immediately exploitable item in the supply-chain set (expression-injection into a `HCLOUD_TOKEN`-bearing shell) and the pinning items are cheap, high-confidence hardening.
- **Render parity omitted:** CI/infra integrity; no REST/GraphQL/MCP/UI surface change.
