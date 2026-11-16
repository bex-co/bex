# w7 · m428 — Auto-deploy decides as Render documents: all six skip phrases, and build filters that reach outside the Root Directory

**Worker:** worker7 **Goal:** a git push deploys, or is ignored, by Render's documented rules: any of Render's six skip phrases skips it, and a build filter's included paths decide even when they lie outside the service's Root Directory. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | All six skip phrases skip an auto-deploy | 30m | — |
| t002 | Render's rule for a root directory and build filters together, as a decision table | 30m | — |
| t003 | Included paths decide when set, and may lie outside the Root Directory | 75m | t002 |
| t004 | Live check on production: the six phrases and a shared-directory change | 30m | t001, t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

On production, in `bex-canary`, with an owned free web service on a monorepo fixture, deleted afterwards (by-id `GET` 404):

1. A push whose head commit message contains any of `[skip render]`, `[render skip]`, `[skip deploy]`, `[deploy skip]`, `[skip cd]` or `[cd skip]` opens no deploy and records `commit_ignored`. The next ordinary push deploys.
2. With `rootDir: backend` and `buildFilter.paths: ["shared/**"]`, a push that changes only a file under `shared/` deploys the service.
3. An ignored path still wins over an included one, and a service with a root directory and no included paths behaves as today (only changes under the root directory deploy).
4. Every remaining cell follows t002's decision table. If t002's evidence contradicts the recommended reading below, t002 amends this definition before t003 starts.
5. ADR018's Build Filters row states the rule without "conscious divergence", and `docs/render-artifacts/auto-deploy-filters.md` holds the table with its inferred cells marked.
6. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

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
