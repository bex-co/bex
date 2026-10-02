# w1 · m165 — The Render drift watchdog has never once passed, and upstream `list_events` is unacknowledged

**Worker:** worker1 **Goal:** the weekly `Render schema drift` workflow reports real parity drift and nothing else — its REST half can actually compare (today it aborts on its own fixture), its MCP half is clean because `list_events` has been decided and recorded, and a future red run names the tool or fixture at fault and the action it needs. **Status:** in progress — t001–t007 done 2026-09-30; **t008 (Closeout) is blocked on shipping this work and a `workflow_dispatch` run**, which needs `/ship` (not authorized in this run)

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `render-schema` aborts before comparing anything: the webhook-vocabulary fixture is internally inconsistent — **DONE** | 35m | — |
| t002 | Decide upstream `list_events`: implement against bex's existing events surface, or acknowledge the divergence — **DONE** | 35m | — |
| t003 | Land the `list_events` decision — pin refresh plus the tool or the acknowledgment entry — **DONE**                     | 50m | t002       |
| t004 | A red drift run says what to do: name the tool or fixture at fault and the action it needs — **DONE**                  | 25m | t001, t003 |
| t005 | Render parity — **DONE**                                                                                               | 20m | t003, t004 |
| t006 | Simplify — **DONE**                                                                                                    | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                                               | 35m | t005       |
| t008 | Closeout                                                                                                    | 10m | t007       |

## Definition of done

**Narrowed 2026-09-30 by user decision** (the `buildSources` Blueprint re-pin is split out as `w1/117`, see § Scope discovery):

