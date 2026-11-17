# w6 · m434 — A Muse turn waits for its background work: the adapter holds the prompt open, and Muse workflows come back on

**Worker:** worker6 **Goal:** a Muse session that starts a workflow, a subagent or a backgrounded command keeps its turn open until that work, and the turn Muse starts to report it, have finished. The driver's turn then carries the result, and bex can offer Muse's Workflow tool again. **Status:** blocked (2026-11-16: t001–t003 done on the adapter's `main`. The authorized 0.12.0 release failed its own verify step on a flaw since fixed on `main`, and publishing the fix needs a new version or a moved tag, which the authorization does not cover. t005, t006 and t008's driver check are ready in stash `loopx/w6/m434`. See `## Park record (2026-11-16, release)`.)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Adapter: an opt-in capability holds `session/prompt` until background work settles, and the host stays open meanwhile](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [Adapter: the turn Muse starts to report finished work streams into the held prompt, with its approvals and cancellation](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Adapter: a real-host loopback test on Muse 1.4.14 with a workflow and a backgrounded command](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [Release the adapter through its publish workflow](t004.md) | 30m | t003 |
| t005 | [bex pins the new adapter and negotiates the capability; a driver test holds the prompt across an in-progress workflow](t005.md) | 1h | t004 |
| t006 | [Muse's Workflow tool is back on (`auto`), and ADR108 D8 is amended](t006.md) | 30m | t005 |
| t007 | [Production: a Muse routine run that calls `workflow` opens its PR or reports that it has nothing to change](t007.md) | 45m | t006 |
| t008 | [Devin parity](t008.md) | 30m | t006 |
| t009 | [Simplify](t009.md) | 20m | t008 |
| t010 | [Test coverage](t010.md) | 45m | t008 |
| t011 | [Closeout](t011.md) | 10m | t007, t010 |

## Definition of done

- Under a capability the client negotiates, `@bex-co/muse-code-acp` answers `session/prompt` only after three conditions hold: no workflow, subagent or background tool call is in progress; no host-owned turn is running; and no approval or user input is pending. The host-owned turn's updates stream into the same prompt. The host is never closed while such work runs. A client that does not negotiate the capability sees 0.11.0's behavior. Fixture tests prove it, and so does a loopback test on Muse 1.4.14-R7309.1 in the adapter's required CI.
- An adapter release carrying it is on npm, published by the adapter's `publish.yml`. bex pins that release in `lego/backend/internal/agentsession/agent-profiles.json`.
- The driver negotiates the capability. A driver test keeps the agent's prompt open while a workflow tool call is `inProgress`, and records the workflow's result before the turn ends.
- `writeMuseSettings` writes `run.workflow_trigger_mode: "auto"`. ADR108 D8 carries a dated amendment, and the agent-image README matches it.
- After the production deploy that carries the change, a Muse `rt-*` routine run that calls `workflow` opens its PR or reports that it has nothing to change. Its transcript shows the workflow's result before `finish`.

## Source + Goal linkage

- **Source:** promoted from [w6/130](../../done/130.md) on 2026-11-16. That note was split from [w6/129](../../done/129.md), which turned Muse's Workflow tool off as a stopgap (`5f29b4e81`, [ADR108 D8](../../../../docs/ADR108-muse-code-agent.md)). The production evidence is in w6/m354 t007 and ADR105's routine table: three of 19 Muse sessions called `workflow` and ended their turn 2 to 3 s later with their work undone.
- **Goal linkage:** cloud coding agents with Devin as the reference ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)), for the Muse profile ([ADR108](../../../../docs/ADR108-muse-code-agent.md)). A Devin session keeps working until its work is done.
- **Expected outcome:** Muse can fan work out in parallel inside a bex turn. A long build or test that Muse 1.4.14's `bash` sends to the background (past `yield_time_ms`, at most 300000) also no longer ends a turn early. That second failure has not been seen on production, but the 0.11.0 adapter does not guard against it.
- **Why now:** w6/129 left Muse on bex without parallel fan-out. The adapter's 0.11.0 behavior is the only thing in the way, and every piece of it is in code that bex-co maintains.
- **Two repositories.** t001–t004 run in [`bex-co/muse-code-acp`](https://github.com/bex-co/muse-code-acp), under its own `AGENTS.md`, `.pm/DO_NOT_DO.md`, `ship` skill (which pushes to its `main`) and `release` skill. The release skill says: "A request to release authorizes the release commit, push, tag, workflow dispatch, and GitHub release. A request only to edit this skill or ship code does not authorize publication." The worker first confirms that its run authorizes pushing to the adapter's `main` and publishing a release (the user's request, or the adapter's own `/loopx`). If it does not, the milestone parks with that authorization as its gate, naming whatever is already done.
- **Devin parity, not Render parity:** the change is visible on the agent session surface (transcript and phase), and for that surface Devin is the reference product.

## Park record

**2026-11-16, `/loopx w6` worker: blocked on authorization, no code changed in either repository.**

- **Premise holds.** The driver still writes `workflow_trigger_mode: "off"` (`lego/agent-image/driver/src/acp.ts:370`). bex pins `@bex-co/muse-code-acp@0.11.0` (`lego/backend/internal/agentsession/agent-profiles.json:72`), and npm `latest` is 0.11.0. The adapter's `release()` is unchanged at `main` `8725956`: it keeps the host only while `safeToReuse` holds, and for at most `idleTimeoutMs ?? 60_000` (`src/muse-sdk-host.ts:503-530`). No open item on the adapter's board covers this.
- **No repository evidence authorizes a bex `/loopx` worker to push to the adapter's `main` or to publish it.**
  - The adapter's `.agents/skills/release/SKILL.md` says: "A request only to edit this skill or ship code does not authorize publication."
  - The adapter's own `.agents/skills/loopx/SKILL.md` ships to its `main`, but says: "This skill does not authorize package publication, deployment or paid-provider runs."
  - Each earlier adapter release that its board records was approved by the user for that release. 0.8.0 and 0.9.0 both say "User approved … on 2026-10-08" (adapter `.pm/w3/done/m3/done/t003.md`, `.pm/w3/done/m5/done/t003.md`).
  - No commit in the adapter's history came from a bex worker, and nothing on the bex board records one.
- **Nothing is implementable in bex first.** t005 pins t004's release and negotiates the capability that t001 names. t006–t011 depend on t005.
- **Gate:** the user authorizes either path below. Run `/unblock w6` to record the decision.
  - A bex `/loopx w6` worker pushes t001–t003 to `bex-co/muse-code-acp` `main` and releases 0.12.0 through its `publish.yml`.
  - The user runs t001–t004 in the adapter repository, with its own `/loopx` and `/release`. t005 then starts once npm has the release.

## Decision record

**2026-11-16, explicit user decision, recorded by the `/loopx w6` worker.**

- **Affected:** t001–t004 directly; t005–t011 follow on t004's release.
- **Source:** the user's own reply. Asked "authorize a bex `/loopx` worker to push t001–t003 to `bex-co/muse-code-acp` `main` and publish its 0.12.0 release through `publish.yml`, or run t001–t004 yourself?", the user answered "authorize and then continue" on 2026-11-16.
- **Chosen path and scope:** the first path in the park record. A bex `/loopx w6` worker pushes t001–t003 to the adapter's `main` with the adapter's `ship` skill, and publishes exactly one release, `@bex-co/muse-code-acp@0.12.0`, through the adapter's `publish.yml`, following the adapter's `release` skill. The authorization covers that push and that one release only: no other version and no other package. If 0.12.0 cannot ship as tagged, the worker parks with a new authorization request instead of minting 0.12.1.
- **Rationale:** a Devin session keeps working until its work is done and shows that work in progress ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)). Every piece of the fix is in code bex-co maintains, so nothing outside the user's own repositories gates it.
- **Gates verified 2026-11-16:** `gh repo view bex-co/muse-code-acp` reports `viewerPermission: ADMIN`; adapter `main` is `8725956` with green CI; the `Publish and Release` workflow is active; npm `latest` is 0.11.0, so 0.12.0 is unused. t007's production check still waits for the deploy that carries t005–t006, and clears by itself.
- **Next steps:** t001 → t004 in the adapter repository under its `AGENTS.md` and `.pm/DO_NOT_DO.md`, then t005 onward in bex. Each task's acceptance criteria stand unchanged.

