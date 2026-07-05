# w1 · m7 — Prod hardening: network · secrets · images

**Worker:** worker1 **Goal:** Close the deliberate demo shortcuts so the cluster is more than a demo: firewall the API, pin the operator image, encrypt secrets + node traffic, and get a stable LB IP on a private network. **Status:** todo

## Tasks (any order — mostly independent)

| id   | title                                             | est | depends_on |
| ---- | ------------------------------------------------- | --- | ---------- |
| t001 | Firewall the app nodes (Hetzner firewall, TF)     | 30m | —          |
| t002 | Immutable operator image (SHA-pin, not `:latest`) | 30m | —          |
| t003 | Secrets at rest (sealed-secrets / SOPS)           | 30m | —          |
| t004 | Encrypt node-to-node (Cilium WireGuard)           | 25m | —          |
| t005 | Private network + stable LB IP (Traefik LB)       | 30m | t004       |

## Definition of done

The kube-API is firewalled (not internet-exposed); the operator runs a SHA-pinned image; repo/registry/Hetzner creds are encrypted at rest; node traffic is WireGuard-encrypted; Traefik has a stable LB IP on a Hetzner private network.

## Source

Converted from `.tmp/010-prod-hardening.md`.
