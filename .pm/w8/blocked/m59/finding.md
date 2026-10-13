# Clearing a static build command retains its implicit native builder

Why: switching a static site to publishing plain repository files must depend on its current settings, rather than whether it had a build command when it was created.

**Severity:** major. Functional CLI hosting QA, sweep 99, 2026-10-11 UTC. **Attribution:** backend configuration projection → operator build selection. No launcher rewrite is needed. Bex release **0.3.2** and unmodified Render **2.27.0**, pin **a764810a768202704e7206eb7b87a47211fcd98e**, against `https://api.bex.co`, human QA grant, workspace `tea-daif693dqjvc73e7as3g`, noninteractive JSON. Platform revision is unobserved. Research HEAD is `741b541d2d32af54cb36ed09f237969c1bef78ae`; application source commits are recorded separately and do not identify the platform revision.

## Native journey and positive control

Create a fresh static site without declaring a runtime or builder:

```sh
bex services create --name UNIQUE_NAME --type static_site \
  --repo https://github.com/bex-co/bex --auto-deploy=false \
  --root-directory examples/static-site --publish-directory dist \
  --build-command "mkdir -p dist; printf '%s' 'OWN_MARKER' > dist/index.html; printf '%s' 'OWN_ASSET' > dist/version.txt" \
  --confirm -o json
```

Static create does not accept `--plan`; its server default and every recorded service read are Free. Wait for `deploys list ID --confirm -o json` to report Live and the public index/asset to return their exact markers. Then:

```sh
bex services update ID --build-command '' --publish-directory . --confirm -o json
```

Expected: the now empty build step publishes `examples/static-site` directly. Actual: the save succeeds, a new deploy fails with `dispatching build: build: native runtime node requires buildCommand`, and the previous publication stays online.

| Owned fixture | Native clear result | Final deploy / public result |
| --- | --- | --- |
| `srv-db5lnhaek99c73ahuqjg`, created with build | Bex exit 0, 1.536s | `dep-db5lqh5m4o4s73bmn97g` build_failed; previous B page/asset remain 200 |
| `srv-db5luiaek99c73ahur10`, fresh equivalent | Render exit 0, 1.554s; captured PATCH 200 | `dep-db5lvrlm4o4s73bmn9lg` build_failed; previous exact page/asset remain 200 |
| `srv-db5lrqlm4o4s73bmn9cg`, created without build, then added the same kind of build | Bex exit 0, 1.642s; captured PATCH 200 | `dep-db5luddm4o4s73bmn9i0` Live; exact repository-at-deploy-commit HTML 200; removed generated asset 404 |

[Initial Bex journey](evidence/initial-bex.json), [fresh Render reproduction](evidence/fresh-render.json), and [passing creation-history control](evidence/passing-control.json) preserve full native stdout/stderr, deploy observations, exact content and diagnostic reads. The transparent forwarder changed only the authority, forwarded actual native requests unchanged to Bex, copied the isolated configuration, never journaled Authorization, and reaped the child/server/config. Both complete captures include:

```json
{
  "method": "PATCH",
  "body": { "serviceDetails": { "buildCommand": "", "publishPath": "." } },
  "status": 200
}
```

The evidence contains each actual path, safe headers and complete response. Read-only GraphQL confirms `runtime:""`, `buildCommand:""`, `publishPath:"."` on all cleared sites, but **builder native on failures versus auto on the passing control**. The first fixture's compound A→B build/output-directory change and native clear-cache `--wait` redeploy both passed; the latter exited 0 after 79.012s with final status Live and exact B content. This separates command clearing from general static build/publication failure.

An additional fresh equivalent, `srv-db5lrv2ek99c73ahuqq0`, received the actual Render clear PATCH 200 and reached the same terminal failure. Its forwarder initially rejected the CLI's subsequent workspace-project hydration read. That exit 1 is **harness-only**, not a product finding; no mutation was replayed. [Qualified receipt](evidence/first-render-qualified.json) and [all qualifications](evidence/qualifications.json) retain this limitation. The fresh Render row above completes normally with the repaired capture.

## Root cause and concrete fix

At the recorded HEAD:

