# w8 · m91 — Supply PostgreSQL clients in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native backup-processing commands can use the supplied PostgreSQL client tools. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed PostgreSQL client programs and libraries](t001.md) | 75m | — |
| t002 | [Audit shared native consumers and preserve image boundaries](t002.md) | 40m | t001 |
| t003 | [Verify Render parity across native authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify the changed client supplier](t004.md) | 15m | t003 |
| t005 | [Test real archive processing and shared toolchain neighbors](t005.md) | 40m | t003 |
| t006 | [Accept fresh native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/205m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep402/README.md).

## Definition of done

After publication, both native clients' fresh first original Free backup-processing builds reach Live without an installer prefix and serve actual runtime archive-to-SQL conversion with the exact source/archive/three-row proofs and native logs. A reviewed multiarch client/library supplier works through the documented native build/pre-deploy/runtime/static phases and intended client/server/archive versions; meaningful real generated-image tests and shared-primary/companion/env/PATH/cache/root/exclusion checks pass. Shared authoring surfaces preserve existing inputs/output contracts. Exact owned fixtures/children/URLs/local resources are cleaned and baseline/quota intact. Explicitly bound unverified cases. Code alone does not close this milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep402,2026-10-17: two unchanged POST201/native0 first builds fail missing pg_restore after exact source/backup proofs; both one-line install controls reach Live and six real runtime archive checks pass.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [ADR018](../../../docs/ADR018-render-parity.md) and [reviewed native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting build/start commands should work with named runtime utilities.
- **Expected outcome:** A native service can inspect/process PostgreSQL archives with supplied client programs, without bootstrapping them in app code or rewriting CLI requests.
- **Why now:** Both independent native clients reproduce the same missing program while primary tooling and exact input data work; the shared image path has no supplier or newer repair. Existing FFmpeg completion explicitly did not certify this tool family.
- **Render parity:** Included because one native image change affects tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard; no new schema field, CLI fork or product toggle.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m91；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。
