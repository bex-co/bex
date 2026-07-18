# w2 · m55 — Browser-hosted Web Shell

**Worker:** worker2 **Goal:** A workspace member with `can_operate` opens `/services/{id}/shell` in the dashboard and gets an interactive TTY attached to a selected Ready running instance — an in-browser xterm.js terminal matching Render's Web Shell — **without** giving bex-api `pods/exec` and while preserving ADR035's isolation (terminal streams flow only through the isolated ssh-gateway). **Status:** implementation complete (t001–t009 done, all suites green); live acceptance (t010) pending a dev-N cluster with the gateway deployed + `BEX_SHELL_*` configured — prod gated on m39 gateway activation.

## Tasks (in order)

| id   | title                                                                        | est | depends_on       |
| ---- | ---------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Design + guardrail reopening: ADR035 Web Shell section + DO_NOT_DO — **DONE** | 60m | —                |
| t002 | Exec ticket: bex-api mints short-lived service-scoped exec ticket — **DONE**  | 60m | t001             |
| t003 | Gateway WebSocket exec endpoint bridging browser → `pods/exec` — **DONE**     | 60m | t001, t002       |
| t004 | Dashboard terminal UI: xterm.js in `service-shell-page.tsx` — **DONE**        | 60m | t002, t003       |
| t005 | Instance picker wired to `GET /v1/services/{id}/instances` — **DONE**         | 45m | t004             |
| t006 | Session lifecycle + caps + reconnect UX for browser sessions — **DONE**       | 45m | t003, t004       |
| t007 | Render parity — **DONE**                                                      | 30m | t004, t005, t006 |
| t008 | Simplify — **DONE**                                                           | 30m | t007             |
| t009 | Test coverage — **DONE**                                                      | 60m | t007             |
| t010 | Closeout (live acceptance)                                                    | 30m | t009             |

## Definition of done

In the dashboard, an authorized workspace member opens `/services/{id}/shell`, selects a Ready instance, and receives an interactive TTY inside the app container **via the browser**, matching Render's Web Shell (live terminal + "Select an instance" picker + the copy-ready `ssh …` command shown above it). bex-api still holds **no** `pods/exec` permission; terminal streams flow only through the isolated ssh-gateway's authenticated WebSocket. Unauthorized, free-plan, suspended, foreign-workspace, unsupported-type, and non-Ready targets fail closed with a useful error and no exec. Redeploy, restart, or session timeout closes the attached session. Browser sessions are audited in `ssh_sessions` **without** recording commands or terminal content (the ADR035 structural omission is preserved). `docs/ADR035-ssh.md` and `.pm/DO_NOT_DO.md` are amended to record the reopening. The gateway WebSocket path, the exec-ticket flow, and the dashboard terminal are covered by meaningful tests and dev-stack live evidence.

## Implementation handoff (2026-07-17)

t001–t009 are implemented and all automated suites are green: backend `go test ./...`, dashboard `yarn test` (1407 tests) + `yarn lint` + `yarn typecheck`. What shipped:

