# Yarn static builds skip automatic dependencies

Why: An ordinary locked Yarn project cannot build with its authored dependency-using command, even though the platform supplies Yarn and promises automatic static dependency installation.

**Severity: major. Attribution: shared native static-build assembly.** Functional CLI sweep401,2026-10-17 UTC, human device-login QA workspace bex-canary/tea-daif693dqjvc73e7as3g, production https://api.bex.co/v1/. Published Bex0.3.2 and unchanged Render2.27.0/a764810a7682 reproduce independently, non-TTY JSON; source2c95fd35b8b1ac980dfddec9cb7de77fcb01a584, serving revision unknown. Actual Node24.22.0/Yarn1.22.22. [Full durable evidence](evidence/cli-sweep401/README.md), [results](evidence/cli-sweep401/results.json).

## Reproduction and observations

Use the actual public render-examples/express-hello-world master commit e7a5c56626fab0673f244d169389bf34070e754c. Its package.json declares Express and its matching Yarn Classic lock pins4.18.2, with no competing manager lock. Those files are committed inputs present before the automatic install phase. A small authored static build imports/instantiates that dependency and writes \_site/index.html plus a source/dependency/hash observer.

Create through the real CLI:

```sh
bex services create --name OWN_QA_NAME --type static_site --repo https://github.com/render-examples/express-hello-world --branch master --build-command 'AUTHORED_OBSERVER_AND_DEPENDENCY_USING_BUILD' --publish-directory _site --env-var NODE_VERSION=24 --auto-deploy=false --confirm -o json
```

The [complete build command](evidence/cli-sweep401/bex-original-build-command.txt) replaces the build placeholder; no credential belongs in it. Repeat in a separate fresh fixture through unchanged Render pin against Bex. Static is intrinsically Free; no unsupported plan/runtime flag is invented.

Both actual unchanged POSTs201/native0 are recorded. Both original first deploy children become build_failed: the actual auto step announces skipping yarn.lock, the observer proves exact original file hashes, working Yarn1.22.22, absent node_modules/Express and absent SKIP_INSTALL_DEPS, then Node fails with Cannot find module 'express'. Both generated-evidence URLs404. This is asynchronous build failure, not a claim that accepted create must return a failure exit.

Each native services update changes only buildCommand by inserting yarn install --frozen-lockfile --non-interactive. Both updates0; one replacement child each reaches Live. Six fresh HTTPS200 reads prove the same source/lock hashes, instantiated Express4.18.2 and exact generated-index length/hash. Twelve shared source/command and four explicit-env reads preserve the exact inputs. The source project is a real public dependency manifest/lock; the static observer is explicitly authored, not the example's web application. Frozen mode only isolates the control.

[Render's static contract](https://render.com/docs/static-sites#dependency-installation) attempts dependency detection/installation by default; its [SKIP_INSTALL_DEPS description](https://render.com/docs/environment-variables) explicitly names yarn/npm. [Yarn Classic's contract](https://classic.yarnpkg.com/lang/en/docs/cli/install/) installs into node_modules and uses matching locked versions unchanged. Actual original failures plus working one-line controls isolate missing application installation from missing Yarn, wrong source, unsupported commands, or static publication.

## Producer, consumer and correction

Pinned pkg/service/create.go:236–247 sends StaticSiteDetailsPOST buildCommand/publishPath unchanged. Backend internal/apps/rest.go:500–501/518/568/588 preserves these; build_strategy.go:44–75 and operator app_controller.go:1483–1495 select native for a nonempty omitted-runtime static build. App_controller.go:1030–1045 passes StaticSite, publish path, command and effective env to build Options. Native.go:157–163 defaults that static runtime to Node.

The actual gap is lego/operator/internal/build/nativestaticdeps.go:56–65: any yarn.lock enters the other-manager skip branch and returns0 without installing. Yarn is already supplied. Native.go:262 is the sole production nativeStaticInstall caller in nativeDockerfile; nativeBuildPreparer:348/368 is its sole recipe producer; build.go:1132 wires it before BuildKit. Static publication at app_controller.go:3705/3746 consumes already-built files and cannot fix missing input dependencies.

Add automatic installation for unambiguous supported **Yarn Classic primary-Node static projects**, before the exact authored build in the current root/effective-env step. Reuse installed Yarn and its normal lock/config semantics. Preserve npm behavior, competing-manager protections, explicit case-insensitive SKIP_INSTALL_DEPS=true, false/empty detection, narrow/empty PATH and authored env/command bytes. Use documented noninteractive/dev-dependency controls as needed without changing caller NODE_ENV; do not impose frozen mode solely because the QA control used it. Keep current best-effort install/fallback semantics unless separate parity evidence justifies changing them.

One shared install path reaches static REST/GraphQL/MCP/Blueprint/dashboard sources, omitted-runtime default and explicit primary Node. Preserve buildless/root-published sites, non-Node primaries, compute, Dockerfile/image paths, version/companion selection, env/files and cache revisions. No request/schema/CLI change, new toggle or tool supplier is needed. Modern Yarn, other managers and workspace-root policy need explicit boundaries, not assumed support.

## Dedupe, verification and limits

[Dedupe](evidence/cli-sweep401/dedupe.txt) searched open/blocked/done PM and docs for Yarn automatic installs, yarn.lock/install and nativestaticdeps. [Completed npm m86](../../done/m86/README.md) explicitly left Yarn/pnpm/Bun automatic installation unverified; its npm DoD remains distinct. [w4/320](../../../w4/done/320.md) concerns Node26 lacking the Yarn executable, while this Node24 fixture successfully runs Yarn. The source helper's only history is npm implementation10dda1f1c; no later Yarn fix was found. Current anti-goals/compatibility ledger remain consistent with this bounded hosting repair.

Two actual BuildJob/ResolveNativeBuild observations retain both resolved recipes. Three real source-pinned base-image controls execute the exact extracted install step and authored command: default skips/fails1; explicit installation passes with unchanged lock; skip=true/manual passes. These are not full generated-image executions. Required future meaningful tests and shared blast-radius review are separate milestone tasks.

All original parent/child/URL ownership and cleanup receipts are durable. Both native deletes0; two parents/four children/URLs404/listabsent; original six services/one Blueprint remain; projects/groups/managed data empty, termination0. Own local containers/contexts/Go export package/relays/copied configs are removed, isolated login retained for the authorized loop. One invalid limit150 diagnostic was corrected to100 and retained as a harness qualification. No product implementation, fixed-client acceptance, browser authoring, broader hosted manager/root/version coverage, full platform suite or physical teardown is claimed. Six todo tasks/140m schedule the repair; filing/evidence uncommitted.
