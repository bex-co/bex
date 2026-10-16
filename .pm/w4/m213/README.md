# w4 · m213 — Supply JavaScript companions in native Go, Rust and Elixir

**Worker:** worker4 **Goal:** Node, npm, Yarn and pnpm execute during native Go/Rust/Elixir builds and in the started service, while each primary toolchain and its version controls remain intact. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Extend the shared companion path to Go and Rust | 25m | — |
| t002 | Verify Elixir image compatibility and enable its companion | 35m | t001 |
| t003 | Audit shared builder consumers and existing primary controls | 25m | t001, t002 |
| t004 | Render parity and version-contract documentation | 15m | t003 |
| t005 | Simplify the milestone changes | 10m | t004 |
| t006 | Verify real tool execution and regression coverage | 25m | t004, t005 |
| t007 | Closeout after live acceptance and cleanup | 15m | t006 |

Total: **7 tasks, 150m**. Filing only; no product fix is implemented by this promotion.

## Definition of done

- Replay each recorded Go, Rust and Elixir build on an owned Free web service. The first deployment and a fresh-page same-settings manual deployment reach Live; Node/npm/Yarn/pnpm version commands all execute before the unchanged primary build/check. The started probes report successful versions for all four tools and the expected primary Go, Rust or Elixir/OTP version. Verify the runtime URL inside and outside the browser and correct deploy terminal states. The original primary-only controls remain successful.
- Use the existing reviewed companion supplier and Node selector precedence for Go/Rust/Elixir. Unsupported secondary Node requests fail by name before the authored build; unchanged inputs preserve the cache identity and changed selectors change it. Primary Go paths, Rust rustup signals and Elixir/OTP selection and compatibility refusals remain correct. No new runtime/API field or package installer is introduced.
- The shared-consumer audit accounts for the one native image-generation/resolution pipeline and its service/Blueprint entrypoints. Ruby/Python companions, primary Node, optional Bun, Python Poetry, authored PATH/env/command handling and default-static Node behavior retain their existing contracts. Verify the copied tools actually run on reviewed primary images; generated Dockerfile text alone is insufficient.
- Keep the version/architecture matrix explicit. Production evidence covers default Go/Rust/Elixir Free web. The local cross-base execution covers only Node24 in Elixir1.18 on arm64. Other Node/Elixir lines, amd64 copied-tool execution, static/worker/private/cron variants and secondary selectors require verification without being labeled already live-tested. Update ADR060/ADR018's runtime exclusions when the repair exists.
- Pass relevant checks and the ship gates. After deployment, replay the three owned journeys and clean their resources and exact sessions. Mark done only when the observed end state holds; then move tasks and this milestone under done/ and synchronize every status.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, a676/a692/a693, 2026-10-16 UTC; [combined finding](finding.md), [archived inbox321](../done/321.md) and the linked complete API/runtime/image receipts. Credentials passed only through the QA login helper.
- **Goal linkage:** ADR008 usable hosting, ADR018 native-runtime parity and ADR060 D7 reviewed image assembly.
- **Expected outcome:** a native backend can run a JavaScript asset step without switching its primary runtime or manually installing the common tools.
- **Why now:** two native failures per runtime and three Live primary-only controls isolate the shared exclusion. Elixir adds three primary lines and a different Debian base; this expands the original sub-hour note into a multi-task image/selector/regression repair. Promote the existing owner rather than create a duplicate.
- **Render parity included:** the tenant-visible build/runtime tools change. [Render's native-runtime tools contract](https://render.com/docs/native-runtimes#tools-and-utilities) supplies the target. Existing reviewed pin/patch divergences remain explicit; exact Render tool versions and authenticated deployments were not compared.
- **Dedupe:** 79 active/blocked milestone titles, whole-board matches, DO_NOT_DO, refreshed main and recent/targeted history. w8/m77 supplies Ruby/Python and explicitly excludes these three runtimes; w4/320 separately owns primary Node26 Yarn. This milestone replaces the open w4/321 scope. All three QA fixtures and their sessions are cleaned.
