# w4 · m213 — Supply JavaScript companions in native Go, Rust and Elixir

**Worker:** worker4 **Goal:** Node, npm, Yarn and pnpm execute during native Go/Rust/Elixir builds and in the started service, while each primary toolchain and its version controls remain intact. **Status:** done 2026-10-29 — remaining live checks replayed on pin `d66f96c06991`: a started Go-compiler probe alongside the four JS tools, primary-only Go/Rust/Elixir controls live, and an unsupported NODE_VERSION refused by name before the build (loopx w4).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Extend the shared companion path to Go and Rust — **DONE** | 25m | — |
| t002 | Verify Elixir image compatibility and enable its companion — **DONE** | 35m | t001 |
| t003 | Audit shared builder consumers and existing primary controls — **DONE** | 25m | t001, t002 |
| t004 | Render parity and version-contract documentation — **DONE** | 15m | t003 |
| t005 | Simplify the milestone changes — **DONE** | 10m | t004 |
| t006 | Verify real tool execution and regression coverage — **DONE** | 25m | t004, t005 |
| t007 | Closeout after live acceptance and cleanup | 15m | t006 |

Total: **7 tasks, 150m**. Filing only; no product fix is implemented by this promotion.

## Definition of done

- Replay each recorded Go, Rust and Elixir build on an owned Free web service. The first deployment and a fresh-page same-settings manual deployment reach Live; Node/npm/Yarn/pnpm version commands all execute before the unchanged primary build/check. The started probes report successful versions for all four tools and the expected primary Go, Rust or Elixir/OTP version. Verify the runtime URL inside and outside the browser and correct deploy terminal states. The original primary-only controls remain successful.
- Use the existing reviewed companion supplier and Node selector precedence for Go/Rust/Elixir. Unsupported secondary Node requests fail by name before the authored build; unchanged inputs preserve the cache identity and changed selectors change it. Primary Go paths, Rust rustup signals and Elixir/OTP selection and compatibility refusals remain correct. No new runtime/API field or package installer is introduced.
- The shared-consumer audit accounts for the one native image-generation/resolution pipeline and its service/Blueprint entrypoints. Ruby/Python companions, primary Node, optional Bun, Python Poetry, authored PATH/env/command handling and default-static Node behavior retain their existing contracts. Verify the copied tools actually run on reviewed primary images; generated Dockerfile text alone is insufficient.
- Keep the version/architecture matrix explicit. Production evidence covers default Go/Rust/Elixir Free web. The local cross-base execution covers only Node24 in Elixir1.18 on arm64. Other Node/Elixir lines, amd64 copied-tool execution, static/worker/private/cron variants and secondary selectors require verification without being labeled already live-tested. Update ADR060/ADR018's runtime exclusions when the repair exists.
- Pass relevant checks and the ship gates. After deployment, replay the three owned journeys and clean their resources and exact sessions. Mark done only when the observed end state holds; then move tasks and this milestone under done/ and synchronize every status.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, a676/a692/a693, 2026-10-16 UTC; [combined finding](finding.md), [archived inbox321](../../done/321.md) and the linked complete API/runtime/image receipts. Credentials passed only through the QA login helper.
- **Goal linkage:** ADR008 usable hosting, ADR018 native-runtime parity and ADR060 D7 reviewed image assembly.
- **Expected outcome:** a native backend can run a JavaScript asset step without switching its primary runtime or manually installing the common tools.
- **Why now:** two native failures per runtime and three Live primary-only controls isolate the shared exclusion. Elixir adds three primary lines and a different Debian base; this expands the original sub-hour note into a multi-task image/selector/regression repair. Promote the existing owner rather than create a duplicate.
- **Render parity included:** the tenant-visible build/runtime tools change. [Render's native-runtime tools contract](https://render.com/docs/native-runtimes#tools-and-utilities) supplies the target. Existing reviewed pin/patch divergences remain explicit; exact Render tool versions and authenticated deployments were not compared.
- **Dedupe:** 79 active/blocked milestone titles, whole-board matches, DO_NOT_DO, refreshed main and recent/targeted history. w8/m77 supplies Ruby/Python and explicitly excludes these three runtimes; w4/320 separately owns primary Node26 Yarn. This milestone replaces the open w4/321 scope. All three QA fixtures and their sessions are cleaned.

## Implementation record (2026-10-16)

- **t001/t002:** Go, Rust and Elixir joined `nativeJSCompanionRuntimes` (`lego/operator/internal/build/nativeresolve.go`), the one eligibility seam in front of the reviewed `nativeJSToolLines` supplier. There is no new installer, API field or per-client path. Node follows primary Node's precedence and refusals. Go, Rust (rustup signals) and Elixir (`ELIXIR_VERSION`, OTP refusals) primary selection is untouched. Base check: the Node images, Go, Rust and Elixir 1.17 are bookworm (glibc 2.36); Elixir 1.18/1.19 are trixie (glibc 2.41).
- **t003 audit:** this is the single caller chain from finding.md (`ResolveNativeBuild` ← `cmd/native-resolve`), allowlisted by runtime. Ruby/Python and primary Node stay controls, and default static builds still use primary Node. Explicitly native static/worker/private/cron services with these runtimes converge on the same path; they were not executed.
- **t004:** ADR060 gained a w4/m213 entry, and ADR018's runtime row now names all five runtimes. Render tool versions were not compared.
- **t006:** `TestResolveNativeBuildAddsJSToolsToNonNodeRuntimes` covers every line of all five runtimes: insertion order, NODE_VERSION=22 selection, the unsupported NODE_VERSION=12 refusal leaving the Dockerfile untouched, and no second node binary for primary Node. `TestNativeImageRunsJSToolsBesidePrimaries` (Docker, arm64) builds and runs Go 1.24, Rust 1 and Elixir 1.17/1.18/1.19 plus the Ruby/Python controls. node/npm/Yarn/pnpm versions and package-script runs pass at build and runtime beside the expected primary versions. With the old allowlist it fails with `node: command not found`. Operator `make test` and `make lint` (all four modules) pass. **Not executed:** amd64, Node 22/26 with these primaries, and non-web service types.

## Live default-runtime controls — a762/a763/a764, 2026-10-16

[Complete native create/deploy requests, version-log reads, public HTTP controls and cleanup](evidence-a762-a764.json). These are positive equivalent default-runtime probes, not a new bug or completion of the full matrix. Each owned Free web service used public `bex-co/bex`, commit `2b046a09ae3b227dab0edba51fd68a038745503e`, root `examples/hello-go`, explicit authored build/start commands and auto-deploy off. Go compiled the example server; Elixir/Rust wrote and compiled small version-reporting HTTP fixtures inside the build. Their complete sources are embedded in the create requests.

| Runtime / pass | First deploy Live (UTC) | Fresh390px unchanged manual deploy Live (UTC) | Primary observed | JS tools at build and runtime |
| --- | --- | --- | --- | --- |
| Go / a762 | 18:44:54.837715 | 18:48:24.821094 | go1.24.13 linux/amd64 at build; real Go HTTP server | Node v24.22.0, npm11.19.1, Yarn1.22.22, pnpm10.34.7 |
| Elixir / a763 | 18:56:09.891883 | 18:59:40.492765 | Elixir1.18.4 / OTP28 in the started server | Same four versions |
| Rust / a764 | 19:06:09.928078 | 19:09:10.037558 | rustc1.99.1 (`58cf8f9c3`, 2026-10-13) in the started server | Same four versions |

Both builds per runtime emitted fresh authored version output, and each second runtime instance emitted its own output. Browser and curl returned the expected marker/version body per service. Elixir's second curl precedes API Live settlement by less than a second; the later browser GET is its post-settlement control. No separate post-settlement external curl is claimed for that second release. Initial503s occurred before first Ready and recovered; they are not new findings. Captured repeat mobile pages had width390 and no console errors.

All three services and six deploy records return404, with all seven baseline resource-ID sets restored per pass. Exact Apps, runtime pods, build jobs and build pods are gone and public URLs return404; Rust's final pod absence is separately recorded at19:10:51 UTC. Each exact session is revoked (whoami401, zero cookies, one blank page), and owned session handles are absent.

**Remaining t007 limits:** Go's compiler version was measured in both builds, not by a started Go-compiler probe; its compiled server and four JS startup commands passed. Original primary-only controls, secondary-selector refusal and alternate primary/Node lines were not replayed. Only the Go build reports linux/amd64; no new architecture matrix is claimed. Retain the existing local all-line gate and Elixir arm64 evidence as their own evidence. Rust1.99.1 is a dated observation, not a requirement to freeze the historical1.99.0 patch. No task or milestone is marked done by this partial update.
