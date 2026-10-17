# Native production bundling fails because webpack is absent

Why: an ordinary documented native build command cannot produce the JavaScript bundle that starts the service.

**Severity:** major. **Ownership:** operator shared native-image tool assembly. Ordinary functional CLI sweep422, 2026-10-17. Published Bex 0.3.2/a6350da0 and unchanged Render 2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source 7fa064b894d6f75e7f44f5bc65d10c5d0dd4d8c5; serving revision unknown. [Complete evidence](evidence/cli-sweep422/README.md).

## Reproduce and isolate

Both unchanged clients use supported `services create --type web_service --plan free --runtime node --env-var NODE_VERSION=24 --auto-deploy=false --confirm -o json`, fresh names and the public git-fixtures/basic/master carrier. A bounded source pack writes an explicitly authored entry module, imported domain module and observer. After exact public/module hashes and compiler availability observations, the build runs:

```sh
webpack ./qa-webpack-entry.mjs --mode production --target node --output-path ./dist --output-filename bundle.cjs --json > qa-webpack-stats.json
node dist/bundle.cjs build
```

Start is `node dist/bundle.cjs serve`. Both actual native creates return unchanged POST 201/exit 0. Both original first builds fail `webpack: command not found`, with ENOENT before observations, no bundle proof and URL 404. Each client adds only `npm install --global --no-audit --no-fund webpack@5.112.0 webpack-cli@7.2.3`. Both native updates exit 0 and produce one replacement each, both Live, with source/bundle arguments/start/data/plan/env unchanged.

Six real HTTP 200 responses run the generated bundle and return independently expected exact Unicode/space labels, total 10 and sorted quantities 2/3/5. Actual compiler JSON statistics, original/module hashes, runtime webpack availability and matching build/runtime bundle byte/hash identities are checked. Six native app events, twelve REST/GraphQL/MCP reads and four explicit-env reads agree. Both exact parents/all four children/URLs are deleted, baseline preserved and current inventories/quotas reconciled. Commands, outputs, wire, source/data/results and cleanup are durable.

[Render](https://render.com/docs/native-runtimes) includes Webpack at build/runtime. [Webpack's CLI docs](https://webpack.js.org/api/cli/) describe literal entry bundling, production mode, Node target, outputs and JSON statistics. Exact Render compiler/CLI versions and a Render-hosted comparison are unverified. The current official core 5.112.0/CLI 7.2.3 pair is diagnostic, not a required platform version. This is an explicitly authored bundled server, not an existing feature of the public carrier repository.

## Producer, consumer and repair

Pinned `pkg/service/create.go:272–310` and `pkg/service/update.go:361–377` serialize the supported commands. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` decodes them, `apps/service.go:2935–2950` persists them, and operator `internal/controller/app_controller.go:1048–1064` passes them into build.Options. Exact shared readbacks and the supplier-only controls show those layers work; no API, launcher or upstream schema repair is needed.

`lego/operator/internal/build/native.go:283–320` assembles the selected base and Node/media/default helpers without a Webpack supplier. The immutable Node 24 base has no executable. `nativepnpm.go:33–62` adds Yarn/pnpm; `nativeresolve.go:131–178` adds the existing JavaScript companions, without Webpack/CLI. `native.go:404–407` exports the sole native recipe; `build.go:1176–1183,936–960` passes recipe/native inputs to BuildKit; `nativeresolve.go:53–120` selects bases/companions. Actual source recipe exports and selected-base bundling/application controls confirm the omission.

Provide a deliberately reviewed immutable multiarch Webpack distribution with a compatible noninteractive CLI and complete dependency/data closure before authored native builds, retained through applicable runtime/pre-deploy/static phases. Copying only the root package or its bin wrapper can leave the command unable to find its CLI/dependencies. Review exact package/content/dependency-lock/platform/engine/freshness and cache identities for Node 22/24/26 and companions across bookworm/trixie. The observed CLI requires Node >=20.9 and a compatible Webpack peer; do not adopt the control pair without reviewing the full graph. Keep tenant project-local compilers and explicit PATH/env behavior. Avoid runtime downloads, interactive installation prompts, donor root-filesystem replacement, new API selectors or CLI forks; preserve existing toolchains, Yarn/pnpm/Bun/media/native-init/source-root/files/env/cache and app libraries.

One generator/preparer/resolver/BuildJob chain serves six primary runtimes/16 reviewed image lines and native static builds; give the base/phase/consumer audit its own task. Preserve REST/GraphQL/MCP/Blueprint/dashboard command/runtime/plan/env semantics, keep Dockerfile/prebuilt-image/CNB excluded, and document supplier adoption/limits in ADR060. PM open/blocked/done and targeted Webpack build history contain no matching supplier repair. The TypeScript compiler finding m99 has a different executable/payload and does not provide Webpack or its CLI. Other named-tool findings also do not supply it. No non-goal or documented divergence applies.

Actual generated-image regressions must fail before/pass after for real multi-module production bundling, emitted-bundle execution, independent module/data results, compiler statistics and source/output identities. Include a meaningful unresolved-module compile error with nonzero status, noninteractive runtime compilation with network disabled, and intended architectures/bases/companions/env/PATH/cache/static/excluded neighbors. After publication both unchanged clients' original first Free services must become Live without an install line and pass HTTP/log/shared-readback/cleanup checks. Code or install controls alone do not meet DoD.

Local selected-base original/control exits 127/0 and three own-loopback HTTP checks pass; full generated native images are not locally executed. Only Node 24/Free web/this two-module Node production bundle is hosted. Other bases/architectures/languages/static/pre-deploy/browser targets, loaders/plugins/dev-server, offline runtime compilation, physical teardown and serving SHA/OCI remain unverified. Product code unchanged; this schedules the repair.
