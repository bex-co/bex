# w1 · m18 — Root Directory: CRD + build engine + webhook path-filter + API surface

**Worker:** worker1 **Goal:** monorepo support — `App.spec.rootDir` makes the operator build from a subdirectory instead of the repo root, and scopes the git-push auto-deploy webhook to only redeploy when the pushed diff touches that subdirectory, mirroring Render's Root Directory setting. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                     | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Add `RootDir` to `AppSpec` (`lego/types/v1alpha1/app_types.go`); `make manifests generate` from `lego/operator/`                            | 30m | —            |
| t002 | Extend `build.Options`/`gitContext()` (`lego/operator/internal/build/build.go`) to append BuildKit's `:<subdir>` git-context suffix when set; extend the existing `gitContext` unit tests | 45m | t001         |
| t003 | Wire `a.Spec.RootDir` into `build.Options` at the controller call site (`lego/operator/internal/controller/app_controller.go`)               | 30m | t002         |
| t004 | Webhook path-scoped auto-deploy: extend `pushEvent` (`lego/backend/internal/apps/webhook.go`) to capture per-commit changed-file paths from the GitHub/Gitea push payload; add a path-prefix filter in `redeployMatching` gated on `spec.rootDir` (empty = today's always-redeploy behavior) | 1h  | t001         |
| t005 | Thread `rootDir` through REST/GraphQL/MCP create+update request/response shapes and CR mapping (`rest.go`, `graphql.go`, `mcp.go`, `service.go`) | 45m | t001         |
| t006 | Update `docs/deployment.md`'s existing gotcha (the "no `spec.rootDir` field yet" line) to describe the shipped behavior and its limits (Dockerfile builder only — buildpack/CNB still not in-cluster) | 20m | t002, t004   |
| t007 | Render parity: compare against Render's Root Directory semantics (render.com/docs/monorepo-support#setting-a-root-directory) across REST/GraphQL/MCP/UI, flag drift as follow-up                | 30m | t005, t006   |
| t008 | Simplify: `/simplify` over the code this milestone changed                                                                                    | 30m | t007         |
| t009 | Test coverage: meaningful tests for subdir build-context construction, webhook path-filter matching (in-root vs. out-of-root push), and the API surface field                                    | 45m | t007         |
| t010 | Closeout: verify DoD, mark done, move milestone to `done/`                                                                                     | 15m | t009         |

## Definition of done

An App with `spec.rootDir` set to a subdirectory builds successfully from a monorepo (BuildKit builds only that subdirectory's Dockerfile), verified against a real monorepo test repo; a push whose diff touches files outside `rootDir` does not trigger an auto-deploy while a push touching files inside it does; `rootDir` is settable and readable identically via REST, GraphQL, and MCP.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` (Root Directory topic) 2026-07-09, from Render's monorepo-support docs (https://render.com/docs/monorepo-support#setting-a-root-directory); closes a gap already flagged at `docs/deployment.md:157` ("Build-from-git needs a root Dockerfile today ... no `spec.rootDir` field yet").
- **Goal linkage:** Render-parity on the REST/GraphQL/MCP surface (`docs/bex-api.md`); natural extension of `GOAL.md` #3 (git push to deploy) and build-from-git (`w1/m5`, done).
- **Expected outcome:** monorepo users point bex at a subdirectory the way they do on Render; unrelated pushes elsewhere in the repo stop causing wasted rebuilds/redeploys.
- **Why now:** `w1/m5` (build-from-git) and `w2/m2` (webhook) are both done and stable in prod — this is the next incremental parity gap on that surface rather than new architecture, and it's already a named known limitation, not invented scope.
- **Render parity: included** (t007) — this milestone touches REST/GraphQL/MCP directly (t005); the dashboard UI half is a separate follow-on milestone (`w5/m13`) since the dashboard has no build/deploy settings section to extend yet.
