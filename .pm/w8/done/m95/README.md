# w8 · m95 — Supply libvips in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native image-processing apps can load the supplied libvips dependency and transform images without tenant installation workarounds. **Status:** done (2026-11-17, production pin `938c5cb36b49`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native libvips libraries, headers and tools](done/t001.md) — **DONE** | 90m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit shared bases, phases, companions and exclusions](done/t002.md) — **DONE** | 35m | t001 |
| t003 | [Verify native image-library Render parity and shared inputs](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify the native libvips supplier](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test generated-image binding, header and CLI transformations](done/t005.md) — **DONE** | 55m | t003 |
| t006 | [Accept original native defaults and close the milestone](done/t006.md) — **DONE** | 15m | t004, t005 |

**Total:**6 tasks/230m. [Finding](finding.md) · complete evidence (sweep 414, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch libvips supplier, complete dependency/data closure and promised headers/CLI tools are usable through intended native build/pre-deploy/runtime/static phases. Meaningful real generated-image import, header compile/link and image transformations pass, preserving all audited bases, companions, env/PATH/root/cache behavior and excluded build paths. After publication both unchanged native clients create original first Free Python services without an install line, become Live and pass exact source/program/input/build/runtime/pixel/PNG/native-log/shared-readback proofs and exact cleanup/baseline/current-quota checks. Documented limits are explicit. Apt install controls and source changes alone do not close the milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep414,2026-10-17: both unchanged native POST201/exit0 first builds fail missing libvips.so.42 after successful PyVips installation; both one-line library controls Live with six real image transformations and independently decoded pixels.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting image-processing dependencies should work as documented.
- **Outcome:** Plain image bindings and supplied native libvips tools work without modifying tenant dependencies or CLI inputs.
- **Why now:** Both independent native clients, exact selected-base controls and real image outputs isolate the absent library. There is no supplier or newer fix; adjacent utility work does not provide it.
- **Render parity:** Included because the image contents affect tenant builds/runtime across REST/GraphQL/MCP/Blueprint/dashboard; no new API field is needed.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m95；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $land 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Unblocked: the gate holds, and acceptance runs on the digest production serves — 2026-11-16 (loopx session)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 closed in `d66e4897e`. At 18:07Z the newest `chore(deploy): pin platform images` commit was `546945798` (pin `7aaa8700befa`, 16:22Z), whose `nativeCommonBase` is `sha256:5d6b8139…`. The SPDX attestations of that image and of the re-pin `sha256:4b809600…` (`71914199f`, w8/335), both fetched again from the registry, list the identical 864 packages, including `libvips42`, `libvips-dev` and `libvips-tools` `8.14.1-3+deb12u3` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)). `deploy/native-base/install.sh` installs them, and `check-tools.py` runs `vips`, `vipsheader` and a library load at every publication.
- **Which digest the acceptance ran on: `4b809600…`.**
  - The first fixture pair started on the live `5d6b8139…`, then still serving because `7aaa8700befa` does not contain the re-pin. It built and went live at 18:17Z.
  - Production then rolled to pin `938c5cb36b49` (`a24555d5c`, 23:07Z), which contains `71914199f`, so its native builds start from `4b809600…`.
  - Hosted acceptance tests what production serves, so the first pair was deleted and the full acceptance re-ran on fresh fixtures on the new pin (t006). Both digests carry the same packages.
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); give the maintained cohort vips-tool checks and a `missing-libvips` omission control (t005); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004).

## Accepted on production and closed — 2026-11-17 (loopx session)

The definition of done holds on production pin `938c5cb36b49` (common image `sha256:4b809600…`, `libvips42` 8.14.1-3+deb12u3). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original PyVips commands on Free Python 3.13. `pip install pyvips==3.2.0` built its API-mode wheel against the supplied library. The exact source, input, program and pixel proofs passed at build. Six runtime requests transformed the image again: the JSON proofs passed the frozen validator, and each PNG decoded independently to the exact red/blue pixels, identical to the build's. The native app logs show all six requests ([t006](done/t006.md)).
- **Supplier.** Debian's `libvips42`, `libvips-dev` and `libvips-tools` in the common image, with their codec libraries, `vips-modules-8.14` loaders, headers and `pkg-config` file ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get the library and tools at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `libvips-dev` and `libvips-tools` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now also runs `vips` tools on the m95 image at compute build, network-off runtime and static build: a resize equal to PyVips' pixels, a JPEG thumbnail, an exact lossless WebP round trip and a BMP that only the packaged `vips-magick` module loads. A `missing-libvips` omission control on the Python primary must fail the original build on `cannot load library 'libvips.so.42'` ([t005](done/t005.md)). Before shipping they ran on Debian 12 arm64 with the identical packages; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other Python lines on production rely on the families runs for the same digest.
  - Not every optional codec or loader module is exercised, and `pyvips[binary]` is not supplied.
- **Cleanup.** Both fixture pairs and their URLs are gone, every Kratos jar was logged out and deleted, the isolated CLI state was removed, and other runs' resources were untouched.
