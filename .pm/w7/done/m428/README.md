# w7 · m428 — Auto-deploy decides as Render documents: all six skip phrases, and build filters that reach outside the Root Directory

**Worker:** worker7 **Goal:** a git push deploys, or is ignored, by Render's documented rules: any of Render's six skip phrases skips it, and a build filter's included paths decide even when they lie outside the service's Root Directory. **Status:** done (2026-11-17: code shipped in `e9fbffd2e`; production walk passed on pin `db48cde2bae9`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | All six skip phrases skip an auto-deploy — **DONE** | 30m | — |
| t002 | Render's rule for a root directory and build filters together, as a decision table — **DONE** | 30m | — |
| t003 | Included paths decide when set, and may lie outside the Root Directory — **DONE** | 75m | t002 |
| t004 | Live check on production: the six phrases and a shared-directory change — **DONE** | 30m | t001, t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

On production, in `bex-canary`, with an owned free web service on a monorepo fixture, deleted afterwards (by-id `GET` 404):

1. A push whose head commit message contains any of `[skip render]`, `[render skip]`, `[skip deploy]`, `[deploy skip]`, `[skip cd]` or `[cd skip]` opens no deploy and records `commit_ignored`. The next ordinary push deploys.
2. With `rootDir: backend` and `buildFilter.paths: ["shared/**"]`, a push that changes only a file under `shared/` deploys the service.
3. An ignored path still wins over an included one, and a service with a root directory and no included paths behaves as today (only changes under the root directory deploy).
4. Every remaining cell follows t002's decision table. If t002's evidence contradicts the recommended reading below, t002 amends this definition before t003 starts.
5. ADR018's Build Filters row states the rule without "conscious divergence", and `docs/render-artifacts/auto-deploy-filters.md` holds the table with its inferred cells marked.
6. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

**Amendment by t002 (2026-11-16).** Render's pages confirm the recommended reading below, and read per file: P1 (monorepo-support, previews) calls a file outside the Root Directory "ignored", like one matching an ignored path. That corrects one claim of this README: not every push that deploys today still deploys. With a root directory, ignored paths and **no** included paths, a push whose files under the root directory are all ignored, and whose other files lie outside it, deployed under the old AND rule and does not under Render's (`build_filter`). Item 3's "behaves as today" therefore holds for every push but that one cell, which is the table's inferred cell C in `docs/render-artifacts/auto-deploy-filters.md`. t003's widening property is stated with that single exception, and production had no App in the cell (t003's count).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 3; filed by `/pmx them all to w7` the same day.
- **Render reference (skip phrases):** render.com/docs/deploys § Skipping an auto-deploy, read 2026-11-16: "The skip phrase is one of `[skip render]` or `[render skip]`. You can also replace `render` with one of the following:" `deploy`, `cd`. That makes six phrases.
- **Render reference (filters):** render.com/docs/monorepo-support, read 2026-11-16: "Build filter paths are always relative to your repository root, even if you've set a different root directory. This means your build filters can include paths from other directories in your repo." Also: "If you specify at least one included path, all non-matching paths are automatically ignored", and "ignoring a path takes precedence over including it."
- **bex today:** `commitHasSkipPhrase` knows two phrases (`lego/backend/internal/apps/webhook.go:1124`). The push decision requires both `rootDirMatches` and `buildFilterMatches` (`webhook.go:1046` and `:1081-1086`), so an included path outside the root directory can never deploy. ADR018 § Services & lifecycle, Build Filters row, calls this a "conscious divergence … a deliberate narrowing" (w1/m34) with no user sign-off.
- **Goal linkage:** ADR008 Render parity first; ADR026 (push-to-deploy). Monorepos are what build filters exist for.
- **Expected outcome:** a monorepo configuration moved from Render deploys on the same pushes on bex, including a change to a shared package, and a commit that says `[skip deploy]` or `[skip cd]` is skipped as it is on Render.
- **Why now:** both rules live in one function, the change is small and independent, and a shared-package change that silently never deploys is hard for an owner to diagnose. The filter divergence has no sign-off.
- **Decision state:** the recommended reading of Render's page is: when included paths are set they decide, wherever they point; when none are set the root directory decides; ignored paths always win. It satisfies every sentence quoted above, but Render never states the combined rule, so it is this proposal's inference resting on ADR008 Render parity first, not a user decision and not a live capture. A capture needs a Render account (`w10/blocked/002`). Under this reading every push that deploys today still deploys.
- **Render parity:** included: the change alters tenant-visible deploy behavior and the `commit_ignored` events read on REST, GraphQL, MCP and the dashboard.

