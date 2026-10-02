# w1 · m159 — Dashboard truth: a datastore's own Status row, the landing after "Move to project", and seven count strings

**Worker:** worker1 **Goal:** the dashboard never contradicts itself about a resource's state or its location. A suspended Key Value or Postgres reads Suspended in every row that names its status, a resource moved into a project opens on the environment it actually landed in, and every `{count}` message reads correctly at one. **Status:** done (t001–t007 done. All four DoD bullets verified: bullets 1 and 3 live on 2026-09-30, bullet 2 live on 2026-10-02 after the cold-load race fix `363ef405a`, bullet 4 by `locale-parity.test.ts`. See § Live re-walk 2026-10-02)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The Key Value and Postgres Details cards translate status and plan instead of printing wire values — **DONE** | 40m | — |
| t002 | "Move to project" lands on the environment the resource is actually in — **DONE** | 45m | — |
| t003 | Seven `{count}` strings get native plurals, and the locale test fails when a count message has none — **DONE** | 40m | — |
| t004 | Render parity — **DONE** | 20m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t006, t004 |

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

**Re-run live 2026-09-28 (`/loopx w1`), and the 2026-09-15 conclusion below it was wrong.** Fixture `qa-20260928-kv` (`red-dat0lijncejs739qiuvg`) created free in `tea-d98210cbbpdc73dcrkvg`, waited to `available`, suspended (`POST …/suspend` → 202), read on all three surfaces, then deleted (`204`, then `GET` → `404`):

| Surface                       | status      | suspended   | plan   |
| ----------------------------- | ----------- | ----------- | ------ |
| REST `GET /v1/key-value/{id}` | `suspended` | `suspended` | `free` |
| GraphQL `keyValue(id:)`       | `suspended` | `suspended` | `free` |
| MCP `get_key_value`           | `suspended` | `suspended` | `free` |

- **The wire now reports `suspended`, and that is the correct Render shape.** Render's own pinned `databaseStatus` enum — which Key Value reuses, there being no separate `keyValueStatus` schema — **contains `suspended`**: `['creating','available','unavailable','config_restart','suspended','maintenance_scheduled','maintenance_in_progress','recovery_failed','recovery_in_progress','unknown','updating_instance']` (`lego/backend/internal/api/openapi/render-public-api-1.json`, `components.schemas.databaseStatus`). A suspended datastore reporting `available` was therefore never Render's shape.
- **Correction to the original t004 write-up.** It recorded `status: available` on all three surfaces and concluded "Render keeps `status` and `suspended` as separate fields … a suspended store legitimately reports `status: available`. Nothing on the wire should change." That was mistaken: the two fields are not orthogonal, because Render's status enum carries `suspended` as one of its values. The wire was fixed the same day by **`b465b6100` "fix(api): report suspended datastore status for CLI parity"** — suspension now outranks readiness in `keyvalue/service.go:244-252` and `postgres/service.go:423`, with `deleting` still winning over both. The motivation recorded there is the pinned CLI, which projects `Status` and drops the separate `Suspended` field (`w5/061`), so a suspended store read as available in `bex key-value list`.
- **The dashboard derivation is still required, and still correct.** `deriveStatus` does more than fold in suspension — it also resolves `creating`/`deleting` and the readiness conditions into the one word a user reads. Both the badge and the Details row now call it through a single `statusLabel()`, so they cannot drift regardless of what the enum reports. m159's fix is unaffected by `b465b6100`; the two changes are the presentation half and the wire half of the same defect.
- **Plan.** All three surfaces return the plan _id_ (`free`), matching Render, whose API returns the plan id while its dashboard shows a display name. The Details row resolves the id through the instance-type catalog the plan picker already queries.
- **No ADR018 change.** All three surfaces agree with each other and with Render's enum, so there is no divergence to record. The single-region substitution observed in passing (create requested `region: "oregon"`, every surface reports `fsn1`) is already recorded deliberate behavior — responses carry the configured `BEX_REGION` (ADR018 §250) — not a new finding.

## Closeout gate (2026-09-28, `/loopx w1`)

