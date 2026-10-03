# w4 · m159 — Keep confirmations open through permission refresh

**Worker:** worker4 **Goal:** routine access refresh does not silently dismiss an unchanged service confirmation, while stale access never dispatches an action **Status:** blocked — t001/t002/t004/t005 done 2026-10-03; t003 live confirmation verification and t006 closeout remain

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Separate confirmation intent from authorization eligibility | 25m | — |
| t002 — **DONE** | Preserve same-context dialogs with fail-closed dispatch | 45m | t001 |
| t003 | Render parity and live confirmation verification | 25m | t002 |
| t004 — **DONE** | Simplify | 15m | t003 |
| t005 — **DONE** | Test coverage | 35m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- On an owned live service, open Manual Deploy → Restart and Settings → Suspend, then leave each untouched through multiple permission-refresh cycles. With unchanged resource/workspace/access, the dialog stays open. The captured intermittent expiry-before-response case is reproduced deterministically in tests.
- While access is checking, stale or unavailable, an open confirmation displays that state and cannot dispatch. A same-context successful refresh makes the existing dialog usable again, but never replays a click or a pending mutation. Cancel remains available.
- Identity/workspace/resource/selected-deploy/access-generation changes still invalidate old intent and in-flight checks. Confirmed denial stays fail-closed; late responses cannot authorize an obsolete action. Preserve the full prior security guarantees recorded in finding.md.
- The observed source-edit control keeps its unsaved draft across refresh, and the observed image rollback → Restart → standard-deploy sequence remains correct. No REST/GraphQL/MCP authorization policy or deployment semantics change.
- Focused provider/hook/component tests and required dashboard checks pass. Live fixtures are deleted and sessions revoked; unprobed protected/commit dialogs and resource families are verified in tests before closeout.

## Source + Goal linkage

- **Source:** user-requested continuous QA sweep 41 using muse.env, explicitly scheduled in w4; [complete evidence, root cause and prior-DoD disposition](finding.md).
- **Goal linkage:** ADR004 dependable service operations, ADR024 authorization and ADR006 consistent platform actions.
- **Expected outcome:** users can read and confirm an action without racing a routine permission poll; stale access still cannot mutate anything.
- **Why now:** the strengthened shared guard in `3bf7de0d1` also invalidates dialog presentation during temporary freshness gaps. Restart and Suspend both disappeared live despite an unchanged successful Admin response. Security work in w6/m146 remains required; this is the newly observed presentation regression, not a replacement for its blocked two-session acceptance.
- **Render parity:** included for the user-facing confirmation flow; the 30-second capability lease is a Bex implementation detail, not a claimed Render policy.
- **Estimate:** 160m across six tasks, including the five shared-hook call sites and fail-closed regression coverage.
