# w1 · m153 — Background polls close open dialogs and interrupt typing on the environment card, the source picker and the env-group editor

**Worker:** worker1 **Goal:** once a polled dashboard query has data, a background poll or refetch never swaps the rendered content for its loading skeleton. Open dialogs stay open, unsaved drafts keep their values, and a focused input keeps focus across every 30 s tick. The skeleton renders only on the true first load, when there is no data yet. **Status:** done (2026-09-15). All eight tasks are complete. Every DoD bullet, t002's menu checks and t003's editor pass live on production. t002 and t003 have their own vitests, which fail on the pre-fix code. t005's Render dashboard comparison was not performed, because no Render session was available; see § Parity.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The project environment card stays mounted across `Environments` polls (`useEnvironments` reports first-load-only `loading`) — **DONE** | 25m | — |
| t002 | The service source picker's GitHub tab stays mounted across `GitConnections` polls — **DONE** | 20m | — |
| t003 | The env-group Environment editor keeps its rows mounted across `EnvGroup` polls (probe live first) — **DONE** | 25m | — |
| t004 | Blast radius: every polled hook that returns raw `loading`, the `refetchQueries` path, and one written convention — **DONE** | 40m | t001, t002, t003 |
| t005 | Render parity — **DONE** | 10m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

Run each bullet on the production dashboard while watching GraphQL operation names, either in the browser's Network tab or with a Playwright request listener. The baseline poll is `RESOURCE_POLL_INTERVAL_MS` = 30 s (`dashboard/src/common/lib/polling.ts:11`). Fixtures are a throwaway project with one environment holding one throwaway service. Only states observed at filing time are listed:

- **The environment settings dialog survives polls.**

  1. On the project page, open the environment's "More actions" → "All settings".
  2. Type `203.0.113.0/24` into "New CIDR block".
  3. Wait 65 s without clicking, covering at least two `Environments` polls.

  The dialog is still open and the field still holds `203.0.113.0/24`. At filing time the `Environments` poll went out 24.3 s after the dialog opened, and the dialog was gone at 24.9 s, taking the typed CIDR with it.

- **The Manage resources dialog survives polls.** Open "Manage resources", tick an unassigned service without saving, and wait 65 s. The dialog is still open and the box is still ticked. At filing time the poll went out at 26.0 s and the dialog closed at 26.9 s. Nothing was written: `GET /v1/environments/<env>` still returned the one previously assigned `serviceIds`.
- **The environment card is never replaced by its skeleton after first load.** 150 ms after an `Environments` poll request, the "Manage resources" button is present and `main` contains no skeleton nodes. At filing time the button was absent, `main` had 26 skeleton nodes (`[data-slot=skeleton]` / `.animate-pulse`), and the card was back 2.5 s later (`.playwright-mcp/qa-env-2.png`, local only).
- **The source picker keeps focus.**

  1. Open `/services/new?type=web_service` on the GitHub tab.
  2. Click "Search repositories…" and type `bex`.
  3. Wait 65 s.

  The input keeps focus and the repo list stays rendered. At filing time the `GitConnections` poll went out 30.0 s after page load. At 30.3 s the search input and every repo button were gone from the DOM and `document.activeElement` was `BODY`. After remounting, the input held `bex` again but did not have focus.

- **The control stays green.** On a service's Environment tab, click Edit → Add variable → Add variable, type a key, and wait 65 s. The draft row and its value survive. At filing time this passed: the draft was still present 50 s later, across polls at 28.5–30.8 s, because `ServiceEnvironmentEditor` already guards the flag (`service-environment-editor.tsx:121-124`). The fix must keep it green.

The env-group editor is not a DoD bullet because it was traced, not observed. t003 probes it live first and adds its bullet here.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixtures, all created and deleted inside the run:

