# w1 · m159 — Dashboard truth: a datastore's own Status row, the landing after "Move to project", and seven count strings

**Worker:** worker1 **Goal:** the dashboard never contradicts itself about a resource's state or its location. A suspended Key Value or Postgres reads Suspended in every row that names its status, a resource moved into a project opens on the environment it actually landed in, and every `{count}` message reads correctly at one. **Status:** todo (t001, t002, t003, t005 and t006 done; t004 parity and the live DoD wait on the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The Key Value and Postgres Details cards translate status and plan instead of printing wire values — **DONE** | 40m | — |
| t002 | "Move to project" lands on the environment the resource is actually in — **DONE** | 45m | — |
| t003 | Seven `{count}` strings get native plurals, and the locale test fails when a count message has none — **DONE** | 40m | — |
| t004 | Render parity | 20m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Each bullet is observable on `https://dashboard.bex.co` with throwaway `qa-<yyyymmdd>-` fixtures, and on a local dev build for the locale half.

- **A suspended datastore reads suspended everywhere.** Create a free Key Value store, suspend it, and open its page: the header badge and the Details card's Status row both read Suspended, and Instance type reads the plan's display name. At filing time the header read **Suspended** while the Details row read **available** and the plan read **free**. The same holds for a Postgres instance, and in both locales.
- **A moved resource is visible where it landed.** "Move to project" on an ungrouped resource row, then open the project: the environment picker selects the environment that holds the resource (Unassigned when that is where it landed), and the resource is on screen. At filing time the page opened on an empty environment while the resource sat under Unassigned.
- **Counts read correctly at one.** `/blueprints/new` with `examples/hello-go/render.yaml` reads "1 resource to sync" (control: `examples/stack-demo/render.yaml` reads "3 resources to sync"), and no user-visible string carries `(s)`.
- **The rule is enforced from now on.** `dashboard/yarn test` fails when an `en` message containing `{count}` has no `_one`/`_other` form.

## Implementation (2026-09-15)

**The datastore Details cards (t001).** Both detail routes rendered the wire value beside a badge that had already interpreted it.

- `dashboard/src/routes/keyvalue.$keyValueId.tsx` and `databases.$databaseId.tsx`: the Status row now renders `t(STATUS_LABEL[deriveStatus(resource).key])` — the exact derivation `KeyValueStatusBadge` / `DatabaseStatusBadge` use, where suspension wins over the `status` enum. A suspended store reports `status: "available"` on the wire, which is why the row and the badge disagreed.
- The Plan row resolves the plan id through the instance-type catalog (`useKeyValueInstanceTypes` / `useDatabaseInstanceTypes`) and falls back to the raw id while the cache-first query is empty, so the row never renders blank.

**The move-to-project landing (t002).** `environments-panel.tsx` defaulted to `environments[0]` whenever the URL requested nothing. A row-level move joins the Project with no Environment, so the resource lands under Unassigned and the page opened empty. The default now falls to Unassigned **only when every Environment is empty**; a project whose Environments hold resources still opens on the first one, which the existing "renders only the selected environment" test pins.

The create form's hint was the other half of the contradiction: it read "A resource joins a Project only through an Environment", which both the row-level move and the selector's own "No environment" option disprove. It now says the resource sits under Unassigned in that case, keeping the "create one from the Project's page first" sentence that `w6/042`'s test pins.

**The count strings (t003).** Five live keys became `_one`/`_other` pairs in `en` (and `_other` only in `zh`, which has no `one` category): `blueprints.previewValid`, `agentSessions.showEarlierMessages`, `git.repoCount`, `envGroups.serviceCount`, `services.scaleSuccess`. Every call site already passed `count`, so none changed. `envGroups.varCount` and `envGroups.fileCount` had no call site anywhere in `dashboard/src` and were deleted rather than pluralized.

**Tests (t006).**

