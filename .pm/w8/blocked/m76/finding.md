# A Git create with no branch builds hard-coded main instead of the repository's default

Why: ordinary native service creation omits branch to use the repository default, but Bex records main and makes a working master-based repo fail its first deployment.

**Severity:** major — core Git hosting fails after a successful native create. **Owner:** backend source/default resolution. Source: functional `$qa-find-bugs-cli` w8/sweep319,2026-10-15 UTC. [Complete evidence](evidence/cli-sweep319/README.md). Source2f6451567; serving revision unknown.

## Reproduce and expected result

```sh
bex services create --name "$FRESH_OWNED_NAME" --type web_service --runtime node \
  --repo https://github.com/expressjs/serve-static \
  --build-command 'node --version' \
  --start-command 'node -e "require("node:http").createServer((q,r)=>r.end("ready")).listen(Number(process.env.PORT),"0.0.0.0")"' \
  --env-var NODE_VERSION=24 --plan free --region frankfurt \
  --auto-deploy=false --confirm -o json
```

The short reproduction preserves branch omission; complete actual marker commands are retained in the evidence. Repeat using the unchanged Render2.27/a764810a7682 client against Bex, a fresh owned name and the isolated workspace/config. Both actual commands exit0/HTTP201 with branch main. Their raw native POSTs omit branch entirely. Both first deployments fail with fatal: couldn't find remote ref main. [Public repository metadata](evidence/cli-sweep319/public-repository-metadata.json), fetched without auth immediately before the journey, identifies the actual default as master.

[Render's Create service contract](https://api-docs.render.com/reference/create-service) says omitted branch uses the repository default. Expected: resolve/store master before the first deployment for this repo, build that ref and reach Live. Preserve any explicitly supplied branch. The CLI did not send an invalid explicit main value; this is server ownership, not an upstream/launcher request repair.

Both same owned resources accept a native branch-only update to master. Their auto-deploy-off setting correctly leaves deployment history unchanged; after checking that state, one explicit native deploy per resource reaches Live/HTTP200 with matching authored/build markers and Node v24.21.0. Repo/runtime/plan/env/build/start text are unchanged. Two failed initial deploys, two successful controls and four exact-owned deployment children are accounted for and cleaned.

## Mechanism and concrete correction

- Pinned cmd/servicecreate.go:67 defines branch with an empty default; pkg/service/create.go:72–73 serializes it only when supplied. Both captured requests prove omission. REST apps/rest.go:565 passes the resulting empty string into CreateRequest; GraphQL apps/graphql.go:717 and MCP creation args reach the same core.
- `apps/service.go:1966–1970` calls specFromCreate before new-service planning. At3048/3069–3070 normalizeCreateDefaults replaces empty branch on any Git repo with appv1alpha1.DefaultBranch (main). Spec persistence at2906 and provisionAppIdentity:2347/2361 then use that chosen value for the row and initial commit lookup. No repository-default lookup occurs at this boundary.
- The operator clone container (`operator/internal/build/build.go:1396–1402,1425–1437`) receives REF and fetches it, then checks out FETCH_HEAD. It correctly fails on the stored nonexistent main; it must not silently fall back away from an explicit user branch. The best-effort commit resolver at apps/commit.go:49–62 cannot supply the missing policy: it treats unresolved refs as optional provenance and also falls back to main for an empty ref.
- Resolve an omitted branch from the known GitHub repo metadata during create request preparation, before spec/row/first-build commit projection. Store and return that actual branch. Preserve explicit nonempty branch values, image/no-repo behavior and validation; a lookup failure must be actionable rather than fabricated as main. Leave the legacy CR DefaultBranch constant/fallback alone where its existing contract still applies.
- Reuse the existing repository metadata boundary. `github/client.go:164,612,621–644` already carries defaultBranch in repository views; its RepoAccessible:538 HTTP lookup currently discards the repository body into a boolean. `github/service.go:1127–1144` validates accessible sources. Extend/reuse that source information with bounded existing lookup/error behavior; avoid a second unrelated client/framework or requiring new user credentials. The dashboard already sends form.branch || selectedRepo.defaultBranch at features/services/lib/create-service-input.ts:184; API omission still needs the server default.
- Preserve first-build commit pinning: resolve the branch once and feed the same selection into row, deploy provenance and actual clone. Audit direct creation/dry-run adapters, source changes, Blueprint connection/defaults, webhooks and legacy missing-branch paths without globally rewriting main to master or changing explicit-empty update policy without its own contract evidence.
- Audit generated Blueprints at apps/blueprint_generate.go:364–368: omitting every branch equal to main can become lossy once a repo with a different default is supported. Export must preserve a selected explicit main as well as resolved non-main branches. Coordinate m75's unknown enclosing-repository validation context: no repository metadata should be fabricated or looked up for an unavailable repo.
- Update the creation-specific MCP descriptions that still promise default main (apps/mcp.go:176,302,530). Keep existing explicit branch edits and auto-deploy-off/no-implicit-deploy semantics. No new CLI flag/runtime field or fork is necessary.

## Dedupe, verification and limits

Searches across all open/blocked/done workstreams, open milestone titles, DO_NOT_DO, CLI ledger and default-branch/missing-main/source history found no owner. The global main fallback predates this hunt. [m70](../../done/m70/README.md) fixes source switches, [m75](../m75/README.md) handles unavailable repository context during standalone validation, and w5/138 pins the first build's commit; none chooses the real default for an omitted known repo branch. This is an uncovered create/default omission, not a claim those fixes regressed or complete original live DoDs were replayed.

15 named existing checks across nine top-level tests pass, including explicit branch adapters, first-build pinning, source/runtime and metadata lookup controls. There is no candidate implementation or full backend suite. Future regressions must give the source boundary actual mocked default-branch metadata (main/master/trunk), preserve explicit branches and image behavior, and verify selection/persistence/commit/build and export. A test that merely expects the present main constant is insufficient.

Only public GitHub Node creation is live-tested. Private/other providers, static/private/worker/cron, Blueprint connections/source changes, unavailable metadata and live GraphQL/MCP/dashboard are unverified; audit them and record scoped follow-ups without claiming blanket support. No provider-integration expansion is proposed. Two exact-owned Free parents/four deployment children are gone; baseline/quota preserved. This schedules185m across six tasks and supplies no product edit, new ship or external submission.
