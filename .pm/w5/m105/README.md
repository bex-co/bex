# w5 · m105 — Settings command edits take effect in the deployed artifact

**Worker:** worker5 **Goal:** A Settings command edit deploys the requested behavior instead of silently retaining an old baked command. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render command-edit deploy behavior for parity | 30m | — |
| t002 | Reproduce native Settings command edits on dev-5 | 50m | t001 |
| t003 | Repair the demonstrated artifact dispatch or selection failure | 55m | t002 |
| t004 | Verify command effects and deployment controls | 45m | t003 |
| t005 | Record cross-surface runtime evidence | 30m | t004 |
| t006 | Render parity | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 15m | t007, t008 |

## Definition of done

- [ ] Dated command-edit/deploy/cache/restart/rollback matrix is recorded.
- [ ] Acceptance tests observable command effects rather than imposing an unverified Render image mechanism.
- [ ] Native start/build edit and unchanged-spec controls have timestamped artifact/runtime evidence.
- [ ] If local behavior passes, production uncertainty remains open; no speculative correction is made.
- [ ] The demonstrated failure has a named cause and a targeted correction.
- [ ] No change bypasses artifact identity or corrupts rollback/reuse behavior.
- [ ] Saved start and build commands take effect in the intended deployed revision.
- [ ] Control cases retain their documented reuse/rebuild behavior.
- [ ] Failed builds never masquerade as a successful updated artifact.
- [ ] Evidence proves the user-visible command effects without treating every digest change as mandatory.
- [ ] No production reproduction or resolution is claimed from local evidence alone.

## Source + Goal linkage

- **Source:** [w4/m110/t001](../../w4/blocked/m110/t001.md); approved by the user on 2026-09-30 following the w5 brainstorm and parity correction.
- **Goal linkage:** ADR008 dependable, deterministic hosting and agent operation.
- **Expected outcome:** A Settings command edit deploys the requested behavior instead of silently retaining an old baked command.
- **Why now:** The central edit-to-deploy contract has an unresolved observed failure; static identity tests already pass and runtime diagnosis is the next useful step.
- **Render parity:** Included because tenant-facing behavior changes. t001 researches Render first; the closing parity task verifies the finished behavior. Label unmatched surfaces as bex extensions, not proven Render parity.
- **Sizing:** 210m research/implementation; 320m including closing tasks; 9 tasks.

## Transfer and execution constraints

Only w4/m110/t001 implementation ownership transfers. Its completed instance-count, rollback-label and log-narration fixes remain complete and out of scope. w4/m110 retains the original production acceptance gate until evidence actually satisfies it. Use dev-5; local provisioning/recovery is pre-approved within harness boundaries. A passing local reproduction is not proof the production incident is resolved: document the remaining measurement needed and keep the item open. Do not invent an implementation merely to finish the queue.
