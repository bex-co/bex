# w1 · m165 — The Render drift watchdog has never once passed, and upstream `list_events` is unacknowledged

**Worker:** worker1 **Goal:** the weekly `Render schema drift` workflow reports real parity drift and nothing else — its REST half can actually compare (today it aborts on its own fixture), its MCP half is clean because `list_events` has been decided and recorded, and a future red run names the tool or fixture at fault and the action it needs. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `render-schema` aborts before comparing anything: the webhook-vocabulary fixture is internally inconsistent | 35m | —          |
| t002 | Decide upstream `list_events`: implement against bex's existing events surface, or acknowledge the divergence | 35m | —          |
| t003 | Land the `list_events` decision — pin refresh plus the tool or the acknowledgment entry                     | 50m | t002       |
| t004 | A red drift run says what to do: name the tool or fixture at fault and the action it needs                  | 25m | t001, t003 |
| t005 | Render parity                                                                                               | 20m | t003, t004 |
| t006 | Simplify                                                                                                    | 15m | t005       |
| t007 | Test coverage                                                                                               | 35m | t005       |
| t008 | Closeout                                                                                                    | 10m | t007       |

## Definition of done

- Both jobs of the `Render schema drift` workflow (`render-schema` and `render-mcp`) pass on a `workflow_dispatch` run — the first passing run the workflow has ever had.
- `scripts/render-schema-drift.sh` compares Render's published schemas against the reviewed pins instead of aborting on `docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json`; the inconsistency is either repaired in the fixture or the self-check that misreads it is corrected, and which one is stated in this README.
- Upstream's `list_events` tool is either implemented on bex's MCP surface or recorded as a deliberate divergence in `docs/ADR018-render-parity.md`, and `TestMCPParityUpstreamToolsAreImplementedOrAcknowledged` passes on that basis rather than by loosening the assertion.
- `lego/backend/internal/api/openapi/render-mcp-tools.json` carries a pin captured no earlier than this milestone, with its `source.commit` and `pin.capturedAt` refreshed.
- A deliberately introduced drift (a tool added to or removed from the pin) still fails the check, and the failure message names the tool and the action — proven by running the script against a mutated pin.
- GitHub issue [#74](https://github.com/bex-co/bex/issues/74) can close its `Render schema drift` half.

## Root cause

Two independent faults, one per job, which is why the workflow has never been green:

- **`render-schema`** — `scripts/render-schema-drift.sh` exits 1 with `Render webhook vocabulary fixture is internally inconsistent: docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json`. It never reaches the upstream comparison, so the REST half of the watchdog has asserted nothing since it shipped. This is the failure mode issue #74 predicted in the abstract ("the assertion is wrong … failing regardless of the manifest") and the reason a five-week streak carried no information.
- **`render-mcp`** — `scripts/render-mcp-drift.sh` exits 1 with `upstream ref=main added=[list_events] removed=[none]`. This half works: Render's MCP server genuinely registers a tool bex has not decided about. `list_events` appears nowhere in `docs/ADR018-render-parity.md`, `lego/backend/internal/api/mcp_parity.go`, or the pinned `render-mcp-tools.json` (22 tools, pinned `2026-08-18` at upstream commit `89c1f01b4`).

bex already owns the substance `list_events` would expose — the three-source durable events feed (`w3/m7`, `w3/m16`, `w3/m19`, `w7/m66`) and owner-scoped `GET /v1/events/{eventId}` hydration (`w8/m24`) — so t002 is a surface decision, not a feature build.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-09-21, items 1 and 5 ("CI is red"), re-verified 2026-09-27. Those items' original findings had shipped by then — `test (backend)` is green, deploys resumed 2026-09-26/27, the ST1005 lint break is fixed, and the alerting task was overtaken by `ci-red-streak.yml` + `ci-schedule-drift.yml`. What re-verification surfaced instead is this workflow, red on every run since 2026-08-24 and reported by that new watchdog as GitHub issue [#74](https://github.com/bex-co/bex/issues/74) ("**Render schema drift** — 5 consecutive failures. Never passed in the runs inspected. The check may have shipped broken.").
- **Goal linkage:** Render API compatibility ([ADR018](../../../docs/ADR018-render-parity.md), [ADR006](../../../docs/ADR006-bex-api.md)). This workflow is the only automated guard against bex's MCP surface drifting from Render's — the guard `w1/m70` added precisely because parity had previously been asserted in hand-written comments until bex reached 213 tools against upstream's 22 with nobody deciding to.
- **Expected outcome:** the weekly gate goes green for the first time and thereafter fails only on real drift; upstream's MCP tool surface is decided rather than merely unmatched; issue #74's second half closes.
- **Why now:** a watchdog that has never passed is worse than no watchdog — issue #74 says so in its own words ("it normalizes a red workflow, and the next genuine failure goes unread"), and it is now demonstrably true: `list_events` has been drifting unnoticed for five weeks *behind* a red signal nobody could read. The cheap half (the fixture) blocks the informative half from ever being seen.
- **Render parity:** **included** — t003 may add an MCP tool, which is a tenant/agent-facing surface, and the decision has to be consistent with what REST and GraphQL already expose for events (`GET /v1/events/{eventId}`, the events feed) rather than adding a fourth vocabulary. If t002 lands on "acknowledge", t005 verifies the ADR018 row instead of code.

## Out of scope

- Implementing any other upstream MCP tool the refreshed pin may reveal — t003 refreshes the pin and t004 makes a red run legible; new tools beyond `list_events` are filed, not built here.
- The external log/metric drain surfaces ADR018 marks `—`, and every other DO_NOT_DO `—` row. Refreshing the pin must not be read as reopening them.
- Forking, vendoring, or patching `render-oss/render-mcp-server` — the pin is captured from upstream, never edited (`.pm/DO_NOT_DO.md`, CLI decision 2026-07-14).
