# m57 production acceptance attempt — DNS blocker (2026-07-18)

## Scope

- Target class: production `api.bex.co` plus the public managed-Key-Value edge.
- Client: Homebrew bottle of official `github.com/render-oss/cli` v2.21.0; the upstream `v2.21.0` tag resolves to `c398207`.
- Entrypoint: `scripts/cli-compat.sh kv-cli-verify` with a disposable tenant API key and an explicit single-source IPv4 CIDR.
- Safety: terminal and connection-info bodies remained in memory; this note contains no bearer, password, credential-bearing URI, workspace/resource id, source address, or kubeconfig content.

## Observed results

- PASS: refreshed authenticated official-CLI session and disposable API-key bootstrap.
- PASS: created a source-restricted public Key Value through bex-api.
- PASS: the resource reached `available`; connection-info contained an external `rediss://` URI; its generated hostname resolved.
- PASS: the automated PTY bypassed the non-TTY interactive-only guard. Both opaque-id and display-name paths reached bex and launched the CLI-provided `redis-cli` command.
- FAIL: all four one-shot commands remained blocked until their 45-second process-tree deadlines. A follow-up verifier revision failed earlier and more precisely because public TCP/6379 did not accept a connection.
- PASS: the Key Value and disposable API key were deleted after each failed run. A final authenticated audit found zero `kvcli-…` Key Values and zero `m57-kv-cli-…` API keys.

## Root-cause evidence

- Hetzner's public API showed the Terraform-owned `bex-traefik` load balancer had a 6379 listener and healthy worker targets.
- Generated `*.kv.bex.co` names resolved to the same A/AAAA address sets as the Cloudflare-proxied HTTP API, not to the load balancer's addresses.
- ADR021 already requires the Valkey wildcard to be DNS-only because ordinary Cloudflare proxying cannot carry raw TCP/6379.
- The repository repair is `scripts/datastore-dns-cloudflare.sh`; it reconciles `*.db.bex.co` and `*.kv.bex.co` to the load balancer with `proxied:false` and provides a read-only `--check`.

## Remaining acceptance

Apply the reconciler with an ephemeral zone-scoped Cloudflare DNS/Edit token, require `--check` and TCP/6379 readiness to pass, then rerun the same official-CLI entrypoint. The checklist row remains open until PING/SET by opaque id, exact GET/DEL by display name, and cleanup all pass through the real public TLS/SNI path.

