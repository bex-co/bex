# w8 · m97 — Supply Ghostscript for native PDF rendering

**Worker:** worker8 **Goal:** ordinary native PDF-processing apps can invoke the promised renderer without tenant installation workarounds. **Status:** blocked — common qualification/adoption dependency; original hosted acceptance remains open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native Ghostscript with its runtime resources](t001.md) | 80m | [w8/m91/t007](../m91/t007.md) |
| t002 | [Audit shared native bases, phases, resources and exclusions](t002.md) | 35m | t001 |
| t003 | [Verify Ghostscript native Render parity and authoring inputs](t003.md) | 20m | t002 |
| t004 | [Simplify the Ghostscript supplier](t004.md) | 15m | t003 |
| t005 | [Test actual generated-image PDF conversion and resource lookup](t005.md) | 50m | t003 |
| t006 | [Accept original first PDF deployments and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/215m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep418/README.md).

## Definition of done

A reviewed immutable multiarch Ghostscript supplier with compatible executable/library/init/resource/font/color data is available through intended native build/pre-deploy/runtime/static phases. Meaningful actual generated-image PDF/pixel and resource lookup checks pass with nonzero error propagation and audited architecture/base/companion/env/GS_LIB/PATH/locale/cache/exclusion behavior. After publication both unchanged native clients' fresh original first Free services become Live without install lines and pass exact source/program/PDF/rendered-image/native-log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are documented; apt controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep418,2026-10-17: both unchanged native POST201/exit0 first builds fail missing-gs, both one-line Ghostscript controls Live with six real PDF conversions and independently checked PNG pixels.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime tools work as documented.
- **Outcome:** Native PDF conversion commands run without rewriting tenant dependencies or CLI requests.
- **Why now:** Both native clients and exact selected-base controls isolate the missing renderer with real source/PDF/pixel proofs; no supplier or newer fix exists. Adjacent image/media utility issues have separate consumers and closure requirements.
- **Render parity:** Included because image contents affect tenant build/runtime through REST/GraphQL/MCP/Blueprint/dashboard; no new API field or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m97；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in [the shared evidence-to-task record](../m91/evidence/original28-free-contract-20261102/capsule/EVIDENCE-TO-TASK-MAP.json); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.
