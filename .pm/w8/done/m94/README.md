# w8 · m94 — Supply SQLite CLI in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native SQLite seed/import/export commands can use the supplied executable without replacing a working language library. **Status:** done (2026-11-16, production pin `7aaa8700befa`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native sqlite3 executable and dependencies](done/t001.md) — **DONE** | 70m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit shared native consumers and preserve library and cache behavior](done/t002.md) — **DONE** | 35m | t001 |
| t003 | [Verify SQLite CLI Render parity across native authoring surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify the changed SQLite CLI supplier](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test real seed/import and readonly exports with healthy library controls](done/t005.md) — **DONE** | 40m | t003 |
| t006 | [Accept original fresh native defaults and close the milestone](done/t006.md) — **DONE** | 15m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · complete evidence (sweep 409, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch sqlite3 executable with complete required libraries is available through intended native build/pre-deploy/runtime/static phases, preserving the healthy Python sqlite3 module and shared primary/companion/env/PATH/root/cache/exclusion behavior. Meaningful real generated-image tests pass. After publication, both unchanged native clients' fresh original first Free Python SQLite services reach Live without an install prefix and pass exact source/program/SQL, independent library/CLI three-row checks, database/file/runtime/native-log proofs and exact cleanup/baseline preservation and quota accounting. Shared authoring contracts stay compatible and unverified scope is explicit. Install controls or source code alone do not close the milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep409,2026-10-17: both unchanged native POST201/exit0 first builds fail missing sqlite3 after healthy independent Python-library seed controls; both one-line install controls Live and six real readonly runtime exports pass.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting commands should work with named supplied utilities.
- **Expected outcome:** Native SQLite seed/import/export commands deploy and run without installing the executable in tenant code or rewriting CLI input.
- **Why now:** Both independent native clients and actual selected-base controls isolate the absent executable while the library and exact SQL/data work; there is no supplier or newer fix. PostgreSQL/Pandoc tool work has separate dependency and acceptance boundaries.
- **Render parity:** Included because native image contents affect tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard; no new schema field or CLI fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m94；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
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
- **Gate verified.** w8/m91 closed in `d66e4897e`. The newest `chore(deploy): pin platform images` commit is still `546945798`, which pinned `7aaa8700befa` at 16:22Z. `7aaa8700befa`'s `nativeCommonBase` is `sha256:5d6b8139…`, so production native builds start from the common image. Its SPDX attestation, fetched again from the registry for this check, lists `sqlite3`, `libsqlite3-0` and `libsqlite3-dev` `3.40.1-2+deb12u2`, with `/usr/bin/sqlite3` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)). `deploy/native-base/install.sh` installs it, and `check-tools.py` runs the CLI on Unicode, quoted and comma values and a failing query at every publication.
- **Which digest the acceptance runs on: the live `5d6b8139…`, now.** w8/335's re-pin to `sha256:4b809600…` (`71914199f`) is on main but not on production: `7aaa8700befa` does not contain it. The next push-triggered roll is due no earlier than about 22:22Z (six hours after the 16:22Z pin), and only the user can dispatch an earlier one. Running now is sound:
  - The re-pin's SPDX names the same 864 packages and versions as `5d6b8139`, so `sqlite3`, `libsqlite3-0` and everything they link are the same packages.
  - The `python313-original` case of the AMD64 utility cohort, which runs this milestone's frozen original program (the seed at build, and three readonly exports over HTTP on a network-off runtime), passed on both digests: families runs [43883107194](https://github.com/bex-co/bex/actions/runs/43883107194) (`5d6b8139`) and [43900534897](https://github.com/bex-co/bex/actions/runs/43900534897) (`4b809600`).
  - Hosted acceptance tests what production serves. Waiting for a roll that changes nothing under test would only hold the item.
  - Every hosted result names the pin it ran on.
- **Next steps (worker, no external gate):** close t001 on the adopted image; audit phases, the Python library and boundaries (t002); give the maintained cohort a SQLite omission control and import, seed-failure and readonly checks (t005); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t003, t006); simplify (t004).

## Accepted on production and closed — 2026-11-16 (loopx session)

The definition of done holds on production pin `7aaa8700befa` (common image `sha256:5d6b8139…`, `sqlite3` and `libsqlite3-0` 3.40.1-2+deb12u2). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified same-pin Render CLI, each running the frozen original seed and export commands on Free Python 3.13. The exact source, SQL and program proofs passed at build, where the CLI's readonly JSON export equalled the Python library's read of the same database and the reference rows. Six runtime requests exported the rows again from the build's byte-identical database, each checked against the library and the reference. The native app logs show all six requests ([t006](done/t006.md)).
- **Supplier.** Debian's `sqlite3` in the common image, linking the distribution `libsqlite3.so.0` that the Python primary's `sqlite3` module also loads, so the CLI and the module run one SQLite version and the module is left as it was ([t001](done/t001.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get the CLI at build only ([t002](done/t002.md)). No API, CLI or dashboard surface changed, and Render lists `sqlite3` at build and runtime on Debian 12 ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now also runs a CLI-only CSV import read back by the CLI and the library, a `-bail` seed that must stop at its first failing statement with the error named, and a refused readonly write, at compute build, network-off runtime and static build. A `missing-sqlite3` omission control, the cohort's first on a Python primary, must fail the original build on `sqlite3: command not found` while the library still answers ([t005](done/t005.md)). Before shipping they ran on Debian 12 arm64 with the identical package; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other Python lines on production rely on the families runs for the same digest.
  - SQLCipher, loadable extensions and the interactive shell are not covered.
- **Cleanup.** Both fixtures and URLs are gone, every Kratos jar was logged out and deleted, the isolated CLI state was removed, and other runs' resources were untouched.
