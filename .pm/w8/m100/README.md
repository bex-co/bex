# w8 · m100 — Supply SWIG for native C-binding builds

**Worker:** worker8 **Goal:** ordinary Python binding builds generate, compile and import their C extension using the documented native SWIG tool without tenant installation workarounds. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native SWIG with library and typemap data](t001.md) | 60m | — |
| t002 | [Audit SWIG delivery across shared native bases and phases](t002.md) | 35m | t001 |
| t003 | [Verify native SWIG parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify native binding-tool supplier changes](t004.md) | 15m | t003 |
| t005 | [Test generated-image binding generation, compilation and import](t005.md) | 50m | t003 |
| t006 | [Accept original first C-binding deployments and close milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/200m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep421/README.md).

## Definition of done

A reviewed immutable multiarch SWIG payload with compatible executable/libraries/include/language/typemap data is usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests verify wrapper/proxy generation, gcc compilation/import and exact independent C arithmetic/Unicode results and source/generated/extension identities, reject an invalid interface and prove data-tree availability and network-disabled runtime generation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original fresh first Free Python services become Live without a SWIG install and pass source/interface/extension/result/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep421,2026-10-17: both unchanged native creates POST201/exit0, both original first builds fail missing-swig, both one-line SWIG-install replacements Live with six real compiled-C-extension HTTP responses.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): regular native binding build tools should work as documented.
- **Outcome:** Native Python C-binding builds can generate their wrappers and run the resulting extension without rewriting commands or CLI requests.
- **Why now:** Real first failures and SWIG-only controls isolate missing delivery; source/board/history has no existing supplier repair. Required SWIG data trees and shared-base adoption need deliberate reviewed delivery.
- **Render parity:** Included because native tool contents affect tenant builds/runtime across REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m100；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.
