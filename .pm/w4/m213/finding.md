# Native JavaScript companion exclusions — Go, Rust and Elixir

Why: the primary compiler/interpreter works, but a normal JavaScript asset step fails because the shared native builder supplies those tools only to Ruby/Python.

**Major.** This combines the preserved [a676/a692 Go/Rust finding](../done/321.md) with a693's newly measured Elixir scope. [Go receipts](../evidence/321-native-go-js-tools.json), [Rust receipts](../evidence/321-native-rust-js-tools.json) and [Elixir receipts, exact source, image execution and cleanup](../evidence/321-native-elixir-js-tools.json) are durable. Product code is unchanged.

## Observations and controls

| Runtime | Repeated native failure | Primary-only Live control |
| --- | --- | --- |
| Go, a676 | go1.24.13 succeeds; two builds fail on missing node/exit 127 | HTTP200; Go works; all four JS executables unavailable |
| Rust, a692 | rustc1.99.0 succeeds; two builds fail on missing node/exit 127 | HTTP200; rustc works; all four JS process attempts unavailable |
| Elixir, a693 | Elixir 1.18.4/OTP28 succeeds; two builds fail on missing node/exit 127 | HTTP200; Elixir/OTP works; all four JS process attempts unavailable |

For each control, remove only the four JS version assertions from the authored build and keep its source, primary runtime, start command, root and port. npm/Yarn/pnpm build assertions are never reached after the first failed node check; their runtime failures are measured separately. These are Free web services with no authored PATH/env-group changes. Exact sources/commands are in the receipts.

The Elixir fixture is `srv-db8u4endbg8c73dghf1g`, `qa-20261016-a693-elixir-js`, public bex-co/bex/main, checkout/source `71ce7fa28df475ea23fc2a5b0fbb591fc847edb1`, root examples/hello-go, port 8080. Its custom build writes an Elixir script into the disposable context; no Go build runs. The dependency-free script uses Erlang's standard TCP library. `+S 2:2` bounds its schedulers. Native create defaulted Auto-Deploy on; it was disabled before any repository push.

The build runs the primary version command, then node/npm/Yarn/pnpm versions, then syntax-checks the authored Elixir script. Start interprets that same script. Initial deploy `dep-db8u4endbg8c73dghf20` prints Elixir 1.18.4 at **08:39:15Z**, then missing node; terminal build_failed **08:39:33.399356Z**, exit 127. A fresh-page unchanged manual retry `dep-db8u5ovdbg8c73dghf40` repeats both at **08:40:47Z** and terminates build_failed **08:41:02.378630Z**, exit 127. Native filtered-pane screenshot `qa-a693-elixir-retry-node-error.png` was viewed; exact bounded compiler/error queries preserve the evidence. The screenshot's older-page loading state is not a new finding.

After removing only the JS assertions, manual control `dep-db8u6mi9cl7s73ae4d6g` reaches Live **08:44:43.077985Z**. Public GET **08:44:43.460Z** and independent curl **08:45:12.729774–13.320271Z** return 200 and:

```text
QA_A693_ELIXIR
elixir=1.18.4
otp=28
node=unavailable
npm=unavailable
yarn=unavailable
pnpm=unavailable
```

The Build Command save also opened config_change `dep-db8u6lndbg8c73dghf4g`, automatically superseded/canceled by the explicit manual control. Do not count it as Live. Final API is Running/free/nativeElixir, Auto-Deploy false, rev-5, no unapplied/applying changes; four deploy rows are two Build Failed, one Canceled and one Live. Image-transfer/queue waits progressed normally.

## Shared cause and concrete target

`lego/operator/internal/build/nativeresolve.go:83–91,114–146` gates the reusable JS supplier through `nativeJSCompanionRuntimes`, containing only Ruby/Python. `nativepnpm.go:42–46` supplies primary Node, while `native.go:83–113,243–269` supplies primary Elixir/Go/Rust and generates the authored build. Add the three newly exercised runtimes at this same eligibility/image seam, preserving the existing reviewed Node binary/modules, Yarn1.22.22 and pnpm10.34.6 supplier. Do not add per-API workarounds or a new installer.

