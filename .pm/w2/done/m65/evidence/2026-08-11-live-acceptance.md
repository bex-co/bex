# m65 t010 — live acceptance evidence (2026-08-11)

**Outcome: PASSED.** A real Zed client connected over SSH into an agent-session
sandbox on the production gateway and reached `/workspace`. User confirmation:
"能联上了" (can connect now).

## What it took — two prod-only gaps the automated suites couldn't catch

The code (t001–t009) shipped green, but the first live attempts failed with
`Permission denied (publickey)` then a server-upload error. Diagnosed against the
production gateway/DB/OpenFGA and fixed both:

1. **Missing DB grant.** `has_table_privilege('bex_ssh_gateway','agent_sessions','SELECT')`
   was `false` in prod — the `GRANT SELECT ON agent_sessions` (t003, in
   `dbrole.sql`) is applied out-of-band by `scripts/ssh-gateway-db-role.sh`, which
   the deploy pipeline does not run, so it was never applied. The resolver
   authorized (FGA had `can_view_sensitive` on `agent_session` — verified) then
   failed at `GetAgentSession` (SQLSTATE 42501), so **every** `ags-…` connect
   failed regardless of session liveness. Applied the grant in prod (additive,
   verified `true`). The gateway's `pods/exec` RBAC in `<ws>-sandbox` was fine
   (an earlier `can-i pods/exec` false-negative was a `--subresource=exec` syntax
   artifact; confirmed `yes`).

2. **SFTP bridge CWD.** After the grant, Zed connected but failed
   "uploading server binary … scp: dest open .zed_server/…: No such file or
   directory". Root cause: the sftp-server ran in the container WORKDIR
   (`/workspace`) while Zed's `~/.zed_server` is `/home/bex/.zed_server`, so the
   relative upload path resolved to `/workspace/.zed_server` (nonexistent). Fixed
   in `session.go` — the sftp bridge now starts in `$HOME`
   (`sh -c 'cd "$HOME" && exec …/sftp-server'`), matching sshd semantics. Shipped
   as `babf63ba` (rolled via the superseding `1b1e5dcd` deploy). Verified the
   pinned agent image has `sftp-server` and `WORKDIR=/workspace`, `HOME=/home/bex`.

## Follow-ups filed

- The out-of-band `dbrole.sql` grant is not applied by the deploy → this class of
  gap recurs for any future gateway grant. Motivates a deploy step (or Argo Job)
  that idempotently applies `dbrole.sql`. (Noted for a w2 inbox note / m68's ops
  scope.)
- The timing gap (fire-and-forget sandbox reaped ~15s after a turn) that made the
  first connect a race is the entire motivation for ADR059 / w2/m67 (Active-tier
  idle grace) and w2/m68 (hibernation).
