# w8 · m98 — Supply jq and rsync for native asset builds

**Worker:** worker8 **Goal:** normal native JSON-manifest and local asset-copy builds run with their documented utilities without tenant installation workarounds. **Status:** done (2026-11-19, production pin `dff52c596d13`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native jq executable and dependencies](done/t001.md) — **DONE** | 50m | [w8/m91/t007](../m91/done/t007.md) |
| t002 | [Supply a reviewed native rsync executable and dependencies](done/t002.md) — **DONE** | 70m | t001 |
| t003 | [Audit shared native bases, phases and both utility consumers](done/t003.md) — **DONE** | 35m | t001, t002 |
| t004 | [Verify jq and rsync Render parity across native inputs](done/t004.md) — **DONE** | 20m | t003 |
| t005 | [Simplify the two-utility supplier changes](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Test generated-image manifest generation and local copying](done/t006.md) — **DONE** | 60m | t004 |
| t007 | [Accept original first asset deployments and close the milestone](done/t007.md) — **DONE** | 20m | t005, t006 |

**Total:**7 tasks/270m. [Two independently isolated omissions](finding.md) · complete evidence (sweep 419, purged 2026-11-04).

## Definition of done

Reviewed immutable multiarch jq and rsync payloads with compatible required libraries/data are usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests independently catch each missing tool and verify exact combined JSON-manifest/local-copy/Unicode/space-name/byte/exclude/stale-own-output behavior, network-disabled runtime and meaningful failures. Audit intended bases/architectures/companions/env/PATH/root/cache/excluded paths. After publication both unchanged native clients' original fresh first Free Node services become Live without any install line and pass real source/program/data/manifest/copy/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; staged apt controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep419,2026-10-17: both original native POST201/exit0 first builds fail missing-jq; both jq-only builds generate correct manifests then fail missing-rsync; both complete-tool replacements Live with six real manifest/file-copy/HTML responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime tools should work as documented.
- **Outcome:** Native asset builds and runtime scripts can filter JSON and copy local files without rewriting commands or CLI requests.
- **Why now:** Actual independent native stages plus selected-base controls isolate two missing executables in one ordinary asset workflow; there is no supplier/newer fix. Shared delivery/audit/acceptance is coherent, while each missing tool has its own implementation and regression task.
- **Render parity:** Included because native image contents affect tenant builds/runtime through REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m98；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
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

- **Affected:** t001–t007. **Source:** verified evidence. No user decision was needed: the 2026-10-29 common-foundation decision and the 2026-11-01 D3 scope (production qualifies linux/amd64 only) both stand.
- **Gate verified.** w8/m91 closed in `d66e4897e` ([t007](../m91/done/t007.md) `status: done`). At 14:42Z the newest `chore(deploy): pin platform images` commit on `origin/main` was `a91e9aeb3` (pin `dff52c596d13`, 11:30Z), so no roll can start before 17:30Z. Its `nativeCommonBase`, and `main`'s, is `sha256:4b809600…`, the image w8/m95 and w8/m97 accepted on. That image's SPDX attestation, fetched again from the registry at 14:43Z, lists `jq` and `libjq1` `1.6-2.1+deb12u3` with `libonig5` `6.9.8-1`, and `rsync` `3.5.0+ds1-0+deb12u1` with `libpopt0`, `libxxhash0`, `liblz4-1`, `libzstd1`, `libacl1`, `libssl3` and `zlib1g` ([ADR060 D7 table](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01)). `deploy/native-base/install.sh` installs both, and `check-tools.py` filters JSON with `jq` and copies a tree with `rsync` at every publication.
- **Next steps (worker, no external gate):** close t001 and t002 on the adopted image; audit phases and boundaries (t003); give the maintained cohort more jq and rsync checks beside the existing `missing-jq` and `missing-rsync` omission controls (t006); replay both unmodified native clients on production with fresh Free hosted acceptance and exact cleanup (t004, t007); simplify (t005).

## Accepted on production and closed — 2026-11-19 (loopx session)

The definition of done holds on production pin `dff52c596d13` (common image `sha256:4b809600…`, `jq` 1.6-2.1+deb12u3, `rsync` 3.5.0+ds1-0+deb12u1). Details are in each task's closing record.

- **Both clients' fresh first builds reached Live** without an installer line: published bex v0.7.0 and the unmodified Render CLI at the lego/cli pin, each running the frozen original commands on Free Node 24. The original `jq` filter wrote the exact manifest and the original `rsync` copy produced exactly the three published files, excluding the draft and deleting the stale output, and the frozen validators passed. Three runtime requests per service repeated both in a fresh directory and passed the frozen oracle, and `/copied-index` served the exact copied `index.html`. The native app logs show every request ([t007](done/t007.md)).
- **Supplier.** Debian's `jq` with `libjq1` and Oniguruma, and Debian's `rsync` with its compression, hashing, ACL and TLS libraries, in the common image ([t001](done/t001.md), [t002](done/t002.md)).
- **Phases.** Build, pre-deploy and runtime run one image; static sites get both tools at build only ([t003](done/t003.md)). No API, CLI or dashboard surface changed, and Render lists `jq` and `rsync` at build and runtime on Debian 12 ([t004](done/t004.md)).
- **Maintained checks.** The utility cohort now also runs m98's own manifest and copy outside the original program, jq's raw, `@sh` and base64 output, Unicode regular expressions, `--arg`/`--argjson`, `--slurp` and exit statuses, and rsync's archive attributes, symlinks, excluded-file protection, itemized dry runs and trailing-slash rule, at compute build, network-off runtime and static build. `missing-jq` and `missing-rsync` now prove that only the program went before the original build fails at the first and second historical stages ([t006](done/t006.md)). Before shipping they ran on Debian 12 arm64 with the identical packages; their amd64 run is the `native-common-image.yml` push run of this commit.
- **Bounds:**
  - No hosted pre-deploy run: Free refuses it, and the Job runs the identical image.
  - ARM64 is out of scope (D3).
  - Other primaries on production rely on the families runs for the same digest.
  - Remote rsync transfers (a daemon or a remote shell) are not accepted.
- **Cleanup.** Both fixtures and their URLs are gone, the run's Kratos session was logged out right after the device approvals and its jars deleted, the isolated CLI state was removed, and other runs' resources were untouched.
