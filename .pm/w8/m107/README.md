# w8 · m107 — Supply ZIP creation in native builds and runtimes

**Worker:** worker8 **Goal:** native builds and download handlers create correct ZIP archives without an authored installer. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed ZIP creation tools and required libraries](t001.md) | 45m | — |
| t002 | [Audit ZIP supply across native bases and phases](t002.md) | 40m | t001 |
| t003 | [Verify ZIP availability parity across hosting surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify ZIP supplier changes](t004.md) | 15m | t003 |
| t005 | [Test generated-image ZIP creation and independent decoding](t005.md) | 50m | t003 |
| t006 | [Accept original first ZIP services and close milestone](t006.md) | 20m | t004, t005 |

**Total:** 6 tasks / 190m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep434/README.md).

## Definition of done

Reviewed immutable ZIP contents and compatible required libraries are available before authored native builds and remain usable in the resulting compute runtime. Real generated-image tests create build/runtime archives and independently verify exact unique member names/bytes/sizes/CRC32 and stored/deflated methods for empty/binary/Unicode/space-named files, including meaningful errors. Audit intended primary lines/architectures/companions/PATH/env/cache and existing excluded paths. After publication both unchanged native clients' original first Free Node services become Live without installing ZIP and pass exact source/archive/JSON/raw-HTTP/native-log/shared-readback checks. Exact fixtures/children/URLs are removed, baseline preserved and current quotas reconciled. Implementation or install-only controls do not satisfy DoD; execution limits remain explicit.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep434, 2026-10-17: both unchanged creates POST 201/native 0; first builds fail missing ZIP; install-only controls create genuine build/runtime ZIPs verified independently through four JSON responses and two raw downloads.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native build assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): documented native tools work in ordinary hosting journeys.
- **Expected outcome:** Native asset builds and download handlers can create ZIP archives without tenant install workarounds.
- **Why now:** Actual missing-tool failures plus supplier-only controls isolate an uncovered native-image omission. Complete package/library delivery and shared base/phase/architecture checks require reviewed implementation and real artifact tests.
- **Render parity:** Included because native image contents affect tenant-facing hosting through REST/GraphQL/MCP/Blueprint/dashboard. Preserve existing fields, commands, source/env and CLI wire contracts; no new selector or client fork is needed.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m107；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.
