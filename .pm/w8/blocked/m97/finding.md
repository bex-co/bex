# Native PDF rendering cannot find Ghostscript

Why: ordinary PDF-thumbnail and document-conversion services fail their builds or runtime conversions despite using a tool documented as supplied by the native environment.

**Severity:** major. **Attribution:** operator shared native-image utility assembly. **Estimate:**215m across6 tasks. Functional CLI sweep418,2026-10-17. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source1845a897afb1ef1e50becd8ed54c97e10c771953; actual serving revision unknown. [Complete evidence](evidence/cli-sweep418/README.md).

## Exact reproduction and counterfactual

Both unchanged native clients create fresh explicit Free Python3.13 web fixtures, POST201/exit0. The authored build reconstructs an exact small stdlib app/reference, writes the own471-byte PDF, verifies original public source/program/PDF hashes, then executes:

```sh
gs -q -dBATCH -dNOPAUSE -sDEVICE=png16m -r72 -dFirstPage=1 -dLastPage=1 -sOutputFile=qa-gs-build.png qa-gs.pdf
```

Startup is `python -u qa-gs-app.py serve`. Both original first children become build_failed with gs not found; before-command metadata confirms its absent executable locator. Both original URLs404. Full native argv/stdout/stderr/exit/duration, exact redacted unchanged request/response and file bytes/hashes are retained.

Each native client changes only one build line installing Ghostscript. Both native updates0 and replacement children Live on the same source. Build and runtime render the same self-authored one-page PDF, containing only solid red/blue rectangles, into exact2x1 RGB8 PNG pixels. Six actual HTTP200 conversions pass: four JSON outputs include independently decoded build/runtime PNGs; two direct PNGs independently pass CRC/header/zlib/filter/dimensions and exact red/blue pixels. Source/program/PDF hashes and actual gs versions agree; six native request events correlate each individual output hash. Twelve REST/GraphQL/MCP and four explicit-env readbacks match before/after. Both exact parents/four deploy children/URLs/local resources are removed and baseline/current-inventory/quota checks pass.

[Render](https://render.com/docs/native-runtimes) includes ghostscript during native build and runtime. [Ghostscript](https://ghostscript.readthedocs.io/en/latest/Use.html) supports this ordinary PDF/output-device/resolution invocation. Selected-base local original127/install0 and full authored-app/HTTP prerequisites independently reproduce the missing executable and exact pixels, with actual gs10.00.0. They are not full generated-image local executions. Exact Render version and an authenticated Render-hosted comparison remain unverified.

## Traced boundary and repair

Pinned `pkg/service/create.go:272–310` serializes native commands; `pkg/service/update.go:361–377` supports the build-only update. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` accepts them, `apps/service.go:2935–2950` stores them, and operator `internal/controller/app_controller.go:1048–1064` passes them to build.Options. Exact shared metadata and real source/runtime execution prove persistence. No request repair or launcher fork is indicated.

`lego/operator/internal/build/native.go:283–320` assembles pinned language images and existing companion/media/default steps, but no Ghostscript. `native.go:404–407` exports the sole recipe; `build.go:1176–1183,936–960` passes it and native inputs to BuildKit. `nativeresolve.go:53–120` selects bases/companions without providing gs. `nativeffmpeg.go:27–40` supplies static FFmpeg executables, not Ghostscript. Actual BuildJob/resolver exports and selected Python-base execution reproduce the omission.

Supply a reviewed immutable amd64/arm64 Ghostscript executable plus complete ABI-matched libraries, initialization/resource/font/color data and helpers needed for the promised tool. Place it before source/authored build and retain it at runtime/pre-deploy/native-static builds. Review bookworm/trixie compatibility, explicit GS_LIB/locale/PATH behavior, version/content/platform identity and freshness/cache invalidation. Copying only a dynamic gs executable, a broken symlink or a donor root filesystem is insufficient. The apt control is diagnostic, not a final reviewed supplier or tenant-side repair policy.

Preserve existing language/JS/Bun/Poetry/uv selections, native-init/FFmpeg, app libraries, explicit environment/PATH, roots, files/env/cache revisions and Dockerfile/prebuilt-image/CNB exclusions. Reuse truly shared delivery mechanics with m91/m93/m94/m95/m96 where useful, retaining separate consumers and acceptance. No new product selector, schema field, launcher change or unrelated utility bundle.

## Scope, dedupe and acceptance

One generator/preparer/resolver/BuildJob chain serves6 primary runtimes/16 reviewed base lines and native static builds. Audit every intended distribution/architecture/phase and siblings. Shared REST/GraphQL/MCP/Blueprint/dashboard native authoring contracts remain consistent.

Regressions must execute actual generated artifacts fail-before/pass-after for real PDF rendering, independently verify dimensions/pixels in build and network-disabled runtime, preserve nonzero renderer failures, and exercise required resource/font/color lookup with a harmless known text page. Verify intended architectures/bases, companions, env/PATH/cache/static/excluded neighbors and document actual version/format limits. After publication both unchanged native clients' fresh original first Free PDF services must become Live without install lines and pass source/program/PDF/image/native-log/shared-readback/cleanup checks. Current controls and code alone do not meet DoD.

All PM open/blocked/done records and targeted native-image history were searched: no Ghostscript item/fix. M95 concerns libvips's dynamic image library, m93 Pandoc documents and m82 FFmpeg; none supplies this executable. Positive ImageMagick PPM conversion406 did not certify PDF/Ghostscript. No non-goal or intentional divergence found. Only Free Python3.13/one rectangle-only PDF/png16m is hosted. Fonts/text/multipage/other output modes, other bases/architectures/languages/static/pre-deploy/browser, full local generated-image execution and actual serving SHA/OCI remain unverified. Product code unchanged; this schedules the repair.
