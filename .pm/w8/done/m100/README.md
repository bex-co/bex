# w8 · m100 — Supply SWIG for native C-binding builds

**Worker:** worker8 **Goal:** ordinary Python binding builds generate, compile and import their C extension using the documented native SWIG tool without tenant installation workarounds. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native SWIG with library and typemap data](done/t001.md) — **DONE** | 60m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit SWIG delivery across shared native bases and phases](done/t002.md) — **DONE** | 35m | t001 |
| t003 | [Verify native SWIG parity across authoring surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify native binding-tool supplier changes](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test generated-image binding generation, compilation and import](done/t005.md) — **DONE** | 50m | t003 |
| t006 | [Accept original first C-binding deployments and close milestone](done/t006.md) — **DONE** | 20m | t004, t005 |

**Total:**6 tasks/200m. [Researched finding](finding.md) · complete evidence (sweep 421, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch SWIG payload with compatible executable/libraries/include/language/typemap data is usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests verify wrapper/proxy generation, gcc compilation/import and exact independent C arithmetic/Unicode results and source/generated/extension identities, reject an invalid interface and prove data-tree availability and network-disabled runtime generation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original fresh first Free Python services become Live without a SWIG install and pass source/interface/extension/result/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep421,2026-10-17: both unchanged native creates POST201/exit0, both original first builds fail missing-swig, both one-line SWIG-install replacements Live with six real compiled-C-extension HTTP responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular native binding build tools should work as documented.
- **Outcome:** Native Python C-binding builds can generate their wrappers and run the resulting extension without rewriting commands or CLI requests.
- **Why now:** Real first failures and SWIG-only controls isolate missing delivery; source/board/history has no existing supplier repair. Required SWIG data trees and shared-base adoption need deliberate reviewed delivery.
- **Render parity:** Included because native tool contents affect tenant builds/runtime across REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m100；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
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
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 15:32Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z), so no roll could start before 17:30Z. Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m97, m98 and m99 accepted on. That image's SPDX attestation, fetched again from the registry at 15:34Z, lists Debian's `swig` and `swig4.0` `4.1.0-0.2` (`pkg:deb/debian/swig4.0@4.1.0-0.2?arch=amd64&distro=debian-12.15`), whose 776 files include `/usr/bin/swig4.0` and the `/usr/share/swig4.0` library with `swig.swg`, `swigrun.swg`, `typemaps/` and `python/` (`python.swg`, `pyruntime.swg`, `std_string.i`), beside `gcc-12` and `python3.11-dev` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)).
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004); give the maintained cohort more SWIG checks and a `missing-swig` omission control (t005).

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, Debian's SWIG 4.1.0). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Python 3.13. The original `swig -python` generated the wrapper and proxy, `gcc` compiled the extension against Python 3.13.18, and the build proof passed the frozen validator with weighted sums 13, 13 and 0 and the exact `Crème brûlée & 李`. Both builds generated the same `a5594a40…` wrapper and `25875c83…` proxy and compiled the same `39b80621…` extension. Three runtime requests per service imported it again and passed the frozen oracle, and the native app logs show every request ([t006](done/t006.md)).
- **Supplier.** Debian 12's `swig`/`swig4.0` `4.1.0-0.2`, whose one package carries the program and its library, with the image's compilers and headers ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get `swig` at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `swig` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now runs m100's interface outside the original program against its frozen generated bytes and an independent result, plus the packaged library (`OUTPUT` typemaps, `exception.i`, `carrays.i`, `std_string.i` under `-c++`), an authored `SWIG_LIB` replacing it, authored include directories with `-o`/`-outdir`, and the exit statuses of a syntax error and an unknown option. This runs at compute build, network-off runtime and static build. A new `missing-swig` omission fails the original build with `swig: command not found`, the sweep 421 failure ([t005](done/t005.md)). Before shipping these ran on linux/arm64 replicas with the identical Debian packages. Their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other primaries on production rely on the families runs for the same digest.
  - Target languages other than Python, `ccache-swig` and wider C++ library wrapping are not accepted.
- **Cleanup.** Both fixtures and their URLs are gone, and the baseline is restored. The run's Kratos session survived a logout that the network reset; it was found by its login time and revoked by id from a second session, which was then logged out, and every jar was deleted. The isolated CLI state was removed, and other runs' resources were untouched.
