# w4 · m184 — Say "applying" while the carrying deploy runs, not "deploy to apply"

**Worker:** worker4 **Goal:** while an in-flight standard deploy carries the saved configuration, the service header and Environment page say the changes go live when it finishes, instead of telling the user to start another deploy. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Operator reports whether the in-flight release carries the saved configuration — **DONE** | 45m | — |
| t002 | Project the flag on REST, GraphQL and MCP — **DONE** | 25m | w4/m184/t001 |
| t003 | Dashboard picks "applying" vs "deploy to apply" copy — **DONE** | 25m | w4/m184/t002 |
| t004 | Render parity — **DONE** | 15m | w4/m184/t003 |
| t005 | Simplify — **DONE** | 10m | w4/m184/t004 |
| t006 | Test coverage — **DONE** | 30m | w4/m184/t004 |
| t007 | Closeout | 10m | w4/m184/t005, w4/m184/t006 |

## Definition of done

Replay the [finding](finding.md) on an owned Free image service, then clean it up:

- `PATCH /v1/services/<id>` changing `healthCheckPath` starts a config-change deploy. While it runs, the header and Environment page say the saved changes go live when the current deploy finishes, and do not tell the user to start another deploy. `undeployedChanges` keeps meaning saved ≠ serving (m145/m165).
- After that deploy fails, the existing "Use a standard deploy to apply them" hint returns.
- A Restart of the served release, a rollback, a cancel, and a Save-only after the deploy started each keep the existing hint, because none of them carries the saved settings.
- REST, GraphQL and MCP expose the same sibling bex-extension field.

## Source + Goal linkage

- **Source:** inbox note w4/201 (loop40 live QA, 2026-10-05), promoted 2026-10-08 during `/loopx w4`: the fix spans operator status, three API surfaces and the dashboard (~2h), above the inbox sub-hour bar. The original note is kept verbatim as [finding.md](finding.md).
- **Goal linkage:** ADR008 actionable supervision; m145/m165 undeployed-changes contract.
- **Expected outcome:** no prompt to start a redundant second deploy while the carrying one is running.
- **Why now:** the hint shows on every config-change deploy for its whole duration (15 minutes in the repro).
- **Render parity included:** a tenant-facing copy and API-extension change; Render shows no "not live yet" notice during a running deploy.

## Progress (2026-10-08)

t001–t006 done.

- **Operator:** `AppStatus.UndeployedChangesApplying` is set by the saved-configuration status controller, only alongside `UndeployedChanges`. `inFlightCarriesSavedConfiguration` returns true when the App is `Deploying`, the requested release is newer than the serving one and not canceled, and it is not a Restart or rollback selection (`sourceGeneration > 0` or `preserveGroupValues`). The release must also either equal the saved configuration in its record, or have no record yet: a normal release snapshots the saved values when it dispatches. Phase changes now wake that controller, so a failed deploy clears the flag promptly. The main reconciler preserves the field across its status-conflict retries, as it does `UndeployedChanges`.
- **API:** `undeployedChangesApplying` on GraphQL, MCP (`AppView`) and REST (`render.go`, omitted when false).
- **Dashboard:** `undeployedChangesHintKey` picks "Saved changes go live when the current deploy finishes." (en/zh) for the header and Environment page; otherwise the existing hint shows. Codegen was regenerated offline from the schema dump.
- **Docs:** ADR018 Cancel-deploy row.

Tests:

- Operator `TestInFlightReleaseCarriesSavedConfiguration` covers: not dispatched (carries), settled, canceled, rollback/Restart selections, and a dispatched record that differs and then equals the saved config.
- Header copy test.
- Operator `make test`, backend apps, dashboard services (883), dashboard `yarn lint`, `make lint`, with no mobile codegen drift.

Not verified until the live replay: the build-based (repo) service window, and the Environment page rendering.
