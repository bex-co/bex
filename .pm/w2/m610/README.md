# w2 · m610 — Git-push auto-builds and `buildFilter` for git-backed Build Sources

**Worker:** worker2 **Goal:** Route signed GitHub/Gitea push webhooks to git-backed Build Source carriers (`bds-…`) so pushes to the tracked branch honor `buildFilter` (`paths`/`ignoredPaths`) and commit skip phrases, trigger a carrier build, and fan out to linked services with `autoDeploy` enabled. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit git-carrier webhook fields (`AutoDeploy`, `GitHubRepoID` on create/update, `buildFilter`, skip phrases, `branch_deleted`) and document exact carrier semantics in `ADR097` | 30m | — |
| t002 | Route signed GitHub/Gitea push webhooks to git-backed Build Source carriers in `webhook.go`, honoring branch match, `buildFilter` (`paths`/`ignoredPaths`), and commit skip phrases | 1h | t001 |
| t003 | Trigger carrier builds via `redeployFetched` with empty `build_deploy_scope` so `deploys.BuildSucceeded` fans out only to linked services with `autoDeploy: true` (and skips suspended services) | 45m | t002 |
| t004 | Guard carrier side effects on push: prevent service-only `commit_ignored`/`branch_deleted` service-event leakage, ignore `checksPass` on carriers, and record clean audit/deploy provenance | 35m | t003 |
| t005 | Add unit and real-Postgres integration tests covering push match, `buildFilter` include/ignore, skip phrases, multi-service fan-out (`autoDeploy` on/off + suspended), and image-backed Build Sources ignoring pushes | 40m | t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- Pushing a signed commit to a git-backed Build Source's connected repository and tracked branch creates a new carrier build (`bld-…`) automatically without manual triggering.
- `buildFilter` (`paths` and `ignoredPaths`) and Render skip phrases (`[skip render]`, `[render skip]`, etc.) gate carrier builds identically to git-backed services.
- When the carrier build succeeds, `deploys.BuildSucceeded` fans out one deploy pinned to that `bld-…` to every linked non-suspended service with `autoDeploy: true`, and skips services with `autoDeploy: false` or suspended.
- Image-backed Build Sources ignore git push webhooks; carrier pushes never leak service-only `commit_ignored`/`branch_deleted` facts into unrelated service feeds, and `TestEveryAppEnumerationDeclaresBuildSources` stays green with the webhook allowlist/reason documented.
- `cd lego/backend && go test ./...` (including real Postgres + OpenFGA) and `make lint` pass, and `docs/ADR097-build-sources.md` + `docs/ADR018/services-lifecycle.md` reflect the shipped behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w2` (2026-11-23), materialized via `/pmx w2`. Root cause verified at `0f9166bc7f`: `lego/backend/internal/apps/webhook.go:977-978` still filters `list.Items = core.ServiceApps(list.Items)` with placeholder comment `// A push does not reach build sources yet (w2/m254 t006 decides).`
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) and [ADR097](../../../docs/ADR097-build-sources.md) — git-backed Build Sources must rebuild automatically on `git push` and honor `buildFilter`, matching render.com.
- **Expected outcome:** Teams linking multiple services to a monorepo Build Source get automatic rebuild-and-fan-out on push instead of being stuck on manual triggers or Blueprint syncs.
- **Why now:** Carrier builds, `BuildSucceeded` fan-out (`w2/m254`), and `buildFilter` storage are already live; only the webhook filter gate blocks automatic git-push builds.
- **Render parity:** Included — changes user-facing git-push auto-build, `buildFilter`, and fan-out deploy behavior.
- **Brainstorm recommendations, not user decisions:** Under ADR008 Render parity first, git-backed Build Source carriers auto-build on every matching push (default `AutoDeploy: true` on the carrier) while each linked service's own `autoDeploy` flag controls whether it rolls out from the resulting build; carriers do not expose `checksPass`.
