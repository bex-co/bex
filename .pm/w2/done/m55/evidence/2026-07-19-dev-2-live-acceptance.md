# m55 Browser Web Shell — dev-2 live acceptance

**Date:** 2026-07-19  
**Environment:** local CAPD app cluster, dev-2 tenant namespace  
**Fixture:** paid web_service w2-m55-web-shell-live (busybox:1.36.1, one Ready replica)

No session token, exec ticket, database credential, host private key, terminal transcript beyond the fixed test markers, or kubeconfig content is recorded here.

## Browser acceptance

A headless Chrome session logged in through the dev-2 Kratos browser flow and opened:

/services/srv-d9e6l9q9086k61mipmt0/shell

The page:

- rendered **Web Shell** beside ssh srv-d9e6l9q9086k61mipmt0@ssh.local;
- listed **Any ready instance** plus the current opaque Ready instance;
- selected the specific instance and reached **Connected** over the deployed gateway's WebSocket;
- ran printf 'bex-dashboard-shell-live\n'; exit 7 in the xterm terminal;
- displayed bex-dashboard-shell-live and then **Session closed**.

This live pass exposed and fixed a dashboard CSP omission: connect-src allowed the HTTPS API but not the Web Shell's WebSocket scheme. Production now explicitly permits wss:; development additionally permits only loopback ws: origins. The browser then received HTTP 101 from the real gateway. The focused route/picker/terminal suite also passed: 3 files, 8 tests.

## Gateway and TTY acceptance

The opt-in cluster tests ran against the deployed gateway and real tenant pod:

- TestGatewayRealKubernetesWebShell passed: resize plus binary stdin reached /bin/sh, the fixed marker returned, exit status 23 propagated, and replaying the same ticket returned 401.
- TestGatewayRealKubernetesWebShellTimeout passed in 2.04 seconds with a temporary two-second gateway timeout; the deployment was restored to its four-hour default.
- TestGatewayRealKubernetesWebShellPodDeletion passed in 2.16 seconds: it waited for an attached marker, deleted the disposable pod, and observed the browser stream close. The Deployment replaced the pod and the App returned to Running.

The local CAPD worker could not reach the Kubernetes API Service IP, so the gateway was pinned to the control-plane node for this acceptance, matching the existing local operator workaround. This was runtime-only fixture state, not a checked-in production scheduling change.

## Fail-closed matrix

Observed status/result:

| Case | Result |
| --- | --- |
| no Kratos session | REST 401 |
| identity from another workspace | REST 403 |
| free plan | REST 409 |
| suspended service | REST 409 |
| same-service but non-Ready instance id | WebSocket error control frame; no exec |
| replayed ticket | WebSocket handshake 401 |
| session deadline | attached stream closed |
| target pod replacement | attached stream closed |

The service was restored to starter, not suspended, Running, with one Ready replica.

## Isolation and audit

Live Kubernetes authorization checks:

| Principal | create pods/exec in dev-2 |
| --- | --- |
| system:serviceaccount:bex-system:bex-ssh-gateway | yes |
| system:serviceaccount:bex-system:bex-api | no |

Recent ssh_sessions rows included both completed and failed closure outcomes with ended_at set. The live schema contains exactly:

id, subject, workspace_id, service_id, instance_id, remote_address, started_at, ended_at, result

There are zero command, content, stdout, stderr, stream, or terminal columns.

## Verification

- go test ./internal/apps ./internal/sshgateway ./internal/store
- focused dashboard Shell route/picker/terminal tests: 3 files, 8 tests
- yarn typecheck
- dashboard production build
- git diff --check

All passed.

## Production status

The underlying m39 SSH gateway is production-activated and has its own public-edge acceptance evidence under w2/done/m39/evidence/. This m55 run verified the Browser Web Shell end to end on dev-2 only; it did not mutate or falsely claim a fresh production Web Shell acceptance. Production still requires the deployed BEX_SHELL_TICKET_SECRET, browser-reachable BEX_SHELL_WS_URL, and gateway WebSocket edge to match the checked-in manifests.
