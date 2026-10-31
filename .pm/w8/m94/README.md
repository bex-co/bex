# w8 · m94 — Supply SQLite CLI in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native SQLite seed/import/export commands can use the supplied executable without replacing a working language library. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native sqlite3 executable and dependencies](t001.md) | 70m | [w8/m91/t007](../m91/t007.md) |
| t002 | [Audit shared native consumers and preserve library and cache behavior](t002.md) | 35m | t001 |
| t003 | [Verify SQLite CLI Render parity across native authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify the changed SQLite CLI supplier](t004.md) | 15m | t003 |
| t005 | [Test real seed/import and readonly exports with healthy library controls](t005.md) | 40m | t003 |
| t006 | [Accept original fresh native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep409/README.md).

## Definition of done

A reviewed immutable multiarch sqlite3 executable with complete required libraries is available through intended native build/pre-deploy/runtime/static phases, preserving the healthy Python sqlite3 module and shared primary/companion/env/PATH/root/cache/exclusion behavior. Meaningful real generated-image tests pass. After publication, both unchanged native clients' fresh original first Free Python SQLite services reach Live without an install prefix and pass exact source/program/SQL, independent library/CLI three-row checks, database/file/runtime/native-log proofs and exact cleanup/baseline preservation and quota accounting. Shared authoring contracts stay compatible and unverified scope is explicit. Install controls or source code alone do not close the milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep409,2026-10-17: both unchanged native POST201/exit0 first builds fail missing sqlite3 after healthy independent Python-library seed controls; both one-line install controls Live and six real readonly runtime exports pass.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting commands should work with named supplied utilities.
- **Expected outcome:** Native SQLite seed/import/export commands deploy and run without installing the executable in tenant code or rewriting CLI input.
- **Why now:** Both independent native clients and actual selected-base controls isolate the absent executable while the library and exact SQL/data work; there is no supplier or newer fix. PostgreSQL/Pandoc tool work has separate dependency and acceptance boundaries.
- **Render parity:** Included because native image contents affect tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard; no new schema field or CLI fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m94；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.
