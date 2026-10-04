# w4 · m163 — Make saved secret files available to native builds

**Worker:** worker4 **Goal:** Native build commands can read accepted service secret files without leaking them into platform-generated artifacts. **Status:** blocked — t001/t002/t003/t005/t006 done 2026-10-03; t004 hosted acceptance and t007 closeout remain

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Audit native secret-file inputs and release semantics | 30m | — |
| t002 — **DONE** | Project isolated file inputs with correct cache identity | 45m | t001 |
| t003 — **DONE** | Mount file inputs into the native build command | 45m | t002 |
| t004 | Render parity and hosted acceptance | 25m | t003 |
| t005 — **DONE** | Simplify | 15m | t004 |
| t006 — **DONE** | Test coverage | 45m | t004 |
| t007 | Closeout | 20m | t005, t006 |

## Definition of done

- Repeat [finding.md](finding.md)'s Free static fixture with the marker file and cp build command. It reaches Live and public GET returns exactly `qa-r59-build-file-marker`, including after fresh Settings reload and a second build.
- The Settings control command `printf 'qa-r59-control' > index.html` still reaches Live and returns `qa-r59-control`. Restoring the file command returns the file marker again.
- REST GET secret-files/qa-r59.txt, GraphQL secretFile and MCP get_secret_file retain the same saved marker. Dashboard reveal matches; storage success is accompanied by actual build access.
- Delete the owned fixture: API returns 404, exact App UID resources and build projections disappear, and the QA session is revoked.
- Additional shared-input/cache/ownership regression coverage in t001–t006 passes; record its new evidence separately from sweep59 observations. All tasks and hosted acceptance are complete before moving to done.

## Source + Goal linkage

- **Source:** user-requested continuous qa-find-bugs with muse.env, w4 filing; sweep59 on 2026-10-03. Complete repeatable wire evidence and source analysis are in [finding.md](finding.md).
- **Goal linkage:** ADR008 core hosting, ADR004 native builds, ADR013 secret configuration, ADR029 static deployment and ADR018 parity.
- **Expected outcome:** builds needing certificate/config files can consume the configuration already offered by the dashboard and APIs.
- **Why now:** two live failures bracket a successful control; the API retains the file but the builder drops the entire file-input class.
- **Sizing:** 225 minutes across seven tasks, including shared-source/cache/cleanup work.
- **Render parity included:** tenant-facing configuration semantics span REST, GraphQL, MCP and dashboard. Static upstream behavior was not authenticated; preserve that limitation.