| Test | Pins | Fails under |
| --- | --- | --- |
| `keyvalue.$keyValueId.test.tsx` — "reads the suspended status and the plan's name in the Details card" | A suspended store shows Suspended in both the badge and the Details row, never the wire word `available`, and the plan reads its catalog name | The pre-fix rows: the raw `status` renders `available` under a Suspended badge |
| `keyvalue.$keyValueId.test.tsx` — the available and creating cases | The status word now appears exactly twice (badge + row) | A row that prints the wire value instead of the label |
| `databases.$databaseId.metadata.test.tsx` (new) | The Postgres card's Status row reads Suspended for a suspended instance and the Plan row reads the catalog name, not `basic-1gb` | The same pre-fix rows |
| `environments-panel.test.tsx` — "lands on Unassigned when every Environment is empty" | A project whose only Environment is empty opens on Unassigned with the moved resource listed | `environments[0]` as the unconditional fallback |
| `locale-parity.test.ts` — "count messages are pluralized" | Any `en` message interpolating `{count}` must have `_one`/`_other`, with label-only counts listed as explicit exemptions | The seven keys this milestone fixed, and any future un-pluralized `{count}` string |

**Simplify (t005).** Three reviewers (reuse, quality, efficiency) over the milestone diff.

- **Applied:**
  - each feature's `labels.ts` gained a `statusLabel()` that composes `deriveStatus` + `STATUS_LABEL` once, and the badge and the Details row now both call it, so the two cannot drift;
  - both `MetadataCard`s moved out of their route files into `features/keyvalue/components/key-value-metadata-card.tsx` and `features/databases/components/database-metadata-card.tsx`, where every one of their siblings already lives — this removed the `export` that existed only so a test could render the card, and let both cards get the same unit test instead of one route-render test and one unit test;
  - `environmentsAllEmpty` is memoized on `[environments]`, matching the `unassignedRows` derivation directly above it;
  - the Unassigned branches collapsed into one named `landOnUnassigned` condition, dropping a level of ternary nesting;
  - formatting redone with the dashboard's own pinned Prettier (3.8.1) rather than the repo's Markdown pin (3.4.2), which wraps `description:` strings differently.
