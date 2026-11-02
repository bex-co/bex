# w8 · m146 — `deploy_ended` and `build_ended` carry Render's `reason` object

**Worker:** worker8 **Goal:** every ended deploy and build event carries Render's `buildDeployEndReason` (`failure` / `buildFailed` / `newDeploy` / `newBuild`), so a client written against Render can tell a crash from an OOM, a timeout or a superseding deploy. **Status:** in progress — t001–t006 done 2026-11-02; t007 live closeout after the operator and bex-api deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Operator records the failed release's last app-container termination — **DONE** | 45m | — |
| [t002](done/t002.md) | Store the structured end reason on the deploy row — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Project `reason` on deploy_ended and build_ended (REST, MCP, GraphQL) — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](t007.md) | Closeout: live crash / superseded / build-failed checks | 30m | t005, t006 |

**Total:** 7 tasks / 225m. [Finding](finding.md).

## Definition of done

- `deploy_ended` and `build_ended` always carry `details.reason` (an empty object when there is no cause, as Render requires the field).
- A start-command crash reads `reason.failure` with `nonZeroExit` = the exit code, `earlyExit: true`, `evicted: false`; an OOM reads `oomKilled`; a progress-deadline timeout reads `timedOutSeconds`; a superseded deploy reads `newDeploy.id` naming the newer deploy; a build failure reads `buildFailed.id` on `deploy_ended`.
- REST and MCP serialize the same object; GraphQL exposes it. `failureReason`/`fullDeployStatus`/`cancelReason` stay as bex extras.
- Live on production: a crash, a superseded deploy and a failed build each read back as above; fixtures deleted by ID with 404 proofs.

## Source + Goal linkage

- **Source:** [w8/229](finding.md) (`/qa-find-bugs-cli` sweep 3093, 2026-11-02). The exit code reaches bex-api only as text (`crashLoopMessage`, `app_controller.go`), and the superseding deploy id only inside `cancel_reason` text; no structured fact exists to project.
- **Goal linkage:** ADR006/ADR018 Render API parity for service events; pinned client `pkg/client/events/events_gen.go` (`DeployEndedEvent.Reason`, `BuildEndedEvent.Reason`, `FailureReason`).
- **Expected outcome:** Render-schema clients read the same machine-readable end reasons from bex.
- **Why now:** w4/454 made crash deploys settle in ~40 s; the event feed should say why, the way cron runs already do (ADR038).
- **Render parity included:** REST/MCP/GraphQL event surfaces change.
