# w5 · m104 — Correct cold-link initialization

**Worker:** worker5 **Goal:** Cold service and agent links retain their destination and settle truthful permissions without waiting for a poll. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render cold-link and workspace behavior for parity | 30m | — |
| t002 | Reproduce and measure cold-link initialization on dev-5 | 45m | t001 |
| t003 | Repair measured capability-request cancellation | 50m | t002 |
| t004 | Resolve workspace before agent eligibility and repair retry | 45m | t002 |
| t005 | Verify destinations and pending geometry end to end | 40m | t003, t004 |
| t006 | Render parity | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 15m | t007, t008 |

## Definition of done

- [ ] Dated parity matrix distinguishes observed behavior, documentation, inference and bex extensions.
- [ ] The service/agent cold-link acceptance is reconciled with findings before t002.
- [ ] A reproducible failing sequence and passing control are timestamped.
- [ ] Workspace delay and pre-dispatch cancellation are distinguished.
- [ ] Current shared capacity is verified; no dev-7 resources are mutated.
- [ ] Initial healthy capability reads settle without periodic polling.
- [ ] Canceled prior-generation work cannot cancel or authorize the current generation.
- [ ] Failure and confirmed denial remain distinct.
- [ ] Eligible cold links are not redirected solely because workspace resolution is pending.
- [ ] Confirmed denial still follows existing policy.
- [ ] Unavailable membership can recover through a real loader retry.
- [ ] Intended destinations survive cold navigation and workspace selection.
- [ ] Pending and ready geometry match at both breakpoints.
- [ ] No faster poll or fixed delay masks initialization failure.

## Source + Goal linkage

- **Source:** [w7/m149 retained diagnosis](../../w7/blocked/m149/README.md) and its preserved regression patch; approved by the user on 2026-09-30 following the w5 brainstorm and parity correction.
- **Goal linkage:** ADR008 dependable, deterministic hosting and agent operation.
- **Expected outcome:** Cold service and agent links retain their destination and settle truthful permissions without waiting for a poll.
- **Why now:** Permission refresh shipped in w6/m144; measured startup residuals remain and w5 has capacity.
- **Render parity:** Included because tenant-facing behavior changes. t001 researches Render first; the closing parity task verifies the finished behavior. Label unmatched surfaces as bex extensions, not proven Render parity.
- **Sizing:** 210m research/implementation; 320m including closing tasks; 9 tasks.

## Transfer and execution constraints

Execution ownership transfers from w7/m149 to w5/m104. The original evidence and task history stay in w7; its tasks wait on this milestone and are not a second implementation queue. Read the original regression patch in place; do not apply it blindly. Historical shared-VM OOM remains a capacity risk, not proof dev-5 is currently blocked. Local harness recovery is pre-approved under root instructions, within isolation boundaries; do not restart the shared VM or alter another workstream as an incidental recovery. Research is immediately actionable. Coordinate with w6/074 without claiming its broader walkthrough complete.
