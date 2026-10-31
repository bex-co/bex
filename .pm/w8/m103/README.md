# w8 · m103 — Supply native Python packaging and development tools

**Worker:** worker8 **Goal:** ordinary native Node builds can generate Python package metadata, install an owned wheel and compile/import their Python C extension with the documented prerequisites, without tenant install workarounds. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply compatible native Python setuptools and metadata tools](t001.md) | 50m | — |
| t002 | [Supply compatible native pip and target wheel installation](t002.md) | 60m | t001 |
| t003 | [Supply matching Python development headers and compiler inputs](t003.md) | 65m | t002 |
| t004 | [Audit all three prerequisites across shared native bases and phases](t004.md) | 40m | t001, t002, t003 |
| t005 | [Verify Python prerequisite parity across native authoring surfaces](t005.md) | 20m | t004 |
| t006 | [Simplify coherent Python-tool supplier changes](t006.md) | 15m | t005 |
| t007 | [Test generated-image metadata, wheel and C-extension behavior](t007.md) | 65m | t005 |
| t008 | [Accept original first integrated services and close milestone](t008.md) | 20m | t006, t007 |

**Total:** 8 tasks / 335m. [Three independently isolated omissions](finding.md) · [complete evidence](evidence/cli-sweep424/README.md).

## Definition of done

Reviewed immutable multiarch setuptools, pip and matching development-header/config/library payloads work coherently with each intended native base/primary Python through applicable build/pre-deploy/runtime/static phases. Actual generated-image tests independently catch each missing prerequisite, then execute real metadata generation, local-only target wheel installation, C compilation/import and Node/Python HTTP behavior with independent data/source/module/extension identities, meaningful failures and network-disabled artifact use. Audit intended bases/architectures/companions/env/PATH/PYTHONPATH/cache/excluded paths while preserving already-working primary Python and existing tools/libraries. After publication both unchanged native clients' original fresh first Free Node services become Live without any install line and pass complete HTTP/log/shared-readback/exact cleanup/baseline/current-quota checks. Limits are explicit; a combined diagnostic install or code alone does not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep424, 2026-10-17: both original first builds fail missing setuptools; both setuptools-only builds prove metadata then fail missing pip; both packaging builds prove local wheel installation then fail missing Python.h; both complete-prerequisite fourth deployments Live with six real integrated HTTP responses.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): native package and extension builds should work with their documented preloaded tools.
- **Outcome:** Node/Python integrations can build and use their genuine modules without rewriting supported requests or installing platform-promised prerequisites themselves.
- **Why now:** Four real native stages plus selected-base application controls independently isolate three missing prerequisites in one workflow; no matching supplier repair exists. Coherent per-base Python ABI/distro delivery and a shared acceptance/audit are useful together, while each omission has its own implementation/regression task.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior through REST/GraphQL/MCP/Blueprint/dashboard. Preserve existing fields and runtime selectors.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m103；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.
