# w8 · m93 — Supply Pandoc in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native document-generating apps can build and serve converted HTML using supplied Pandoc. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native Pandoc distribution](t001.md) | 75m | — |
| t002 | [Audit all shared native consumers and cache boundaries](t002.md) | 35m | t001 |
| t003 | [Verify native document-tool Render parity](t003.md) | 20m | t002 |
| t004 | [Simplify the changed native document supplier](t004.md) | 15m | t003 |
| t005 | [Test actual document conversion and affected neighbors](t005.md) | 40m | t003 |
| t006 | [Accept fresh native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/200m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep407/README.md).

## Definition of done

A reviewed pinned multiarch Pandoc supplier runs real document conversions through intended native build/pre-deploy/runtime/static phases, with primary/companion/env/PATH/root/cache/exclusion regressions and actual generated-image tests. After publication, both unchanged native clients' fresh original first Free services reach Live without any install prefix, and exact source/program/document, independently parsed runtime HTML and native logs pass. Exact fixture/children/URLs/local cleanup and baseline/quota restoration pass. Shared authoring contracts stay compatible; unverified scope is explicit. Install controls or code alone do not close this milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep407,2026-10-17: both unchanged native POST201/exit0 first builds fail missing Pandoc, both one-line install controls Live, six fresh runtime conversion/independent HTML checks pass.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): native hosting commands should work with the named utility contract.
- **Expected outcome:** Markdown-based document generators deploy and convert at runtime without installing a supplied tool in app code or changing CLI requests.
- **Why now:** Both independent clients and actual selected-base controls isolate the missing program; source has no supplier or newer fix. Existing FFmpeg/PostgreSQL-client work does not provide Pandoc.
- **Render parity:** Included because a native image change affects tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard, while preserving input/output schemas.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m93；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。