## Record

- **Production count (t003 step 5, 2026-11-16T22:52Z, `kubectl get apps.app.bex.co -A` on the production app cluster).** 26 Git-backed Apps; 14 set a root directory; 6 set included paths, and all 6 also set a root directory. Each of the 6 includes only its own root (`<rootDir>/**`), so the new rule decides every push for them exactly as the old one did. 0 Apps set ignored paths, so none sits in the narrowed cell C. The kubeconfig was fetched through ADR019's recovery flow, as w7/m427 did: this machine's host-key pin was stale, `Verify production SSH trust` run 43877180814 reported the pinned ED25519 fingerprint, all three running control planes matched it on a fresh keyscan, and the fetch used a temporary pin built from those keys; `.env`, `~/.ssh/known_hosts` and the pin files were not changed.
- **Parked 2026-11-16 (release gate, clears by itself).** The code shipped in `e9fbffd2e`. t004 starts once `git merge-base --is-ancestor e9fbffd2e <newest "pin platform images to" sha on main>` succeeds; then t005 and t008. The fixture is a temporary bex-co repository with `backend/` and `shared/`, as w1/m181 t006 used (`gh repo create`; deleting it needs the signed-in browser, since `gh` lacks `delete_repo`).

## Unblock record (2026-11-17, `/pm unblock w7/m428`, verified evidence)

- **Affected:** t004, t005, t008. **Source:** verified evidence, not a user decision.
- **Gate cleared:** the newest production pin on `origin/main` is `fc4fbf28a` (`chore(deploy): pin platform images to db48cde2bae9`, 2026-11-17 12:08Z), and `git merge-base --is-ancestor e9fbffd2e db48cde2bae9` succeeds. Production serves it: both `bex-api` pods run `ghcr.io/bex-co/bex-operator@sha256:ced89914…`, the digest that pin wrote, and started at 12:12Z.
- **Fixture:** the `bex-canary` workspace has no Git connection (`GET /v1/git/connections` answers `[]`), so a push could never reach a service there. The walk ran in workspace `bex` (`tea-d98210cbbpdc73dcrkvg`), which holds the bex-co installation `90623475`, as w1/m181 t006 did. `GH_TOKEN` carries `delete_repo`, so the fixture repository is deleted with `gh`, not the browser.
- **Next:** t004's walk, t005, then t008.

## Production walk (t004, 2026-11-17, pin `db48cde2bae9`): PASS

**Fixture.** A temporary private repository `bex-co/w7-m428-autodeploy-probe` with a root `Dockerfile`, `backend/Dockerfile` (both a busybox `httpd` on port 10000), `backend/`, `shared/` and `docs/`. A Free Docker web service `qa-20261117-m428-filters` (`srv-dbu7geeggh5s73ememrg`) in workspace `bex`, created at 15:52:25Z with `autoDeploy: yes` (`autoDeployTrigger: commit`), no Root Directory and no build filter. Its first deploy `dep-dbu7geeggh5s73emems0` (`8775708`) went live at 15:53:24Z. Each push was one commit; "no deploy" means the deploy list did not change and the reason is the event's `reasonCode`. Each settings `PATCH` answered 200, recorded `root_directory_changed`/`build_filter_changed` and opened no deploy.

