# w8 · m97 — Supply Ghostscript for native PDF rendering

**Worker:** worker8 **Goal:** ordinary native PDF-processing apps can invoke the promised renderer without tenant installation workarounds. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native Ghostscript with its runtime resources](done/t001.md) — **DONE** | 80m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit shared native bases, phases, resources and exclusions](done/t002.md) — **DONE** | 35m | t001 |
| t003 | [Verify Ghostscript native Render parity and authoring inputs](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify the Ghostscript supplier](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test actual generated-image PDF conversion and resource lookup](done/t005.md) — **DONE** | 50m | t003 |
| t006 | [Accept original first PDF deployments and close the milestone](done/t006.md) — **DONE** | 15m | t004, t005 |

**Total:**6 tasks/215m. [Finding](finding.md) · complete evidence (sweep 418, purged 2026-11-04).

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

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Unblocked: the gate holds, and acceptance runs on the digest production serves — 2026-11-19 (loopx session)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 closed in `d66e4897e` ([t007](../m91/done/t007.md) `status: done`). At 14:20Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z). Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m95 accepted on. That image's SPDX attestation, fetched again from the registry, lists `ghostscript`, `libgs10`, `libgs10-common` and `libgs-common` `10.0.0~dfsg-11+deb12u8`, with `fonts-urw-base35` `20200910-7` and `poppler-data` `0.4.12-1` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)). `deploy/native-base/install.sh` installs `ghostscript`, and `check-tools.py` renders a PostScript page with `gs` at every publication.
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); give the maintained cohort more Ghostscript checks and a `missing-ghostscript` omission control on the Python primary (t005); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004).

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, `ghostscript` 10.0.0~dfsg-11+deb12u8). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Python 3.13. The original `gs … -sDEVICE=png16m` line rendered the PDF at build, and the exact source, PDF, program and pixel proofs passed. Six runtime requests rendered it again: the JSON proofs passed the frozen validator, and each PNG decoded independently to the exact red/blue pixels, identical to the build's. The native app logs show all six requests ([t006](done/t006.md)).
- **Supplier.** Debian's `ghostscript` in the common image, with `libgs10`, its `Resource` and `iccprofiles` data, the helper scripts, the URW Base35 fonts and `poppler-data` ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get `gs` at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `ghostscript` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now also renders m97's own PDF outside the Python primary, embeds the packaged URW Helvetica through `ps2pdf`, extracts text with `txtwrite`, selects a page, searches an authored `GS_LIB`, and requires `-dSAFER`'s file refusal and a failing program's exit 1, at compute build, network-off runtime and static build. A `missing-ghostscript` omission control on the Python primary must fail the original build on `gs: command not found` ([t005](done/t005.md)). Before shipping they ran on Debian 12 arm64 with the identical packages; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other Python lines on production rely on the families runs for the same digest.
  - Other output devices, CJK fonts and document workflows are not separately accepted.
- **Cleanup.** Both fixtures and their URLs are gone, the run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
