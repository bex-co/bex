# w6 · m306 — Builds of different services start together, and a held build is visible

**Worker:** worker6 **Goal:** a workspace's builds are limited only per service, as on Render, and a release held before its build Job exists is measured and alerted on. **Status:** in progress (t001–t005 and t008 done; t006 waits for the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record how long a release is held before its build Job exists](done/t001.md) — **DONE** | 1h | — |
| t002 | [The census and `BuildQueuedTooLong` see held releases](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Retire the per-workspace build cap](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Fair order at the cluster ceiling](done/t004.md) — **DONE** | 1.5h | t001, t003 |
| t005 | [Wake held releases when a build ends](done/t005.md) — **DONE** | 1h | t004 |
| t006 | [Verify on production and close the ADR018 divergence](t006.md) | 45m | t005 |
| t007 | [Render parity](t007.md) | 30m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](t009.md) | 30m | t008 |
| t010 | [Closeout](t010.md) | 10m | t009 |

## Definition of done

- On production, a four-service Blueprint on an otherwise idle build plane runs its four builds at once (t006 record).
- A release held by the cluster ceiling or a datastore wait appears in `bex_builds_queued`, and `BuildQueuedTooLong` fires on such a stall (alert unit test; series visible on production).
- At a full ceiling, a freed slot goes to the workspace with the fewest active builds, and a held release starts within seconds of a slot freeing (envtest).
- ADR018 records build concurrency as aligned with Render; ADR060's status says the D6 amendment shipped.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`. Decision recorded in docs/ADR060 §D5/§D6/§D8 (amended 2026-11-10), docs/ADR034 (amendment note), docs/ADR018 § Deploys "Build concurrency" and § Gap backlog. Related, not duplicated: `w21/blocked/012` (an undiagnosed 20–35-minute `queued` stall; t001–t002 give it the data it lacks).
- **Goal linkage:** ADR008 Render parity first. Render limits only one active build per service; bex's per-workspace cap of 2 was never recorded in ADR018 or signed off, so it is an open parity gap, not a divergence. The README's "first deploy takes 20–40 minutes" is its customer-visible face.
- **Expected outcome:** a Blueprint with several services builds in one wave; any wait before a build starts is measured and alerted on; a full ceiling is shared fairly.
- **Why now:** it is the largest avoidable delay in a first deploy and blocks the one-click Deploy-to-bex path (w21/008, w21/009). The SLI lands first (t001–t002) so the cap's removal is measured from both sides.
- **Render parity task:** included. The change is tenant-visible (when builds start and how deploys read on every surface).
- **Recommendation recorded, not a user decision:** retiring the cap is ADR060 §D6's decision under ADR008's rule; aligning with Render needs no divergence sign-off. The build pools' maximum size (ADR060 §D8, today burst max 2 → 4 slots) stays a user cost decision; this milestone does not need it.