- **Declined:**
  - **Rendering the status badge component as the Details row's value.** It removes the same two lines `statusLabel()` now removes, but puts a colored pill inside a definition list — a visual change this milestone has no way to verify, next to a header that already shows that badge.
  - **Lifting `useDatabaseInstanceTypes` to the databases page.** It would not remove the query from the cold-load path: the plan section is lazy behind `DeferredMount`, so there is nothing to dedupe against on first paint, and Apollo's cache already serves it when that section mounts. The op rides the initial `BatchHttpLink` batch and never blocks paint, which is the cost of showing a plan's name instead of its id.
  - **A `byID` accessor on both instance-type hooks** (services' `useInstanceTypes` has one). Four `.find` call sites across two features would change; it is a real cleanup but outside a three-bug milestone.

**Suites.**

- **`dashboard/yarn test`**: 3219 tests pass. Four route-tree files fail to _collect_ in the scratch worktree only — Vite denies an absolute path outside the project root because `node_modules` is symlinked there; all four pass (76 tests) in a checkout with real dependencies, at `origin/main` as well as with this diff.
- **`yarn typecheck`**, **`yarn eslint .`** and **`yarn lint:unused`** (knip) are clean.

## Live verification (2026-09-15, production)

**Fixture.** `qa-20260915-m159kv` (`red-dakv3hqsh60c73ao4li0`), a free Key Value store in the QA workspace, created 01:32:55Z and suspended at 01:34:32Z (0.8 s). Deleted at the end of the check.

**Before the fix** — read on the dashboard still running the pre-m159 build, through headless Chrome with the QA session (Playwright MCP was disconnected this session):

```text
01:36:44Z  GET https://dashboard.bex.co/keyvalue/red-dakv3hqsh60c73ao4li0
           h1            : qa-20260915-m159kv
           header badge  : Suspended
           Details card  : Status = available
                           Instance type = free
```

- **The filed contradiction, reproduced.** The header badge and the Status row of the same card disagree about the same store, and both the status and the plan render as raw wire values.

**Move to project, before the fix.** A second fixture on the same pre-m159 dashboard: project `qa-20260916-m159p` (`prj-dakvcc0dp28s73ek2h4g`) created 01:51:44Z with one environment, `qa-e1` (`env-dakvcc0dp28s73ek2h50`). The suspended store was then moved in through `setProjectKeyValues` — the same mutation the resource row's "Move to project" action sends — which returned the store in the project's `keyValueIds`.

```text
01:52:54Z  GET https://dashboard.bex.co/project/prj-dakvcc0dp28s73ek2h4g
           h1                   : qa-20260916-m159p
           environment selector : qa-e1
           that environment     : "0 resources"
           moved store visible  : no
           Unassigned offered   : no
```

- **The filed symptom, reproduced.** Immediately after a successful move the project page opens on an empty Environment, and the resource that was just moved is nowhere on screen.

**The count string, before the fix.** On the same pre-m159 dashboard, `/blueprints/new` with `bex-co/bex` and the Blueprint path `examples/hello-go/render.yaml` (a one-resource file):

```text
01:55Z  Blueprint file parsed successfully — 1 resources to sync.
```

- **The filed symptom, reproduced** — `w1/098`'s exact string, still live.

**Post-fix live verification: still outstanding (2026-09-16 04:48Z).** The fix is on `main` and green; what is missing is the production re-check, and it is blocked on an image pin that never landed. Production still runs the `w1/m158` build (`eb035151a`): every deploy run tonight either was superseded before its write-back or failed on an unrelated gate, because 13 commits landed on `main` in the final hour against a pipeline that takes ~50 minutes. Nothing about this milestone's code is implicated.

The pre-fix evidence above was captured deliberately while that was still true, so the "before" half is real. To finish: once any pin newer than `eb035151a` lands, re-create a fixture and re-run the same probe for the datastore card, the move-to-project landing and the count string — the tooling is in the session scratchpad (`m159-verify.mjs`, `m160-postfix.py`, `m161-idle-probe.py` with `m161-ws-client.py`).

## Render parity (t004)

**The three API surfaces agree, and they keep Render's shape.** Read live at 01:47Z against the suspended fixture `red-dakv3hqsh60c73ao4li0`:

| Surface                       | status      | suspended   | plan   |
| ----------------------------- | ----------- | ----------- | ------ |
| REST `GET /v1/key-value/{id}` | `available` | `suspended` | `free` |
| GraphQL `keyValue(id:)`       | `available` | `suspended` | `free` |
| MCP `get_key_value`           | `available` | `suspended` | `free` |

- **This is the contract, not a bug.** Render keeps `status` and `suspended` as separate fields, and bex mirrors that on all three surfaces — a suspended store legitimately reports `status: "available"`. Nothing on the wire should change, and **no divergence is recorded in ADR018**.
- **Which is exactly why the dashboard must derive what it shows.** The presentation layer is the only place these two facts are combined into the one word a user reads; printing `status` raw is what made the Details card contradict the badge beside it. `deriveStatus` (suspension wins over the enum) is that combination, and after m159 both the badge and the Details row call it through one `statusLabel()`.
- **Plan.** All three surfaces return the plan _id_ (`free`), which is also correct — the human-readable name lives in the instance-type catalog the dashboard already queries for its plan picker, and the Details row now resolves it there.

## Blast radius

- **Who is hit.** Every datastore detail page, every "Move to project" from a resource row, and seven strings across Blueprints, env groups and services.
- **Severity.** Minor each, but all three are truthfulness defects a user meets immediately after a successful action.

## Source + Goal linkage

- **Source:** `w1/085`, `w1/086` and `w1/098` — live `/qa-find-bugs` passes 2, 4 and 26 (2026-09-14). Promoted 2026-09-15 during the w1 triage, grouped because each is sub-hour on its own and they share one review and one ship.
- **Goal linkage:** `docs/ADR018-render-parity.md` (the dashboard is a parity surface) and `dashboard/CLAUDE.md` — a view must describe the state it is actually showing.
- **Expected outcome:** no dashboard row contradicts the resource it describes, and no count message reads "1 resources".
- **Why now:** all three were found live and none needs a decision; they are the cheapest user-visible wins left in w1's inbox.
- **Render parity is included** because all three are user-facing dashboard surfaces; `w6/done/062` set the plural rule this restores.
