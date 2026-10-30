# w8 · m98 — Supply jq and rsync for native asset builds

**Worker:** worker8 **Goal:** normal native JSON-manifest and local asset-copy builds run with their documented utilities without tenant installation workarounds. **Status:** todo — 2026-10-29 按 parity 规则恢复；m91 建立共享原生基础镜像供应链，各项保留原功能与线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native jq executable and dependencies](t001.md) | 50m | — |
| t002 | [Supply a reviewed native rsync executable and dependencies](t002.md) | 70m | t001 |
| t003 | [Audit shared native bases, phases and both utility consumers](t003.md) | 35m | t001, t002 |
| t004 | [Verify jq and rsync Render parity across native inputs](t004.md) | 20m | t003 |
| t005 | [Simplify the two-utility supplier changes](t005.md) | 15m | t004 |
| t006 | [Test generated-image manifest generation and local copying](t006.md) | 60m | t004 |
| t007 | [Accept original first asset deployments and close the milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/270m. [Two independently isolated omissions](finding.md) · [complete evidence](evidence/cli-sweep419/README.md).

## Definition of done

Reviewed immutable multiarch jq and rsync payloads with compatible required libraries/data are usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests independently catch each missing tool and verify exact combined JSON-manifest/local-copy/Unicode/space-name/byte/exclude/stale-own-output behavior, network-disabled runtime and meaningful failures. Audit intended bases/architectures/companions/env/PATH/root/cache/excluded paths. After publication both unchanged native clients' original fresh first Free Node services become Live without any install line and pass real source/program/data/manifest/copy/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; staged apt controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep419,2026-10-17: both original native POST201/exit0 first builds fail missing-jq; both jq-only builds generate correct manifests then fail missing-rsync; both complete-tool replacements Live with six real manifest/file-copy/HTML responses.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime tools should work as documented.
- **Outcome:** Native asset builds and runtime scripts can filter JSON and copy local files without rewriting commands or CLI requests.
- **Why now:** Actual independent native stages plus selected-base controls isolate two missing executables in one ordinary asset workflow; there is no supplier/newer fix. Shared delivery/audit/acceptance is coherent, while each missing tool has its own implementation and regression task.
- **Render parity:** Included because native image contents affect tenant builds/runtime through REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m98；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。
