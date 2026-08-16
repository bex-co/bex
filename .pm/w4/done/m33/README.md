# w4 · m33 — Codex round-4 security remediation: authorize the sink, finish the revoke

**Worker:** worker4 **Goal:** Close the six P0/P1 findings from the round-4 codex-security scan (`e815388e`). Three are one-relation authorization bugs where a second path to a sensitive sink kept the weaker relation; two are revocation paths that report success without revoking; one is a CI trust boundary. **Status:** done — t001–t003 landed in the original pass; t004/t005/t006 were resolved by later codex rounds (#8/#9/#11) that re-found and re-fixed the same shapes, verified against HEAD 2026-08-15; two missing t005 test assertions backfilled here

## Tasks (in order)

| id   | title                                                                     | est | depends_on | status |
| ---- | ------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Native SSH: gate the `pods/exec` sink on `can_view_sensitive` (finding #8) | 30m | —          | — **DONE** |
| t002 | Agent-session `Steer`: gate on `can_create` (finding #3)                   | 45m | —          | — **DONE** |
| t003 | `ChangeRole` retry: converge to the exact role, not just the grant (#4)    | 30m | —          | — **DONE** |
| t004 | Device flow: never pair a grant on the signed-out GET (finding #7)         | 1.5h | —         | — **DONE** |
| t005 | Managed Postgres `DeleteUser`: revoke the login before success (#2)        | 1.5h | —         | — **DONE** |
| t006 | Pin every credentialed `workflow_dispatch` job to main (finding #1)        | 45m | —          | — **DONE** |
| t007 | Render parity                                                             | 20m | t004, t005, t006 | — **DONE** |
| t008 | Simplify                                                                  | 15m | t007       | — **DONE** |
| t009 | Test coverage                                                             | 45m | t007       | — **DONE** |
| t010 | Closeout                                                                  | 15m | t009       | — **DONE** |

## Definition of done

A contributor (`can_operate` + `can_manage_ssh_keys`, no `can_view_sensitive`) is refused both an interactive pod shell — over native `ssh` as well as the browser terminal — and an agent-session `Steer`, across REST, GraphQL, and MCP; a downgrade whose OpenFGA revoke failed converges to exactly one role tuple on retry; a deleted managed-Postgres user can no longer open a session before the API returns success; a `workflow_dispatch` from a non-main ref reaches no job that holds a production credential; and the signed-out device-verification link leaves the grant pending until an authenticated, same-origin confirmation POST.

## Source + Goal linkage

- **Source:** `codex-security` scan of `e815388e` (report at `~/.codex/state/plugins/codex-security/scans/bex/codex-security-bex-JOJ6g5/report.md`), triaged 2026-08-09. All 16 findings were verified against the code; the ten P2/P3 items are filed as `w4/029.md`. Findings #5 (agent GitHub-token branch scope) and #6 (bex-api cluster-wide CR write) were triaged as accepted-with-mitigation and partly-valid respectively — see `029.md`.
- **Goal linkage:** multi-tenant security (w4's charter, `docs/ADR012-auth.md`, `docs/ADR035-ssh.md`, `docs/ADR047-cloud-coding-agent-sessions.md`).
- **Expected outcome:** the role ladder holds at every transport into the `pods/exec` sink and at every verb that provisions a credential-bearing sandbox; "delete" means "revoked" for Postgres logins and OpenFGA tuples alike.
- **Why now:** #8 and #3 are **regressions introduced by the previous remediation pass** (`18cd7871`, "codex #1"). That pass raised the relation on the browser-shell path and the agent-session `Create` path and left their siblings — native SSH and `Steer` — on `can_operate`, so the boundary it documented was bypassable from day one. The same shape will recur until the guard test pins every path to a sink, which t001/t002 add.
- **Render parity:** included. `Steer` is exposed on REST, GraphQL, and MCP, and a relation change alters who gets a 403 on all three; Render has no equivalent verb, so the check is cross-surface consistency within bex, not against render.com.