Node selection must use existing NODE_VERSION → .node-version → .nvmrc → engines precedence/refusal behavior. This makes previously ignored secondary Node signals effective; document it. Primary Go stays intact, Rust keeps rustup/RUSTUP_TOOLCHAIN/rust-toolchain behavior, and Elixir keeps ELIXIR_VERSION/ERLANG_VERSION selection and incompatible-OTP refusals (`nativeversion.go:388–407`). Existing wire types already express runtime and commands; the observed native requests and running primary versions confirm that route.

**Compatibility is not a shared-distro assumption.** The exact default Elixir image `elixir:1.18@sha256:f3cc81b7f0f897763f7c74e8f526ddaf18b68f0cdbea6dd65b58f2c5c5629fbd`, executed locally on arm64, reports Debian 13/trixie, Elixir 1.18.4/OTP28 and no four JS tools. Go/Rust and reviewed Node are Bookworm. The exact reviewed Node24 binary from `node:24-bookworm@sha256:3d27e5c11e5786e309ec3e03f93ae536eb36e6e5eb3714d5eb3300a36157add0` executes as v24.21.0 beside Elixir 1.18.4 in that pinned image with read-only root, network disabled and both commands required to exit0. The temporary source container is removed. This proves one local binary pair, not complete npm/Yarn/pnpm execution, other lines or amd64 compatibility. Verify the reviewed combinations before enabling them; preserve the primary base and make any required dependency change explicit.

`nativeversion_test.go:510–522` currently forbids node in Go/Rust/Elixir. Replace those stale negative cases with positive execution/selection coverage. Extend `native_test.go:1056`'s real-image gate, retaining Ruby/Python controls. Check actual authored builds and runtime tools, selectors, cache identity and existing good PATH/quoting/Bun/Poetry behavior; do not settle for generated-text assertions.

## Blast radius, parity and limits

Production census remains one nativeNodeTools call in nativeDockerfile, one nativeDockerfile call in nativeBuildPreparer, one preparer call in build.go:1126 and one ResolveNativeBuild call in cmd/native-resolve/main.go:46. The global repair is allowlisted to Go/Rust/Elixir eligibility; existing Ruby/Python and primary Node remain controls. Web/private/worker/cron and explicitly native static builds converge here. Default static builds use Node. Image/Dockerfile sources and managed Postgres/Key Value are outside the repair. Only Free web was live-tested. REST/GraphQL/MCP/dashboard/Blueprint runtime inputs still meet the same builder; no field/error taxonomy change is needed.

[Render's current native-runtime tool list](https://render.com/docs/native-runtimes#tools-and-utilities) includes these JS tools at build and runtime. Exact Render versions and authenticated deployments were not measured. Update ADR060/ADR018's explicit exclusions after repair, preserving reviewed-pin/patch differences and w4/320's separate primary-Node26 Yarn scope.

Whole-board search found 27 Elixir/tool matches in 20 files, plus 79 active/blocked milestone titles, DO_NOT_DO and recent/targeted refreshed-main history. w4/321 is the existing predicate owner. w8/m77's completed tasks intentionally retained Go/Rust/Elixir; its Ruby/Python closeout stays separate. The old w8/m61 Elixir PATH/file probe runs no runtime commands and does not claim JS availability. This is new coverage of the excluded runtimes, not a claimed regression of the Ruby/Python implementation.

Unverified: repaired tool execution, other primary/Node lines, secondary selectors, amd64 copied-tool execution and other architectures, non-web/static variants, full cache/PATH matrix and authenticated Render comparison. The local Node24 pair does not close that matrix. No product fix was implemented.

## Cleanup

Go/Rust cleanup remains in their full receipts. Elixir native delete succeeded **08:45:31.084Z**; service and all four recorded deploy reads404, all seven baseline sorted ID sets restored **08:45:41.976Z**. Owned pods absent and public URL404. Exact logout exited0; browser401/cookies0/pages1/blank **08:45:57.379Z**, owned handles removed and shared handle checked only by exact stamp. Local compatibility execution containers used --rm and the created source container was removed. Build-plane Jobs were not separately inventoried. Console errors0; one aborted live-log subscription during navigation is retained.