| # | Settings | Push (commit, UTC) | Changed files | Result |
| --- | --- | --- | --- | --- |
| 1 | none | `fa09351f2` 15:54:00, `[skip render]` | `index.html` | no deploy, `commit_ignored` `skip_phrase` |
| 2 | none | `3adf2da1e` 15:54:14, `[render skip]` | `index.html` | no deploy, `skip_phrase` |
| 3 | none | `340f12013` 15:54:28, `[skip deploy]` | `index.html` | no deploy, `skip_phrase` |
| 4 | none | `c919bf3e4` 15:54:42, `[deploy skip]` | `index.html` | no deploy, `skip_phrase` |
| 5 | none | `0b37d7f5d` 15:54:56, `[skip cd]` | `index.html` | no deploy, `skip_phrase` |
| 6 | none | `d9de76f76` 15:55:10, `[cd skip]` | `index.html` | no deploy, `skip_phrase` |
| 7 | none | `353013d97` 15:55:46, ordinary | `index.html` | **deployed** `dep-dbu7i0uggh5s73ememt0` (`new_commit`), live 15:56:32 |
| 8 | `rootDir: backend`, `paths: [shared/**]` (15:56:58) | `27153e7a2` 15:57:13 | `shared/lib.txt` | **deployed** `dep-dbu7imuggh5s73ememtg` (`new_commit`), live 15:58:00. Table row 6, cell B: the old AND rule never deployed this |
| 9 | + `ignoredPaths: [shared/*.txt]` (15:58:06) | `6aed86602` 15:58:12 | `shared/lib.txt` | no deploy, `build_filter`: ignored wins (row 6) |
| 10 | same | `68fe8ce1c` 15:58:45 | `backend/index.html`, `docs/notes.txt` | no deploy, `build_filter` (row 6, mixed cell A + outside, not included) |
| 11 | same | `9556ea802` 15:58:56 | `backend/index.html` | no deploy, `build_filter` (row 6, cell A) |
| 12 | same | `6f356effe` 15:59:04 | `docs/notes.txt` | no deploy, `build_filter` (row 6, outside root, not included) |
| 13 | filter cleared, `rootDir: backend` (15:59:12) | `df98f9a02` 15:59:22 | `shared/lib.txt`, `docs/notes.txt` | no deploy, `root_directory` (row 4) |
| 14 | same | `4f83efc3d` 15:59:31 | `backend/index.html` | **deployed** `dep-dbu7jpeggh5s73ememv0`, live 16:00:18 (row 4) |
| 15 | `ignoredPaths: [backend/*.md]`, no included paths (16:00:26) | `075dbe755` 16:00:32 | `backend/README.md` | no deploy, `build_filter` (row 5, ignored under root) |
| 16 | same | `68724e19a` 16:00:41 | `backend/README.md`, `docs/notes.txt` | no deploy, `build_filter` (row 5, cell C: the one narrowed cell) |
| 17 | same | `3fb706beb` 16:00:50 | `backend/index.html` | **deployed** `dep-dbu7kdeggh5s73emen00`, live 16:01:44; `https://qa-20261117-m428-filters.onbex.co/` answered `200 backend v4` |

- **Definition of done, items 1–4:** item 1 is rows 1–7, item 2 is row 8, item 3 is rows 9 and 13–14, and item 4 is rows 8–17. Together they cover table rows 4, 5 and 6, including cells A, B and C. Rows 1–3 have no Root Directory: row 7 is row 1, and the unit table covers rows 2 and 3.
- **Event reads.** REST, GraphQL, MCP and the dashboard all read the same 13 `commit_ignored` events (6 `skip_phrase`, 6 `build_filter`, 1 `root_directory`): REST `GET /v1/services/{id}/events` and MCP `list_events` as `details {reasonCode, id, url}`, GraphQL `serviceEvents` as `details {reasonCode, commitId, commitUrl}`, and the Events tab as "Commit ignored" with the reason sentence and a commit link (t005).
- **Cleanup.** At 16:04:12Z `DELETE /v1/services/srv-dbu7geeggh5s73ememrg` answered 204. By 16:04:17Z the by-id `GET` answered `404 not_found`, and the App CR was gone. `gh repo delete` removed the repository (`gh api repos/bex-co/w7-m428-autodeploy-probe` answers 404). The QA session was logged out (`qa-login.sh --logout`, then `GET /v1/owners` answered 401), and its jar, state file, local clone and kubeconfig were deleted.
- **Production access.** The local host-key pin was stale again. A fresh `Verify production SSH trust` run (44119356513) reported the pinned ED25519 fingerprint `SHA256:W/o3ED3/xJrcoF0ONPn3Cb1S98X32ubnDcTeYtSVRyU`. A fresh keyscan of all three running control planes matched it, and the kubeconfig was fetched through a temporary pin built from those keys. `.env`, `~/.ssh/known_hosts` and the pin files were not changed.
