# w22 · m604 — Render CLI v2.38.0: adopt it, with sandbox-group environments and the new release notice kept off

**Worker:** worker22 **Goal:** the `bex` launcher runs Render CLI v2.38.0: its sandbox-group environment commands work against bex, its new once-a-day release notice never tells a `bex` user to install a Render CLI, a 429 prints bex's message readably, and the scheduled schema-drift job is green again. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                     | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Move the pin to v2.38.0 (`09b7802a…`): `go.mod`, the three values in `UPSTREAM_RENDER_CLI.md`, the toolchain — **DONE**              | 30m | —                      |
| t002 | Re-diff the seams in "Updating the pin" steps 2–9: help tree and `ea` names, wire transport, telemetry, skills, branding — **DONE**   | 60m | t001                   |
| t003 | Keep the new release notice off: find its client, URL and cache file, and a test that fails if any notice names a release — **DONE** | 45m | t001                   |
| t004 | `sandbox-groups environment set`/`remove` and `create --environment` through the launcher, with contract tests both sides — **DONE** | 45m | t002, w22/m603         |
| t005 | A 429 now prints the API's message: check `SANDBOX_BUILD_LIMIT`, the w22/m443 rate limits and the group concurrency 429 — **DONE**   | 30m | t002                   |
| t006 | The checklist's graded v2.37.0 → v2.38.0 section, and the drift job dispatched green — **DONE**                             | 30m | t003, t004, t005       |
| t007 | Walk the new commands on production through a launcher built from the tree, once the pin carries w22/m603 — **DONE**                 | 40m | t006                   |
| t008 | Render parity — **DONE**                                                                                                             | 20m | t004, t005             |
| t009 | Simplify — **DONE**                                                                                                                  | 20m | t008                   |
| t010 | Test coverage — **DONE**                                                                                                             | 30m | t008                   |
| t011 | Closeout — **DONE**                                                                                                       | 10m | t007, t009, t010       |

## Definition of done

- `bex -v` prints `compatible with Render CLI v2.38.0`.
- No command prints a notice about a Render CLI release, with a test that fails if one does (the third update-check call site included).
- `bex sandbox-groups environment set <group> <environment>` (by id and by name), `environment remove <group>` and `create --environment` work against bex, with contract tests on the launcher's side and bex-api's.
- bex's 429 bodies print a readable message through v2.38.0's new 429 path.
- `docs/cli-compatibility-checklist.md` has a graded v2.37.0 → v2.38.0 section; `workflows flows runs steps attempts` is one row naming the Workflows non-goal.
- A dispatched `render-schema-drift.yml` run has `render-schema` green.
- The new commands are walked on production in `bex-canary` with the QA user, on a group and an environment the run creates and deletes.

## Source + Goal linkage

