# CLI sweep 588 — two groups, one key: later-linked group wins, and keeps winning

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixtures `qa588-web` (`srv-dbb8r0vo17is73fus01g`, free busybox serving `V=[%s]`) plus groups `qa588-g1` (`evg-dbb8r0dt4o5c73dqgfn0`) and `qa588-g2` (`evg-dbb8r0lt4o5c73dqgfo0`), both defining `V588`. All deleted at close (service 404, URL 404, both groups 404, zero `qa588` names).

- **Link order decides:** `g1` was linked first, `g2` second; the live page read `V=[g2]`. Between competing groups, the later link has precedence for the shared key.
- **Not last-write-wins:** updating `g1`'s value to `g1-updated` (accepted, 200) never surfaced — across ~5 minutes of polling the page stayed `V=[g2]`. The shadowed group's edits roll nothing visible; precedence is structural (link order), not temporal by write.
- Combined with sweeps 586–587, the full order is: service's own value (even empty) > later-linked group > earlier-linked group.

## Unrun

- Whether unlinking the winning group uncovers the earlier group's value or removes the key entirely.
- Multi-service fan-out: one group linked to two services, one update rolling both.
- Group var update while the linked service is suspended.