- Private service `qa-20260914-priv` (`srv-dak3fq0gsm7s73f64jn0`) and web service `qa-20260914-probe` (`srv-dak3go0gsm7s73f64jqg`). Both are free, docker, from `bex-co/bex`.
- Project `qa-20260914-netp` (`prj-dak3hta6m8ac739r6300`), with environment `qa-20260914-iso` (`env-dak3hta6m8ac739r630g`) holding the private service.
- Env group `qa-20260914-egpoll` (`evg-dak3qdq6m8ac739r63eg`).
- Cleanup:
  - Services, environment and project were deleted at 18:25:41Z (`DELETE` → `204`, then `GET` → `404`).
  - The probe URL returned `404` at 18:25:49Z.
  - The env group was deleted in its own probe (`204`, then `404`).

1. **Settings dialog.** A first, untouched open stayed at 1 dialog for samples 0.5–15 s and was at 0 by 20 s. Then, with an edit:

   ```text
   18:16:47.189Z  open "All settings" on qa-20260914-iso; fill "New CIDR block" = 203.0.113.0/24
   +24.3 s  graphql: ViewerCapabilities, Workspaces, Projects, Environments, EnvGroups, Services, Databases, KeyValues
   +24.9 s  document.querySelectorAll('[role=dialog]').length → 0
            main text: "… Environments | qa-20260914-iso | New Service | New Environment" (no card, no table)
   ```

2. **Manage resources.**

   ```text
   18:18:56.932Z  open "Manage resources"; check "qa-20260914-probe" (not saved)
   +26.0 s  graphql Environments (poll)
   +26.9 s  dialog count → 0
            GET /v1/environments/env-dak3hta6m8ac739r630g → serviceIds ["srv-dak3fq0gsm7s73f64jn0"] (draft discarded)
   ```

3. **Skeleton during the poll.** The page was loaded fresh and `page.waitForRequest` waited for operation `Environments`, which fired at 29.7 s:

   ```text
   +150 ms  getByRole('button', { name: 'Manage resources' }).count() → 0; main skeleton nodes → 26
   +2.65 s  'Manage resources' count → 1
   ```

4. **Source picker.**

   ```text
   18:29:17.588Z  load /services/new?type=web_service; click "Search repositories…"; type "bex"
   +30.0 s  graphql: ViewerCapabilities, Workspaces, GitConnections, Projects, Environments
   +30.3 s  input[placeholder="Search repositories…"] absent; repo buttons 0; activeElement BODY
   +32.8 s  input back with value "bex"; activeElement still BODY
   ```

5. **Control: the service env-var editor.**

   ```text
   18:23:00.435Z  /services/srv-dak3go0gsm7s73f64jqg/env → Edit → Add variable → Add variable; Key = QA_DRAFT_PROBE
   +28.5…+30.8 s  graphql: ViewerCapabilities, Workspaces, Deploys, EnvGroups, Server
   +50 s   draft input still present; bar "1 variable operation · 0 file operations"
           Cancel; GET /v1/services/srv-dak3go0gsm7s73f64jqg/env-vars → 0 vars
   ```

6. **The env-group editor (t003, pre-fix, probed 2026-09-15).** Production still served the dashboard image pinned to `137a5186e`, which predates this milestone. Fixture: env group `qa-20260915-egpoll` (`evg-dakeflp5v75s738uf890`) with one variable `QA_K`. Sampled once a second.

   ```text
   06:42:45.656Z  /env-groups/evg-dakeflp5v75s738uf890 → Edit → Add variable (menu) → Add variable; Key = QA_POLL
   +0.8 s   draft present, focused; keys QA_K,QA_POLL; 0 skeleton nodes
   +26.4…+27.9 s  three POST /graphql (the EnvGroup poll tick)
   +27.0 s  activeElement BODY
   +28.0 s  no key inputs in main; 12 skeleton nodes
   +29.0 s  rows back, draft value QA_POLL kept; activeElement still BODY
   +56.4…+58.4 s  second poll tick; no further sampled change; Add variable / Save and deploy never disabled
   ```

   It reproduces: a poll unmounts the rows and drops focus, while the draft value survives because it lives above the rows. The disabled-controls half of the trace was not observed.

