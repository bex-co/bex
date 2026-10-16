# Explicit Bun version does not make Bun available in native builds or runtime

Why: a standard Node/Bun service configuration accepted by the CLI cannot build, even though its version setting reaches both execution environments.

**Severity:** major, runtime compatibility gap. **Attribution:** operator/native toolchain assembly. **Status:** todo. Functional `$qa-find-bugs-cli` w8/sweep306,2026-10-15 UTC. Source HEAD562cbfd25a6a75c559d705efc3736432425e9c7d; exact deployed backend/operator revision is unobserved. Published Bex0.3.2 and unmodified Render2.27/a764810a768202704e7206eb7b87a47211fcd98e, isolated human-QA workspace bex-canary / tea-daif693dqjvc73e7as3g, non-TTY JSON. [Full durable evidence](evidence/cli-sweep306/README.md).

## Reproduction and observed boundary

Use a new ledger-owned name/nonce under an isolated QA config, then run the exact authored commands below. The harmless original marker can be replaced consistently with a fresh one. Complete original argv and full redacted POST/response for both independent fixtures are in the evidence.

```sh
AUTHORED_BUN_BUILD='printf '"'"'%s\n'"'"' QA306_69d708_BUN && bun -e '"'"'require('"'"'"'"'"'"'"'"'node:fs'"'"'"'"'"'"'"'"').writeFileSync('"'"'"'"'"'"'"'"'qa306-built.json'"'"'"'"'"'"'"'"',JSON.stringify({marker:"QA306_69d708_BUN",runtime:'"'"'"'"'"'"'"'"'bun'"'"'"'"'"'"'"'"',version:Bun.version,requested:process.env.BUN_VERSION}));'"'"''
AUTHORED_BUN_START='bun -e '"'"'const built=JSON.parse(require('"'"'"'"'"'"'"'"'node:fs'"'"'"'"'"'"'"'"').readFileSync('"'"'"'"'"'"'"'"'/opt/render/project/src/qa306-built.json'"'"'"'"'"'"'"'"'));Bun.serve({hostname:'"'"'"'"'"'"'"'"'0.0.0.0'"'"'"'"'"'"'"'"',port:Number(process.env.PORT||3000),fetch(){return new Response(JSON.stringify({marker:"QA306_69d708_BUN",runtime:'"'"'"'"'"'"'"'"'bun'"'"'"'"'"'"'"'"',version:Bun.version,requested:process.env.BUN_VERSION,built}));}});'"'"''
bex services create --name "$FRESH_OWNED_NAME" --type web_service --runtime node \
  --repo https://github.com/expressjs/serve-static --branch master \
  --build-command "$AUTHORED_BUN_BUILD" --start-command "$AUTHORED_BUN_START" \
  --env-var BUN_VERSION=1.3.4 --plan free --region frankfurt \
  --auto-deploy=false --confirm -o json

```

Expected: the supported explicit Bun opt-in makes Bun available before the build command and in the produced service image; the authored Bun build/start can reach Live with its marker/version. Actual: both native commands exit0/HTTP201, but the first deploys fail with `/bin/bash: line 2: bun: command not found`; both native readers capture BuildKit exit127. No Bun application becomes live. Both valid native POSTs contain runtime node, plan free, exact build/start commands and envVars[{key:BUN_VERSION,value:1.3.4}]. A failed build is correctly classified; the missing supported companion toolchain is the gap.

Normal Node controls on each same owned service then reach Live and HTTP200 through native command updates. Actual build and runtime report Node v26.11.1 and requested BUN_VERSION=1.3.4, while spawning Bun returns ENOENT at both phases. This separates tool installation from env delivery and normal native execution. No source/repo/plan change or manual Bun installation is used. These are controls, not a repaired Bun deployment.

