# w2/m39 production SSH preflight — 2026-07-15

This is sanitized partial evidence, not t009 completion. No private key, bearer token, kubeconfig, environment value, command output from a tenant shell, or terminal content is recorded.

## Deployed edge

- `bex-system/bex-ssh-gateway` reached 2/2 Ready on the production app cluster with the GitOps-pinned shared image.
- The out-of-band `bex-ssh-host-key` Secret is installed. Public Ed25519 fingerprint: `SHA256:cwBcvu8ou7s53NcHrFMxqd3955BuoJRhDx9mMMGFpNE`.
- Traefik's LoadBalancer exposes TCP/22. Public ingress addresses are `49.12.20.236` and `2a01:4f8:c01e:3d1f::1`; Hetzner additionally reports private ingress `10.10.0.7`, which the activation guard now excludes from the public-DNS equality check.
- Direct IPv4 `ssh-keyscan` returned the installed stable fingerprint. This client had no IPv6 route, so it could not independently scan the public IPv6 address.
- `ssh.bex.co` still resolved to Cloudflare proxy addresses. `scripts/ssh-activate.sh --check` rejected that mismatch before host scanning or Kubernetes mutation, and `bex-system/bex-ssh` remained absent.
- `scripts/ssh-dns-cloudflare.sh` now provides the missing least-privilege handoff: it derives the public ingress set, reconciles exact DNS-only A/AAAA records through Cloudflare's API, removes stale duplicates, and verifies the result. Offline tests cover read-only checks, proxied/stale rejection, update/delete, missing-family creation, private-ingress filtering, and token redaction. No Cloudflare credential was available locally, so it was not run against the production zone.

## Authorization boundary

Production `kubectl auth can-i` checks proved:

- gateway ServiceAccount: App/pod read and `pods/exec create` in `default` — allowed;
- gateway ServiceAccount: Secret read in `default` — denied;
- gateway ServiceAccount: `pods/exec create` in `bex-system` — denied;
- bex-api ServiceAccount: `pods/exec create` in `default` — denied.

## Direct IPv4 data-path smoke

A disposable paid two-replica `nginxinc/nginx-unprivileged:1.27-alpine` service and disposable Ed25519 client key exercised the production path through `49.12.20.236:22`:

- unknown key rejected before registration;
- registered key reached a random Ready instance and saw the expected runtime environment;
- a complete instance id selected that replica and propagated exit status 37;
- deleting the key rejected the next connection;
- the key, service, App CR, and owned workload resources were removed.

The initial root-oriented `nginx:1.27-alpine` fixture could not start under bex's dropped-capability tenant policy. The verifier now uses nginx's unprivileged image. The smoke also exposed and covered a store/direct-CR distinction: bootstrap-created Apps have public `srv-…` labels without Postgres source rows, so row-owned writes must be gated by the `managed-by` label, not app-id presence alone.

## Remaining live proof

- Supply an ephemeral zone-scoped Cloudflare DNS Edit token and run `BEX_SSH_HOST=ssh.bex.co CLOUDFLARE_ZONE_ID=<zone-id> CLOUDFLARE_API_TOKEN=<token> scripts/ssh-dns-cloudflare.sh`; then run the same command with `--check`. Omit `CLOUDFLARE_ZONE_ID` only when the token also has Zone Read. This must publish DNS-only `A 49.12.20.236` and `AAAA 2a01:4f8:c01e:3d1f::1`.
- Run the guarded non-check activation and prove bex-api advertises `sshAddress` only afterward.
- Run `scripts/ssh-verify.sh` through the hostname with raw OpenSSH and the unmodified official Render CLI v2.21.0 (`c398207`) in a TTY.
- Record restart/redeploy closure and the complete live denial matrix. Deterministic tests already cover these behaviors, but they do not substitute for t009's public-edge evidence.