**t001–t006 are done. t007 closeout is held on one thing: nobody has looked at the live dashboard.**

What *is* verified, and how:

| DoD bullet | Evidence | Level |
| --- | --- | --- |
| A suspended datastore reads suspended everywhere | REST + GraphQL + MCP all report `status: suspended` / `plan: free` on live fixture `qa-20260928-kv` (created, suspended, read, deleted). Both Details cards asserted by `key-value-metadata-card.test.tsx` and `database-metadata-card.test.tsx` — "reads the suspended status, not a stale ready label (w1/m159)" + "reads the plan…" | live API + mounted component |
| A moved resource is visible where it landed | `environments-panel.test.tsx` — "lands on Unassigned when every Environment is empty (w1/m159)" | mounted component |
| Counts read correctly at one | the five `_one`/`_other` pairs, asserted per namespace | catalog + mounted |
| The rule is enforced from now on | `locale-parity.test.ts:113` `it.each(NAMESPACES)("$name: count messages are pluralized")` | green |

All four ran green on 2026-09-28: `4 files / 169 tests passed`.

**What is missing is only the browser walk the DoD prescribes** ("observable on `https://dashboard.bex.co` with throwaway `qa-<yyyymmdd>-` fixtures"). This session did not perform it, and the reason is deliberate rather than technical: signing the dashboard in requires putting `QA_PASSWORD` into a tool-call argument, which writes a live production credential into the session transcript. `.pm`/AGENTS.md's "never commit/print `.env`" is the rule being honored; the Kratos session token minted for the API probes has the same problem if injected as a cookie.

**To clear this gate, either:**

1. **Accept the evidence above** — every DoD bullet is asserted at component level against the exact states named, and the API half is verified live on production. Say so and t007 can run.
2. **Do the walk** — sign the browser in (`! ` prefix in this session works, or hand over an already-authenticated browser profile) and confirm the three visual bullets: a suspended Key Value and Postgres reading Suspended in both the header badge and the Details Status row in both locales, a row-level "Move to project" landing on Unassigned with the resource on screen, and `/blueprints/new` on `examples/hello-go/render.yaml` reading "1 resource to sync".

Nothing else in the milestone is outstanding.

## Live closeout 2026-09-30

**Result: two of three bullets pass live; the move-to-project landing fails on a cold load of the project page. t007 stays open and m159 stays in `blocked/`.**

**Build under test.** Production dashboard pin `ghcr.io/bex-co/bex-dashboard@sha256:6564b585…` (`deploy/gitops/base/dashboard.yaml`), written by `5c83c46c6` "pin platform images to `de9ac4d1c880`". `git merge-base --is-ancestor 7a2982f4e de9ac4d1c880` succeeds, so m159's fix commit (`7a2982f4e`, 2026-09-15) is in the deployed build.

**How the walk ran.** Headless Chromium (Playwright 1.63, scratchpad install) driven by Node scripts. The QA session came from `scripts/qa-login.sh <scratch file>`, which reads `QA_EMAIL`/`QA_PASSWORD` from `.env` inside its own process. No credential, cookie or token entered a tool argument or any output. The session was revoked at the end (`qa-login.sh --logout` → `ok logged-out`) and the state file was deleted. Workspace: `tea-d98210cbbpdc73dcrkvg`. Suspend and "Move to project" went through the UI (row-actions menu and the Danger Zone dialogs). Create and delete went through GraphQL from inside the signed-in page.

**Fixtures** (all created 2026-09-30 17:56–17:58Z, deleted 18:04Z):

| Fixture                          | id                         | Deleted → `GET` |
| -------------------------------- | -------------------------- | --------------- |
| Key Value `qa-20260930-m159kv`   | `red-daukqpg5maoc7384lncg` | 404             |
| Postgres `qa-20260930-m159pg`    | `dpg-daukqpg5maoc7384lndg` | 404             |
| Project `qa-20260930-m159p`      | `prj-daukqqo5maoc7384lnf0` | 404             |
| Environment `qa-e1` (in project) | `env-daukqvjei8sc73dev1eg` | 404             |