## Root cause

- **Apollo 4 reports every poll as loading.**
  - The dashboard pins `@apollo/client` 4.1.3 (`dashboard/package.json:30`).
  - 4.0 made `notifyOnNetworkStatusChange` default to `true` (`node_modules/@apollo/client/CHANGELOG.md:709`; the `watchQuery` default at `core/QueryManager.js:708`).
  - `ObservableQuery` sets `result.loading = isNetworkRequestInFlight(result.networkStatus)` (`core/ObservableQuery.js:1307`), which includes `NetworkStatus.poll` = 6 (`core/networkStatus.js:32`).
  - A `cache-and-network` or `cache-first` query with `pollInterval` therefore re-emits `loading: true` over its cached data on every tick.
- **Three consumers swap to a skeleton on that raw flag.**
  - **Environment card.** `environments-panel.tsx:194-215` renders `loading ? <ProjectEnvironmentCardSkeleton /> : error ? … : <EnvironmentCard …/>`.
    - `loading` comes from `useEnvironments` (`features/environments/hooks/use-environments.ts:123`, `loading: !resolved || loading`), which polls at `:114`.
    - `EnvironmentCard` owns the rename, delete, assign and settings open state, the selection and the move target (`environment-card.tsx:127-135`). It mounts `ConfirmDialog`, `ManageResourcesDialog` and `EnvironmentSettingsDialog` (`:424`, `:440`, `:448`).
    - The settings form's drafts live inside the dialog (`environment-settings-dialog.tsx:57-67`). An unmount resets all of it.
  - **Source picker.** `service-source-picker.tsx:159` renders `connectionLoading ? <Skeleton /> : …`, where `connectionLoading` is `useGitConnection().loading` (`:77-79`).
    - That value is returned raw from `features/git/hooks/use-git-connection.ts:82`, and the query polls at `:58`.
    - The search value lives in the picker (`:82`) and survives. Focus, the list's scroll and `GitCredentialsMenu`'s popover and confirm state do not; the menu was traced, not probed.
  - **Env-group editor (traced, not observed).** `routes/env-groups_.$groupId.tsx:209-214` passes `useEnvGroup`'s raw `loading` to `EnvGroupEditors`.
    - That flag is returned raw at `features/env-groups/hooks/use-env-groups.ts:142`, and the query polls at `:131`.
    - It flows through `EnvironmentEditor loading` (`env-group-editors.tsx:34`) into `EnvironmentSection`'s `loading ? <RowsSkeleton /> : rows` (`service-environment-editor.tsx:991`).
- **The safe sites guard it one by one.**
  - `use-env-groups.ts:109-112` documents the rule ("cache-and-network reports `loading` again while refreshing a cached result… reserve the page skeleton for the true first load") and returns `loading && data === undefined`.
  - `ServiceEnvironmentEditor` passes `env.loading && env.keys.length === 0` (`service-environment-editor.tsx:121-124`).
  - These hooks return guarded flags:
    - `use-deploy.ts:70`
    - `use-deploy-timeline.ts:76`
    - `use-deploys.ts:205`
    - `use-webhook-deliveries.ts:284`
    - `use-push-notification-settings.ts:86`
    - `use-disks.ts:81` and `:118`
  - There is no shared helper, and no hook reads `networkStatus`.

## Blast radius

- **Skeleton gates.** In non-test `dashboard/src` `.tsx`, 23 skeleton gates match `loading ? <…Skeleton…>` or `if (loading) return <…Skeleton…>`, and 43 files set `pollInterval`.
  - A pass-25 code trace of every gate found 2 affected (the environment card and the source picker) and 21 safe: the hook doesn't poll, or the flag is guarded.
  - The trace also found the env-group editor above, plus two partial sites:
    - `routes/env-groups.tsx:122` → `new-env-group-dialog.tsx:405` swaps the service checkbox grid for loading text on each `useServices` poll while the dialog is open. The selection survives.
    - `routes/env-groups_.$groupId.tsx:220` passes `servicesLoading` into `LinkedServicesCard busy`; the effect of that was not opened.
