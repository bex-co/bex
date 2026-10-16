# w4 · m215 — Keep accepted Postgres upgrades non-Ready

**Worker:** worker4 **Goal:** An accepted major upgrade never returns to Available before the target major is healthy, and the dashboard follows its progress without a reload. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Hold readiness while an accepted major upgrade awaits CNPG acknowledgement | 45m | — |
| t002 | Replay the native upgrade status sequence and recovery controls | 35m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 25m | t003 |
| t006 | Closeout | 10m | t004, t005 |

Total: **140m, 6 tasks**. [Researched finding](finding.md), [durable observations](evidence.json), [isolated diagnostic](diagnostic.go.txt), [diagnostic output](diagnostic.log.txt).

## Definition of done

- After an accepted supported upgrade, neither the API nor the open dashboard returns to Available before the target major is observed and the required instances are ready. The sequence may use Restarting before Upgrading; the old observed version remains truthful until it changes.
- A fresh desktop and 390px native upgrade shows the transition through completion without reloading. Repeat with another supported source/target pair; verify the process version and retained fixture data, then remove every owned resource and session.
- First provision, unchanged healthy databases, manual restart, suspension/resume, failed-upgrade revert and missing/unknown version observations keep their existing explicit meanings. A rejected or reverted target cannot leave a perpetual Upgrading state.
- Meaningful operator and public status regressions fail on the old implementation. Run the relevant operator suite/lint, backend status tests and dashboard checks required by any changed code.
- Finish the standing tasks and move this milestone and its tasks to done only after those observations hold.

## Source + Goal linkage

- **Source:** Functional `$qa-find-bugs w4`, a712/a713, 2026-10-16 UTC. Two fresh native a713 transitions reproduce the issue; a712 independently first exposed the badge lag. See the finding for the exact capture and source limits.
- **Goal linkage:** ADR008 usable Render-alternative hosting; ADR009 truthful upgrade state; ADR018 cross-surface status parity.
- **Expected outcome:** The owner can trust upgrade progress instead of seeing Available while the offline upgrade runs.
- **Why now:** Accepted upgrades already work, but old healthy CNPG status can satisfy the new Database generation before CNPG acknowledges the target. It also cancels fast dashboard polling. Fix this gap separately from the completed manual-restart acknowledgement work in w4/m137/t010 and the SQL execution gate in w4/177.
- **Render parity included:** This changes an operator-produced status consumed by REST, GraphQL, MCP and the dashboard. Preserve the existing Bex vocabulary and validation; review the documented Render upgrade lifecycle.