- **Control reads.** The same REST routes returned 200 for existing workspace resources, so the 404s mean deleted and not an unknown route. A final GraphQL list of projects, Key Value stores and databases shows no `qa-2026…` leftovers.

### 1. A suspended datastore reads suspended everywhere: **PASS**

The free Key Value store was suspended at 18:01:00Z through the "Suspend Key Value Instance" dialog, using the sudo phrase. The free Postgres (the free plan was available and cost nothing) was suspended at 18:01:53Z through "Suspend Database". The API then reported `status: suspended` for both.

```text
en kv: header badge "Suspended" | Status = Suspended | Instance type = Free
en pg: header badge "Suspended" | Status = Suspended | Instance type = Free
zh kv: header badge "已暂停"     | 状态 = 已暂停     | 实例类型 = Free
zh pg: header badge "已暂停"     | 状态 = 已暂停     | 实例类型 = Free
```

- **No wire words.** Neither the wire word `available` nor the lowercase plan id `free` appears anywhere on either page, in either locale.
- **The zh plan name.** In zh the plan reads `Free` because that is the catalog display name the API returns (`keyValueInstanceTypes` / `databaseInstanceTypes`: `{id: free, name: Free}`). The plan picker shows the same name.
- **Screenshots** (session scratchpad `m159/`): `kv-suspended-{en,zh}.png` and `pg-suspended-{en,zh}.png`. Each shows the h1 with a grey Suspended / 已暂停 pill and a Details card whose Status row reads the same word, with Instance type `Free`.

### 2. A moved resource is visible where it landed: **FAIL (cold load)**

The setup was a project with one empty environment, `qa-e1`. On the Overview's Ungrouped Resources row for `qa-20260930-m159kv`, the walk used Actions → "Move to project" → `qa-20260930-m159p`. The toast read `"qa-20260930-m159kv" moved to "qa-20260930-m159p".` (18:02:51Z). The screenshots are `move-menu.png` and `move-after.png`.

- **Pass: opening the project by clicking its card on the Overview.** The page lands on `/project/prj-daukqqo5maoc7384lnf0?env=unassigned`, and the selector reads **Unassigned** (zh: **未分配**). The picker also offers `qa-e1`. The table under Unassigned lists `qa-20260930-m159kv · Key Value · Suspended`. Screenshots: `project-landing-{en,zh}.png`.
- **Fail: opening the bare project URL on a fresh page load** (reload, bookmark, pasted link or new tab). 3 of 3 attempts rewrote the URL to `?env=env-daukqvjei8sc73dev1eg`. The selector reads `qa-e1` with the text "0 resources" and "No resources in this environment yet", and the moved store is not on screen. This is the filed symptom exactly. Screenshot: `project-cold-load-en.png`.

```text
cold load #1..#3: final=/project/prj-daukqqo5maoc7384lnf0?env=env-daukqvjei8sc73dev1eg selected=qa-e1 kvVisible=false
  batch 1: Workspaces+Environments
  batch 2: Projects+BillingReadiness+Services+Databases+KeyValues+EnvGroups
```

- **Root cause (from reading the code; the fix is untested).** The failure is a load-order race in `dashboard/src/features/environments/components/environments-panel.tsx`.
  - `Environments` resolves in the first `BatchHttpLink` batch. The Project's resource lists arrive in the second.
  - In that window `unassignedRows` is empty, so `selectEnvironmentId` falls through to `environments[0]` (`:66-67`).
  - The canonicalizing effect (`:131`) is gated only on the environments query's `loading` (`:100`). It writes `?env=<first env>` into the URL right away, and that explicit id then wins over the Unassigned default for good.
  - The comment at `:115-119` guards this race for an explicit `?env=unassigned` URL, but not for a URL with no `env`.
  - The card click passes only because the Overview has already filled Apollo's cache with the resource lists.
  - The t006 test (`environments-panel.test.tsx` "lands on Unassigned when every Environment is empty") passes `projectRows` synchronously, so it cannot see the race.
- **Suggested fix.** Do not canonicalize a missing `env` until the Project's resource lists have settled too. For example, pass a `rowsLoading` flag into `EnvironmentsPanel` and add it to the `:131` guard. Add a test that resolves `Environments` before the resource lists.

