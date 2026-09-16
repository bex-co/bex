# w5 · m100 — Pre-deploy stdout reachable via `type=build` (CLI parity)

**Worker:** worker5 **Goal:** a `pre_deploy_failed` deploy is diagnosable from the pinned Render CLI's closed `--type` enum, because the pre-deploy Job's stdout ships into the durable `build` stream the way Render's deploy log stream carries pre-deploy output. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Alloy: ship `component=predeploy` Job pods into the `type=build` Loki pipeline | 40m | — | — **DONE**
| t002 | Operator/docs: record dual `predeploy` + `build` reachability; keep live `type=predeploy` | 20m | t001 | — **DONE**
| t003 | Backend/regression: assert build history can carry a predeploy-sourced marker; mixed-type 400 stays | 30m | t001 | — **DONE**
| t004 | Render parity | 15m | t002, t003 | — **DONE**
| t005 | Simplify | 15m | t004 | — **DONE**
| t006 | Test coverage | 20m | t005 | — **DONE**
| t007 | Closeout | 10m | t006 | — **DONE**

## Definition of done

A service whose pre-deploy command prints a unique marker and exits non-zero yields that marker from `GET /v1/logs?type=build` (and therefore from `bex logs --type build`). Live `type=predeploy` still works alone. Mixing `predeploy` with other types remains a named 400. Deploy-detail interleaving does not double-render the same line when both legs return it (`dedupeLogLines` by timestamp/instance/message). ADR010/ADR018 and the CLI checklist `--type` row document the dual path.

## Source + Goal linkage

- **Source:** inbox note `.pm/w5/062.md` (live `/qa-find-bugs-cli` 2026-09-15 sweep 4) — option (a).
- **Goal linkage:** Render CLI / automation parity — users must diagnose failed migrations without a bex-only log type the pinned client rejects.
- **Expected outcome:** `bex logs --resources <srv> --type build` shows the pre-deploy command's stdout; `failureReason`'s "check the pre-deploy logs" is reachable via that path. Complementary to inbox `064` (narrating the exit code in platform `==>` lines).
- **Why now:** the extension and the client enum are each correct; their intersection is a documented dead end. Render parity included because this changes the logs contract consumers see.