- The `render-mcp` job of the `Render schema drift` workflow passes on a `workflow_dispatch` run — the first passing run either job has had.
- The `render-schema` job **fails only on genuine upstream drift and names it**: today exactly the Blueprint-schema drift (`buildSources` and siblings, owned by `w1/117`), with the webhook-OpenAPI half green. It is expected to stay red until `w1/117` lands; that red is the check working, not the check broken.
- `scripts/render-schema-drift.sh` compares Render's published schemas against the reviewed pins instead of aborting on `docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json`; the inconsistency is either repaired in the fixture or the self-check that misreads it is corrected, and which one is stated in this README. (t001: the self-check was corrected — deleted.)
- Upstream's `list_events` tool is either implemented on bex's MCP surface or recorded as a deliberate divergence in `docs/ADR018-render-parity.md`, and `TestMCPParityUpstreamToolsAreImplementedOrAcknowledged` passes on that basis rather than by loosening the assertion.
- `lego/backend/internal/api/openapi/render-mcp-tools.json` carries a pin captured no earlier than this milestone, with its `source.commit` and `pin.capturedAt` refreshed.
- A deliberately introduced drift (a tool added to or removed from the pin) still fails the check, and the failure message names the tool and the action — proven by running the script against a mutated pin.
- GitHub issue [#74](https://github.com/bex-co/bex/issues/74) can close its `Render schema drift` half (or be narrowed to the `render-schema`/`w1/117` drift, which is now a legible real-drift red).

## Root cause

Two independent faults, one per job, which is why the workflow has never been green:

- **`render-schema`** — `scripts/render-schema-drift.sh` exits 1 with `Render webhook vocabulary fixture is internally inconsistent: docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json`. It never reaches the upstream comparison, so the REST half of the watchdog has asserted nothing since it shipped. This is the failure mode issue #74 predicted in the abstract ("the assertion is wrong … failing regardless of the manifest") and the reason a five-week streak carried no information.
- **`render-mcp`** — `scripts/render-mcp-drift.sh` exits 1 with `upstream ref=main added=[list_events] removed=[none]`. This half works: Render's MCP server genuinely registers a tool bex has not decided about. `list_events` appears nowhere in `docs/ADR018-render-parity.md`, `lego/backend/internal/api/mcp_parity.go`, or the pinned `render-mcp-tools.json` (22 tools, pinned `2026-08-18` at upstream commit `89c1f01b4`).

bex already owns the substance `list_events` would expose — the three-source durable events feed (`w3/m7`, `w3/m16`, `w3/m19`, `w7/m66`) and owner-scoped `GET /v1/events/{eventId}` hydration (`w8/m24`) — so t002 is a surface decision, not a feature build.

## t001 result — the fixture was innocent; the script's literal was stale (2026-09-28)

**Six of the seven jq clauses passed.** Evaluated individually against
`docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json`:

| clause | asserted | actual |
| --- | --- | --- |
| `renderOpenAPI \| length` | 67 | 67 ✅ |
| `renderDashboard \| length` | 64 | 64 ✅ |
| computed `apiOnly` == declared | — | identical ✅ |
| computed `dashboardOnly` == declared | — | identical ✅ |
| `apiOnly \| length` | 6 | 6 ✅ |
| `dashboardOnly \| length` | 3 | 3 ✅ |
| **`bexSupported \| length`** | **35** | **53** ❌ |

So the fixture is faithful and current; the **script** was wrong. bex's advertised
webhook vocabulary grew `35 → 42 → 53` as `w3/m82` and its siblings shipped
(ADR018 records the 42 → 53 step), the fixture tracked it, and the hardcoded
literal did not.

**The clause was redundant as well as stale**, which is why it was deleted rather
than bumped to 53. `lego/backend/internal/webhooks/vocabulary_test.go` already
owns every one of those seven clauses and asserts strictly more, **by value rather
than by count**: `TestRenderDashboardAndAPIVocabulariesStayDistinct` pins
`capturedAt`, the 67/64 counts and both set differences; and
`TestBexWebhookVocabularyHasOneTruthfulDispositionPerValue` pins
`bexSupported == bex's real EventTypes`, plus the full disposition ledger. That
test runs on every `go test ./...`, which gates deploys — a stronger gate than a
weekly cron. Bumping the literal to 53 would only have restarted the same clock.
Both tests pass today; corrupting the fixture (dropping one `bexSupported` entry)
makes the second fail and **name the missing value** (`service_woken`), so the
invariant is demonstrably still enforced.

**This was the whole reason the job had never passed.** The aborted jq block ran
*before* any network call, so the REST drift comparison — the only thing this
script uniquely does, and the thing no Go test can do — never executed.

**And it immediately earned its keep.** First real run reports:

- `Render webhook OpenAPI enum matches pinned 67-value fixture` ✅
- `::error title=Render Blueprint schema drift::pinned=665539cb… upstream=57aa0a1f…` — **genuine upstream drift, invisible for five weeks.** Render's `render.yaml.json` has gained `buildSources` ("Build sources that are not assigned to a project") on the top-level resources object.

Per this task's step 5 that drift is **recorded and filed, not absorbed** — see
§ Scope discovery below.

## t002 decision — converge on Render's name, because bex already built this tool (2026-09-28)

Captured upstream at `d9a8abd53669` (23 tools, `+list_events`, nothing removed):

```
list_events  args: cursor, endTime, eventTypes, limit, serviceId, startTime, workspaceId
             required: serviceId
```

**The premise in `events/mcp.go`'s header is now false.** It reads: "The official
server (render-oss/render-mcp-server @ 2a00be1, checked 2026-07-12) … has NO
events tool … there is no name to mirror and no parity gap to close — an events
tool is bex going further." bex therefore shipped **`list_service_events`** as a
deliberate extension. Upstream has since shipped the same capability as
`list_events`, so this is not a missing feature — it is **two names for one tool**:

| | bex `list_service_events` | upstream `list_events` |
| --- | --- | --- |
| service key | `serviceId` | `serviceId` (required) |
| type filter | `type` (single) | **`eventTypes`** |
| window | `startTime`, `endTime` | same |
| paging | `cursor`, `limit` | same |
| workspace | via middleware | explicit `workspaceId` |

**Decision: implement `list_events` to upstream's contract** (t003), rather than
acknowledge a divergence. Every argument is already answerable — bex's
`GET /v1/services/{id}/events` parses `type`/`startTime`/`endTime`/`cursor`/`limit`
(`events/rest.go:236`) — so this is a thin wrapper over an existing Render-shaped
read, not new data plumbing. Acknowledging instead would leave an agent written
against Render's MCP unable to call bex's equivalent, which is the whole point of
the pin treating tool **and argument** names as the contract.

**Question 1 is now answered, and it enlarges t003.** Upstream's source settles the
arity: `pkg/event/tools.go:63` declares `mcp.WithArray("eventTypes", …)` — an array
of strings — and `:170` collapses it onto the REST contract as
`eventtypes.ServiceEventType(strings.Join(eventTypes, ","))`. So Render's REST
`type` query parameter accepts **comma-joined multiple types**, which is how its
own MCP server drives it. (The pinned spec's `type` parameter is
`anyOf:[{string, enum:[43]}]`, a single alternative, so the spec alone does not
reveal this — only the source does.)

bex's filter is single-valued: `FilterOf(eventType, …)` builds
`Filter{Type: eventType}` (`events/service.go:661-662`), so `"deploy_started,build_ended"`
would match a literal type containing a comma — i.e. nothing. **This is therefore a
REST parity gap as well as an MCP one**, and t003 is not the thin wrapper t002
first assumed. Its real scope:

1. make the event `Filter` accept **multiple** types — comma-separated on REST (Render's wire), an array on MCP, and consistently on GraphQL;
2. add `list_events` with `eventTypes` as an array, `serviceId` required;
3. settle `list_service_events` (below);
4. rewrite `events/mcp.go`'s header, whose factual claim no longer holds;
5. refresh the pin to `d9a8abd53669`.

Item 1 changes a user-facing REST filter, so it carries its own parity and test
obligations — which is why this milestone is parked rather than pushed through on a
50-minute estimate.

Remaining open question for t003:
- **What happens to `list_service_events`.** Keep as an alias, or retire it. It is
  a published bex tool, so retiring it is a breaking change for anything already
  calling it. `get_service_event` (single event by `evt-…`) stays a bex extension
  either way — upstream still has no equivalent. **This is a user decision, not an
  implementation detail.**

`events/mcp.go`'s header comment must be rewritten in t003; its factual claim no
longer holds.

## Scope discovery — the DoD needs a second decision (2026-09-28)

The milestone's DoD says both jobs pass. `render-mcp` will go green with t003.
**`render-schema` will not**, because it now correctly reports the real Blueprint
schema drift above. Making it green means re-pinning `render.yaml.json` to
`57aa0a1f…`, and that is a Blueprint parity decision, not a side effect: bex's
compiler is fail-closed on unknown fields (ADR049), so adopting the new schema
forces a choice for `buildSources` — accept-and-ignore, implement, or reject with a
named error. That is its own piece of work and is **not** in this milestone's
scope. Either file it separately and narrow this DoD to `render-mcp` plus a
`render-schema` that fails only on true drift, or fold the re-pin in and accept the
larger scope. Flagged for the user.

**Resolved 2026-09-30 (user):** separate. Filed as **`w1/117`** (evidence, the
three ADR049 options, and a check of Render's docs — `render.com/docs/blueprint-spec`
does not yet mention `buildSource(s)`, `render.com/docs/build-sources` is 404). The DoD
above is narrowed accordingly. The same decision settled t002's open question:
**`list_service_events` stays as a working alias** (no breaking change);
`get_service_event` stays a bex extension.

## t003 result — `list_events` implemented to upstream's contract; pin re-captured (2026-09-30)

**Pin.** Re-captured with `scripts/render-mcp-capture.py --ref main` at upstream
`d9a8abd5366944987d89031ae014cc44cd683420` (commitDate `2026-09-25T16:21:45Z`, still
upstream `main` on 2026-09-30); `pin.capturedAt` `2026-09-30`. The `tools` array is the
capture verbatim (`diff` against the capture is empty); only the human `pin` block
(`capturedAt`, a dated note line) and `source` changed. New digest
`8c044390d89fc2a2129714ba39e786de201af9d05deccb8f751e204c1c15ed2b` in both
`renderMCPToolsSHA256` and `scripts/render-mcp-drift.sh`. Real run:
`Render MCP tool surface matches the pin (23 tools at main)`, exit 0.

**The refresh revealed a second, unreported upstream change** (step 5):
`bc94f8d` (#154) added `dockerCommand`/`dockerContext`/`dockerfilePath` to
`create_web_service`/`create_cron_job` and made `buildCommand`/`startCommand` optional.
bex already takes `dockerfilePath` and leaves build/start optional, so the gap is
`dockerCommand` + `dockerContext`. Both tools were already `Divergent` (region) — their
`mcpAcceptedDivergences` reasons now name the new gap, and it is **filed as `w1/118`**,
not built. The old drift headline had hidden it entirely (`added=[list_events]` only);
see t004.

**Code.**

- `events.Filter.Type string` → `Types []string`. `FilterOf` reads each entry in
  Render's comma wire (`splitTypes`: trim, drop blanks, dedupe), so REST `?type=a,b`,
  GraphQL `type: "a,b"` and MCP `eventTypes: [a, b]` are the same set. `pushDown`
  returns the **union** of each type's verbs/phases/fact types, still in SQL.
- `store.AutoDeployFilter` became an OR-able bit set (`Changed` 3 → 4) applied **only to
  `apps.SetAutoDeploy` rows** — before, it constrained every audit row, so
  `autodeploy_enabled,plan_changed` would have silently dropped the plan rows.
- REST gate: `renderCommaListQueryCompatibility` re-declares `list-events`' `type`, on
  that operation only (fresh ref; the shared `eventTypeParam` component untouched), as
  `style=form, explode=false` array of the pinned enum. Each element is still
  enum-checked: `?type=deploy_ended,env_vars_changed` is still a 400 naming the enum.
  Unknown types on GraphQL/MCP match nothing — the existing single-type behaviour.
- MCP `list_events`: `serviceId` (required), `eventTypes` array, `startTime`, `endTime`,
  `cursor`, `limit` + injected `workspaceId` → classified **`Parity1to1`**. Upstream's
  7-day default lookback. `list_service_events` kept unchanged as an alias (its `type`
  now also accepts the comma form, like REST). `events/mcp.go` header rewritten.
- `TestMCPParityInventory`: 188 → 189 tools, `Parity1to1` 10 → 11. The guard assertions
  in `mcp_parity_test.go` are unchanged (diff touches only the inventory table).
  Scope matrix: `MCP list_events` = `Read` (same as `list_service_events`).
- GraphQL schema unchanged (the `type` argument already existed) → no dashboard codegen.

## t004 result — a red run names the fault and the action (2026-09-30)

- **`render-mcp`** headline now carries `changed=[…]` beside `added`/`removed`, and the
  body gives the decision per kind (implement / `mcpKnownUpstreamOnly` for added; delete
  the acknowledgment for removed; fix or `mcpAcceptedDivergences` for changed) plus a
  corrected refresh recipe (the old one, `--out $pinned`, would have overwritten the
  human `pin` block and never mentioned the script's own digest). Proven against real
  upstream with the **pre-refresh pin**:
  `::error title=Render MCP tool drift::upstream ref=main added=[list_events] removed=[none] changed=[create_cron_job create_web_service]`
  and with a **mutated pin** (list_events dropped, `zz_fake_tool` added):
  `… added=[list_events] removed=[zz_fake_tool] changed=[none]`, exit 1.
- **`render-schema`** now names what moved, the file, and the action. Real run today
  (exit 1, the w1/117 drift):
  `::error title=Render Blueprint schema drift::https://render.com/schema/render.yaml.json no longer matches the pin …/render.yaml.json (pinned=665539cb… upstream=57aa0a1f…)`,
  then `Added upstream:` — `definition buildSource`, `buildSourceGit`,
  `buildSourceRuntime`, `workflowService`, `cronPlan`/`keyValuePlan`/`postgresPlan`/`serverPlan`,
  `property allOf.1.properties.buildSources`, `definitions.serverService.properties.buildSource`, … —
  `Removed upstream: definition plan`, and the ADR049 action (accept-and-ignore /
  implement / reject with a named error, recorded in `capabilities.json`, then move all
  three digests). Webhook half: `Render webhook OpenAPI enum matches pinned 67-value fixture`.
  Integrity, webhook-drift and moved-enum failures also name file + action.
- Check-never-update: both scripts write only to `mktemp` files; asserted by test
  (pin/fixture byte-identical after a drifting run).
- **Not done here:** "a `workflow_dispatch` run has both jobs green" — superseded by the
  narrowed DoD and moved to t008 (needs the change on `main`).

## t005 result — Render parity (implement path) (2026-09-30)

Checked: MCP `list_events`/`list_service_events`, REST `GET /v1/services/{id}/events`,
GraphQL `serviceEvents` all go through `FilterOf` → `Service.List`: same types, ids,
details, cursors, same multi-type union (`TestEventsMultiTypeFilterAgreesAcrossSurfaces`),
same errors (403 non-member, 503 store-less, 400 bad cursor/window — `Service.List`
owns them). ADR018 (inventory paragraph, events row, "bex ahead" bullet) and ADR006
(MCP paragraph, inventory, tool table) updated; `docs/render-artifacts/service-events.md` too.
No `.pm/DO_NOT_DO.md` row reopened. Dashboard unaffected (no schema change).

Recorded differences from upstream's `list_events` (kept, not absorbed — none breaks
call-compatibility, which is what the pin measures):

- **Output shape:** bex returns structured `{events: [{event, cursor}]}` (the REST
  envelope, like every bex MCP tool); upstream returns text (a JSON array + `cursor: …`).
- **Cursor pages:** upstream drops its 7-day default once a cursor is given; bex keeps
  the default window on every page (paging stays inside the same query), so reaching
  past 7 days needs an explicit `startTime` — the same advice upstream's description gives.
- **Bad `startTime`:** upstream returns a tool error; bex ignores an unparseable time and
  uses the default window, the documented `FilterOf` rule shared with REST/GraphQL.
- **Item enum:** upstream declares Render's enum on `eventTypes` items; bex does not
  (unknown types match nothing, as on GraphQL), so bex-named types remain filterable
  over MCP.
- A deployment with `BEX_MAX_QUERY_HOURS` < 168 would 400 `list_events`' default window;
  the default cap is 720h.

## t006 result — simplify (2026-09-30)

Applied: the two MCP list handlers share one `listTool` body; `pushDown` split into a
union loop over the unchanged single-type `pushDownOne`; one `appendMissing` helper for
both the type set and the pushed-down sets. The MCP `changed` computation is a single
jq expression. Declined: a shared "drift report" helper for the two scripts — their
reports differ in kind (tool names vs schema paths), so there is no second caller of
the same logic.

## t007 result — test coverage (2026-09-30)

Each new test was run against the pre-fix code and failed:

- `scripts/render-mcp-drift.test.sh` +6 cases (changed tool named; required-only change
  named; added not also "changed"; added/changed actions name `mcpKnownUpstreamOnly` /
  `mcpAcceptedDivergences`; pin untouched) — 5 fail on the old script.
- New `scripts/render-schema-drift.test.sh` (16 cases, network-free via `file://` URLs,
  wired into `scripts.yml` + its shellcheck list) — 11 fail on the old script.
- `store.TestPGAutoDeployFilterIsAVerbScopedBitSet` (real Postgres) — 3 of 6 subtests fail
  on the old SQL (`rows = [enabled], want [scale enabled]`, …).
- `events.TestMultiTypeFilterIsTheUnionPushedDown`, `TestFilterOfReadsRendersCommaWire`.
- `api.TestEventsMultiTypeFilterAgreesAcrossSurfaces` (old gate: REST
  `?type=deploy_ended,server_failed,suspender_added` = 400),
  `TestEventsMultiTypeRESTStillValidatesEveryType`, `TestListEventsWindowFollowsUpstream`,
  `TestListEventsCommaListIsOperationScoped`.
- An unimplemented upstream tool fails `go test ./...`: with `list_events` unregistered,
  `TestMCPParityUpstreamToolsAreImplementedOrAcknowledged` fails naming it.
- The fixture-consistency fault is in-suite since t001 (`webhooks/vocabulary_test.go`).
- No test asserts on tool descriptions or annotations.

Runs (2026-09-30): `cd lego/backend && BEX_TEST_DB_URI=<ephemeral postgres:17> go test -p 1 -count=1 ./...`
→ 67 packages ok (OpenFGA-gated tests skipped: no local OpenFGA); `cd lego/operator && make lint`
→ 0 issues ×4; `bash scripts/render-mcp-drift.test.sh`, `bash scripts/render-schema-drift.test.sh`
→ all pass; `bash scripts/render-mcp-drift.sh` → exit 0; `bash scripts/render-schema-drift.sh`
→ exit 1 on the w1/117 Blueprint drift only.

## t008 — remaining (blocked on ship)

1. `/ship` this work; 2. `gh workflow run render-schema-drift.yml`, record the run id:
`render-mcp` green, `render-schema` red **only** with the Blueprint drift above; 3. narrow
issue #74 to that legible drift (→ `w1/117`); 4. move t008 to `done/`, this milestone to
`w1/done/m165/`, tick the w1 README box.

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
