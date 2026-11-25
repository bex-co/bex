# w8 · m107 — Supply ZIP creation in native builds and runtimes

**Worker:** worker8 **Goal:** native builds and download handlers create correct ZIP archives without an authored installer. **Status:** done (2026-11-19, production pin `c624a4f217ff`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed ZIP creation tools and required libraries](done/t001.md) — **DONE** | 45m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit ZIP supply across native bases and phases](done/t002.md) — **DONE** | 40m | t001 |
| t003 | [Verify ZIP availability parity across hosting surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify ZIP supplier changes](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test generated-image ZIP creation and independent decoding](done/t005.md) — **DONE** | 50m | t003 |
| t006 | [Accept original first ZIP services and close milestone](done/t006.md) — **DONE** | 20m | t004, t005 |

**Total:** 6 tasks / 190m. [Researched finding](finding.md) · complete evidence (sweep 434, purged 2026-11-04).

## Definition of done

Reviewed immutable ZIP contents and compatible required libraries are available before authored native builds and remain usable in the resulting compute runtime. Real generated-image tests create build/runtime archives and independently verify exact unique member names/bytes/sizes/CRC32 and stored/deflated methods for empty/binary/Unicode/space-named files, including meaningful errors. Audit intended primary lines/architectures/companions/PATH/env/cache and existing excluded paths. After publication both unchanged native clients' original first Free Node services become Live without installing ZIP and pass exact source/archive/JSON/raw-HTTP/native-log/shared-readback checks. Exact fixtures/children/URLs are removed, baseline preserved and current quotas reconciled. Implementation or install-only controls do not satisfy DoD; execution limits remain explicit.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep434, 2026-10-17: both unchanged creates POST 201/native 0; first builds fail missing ZIP; install-only controls create genuine build/runtime ZIPs verified independently through four JSON responses and two raw downloads.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native build assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): documented native tools work in ordinary hosting journeys.
- **Expected outcome:** Native asset builds and download handlers can create ZIP archives without tenant install workarounds.
- **Why now:** Actual missing-tool failures plus supplier-only controls isolate an uncovered native-image omission. Complete package/library delivery and shared base/phase/architecture checks require reviewed implementation and real artifact tests.
- **Render parity:** Included because native image contents affect tenant-facing hosting through REST/GraphQL/MCP/Blueprint/dashboard. Preserve existing fields, commands, source/env and CLI wire contracts; no new selector or client fork is needed.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m107；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $land 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Unblocked: the gate holds, and acceptance runs on the digest production serves — 2026-11-19 (loopx session)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 18:57Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `99bbc4239` (pin `c624a4f217ff`, rolled by `deploy.yml` dispatch run 44587868376, deploy job 18:50–18:55Z). Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m96 to m103 accepted on. That image's SPDX attestation, fetched again from the registry at 18:55Z, lists `zip` `3.0-13+deb12u1`, `unzip` `6.0-28+deb12u1`, `libbz2-1.0` `1.0.8-5+b1` and `libc6` `2.36-9+deb12u14` (linux/amd64, Debian 12.15).
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004); give the maintained cohort more ZIP checks and a `missing-zip` omission control (t005).

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `c624a4f217ff` (common image `sha256:4b809600…`, Debian's zip 3.0-13+deb12u1 on libbz2 1.0.8, beside unzip 6.0). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Node with Render's default Node 24.22.1. The original `zip -X -9 -q` found zip and wrote a six-member archive that `unzip -t` and Python's zipfile both accepted, with exact names, bytes, sizes, CRC32 and stored or deflated methods; both builds passed the frozen validator, and their archives are byte-identical to what the arm64 replica writes. Three runtime requests per service passed the frozen oracle, each creating a fresh archive at runtime and `/download` serving one as `application/zip`, and the native app logs show every request ([t006](done/t006.md)).
- **Supplier.** Debian's `zip` with its libbz2 and glibc closure, installed in the common image; nothing is downloaded and no tenant `apt-get` remains ([t001](done/t001.md)).
- **Phases.** zip runs at build for every native primary and static site and at runtime on the shared image, as Render lists it ([t002](done/t002.md)). No API, CLI or dashboard surface changed ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now runs m107's own program outside the original build against frozen build and runtime archive bytes, and checks zip's levels and methods, Unicode and space-containing names restored by `unzip`, `-X`, bzip2, symlinks, recursion and member selection, stdin and piped streams, update, delete, comments, `-T`, and exit statuses 12, 18, 16 and 3 with zip's messages, every archive read back by zipfile alone, at build, network-off runtime and static build. A new `missing-zip` omission fails the original build with `spawnSync zip ENOENT`, the sweep 434 failure ([t005](done/t005.md)). Before shipping these ran on a linux/arm64 replica with the same package versions; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3), and the common image is bookworm only, as Render's runtimes are.
  - Other primaries on production rely on the families runs for the same digest.
  - Encryption, split archives and Zip64 workloads are not accepted, and Render names no zip version.
- **Cleanup.** Both fixtures and their URLs are gone, and the baseline is restored. Both Kratos sessions were logged out right after the device approvals and their jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
