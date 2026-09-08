# m90 Web Shell production activation — evidence

**Date:** 2026-09-08  
**Environment:** production (`dashboard.bex.co`, `api.bex.co`, `wss://ssh.bex.co/shell`, `hetzner-prod`)  
**Worker:** worker2

No session token, exec ticket, Secret value, terminal transcript beyond fixed test markers, or kubeconfig content is recorded here.

## t001 — edge activation verified (already live)

Observed on `hetzner-prod` (read-only + public probes):

| Check | Result |
| --- | --- |
| Secret `bex-system/bex-shell-ticket` | present (`Opaque`, key `secret`) — not rotated |
| Gateway log | `web shell listening on :8080` |
| Ingress | `bex-ssh-shell` → `ssh.bex.co` `/shell` → Service port 8080 |
| TLS | `CN=ssh.bex.co`, valid through 2026-10-16 |
| Unauthenticated upgrade | HTTP 401 body `missing ticket` (alive-but-refusing) |
| bex-api env | `BEX_SHELL_TICKET_SECRET` from Secret; `BEX_SHELL_WS_URL=wss://ssh.bex.co/shell` |
| Ticket mint | `POST /v1/services/{id}/shell-ticket` → 201 with `url=wss://ssh.bex.co/shell` |

## t002 — production browser + fail-closed matrix

### Happy path (dashboard.bex.co)

Authenticated QA session opened:

`https://dashboard.bex.co/services/srv-d9bkcspg9s7c73d0n8ug/shell`

(paid Running `web_service` `agentmarketcap-1`, plan starter, workspace `tea-d98210cbbpdc73dcrkvg`)

Playwright (Chromium, storage-state from `scripts/qa-login.sh`):

- page title included Web Service / bex Dashboard; body showed **Web Shell** and copy-ready `ssh …`
- reached **Connected**
- typed `printf 'bex-m90-dashboard\n'; exit 0` into the xterm helper
- observed marker **bex-m90-dashboard** and session **closed**

Screenshot (UI chrome only): [`2026-09-08-dashboard-shell.png`](2026-09-08-dashboard-shell.png)  
Notes: [`2026-09-08-dashboard-shell-notes.txt`](2026-09-08-dashboard-shell-notes.txt)

### Gateway TTY (same edge, API-minted ticket)

`TestGatewayRealKubernetesWebShell` against production with a minted ticket for the same service:

- resize control frame accepted
- binary stdin echoed marker `bex-web-shell-live`
- exit code 23 propagated
- replayed ticket → handshake 401

### Fail-closed matrix

| Case | Result |
| --- | --- |
| no Kratos session | REST 401 `unauthorized` |
| unknown / foreign-shaped service id | REST 404 `app not found` (no exec; same information-hiding shape as `ssh-verify.sh`) |
| free plan (disposable fixture `srv-dafort7mvahc73d4kbk0`) | REST 409 `service is not eligible and running`; `sshAddress` omitted |
| suspended (same fixture) | REST 409 `service is not eligible and running` |
| non-Ready / bogus instance id | ticket minted; WS `{type:"error", message:"…no instance available…"}`; no exec |
| replayed ticket | WebSocket handshake 401 (live test) |

Disposable fixture created as busybox starter image service, used only for suspend/free legs, then `DELETE` → 204.

## Isolation and audit

| Principal | `create pods --subresource=exec` in `tea-d98210cbbpdc73dcrkvg` |
| --- | --- |
| `system:serviceaccount:bex-system:bex-api` | **no** |
| `system:serviceaccount:bex-system:bex-ssh-gateway` | **yes** |

`ssh_sessions` schema (migration `0030_ssh_sessions.up.sql`) columns remain metadata-only:

`id, subject, workspace_id, service_id, instance_id, remote_address, started_at, ended_at, result`

No command, content, stdout, stderr, stream, or terminal columns.

## t003 — WS-edge liveness probe

- New `ProbeUnauthenticatedRefusal` + `scripts/shell-ws-probe.sh`
- Wired into `.github/workflows/ssh-edge-liveness.yml` (failure of WS probe fails the run)
- Manifest lockstep in `scripts/gitops-validate.sh` (Ingress host/path/port ↔ gateway `:8080` ↔ shared `bex-shell-ticket` ↔ `BEX_SHELL_WS_URL`)

Local discrimination (this session):

| Target | Exit |
| --- | --- |
| `wss://127.0.0.1:1/shell` (dead) | **1** — connection refused |
| `wss://ssh.bex.co/shell` (prod) | **0** — 401 `missing ticket` |

**Owed for closeout:** first green `ssh-edge-liveness.yml` run on `main` that includes the step `Probe the Web Shell WS edge for refusal-shape`. As of 2026-09-08, probe files are **not** on `origin/main`; the latest success ([run 34164258626](https://github.com/bex-co/bex/actions/runs/34164258626)) still has only KEXINIT + fallback-TLS on `public-edge-liveness` — no WS probe step. Requires `/ship` then `gh workflow run` (or the next schedule).

## Verification commands run

- `GOWORK=off go test ./internal/sshgateway/webshell -run ProbeUnauthenticated` — PASS
- `bash scripts/shell-ws-probe.sh` red + green as above
- Playwright dashboard notes as above
- Live `TestGatewayRealKubernetesWebShell` — PASS
