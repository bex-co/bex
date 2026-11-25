# w8 · m101 — Supply Webpack for native production bundles

**Worker:** worker8 **Goal:** ordinary native production bundling emits a working JavaScript service with a supplied noninteractive compiler/CLI, without tenant installation workarounds. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed Webpack, compatible CLI and dependency closure](done/t001.md) — **DONE** | 75m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit bundler delivery across native bases and phases](done/t002.md) — **DONE** | 40m | t001 |
| t003 | [Verify Webpack native parity across authoring surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify bundler supplier changes](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test actual generated-image production bundles and failures](done/t005.md) — **DONE** | 55m | t003 |
| t006 | [Accept original first bundled services and close milestone](done/t006.md) — **DONE** | 20m | t004, t005 |

**Total:** 6 tasks / 225m. [Researched finding](finding.md) · complete evidence (sweep 422, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch Webpack payload with compatible noninteractive CLI and complete dependency/data closure is usable through intended native build/pre-deploy/runtime/static phases. Actual generated-image tests verify multi-module production bundling, emitted-bundle execution, independent Unicode/data/module results, compiler statistics and source/output identities; reject unresolved modules and prove noninteractive network-disabled runtime compilation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original first Free services become Live without an install line and pass source/module/bundle/results/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep422, 2026-10-17: both unchanged native creates POST 201/exit 0, both original first builds fail missing-webpack, both single pinned core/CLI-install replacements Live with six actual generated-bundle HTTP responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): normal native bundling commands should work as documented.
- **Outcome:** Native builds can bundle their modules and run the output without rewriting requests or adding tenant install workarounds.
- **Why now:** Actual first failures and supplier-only controls isolate missing compiler/CLI delivery; no existing source/board/history repair supplies it. Full dependency closure, engine compatibility and shared-base adoption require deliberate review.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior through REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m101；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
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
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 16:01Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z), so no roll could start before 17:30Z. Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m97 to m100 accepted on. That image's SPDX attestation, fetched again from the registry at 16:02Z, lists `webpack` `5.112.2`, `webpack-cli` `7.2.3` (with its own `commander` `14.0.3`) and the private `node` `24.22.3` under `/opt/bex/native-node-tools`. Every one of the 102 packages `deploy/native-base/package-lock.json` installs on linux/amd64 is there at its locked version ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)).
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004); give the maintained cohort more Webpack checks and a `missing-webpack` omission control (t005).

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, webpack 5.112.2 with webpack-cli 7.2.3 on a private Node 24.22.3). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Node 24. The original `webpack` command bundled the two authored modules for Node in production mode, its statistics reported 5.112.2 with no errors or warnings, and the bundle passed the frozen validator with the exact Unicode and space labels, total 10 and sorted quantities 2, 3, 5. Both builds emitted the same `8fd8eb19…` bundle. Three runtime requests per service ran the bundle and `webpack` again and passed the frozen oracle, and the native app logs show every request ([t006](done/t006.md)).
- **Supplier.** npm's current `webpack` with the CLI the lock carries and the whole integrity-locked dependency closure, minimizer included, run by `/usr/bin/webpack` on a private Node, so the app's Node and npm are untouched ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get `webpack` at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `webpack` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now runs m101's own sources outside the original program against the frozen production bundle and an independent result, plus exit statuses, a `webpack.config.mjs` with `--env`, a dynamic-import chunk beside a JSON module, the filesystem cache inside the project, source maps, the private Node with no `node` on `PATH`, a project-local `webpack` ahead of the platform's in npm scripts, and `webpack serve` installing nothing without a terminal. This runs at compute build, network-off runtime and static build. A new `missing-webpack` omission fails the original build with `webpack: command not found`, the sweep 422 failure ([t005](done/t005.md)). Before shipping these ran on a linux/arm64 replica of the image's `node-tools` stage. Their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other primaries on production rely on the families runs for the same digest.
  - `webpack --version` lists only the calling project's packages, a configuration that calls `require('webpack')` needs the project's own webpack, and only the `webpack` command is supplied; the first two are the same under a global npm install. No dev server, loaders, plugin collections or browser targets are accepted ([t003](done/t003.md)).
  - `webpack-cli` 7.3.1 has been npm's `latest` since 2026-10-30; the image carries 7.2.3, `latest` when the lock was cut. Moving it is a new image digest for a later image refresh.
- **Cleanup.** Both fixtures and their URLs are gone, and the baseline is restored. The run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
