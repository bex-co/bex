# Native TypeScript builds fail because tsc is absent

Why: ordinary native TypeScript build commands cannot emit the JavaScript service even though the documented native environment supplies a compiler.

**Severity:** major. **Ownership:** operator native-image assembly. Functional CLI sweep420,2026-10-17. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source7fa064b894d6f75e7f44f5bc65d10c5d0dd4d8c5; actual serving revision unknown. [Complete evidence](evidence/cli-sweep420/README.md).

## Reproduction and observation

Both clients use supported `services create --type web_service --plan free --runtime node --env-var NODE_VERSION=24 --auto-deploy=false --confirm -o json`, with fresh names, the public git-fixtures/basic/master carrier and exact authored typed input. The build reconstructs bounded own files, observes original public/source hashes, then runs:

```sh
node qa-ts-before.cjs
tsc --strict --noEmitOnError --module NodeNext --target ES2022 --outDir dist qa-ts-app.cts
node dist/qa-ts-app.cjs build
```

Start is `node dist/qa-ts-app.cjs runtime`. Actual unchanged native POST201/exit0 creates both Free services, but both original first builds fail `tsc: command not found`; before observations report ENOENT. Both failed URLs404, with no emitted-JavaScript build proof. Each client adds only `npm install --global --no-audit --no-fund typescript@7.0.2` after the before observer. Both native updates0 yield one replacement each, both Live. No source/data/start/plan/env change or client fork is required.

The actual strict compiler lowers the enum and emits JavaScript. Six real HTTP200 responses execute that emitted file and return exact independently expected Unicode/space labels, total10, warm/cool/warm enum values, actual compiler version and original source hash. Build/runtime emitted-file hashes agree per service. Six native app events and12 REST/GraphQL/MCP plus4 explicit-env readbacks agree. Both exact parents/all four children/URLs are deleted and baseline/current quotas reconciled. Full native commands/results/redacted create wire, actual source/results and cleanup are durable in the evidence.

[Render](https://render.com/docs/native-runtimes) documents TypeScript as supplied in all native runtimes during build and runtime. [Official tsc usage](https://www.typescriptlang.org/docs/handbook/compiler-options.html) supports literal file compilation and the options used. Node's ability to execute some TypeScript syntax does not provide this tsc compiler contract. Exact Render compiler version and a Render-hosted comparative run are unverified; the official current7.0.2 NPM control is diagnostic and not a required platform version.

## Producer, consumer and repair

Pinned `pkg/service/create.go:272–310` and `pkg/service/update.go:361–377` serialize the supported native commands. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` decodes them, `apps/service.go:2935–2950` persists them, and operator `internal/controller/app_controller.go:1048–1064` passes them into build.Options. Shared exact readbacks and successful compiler-only controls prove transport/persistence work. No API, launcher or upstream schema repair is needed.

`lego/operator/internal/build/native.go:283–320` assembles the selected primary image and Node/media/default helpers but never supplies a compiler. The immutable selected Node24 base has no tsc. `native.go:404–407` exports this sole native recipe; `build.go:1176–1183,936–960` passes it and native inputs into BuildKit. `nativeresolve.go:53–120` selects bases/companions without supplying TypeScript. Actual generated recipe exports and selected-base application controls confirm this mechanism.

Provide a reviewed immutable amd64/arm64 TypeScript compiler distribution before authored native build, retaining its usable executable, required runtime libraries and standard-library/data files through runtime/pre-deploy/static phases where applicable. Choose/review the supported release deliberately; do not blindly convert the7.0.2 control into a pin. Current observed7.0.2 uses native per-platform packages, so copying only the root NPM package or assuming old pure-JS/compiler-API/language-server layouts is insufficient. Review engine/platform/distro/ABI/data compatibility for Node22/24/26 and language companions across bookworm/trixie, with exact content/version/platform/freshness and cache identities. Keep tenant project-local compilers and explicit PATH/version/env behavior. Avoid runtime downloads, donor root-filesystem replacement, new API selectors and CLI forks; preserve existing Node/Yarn/pnpm/Bun/media/native-init/env/files/root/static/Docker/image/CNB behavior.

## Scope, dedupe and acceptance

One native generator/preparer/resolver/BuildJob chain serves6 primary runtimes/16 reviewed image lines and native static builds; the base/companion/phase audit is separate work. Preserve REST/GraphQL/MCP/Blueprint/dashboard command/runtime/plan/env semantics; document adoption and actual version limits in ADR060. Existing PM open/blocked/done and targeted TypeScript/tsc build histories have no matching native supplier repair. The only broad board search match concerns unrelated dashboard event vocabulary. Other native-tool milestones cover different executables/libraries. No non-goal or documented divergence found.

Real generated-image regressions must fail before/pass after: strict typed/enum compilation, emitted-JavaScript execution with independently expected data and exact source/output identity, a deliberate type error rejected by noEmitOnError without emitted output, actual network-disabled runtime compilation and affected architectures/bases/companions/env/PATH/cache/static/excluded paths. After publication both unchanged native clients' original fresh first Free services must become Live without a compiler install and pass HTTP/log/shared-readback/cleanup checks. Code or explicit-install controls alone do not meet DoD.

Local selected-base original127/control0 and three loopback HTTP controls pass; full generated native images are not locally executed. Only Node24/Free web/this .cts compile is hosted. Other bases/architectures/languages/static/pre-deploy/browser, broader compiler/API/language-server behavior, offline runtime compilation, physical teardown and serving SHA/OCI remain unverified. No product implementation changed; this schedules the repair.
