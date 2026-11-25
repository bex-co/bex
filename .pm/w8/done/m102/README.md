# w8 · m102 — Supply pigz for native asset builds

**Worker:** worker8 **Goal:** native build scripts can compress their assets with the documented build-time pigz tool and publish correct gzip bytes without tenant installation workarounds. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed build-time pigz and required libraries](done/t001.md) — **DONE** | 50m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit build-time compression across native bases and phases](done/t002.md) — **DONE** | 35m | t001 |
| t003 | [Verify build-only pigz parity across authoring surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify compression-tool supplier changes](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test generated-image compression and published gzip bytes](done/t005.md) — **DONE** | 50m | t003 |
| t006 | [Accept original first compressed-asset services and close milestone](done/t006.md) — **DONE** | 20m | t004, t005 |

**Total:** 6 tasks / 190m. [Researched finding](finding.md) · complete evidence (sweep 423, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch pigz payload with compatible required libraries is usable before intended native authored web/static builds. Real generated-image tests verify actual compression and independent exact gzip inflation/header/CRC32/ISIZE/output identity for empty/binary/Unicode data and meaningful errors. Native service/static publication serves the built bytes without requiring runtime pigz. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original first Free Python services become Live without an install line and pass source/input/artifact/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep423, 2026-10-17: both unchanged native creates POST 201/exit 0, both original first builds fail missing-pigz, both one-line install replacements Live with four JSON artifact proofs and two independently verified raw gzip downloads.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): normal native asset build tools should work as documented.
- **Outcome:** Native builds can compress and publish owned assets without rewriting commands or CLI requests.
- **Why now:** Actual original failures and supplier-only controls isolate a missing build tool; source/board/history has no matching repair. Compatible shared-base delivery and exact publication behavior need reviewed implementation and real artifact tests.
- **Render parity:** Included because native build contents affect tenant hosting behavior through REST/GraphQL/MCP/Blueprint/dashboard. Pigz is documented as build-only; no runtime tool, performance guarantee, new API field or CLI fork is required.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m102；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $land 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Drain disposition — 2026-11-19 (worker hang)

The m91 gate holds: m91 is done, and the common image `sha256:4b809600…` serves production on pin `dff52c596d13`. Siblings m97–m101 shipped on that image.

The unblock worker re-verified the gate and prepared both clients' pigz builds, but recorded no acceptance result. At 16:33 UTC its result for a trivial `rm` never returned, the same hang seen on m96. Before that call it had already confirmed that its three QA sessions answer 401. The orchestrator deleted those jars, confirmed that no m102 fixture is live in bex-canary, and stopped the worker. Its board move is kept in stash `loopx/w8/m102` (record only, no source changes).

**Gate:** `worker did not reach a verified outcome: hung before acceptance, nothing committed; needs a fresh worker to /pm unblock and run t001–t006 as m97–m101 did`.

## Unblocked: the gate holds, and acceptance runs on the digest production serves — 2026-11-19 (loopx session, third worker)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand. Stash `loopx/w8/m102` (the hung worker's board move only) is not restored; this record replaces it.
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 17:20Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z). Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m97 to m101 accepted on. That image's SPDX attestation, fetched again from the registry at 17:19Z, lists `pigz` `2.6-1`, `zlib1g` `1:1.2.13.dfsg-1`, `libc6` `2.36-9+deb12u14` and `gzip` `1.12-1` (linux/amd64, Debian 12.15).
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases and boundaries (t002); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004); give the maintained cohort more pigz checks and a `missing-pigz` omission control (t005).

## Accepted on production and closed — 2026-11-19 (loopx session, third worker)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, Debian's pigz 2.6-1 on zlib 1.2.13). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Python 3.13. The original `pigz -n -p 2 -c` found `/usr/bin/pigz` and compressed the 12,544-byte Unicode asset into the same 130 bytes (`8aecad5d…`) on both, and the program's build proof passed the frozen validator: header, CRC32, ISIZE and inflated bytes equal to the reference. Three runtime requests per service passed the frozen oracle, the raw `/asset.gz` inflating to the exact input under host zlib, and the native app logs show every request ([t006](done/t006.md)).
- **Supplier.** Debian's `pigz` with its `unpigz` link and its zlib and thread closure, installed in the common image; nothing is downloaded and no tenant `apt-get` remains ([t001](done/t001.md)).
- **Phases.** pigz runs at build for every native primary and static site ([t002](done/t002.md)). Render lists it as build-only; bex's runtime shares the build's image, so pigz is there too, but nothing at runtime needs it. No API, CLI or dashboard surface changed ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now runs m102's own program outside the original build against the frozen production bytes, reads every retained `.gz` back with zlib alone at build, network-off runtime and static build, and checks in-place names, modes and times with `-N`, thread-count independence and `-i`, the zlib, ZIP and zopfli formats, concatenated members, `unpigz`, `PIGZ` options and pigz's exit statuses. At the runtime, m102's own server serves the built bytes with no pigz on `PATH`. A new `missing-pigz` omission fails the original build with `pigz: command not found`, the sweep 423 failure ([t005](done/t005.md)). Before shipping these ran on linux/arm64 replicas with the same package versions; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3), and the common image is bookworm only, as Render's runtimes are.
  - Other primaries on production rely on the families runs for the same digest.
  - No compression-speed or parallel-scaling guarantee.
- **Cleanup.** Both fixtures and their URLs are gone, and the baseline is restored. The run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
