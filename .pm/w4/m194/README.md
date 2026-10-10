# w4 · m194 — Clear cron overrides without restoring an old command

**Worker:** worker4 **Goal:** an image cron's Command reports its configured runtime override, and clearing it makes subsequent jobs use the image default **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Clear the effective cron override in the saved and carrying release | 45m | — |
| t002 | Expose and audit the configured cron override across shared callers | 35m | t001 |
| t003 | Replay both clears and the nonempty command control on an owned Free cron | 25m | t001, t002 |
| t004 | Check Render parity for command reads and clearing | 20m | t003 |
| t005 | Simplify the command correction | 15m | t004 |
| t006 | Cover fallback clearing, projections and native controls | 35m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Create the owned Free BusyBox image cron from [finding.md](finding.md), wait for Live, and fresh-load Settings. Its configured Docker Command is visible in Command; the field does not show an empty image-default state while a StartCommand override is stored.
- Repeat the recorded **Edit command → harmless replacement → Save → clear → Save → wait for the carrying deploy to be Live → reload → Trigger Run** twice, including the fresh-page replay. Both reads stay empty after clearing, and subsequent jobs use the image default without either original QA marker. The new carrying cron template has no platform command override.
- Repeat the observed nonempty replacement control after waiting for its deploy to be Live: the exact replacement marker alone appears, and run history, expanded detail and Activity settle to the correct successful state.
- Remove the owned fixture, verify the original six-family ID sets remain, and revoke the owned session. Durable command requests/readbacks/runtime evidence and actual validation results are recorded; source-only Docker/native/scheduled/alias claims remain explicitly unverified until exercised.

## Source + Goal linkage

- **Source:** user-requested functional qa-find-bugs loop, 2026-10-10 UTC pass a48, filing to w4. [Complete finding](finding.md) includes both correct empty-string requests, Live release controls, original-script runtime markers, a successful nonempty control, caller counts, dedupe and cleanup.
- **Goal linkage:** [ADR038 cron jobs](../../../docs/ADR038-cron-jobs.md), [ADR004 deployment](../../../docs/ADR004-app-deployment.md), [ADR006 API contract](../../../docs/ADR006-bex-api.md), and [ADR018 Render parity](../../../docs/ADR018-render-parity.md).
- **Expected outcome:** existing and new image/Dockerfile crons expose the configured override and honor explicit clearing all the way to the selected runtime template, while native/buildpack baked startup semantics remain correct.
- **Why now:** the current dashboard promises blank means image default, yet two successful, fully deployed clears execute an old 60-second Blueprint script. The wire conversion is already fixed; clearing only the displayed field leaves a functional hosting failure.
- **Sizing:** 185m across coordinated writers, read projection/callers, live acceptance and regression work; this exceeds an inbox note. Native shell preservation and carrying-release selection are required boundaries.
- **Standing closing tasks:** Render parity, Simplify, Test coverage and Closeout are included because this changes user-facing command reads/writes across API and dashboard surfaces.
- **Scope limits:** no implementation in this filing; no CLI fork, paid changes or unrelated fixtures. The image CLI builder's upstream refusal remains w8/072.