- **33 polled hooks still return raw `loading`**, including the three above; t004 holds the full list, from the trace. Their current consumers guard at the call site, so any new consumer repeats this bug.
- **`refetchQueries` has the same effect once per action.** Per the trace, autoscaling save and disable (`use-autoscaling.ts:55-57`, `:76-78`) briefly unmount `ManualScalingSection` behind `routes/services.$serviceId.scaling.tsx:73`. It is not on a timer.
- **The four explicit `notifyOnNetworkStatusChange: true` opt-ins must keep their behavior under any global change:** `use-resource-actions.ts:50`, `use-invite-redemption.ts:25`, `use-service-events.ts:93` and `use-webhook-event-types.ts:23`.

## Adjacent classes

- **First load with no cache** still shows the skeleton; that is the only time it should.
- **An error while data is cached.** `errorPolicy: "all"` keeps the data. `environments-panel.tsx:196` would still replace the card with the error body, so a transient poll error must not unmount stateful UI either. Show the error inline over the cached card.
- **Changed data under an open form.** `environment-settings-dialog.tsx:35-39` keys the form on the ACL, so a poll that returns a teammate's change remounts the form. Decide in t001 whether to keep that remount; either way the dialog must not close.
- **A deleted or missing resource on poll.** The URL canonicalization effect (`environments-panel.tsx:106-112`) must still run on settled data.
- **Empty lists.** List-length guards (`loading && list.length === 0`) still flicker the skeleton over a stateless empty state. The convention should prefer "no data yet" (`data === undefined`).
- **Hidden tabs.** `skipPollWhenHidden` is unaffected.

## Unverified (reasoned, not probed this run)

- **The env-group editor was traced only.** The live attempt couldn't find the new row's key input by placeholder `Key` after Edit → Add variable, and the group was deleted (t003).
- **Traced only:**
  - `GitCredentialsMenu` popover and disconnect-confirm dismissal on poll;
  - the picker on `/blueprints/new`;
  - `LinkedServicesCard busy`;
  - the `new-env-group-dialog` flicker;
  - the autoscaling `refetchQueries` unmount.
- **Mobile widths** were not probed.
- **The trace's line numbers need re-verifying in t004:** the 33-hook list, the per-site verdicts, the picker's consumer routes, and whether the Apollo client sets any `defaultOptions`.

## Implementation (2026-09-14)

**Decision (t004): option A, the client default.** Neither Apollo client set `defaultOptions` (`common/apollo/factory.client.ts`, `factory.server.ts`). Both now take `apolloDefaultOptions` (`common/apollo/default-options.ts`): `watchQuery.notifyOnNetworkStatusChange: false`. Read from `@apollo/client` 4.1.3 source:

- `ObservableQuery` (`core/ObservableQuery.js` ~151) drops an emission whose only change is the network status when the option is `false`.
- So a poll (`NetworkStatus.poll`) or `refetch()` over unchanged data emits nothing, and `loading` stays `false`.
- A first load, a variables change with no cached result, or data that actually changed still emits, so a skeleton still shows on first load and a workspace switch still loads.

Why A over B:

- It removes the cause for all 33 raw hooks at once.
- It also covers the `refetchQueries` unmount (autoscaling save and disable) and the two partial sites (`new-env-group-dialog`'s checkbox grid, `LinkedServicesCard busy`).
- It covers every future hook without a per-hook rule to remember.

What the default does **not** do, found in review (t006) and handled:

- **Lazy queries.** `useLazyQuery` learns of its own request only through the network-status emission, so under the default its `loading` never turned true. `generate-blueprint-dialog.tsx` would have lost its spinner and double-submit guard, and `use-validate-blueprint.ts` its disabled Validate button. Both now set `notifyOnNetworkStatusChange: true`, and every lazy query must.
- **A warm-cache mount.** A `cache-and-network` mount over a warm cache (or a switch to already-cached variables) still reports `loading` with data present. So the rule is "gate a skeleton on `loading && no data`", not "`loading` means no data", and hooks keep that guard.

The explicit `notifyOnNetworkStatusChange: true` opt-ins keep reporting refreshing. The rule is written in `dashboard/CLAUDE.md` § "Polling and loading".

**t001–t003.**

- The default stops polls and refetches from re-announcing `loading`. On top of it, `useEnvironments` (`!resolved || (loading && data === undefined)`), `useGitConnection`/`useGitConnections` and `useEnvGroup` (`loading && data === undefined`) now report first-load-only `loading`, as the tasks prescribed. That keeps the environment card, the source picker's GitHub tab and the env-group editor rows (and the controls it disables) mounted on a warm-cache mount too. `use-env-groups.ts`'s list-hook comment was corrected to match.
- t001 step 2: `environments-panel.tsx` renders the error body only when no environments are cached. A failed refresh over cached data shows "Couldn't refresh environments. Showing the last loaded data." inline (`role="alert"`, en + zh) above the still-mounted card.
- t001 step 4, the settings form's ACL key: kept. A poll that brings a teammate's changed ACL remounts the form so a stale draft cannot silently overwrite it. The dialog stays open, because the card no longer unmounts.

**Blast-radius counts (t004, re-run 2026-09-15 on main, `dashboard/src`, excluding `__tests__`).**

| Grep | Count | Verdict |
| --- | --- | --- |
| `pollInterval` | 43 files, 62 lines | Covered by the client default: a poll over unchanged data emits nothing |
| `notifyOnNetworkStatusChange: true` | 6 | 2 lazy queries (`generate-blueprint-dialog`, `use-validate-blueprint`), which must opt in; `use-resource-actions` guards `loading && data === undefined`; `use-invite-redemption`, `use-service-events` and `use-webhook-event-types` are not polled |
| `useLazyQuery(` | 2 | Both opt in |
| `loading ? <…Skeleton` in `.tsx` | 1 | `services.$serviceId.scaling.tsx:73` on `useAutoscaling`, which is not polled. Its `refetchQueries` after a save or disable is covered by the default |

No polled raw `loading` gates a skeleton.

**Parity (t005).**

- **Scope confirmed (2026-09-15).** The shipped fix, `f3cc6ee97`, touches only `dashboard/src/**`, `dashboard/CLAUDE.md` and `.pm/w1/m153/done/`, with no schema, REST or MCP change.
- **Render comparison: not performed.** It needs a signed-in Render dashboard session, and none was available to this run. Render's documentation does not describe background refresh behavior.
- **No drift filed.** What bex now does (open dialogs, typed input and focus survive background polls) is the baseline users expect of any dashboard. If a future Render session shows otherwise, file it as `w1/NNN`.

**Tests (t007).**

- `common/apollo/__tests__/default-options.test.tsx` uses a real `ApolloClient`, `InMemoryCache` and `MockLink`. The default is measured on a raw `useQuery` shaped like the polled hooks (`cache-and-network`, `pollInterval`), so a hook's own guard cannot mask it:
  - with the dashboard defaults, a `refetch()` over cached environments never reports `loading` and the data stays;
  - the real `useEnvironments` keeps its environments and reports no `loading` across the same refetch;
  - the first load still reports `loading`;
  - **control:** the same refetch without the defaults does report `loading`, which is the filed bug, so the harness is shown to detect it;
  - a lazy query reports `loading` when it opts in, and never without the opt-in (why every lazy query must opt in);
  - a query that opts in still reports refreshing.
- `environments-panel.test.tsx`: a failed refresh over cached environments keeps the card mounted with the error inline. It was shown failing on pre-fix `HEAD` in a scratch worktree ("Unable to find an element by: [data-testid=\"env-card\"]").
- The live DoD probes are recorded under § Live verification: bullets 1–5, t002's `GitCredentialsMenu` and `/blueprints/new` checks, and t003's editor after the fix. t003's pre-fix probe is § Evidence 6.
- **Added at closeout (2026-09-15).** t002 and t003 each required a vitest of their own, and `f3cc6ee97` shipped none. Each of the following runs the real hooks against a real client with `apolloDefaultOptions` and a gate link that holds a `refetchQueries` response in flight, and asserts both in flight and settled:
  - `features/services/components/__tests__/service-source-picker.test.tsx` (t002): the same "Search repositories…" input stays mounted and focused with its typed filter; the repo list and skeleton count are unchanged.
  - `routes/__tests__/env-groups-detail-refetch.test.tsx` (t003): the real `EnvGroupDetailPage` keeps the draft row, which stays focused and typed, keeps its key rows, adds no skeletons, and keeps Add variable and Save and deploy enabled.
  - **Pre-fix proof.** With `default-options.ts` stubbed to `{}` and `use-git-connection.ts` / `use-env-groups.ts` at `f3cc6ee97^`, the tests fail at the in-flight check: `Unable to find an accessible element with the role "textbox" and name "Search repositories…"`, and `… name "Value for API_TOKEN"`. Either layer alone keeps them green, so the fix is defended in depth.
  - **Suite.** Full `yarn test` 416 files and 3262 tests pass; `yarn typecheck` and `yarn lint` exit 0.

## Live verification (2026-09-15, production)

**Build.** The dashboard has been pinned to `c4212ec71` since 07:22:30Z. Its bundle contains `watchQuery:{notifyOnNetworkStatusChange:!1}`.

**Browser.** Headless Chrome (playwright-core 1.58.2, `channel: "chrome"`), signed in with the QA session and at 1440×900. The Playwright MCP browser was disconnected. Each check ran in its own page, and GraphQL operation names were recorded from request bodies. Run 1 started at 10:11:12Z; times below are seconds from page load.

**The environment card is never replaced by its skeleton (bullet 3).** On `/project/prj-dake9ih5v75s738uf7q0`, each `Environments` request was checked 150 ms after it was sent:

```text
+33.0 s  Manage resources count 1 · main skeleton nodes 0
+64.5 s  Manage resources count 1 · main skeleton nodes 0
```

**The source picker keeps focus (bullet 4).** "Search repositories…" was clicked, `bex` was typed, then 65 s passed:

```text
/services/new?type=web_service  GitConnections at +32.1, +35.0, +62.5, +65.2 s → activeElement is the search input, value "bex"; 76 repo buttons
/blueprints/new                 GitConnections at +32.5, +34.8, +62.9, +65.1 s → activeElement is the search input, value "bex"; 76 repo buttons
```

**`GitCredentialsMenu` (t002).** On `/services/new?type=web_service`:

- "Credentials (1)" was opened at +5.3 s. It stayed open, with its account row, through `GitConnections` at +32.7, +34.8, +62.9 and +65.1 s.
- Its disconnect button opened "Disconnect GitHub?" at +70.3 s. That stayed open through `GitConnections` at +93.2, +95.3, +123.4 and +125.5 s.
- The confirm was closed with Escape; its Disconnect action was never clicked.

**The environment settings dialog survives polls (bullet 1, run 3 at 10:17:33Z).**

```text
+2.9 s    More actions → All settings; "New CIDR block" = 203.0.113.0/24
+31.4 s   graphql Environments+EnvGroups+Services+Databases+KeyValues (poll)
+63.0 s   graphql Environments+… (poll)
+94.4 s   graphql Environments+… (poll)
+103.5 s  dialogs 1; "New CIDR block" still 203.0.113.0/24 → Escape (nothing saved)
```

**Manage resources survives polls (bullet 2, run 2 at 10:14:22Z).**

```text
+6.2 s   Manage resources; tick qa-20260915-m156b (was unticked; not saved)
+37.9 s  graphql Workspaces+Environments+… (poll)
+69.7 s  graphql Workspaces+Environments+… (poll)
+73 s    dialogs 1; qa-20260915-m156b still ticked → Escape
         GET /v1/environments/env-dake9ih5v75s738uf7qg → serviceIds ["srv-dakeca81e15c73bfgpq0"] (unchanged)
```

**The env-group editor keeps its draft row (t003, after the fix, run 2).** On `/env-groups/evg-dakeflp5v75s738uf890`, Edit → Add variable, sampled once a second:

```text
+39.2 s          Key = QA_POLL typed
+64.8 s, +95.1 s graphql Workspaces+EnvGroup+Services (poll ticks)
+39.2…+106 s     one unchanged sample throughout: draft present and focused · 2 key inputs · 0 skeleton nodes · Add variable enabled · Save and deploy enabled
                 → Cancel (nothing saved)
```

Before the fix (§ Evidence 6), the same poll unmounted the rows, left 12 skeleton nodes and dropped focus to `BODY`.

**Control: the service Environment editor (bullet 5, run 2).** On `/services/srv-dakeca81e15c73bfgpq0/env`:

- Edit → Add variable, with Key `QA_DRAFT_PROBE` typed at +8.4 s.
- Poll ticks followed at +34.4–35.7 s and +64.4–65.9 s (ViewerCapabilities, Workspaces, Server, EnvGroups, Deploys).
- The draft was still present after 65 s. The edit was then cancelled.

## Dedupe

- `grep -ril "notifyOnNetworkStatusChange|dialog closes|closes itself|poll.*skeleton|skeleton.*poll|unmount.*poll" .pm` hits only unrelated items:
  - `w9/done/m62`: visibility-gated polling;
  - `w9/done/m63`: metrics first-fetch shimmer;
  - `w5/done/m89`: no new polling loop;
  - `w1/done/066`: eager-refetch cadence;
  - `w4/done/m106`: custom-domain TXT after its dialog closes;
  - `w5/done/m91`: live metrics instance.
- No open `- [ ]` item in any workstream README mentions skeletons or polling, and `.pm/DO_NOT_DO.md` has no polling, skeleton or Apollo entry.
- Not fixed on `main` as of `8c7d689d5` (origin `79863ce0c`). The environment-card skeleton gate arrived with `458133694` ("align route skeletons with loaded pages"). `git log -S notifyOnNetworkStatusChange -- dashboard/src` shows only the four per-query opt-ins.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 25. It came up while toggling "Block cross-environment connections" during journey 10: private-service reachability and environment network isolation, both of which passed. Isolation took about 50 s to block the private service and about 30 s to restore it.
- **Goal linkage:**
  - The dashboard is the primary operating surface (`dashboard/CLAUDE.md` § Navigation pending states: a skeleton is a pending-state preview of the ready page, not something that replaces a ready page).
  - `common/lib/polling.ts:3-10` promises freshness "without a reload". That freshness must not cost users their in-progress work.
- **Expected outcome:** a user can take as long as they need in any environment dialog or the repo search. Polls refresh the data underneath without closing, resetting or unfocusing anything.
- **Why now:**
  - Every environment ACL change (protection, network isolation, IP allowlist) and every resource assignment goes through the two dialogs that close every 30 s. A user who reads the hint text, or types a CIDR slowly, loses the edit.
  - The first action of every GitHub-backed service or Blueprint create is the search box that drops focus.
  - The cause is in shared hooks, so settling the convention now stops new consumers repeating it.
- **Render parity:** included as a short check (t005). No REST/GraphQL/MCP shape changes; the change is dashboard behavior only, compared with Render's dashboard.