- **Shared ticket (`lego/backend/internal/shellticket/`)** — HMAC-SHA256 signed, short-TTL (90s), single-use exec ticket; leaf pkg imported by both bex-api and the gateway. Unit-tested (round-trip, tamper/wrong-secret, expiry+skew, malformed, empty-secret).
- **bex-api mint (`apps.CreateShellSession`)** — `AuthorizeApp(can_operate)` + the `sshEligible`/Running gate → signed ticket + gateway WS URL; `ErrShellUnavailable` (503) when `BEX_SHELL_TICKET_SECRET`/`BEX_SHELL_WS_URL` unset; foreign-instance 400; no-identity 403. REST `POST /v1/services/{id}/shell-ticket` + GraphQL `createShellSession` (MCP omitted — browser-only). **bex-api gains no `pods/exec`.** Verb + REST-status tests + the events-vocabulary excuse (mirrors `ResolveSSHSession`).
- **Gateway WebSocket (`sshgateway/websocket.go`)** — authenticated WS listener (`BEX_SHELL_WS_ADDR`, `/shell`) that verifies the ticket, enforces single-use, re-runs `ResolveSSHSession` (targeting + `AuthorizeApp`), applies the same session caps, and bridges the browser (binary stdin/stdout, JSON resize control) to `KubeExecutor.Execute`; content-free `ssh_sessions` audit; bounded reused metrics. httptest+gorilla tests cover happy path, pinned instance, bad/expired/replayed ticket (401), disabled (503), resolve-failure error frame, and the per-identity cap. Gateway Deployment/Service expose the port (optional secret ⇒ disabled by default).
- **Dashboard** — `web-shell-terminal.tsx` (xterm.js, dynamic client-only import; mint→WS→stream, binary stdin, resize, status/reconnect, unavailable/error states), `web-shell-panel.tsx` (Render's "Select an instance" picker, reconnect-on-change, stale-selection fallback), wired into `service-shell-page.tsx` alongside the copy-ready SSH command. `serviceInstances` GraphQL query + hooks. Component + panel + route tests.
- **Docs** — ADR035 § Browser Web Shell (transport, Kratos-session→ticket auth, re-drawn boundary; "browser terminals" removed from non-goals); `.pm/DO_NOT_DO.md` carveout; ADR018 SSH row + non-goals; CLAUDE.md + `.env.example`/`.env.template` env vars.

**t010 (live acceptance) is deliberately open.** The DoD's dev-stack live evidence needs a running dev-N cluster with the operator, bex-api, and the ssh-gateway deployed and `BEX_SHELL_TICKET_SECRET`/`BEX_SHELL_WS_URL`/`BEX_SHELL_WS_ADDR` configured (plus a running paid service). Production is additionally gated on **m39's gateway production activation** (m39 t008/t009 — public DNS/TCP-22 + the gateway WebSocket edge), which is still open. Do not move this milestone to `done/` or check its box in `w2/README.md` until the sanitized live-acceptance evidence is recorded under `w2/m55/evidence/`.

## Source + Goal linkage

- **Source:** user request, 2026-07-17 — "learn from Render's `/shell` page and make bex's `/services/{id}/shell` the same," with the transport chosen to be "consistent with render.com." Live evidence captured this session: Render's "Web Shell" page is a live in-browser xterm terminal (active input, real prompt `render@srv-…-xc8cn`) plus a "Select an instance" replica picker plus the copy-ready `ssh srv-…@ssh.oregon.render.com` command above it. bex's prior `/services/{id}/shell` (from `w2/m39` t014) was intentionally only the copy-ready SSH guide. This milestone **explicitly reopens** the browser-terminal anti-goal — the same way the user's 2026-07-14 decision reopened interactive SSH to create `w2/m39`; `t001` amends `.pm/DO_NOT_DO.md` and ADR035 to lift it rather than leaving a silent conflict.
- **Goal linkage:** `docs/ADR008-vision.md` pillars 1 & 3 — Render-compatible surfaces and human/agent-operable infrastructure. Extends `w2/m39` (running-instance SSH): reuses its isolated gateway exec mechanism and adds the browser transport, preserving the operator/backend boundary (the DB-free operator is untouched; `pods/exec` stays confined to the gateway ServiceAccount).
- **Expected outcome:** maintainers and tenants can open a live shell in a running service directly from the dashboard — Render parity — without receiving Kubernetes credentials, without bex-api gaining `pods/exec`, and without exposing an sshd in any tenant image.
- **Why now:** the user explicitly reopened this narrow browser-terminal gap after the m39 SSH work shipped its isolated gateway. The exec mechanism (executor, Render-compatible instance targeting, `AuthorizeApp(can_operate)`, session caps, content-free audit) already exists, so the browser transport can be bounded without reviving hosted sandboxes or ephemeral shells. **Render parity is included** because the change touches the dashboard UI plus a new bex-api ticket endpoint and a new gateway endpoint — the surface must stay consistent and be compared against render.com.

## Explicitly out of scope (still excluded)

- Ephemeral shell instances (`render ssh --ephemeral`), one-off jobs, cron shells, and E2B/hosted sandboxes.
- SFTP/SCP, SSH agent forwarding, X11 forwarding, and `direct-tcpip`/port forwarding.
- Installing shells or sshd into tenant images; distroless/shell-less images remain unsupported (a shell-less image fails closed exactly as in m39).
- Granting bex-api any `pods/exec` permission — the isolation split from ADR035 is a hard invariant.
