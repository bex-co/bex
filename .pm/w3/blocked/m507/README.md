# w3 · m507 — Project views can be created, edited, duplicated and deleted, and are shared by the whole project, as on Render

**Worker:** worker3 **Goal:** a project's members manage its observability views as Render's do: New View by type, Edit view (name and filter rows), Duplicate and Delete, every change visible to everyone on the project, recorded in the audit log under Render's event names. **Status:** blocked — t001–t005 and t007–t010 done and shipped in `bb151b367`; t006 is verified on dev-3, and its production walk waits for a production pin carrying `bb151b367` (§ Gate)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | GraphQL create, update, duplicate and delete for saved searches, audited under Render's event names — **DONE** | 1h30m | w3/m505/t003 |
| [t002](done/t002.md) | New View: pick a type, name it, save it — **DONE** | 1h30m | t001, w3/m505/t006 |
| [t003](done/t003.md) | The Edit view modal: name and filter rows — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | Duplicate and Delete, defaults included — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Role-aware controls — **DONE** | 30m | t004 |
| [t006](t006.md) | Verify on dev-3 and on production | 30m | t005 |
| [t007](done/t007.md) | ADR032 and ADR018 — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 30m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

## Definition of done

- A project editor creates a view by type, edits its name and filters, duplicates it and deletes it (default views included); every change is what every other member of the project sees.
- A viewer sees the controls disabled with a reason, and the API refuses them.
- A duplicate name answers `duplicate_saved_search_name`; the audit log records `CreateSavedSearchEvent` and `DeleteSavedSearchEvent`.
- Verified with two members on dev-3 and on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/projects](https://render.com/docs/projects) § Observability and the capture: views are named, saved filter sets shared by the project ("Editing filters will update the view for everyone on the project.").
- **Recommendation, not a user decision:** write access follows the project's edit permission, and New View creates on Done; the capture could not show either, so ADR032 records them as bex's choices to revisit if a capture shows otherwise.
- **Expected outcome:** teams keep the filtered views they use (one service's errors, one environment's requests) instead of re-filtering every visit.
- **Why now:** w3/m505 ships the store and the defaults; without this the defaults are fixed and the sidebar's New View has nothing behind it.
- **Render parity included:** new GraphQL mutations, audit events and dashboard controls.

## Verification record

### dev-3 (2026-11-19, two members and a Viewer, enforced authorization)

dev-3's bex-api runs fail-open by default, which would make every member an Admin, so it was restarted from this tree against an isolated OpenFGA seeded with `deploy/gitops/authz/model.json` (`DEV_OPENFGA_URL`). Three fresh Kratos identities: an owner, who moved their workspace `tea-dbvja49jg4r5sbdh1j8g` to Scale (Viewer is a Scale role) and invited the other two through the emailed invites (Mailpit), a **Developer** and a **Viewer**. Project `prj-dbvjaehjg4r5sbdh1jh0` ("observed") with environments production and staging, a web service `storefront` (production) and a cron job `nightly-report` (staging).

API, as the Developer, each change read back by the Viewer:

- `createSavedSearch(APPLICATION_LOGS, "Storefront errors", ENVIRONMENT IS production + LEVEL IS error)` answered the view (`svs-dbvjah1jg4r5sbdh1jn0`); the Viewer's `savedSearches` listed it after Render's three defaults.
- `updateSavedSearch` renamed it "Production errors" with `ENVIRONMENT IS production` + `RESOURCE_TYPE IS_NOT [POSTGRES, KEY_VALUE]`; the Viewer's `savedSearch(id)` read the new name and document.
- `duplicateSavedSearch` answered "Production errors (copy)", and of the default Request Logs view "Request Logs (copy)". Deleting the default Request Logs view and the first copy left the Viewer reading Application Logs, Service Metrics, Production errors and Request Logs (copy).
- A taken name answered `duplicate_saved_search_name` (`extensions.name`); `WORKFLOW_RUNS` was refused by the schema; `updateSavedSearch(type: REQUEST_LOGS)` answered "a view's type cannot change".
- The Viewer's create, update, duplicate and delete each answered `FORBIDDEN` and changed nothing.
- The owner's audit log (REST `GET /v1/owners/{id}/audit-logs`) held `CreateSavedSearchEvent` success ×3 by the Developer (the create and both duplicates, `metadata.savedSearch` and `targetName` naming each), `DeleteSavedSearchEvent` success ×2, and the Viewer's refusals as `CreateSavedSearchEvent`/`DeleteSavedSearchEvent`/`projects.UpdateSavedSearch` with status `error` and `relation: can_create`. The Developer's edit recorded no row.

Dashboard (a local dashboard against dev-3, signed in through `qa-login.sh`):

- **Developer, 1440:** the sidebar's Observability group listed the views and **+ New View**; its menu read TYPE, Application Logs, Request Logs. Request Logs opened "New view" named "Request Logs" (the default of that name had been deleted). Adding Environment = production and Resource = storefront (the picker listed only the web service for Request Logs) and Done created the view and opened `/observability/logs/request?savedSearchId=svs-dbvjbjhjg4r5sbdh1jv0`, listed in the sidebar. **Edit view** showed both rows, with Environment's operator fixed at "is"; renaming to "Storefront requests" and adding Status Code `404` saved, the heading changed at once, and the Viewer's API read returned the new document. Edit view, a typed new name, then **Close** left the heading unchanged. **Duplicate** opened "Storefront requests (copy)"; its trash button asked for `sudo delete view Storefront requests (copy)`, deleted it, and landed on the project's first log view, Application Logs.
- **Viewer, 1440:** Edit view, Duplicate and Delete view were disabled, and New View was `aria-disabled`; focusing it showed "Your role can't make this change. Ask a workspace admin.", and clicking it opened no menu.
- **Pending and ready, 1440 and 390** (the saved-search read held back by a route delay): the header, its eyebrow, and the three buttons have the same boxes in both states at both widths (1440: buttons at x 1158/1269/1384, y 92; 390: wrapped under the title at y 128). The Edit view modal fits 390 with no horizontal scroll (rows wrap Field/Operator, then Values/trash; Done and Close stack).
- **Found:** a Viewer has no log access (`can_view_logs` is Contributor and up), so each project view reads an empty scope for them and says the project has nothing in it. Not this milestone's change; filed as [w3/078](../../078.md).

Cleanup: the fixture project was deleted (by-id `GET` of the project and both services answer 404; `savedSearch` of its view answers not found), both browser sessions were revoked, the three identities were deleted through Kratos' admin API, the isolated OpenFGA container was removed, and dev-3's bex-api was restarted with its default settings.

## Render parity (t008)

Compared 2026-11-19 against the capture ([project-observability.md](../../../../docs/render-artifacts/project-observability.md)), [render.com/docs/projects](https://render.com/docs/projects) § Observability, and Render's pinned public OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`: the audit enum and error codes). Render has no public REST or MCP operation for saved searches, so neither surface gained one, and the GraphQL mutations are bex's own (Render's private mutations were not captured).

| Surface | Render | bex | Verdict |
| --- | --- | --- | --- |
| Create | New View → TYPE menu (Application Logs, Request Logs, Workflow Runs) | `createSavedSearch(projectId, type, name, filters)`; the menu offers Application Logs and Request Logs | matched; Workflow Runs is an ADR018 non-goal, Service Metrics joins with w3/m508 |
| What New View opens | not captured | the Edit view modal titled "New view" with the type's name, created on Done | bex's choice, ADR032 |
| Edit | Edit view: Name, Filters rows (Field, Operator, Values, trash), add, the shared notice, Close, Done | the same parts and notice; `updateSavedSearch(id, name, filters)` | matched; Instance, Text, Status Code, Host and Path values are typed where Render's are dropdowns: filed as [w3/079](../../079.md) |
| Operators | `is` for Environment; `is one of` / `is not one of` for Resource Type | the same; Resource also takes both; line fields `is` only | `IS_NOT` on line fields waits on w3/076 (already filed) |
| Duplicate | header button on every view | `duplicateSavedSearch(id)`, copy named `<name> (copy)`, opened | copy name unverified (not captured): bex's choice |
| Delete | trash button on every view, defaults too | `deleteSavedSearch(id)` behind `sudo delete view <name>`; lands on the first remaining log view | confirmation is the dashboard's destructive rule; Render's prompt was not captured |
| Shared by the project | "Editing filters will update the view for everyone on the project." | every write is the project's; another member reads it | matched |
| Duplicate name | `duplicate_saved_search_name` (error code enum) | the same code on create and edit | matched |
| Audit | `CreateSavedSearchEvent`, `DeleteSavedSearchEvent` (enum); no update event | the same two, create and duplicate → create; an edit records none | matched |
| Who may write | not captured | the project's edit permission (`can_create`); Viewers and Contributors disabled with the reason, refused by the API | bex's choice, ADR032 |
| View cap | none seen | 100 a project (`SAVED_SEARCH_LIMIT`) | bex bound, ADR032 |

Within bex the surfaces agree: GraphQL is the only write surface, the dashboard calls it, the audit log's REST and GraphQL reads answer the same event names, and the Viewer's refusals read the same on GraphQL and in the dashboard. ADR018's Projects row records the shipped behavior and the choices above.

## Simplify (t009)

`/simplify` ran three reviews (reuse, quality, efficiency) over the diff. Applied: one `authorizedSavedSearch` lookup for every by-id view verb and one type check; the project and view name rule (`requiredName`) shared with project create and rename; `core.MaxLabelLength` exported for the copy name; Duplicate hands the stored document on parsed, not re-encoded; the store returns written rows (`RETURNING`) instead of reading them again; the new operation keys sit in their sorted places. Dashboard: the mutation hook uses `useMutation` with cache updates and derives its busy state; the delete steps the page aside only once bex-api has deleted the view, and clears it after the next view opens; the header buttons take the role reason themselves; one `isProjectLogView` guard, type list and title table (`project-log-views.ts`); the modal reads the workspace lists once without polling (`poll` option on `useDatabases`/`useKeyValues`), uses `RemovableChip` for typed values, the service-type helpers and one stale-value helper, and computes a new row inside the state update; the Method list is the Logs tab's (`LOG_METHODS`). Not taken: a shared checkbox multi-select with the metrics card (a refactor of that card), keying the page's scope on the filters alone (the React Compiler lint refuses the memo; a rename reconnects the live tail once), a project-scoped GraphQL read of the view's resources (a new API surface), and `trackOwnDelete` (the view page has no not-found redirect to hold back).

## Test coverage (t010)

| Definition of done | Tests |
| --- | --- |
| An editor creates by type, edits name and filters, duplicates and deletes, defaults included; every member reads the change | `projects/savedsearches_test.go` `TestProjectEditorManagesViews`, `TestSavedSearchMutationsOverGraphQL`; `store/saved_searches_pg_test.go` `TestPGSavedSearchWrites` (real Postgres: fresh id, uniqueness per project, defaults edited and deleted, the bound); dashboard `edit-view-dialog.test.tsx`, `observability-view-actions.test.tsx`, `use-saved-search-mutations.test.tsx`, `project-sidebar.test.tsx` (New View), `project-log-view-page.test.tsx`, `view-filters.test.ts` |
| A viewer sees the controls disabled with a reason, and the API refuses them | `TestViewWritesNeedTheProjectsEditPermission`; `observability-view-actions.test.tsx` and `project-sidebar.test.tsx` (the Viewer cases); `api/server_test.go`'s authz and audit sweeps over the four new verbs |
| `duplicate_saved_search_name`; `CreateSavedSearchEvent` and `DeleteSavedSearchEvent` | `TestViewWritesRefuseByName`, `TestViewWritesRecordRendersEvents`, `audit/service_test.go` `TestRESTAnswersRendersSavedSearchEvents` |

Mutation checks, each reverted: mapping `projects.CreateSavedSearch` to another verb failed `TestRESTAnswersRendersSavedSearchEvents` and `TestRenderEventVerbsExist`; authorizing create and delete with `can_view` failed `TestViewWritesNeedTheProjectsEditPermission`; dropping the role gate from the header buttons and from New View failed their Viewer tests.

## Gate (2026-11-19)

**Clears by itself; no decision needed.** t006's production half needs a production deploy carrying `bb151b367`. `deploy.yml` deploys a push only when the production pin is at least 6 hours old, and the newest pin (`d38508f70 chore(deploy): pin platform images to 1c63b6fe0a34`) landed at 19:49Z, so the next deploy runs after about 01:49Z on 2026-11-20. Check: the newest `chore(deploy): pin platform images to <sha>` on `main` passes `git merge-base --is-ancestor bb151b367 <sha>`, and production's `bex-api` runs that digest.

**Next (t006 step 2, then t011):** sign in as the QA user (`bash scripts/qa-login.sh`) and, in `bex-canary`, use or create a project of the QA user's own. Create a view with New View, edit its name and filters, duplicate it and delete the copy and one default view, at 1440 and 390, and read each change through GraphQL `savedSearches(projectId)` in a second session. Read `CreateSavedSearchEvent` and `DeleteSavedSearchEvent` in the workspace audit log. If a Viewer session is available, confirm its controls are disabled with the reason and its writes answer `FORBIDDEN`; otherwise record that the Viewer case rests on dev-3. Record the evidence here, delete the fixture with by-id 404 evidence, log out, and close the milestone.
