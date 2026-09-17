# w4 · m110 — Settings config changes roll the stale image; instance counts, rollback dialog, and build narration mislead

**Worker:** worker4 **Goal:** a build/start command edit in Settings rebuilds instead of re-rolling the old image; Total Instances counts live pods, not terminated ones; the rollback dialog names the commit it restores; the deploy log stops narrating builds that never ran. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                    | est   | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | ----- | ---------- |
| t001 | Settings build/start command edits must rebuild — config-change deploys roll the stale image              | 3h    | —          |
| t002 | INSTANCES counts terminated pods — Total Instances over-reports for minutes after every rollout           | 1h30m | —          |
| t003 | The rollback confirmation dialog names the commit being restored (m108/t004 acceptance gap)                | 45m   | —          |
| t004 | The deploy log stops narrating phantom builds for deploys that perform no build                           | 1h    | —          |

## Definition of done

Each bullet is a command or a click the next person can repeat on production and watch succeed.

- **Command edits rebuild.** On a native-runtime web service, change the start command in Settings to `sh -c "echo QAENV:$QA_TEST_VAR && npm start"` (with `QA_TEST_VAR` set) and confirm: the config-change deploy emits Build started/ended events, resolves a new image digest, and the new pod's first log line is the echo. Repeat with a build-command edit (`npm install && echo QA_BUILD_MARKER`): the marker appears in the build log and the digest changes. The old behavior — 13–30s deploy, no build events, pod runs the previous image's baked command — is gone.
- **Instance counts are live counts.** After any rollout on a 1-replica service, the INSTANCES series and the Metrics **Total Instances** headline read 1 on the first post-rollout bucket — not 4 decaying to 1 over 15 minutes with zero changes.
- **The rollback dialog names the code.** On any `deactivated` deploy row, **Rollback** opens a dialog whose body names `<short-sha> <subject>` of the deploy being restored; when the deploy genuinely has no commit the dialog keeps today's generic body.
- **No build, no build narration.** A deploy that performs no build (reuse rollout, rollback) shows no `==> Build queued` / `==> Building from …` lines; its log tells the rollout story only. A deploy that does build keeps them.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-17 (w4-targeted run, `muse.env` credentials). Journeys exercised: project/env create, web service from Public Git URL (`qa-20260917-web`, free; deleted), manual deploy → cancel → redeploy, rollback, deploy-hook trigger, env-var save → config-change redeploy, start/build command edits, static site + redirect/header rules, private service, cron, Postgres (external verify-full), Key Value (TLS round-trip), blueprint plan→apply, broken-build failure states, sleep→wake, full cleanup to zero residue. Evidence: `.playwright-mcp/qa-deploys-rollback-dialog.png`, `.playwright-mcp/qa-metrics-instances-4.png` (gitignored, local to that session); the durable evidence is the probes quoted in each task.
- **Goal linkage:** Render parity and product truthfulness ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md), [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md), [docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md) for deploy semantics, `docs/render-artifacts/metrics-page.md` for metrics semantics). A Settings save that promises "Service will redeploy with the new build command" and then serves the old image breaks the most basic edit→effect contract in the product.
- **Expected outcome:** Settings edits take effect on the next deploy without a manual rebuild dance; the Metrics instance chart can be reconciled against the header's instance count; destructive/restorative dialogs name their target; deploy logs describe what happened.
- **Why now:** t001 is a correctness bug in the central edit→deploy loop — every Settings command edit on every native service silently no-ops its baked-in half today, and the log narration (t004) actively hides it by printing a build story for a build that never ran. t002 misleads scaling decisions after every rollout. All four sit on seams the codebase already models correctly (artifact identity, live-count primitives, commit-carrying rows), so each fix is a change of wiring, not of architecture.
