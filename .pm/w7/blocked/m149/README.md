# w7 · m149 — Correct cold deep-link initialization

**Worker:** worker7 **Goal:** Direct service and agents URLs settle from initial workspace and capability results instead of waiting for the periodic poll. **Status:** blocked (gated on w7/m148/t007 and a live measurement)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce and measure cold-link initialization | 50m | w7/m148/t007 |
| t002 | Repair the measured workspace and capability startup failure | 50m | w7/m149/t001 |
| t003 | Wait for workspace resolution before agents route eligibility | 35m | w7/m149/t001 |
| t004 | Preserve and verify cold-link destinations end to end | 45m | w7/m149/t002, w7/m149/t003 |
| t005 | Render parity | 30m | w7/m149/t001, w7/m149/t002, w7/m149/t003, w7/m149/t004 |
| t006 | Simplify | 25m | w7/m149/t005 |
| t007 | Test coverage | 45m | w7/m149/t005 |
| t008 | Closeout | 15m | w7/m149/t006, w7/m149/t007 |

## Definition of done

- [ ] Record a reproducible failing sequence with timestamps and a passing navigation control.
- [ ] Distinguish delayed workspace resolution from pre-dispatch rejection or cancellation; do not infer the cause from timing alone.
- [ ] No persistent token or secret logging is introduced.
- [ ] Healthy initial results settle capability state before the periodic poll is needed.
- [ ] A stale response from a prior workspace or generation cannot enable actions.
- [ ] Failed refresh and confirmed denial remain distinct; no permission gate is weakened.
- [ ] An eligible cold agents URL does not redirect home solely because workspace selection is pending.
- [ ] Confirmed ineligibility still follows the existing refusal/navigation policy.
- [ ] Pending geometry matches the destination at desktop and narrow-mobile widths.
- [ ] Service env/Settings and agents cold links retain their intended destination.
- [ ] Authorized, denied, unavailable and recovered states are recorded at desktop and narrow-mobile widths.
- [ ] No fixed delay or faster polling masks the startup failure.

## Source + Goal linkage

- **Source:** Approved w7 brainstorm item 2 via `$pm all for w7`; promoted `w7/049`, archived at `.pm/w7/done/049.md`.
- **Goal linkage:** ADR008 dependable hosting controls and agent-session access; ADR018 Viewer capability projection.
- **Expected outcome:** Cold direct links preserve their destination and produce truthful allowed, denied, and unavailable states without waiting for the 30-second poll.
- **Why now:** The recorded user-facing failure is parked on executable diagnosis, not unavailable credentials. w6/m144 shipped the refresh lifecycle; this fixes its initialization residual and coordinates with w6/074 without duplicating that broader walkthrough.
- **Render parity:** Included: tenant-facing changes; compare the applicable ADR018 coverage and all exposed surfaces.
- **Sizing:** 180m implementation; 295m including standing closing tasks, 8 tasks.

- **Dependency:** t001 follows w7/m148/t007 so the live diagnosis starts on a verified local substrate. Coordinate evidence with w6/074; do not claim its whole walkthrough complete.

## BLOCKED 2026-09-17 — gated upstream, and on a measurement the DoD requires

Nothing here is implementable yet, and that is the milestone's own design rather than a judgement call:

- **t001 `depends_on: w7/m148/t007`**, which is held: m148 is parked because the local CAPD cluster has rotted (machines 20d old, backing containers gone). Every other task in this milestone chains off t001, so the whole queue is behind it.
- **t001 is a measurement task, and the DoD forbids substituting reasoning for it** — _"Distinguish delayed workspace resolution from pre-dispatch rejection or cancellation; do not infer the cause from timing alone."_ That is the same gate `049` was parked on, now written into the acceptance criteria.

A source pass during the `049` triage already advanced the diagnosis and is worth reading before t001 runs, so the measurement is aimed rather than exploratory (`.pm/w7/done/049.md`):

- **Candidate (b) is falsified.** `routes/__root.tsx` `beforeLoad` awaits the session and returns it in context, so `useIsAuthenticated()` is true on first render and the workspaces query is never `skip`ped during hydration.
- **The contradiction to resolve is sharp.** `capabilities.grantUnavailable` has exactly two sources (`capability-policy.ts:166`, `:179`), both requiring a *resolved* state — yet the page showed it while no `ViewerCapabilities` request reached the wire for 30s. A query rejecting before dispatch fits; `refresh()` passes `context.fetchOptions.signal`, and an already-aborted signal rejects with no request.
- **One fix step is already satisfied and should be dropped.** Step 1 of `049`'s fix ("the same copy covers 'never asked' and 'asked and failed'") is not true in current code: a `checking` state returns `undefined`, so pending and failed are already distinct.

t003 (the `/agents` guard) looks separable, and its first two criteria are, but its third — _"Pending geometry matches the destination at desktop and narrow-mobile widths"_ — is a rendering requirement, and it still declares `depends_on t001`. Implementing it ahead of the measurement would contradict the sequencing this milestone chose and risk the exact error its DoD warns about.
