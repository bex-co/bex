# Native asset compression fails because build-time pigz is absent

Why: a documented native build utility cannot compress the asset that the service is meant to publish.

**Severity:** major. **Ownership:** operator shared native build-image assembly. Ordinary functional CLI sweep423, 2026-10-17. Published Bex 0.3.2/a6350da0 and unchanged Render 2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source and serving revision are recorded in the evidence; serving SHA/OCI is unobserved. [Complete evidence](evidence/cli-sweep423/README.md).

## Reproduce and isolate

Both unchanged clients use supported `services create --type web_service --plan free --runtime python --env-var PYTHON_VERSION=3.13 --auto-deploy=false --confirm -o json`, fresh names and the public git-fixtures/basic/master carrier. A bounded source pack writes an explicitly authored app and input reference. After exact public/app/input hash observations, the build runs:

```sh
pigz -n -p 2 -c qa-pigz-input.txt > qa-pigz-output.gz
python qa-pigz-app.py build
```

Start is `python qa-pigz-app.py serve`. Both actual native creates return unchanged POST 201/exit 0, but both original first builds fail `pigz: command not found`, with no executable before compression, no gzip build proof and URL 404. Adding only `apt-get update -qq && apt-get install -y --no-install-recommends pigz` makes both native updates exit 0 and produce one replacement each, both Live. Source/compression arguments/start/data/plan/env are unchanged.

The owned 12,544-byte UTF8 asset repeats a documented Unicode/space-containing line. Actual pigz produces its gzip file; the build independently inflates the data and records header/trailer fields and the compressor version. Runtime uses Python's standard library to inspect and serve this artifact; it does not invoke pigz. Six HTTP 200 responses include four JSON proofs and two raw gzip downloads. Independent host zlib inflation plus explicit header/CRC32/ISIZE checks recover the exact input; bytes/hashes match the recorded build/runtime artifact. Six native app events, twelve REST/GraphQL/MCP reads and four explicit-env reads agree. Both exact parents/all four children/URLs are deleted, baseline preserved and current inventories/quotas reconciled. Commands, outputs, wire, source/data/results and cleanup are durable.

[Render](https://render.com/docs/native-runtimes) marks pigz as **build-time only**. [Official pigz documentation](https://zlib.net/pigz/) describes gzip compatibility and zlib/pthread dependencies. The claim requires build-time pigz and a correct output artifact at runtime; it makes no runtime pigz availability requirement. The two-worker option on this small asset does not prove parallel throughput. Observed Debian pigz 2.6-1/version 2.6 is diagnostic, not an adopted platform pin. Exact Render versions and a Render-hosted comparative run are unverified. This is an explicitly authored asset pipeline, not a feature of the public carrier repository.

## Producer, consumer and repair

Pinned `pkg/service/create.go:272–310` and `pkg/service/update.go:361–377` serialize the supported commands. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` decodes them, `apps/service.go:2935–2950` persists them, and operator `internal/controller/app_controller.go:1048–1064` passes them into build.Options. Shared exact readbacks and supplier-only controls show those layers work; no API, launcher or schema repair is needed.

`lego/operator/internal/build/native.go:283–320` assembles the selected base and existing Node/media/default helpers without pigz. The immutable selected Python 3.13 base has no executable. `native.go:404–407` exports the sole native recipe; `build.go:1176–1183,936–960` passes recipe/native inputs into BuildKit; `nativeresolve.go:53–120` selects bases and companions without a pigz supplier. Actual source recipe exports and selected-base real compression/application controls confirm the omission.

Provide reviewed immutable amd64/arm64 pigz with compatible required zlib/pthread/runtime-library closure before authored native builds. Preserve its output artifacts through the normal runtime/static publication path. A complete binary/library payload is required; no runtime pigz dependency or always-present runtime tool is part of this fix's acceptance. Review exact version/content/platform/distro/ABI/freshness and cache identities across bookworm/trixie and intended bases. The diagnostic apt install is not a final reviewed supplier. Preserve existing gzip/zlib/app libraries, project-local tools, explicit PATH/env, language companions/media/native-init/source-root/files/cache, and existing native runtime behavior. Avoid runtime downloads, a generic supplier framework, donor root-filesystem replacement, new API selectors and CLI forks.

One generator/preparer/resolver/BuildJob chain serves six primary runtimes/16 reviewed image lines and native static builds; give its base/phase/consumer audit a separate task. Preserve REST/GraphQL/MCP/Blueprint/dashboard source/runtime/commands/plan/env semantics and keep Dockerfile/prebuilt-image/CNB excluded. Document build-only availability and next-rebuild adoption in ADR060. PM open/blocked/done and targeted pigz build history have no matching supplier repair; HTTP compression and other named-tool findings do not supply this build executable. No non-goal or documented divergence applies.

Actual generated-image regressions must fail before/pass after for real pigz compression and independent exact gzip inflation/header/CRC32/ISIZE/output identity checks, including empty/binary/Unicode inputs and a meaningful supported utility failure. Verify the service or static publication serves the built bytes without requiring runtime pigz; audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original first Free services must become Live without an install line and pass artifact/HTTP/log/shared-readback/cleanup checks. Code or explicit-install controls alone do not meet DoD.

Local selected-base original/control exits 127/0 and three own-loopback HTTP checks pass; full generated native images are not locally executed. Only Python 3.13/Free web/the named gzip build is hosted. Other bases/architectures/languages/static/pre-deploy/browser, archive/zip/zlib modes, performance, full local generated images, physical teardown and serving SHA/OCI remain unverified. Product code unchanged; this schedules the repair.
