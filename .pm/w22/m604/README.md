# w22 · m604 — Render CLI v2.38.0: adopt it, with sandbox-group environments and the new release notice kept off

**Worker:** worker22 **Goal:** the `bex` launcher runs Render CLI v2.38.0: its sandbox-group environment commands work against bex, its new once-a-day release notice never tells a `bex` user to install a Render CLI, a 429 prints bex's message readably, and the scheduled schema-drift job is green again. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                     | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Move the pin to v2.38.0 (`09b7802a…`): `go.mod`, the three values in `UPSTREAM_RENDER_CLI.md`, the toolchain              | 30m | —                      |
| t002 | Re-diff the seams in "Updating the pin" steps 2–9: help tree and `ea` names, wire transport, telemetry, skills, branding   | 60m | t001                   |
| t003 | Keep the new release notice off: find its client, URL and cache file, and a test that fails if any notice names a release | 45m | t001                   |
| t004 | `sandbox-groups environment set`/`remove` and `create --environment` through the launcher, with contract tests both sides | 45m | t002, w22/m603         |
| t005 | A 429 now prints the API's message: check `SANDBOX_BUILD_LIMIT`, the w22/m443 rate limits and the group concurrency 429   | 30m | t002                   |
| t006 | The checklist's graded v2.37.0 → v2.38.0 section, and the drift job dispatched green                                      | 30m | t003, t004, t005       |
| t007 | Walk the new commands on production through a launcher built from the tree, once the pin carries w22/m603                 | 40m | t006                   |
| t008 | Render parity                                                                                                             | 20m | t004, t005             |
| t009 | Simplify                                                                                                                  | 20m | t008                   |
| t010 | Test coverage                                                                                                             | 30m | t008                   |
| t011 | Closeout                                                                                                                  | 10m | t007, t009, t010       |

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
