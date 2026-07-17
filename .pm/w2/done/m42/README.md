# w2 · m42 — Deploy commit author timestamp (`commit.createdAt`)

**Worker:** worker2 **Goal:** deploys expose Render's nested `commit.createdAt` from the resolved Git commit's real author timestamp — captured once at deploy-open, never inferred from the deploy row — across REST/GraphQL/MCP and the dashboard. **Status:** DONE 2026-07-16

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- | --- |
| t001 | Author-timestamp capture at deploy-open + store migration | 45m | — | — **DONE** |
| t002 | `commit.createdAt` on REST/MCP + GraphQL | 30m | t001 | — **DONE** |
| t003 | Dashboard display; omit-when-unavailable | 30m | t002 | — **DONE** |
| t004 | Render parity | 30m | t003 | — **DONE** |
| t005 | Simplify | 30m | t004 | — **DONE** |
| t006 | Test coverage | 45m | t004 | — **DONE** |
| t007 | Closeout | 15m | t006 | — **DONE** |

## Definition of done

A Git-backed deploy's REST/GraphQL/MCP representations carry `commit.createdAt` equal to the commit's author timestamp (verified against the actual repo commit); when resolution is unavailable the field is omitted — never fabricated from deploy time; the dashboard deploy detail shows it; `docs/render-artifacts/deploy-detail-page.md:26`'s residual is closed.

## Source + Goal linkage

- **Source:** promoted from inbox `w2/011` (filed by `w2/m38`'s 2026-07-15 Render recheck), `/pm-brainstorm` round 12.
- **Goal linkage:** Render deploy-object parity (extends w2/m38's lifecycle depth + w9/m1's deploy page).
- **Expected outcome:** the deploy object's commit block matches Render field-for-field.
- **Why now:** w2/m38's commit-resolution path is freshly shipped — the capture point exists; the milestone is small while the code is warm. Render parity closing task included — REST/GraphQL/MCP/UI change.

## Implementation summary (2026-07-16)

- **`github/client.go`**: `Commit` struct gains `AuthorAt *time.Time`; `GetCommit` parses `commit.author.date` from the GitHub API response.
- **`github/service.go`**: `resolveCommit` returns `store.CommitInfo{..., AuthorAt: c.AuthorAt}`.
- **`store/store.go`**: `CommitInfo` gains `AuthorAt *time.Time`; `Deploy` gains `CommitAuthorAt *time.Time`; `CreateDeploy`/`CreateRollbackDeploy` persist it; `deployColumns`/`scanDeploy` round-trip it.
- **`store/migrations/0040_deploy_commit_author_at.up.sql`**: `ALTER TABLE deploys ADD COLUMN commit_author_at timestamptz NULL`.
- **`deploys/service.go`**: `DeployView` gains `CommitAuthorAt *time.Time`; `view()` projects it.
- **`deploys/rest.go`**: `renderCommit` gains `CreatedAt *string`; `toRenderDeploy` populates it when non-nil.
- **`deploys/graphql.go`**: `deployGQLType` gains `commitCreatedAt` field.
- **`dashboard/src/features/deploys/api/deploy.graphql`** + **`deploys.graphql`**: `commitCreatedAt` added to both query selections.
- **`dashboard/src/graphql/definitions.ts`**: `Deploy` type, `DeployQuery`/`DeploysQuery` types, and pre-compiled document constants updated.
- **`dashboard/src/features/deploys/hooks/use-deploy.ts`** + **`use-deploys.ts`**: `DeployView`/`DeployRow` gain `commitCreatedAt`; `toDeployView`/`toRows` project it.
- **`dashboard/src/features/deploys/components/deploy-header.tsx`**: shows the author timestamp next to the short SHA when present.
- MCP: inherits via the shared `toRenderDeploy` — no separate change needed.
- 1246 dashboard tests pass; backend `go test ./...` clean; lint 0 issues.
