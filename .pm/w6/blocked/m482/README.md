# w6 · m482 — An ssh-gateway release gives open SSH and Shell sessions an hour, as Render's SSH maintenance does

**Worker:** worker6 **Goal:** a production deploy that restarts `bex-ssh-gateway` leaves every open SSH and dashboard Shell session working until the user ends it, its instance stops, its access is revoked, or an hour has passed since the gateway began draining; new connections reach only the new pods. **Status:** blocked (t001–t005, t007–t009 done and shipped; the drain is live on production since pin `826d32e42`, and t006's probes hold an SSH and a Web Shell session on a `bex-canary` fixture since 2026-11-19 14:15Z; t006 reads them across the next deploy that restarts the gateway, then t010)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [On SIGTERM the gateway stops accepting and keeps established SSH and Shell sessions for up to an hour](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [The agent and sandbox listeners drain within the same bound](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [The Deployment, PDB and deploy.yml let an old pod drain for an hour without holding the deploy](done/t003.md) — **DONE** | 45m | t001 |
| t004 | [Tests: a session open at SIGTERM keeps working, revocation still ends it, the hour closes it](done/t004.md) — **DONE** | 1h | t001, t002 |
| t005 | [Record the rule in ADR035](done/t005.md) — **DONE** | 20m | t003 |
| t006 | [Ship and verify on production](t006.md) | 45m | t004, t005 |
| t007 | [Render parity](done/t007.md) — **DONE** | 20m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- On production, an SSH session (`bex ssh` / `ssh <srv-id>@ssh.bex.co`) and a dashboard Shell session opened before a deploy that restarts `bex-ssh-gateway` both keep echoing after the new gateway pods are Ready. Each ends only when the user closes it, the instance stops, access is revoked, or an hour after the old pod received SIGTERM.
- During the drain a new SSH or Shell connection lands on a new pod (the old pod's listeners are closed).
- `deploy.yml`'s gateway step returns once the new pods are Ready; it does not wait out the hour.
- A test fails if a SIGTERM cuts an open session at once, and one fails if a revoked member keeps a draining session.
- ADR035 records the rule and cites Render's one-hour figure.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-17 (item 1 after verification), approved by the user's `/pmx them all to w6` 2026-11-18. Evidence:
  - `.github/workflows/deploy.yml:1377` runs `kubectl -n bex-system rollout restart deploy/bex-ssh-gateway` on every production deploy, including a no-op one (about 4–6 pins a day, e.g. six on 2026-11-17).
  - `lego/backend/internal/sshgateway/nativessh/server.go:305-306`: "Individual connections inherit that cancellation and therefore cannot outlive a terminating gateway process." `cmd/ssh-gateway/main.go:84` cancels on SIGTERM, `gateway.Serve` returns, `main` returns, and every session dies with the process. The auxiliary listeners (`startAuxListener`) get a 5 s `Shutdown`, which does not wait for hijacked (WebSocket) connections at all.
  - w32/m381 t004 named "An ssh-gateway rollout cuts the stream" as blast radius without filing it. w4/m352 made sessions survive the tenant's own deploy, not a platform release.
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/ssh](https://render.com/docs/ssh) (read 2026-11-17): Render closes a service's SSH sessions when "The service is redeployed or restarted for any reason" and when "Render is scheduled to perform maintenance on underlying infrastructure that enables SSH connections. In this case, Render gives existing connections one hour before automatically closing them." The ssh-gateway is that infrastructure. No ADR018 row signs off cutting sessions at once.
- **Expected outcome:** a user's SSH or Shell session no longer drops several times a day for platform releases; agent model, git and egress streams drain instead of being cut after 5 s.
- **Why now:** every production deploy cuts every open session today, and deploys run several times a day. This continues w6/m469 and w6/m470 (platform releases keep datastore connections) on the SSH path. This is not Render's `/maintenance` scheduling API, which `.pm/DO_NOT_DO.md` lists as a non-goal; it is the session behavior Render documents for its own infrastructure work.
- **Render parity task included:** SSH and the dashboard Shell are tenant-facing surfaces (REST `shell-ticket`, GraphQL `createShellSession`, the Shell page's end reasons).

## Unblock record (2026-11-19, verified evidence)

- **Affected:** t006, then t010. **Source:** verified evidence, no new decision; scope and definition of done unchanged.
- **The release gate cleared.** Production pins `826d32e42` (`7af50a28168d`, 03:48Z), `19faf35f3` (`4b2cebd0bd03`, 09:13Z) and `a91e9aeb3` (`dff52c596d13`, 11:30Z) all contain `97c2ac2f5`. The first rolled the gateway onto the drain; the other two were the first drain-capable deploys.
- **What production already shows (read-only, 14:05Z).** `bex-ssh-gateway` reads `maxUnavailable 0`, `maxSurge 1`, `terminationGracePeriodSeconds 3660`. `deploy.yml`'s gateway step returned 11 s after `rollout restart` on 09:18:37Z and 6 s after it on 11:36:03Z, each while "1 old replicas are pending termination" (deploy jobs 135456050272 and 135529544614). kube-state-metrics shows the drain-capable old pods gone about a minute after deletion, so an idle gateway exits at once. No session was held across either deploy, so that still proves nothing about a held session.
- **Remaining gate:** none needs a person. The sessions must be open before a deploy, and the next one cannot start before 17:30Z (`deploy.yml` waits 6 h after the 11:30Z pin), so t006 armed self-recording probes instead of holding a terminal (t006 § Progress 2026-11-19). The next deploy writes its pin back about five minutes before it restarts the gateway, so the workstream line's pin gate on `a91e9aeb3` (the first pin after it) clears with it.
- **Next (the worker after that pin):** read both probe pods' logs across the gateway restart, as t006 lists, and check them against the first two definition-of-done items; then delete the fixture, the probes and both keys, and close with t010.
