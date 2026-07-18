# w2/m39 production SSH preflight — 2026-07-15

This is sanitized partial evidence, not t009 completion. No private key, bearer token, kubeconfig, environment value, command output from a tenant shell, or terminal content is recorded.

## Deployed edge

- `bex-system/bex-ssh-gateway` reached 2/2 Ready on the production app cluster with the GitOps-pinned shared image.
- The out-of-band `bex-ssh-host-key` Secret is installed. Public Ed25519 fingerprint: `SHA256:cwBcvu8ou7s53NcHrFMxqd3955BuoJRhDx9mMMGFpNE`.
- Traefik's LoadBalancer exposes TCP/22. Public ingress addresses are `49.12.20.236` and `2a01:4f8:c01e:3d1f::1`; Hetzner additionally reports private ingress `10.10.0.7`, which the activation guard now excludes from the public-DNS equality check.
- Direct IPv4 `ssh-keyscan` returned the installed stable fingerprint. This client had no IPv6 route, so it could not independently scan the public IPv6 address.
- `ssh.bex.co` still resolved to Cloudflare proxy addresses. `scripts/ssh-activate.sh --check` rejected that mismatch before host scanning or Kubernetes mutation, and `bex-system/bex-ssh` remained absent.
- `scripts/ssh-dns-cloudflare.sh` now provides the missing least-privilege handoff: it derives the public ingress set, reconciles exact DNS-only A/AAAA records through Cloudflare's API, removes stale duplicates, and verifies the result. Offline tests cover read-only checks, proxied/stale rejection, update/delete, missing-family creation, private-ingress filtering, and token redaction. No Cloudflare credential was available locally, so it was not run against the production zone.
- A final read-only refresh after the gateway advanced to image digest `f63535ace321734ad3ca48d5099d5a317c67c62b48c1dfb5b50cd2c2f94cd54c` found it still 2/2 Ready and serving the same stable fingerprint over direct IPv4. DNS still returned only Cloudflare proxy addresses, the activation ConfigMap remained absent, and no in-scope Cloudflare credential had appeared.

## Authorization boundary

Production `kubectl auth can-i` checks proved:

- gateway ServiceAccount: App/pod read and `pods/exec create` in `default` — allowed;
- gateway ServiceAccount: Secret read in `default` — denied;
- gateway ServiceAccount: `pods/exec create` in `bex-system` — denied;
- bex-api ServiceAccount: `pods/exec create` in `default` — denied.

The live checks use `kubectl auth can-i create pods --subresource=exec`; this kubectl version returns a false negative for the superficially similar positional spelling `create pods/exec` even though its rule listing and the canonical subresource check agree.

## Direct IPv4 data-path smoke

A disposable paid two-replica `nginxinc/nginx-unprivileged:1.27-alpine` service and disposable Ed25519 client key exercised the production path through `49.12.20.236:22`:

- unknown key rejected before registration;
- registered key reached a random Ready instance and saw the expected runtime environment;
- a complete instance id selected that replica and propagated exit status 37;
- deleting the key rejected the next connection;
- the key, service, App CR, and owned workload resources were removed.

The initial root-oriented `nginx:1.27-alpine` fixture could not start under bex's dropped-capability tenant policy. The verifier now uses nginx's unprivileged image. The smoke also exposed and covered a store/direct-CR distinction: bootstrap-created Apps have public `srv-…` labels without Postgres source rows, so row-owned writes must be gated by the `managed-by` label, not app-id presence alone.

## Official CLI parity re-audit

- The current [official CLI release](https://github.com/render-oss/cli/releases/tag/v2.21.0), v2.21.0 (`c398207`), was downloaded from the macOS arm64 release asset and reports `render v2.21.0`. The locally computed SHA-256 is `3d721f8e5f26e8d920eec899c28b200e74901529ad5d964b180c5d09c7ad3546` for the release ZIP and `b936020f083a83f170b1eeae1b7e739ee533812f794c463e4cbae18ba8b550a8` for its extracted binary.
- Every upstream Go package except the credentialed `e2e` package passed at the exact v2.21.0 tag. The upstream e2e suite requires a Render account and was not used as bex evidence.
- Source review reconfirmed that service names and complete instance ids are working public arguments. Both instance-picker callbacks still assign the service id instead of their selected `instanceID`, so the acceptance harness deliberately uses the supported full-instance-id argument for exact targeting and does not claim picker compatibility.
- `scripts/ssh-verify.sh` accepts `BEX_RENDER_CLI_BIN` so the live run can execute that exact checksum-verified binary rather than an ambient `render` executable from `PATH`.
- A deterministic verifier test proves a missing pinned binary fails before fixture setup, and that the CLI's OpenSSH shim records the selected destination while enforcing batch authentication and the pinned host-key file. The GitOps job now runs this test and is triggered by every SSH activation, DNS, verifier, shared edge-helper, and operator-manifest path it guards.
- The focused race pass exposed a pre-existing unsynchronized API-key test fake used by root API integration tests. Locking that in-memory fake removed the race without changing production code; `go test -race ./internal/api` and the SSH package race tests now pass.

## Remaining live proof

- Supply an ephemeral zone-scoped Cloudflare DNS Edit token and run `BEX_SSH_HOST=ssh.bex.co CLOUDFLARE_ZONE_ID=<zone-id> CLOUDFLARE_API_TOKEN=<token> scripts/ssh-dns-cloudflare.sh`; then run the same command with `--check`. Omit `CLOUDFLARE_ZONE_ID` only when the token also has Zone Read. This must publish DNS-only `A 49.12.20.236` and `AAAA 2a01:4f8:c01e:3d1f::1`.
- Run the guarded non-check activation and prove bex-api advertises `sshAddress` only afterward.
- Run `scripts/ssh-verify.sh` through the hostname with raw OpenSSH and the unmodified official Render CLI v2.21.0 (`c398207`) in a TTY.
- Record restart/redeploy closure and the complete live denial matrix. Deterministic tests already cover these behaviors, but they do not substitute for t009's public-edge evidence.
