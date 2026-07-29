# w3 · m32 — Sandbox cluster substrate: multi-node OpenSandbox + Kata + Render CLI `ea sandbox`

**Worker:** worker3 **Goal:** realize the re-opened pillar 5 per ADR042 — stand up a multi-node OpenSandbox Kubernetes-runtime substrate with Kata isolation, a single-trusted-hop security model, and a bex-api surface that makes `render ea sandbox create/exec/list/stop` work unmodified. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                       | est | depends_on            |
| ---- | -------------------------------------------------------------------------------------------------------------------------- | --- | --------------------- |
| t001 | `opensandbox-server` container image (Dockerfile, push to Zot)                                                             | 2h  | —                     |
| t002 | OpenSandbox control-plane server in-cluster (Deployment/Service/ConfigMap/PVC/SA+Role) + cluster TOML (k8s runtime, in-cluster SA, batchsandbox template) | 3h  | t001                  |
| t003 | Snapshot registry → in-cluster Zot + push/pull secrets; warm `Pool`; execd in-cluster                                      | 2h  | t002                  |
| t004 | Kata `RuntimeClass` + bake kata-containerd into the worker image (CAPH) + dedicated sandbox node pool (tainted)            | 1d  | —                     |
| t005 | Per-tenant `<ws>-sandbox` boundary: egress-deny/metadata-deny (mirror build-boundary), Kata scheduling                      | 2h  | w3/m31/t001, t004      |
| t006 | Security — single trusted hop: NetworkPolicy admit-only-bex-api on opensandbox-system + OpenSandbox multi-tenant mode (per-workspace key → `<ws>-sandbox` via HTTP provider → bex IAM) | 3h  | t002, t005             | — **DONE** |
| t007 | k3s validation node (containerd-CRI + Kata) + validate BatchSandbox lifecycle/snapshot round-trip (rootfs-only) under Kata + Cilium | 1d  | t003, t004, t006       |
| t008 | Capture Render CLI `ea sandbox` wire shape (`render-oss/cli`) → `docs/render-artifacts`                                    | 1h  | —                     |
| t009 | bex-api sandbox feature package: `lego/backend/internal/sandbox` (service/client/rest/mcp/graphql) + template registry + 5-point wiring (consumes t006's per-workspace keys) | 1d  | t007, t008, w3/m31/t010 |
| t010 | `cli-compatibility-checklist` rows 238-241 `[-]`→`[x]` + `scripts/cli-compat.sh` sandbox leg (unmodified CLI)               | 2h  | t009                  |
| t011 | Render parity (`ea sandbox` across REST/MCP + CLI compat)                                                                  | 1h  | t010                  |
| t012 | Simplify (`/simplify` over changed code)                                                                                   | 30m | t011                  |
| t013 | Test coverage (sandbox client, lifecycle, authz/quota, snapshot round-trip)                                                | 3h  | t011                  |
| t014 | Closeout                                                                                                                   | 15m | t013                  |

## Definition of done

`render ea sandbox create/exec/list/stop` works against bex unmodified (`cli-compatibility-checklist` rows 238-241 green via a `scripts/cli-compat.sh` sandbox leg); sandboxes run on a multi-node OpenSandbox Kubernetes-runtime substrate in per-tenant `<ws>-sandbox` namespaces under Kata; the bex-api↔OpenSandbox link is single-trusted-hop (NetworkPolicy admit-only-bex-api + api_key + OpenSandbox multi-tenant mode); pause/resume round-trips the rootfs (v1 rootfs-only — memory hibernation is a documented watch item, not v1); validated on a k3s node (real containerd-CRI), not the OrbStack mock.

## Source + Goal linkage

- **Source:** [docs/ADR042-sandbox-cluster-substrate.md](../../docs/ADR042-sandbox-cluster-substrate.md) (re-opens pillar 5; refines [ADR014](../../docs/ADR014-sandboxes.md)); pillar-5 re-open recorded in `.pm/DO_NOT_DO.md` #18 (2026-07-27).
- **Goal linkage:** pillar 5 — hosted agent execution environments ([ADR008-vision](../../docs/ADR008-vision.md)).
- **Expected outcome:** agents (and the Render CLI) can spawn/exec/pause/resume sandboxes against bex, backed by a real multi-node substrate instead of the single-host Docker toy.
- **Why now:** pillar 5 re-opened 2026-07-27; depends on m31's per-tenant `<ws>-sandbox` namespace model so hosting and sandbox share one tenancy model.
- **Render parity:** INCLUDED — `render ea sandbox` is a CLI/REST surface ([cli-compatibility-checklist:238-241](../../docs/cli-compatibility-checklist.md), today 404).
- **Cross-theme note:** this is AI-native/sandbox work, not observability; placed in w3 by explicit user direction (2026-07-27), recorded in `.pm/DO_NOT_DO.md` #18.