### 3. Counts read correctly at one: **PASS**

On `/blueprints/new` → GitHub → `bex-co/bex` (branch `main`), with the path field set:

```text
en examples/hello-go/render.yaml   : Blueprint file parsed successfully — 1 resource will change.
en examples/stack-demo/render.yaml : Blueprint file parsed successfully — 3 resources will change.
zh examples/hello-go/render.yaml   : 蓝图文件解析成功 — 将变更 1 个资源。
zh examples/stack-demo/render.yaml : 蓝图文件解析成功 — 将变更 3 个资源。
```

- **The DoD wording is out of date.** The DoD's "to sync" wording is from filing time. `w4/m138` (`f75f0749f`, which is in the deployed build) later reworded the key to "will change". The plural is what m159 fixed, and it is correct at both 1 and 3.
- **No `(s)`.** No `(s)` appears anywhere in the page body.
- **Screenshots:** `bp-{en,zh}-{hello-go,stack-demo}.png`.

### Side observation (not filed; seen only in headless Chromium)

On a fresh headless load, no `ViewerCapabilities` request goes out. Every capability-gated button stays disabled, and the Instance type card shows "Permissions could not be refreshed. Try again — this is not a role change." This covers Suspend, Delete and the plan picker.

- **It clears on focus.** A `window` `focus` event fires the request (200, all grants `allowed`) and enables the buttons. The walk sent that event before each UI action.
- **Unconfirmed in a headed browser.** It was not reproduced in a real browser, so it is recorded here and not filed. The code in question is `features/capabilities/context/capabilities-provider.tsx`: the initial `refresh("workspace")` whose request never leaves the page.

### To close

Fix the cold-load race above (a small w1 follow-up, or reopen t002), ship it, then re-walk only bullet 2 with a fresh-page load of `/project/<id>`. Bullets 1 and 3 need no re-walk.

## Blast radius

- **Who is hit.** Every datastore detail page, every "Move to project" from a resource row, and seven strings across Blueprints, env groups and services.
- **Severity.** Minor each, but all three are truthfulness defects a user meets immediately after a successful action.

## Source + Goal linkage

- **Source:** `w1/085`, `w1/086` and `w1/098` — live `/qa-find-bugs` passes 2, 4 and 26 (2026-09-14). Promoted 2026-09-15 during the w1 triage, grouped because each is sub-hour on its own and they share one review and one ship.
- **Goal linkage:** `docs/ADR018-render-parity.md` (the dashboard is a parity surface) and `dashboard/AGENTS.md` — a view must describe the state it is actually showing.
- **Expected outcome:** no dashboard row contradicts the resource it describes, and no count message reads "1 resources".
- **Why now:** all three were found live and none needs a decision; they are the cheapest user-visible wins left in w1's inbox.
- **Render parity is included** because all three are user-facing dashboard surfaces; `w6/done/062` set the plural rule this restores.

## Cold-load race fix (2026-09-30)

The live walk's bullet-2 failure is fixed in code. `EnvironmentsPanel` canonicalized the URL as soon as the Environments query resolved; on a fresh `/project/<id>` load that happens before the services/databases/key-value lists, so `unassignedRows` was still empty, `selectEnvironmentId` fell back to `environments[0]`, and that id was written into `?env=` and then honored as an explicit request forever after.

- `environments-panel.tsx`: new `resourcesLoading` prop; the canonicalizing effect waits for it as well as for `loading`.
- `project.$projectId.index.tsx`: passes `servicesLoading || databasesLoading || keyValuesLoading`.
- Regression: `environments-panel.test.tsx` — "does not pin the first Environment into the URL before resources load (w1/m159)" renders the cold-load order (Environments first, rows later) and asserts the only URL write is `unassigned`. It **fails without the guard** (called with `env-1`) and passes with it. `vitest src/features/environments` 66/66, `yarn typecheck` clean.

**Remaining for t007:** once this ships and a pin lands, re-walk bullet 2 only (fresh load of `/project/<id>` after a row-level move lands on Unassigned with the resource on screen). Bullets 1 and 3 passed live today.