[Render's Bun version documentation](https://render.com/docs/bun-version) makes version selection or a Bun lockfile an opt-in, with BUN_VERSION preceding .bun-version;1.3.4 is an explicit documented example. [Native runtime documentation](https://render.com/docs/native-runtimes) includes Bun during build and runtime. Therefore merely omitting Bun without any opt-in is not this finding: both tested services explicitly select it. The pinned CLI's existing node runtime and ordinary env-var wire already express this configuration; no new runtime enum or CLI fork is required. Independent authenticated Render production is untested.

## Producer, consumer and concrete correction

- Pinned cmd/servicecreate.go normalizes the unchanged input; pkg/service/create.go:45,95 parses and assigns EnvVars. The captured raw native body proves both commands and BUN_VERSION survive this boundary. Backend apps/rest.go:540,556–574 maps the valid environment and runtime settings into CreateRequest. Node controls independently prove current env persistence/delivery, so changing the CLI or stripping this accepted setting is not the fix.
- `lego/operator/internal/build/nativeversion.go:68–93` selects only primary Python/Node/Ruby/Elixir version signals (Go/Rust retain their policies). It never reads BUN_VERSION, .bun-version or Bun lockfiles. `nativeresolve.go:48–69` decodes the complete build-env bundle, calls that selector, and only rewrites the primary FROM image. Actual six-runtime resolver exports show the harmless Bun setting is decoded but adds no toolchain.
- `native.go:82–117` inventories primary language images; `nativeDockerfile:234–273` builds directly on that image, copies the source/env loader, runs the authored build and retains the same image for CMD. There is no optional Bun stage or install/copy before RUN, nor an artifact carrying Bun into runtime. Bare Bun therefore fails in Bash; ordinary Node and the env bundle work.
- Add **optional reviewed Bun companion selection** to the native resolve/assembly path. Keep node as the primary runtime. Resolve explicit BUN_VERSION first, then the documented file/lock opt-ins; keep absent-opt-in behavior unchanged. Reuse bounded version-file reading and the existing supported-version error policy. Decide/document supported Bun versions using the existing reviewed, digest-pinned inventory, including the reproduced supported request; do not silently claim an ignored version is active.
- Materialize the selected compatible Bun binary into the generated image **before** its authored build RUN and retain it for start/pre-deploy execution. Narrate the selected Bun version/source. Thread version identity into generated inputs/cache behavior so changing BUN_VERSION or a version file cannot reuse a previous Bun selection. Preserve the exact application PATH/command/env semantics fixed by m61. Primary language selection must remain independently correct.
- Inventory updates belong with `lego/operator/internal/build/toolchain-freshness.json` and the existing `scripts/build-toolchain-freshness.sh` validation path. This finding does not implement the mechanism or qualify a proposed image; maintained failing-before/fixed-after execution checks and actual hosted Bun acceptance remain required.

## Shared callers and regression scope

There is **one production nativeDockerfile call**, in nativeBuildPreparer at native.go:321; **one production ResolveNativeBuild call**, cmd/native-resolve/main.go:45; and **one production ResolveNativeVersion call**, nativeresolve.go:53. `build.go:1125–1130` wires the preparer/resolver before BuildKit. The shared native family contains six runtimes; static builds without a declared runtime select Node through nativeRuntime:148. Web/private/worker/cron/static builds share this mechanism; only Free Node web is live-tested here. Runtime appEnv/envFromSources at controller/deployment_projection.go:260–261 and the retained image supply running configuration. Dockerfile, prebuilt-image, buildpack and direct no-build static paths have distinct ownership and must retain their current behavior.

Add tests for explicit selection, .bun-version, lock opt-in/default, unsupported requests and omitted opt-in; primary runtime precedence stays intact. Execute the generated build/run artifact to prove Bun exists in both phases, rather than accepting Dockerfile string presence. Verify selected-version changes invalidate the relevant build inputs while unchanged requests remain stable. Audit all six primary images and applicable service types, plus static-build defaulting; preserve exact/narrow/empty PATH policy, native command bytes, env/file preservation and existing primary-version narration. REST/GraphQL/MCP/Blueprint/dashboard env writes must reach the same selector without a new wire field. Source-established siblings remain unverified until those checks run.

## Dedupe, sizing and limits

Exact Bun/BUN_VERSION/.bun-version/bun.lock searches across open/blocked/done board items find no owner.73 actual open/blocked milestone READMEs were read with no Bun match; targeted BUN_VERSION product history is empty. Related closed w8/m51 handles primary language versions and its full original DoD remains intact; Bun is an uncovered companion-tool gap. m61's PATH/entrypoint work is preserved and the live Node control works.052 correctly exposes the authored command/exit code. m68 command bytes and m62 env representability are separate; the exact captured bytes and live env controls pass here. No duplicate, CLI defect, newly introduced regression or inferred deploy-lag issue is filed.

Estimate200m across seven tasks: selector policy40, build/runtime materialization45, shared/cache audit25, Render parity20, Simplify10, execution regressions40 and closeout20. This exceeds the inbox sizing rule. No .bun-version/lock-only hosted case, other native runtime live case, private/worker/cron/static Bun case, fixed hosted build, complete real-DB/operator suite, frontend write journey, direct Kubernetes residue inventory or independent Render production is claimed.

## Cleanup and acceptance gate

Two exact-owned parents and four deployment children are fully cleaned. Native delete0 is followed by service404, list absence and former runtime404; immutable baseline6/quota6/terminating0. Config/relay cleanup completed. After implementation reaches production through the normal authorized workflow, repeat fresh two-client explicit Bun creates, build/runtime version+marker verification, variant/shared regressions and exact-owned cleanup before closing. This schedules a fix; QA changed no product code and performed no new ship, release, platform deployment or external submission.

## Hosted acceptance2026-10-15

Implementation760502dad is verified by [sweep327](evidence/cli-sweep327/README.md): both original native Bun creates reach first Live/HTTP200 with exact markers and resolved Bun1.3.14 at build/runtime, and a separate no-opt-in Node control passes with Bun absent. Current generated Node/Python images execute Bun successfully, both source packages and inventory checks pass, and all three parents/children/former URLs are absent with baseline/quota retained. The earlier missing-Bun diagnosis remains the historical source finding; no serving SHA or untested live configuration surface is inferred.
