# w8 · m103 — Supply native Python packaging and development tools

**Worker:** worker8 **Goal:** ordinary native Node builds can generate Python package metadata, install an owned wheel and compile/import their Python C extension with the documented prerequisites, without tenant install workarounds. **Status:** done (2026-11-19, production pin `c81d2d447bd8`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply compatible native Python setuptools and metadata tools](done/t001.md) — **DONE** | 50m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Supply compatible native pip and target wheel installation](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [Supply matching Python development headers and compiler inputs](done/t003.md) — **DONE** | 65m | t002 |
| t004 | [Audit all three prerequisites across shared native bases and phases](done/t004.md) — **DONE** | 40m | t001, t002, t003 |
| t005 | [Verify Python prerequisite parity across native authoring surfaces](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Simplify coherent Python-tool supplier changes](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test generated-image metadata, wheel and C-extension behavior](done/t007.md) — **DONE** | 65m | t005 |
| t008 | [Accept original first integrated services and close milestone](done/t008.md) — **DONE** | 20m | t006, t007 |

**Total:** 8 tasks / 335m. [Three independently isolated omissions](finding.md) · complete evidence (sweep 424, purged 2026-11-04).

## Definition of done

Reviewed immutable multiarch setuptools, pip and matching development-header/config/library payloads work coherently with each intended native base/primary Python through applicable build/pre-deploy/runtime/static phases. Actual generated-image tests independently catch each missing prerequisite, then execute real metadata generation, local-only target wheel installation, C compilation/import and Node/Python HTTP behavior with independent data/source/module/extension identities, meaningful failures and network-disabled artifact use. Audit intended bases/architectures/companions/env/PATH/PYTHONPATH/cache/excluded paths while preserving already-working primary Python and existing tools/libraries. After publication both unchanged native clients' original fresh first Free Node services become Live without any install line and pass complete HTTP/log/shared-readback/exact cleanup/baseline/current-quota checks. Limits are explicit; a combined diagnostic install or code alone does not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep424, 2026-10-17: both original first builds fail missing setuptools; both setuptools-only builds prove metadata then fail missing pip; both packaging builds prove local wheel installation then fail missing Python.h; both complete-prerequisite fourth deployments Live with six real integrated HTTP responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): native package and extension builds should work with their documented preloaded tools.
- **Outcome:** Node/Python integrations can build and use their genuine modules without rewriting supported requests or installing platform-promised prerequisites themselves.
- **Why now:** Four real native stages plus selected-base application controls independently isolate three missing prerequisites in one workflow; no matching supplier repair exists. Coherent per-base Python ABI/distro delivery and a shared acceptance/audit are useful together, while each omission has its own implementation/regression task.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior through REST/GraphQL/MCP/Blueprint/dashboard. Preserve existing fields and runtime selectors.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m103；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
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

- **Affected:** t001–t008. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 18:04Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z), and a roll onto `c81d2d447` was in progress. `nativeCommonBase` is `sha256:4b809600…` at that pin and on `main`, the image w8/m96 to m102 accepted on. Its SPDX attestation, fetched again from the registry at 18:04Z, lists `python3-setuptools` and `python3-pkg-resources` `66.1.1-1+deb12u2`, `python3-pip` `23.0.1+dfsg-1` with `python3-wheel` `0.38.4-2`, and `python3-dev` `3.11.2-1+b1` with `python3.11-dev` and `libpython3.11-dev` `3.11.2-6+deb12u9`, beside `python3.11` `3.11.2-6+deb12u9` and `gcc-12` `12.2.0-14+deb12u1` (linux/amd64, Debian 12.15).
- **Next steps (worker, no external gate):** close t001–t003 on the adopted image; audit bases and phases (t004); check parity and replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t005, t008); simplify (t006); give the maintained cohort setuptools, pip and header checks beyond the original build (t007). The three omission controls (`missing-python-metadata`, `missing-python-pip`, `missing-python-headers`) already exist in the cohort.

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `c81d2d447bd8` (common image `sha256:4b809600…`, Debian's `python3-setuptools` 66.1.1, `python3-pip` 23.0.1 and `python3-dev` 3.11.2 for the system Python 3.11.2). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Node 24. Where sweep 424's builds failed in turn on missing setuptools, pip and `Python.h`, both builds wrote the `qa-native-helper` 1.0 metadata, installed the own wheel offline into its target with the frozen module hash `b63a48ce…`, and compiled the same 24,448-byte extension (`f280afec…`); the program's build proof passed the frozen validator. Three runtime requests per service passed the frozen oracle through Node and Python, and the native app logs show every request ([t008](done/t008.md)).
- **Supplier.** Debian's packages with `pkg_resources`, `wheel`, `python3-venv`'s bundled wheels and `libpython3.11`, installed in the common image; nothing is downloaded and no tenant `apt-get` remains ([t001](done/t001.md), [t002](done/t002.md), [t003](done/t003.md)).
- **Phases.** The tools are on the one image that build, pre-deploy and runtime run, for every native primary, beside any Python primary's own interpreter; a static site has them at build ([t004](done/t004.md)). Render lists all three for builds and runtime. No API, CLI or dashboard surface changed ([t005](done/t005.md)).
- **Maintained checks.** The utility cohort now builds a second project from m103's own sources: metadata and an sdist, a `cp311` platform wheel with its `RECORD` checked, an offline `--find-links` install into a target with console script, `PYTHONPATH`, `importlib.metadata` and `pkg_resources` views, PEP 668's refusal and an offline venv, `python3-config` and an embedded `libpython3.11`, and pip's and the compiler's named errors; at the network-off runtime the wheel reinstalls offline. The three existing omission controls still fail the original build at the sweep 424 steps ([t007](done/t007.md)). Before shipping these ran, with 19 source and 6 image mutations each failing, on a linux/arm64 replica with the same package versions; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3), and the common image is bookworm only, as Render's runtimes are.
  - Debian's PEP 668 marker stays: a system or `--user` pip install is refused, as on Debian 12; Render's page is silent on it.
  - No package index, build isolation or system-Python version other than 3.11.2; Python primaries rely on the families runs for the same digest.
- **Cleanup.** Both fixtures and their URLs are gone, and the baseline is restored. The run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