- `lego/backend/internal/apps/service.go:2872–2876` materializes **builder native** when static create has a build command and no runtime/Dockerfile/explicit builder. Runtime intentionally stays empty.
- Pinned `pkg/types/service/serviceupdate.go:29,73` and `pkg/types/util.go:10–16` preserve a present empty build-command pointer. `pkg/service/update.go:456–460` serializes the exact clear body; `cmd/serviceupdate.go:139–154` PATCHes then hydrates the saved service. Both native captures prove this boundary works.
- `lego/backend/internal/apps/rest.go:860–861,1006–1007` maps the static command/path to the shared patch. `settings.go:379–395` and `service.go:4063–4071` write the new BuildCommand but leave Builder unchanged. The compound staging at `settings.go:640–685` publishes one consistent configuration; this is not the already-fixed intermediate-PATCH problem.
- `lego/operator/internal/controller/app_controller.go:1464–1469` permits direct publication only with empty/auto builder, so the retained native builder excludes it. `effectiveBuilder` at 1476–1493 keeps native. `internal/build/build.go:607–610` validates native options; `internal/build/native.go:148–160` defaults static runtime to node and refuses the empty command.

Normalize the **implicit static build strategy** together with command intent in the canonical backend operation, including its preflight/staged probe. A static site whose native builder came only from its initial command must return to empty/auto when that command is explicitly cleared and no other build input remains; adding a command must still execute it. Avoid persisting redundant create-time strategy where operator derivation suffices, and handle already-existing implicitly native sites. Distinguish explicit CR builder/runtime/Dockerfile/buildpack intent before changing its semantics; preserve non-static commands, source fields and the single-rollout guarantee. Do not weaken native build validation or add a CLI payload workaround.

## Shared callers and acceptance scope

One `SetCommands` implementation backs the REST patch row and GraphQL `setBuildCommand`/`setStartCommand` (`graphql.go:1590–1618`); MCP `update_service` reaches `ApplyServicePatch` and explicitly documents empty commands as clearing (`mcp.go:99,813`). Dashboard `use-build-command` feeds the shared GraphQL setter through BuildDeploySection. Four runtime-backed families and static share command handling; explicit builder/runtime forms and Blueprint create/export also consume the projected strategy. Dedicated verification must count these callers and exercise the two static creation histories, omitted versus empty versus nonempty commands, explicit strategy neighbors, compound path saves, and normal web/private/worker/cron behavior.

Only the native CLI failures, GraphQL **reads**, public HTTP and cleanup are live-confirmed. Dashboard save, MCP update, GraphQL mutation and explicit strategy neighbors are unverified. [Render static-site documentation](https://render.com/docs/static-sites) was checked 2026-10-11 for the Git/build/publication lifecycle; it does not establish this empty-command transition. Render production was not contacted. The concrete target follows the pinned clear payload, Bex's documented optional build in [ADR029](../../../../docs/ADR029-static-sites.md) and the passing same-request control. Parity acceptance must verify the remaining equivalents without claiming they were exercised here.

## Dedupe, cleanup and boundaries

Open, blocked and done board searches for clearing static commands, retained native builders and the exact failure found no matching repair. w4/m197 restores retained rollback publications, w7/done/m88 resets build caches, w8/done/m56 stages compound patches, w4/246 is confirmation wording, and w4/blocked/m163 concerns build-file delivery. Those are distinct; targeted current source/history contains no clear-strategy fix, and unrelated application-source documentation updates do not resolve this failure. Recheck main and deployed revision before implementation acceptance.

All four owned fixtures were deleted by recorded ID after child-deploy enumeration. [Cleanup receipts](evidence/cleanup.json) show native delete success, detail/list/runtime absence, original six baseline services restored, quota services used 6 and terminating 0; Postgres/Key Value remain empty. No product fixes or shipping occurred. Scope is this functional configuration bug.

## Fresh live acceptance — sweep180, 2026-10-13 UTC

[Four isolated fixtures, exact native clear wire and full cleanup](evidence/live-static-sweep180/README.md) reproduce the same creation-history failure through both clients: save exit0/HTTP200, then build_failed and old content retained. Both direct-created→built→cleared controls return Live/source page/asset404. The source fix is present and passes targeted tests, but platform revision is unknown; update this existing item rather than file another bug.

## Native acceptance change — sweep184, 2026-10-13 UTC

[Fresh two-client build-created clear transitions](evidence/live-static-sweep184/README.md) now pass with actual native PATCH captures, Live/exact source index/removed asset and complete cleanup. Desired platform pin advanced, exact live SHA unknown. Existing m59 is active again for its remaining cross-surface/legacy/neighbor DoD; this update neither implements a fix nor closes untested acceptance.