## Park record (2026-11-16, release)

**`/loopx w6` worker: t001–t003 done, parked on t004's release.**

- **Adapter `main`.** `0e38f27` adds `muse/backgroundHold` and is green in adapter CI run 43977123219: the 1.4.14 leg ran the new live tests and the 1.1.1 leg skipped them. Three follow-ups came after it. `8ad6672` sends a finished workflow's card before the turn that reports it. `964c747` stops adopting a reporting turn from dropping an approval decision in flight. `2ab66a2` fixes a fixture-test match. Adapter CI run 43982826349 is green on `2ab66a2`, with both loopback legs. Their records are in `done/t001.md` to `done/t003.md`.
- **The 0.12.0 attempt.** Release commit `83612e6` carries `package.json` 0.12.0 and a `## [0.12.0] (2026-11-16)` CHANGELOG section. The annotated tag `v0.12.0` points at it and is pushed. `publish.yml` run 43978268628 failed `verify-release` in the Muse 1.4.14 loopback leg, on the first run and on a rerun of the failed jobs. Both times `background-hold-live.test.ts`'s workflow test failed with `expected -1 to be greater than 14`, then 15.
  - Cause: in the tagged source, the host sent a finished workflow's terminal card on its next 100 ms observation tick, so the reporting turn's text could reach the client first. `8ad6672`, which came after the tag, fixes it.
  - The hold itself held: each failing run took as long as the passing ones (8.2–8.5 s against 8.3 s).
  - The npm publish and GitHub release jobs were skipped. npm `latest` is still 0.11.0, and no GitHub release exists.