- **Source:** promoted from [w22/026](../done/026.md) (found by w22/m444, 2026-11-21) by `/pmx them all to w22`, item 2 of `/pm-brainstorm one aspect of render parity for w22` (2026-11-23); verified at `origin/main` `cb469e7cfd`.
  - **Upstream:** tag `v2.38.0` = `09b7802aa379c92a087884412d88b2b3f8a75a9a`, published 2026-11-20 03:27 UTC ([release notes](https://github.com/render-oss/cli/releases/tag/v2.38.0)). The pin is v2.37.0 (`11ef59f330…`, `lego/cli/UPSTREAM_RENDER_CLI.md`).
  - **Drift job:** `render-schema-drift.yml` runs 45237965785 (schedule) and 45239352057 (dispatch), 2026-11-23, fail only in `render-schema` with "render-oss/cli v2.38.0 is newer than bex's pin v2.37.0"; the Blueprint schema, the webhook enum and the 265 REST operations match. w4/m596's done criteria accept that red only while w22/026 (now this milestone) owns it.
  - **What v2.38.0 adds** (read in w22/026 and in the brainstorm): the three environment commands (`cmd/sandboxgroupsenvironment*.go`; `pkg/sandboxgroup/repo.go` sends `POST`/`DELETE /environments/{id}/resources` with the group id), `workflows flows runs steps attempts list`/`show`, a once-a-day stderr notice when a newer release exists, and a 429 that shows the API's message.
- **Goal linkage:** ADR008 § Render parity first; the CLI launcher contract (`lego/cli/UPSTREAM_RENDER_CLI.md`); ADR104 gap 15.
- **Expected outcome:** a `bex` user gets the commands Render's CLI users got on 2026-11-20, never a prompt to install Render's CLI, and the drift job alerts again only on new drift.
- **Why now:** the drift job is red today, so a new upstream change would go unnoticed behind it; the diff from v2.37.0 is at its smallest. Server work for the environment commands is w22/m603, which t004 waits for.
- **Render parity included:** the change reaches bex-api through the CLI's REST calls and the 429 messages, so t008 compares them with Render's.

## Progress

**Triage (2026-11-24, worker22):** work. At `ec1f1da161` the pin was still v2.37.0 (`lego/cli/go.mod` `v1.1.3-0.20261117005113-11ef59f330de`), and w22/m603, which t004 waited for, is done and on production (pin `ff5de0a130e6`). Nothing gated the rest.

**t001: the pin** is v2.38.0 (`09b7802aa379`, module `v1.1.3-0.20261120013850-09b7802aa379`), in `go.mod`, `UPSTREAM_RENDER_CLI.md`, `scripts/bex-cli-build.sh` and `scripts/bex-cli-validate.sh`. Upstream kept Go 1.27.4; `golang.org/x/crypto` v0.59.0, `x/text` v0.44.0, `x/term` v0.48.0 and `x/mod` v0.42.0 came with it. The build passed; one test failed at once, `TestTheDetachedSenderWritesNothingButConsumesItsEventFile`, because the new notice's lookup wrote `~/.bex/state/update-check.json` in a run with an agent marker (t003).

**t002: the seams of "Updating the pin", steps 2–9.**
- 2: `RootCmd` is exported. The new commands sit under `sandbox-groups environment` and `workflows flows runs steps attempts`, so no Bex-native name collides.
- 3: `isRootVersionRequest` is unchanged and `pkg/client/version` still uses `http.Get` on the same path. The login banner is gone, and a third call site is new (t003).
- 4: the help template and the overlay's hooks are unchanged. The new help pages are branded with no new rule (`bex sandbox-groups environment set --help` reads `bex sandbox-groups environment set sbg-abc123 production`), none links `render.com/docs` or a `~/.render` path, and `--bogus-xyz` on the new commands prints no `render ` line. The run-time inventory grep gained no hit.
- 5: `pkg/sandbox/repo.go` and `sse.go` are unchanged. The one operation change is `POST /environments/{id}/resources`, now parsed as a 201 with the environment (bex has answered 201 since w4/m596).
- 6: `pkg/analytics` and `internal/analyticsnotice` are unchanged, and so is the telemetry input. `cmd/execution.go` now skips the update notice when the analytics notice printed, which changes nothing in bex.
- 7: `pkg/skills` and `cmd/skills*.go` are unchanged.
- 8: `bash scripts/bex-cli-validate.sh` and `cd lego/cli && go test ./...` pass. The live device-flow check ran as part of the t007 walk, not through `scripts/bex-cli-auth-e2e.sh`.
- 9: the `ea` tree is unchanged; the checklist section is t006.

**t003: the release notice.** `internal/updatenotice` is started by `cmd.Execute` and shown from `onExecutionComplete` after any command that printed no JSON or YAML, once a day, to a person at a terminal or to an AI agent in one line: "Update available: v2.38.0 → v<latest>. To upgrade, see: https://render.com/docs/cli#1-install-or-upgrade. Release notes: https://github.com/render-oss/cli/releases/tag/v<latest>". Its lookup is `cfg.RepoURL + "/releases/latest"` through an `http.Client` with no transport of its own, so the bridge's refusal covers the request. It is not enough: the notice caches its answer in `config.StateDir()/update-check.json` and prints a cached newer release with no request, and a stale cache keeps that release when the lookup fails. With only the transport seam, a run with a cache naming 99.0.0 printed the line above. The fix: `bridge.Apply` always sets `RENDER_CLI_DISABLE_UPDATE_NOTIFIER=1`, which `updatenotice.Start` reads before the cache or the network. bex prints no hint of its own in that place beyond the one it already prints after commands (`internal/update`, TTY only); how that hint differs from Render's new rules is filed as `w22/034`. Tests: `TestUpstreamUpdateNoticeNeverPrints` (the binary, as an agent, with no cache, a fresh cache and a stale cache) and `TestApplyAlwaysTurnsUpstreamsUpdateNoticeOff`.

**t004: the environment commands' contract** is the `environment` section of `lego/cli/testdata/sandbox-groups-contract.json`: for `create --environment <name>`, `environment set` by id and by name, a move from A to B, `remove`, and `remove` of a group in no environment, every request the pinned client sends, with the status bex answers, and the objects it reads. `TestPinnedSandboxGroupEnvironmentCommandsSendTheContract` drives the pinned commands against those answers and compares every request; `TestPinnedCLIGroupEnvironmentCommandsReadBexAnswers` (backend, real Postgres) replays each request against bex-api's routes and checks the status, the shape of each group and environment, and where the group ends. The owner and project reads are left to their own packages (w8/m149). The help pages of the three commands are pinned in `TestBexNestedHelp`. No server code changed: m603 serves all of it.

**t005: 429 messages.** v2.38.0 prints `too many requests: <message>` and never the code. `SANDBOX_BUILD_LIMIT` and the group concurrency 429 already read well. The rate limiter's REST body said "rate limit exceeded; see Retry-After header", which names a header the CLI's reader cannot see, and the auth admission's said the same. Both now say when to retry: `rate limit exceeded; retry in 3 seconds` (`core.RateLimitExceededMessage`, which GraphQL fields' and MCP tools' own budgets already worded so, now with "1 second" singular) and `authentication upstream overloaded; retry shortly` (its GraphQL wording). Each printed line is in the checklist section; `TestPinnedClientPrintsBexTooManyRequestsMessages` drives them through the pinned client.

**t006: the checklist.** `docs/cli-compatibility-checklist.md` has the graded v2.37.0 → v2.38.0 section and the new `sandbox-groups` row; `workflows flows runs steps attempts` is one `[-]` row naming the Workflows non-goal. After `0476771aa8` shipped, the dispatched `render-schema-drift.yml` run [45364439235](https://github.com/bex-co/bex/actions/runs/45364439235) passed with `render-schema` green ("Render CLI pin v2.38.0 is the latest upstream release"), and so did its other three jobs.

**t008: Render parity.** Compared with v2.38.0's own fake (`internal/fakes/renderapi/environments.go`): the add answers 201 with the environment, the removal 204, an unknown group 404 "sandbox group not found" on both, and bex-api does the same (m603). Every bex 429 body has the `message` the client prints and no code it would print. One drift: bex's own release hint does not follow the new notice's rules (one line for agents, none after JSON or YAML output, `RENDER_CLI_DISABLE_UPDATE_NOTIFIER`), filed as `w22/034`. ADR018 § surface pins states both.

**t009: simplify.** The release-notice switch does not replace the transport seam: the seam still answers the lookup locally, and the switch covers the cache the seam cannot reach. Applied: the contract stub's empty pages and a helper's name. Kept: the 429 bodies in the CLI test are written out, because the CLI module imports no server package.

**t010: test coverage.** The gaps the task named are covered: the release notice with no cache, a fresh cache and a stale cache (the lookup is refused in all three); `environment set` by a name two projects share, refused by the client with "multiple environments found with name …" before anything moves (`TestPinnedEnvironmentSetRefusesANameTwoProjectsShare`); `environment remove` of a group in no environment (`removeWhenInNone`). Mutation spot-check, each applied alone and reverted:

| Mutation | Failed by |
| --- | --- |
| Remove the notice switch from `bridge.apply` | `TestUpstreamUpdateNoticeNeverPrints` (all three cache states), `TestApplyAlwaysTurnsUpstreamsUpdateNoticeOff` |
| Change the add path of `setByID` in the contract | `TestPinnedSandboxGroupEnvironmentCommandsSendTheContract/setByID` and `TestPinnedCLIGroupEnvironmentCommandsReadBexAnswers/setByID` (404) |
| bex-api's environment list ignores `name` (the name resolution) | `TestPinnedCLIGroupEnvironmentCommandsReadBexAnswers/createIn` and `/setByName`: two environments where the client needs one |
| The REST 429 message back to "see Retry-After header" | `TestRateLimiterRESTShape` |

**t007: the production walk** is recorded in the checklist section (2026-11-24, pin `ff5de0a130e6`): the launcher built from this tree, as the QA user in `bex-canary`, with an agent marker in its environment and a planted cache naming a newer release. `create --environment` by name, `set` by id, a move to B by name, `remove` and `remove` again each exited 0 and `sandbox-groups list` agreed after each; no release notice printed and the cache was untouched. The project, both environments and the group (all disposable) were deleted, the CLI logged out, and the run's Kratos sessions were logged out.

## Closed (2026-11-24)

**t011: the definition of done holds.** `bex -v` from the tree prints `compatible with Render CLI v2.38.0`. No command prints a Render CLI release notice, and `TestUpstreamUpdateNoticeNeverPrints` fails without the switch. `environment set` by id and by name, `environment remove` and `create --environment` passed on production and in the contract tests on both sides. bex's 429s print readably, each line recorded in the checklist. The checklist has the graded v2.37.0 → v2.38.0 section with `workflows flows runs steps attempts` as one `[-]` row. Drift run 45364439235 has `render-schema` green. The walk ran in `bex-canary` as the QA user, on a project, two environments and a group it created and deleted.

**Shipped:** `0476771aa8` (the pin, the notice switch, the contracts, the 429 messages, the documents and `w22/034`).
