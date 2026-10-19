# w8 · m119 — An overlapping deploy serves while recorded canceled; Restart then reverts to an older release

**Worker:** worker8 **Goal:** When deploys overlap, the release that actually serves is the one deploy history and `status.activeRevision` report, following Render's Wait policy. A Restart in that window keeps what is serving and never silently drops the saved change. **Status:** blocked — t001–t008 done; shipped `69e925197` and verified live (12/13 DoD checks). The open Restart/undeployedChanges bullet needs w8/m120: a release held through rollout keeps its own settings in its record

## Tasks (in order)

| id   | title                                                                                      | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------------ | --- | ------------------ |
| t001 | Choose the overlap fix: hold the pending slot through rollout, or record the pinned rollout | 45m | — — **DONE** |
| t002 | Operator: the pinned release's completed rollout is recorded served before the next release counts | 90m | w8/m119/t001 — **DONE** |
| t003 | Backend: an executing row that served closes live, and Restart in the window keeps it      | 60m | w8/m119/t002 — **DONE** |
| t004 | Restart during an in-flight deploy: Render semantics, named cancel, saved change kept pending | 45m | w8/m119/t003 — **DONE** |
| t005 | Blast radius: every pinned App type, overlapping trigger and activeRevision reader         | 40m | w8/m119/t003 — **DONE** |
| t006 | Render parity: deploy history, Restart and current revision across REST, GraphQL, MCP, UI  | 30m | w8/m119/t004, t005 — **DONE** |
| t007 | Simplify the changed code                                                                  | 20m | w8/m119/t006 — **DONE** |
| t008 | Test coverage: overlap live/deactivated progression and Restart in the window              | 60m | w8/m119/t006 — **DONE** |
| t009 | Close out the milestone                                                                    | 10m | w8/m119/t007, t008 |

Nine tasks, 400m.

## Definition of done

The same live journey as the [finding](finding.md), run against production after deploy with the pinned `bex` CLI. Create a fresh Free native Python web service, then run two back-to-back `bex services update --start-command` calls (A then B) whose commands write distinct markers served at `/v.txt`, and poll every 1–2s.

- Once the URL serves A, `bex deploys list` shows A `live` and the previous deploy `deactivated` within one reconciler resync. When B serves, B is `live` and A is `deactivated`. At no sample does the URL serve a release whose row is `canceled`, or a release other than the single `live` row (allowing one resync of lag at each transition).
- The App's `status.activeRevision` names the serving release's generation throughout, including while B builds.
- A third and fourth update (G then H), with `bex restart --confirm` issued while G serves and H builds: the service keeps serving G, never F or older, after the restart settles. H is not silently lost. Either H still deploys, or its row is closed with a `failureReason` naming the restart and the saved H configuration is surfaced as pending, per t004's Render-backed decision.
- Controls unchanged: a third trigger while one row is executing and one queued still cancels only the replaced queued row (w2/018); a build failure of the executing row still reads `build_failed` (w6/m100); a user `bex deploys cancel` keeps its m117/w5/m114 semantics.
- Regression tests fail before and pass after. The operator test asserts that the pinned release reaches Running/`activeRevision` (or its served stamp) before the pending generation counts as the release. A backend test feeds the reconciler the real operator sequence, not a hand-made Ready state. Both pass in CI.
- The fixture is deleted by ID with a 404 absence proof.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` solo loop sweep s3, 2026-10-19 00:05–00:31 UTC, `https://api.bex.co/v1/`, workspace `bex-canary`, human device login, pinned-CLI HEAD build `v0.3.3+d5c3373ed`. Three reproductions of the canceled-but-served window plus one Restart reproduction, with read-only App CR status timelines: [finding.md](finding.md).
- **Goal linkage:** deploys are the core hosting primitive (ADR008 Render-alternative core; ADR004 deployment; ADR060 §D1a run-to-completion builds). ADR018 already claims Render's Wait policy for overlapping deploys (w2/018). This milestone makes that claim true on the real operator and backend pair.
- **Expected outcome:** for the length of a newer build, deploy history, the current revision and Restart all agree with what the URL serves. A user who saves two changes in a row, or pushes twice, can trust which code is live, and a Restart cannot revert the service to an older release.
- **Why now:** any two overlapping triggers on any repo-backed service reproduce it; this sweep needed only two CLI updates one second apart, three times out of three. The failure modes compound: history shows the wrong live deploy for minutes, and a Restart (a routine incident action) silently reverts to older code and abandons the saved change, with every command exiting 0. The open w8/m117 works in the same overlap state machine, so the two should be sequenced or designed together rather than fixed in opposite directions.
- **Render parity included because** the fix changes tenant-visible deploy statuses, `latestDeploy`/current revision and Restart behavior on REST, GraphQL, MCP and the dashboard deploy history.
