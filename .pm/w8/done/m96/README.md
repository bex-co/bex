# w8 · m96 — Supply GNU gettext for native message compilation

**Worker:** worker8 **Goal:** ordinary native internationalized apps can compile and use their message catalogs without installing promised system tools in tenant code. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed GNU gettext tools in native images](done/t001.md) — **DONE** | 70m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Audit shared native bases, locale behavior and exclusions](done/t002.md) — **DONE** | 30m | t001 |
| t003 | [Verify GNU gettext Render parity across authoring surfaces](done/t003.md) — **DONE** | 20m | t002 |
| t004 | [Simplify the GNU gettext supplier](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test real Django compilation and runtime catalog behavior](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [Accept original native defaults and close the milestone](done/t006.md) — **DONE** | 15m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · complete evidence (sweep 416, purged 2026-11-04).

## Definition of done

A reviewed immutable multiarch GNU gettext utility set and complete compatible library/data/locale closure are available through intended native build/pre-deploy/runtime/static phases. Real generated-image Django compilation, runtime external compilation, independent catalog parsing and exact Unicode/format/plural translations pass, with meaningful failures and audited sibling/architecture/phase/env/PATH/locale/cache/exclusion behavior. After publication both unchanged native clients' fresh original first Free services become Live without install lines, pass source/program/PO/catalog/translation/log/shared-readback checks and complete exact cleanup with baseline/current-inventory/quota reconciliation. Limits remain explicit; apt controls or source changes alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep416,2026-10-17: both valid native POST201/exit0 first builds fail Django compilemessages missing-msgfmt after successful dependency install; both one-line gettext controls Live with exact real compiled-catalog and six runtime translation responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime dependencies work as documented.
- **Outcome:** Standard translation builds and message utilities run without rewriting tenant commands or CLI requests.
- **Why now:** Both native clients and exact selected-base controls isolate the missing GNU utility while Django and catalog data work. No matching supplier/fix exists; adjacent utility issues have different consumers/closure requirements.
- **Render parity:** Included because native image contents affect tenant builds/runtime through REST/GraphQL/MCP/Blueprint/dashboard; no new product control or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m96；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $land 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。


## Shared foundation decision applied — 2026-10-31

The explicit2026-10-29 user choice of one common Bookworm tools foundation overrides the earlier worker-derived C design. [m91/t007](../m91/done/t007.md) supplies the foundation and exact runtime installation. This milestone retains its original programs/data/commands and functional/hosted DoD. There is no remaining user architecture decision. Already-started C publications may finish as retained evidence; no new automatic C matrix or catalog adoption.


## Drain disposition — 2026-11-01

**Blocked on w8/m91/t007, owned by the native build worker.** The common supplier and meaningful generated behavior/error harnesses are implemented. The current source audit and historical simplify/utility evidence are mapped in the shared evidence-to-task record (m91 evidence, purged 2026-11-04); [ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) now records chosen versions, phase/ABI and feature limits. This finishes the independent source/documentation preparation, not this milestone's acceptance.

Release this dependency after complete current-source common qualification on both architectures, the remaining runtime/companion/legacy/exact-donor cases, actual pre-deploy/required phase checks and reviewed production adoption. Then the same worker must run this milestone's unchanged original two-client hosted cases, shared readbacks, logs and exact cleanup/baseline/quota checks. The new e6a2386 AMD64 cohort passes; ARM64 is still running at this disposition. No original DoD or task acceptance is removed, and no task is marked done merely because supplier code or a test harness exists. No new architecture decision or user permission is needed.

## Drain disposition — 2026-11-17

The m91 gate cleared (m91 shipped in d66e4897e, and production runs the common Bookworm image). The `/loopx` unblock worker then did not reach a verified outcome. It hung mid-step twice. After the second stop at 10:46 UTC, its SendMessage resume never ran.

When it stopped, it was mutation-checking a 12-line `utility-check.py` addition (ITS data removed; C locale in place of C.UTF-8). It left these uncommitted:

- that addition;
- a 6-line `nativeutility_integration_test.go` change;
- a 1-line ADR060 edit;
- a half-done board move to `done/` that left every task `status: blocked`.

They are kept in the named stash `loopx/w8/m96`. **Gate:** `worker did not reach a verified outcome: no LOOPX-RESULT, leftovers uncommitted; needs a fresh worker to restore stash loopx/w8/m96, finish the mutation checks and the original two-client hosted acceptance, then ship or park`.

## Drain disposition — 2026-11-19

A second unblock worker restored stash `loopx/w8/m96` and recorded t001–t004 as done in the reopened milestone. It ran both clients' builds and app runs in bex-canary; its notes and raw logs are in the orchestrator's scratch `m96c/`, which is not committed. It then hung like the first worker: its 12:58 UTC Kratos logout call finished at the OS level, but the result never reached the worker. The orchestrator revoked its three QA sessions (rc 0), confirmed that no m96 fixture is live in bex-canary, stopped the worker, and parked the item again.

**Gate:** `worker did not reach a verified outcome: hung at its final logout, t005–t006 open, nothing committed; needs a fresh worker to restore stash loopx/w8/m96-2 (it supersedes loopx/w8/m96: same source edits plus the t001–t004 records), finish t005–t006, ship or park, then drop both stashes`.

## Unblocked: the gate holds, and the stash's work is restored — 2026-11-19 (loopx session, third worker)

- **Affected:** t001–t006. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 is closed ([t007](../m91/done/t007.md) `status: done`). At 17:49Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z). Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m97 to m102 accepted on. Its SPDX attestation lists `gettext` and `gettext-base` `0.21-12` (t001).
- **Stash `loopx/w8/m96-2` restored.** Its three source edits (the `utility-check.py` block, the `missing-gettext` omission and the ADR060 D7 row) were applied as a three-way patch, not checked out whole: m97–m102 had changed the same files since the stash's base `5a85568ae`. Its t001, t002, t004 and t005 records are kept; t003 and t006 were still open in it and are closed here. Its board move to `.pm/w8/m96/` was not restored; this record replaces it. Both `loopx/w8/m96*` stashes were dropped after this ship.