## Live re-walk 2026-10-02

**Result: bullet 2 passes live. A fresh load of `/project/<id>` after a row-level "Move to project" lands on Unassigned with the store on screen, 6 of 6 times in en and zh. With bullets 1 and 3 passed on 2026-09-30, every DoD bullet is met and m159 is closed.**

**Build under test.** `deploy/gitops/base/dashboard.yaml` on `origin/main` pins `ghcr.io/bex-co/bex-dashboard@sha256:ab909cdb…`, written by `2790aaffe` "pin platform images to `e97ca42273a9`" (2026-10-02 06:27:47Z, deploy run 36969401352). `git merge-base --is-ancestor 363ef405a e97ca4227` succeeds, so the cold-load fix is in the build. The page itself confirms the fix is live: the first `BatchHttpLink` batch still carries `Environments` alone and the resource lists still arrive in a later batch, yet the URL is canonicalized only to `?env=unassigned`.

**How the walk ran.** Same method as 2026-09-30: headless Chromium driven by Node scripts in the session scratchpad (`m159b/`, a copy of `m159/lib.mjs` pointed at a fresh state file). The QA session came from `scripts/qa-login.sh <scratch file>`, so no credential, cookie or token entered a tool argument or any output. The session was revoked at the end (`--logout` → `ok logged-out`) and the state file was deleted. Workspace: `tea-d98210cbbpdc73dcrkvg`. Create and delete went through GraphQL from inside the signed-in page; the move went through the UI.

**Fixtures** (created 2026-10-02 06:34:40–06:35Z, deleted by 06:37:40Z):

| Fixture                          | id                         | Deleted → `GET` |
| -------------------------------- | -------------------------- | --------------- |
| Key Value `qa-20261002-m159kv`   | `red-davl10oaijhc73cn2pog` | 404             |
| Project `qa-20261002-m159p`      | `prj-davl11gaijhc73cn2pq0` | 404             |
| Environment `qa-e1` (in project) | `env-davl13eo5s1c7398lh60` | 404             |

- **Control reads.** `GET /v1/projects` and `GET /v1/key-value` for the workspace returned 200 with items, so the 404s mean deleted, not an unknown route. No `qa-20261002-m159…` name remains in the workspace.
- **Setup check.** Before the move, `environments(projectId:)` returned exactly one Environment, `qa-e1`, holding nothing.

**The move.** On the Overview's Ungrouped Resources row for `qa-20261002-m159kv`, Actions → "Move to project" → `qa-20261002-m159p`. The toast read `"qa-20261002-m159kv" moved to "qa-20261002-m159p".` (06:35:14Z).

**Fresh loads of the bare project URL.** Each attempt is a new browser context that opens `/project/prj-davl11gaijhc73cn2pq0` with no `env` parameter:

```text
en cold #1..#3: final=/project/prj-davl11gaijhc73cn2pq0?env=unassigned selected=Unassigned kvVisible=true
zh cold #1..#3: final=/project/prj-davl11gaijhc73cn2pq0?env=unassigned selected=未分配     kvVisible=true
  batch 1: Workspaces+Environments
  batch 2: Projects+BillingReadiness+Services+Databases+KeyValues+EnvGroups
```

- **The race window is still there, and it is now handled.** The batch order is the same one that failed on 2026-09-30. The Environments query resolves first, yet the page waits for the resource lists before it writes `?env=`.
- **Screenshots** (session scratchpad `m159b/`): `project-cold-load-{en,zh}.png`. Each shows the selector reading Unassigned / 未分配 and a table listing `qa-20261002-m159kv · Key Value` under All (1) / Key Values (1).

**Control: opening the project by clicking its card on the Overview** still lands correctly. It opens `?env=unassigned`, the selector reads Unassigned (zh: 未分配), and the store is visible. Screenshots: `project-landing-{en,zh}.png`.

**Side observation from 2026-09-30, no longer seen.** Every fresh load in this walk sent `ViewerCapabilities` on its own, with no synthetic `focus` event. The headless-only "Permissions could not be refreshed" state recorded on 2026-09-30 did not recur, so there is still nothing to file.

