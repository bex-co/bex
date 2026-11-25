# w8 · m99 — Supply TypeScript for native builds and runtime

**Worker:** worker8 **Goal:** ordinary strict TypeScript builds emit a working JavaScript service using the documented compiler without tenant installation workarounds. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed immutable native TypeScript compiler](done/t001.md) — **DONE** | 75m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit compiler delivery across shared bases and native phases](done/t002.md) — **DONE** | 40m | t001 |
| t003 | [Verify TypeScript native parity across authoring surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify compiler supplier changes](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test actual generated-image TypeScript compilation and errors](done/t005.md) — **DONE** | 55m | t003 |
| t006 | [Accept original first TypeScript deployments and close milestone](done/t006.md) — **DONE** | 20m | t004, t005 |

**Total:**6 tasks/225m. [Researched finding](finding.md) · complete evidence (sweep 420, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch compiler payload with compatible executable/libraries/standard-library/data closure is usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests verify strict interface/enum compilation and emitted-JavaScript execution with independent source/output/data identities, reject an actual type error without emitting output, and prove network-disabled runtime compilation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged native clients' original fresh first Free services become Live without a compiler install and pass source/typed-result/HTTP/log/shared-readback/exact cleanup/baseline/current-quota checks. Limits are explicit; diagnostic controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep420,2026-10-17: both unchanged native creates POST201/exit0, both original first builds fail missing-tsc, both single pinned-compiler-install replacements Live with six real emitted-JavaScript/typed-result responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting compiler commands should work as documented.
- **Outcome:** Native TypeScript builds and runtime scripts can use an actual compiler without changing CLI requests or adding tenant install workarounds.
- **Why now:** Real independent native first failures and compiler-only controls isolate missing compiler delivery; neither source nor board/history has an existing supplier repair. Native package/data layout and shared-base adoption require a deliberate reviewed implementation.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m99；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $land 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Unblocked: the gate holds, and acceptance runs on the digest production serves — 2026-11-19 (loopx session)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 15:05Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z), so no roll could start before 17:30Z. Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m97 and w8/m98 accepted on. That image's SPDX attestation, fetched again from the registry at 15:06Z, lists `typescript` `7.0.2`, `@typescript/typescript-linux-x64` `7.0.2` (the native compiler from `microsoft/typescript-go`) and the private `node` `24.22.3` under `/opt/bex/native-node-tools` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)).
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004); give the maintained cohort more TypeScript checks and a `missing-typescript` omission control (t005).

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, TypeScript 7.0.2 on a private Node 24.22.3). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Node 24. The original strict `tsc` compiled the interface and enum source and the emitted JavaScript passed the frozen validator with the exact Unicode and space labels, total 10 and warm/cool/warm tones. Both builds emitted the same `af1acfde…` bytes. Three runtime requests per service called `tsc` again and passed the frozen oracle, and the native app logs show every request ([t006](done/t006.md)).
- **Supplier.** npm's current `latest` TypeScript, integrity-locked with its native platform package and standard library, run by `/usr/bin/tsc` on a private Node, so the app's Node and npm are untouched ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get `tsc` at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `typescript` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now runs m99's own source outside the original program against its frozen emit and an independent result, plus exit statuses, the packaged standard library, project and `--build` modes, ES modules with declarations and source maps, the private Node with no `node` on `PATH`, and a project-local `tsc` ahead of the platform's in npm scripts. This runs at compute build, network-off runtime and static build. A new `missing-typescript` omission fails the original build with `tsc: command not found`, the sweep 420 failure ([t005](done/t005.md)). Before shipping these ran on a linux/arm64 replica of the image's `node-tools` stage. Their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other primaries on production rely on the families runs for the same digest.
  - No language server, watch mode or JavaScript compiler API is accepted.
- **Cleanup.** Both fixtures and their URLs are gone. The run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
