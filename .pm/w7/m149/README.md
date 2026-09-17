# w7 · m149 — Correct cold deep-link initialization

**Worker:** worker7 **Goal:** Direct service and agents URLs settle from initial workspace and capability results instead of waiting for the periodic poll. **Status:** todo

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
