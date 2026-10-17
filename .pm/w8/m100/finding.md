# Native Python C-extension builds lack SWIG

Why: an ordinary supported binding build cannot generate its Python extension because the documented native environment omits swig.

**Severity:** major. **Ownership:** operator shared native-image tool assembly. Ordinary functional CLI sweep421,2026-10-17. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source7fa064b894d6f75e7f44f5bc65d10c5d0dd4d8c5; serving revision unknown. [Complete evidence](evidence/cli-sweep421/README.md).

## Reproduce and isolate

Both unchanged clients use supported `services create --type web_service --plan free --runtime python --env-var PYTHON_VERSION=3.13 --auto-deploy=false --confirm -o json`, fresh names and the public git-fixtures/basic/master carrier. A bounded source pack writes the explicitly authored app and own C interface. After exact public/app/interface hash observations, the build executes:

```sh
swig -python qa_swig.i
gcc -shared -fPIC $(python3-config --includes) qa_swig_wrap.c -o _qa_swig$(python3-config --extension-suffix)
python qa-swig-app.py build
```

Start is `python qa-swig-app.py serve`. Both actual unchanged native creates return POST201/exit0, but both original first builds fail `swig: command not found`; before observations report no executable. No extension build proof appears, and both URLs404. Adding only `apt-get update -qq && apt-get install -y --no-install-recommends swig` makes both native updates0 replacements Live, with source/generation/compiler/start/data/plan/env unchanged.

Real SWIG generates the wrapper C and proxy Python files; real gcc builds the shared object against the selected Python. Six HTTP200 responses execute its C functions and return independently expected weighted sums13/13/0 and exact Unicode text. Source/interface/generated-file hashes and the compiled extension filename/bytes/hash are checked at build/runtime; the extension identity agrees per service. Six native app events,12 REST/GraphQL/MCP reads and4 explicit-env reads agree. Both exact parents/all four children/URLs are deleted, baseline preserved and current quota inventories reconciled. Complete commands, outputs, wire, data and cleanup are retained.

[Render](https://render.com/docs/native-runtimes) includes SWIG at build/runtime. [SWIG's Python guide](https://www.swig.org/Doc4.1/Python.html) describes the wrapper/proxy and dynamic module pipeline used. Exact Render SWIG version and a Render-hosted comparison are unverified. This is an explicitly authored C binding, not an existing feature of the public carrier repository.

## Producer, consumer and repair

Pinned `pkg/service/create.go:272–310` and `pkg/service/update.go:361–377` serialize the supported commands. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` decodes them, `apps/service.go:2935–2950` persists them, and operator `internal/controller/app_controller.go:1048–1064` passes them into build.Options. Exact shared readbacks and the one-line supplier control show these layers work; no API, launcher or schema repair is needed.

`lego/operator/internal/build/native.go:283–320` assembles the selected base, existing Node/media/default helpers and authored build without a SWIG supplier. The selected immutable Python3.13 base has no swig. `native.go:404–407` emits that sole recipe; `build.go:1176–1183,936–960` passes recipe/native inputs to BuildKit; `nativeresolve.go:53–120` selects primary/companion tools without providing SWIG. Actual recipe exports and selected-base real compilation/application controls confirm the missing-tool mechanism.

Provide a reviewed immutable amd64/arm64 SWIG distribution with its compatible executable/library closure and required language/include/typemap data before authored native builds, retained through applicable runtime/pre-deploy/static phases. A binary-only copy without SWIG's data tree is insufficient. Review exact version/content/platform/distro/ABI/data/freshness identities and cache inputs across bookworm/trixie and all intended bases. The diagnostic Debian SWIG4.1.0 install is not a reviewed final supplier. Preserve Python headers/toolchain, project-local tools, explicit PATH/env, source-root/files/cache/companions/media/native-init and existing app libraries. Avoid runtime downloads, donor root-filesystem replacement, new API selectors or CLI forks.

One generator/preparer/resolver/BuildJob chain serves6 primary runtimes/16 reviewed image lines and native static builds; give its base/phase/consumer audit a separate task. Preserve REST/GraphQL/MCP/Blueprint/dashboard runtime/command/plan/env semantics, keep Dockerfile/prebuilt-image/CNB excluded, and document adoption/limits in ADR060. PM open/blocked/done and targeted SWIG build history have no existing supplier repair. Other native-tool findings concern different executables/libraries; no non-goal or documented divergence applies.

Actual generated-image regressions must fail before/pass after for real wrapper/proxy generation, gcc compilation, import and exact independent C arithmetic/Unicode results. Include a meaningful invalid-interface failure with nonzero status, network-disabled runtime generation, data-tree presence and intended base/architecture/companion/env/PATH/cache/static/excluded neighbors. After publication both unchanged clients' original first Free services must become Live without the install line and pass exact source/extension/results/HTTP/log/shared-readback/cleanup checks. Code or explicit-install controls alone do not meet DoD.

Local selected-base original127/control0 and three own-loopback HTTP checks pass; full generated native images are not executed locally. Only Python3.13/Free web/this tiny C binding is hosted. Other bases/architectures/languages/static/pre-deploy/browser, broader C++/typemap/language behavior, offline runtime generation, physical teardown and serving SHA/OCI remain unverified. Product code unchanged; this schedules the repair.
