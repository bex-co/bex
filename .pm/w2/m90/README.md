# w2 · m90 — Web Shell production activation + WS-edge liveness

**Worker:** worker2 **Goal:** an authorized workspace member opens `/services/{id}/shell` on the production dashboard and gets a live terminal over `wss://ssh.bex.co/shell`, and a dead Web Shell edge alerts instead of failing silently. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Activate the prod WS edge: `bex-shell-ticket` Secret + verify `wss://ssh.bex.co/shell`     | 40m | —                |
| t002 | Production browser acceptance: live terminal + fail-closed matrix                          | 45m | t001             |
| t003 | Extend `ssh-edge-liveness.yml` with a Web Shell WS-edge probe                              | 30m | t001             |
| t004 | Record the activation in ADR035/ADR018; remove the "awaits activation" caveats             | 15m | t002             |
| t005 | Simplify                                                                                   | 20m | t003, t004       |
| t006 | Test coverage                                                                              | 30m | t003, t004       |
| t007 | Closeout                                                                                   | 10m | t006             |

## Definition of done

A recorded production Web Shell session from the real `dashboard.bex.co` against a paid Running service: the terminal reaches Connected, echoes stdin/stdout, honors resize, and closes on process exit. The fail-closed matrix (unauthorized, foreign-workspace, free-plan, suspended, non-Ready target, replayed ticket) refuses with no exec. The first green `ssh-edge-liveness.yml` run after the change includes the WS-edge probe, and the probe was proven red once against a dead/absent edge (a refusal-shaped answer and a dead edge are distinguishable). ADR035 and ADR018's SSH row no longer carry the "awaits activation" caveat (activation date recorded). bex-api still holds no `pods/exec` (asserted in evidence). Evidence in `.pm/w2/m90/evidence/`; no secret values recorded anywhere.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-07 #3. `w2/done/m55`'s closeout honestly recorded "production still requires the deployed shell-ticket Secret, browser-reachable WebSocket URL, and gateway WebSocket edge"; ADR018's SSH row still says the WS edge "awaits activation" (re-confirmed by w6/m132's 2026-08-28 edit). All manifests are already checked in: `lego/operator/config/ssh/ingress-shell.yaml`, the gateway's gated listener (`config/ssh/deployment.yaml:93-101`), bex-api's Secret wiring (`config/api/deployment.yaml:437`), and `scripts/shell-ticket-secret.sh`.
- **Goal linkage:** Render parity honesty — ADR018 marks the Web Shell ✅ UI while the production terminal cannot connect; completes the m39 → m55 → w6/m132 SSH-edge lineage.
- **Expected outcome:** the in-dashboard terminal actually works in production, and its edge can never die silently again.
- **Why now:** w6/m132 proved the cost of an unmonitored SSH edge — the TCP handshake was dead for six weeks while `sshAddress` kept advertising it. The Web Shell edge is in exactly that pre-m132 state today (shipped manifests, never activated, never monitored), and m132's probe/synthetic context (`scripts/ssh-kexinit-probe.sh`, `ssh-edge-liveness.yml`) is warm.
- **Render parity omitted:** operational activation of an already-shipped, already-ledgered surface — no REST/GraphQL/MCP/UI wire change (t004 updates ledger caveat text only).
