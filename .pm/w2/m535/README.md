# w2 · m535 — Sandbox Pod wiring, Cilium egress policies & broker CA trust across create, suspend/resume & snapshots

**Worker:** worker2 **Goal:** Wire `/v1/sandboxes` Pods to `w2/m534`'s credential broker across create, idle suspend/resume, and filesystem snapshot restore — stamping immutable group labels, distributing the public Sandbox Broker CA into standard client trust stores, injecting `HTTPS_PROXY` with zero user config, and opening only the narrow Cilium egress path to `bex-ssh-gateway` while keeping `deny-all`, `allow-list`, and all platform isolation floors intact. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Provision Sandbox Broker CA Secret in `bex-system` (private key mounted only into `bex-ssh-gateway`) and project the public CA certificate read-only for sandbox Pods | 45m | `w2/m534` |
| t002 | Stamp immutable `bex.co/sandbox-group` label on every `/v1/sandboxes` Pod at create, resume, and snapshot restore | 45m | t001 |
| t003 | Inject `HTTPS_PROXY`/`https_proxy`/`NO_PROXY` and broker CA trust bundle/env vars into `/v1/sandboxes` Pods so standard HTTP clients work out of the box | 1h | t002 |
| t004 | Update Cilium egress policies and admission rules so `allow-all` and `allow-list` sandbox Pods reach `bex-ssh-gateway`'s broker port only, keeping `deny-all` sealed and platform floor denies intact | 1h | t002 |
| t005 | Verify suspend/resume and snapshot capture/restore preserve broker connectivity with zero secrets in rootfs or snapshots, and record the `HTTPS_PROXY` vs transparent-443 caveat in ADR104/ADR018 | 1h | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- A newly created, resumed, or snapshot-restored `/v1/sandboxes` Pod (`allow-all` or `allow-list`) routes outbound HTTPS from `curl`, `wget`, Python, Node, and Go through `bex-ssh-gateway`'s broker automatically, trusting the Sandbox Broker CA without user setup, and receives injected credentials for its group with zero secret values in `env` or the filesystem.
- A `deny-all` sandbox cannot reach `bex-ssh-gateway`'s broker port or any public host; an `allow-list` sandbox reaches only its listed domains through the broker; all node, kube-apiserver, metadata, private-CIDR, and cross-tenant denies remain enforced (`scripts/gitops-validate.sh` and admission tests pass).
- Adding, editing, disabling, or deleting a group credential takes effect on already-running sandboxes without recreating them.
- Suspended rootfs images and filesystem snapshot captures contain zero credential secret bytes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-18), materialized via `/pmx them all to w2`. Builds on [ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md) and [ADR104](../../../docs/ADR104-sandbox-render-parity.md).
- **Goal linkage:** Render parity first ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) — completes runtime delivery of sandbox credential brokering inside tenant sandboxes while preserving ADR042/ADR043 zero-trust tenant isolation.
- **Expected outcome:** Code running in a sandbox calls third-party HTTPS APIs authenticated by its group's brokered credentials without holding any API key.
- **Why now:** Connects `w2/m534`'s broker to real sandbox Pods across every lifecycle path (`create`, `resume`, `snapshot restore`) before focused security review and live production verification (`w2/m536`).
- **Render parity:** Omitted — cluster/Pod wiring, broker CA distribution, and Cilium policy only; no public API schema change (behavioral parity is verified end to end in `w2/m536`).
- **Depends on:** `w2/m534`.
- **Brainstorm recommendations, not user decisions:** Using `HTTPS_PROXY` + Broker CA trust rather than node-level transparent TCP-443 interception (Firecracker host layer on Render) is the brainstorm recommendation; clients that ignore `HTTPS_PROXY` or pin remote leaf certs bypass injection (without leaking secrets). Record this explicitly as an open divergence awaiting user sign-off in `ADR104` and `ADR018`.
