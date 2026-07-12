# w7 · m4 — Tenant egress hardening: block cloud metadata + node-local endpoints

**Worker:** worker7 **Goal:** A hostile or compromised tenant pod can no longer reach the cloud-metadata endpoint (`169.254.169.254`) or node-local services (kubelet, nodePorts on the node's public IP) that the m1 egress policy left open — closing the SSRF → credential-theft and pod→node access paths while genuine external egress keeps working. **Status:** done

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |            |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Except `169.254.0.0/16` (link-local incl. cloud metadata) in the tenant egress policy; document the threat  | 30m | —          | — **DONE** |
| t002 | Close the pod→node path: assess node-public-IP reachability (kubelet `:10250`, nodePorts) + add a deny      | 60m | t001       | — **DONE** |
| t003 | Extend `verify-tenant-isolation.sh` with DENY probes: metadata + node IP blocked, external egress allowed   | 30m | t002       | — **DONE** |
| t004 | Simplify — `/simplify` over the code this milestone changed                                                  | 20m | t003       | — **DONE** |
| t005 | Test coverage — meaningful tests for the extended egress policy generation                                   | 30m | t003       | — **DONE** |
| t006 | Closeout — DoD verified, milestone moved to `done/`                                                          | 15m | t005       | — **DONE** |

## Definition of done

On the prod-shaped cluster: from a tenant pod, a connection to `169.254.169.254` (cloud metadata) and to a node's public IP on `:10250` / a nodePort is **blocked**, while egress to a real external host (e.g. `https://example.com`) still **succeeds**; `scripts/verify-tenant-isolation.sh` proves the extended reachability matrix (metadata + node DENY probes added) and exits 0; the metadata/SSRF and node-path threat is written up in `docs/ADR022-tenant-isolation.md`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-11). Verified 2026-07-11: the m1 egress policy (`lego/operator/internal/controller/app_controller.go:1102-1137`) allows `0.0.0.0/0` except `{10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10}` — `169.254.0.0/16` (link-local incl. the Hetzner metadata service `169.254.169.254`) is **not** excepted, and node public IPs fall under the allowed `0.0.0.0/0`. `scripts/verify-tenant-isolation.sh` tests "internet egress must SUCCEED" but never asserts metadata/link-local is BLOCKED.
- **Goal linkage:** GOAL.md V0 #7 (security review); extends the m1 network-isolation axis of the `DO_NOT_DO.md` namespace-tier ladder (network → runtime → API).
- **Expected outcome:** tenant→cloud-metadata credential theft (Capital-One-class SSRF) and tenant→node-service access are closed at the CNI layer, and the reachability matrix proves it — a live hole in already-shipped m1 becomes closed and tested.
- **Why now:** this is a **live exposure in shipped m1** running on the prod Hetzner cluster today — the metadata endpoint is reachable from every tenant pod, and closing it is cheapest before real tenants exist to migrate.
- **Render parity: omitted** — pure mechanism (CNI egress policy + operator/Cilium policy generation + a verification script); no REST/GraphQL/MCP/UI surface change. This milestone's closing tasks are therefore Simplify → Test coverage → Closeout only.
