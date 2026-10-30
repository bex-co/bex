# Native Node asset builds lack jq and rsync

Why: normal build scripts cannot generate a JSON release manifest or copy local assets with utilities documented as supplied by the native environment.

**Severity:** major for each of two independently isolated omissions. **Ownership:** operator native-image assembly. **Estimate:**270m across7 tasks, with separate jq and rsync repairs. Functional CLI sweep419,2026-10-17. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source94e8cea37f4177c9092f8cd2a5a63a6eda695c0c; serving revision unknown. [Complete evidence](evidence/cli-sweep419/README.md).

## Two real failures, isolated in three stages

Both native clients create a fresh explicit Free Node24 web service through unchanged POST201/exit0. The authored app verifies exact original public source, program, own input manifest and assets, then the build executes:

```sh
jq -e -c '{release:.release,paths:[.assets[]|select(.publish==true)|.path]}' qa-assets-input.json > qa-assets-filtered.json
node qa-assets-app.cjs after-jq
rsync -a --delete --exclude='*.tmp' qa-assets-src/ dist/
node qa-assets-app.cjs build
```

Start is `node qa-assets-app.cjs serve`. Both original first builds fail missing-jq before manifest generation. Before observations report ENOENT for both named utilities. Both failed URLs404. Each client first adds only a jq install line. Both second builds prove the exact jq manifest (independently parsed in Node), then fail because rsync is missing; no asset-copy proof is emitted and URLs remain404. Adding only rsync to that same install line makes both third children Live, with source/program/data/start/Free/env otherwise unchanged.

The exact local asset copy includes three known files with Unicode and space-containing names, excludes the own draft and deletes an own stale output. Six real HTTP200 requests perform fresh external jq filters and rsync copies; four JSON outputs match independent manifests and complete file byte/hash maps at build/runtime, while two serve exact copied UTF8 HTML bytes. Six native app events and18 REST/GraphQL/MCP plus6 explicit-env readbacks agree. Both exact parents/all six deploy children/URLs are removed, baseline preserved and current quotas reconciled. Full native argv/stdout/stderr/exit/duration, redacted unchanged create wire, exact data, source hashes and cleanup are retained.

The public git-fixtures/basic/master6ecf0ef2c2dffb796033e5a02219af86ec6584e5 repository is a carrier; this is an explicitly authored asset app. [Render](https://render.com/docs/native-runtimes) includes jq and rsync at build and runtime. [jq's manual](https://jqlang.org/manual/v1.6/) and [rsync's manual](https://download.samba.org/pub/rsync/rsync.1) define the actual filter/local archive/exclude/delete behavior. Exact Render versions and a Render-hosted comparison are unverified. All rsync sources/destinations are local own fixture paths; no remote transfer, pre-existing file or external account is involved.

## Producer, consumer and repair

Pinned `pkg/service/create.go:272–310` serializes native commands and `pkg/service/update.go:361–377` carries supported updates. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` decodes them, `apps/service.go:2935–2950` persists them, and operator `internal/controller/app_controller.go:1048–1064` passes them to build.Options. Exact shared readbacks show transport/persistence work; no request or launcher repair is needed.

`lego/operator/internal/build/native.go:283–320` assembles selected language bases plus existing companion/media/default steps, without jq/rsync suppliers. The selected immutable Node24 image has neither executable. `native.go:404–407` is the sole recipe producer; `build.go:1176–1183,936–960` passes the recipe/native inputs to BuildKit; `nativeresolve.go:53–120` selects bases/companions without providing these tools. Actual default/complete-control recipe exports and selected-base executable/application runs reproduce both omissions.

Provide reviewed immutable amd64/arm64 jq and rsync tools with complete ABI-compatible library/data closure before authored build and retained for runtime/pre-deploy/native-static phases. Keep a separate implementation/regression obligation for each missing executable. Shared delivery can use one coherent reviewed layer when compatible, without a generic supplier framework or donor root-filesystem replacement. Audit bookworm/trixie and already-working base tools; record exact content/version/platform/freshness identities and relevant cache invalidation. Preserve explicit env/PATH, language/companion/native-init/FFmpeg, source-root/files/env/cache behavior and app libraries. The install controls are diagnostic, not reviewed final suppliers. No new product version selector, API field, CLI fork or forced language-parser/file-copy rewrite.

## Scope, dedupe and acceptance

One native generator/preparer/resolver/BuildJob chain serves6 primary runtimes/16 reviewed image lines and native static builds. Shared-scope and authoring checks have their own tasks. Keep Dockerfile/prebuilt-image/CNB excluded. Preserve REST/GraphQL/MCP/Blueprint/dashboard runtime/command/plan/env semantics; update ADR060 with adoption and actual tool limits.

Fail-before/pass-after regressions must execute real generated images with actual jq manifest generation, local rsync copies, independently verified Unicode/space filenames, exact bytes, excludes and stale-own-output removal. Prove actual runtime commands with network disabled, meaningful nonzero utility failures and affected base/architecture/companion/env/PATH/cache/static/excluded neighbors. After publication both unchanged native clients' original first Free services must become Live without any install line and pass exact source/program/data/manifest/copy/HTTP/log/shared-readback/cleanup checks. Counterfactual controls or code alone do not meet DoD.

All PM open/blocked/done records and targeted jq/rsync native-image histories were searched with no matching repair. m91/m93/m94/m95/m96/m97 provide different native utilities/libraries; none provides these two executables. No non-goal or documented divergence found. Local selected-base outcomes127/127/0 and full own-loopback app checks pass; no full generated image is locally executed. Only Node24/Free web/the named filter/local file copy is hosted. Other bases/architectures/languages/static/pre-deploy/browser, full archive/remote modes, hosted filesystem inspection, physical teardown and serving SHA/OCI remain unverified. Product code unchanged; this schedules two related repairs.
