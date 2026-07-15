# w2 · m39 — SSH into running service instances

**Worker:** worker2 **Goal:** A workspace member can register an SSH public key and use standard `ssh` or the official Render CLI to open an authorized terminal in an eligible running bex service instance. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Freeze the SSH contract and threat model | 45m | — |
| t002 | Store identity-scoped SSH public keys | 45m | t001 |
| t003 | SSH-key management on REST · GraphQL · MCP | 45m | t002 |
| t004 | Render `sshAddress` + running-instance discovery | 45m | t001 |
| t005 | Dashboard SSH keys + copy-ready Connect surface | 45m | t003, t004 |
| t006 | Isolated SSH gateway: public-key auth + service authorization | 60m | t002, t004 |
| t007 | Bridge SSH shell/exec channels to Kubernetes exec | 60m | t006 |
| t008 | Production wiring: host key · port 22 · DNS · least-privilege RBAC | 60m | t007 |
| t009 | Live acceptance with OpenSSH and the official Render CLI | 45m | t005, t008 |
| t010 | Render parity | 30m | t009 |
| t011 | Simplify | 30m | t010 |
| t012 | Test coverage | 60m | t010 |
| t013 | Closeout | 15m | t012 |

## Definition of done

For a running, non-free web, private, or background-worker service with a live deploy, an authorized workspace member can add an Ed25519/ECDSA/RSA public key, run `render ssh <service-name>` unmodified against bex-api, accept bex's stable SSH host key, and receive an interactive TTY inside the selected app container with the app's runtime environment. A specific Ready instance can be selected through Render's `GET /v1/services/{id}/instances` shape; a missing/disabled key, foreign-workspace service, suspended service, unsupported service type, free plan, non-Ready instance, or image without a shell fails closed with a useful error and no exec. Redeploy/restart closes attached sessions. The gateway is separately deployed, public-key-only, auditable without recording commands or terminal contents, and its ServiceAccount can only read eligible tenant pods and create `pods/exec` sessions. REST, GraphQL, MCP, dashboard, raw OpenSSH, and the official Render CLI are covered by meaningful tests or live acceptance evidence.

## Source + Goal linkage

- **Source:** user request, 2026-07-14, explicitly moving interactive SSH out of `.pm/DO_NOT_DO.md`; Render's current SSH contract is documented at `render.com/docs/ssh`, and the official `render-oss/cli` consumes `serviceDetails.sshAddress`, `GET /services/{id}/instances`, and the local OpenSSH client.
- **Goal linkage:** pillars 1 and 3 in `docs/ADR008-vision.md`: Render-compatible APIs and agent-operable infrastructure. SSH supplies the deliberately missing runtime-debugging path while preserving the operator/backend boundary—the backend gateway uses Kubernetes exec but the DB-free operator remains unchanged.
- **Expected outcome:** maintainers and tenants can inspect and debug a live service with their normal SSH tooling without receiving Kubernetes credentials or exposing an SSH daemon in every tenant image.
- **Why now:** the user explicitly reopened this narrow parity gap after the official Render CLI compatibility audit documented `render ssh` as unavailable. The running-pod, authz, identity, deploy-history, and instance-list inputs already exist or have close precedents, so the work can now be bounded without reviving hosted sandboxes. Render parity is included because the service REST shape, identity surfaces, CLI behavior, and dashboard all change.

## Explicitly out of scope

- Ephemeral shell instances (`render ssh --ephemeral`), one-off jobs, cron shells, and E2B/hosted sandboxes.
- A browser terminal/Shell tab, SFTP/SCP, SSH agent forwarding, and `direct-tcpip` port forwarding; parity review records these as follow-ups rather than silently claiming them.
- Installing shells or SSH daemons in tenant images; distroless/shell-less images remain unsupported.