- **Why parked.** The fix changes the tagged source. The adapter's release skill then requires a new version and tag, and it forbids moving a remote release tag. The authorization in `## Decision record` covers exactly 0.12.0. Rerunning a check that fails on a known flaw until it passes would weaken the check, so the worker did not keep rerunning.
- **Adapter state left for the release.** `main` says 0.12.0 in `package.json` and in the CHANGELOG, but npm does not have it, and the remote tag `v0.12.0` points at `83612e6`. The release that follows must reconcile this as the release skill says: retitle the section under the new version, or retag under an explicit exception.
- **Ready in bex, in stash `loopx/w6/m434` (`ab528b0ca`).** It holds t005, t006 and t008's driver check. None of it can ship before the release: against 0.11.0, the driver's acknowledgement check would fail every Muse session.
  - The pin moves to `@bex-co/muse-code-acp@0.12.0` in `agent-profiles.json`, `profiles_test.go` and ADR108 D1. Change it to the version actually published.
  - The driver negotiates `muse/backgroundHold` for the `muse` profile only, and fails setup when the adapter does not acknowledge it. Only the Muse adapter defines the key, and it changes when a prompt is answered.
  - The fixture gains `ACP_FIXTURE_BACKGROUND_WORK` and `ACP_FIXTURE_OLD_MUSE_ADAPTER`.
  - `test/muse.test.ts` gains five tests. A muse turn stays open across a workflow and records `result < report < finish`. A hold that never settles ends at the ceiling. A held turn reads `running`, and a stop ends it `interrupted`. An adapter without the hold fails setup before any prompt. A claude turn is not held.
  - `writeMuseSettings` writes `workflow_trigger_mode: "auto"`. The ADR108 D8 amendment, the agent-image README and the `docs/AGENTS.md` line are amended to match.
  - Verified: the driver suite passed 252/252 before the five new tests, and with negotiation removed three of them fail. `go test ./internal/agentsession/...` passes with the new pin.
- **t006 probe (local, to repeat on the published package).** The local 0.12.0 build ran Muse 1.4.14-R7309.1 (`muse-aarch64-macos`, the manifest SHA-256), driven over raw ACP with the profile's session config against a stub Responses endpoint. The adapter acknowledged `muse/backgroundHold`. Under `auto` the first model request offered 26 tools, including `muse.workflow`; under `off` it offered 25. These are w6/129's numbers. The script is not kept in either repository: it sends initialize, `session/new`, the three `set_config_option` calls and a prompt, then lists the tools in the first `/responses` body.
- **t008 Devin reference (gathered early).** On Devin, a coordinator "is woken automatically, even if it is asleep, so it doesn't poll" when a managed child finishes ([advanced capabilities](https://docs.devin.ai/work-with-devin/advanced-capabilities)). Stopping a Dynamic Workflow "cancels the script and puts the remaining child sessions to sleep" ([dynamic workflows](https://docs.devin.ai/work-with-devin/dynamic-workflows)). The held prompt matches both: the turn stays `running` while the workflows run, their results and the report land in that turn, and a stop cancels the work.
- **Gate: the user authorizes one of these two paths. Run `/unblock w6`.**
  - (A) Publish `@bex-co/muse-code-acp@0.12.1` from adapter `main` through `publish.yml`. `main` carries `8ad6672`, `964c747` and `2ab66a2`.
  - (B) Delete the unpublished remote tag `v0.12.0`, then publish 0.12.0 from the fixed `main`. This is an exception to the adapter release skill's tag rule.
- **After the gate.** Release the version, pin it in the stash's t005 changes, rerun the t006 probe on the published package, and ship t005 and t006. Then do t008 to t010. t007 waits for the production deploy.
