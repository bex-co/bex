# w4 · m212 — Keep webhook disable intervals consistent

**Worker:** worker4 **Goal:** disabled-period source events never become new webhook notifications after re-enable, while eligible backlog and admitted retries remain durable. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Specify source-time eligibility and migration boundaries | 30m | — |
| t002 | Persist eligibility through every endpoint state writer | 50m | w4/m212/t001 |
| t003 | Enforce interval eligibility during webhook admission | 45m | w4/m212/t002 |
| t004 | Render parity and surface consistency | 25m | w4/m212/t003 |
| t005 | Simplify changed code | 15m | w4/m212/t004 |
| t006 | Prove interval and retry behavior with meaningful tests | 50m | w4/m212/t004 |
| t007 | Replay live evidence and close out | 15m | w4/m212/t005, w4/m212/t006 |

## Definition of done

- On owned Free receiver and webhook A subscribed to Deploy Ended, disable A through Settings, restart the receiver, wait for Live and record the source event ID. A has no attempt while disabled. After enabling A and waiting at least 30 seconds, A still has no attempt or receiver POST for that disabled-period event. Repeat from a fresh Settings load.
- Repeat with a second owned enabled webhook B. B receives the event with HTTP 204; A skips it both before and after re-enable. A's result is the same as the preceding case.
- Restart while A is enabled: its new deploy-ended event receives HTTP 204 and appears in the real receiver and immutable history. Preserve the observed failing-route control: first attempt 503, restore URL, native Resend delivers the same source event/payload as attempt 2 with 204 while attempt 1 remains failed.
- Actual REST/GraphQL/MCP endpoint enabled-state inputs use the same interval decision, without changing their dialect or authorization/error shapes; record which adapters were replayed. Regression tests prove initially disabled creation, auto-disable, pre-disable undispatched eligible events, queued retries, two worker caches, exact cutovers and migration behavior; these classes are source-traced work rather than live claims from a675.
- Clean up every owned endpoint/receiver, restore baseline inventory ID sets, revoke only the QA session, and record results. Passing local tests alone does not close the live bullets.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, 2026-10-16 UTC a675, muse.env through the QA login helper. [Finding](finding.md) and [durable requests/responses](evidence-a675.json).
- **Goal linkage:** reliable Render-alternative hosting automation (ADR008, ADR006, ADR052); event delivery must reflect endpoint configuration consistently.
- **Expected outcome:** enabling A cannot replay disabled-period events merely because another endpoint was absent or a dispatcher cursor was behind; enabled-period backlog and admitted retries keep their delivery guarantees.
- **Why now:** two live failures and an opposite passing control establish customer-visible nondeterminism in the existing shipped webhook feature. Resolve eligibility before users depend on disabling automation during changes.
- **Render parity:** included because endpoint state across REST/GraphQL/MCP/dashboard changes delivery eligibility. Render docs do not specify disabled-gap replay; preserve declared bex differences and record any newly chosen divergence. This ships a functional fix, not a new event bus.
