# w8 · m96 — Supply GNU gettext for native message compilation

**Worker:** worker8 **Goal:** ordinary native internationalized apps can compile and use their message catalogs without installing promised system tools in tenant code. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed GNU gettext tools in native images](t001.md) | 70m | [w8/m91/t007](../m91/t007.md) |
| t002 | [Audit shared native bases, locale behavior and exclusions](t002.md) | 30m | t001 |
| t003 | [Verify GNU gettext Render parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify the GNU gettext supplier](t004.md) | 15m | t003 |
| t005 | [Test real Django compilation and runtime catalog behavior](t005.md) | 45m | t003 |
| t006 | [Accept original native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep416/README.md).

## Definition of done

A reviewed immutable multiarch GNU gettext utility set and complete compatible library/data/locale closure are available through intended native build/pre-deploy/runtime/static phases. Real generated-image Django compilation, runtime external compilation, independent catalog parsing and exact Unicode/format/plural translations pass, with meaningful failures and audited sibling/architecture/phase/env/PATH/locale/cache/exclusion behavior. After publication both unchanged native clients' fresh original first Free services become Live without install lines, pass source/program/PO/catalog/translation/log/shared-readback checks and complete exact cleanup with baseline/current-inventory/quota reconciliation. Limits remain explicit; apt controls or source changes alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep416,2026-10-17: both valid native POST201/exit0 first builds fail Django compilemessages missing-msgfmt after successful dependency install; both one-line gettext controls Live with exact real compiled-catalog and six runtime translation responses.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime dependencies work as documented.
- **Outcome:** Standard translation builds and message utilities run without rewriting tenant commands or CLI requests.
- **Why now:** Both native clients and exact selected-base controls isolate the missing GNU utility while Django and catalog data work. No matching supplier/fix exists; adjacent utility issues have different consumers/closure requirements.
- **Render parity:** Included because native image contents affect tenant builds/runtime through REST/GraphQL/MCP/Blueprint/dashboard; no new product control or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m96；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.