## Accepted on production and closed — 2026-11-19 (loopx session, third worker)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, Debian's GNU gettext 0.21-12). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without a system installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Python 3.13. Django's `compilemessages` found `/usr/bin/msgfmt`, where sweep 416's builds failed with `Can't find msgfmt`, and compiled the original French PO into the same 461-byte catalog (`641f0dd7…`) on both. The program's build proof passed the frozen validator: exact Unicode, `python-format` and plural translations through Django and Python's own `gettext`, and a fresh `msgfmt` compilation with the same bytes. Three runtime requests per service passed the frozen oracle, including a runtime `msgfmt` compilation and the raw catalog read back on the host, and the native app logs show every request ([t006](done/t006.md)).
- **Supplier.** Debian's `gettext` and `gettext-base`, with their libraries and ITS data, installed in the common image; nothing is downloaded and no tenant `apt-get` remains ([t001](done/t001.md)).
- **Phases.** The programs are on the one image that build, pre-deploy and runtime run, for every native primary; a static site has them at build ([t002](done/t002.md)). Render lists gettext for build and runtime. No API, CLI or dashboard surface changed ([t003](done/t003.md)).
- **Maintained checks.** The utility cohort now runs the original PO through `msgfmt --statistics`, an exact `msgunfmt`/`msgcmp` round trip, `xgettext` on AppStream metainfo through the packaged ITS rules, `msginit`'s French plural header, `envsubst`, the C library's `gettext`/`ngettext` lookup by `LANGUAGE` under `C.UTF-8` (and not under `C`), and named syntax and python-format errors, at the Node compute build, its network-off runtime and the static build. A new `missing-gettext` omission fails the original build with Django's `Can't find msgfmt`, the sweep 416 failure ([t005](done/t005.md)). Before shipping these ran, with 14 mutations each failing, on a linux/arm64 replica with the same package versions; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3), and the common image is bookworm only, as Render's runtimes are.
  - Only the `C.UTF-8` system locale exists, as on Debian's official Python images; no other system locale is claimed.
  - Other primaries on production rely on the families runs for the same digest.
- **Cleanup.** Both fixtures and their URLs are gone, and the baseline is restored. The run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
