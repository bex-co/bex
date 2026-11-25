# w8 · m93 — Supply Pandoc in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native document-generating apps can build and serve converted HTML using supplied Pandoc. **Status:** done (2026-11-16, production pin `7aaa8700befa`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native Pandoc distribution](done/t001.md) — **DONE** | 75m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit all shared native consumers and cache boundaries](done/t002.md) — **DONE** | 35m | t001 |
| t003 | [Verify native document-tool Render parity](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify the changed native document supplier](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test actual document conversion and affected neighbors](done/t005.md) — **DONE** | 40m | t003 |
| t006 | [Accept fresh native defaults and close the milestone](done/t006.md) — **DONE** | 15m | t004, t005 |

**Total:**6 tasks/200m. [Finding](finding.md) · complete evidence (sweep 407, purged 2026-11-04).

## Definition of done

A reviewed pinned multiarch Pandoc supplier runs real document conversions through intended native build/pre-deploy/runtime/static phases, with primary/companion/env/PATH/root/cache/exclusion regressions and actual generated-image tests. After publication, both unchanged native clients' fresh original first Free services reach Live without any install prefix, and exact source/program/document, independently parsed runtime HTML and native logs pass. Exact fixture/children/URLs/local cleanup and baseline/quota restoration pass. Shared authoring contracts stay compatible; unverified scope is explicit. Install controls or code alone do not close this milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep407,2026-10-17: both unchanged native POST201/exit0 first builds fail missing Pandoc, both one-line install controls Live, six fresh runtime conversion/independent HTML checks pass.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): native hosting commands should work with the named utility contract.
- **Expected outcome:** Markdown-based document generators deploy and convert at runtime without installing a supplied tool in app code or changing CLI requests.
- **Why now:** Both independent clients and actual selected-base controls isolate the missing program; source has no supplier or newer fix. Existing FFmpeg/PostgreSQL-client work does not provide Pandoc.
- **Render parity:** Included because a native image change affects tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard, while preserving input/output schemas.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m93；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $land 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Unblocked: the gate holds, and acceptance runs on the live digest — 2026-11-16 (loopx session)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 closed in `d66e4897e`. The newest `chore(deploy): pin platform images` commit is `546945798`, which pinned `7aaa8700befa` at 16:22Z. `git merge-base --is-ancestor cfe44cd09 7aaa8700befa` holds, so production native builds start from the common image `sha256:5d6b8139…`. Its signed SPDX lists `pandoc` `2.17.1.1-2~deb12u1` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)). `deploy/native-base/install.sh` installs it, and `check-tools.py` checks a UTF-8 conversion and an unknown-format refusal at every publication.
- **Which digest the acceptance runs on: the live `5d6b8139…`, now.** w8/335's re-pin to `sha256:4b809600…` (`71914199f`) is on main but not on production: `7aaa8700befa` does not contain it. The next push-triggered roll is due no earlier than about 22:22Z (six hours after the 16:22Z pin), and only the user can dispatch an earlier one. Running now is sound:
  - The re-pin's SPDX names the same 864 packages and versions as `5d6b8139`, so `pandoc`, `pandoc-data` and every library under them are the same packages.
  - Families run [43900534897](https://github.com/bex-co/bex/actions/runs/43900534897) passed the AMD64 utility cohort (`qualify (amd64)`) on `4b809600`. That cohort's `node-compute` case runs this milestone's frozen original program: the conversion at build, and three HTTP requests on a network-off runtime.
  - Hosted acceptance tests what production serves. Waiting for a roll that changes nothing under test would only hold the item.
  - Every hosted result names the pin it ran on.
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); give the maintained cohort a Pandoc omission control and data, Lua and static-build checks (t005); replay both unmodified native clients on production and run fresh Free hosted acceptance with exact cleanup (t003, t006); simplify (t004).

## Accepted on production and closed — 2026-11-16 (loopx session)

The definition of done holds on production pin `7aaa8700befa` (common image `sha256:5d6b8139…`, `pandoc` 2.17.1.1-2~deb12u1). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified same-pin Render CLI, each running the frozen original document commands on Free Node 24. The exact source, document and program proofs passed at build, and six runtime requests converted the document again, with every HTML body parsed independently and byte-identical to the build's. The native app logs show all six requests ([t006](done/t006.md)).
- **Supplier.** Debian's `pandoc` and `pandoc-data` in the common image, with their library closure. Every Markdown read needs the data, so both travel in one package set ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get Pandoc at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `pandoc` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now also runs a standalone template and Lua filter conversion with an independent parse and two error cases, at compute build, network-off runtime and static build, plus a `missing-pandoc` omission control that must fail the original build on `pandoc: command not found` ([t005](done/t005.md)). Before shipping they ran on Debian 12 arm64 with the identical package; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Non-Node primaries on production rely on the families runs for the same digest.
  - Formats beyond Markdown and HTML, PDF engines and remote fetches are not covered.
- **Cleanup.** Both fixtures and URLs are gone, every Kratos jar was logged out and deleted, the isolated CLI state was removed, and other runs' resources were untouched.
