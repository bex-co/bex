# w9 · m179 — Remove approval mode (ADR100 withdrawn)

**Worker:** worker9 **Goal:** cloud agent sessions are purely autonomous again, and every approval-mode surface w9/m176 shipped is gone from the backend, gateway, driver, dashboard, CLI and docs. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend: remove decisions, approval fields and routes; drop the schema | 90m | — |
| t002 | Gateway and driver: remove the decision channel, gating, `ask_user` and codex rules | 60m | — |
| t003 | Dashboard: remove the callout, Blocked grouping, settings section and composer picker | 60m | — |
| t004 | CLI and docs: remove `--approval-mode`/`--bypass-approval`/`answer`; mark ADR100 withdrawn | 30m | t001 |
| t005 | Simplify | 15m | t001, t002, t003, t004 |
| t006 | Test coverage | 30m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- On production, the API no longer exposes `approvalMode`, `bypassApproval`, `waitingFor`, `pendingDecision`, the decision answer route or `answer_agent_session_decision`, and a create that sends `approvalMode` is refused as an unknown field (strict decoding).
- A session told to run `sudo true` runs it without pausing.
- The dashboard has no approval UI.
- The `agent_session_decisions` table and the approval columns are dropped by a new migration.
- ADR100 reads Withdrawn, and ADR094 keeps `needs_input` reserved and never emitted.

## Source + Goal linkage

- **Source:** user decision 2026-11-07 ("整体移除 然后放到 do not do"); `.pm/DO_NOT_DO.md` #APPROVAL-MODE.
- **Goal linkage:** ADR008 pillar 5 (long-running unattended cloud agents); ADR090's autonomy contract.
- **Expected outcome:** less code to keep correct, and one behavior for every session.
- **Why now:** the feature just shipped (default off), so removing it before anyone depends on it is cheap.
- **Parity:** omitted. This removes an optional surface; Devin's cloud sessions are autonomous by default.
