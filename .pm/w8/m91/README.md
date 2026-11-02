# w8 · m91 — Supply PostgreSQL clients in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native backup-processing commands can use the supplied PostgreSQL client tools. **Status:** in progress — t007 implements the user-selected common Bookworm foundation; original functional and hosted acceptance remains open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t007 | [Implement the user-selected common Bookworm foundation](t007.md) | 240m | — |
| t001 | [Supply reviewed PostgreSQL client programs and libraries](t001.md) | 75m | t007 |
| t002 | [Audit shared native consumers and preserve image boundaries](t002.md) | 40m | t001 |
| t003 | [Verify Render parity across native authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify the changed client supplier](t004.md) | 15m | t003 |
| t005 | [Test real archive processing and shared toolchain neighbors](t005.md) | 40m | t003 |
| t006 | [Accept fresh native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**7 tasks/445m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep402/README.md).

## Definition of done

After publication, both native clients' fresh first original Free backup-processing builds reach Live without an installer prefix and serve actual runtime archive-to-SQL conversion with the exact source/archive/three-row proofs and native logs. A reviewed multiarch client/library supplier works through the documented native build/pre-deploy/runtime/static phases and intended client/server/archive versions; meaningful real generated-image tests and shared-primary/companion/env/PATH/cache/root/exclusion checks pass. Shared authoring surfaces preserve existing inputs/output contracts. Exact owned fixtures/children/URLs/local resources are cleaned and baseline/quota intact. Explicitly bound unverified cases. Code alone does not close this milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep402,2026-10-17: two unchanged POST201/native0 first builds fail missing pg_restore after exact source/backup proofs; both one-line install controls reach Live and six real runtime archive checks pass.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [ADR018](../../../docs/ADR018-render-parity.md) and [reviewed native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting build/start commands should work with named runtime utilities.
- **Expected outcome:** A native service can inspect/process PostgreSQL archives with supplied client programs, without bootstrapping them in app code or rewriting CLI requests.
- **Why now:** Both independent native clients reproduce the same missing program while primary tooling and exact input data work; the shared image path has no supplier or newer repair. Existing FFmpeg completion explicitly did not certify this tool family.
- **Render parity:** Included because one native image change affects tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard; no new schema field, CLI fork or product toggle.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m91；共同实现顺序见 w8/m91 的 design.md，13 项各自的原 DoD 均保留。
- **结论来源：** 现有 AGENTS.md / ADR008 的 Render parity 优先规则及本次源码复核，不是用户新决定。Render 官方 native-runtimes 工具表仍要求这些工具；当前 native recipe / 工作流没有覆盖它们的共享供应镜像。
- **采用方案：** m91/design.md 的 C：按已审阅官方基础镜像构建 bex 原生基础镜像，CI 发布 amd64/arm64，评审后锁 digest，所有 workspace 随后续原生构建采用。共享依赖、数据、头文件按发行版正常安装；不在每次租户构建中 apt-get，不用可选的偏离方案挡住 parity。沿用现有 runner / GHCR，不新增云资源；镜像体积和刷新耗时由 worker 实测记录。
- **剩余门槛：** 无用户决策门槛；发布产物、锁定及线上验收都是尚未完成的 worker 工作。需要提交时仍遵守仓库显式 $ship 规则。
- **worker 下一步：** m91 先建立可复现共享供应链；其余项复用该机制逐工具做真实功能回归。保留 bookworm/trixie、amd64/arm64、主运行时/companion、build/pre-deploy/runtime/static、PATH/缓存及排除路径的原验收，不能仅凭 --version 或配置存在关闭。发布前复核所选版本和上游摘要。

## Supplier implementation checkpoint — 2026-10-29

The publishing factory and offline utility gates are implemented for all 27 current reviewed lines. Real local transformations passed on Node 24 amd64/arm64, Node 14 Bullseye and Elixir 1.18 Trixie; PostgreSQL 18 archive processing, live psql/pg_dump and malformed-input control passed. A real generated exact-OTP-28.0 build and network-disabled runtime passed using a genuine immutable local Bookworm supplier. Primary Python 3.7/3.8/3.14 metadata, matching C headers/extensions, wheel installation, local import precedence and post-tree-replacement restoration passed. These supplier tests do not replace the individual tool milestones' larger hosted/phase acceptance.

Three final simplify reviewers cleared the mechanism after removing an unused config field and avoiding wheel downloads on non-Python bases. The production supplier catalog remains empty until actual published digests, signatures, SBOMs and anonymous pulls are reviewed. No source-only task or milestone is closed here; publication, adoption and the original fresh two-client hosted DoD remain active worker work. See `deploy/native-base/README.md` and the implementation review in design.md.

## Generated-image and loader checkpoint — 2026-10-29

The [current-generator bundle](evidence/current-generator-20261029/README.json) records thirteen original program/data cases and ten extended behavior/error checks, thirty-nine offline runtime HTTP/artifact proofs and the same checks through a command executed on the generated image, plus independent static gzip/ZIP decoding. The command test is not a Kubernetes pre-deploy Job; its Python cases use the system companion, not the original primary-Python hosted fixtures. The supplier-removed generated baseline fails on the actual PostgreSQL dependency. Original hosted DoDs stay open.

The [exact Python default proof](evidence/exact-python-default-20261029/README.md) found and fixed a stale loader cache after exact primary replacement. Python 3.14.8, eighty shared objects, actual `dlopen`, packaging/C-extension behavior, patched Brotli and the genuine compressed SPDX SBOM pass locally on arm64. Both empty and populated catalogs now run meaningful resolver/neighbor checks, including upstream-retaining backports. The production catalog is still empty; published artifacts, adoption and actual hosted lifecycle checks remain active work.

## Maintained generated-image checkpoint — 2026-10-30

The [new maintained gate and genuine Python 3.13 evidence](evidence/maintained-python313-20261030/README.json) preserve six original authored Python programs/data/validators and verify all six builds plus eighteen runtime HTTP proofs on the actual primary interpreter. A genuine unchanged Python 3.13 supplier and an explicitly separate offline application/Poetry wheel-cache layer replace the earlier companion-interpreter limitation for those cases. The local arm64 gate also covers eleven utility behavior/error groups through generated Node compute/static images, the original Node packaging/C-extension program and an actual supplier-removed failure.

The opt-in `TestNativeUtilityGeneratedImages` and bounded testdata live outside the factory context. Package tests, lint, twelve existing populated-catalog unit/host-shell neighbors and full operator/codegen checks pass. Three simplify reviews cleared the code after inlining a one-use runner and dropping unused callback state; all four real generated-image cases passed again after those wrapper-only cleanups. Frozen original programs, data and validators are unchanged. The bundle hashes raw/gzip evidence and distinguishes the earlier full-suite run from the final image replay.

This is local execution evidence. Published amd64/arm64 suppliers, actual production adoption, original fresh two-client hosted acceptance, Kubernetes pre-deploy/static publication and owned hosted cleanup remain open. The production supplier catalog is still empty.

## Actual local pre-deploy and amd64 default checkpoint — 2026-10-30

The [dev-8 Job proof](evidence/dev8-predeploy-20261030/README.md) executes the canonical controller-generated PodTemplate with the retained immutable Node image: thirteen original programs, thirty-nine loopback HTTP proofs, ten utility groups and env/file/credential checks pass. An omitted-pull-secret revision remains unstarted with authentication denial even after caching the image. Exact owned objects, Pods and the isolated registry proxy are removed; baseline UIDs are unchanged. It uses companion Python 3.11 and a metadata owner wrapper, so it does not establish primary-Python, native-init or full controller/backend release behavior.

The [genuine amd64 exact-default proof](evidence/exact-python-default-amd64-20261030/README.md) independently verifies Python 3.14.8, all eighty shared libraries, actual loading/C-extension behavior and patched Brotli in full genuine Syft/SPDX archives. It uses the reviewed Python primary with the unchanged factory recipe, rather than the earlier bounded generic supplier. Published-supplier replay, actual production adoption and original fresh hosted acceptance remain open; no task is closed by these local checkpoints.

## Publication recovery checkpoint — 2026-10-30

[Actual failed-job evidence](evidence/publication-recovery-20261030/README.md) separates Node 14/16 scan-analysis deadlines from Node 18's pre-build registry transport timeout. The narrow fifteen-minute scan fix shipped as `059c163b3`, retaining every scan/probe/signature and existing runner. Its new matrix is queued; no failed candidate or cache was approved, no running compile was cancelled, and no successful publication/adoption is claimed.

## Complete synthetic-catalog suite checkpoint — 2026-10-30

The [full operator suite with a complete synthetic catalog](evidence/complete-catalog-suite-20261030/README.md) passed in 234.906 seconds, including build/native-resolve tests and controller/envtest. A Go overlay supplied twenty-seven explicitly synthetic references; image integration gates were unset, and all 550 source/codegen/RBAC hashes remained identical. The real source and empty production catalog were unchanged. This checks broader populated-catalog assumptions; it does not establish published suppliers, actual adoption or hosted acceptance, which remain open.

## Actual local native-init coexistence checkpoint — 2026-10-30

The [actual native-init coexistence proof](evidence/native-init-coexistence-20261030/README.md) ran `/native-init` as PID 1 with thirteen original programs, thirty-nine HTTP proofs and ten utility groups. PATH/literal/empty environment checks, child exit 37 and forwarded SIGTERM/exit 23 pass; all four new owned containers and the generated image are gone. The canonical generator/resolver, `StageNativeInit` and BuildJob named-context metadata are retained, while actual execution used Docker BuildKit, not a production buildctl container or Kubernetes build Job lifecycle. This is local arm64 Node with companion Python 3.11; it adds no primary-Python 3.13, amd64, publication, adoption or hosted claim. Those acceptance gates remain open.

## Fresh acceptance identities and baseline checkpoint — 2026-10-30

The [preparation refresh](evidence/acceptance-preparation-refresh-20261030/README.md) changes only unused display names/date arguments and adds UTC/prior-intent guards, preserving all twenty-eight original program/data/command identities. A removed foreign QA fixture has a documented cleanup and independent exact 404; historical IDs remain protected, and baseline reclassification requires fresh evidence before any own create. Pure guard checks pass. No native hosted request or acceptance is claimed; actual adoption and the fresh own-authenticated baseline remain required.

## Python 3.7 recovery eligibility checkpoint — 2026-10-30

[Python 3.7 publication evidence](evidence/publication-python37-recovery-20261030/README.md) shows an actual successful build, both runtime architectures and both scans, followed by a first-SPDX Rekor POST failure with no underlying status exposed. Exact anonymous metadata and unchanged recipe/base checks pass. A data-only authorization permits only that immutable artifact through the existing full-gate recovery path; no new dispatch or approval is claimed. The same phase also failed in Node 14 recovery, so report-size/API behavior is being investigated before repeating it. Actual publication, full catalog adoption and hosted acceptance remain open.

## Full-report signing recovery checkpoint — 2026-10-30

[Repeated first-SPDX failure and protocol evidence](evidence/publication-rekor-v2-20261030/README.md) includes a genuine 25 MB exact-candidate report and a verified Cosign 3.1.4 local experiment producing a 415-byte v2 log request from an unchanged 23 MB predicate. The narrow workflow fix pins 3.1.4 and explicitly selects its official TUF v2 signing configuration while preserving full SPDX/Trivy reports and every identity/runtime/scan/transparency gate. Independent review, workflow validation and actionlint pass. These local protocol checks do not approve public signatures; actual CI replay, per-candidate review, full catalog adoption and original hosted acceptance remain open.

## First published supplier review — 2026-10-30

The [published Node 14 review](evidence/published-node14-20261030/README.md) verifies the actual immutable supplier on amd64 and arm64: separate producer/verification identities, unchanged recipe/base, successful current runtime and scan gates, all five full signed bundles, strict source/issuer/subject/transparency/TSA checks and anonymous image-byte proofs pass. The compact archive preserves immutable object IDs, complete-predicate retrieval and hashes, exact Docker-source root derivation and meaningful rejection guards. Full payloads stay outside Git.

Critical scan findings, including the custom Brotli backport flags, remain recorded; this is no security waiver or zero-CVE claim. Only this supplier evidence is approved. The production catalog is unchanged, and other supplier reviews, complete adoption and the original generated/hosted acceptance remain open. No task is closed by this checkpoint.

## Three published supplier reviews — 2026-10-30

The same strict review now passes for [Node 16 Bookworm](evidence/published-node16-20261030/README.md) and [Python 3.7 Bullseye](evidence/published-python37-20261030/README.md), alongside the earlier Node 14 supplier. Each has five complete signed bundles, independent amd64/arm64 report and image-byte bindings, current successful runtime/scan gates, exact producer/recovery inputs and actual wrong-source/wrong-subject rejection controls. The generic native-only review driver and compact full-predicate replay records are retained; complete report bodies remain outside Git in verified gzip/public immutable OCI objects.

All critical findings remain visible, including the Bullseye custom-backport flags. These are supplier identity/runtime/report reviews, not security waivers. The full 27-line catalog, production adoption, exact generated-image replay and original hosted DoDs remain open; no task or milestone is closed.

## Verified recovery retirement — 2026-10-30

[Fresh source-specific discovery reads](evidence/recovery-retirement-20261030/README.md) match the approved Node16/Python3.7 index bytes; their temporary recovery authorizations are retired. Other recovery records remain unchanged. This cleanup adds no supplier adoption or hosted acceptance claim; all original tasks remain open.

## Four published supplier reviews — 2026-10-30

[Python 3.8 Bookworm](evidence/published-python38-20261030/README.md) now passes the same strict amd64/arm64 supplier review, bringing the total to four of twenty-seven. Its five complete signed bundles, exact producer/recovery recipe inputs, independent architecture reports, actual compressed/decompressed layer proofs and all thirty-five anonymous image blobs pass. The final pass used bounded resumable layer files after transport truncation, preserving every artifact gate; failed attempts, 43 guards and proof-linked cleanup remain recorded.

The signed scans retain 229 critical findings per architecture, with no Brotli findings. This is supplier evidence approval only. The production catalog remains empty; the remaining twenty-three reviews, full adoption, exact generated-image replay and original hosted DoDs remain open. No task is closed.

## Corrected current publication scheduling — 2026-10-30

[The current queue receipts](evidence/current-publication-queue-20261030/README.md) retain the guarded retirement of an entirely queued obsolete full matrix and exactly16 targeted dispatches on the HTTP1/complete-retry source. The old full matrix had latent rows, not missing definitions; no running build was interrupted. Four approved suppliers and seven already-active targets were excluded. Catalog/adoption and all original hosted acceptance remain pending.

## Six suppliers independently verified — 2026-10-30

[Python 3.9](evidence/published-python39-20261030/README.md) and [Python 3.10](evidence/published-python310-20261030/README.md) passed complete strict review on the corrected fd358 workflow, including all five signed bundles, independent architecture reports and fresh complete public blob reads. Their 12,638/7,477 findings per architecture remain retained. These join Node 14/16 and Python 3.7/3.8 for six of 27 approved suppliers. [Fresh discovery checks](evidence/recovery-python39-python310-retirement-20261030/README.md) confirm the already-shipped temporary recovery retirement. The catalog remains empty; 21 suppliers, complete adoption and all original hosted acceptance remain pending.

## Seventh supplier independently verified — 2026-10-30

[Fresh-build Ruby 3.0](evidence/published-ruby30-20261030/README.md) passed the unchanged strict review, all five full signed bundles and 33 fresh public blob reads totaling 1.159 GB. Its reports retain all findings, including reported Brotli CVEs; runtime backport probes passed independently. Seven of 27 suppliers are verified. The remaining 20, complete catalog/adoption and all original hosted acceptance remain pending.

## Catalog adoption guards — 2026-10-30

[The before/after checks](evidence/catalog-coverage-20261030/README.md) close two review gaps before adoption: canonical freshness now covers `nativebases.go`, and the existing all-line resolver test validates and exercises the real catalog whenever populated. The old checks accepted an uncovered supplier digest and passed a deliberately partial one-entry map; the new checks reject both, while empty and complete synthetic catalogs pass. Full operator/codegen/envtest and corrected full-workspace lint/deadcode passed; the initial incorrect `GOWORK=off` lint invocation is retained in the evidence.

Seven of 27 suppliers remain verified; the production catalog is empty. Twenty reviews, complete adoption and original hosted acceptance remain active worker work. No task is closed by this guard.

## Full SBOM transport recovery — 2026-10-30

[Independent actual Ruby 3.2 failure and primary-source transport evidence](evidence/publication-syft-http1-20261030/README.md) confirm Syft's registry HTTP2 stream fault. Concurrent peer fix `3990b9acf35f62ba1d702387bc2061a83342b2b3` already supplies HTTP1, bounded complete SBOM retries and the same exact-index recovery record; its implementation is preserved. The independently reviewed job-env proposal was superseded before push, so this checkpoint ships evidence only. Seven suppliers remain approved; Ruby 3.1 strict review is active. Corrected publication, all-27 adoption and all original hosted DoDs remain open.

## Bounded artifact-review concurrency — 2026-10-30

[The reviewed private V9 tool](evidence/native-review-concurrency-20261030/README.md) verifies final anonymous image bytes with bounded parallel streams while retaining canonical hash/size/cache/auth checks and requiring every result before approval. Four meaningful local concurrency/corruption controls, the existing 43 strict guards and three reviews pass. This is tool validation only; no actual candidate has run on V9, and Ruby 3.1 remains on V8. Seven suppliers are approved; complete adoption and the original hosted DoDs remain open.

## Isolated verifier cache and byte diagnostics — 2026-10-30

[The reviewed private V10 tool and exact-source diagnosis](evidence/native-review-tuf-isolation-20261030/README.md) isolate strict Cosign TUF caches and retain whitelisted byte-failure counters. Two rejected Ruby 3.1 V8 attempts remain explicit: one unclassified size/SHA rejection and one incomplete report pair after TUF persistence failure. Four concurrency, six focused and43 strict controls pass; a new complete validation is assigned, with no artifact approval yet. Seven suppliers remain approved, and all original hosted DoDs remain open.

## Complete-layer scanner recovery — 2026-10-30

[Actual Elixir failures, pinned scanner source and reviewed staging controls](evidence/publication-trivy-staging-20261030/README.md) support a coverage-preserving publication fix: complete remote layers are staged and verified before tar analysis, databases finish outside each full-scan budget, and owned temporary storage is measured and bounded. Both immutable recovery candidates repeat all current runtime, complete scan, SBOM and signing gates. Local controls and repository checks pass; actual runner capacity and successful corrected publication remain unproved. Seven suppliers remain approved; no task or original hosted DoD is closed.

## Bounded ranged body review — 2026-10-30

[Rejected short-body evidence and reviewed V11 controls](evidence/native-review-ranged-streams-20261030/README.md) retain all earlier Ruby3.1 failures and add bounded contiguous range resumption while preserving the canonical whole-SHA/size/cache and strict trust gates. Eleven new range controls plus existing concurrency/isolation/strict guards and three source reviews pass. One new complete validation is assigned; no supplier approval or original hosted DoD is claimed. Seven suppliers remain approved.

## Corrected Elixir publication targets — 2026-10-30

[The exact scheduling receipts](evidence/corrected-elixir-publication-queue-20261030/README.md) confirm guarded retirement of only this session's duplicate full push request and one corrected publication each for Elixir1.18/1.19 on846a560. Actual runtime job identities are queued; old failures remain retained. No running compile was cancelled and no supplier is newly approved. All original hosted acceptance remains pending.

## Eighth supplier independently verified — 2026-10-30

[Fresh-build Ruby3.1](evidence/published-ruby31-20261030/README.md) passed a new complete V11 validation: all33 blobs/1.354GB, five strict signed bundles, independent architecture reports and actual negative controls. Twenty-one genuine prior complete reads retain custody and fresh HEAD checks; twelve new reads include both previously rejected blobs from zero. Root rechecked the full frozen chain and raw/gzip custody. Earlier failures remain rejected and the derived-gzip loss remains explicit. All13,690 findings/228critical per architecture are retained. Eight of27 suppliers are approved; the production catalog and original hosted acceptance remain pending.

## Elixir1.17 publication recovery — 2026-10-30

[Actual failed signing log and immutable recovery intake](evidence/elixir117-publication-recovery-20261030/README.md) show both architecture runtime/scan gates passed before old-source Syft's HTTP2 fault. The current workflow already contains the transport fixes; a minimal exact-index recovery row repeats all current gates without rebuilding or launching a full matrix. No new supplier approval or hosted acceptance is claimed; eight suppliers remain approved.

## Concurrent fixed-profile CI queue handoff — 2026-10-30

Concurrent `w1/m188` is rolling ten unstarted modern99f requests forward to the already-fixed83c/39b2 publication profile after the repeated old scan/Syft failures. All ten replacement requests are bound to83c/main/attempt1, with the factory, official bases and27-line matrix unchanged; each target had a fresh all-source duplicate check. The [w1 board](../../w1/README.md) links the immutable plan, explicit approval and21:13 request-phase receipts.

Each old request retires only after its correct replacement native job exists and a fresh old-job read still shows queued/runner0/zero steps. Started Node24/Go1.27/Elixir1.20 work and the three legacy requests are preserved; modern obligations are retained. Supplier review, full27 adoption and original hosted DoDs remain owned byw8/m91. Dispatch and queue handoff add no supplier approval or task closure.

**Queue-handoff completion — 2026-10-30 21:32Z:** all ten replacement requests now have actual83c native jobs. Each old99f request received one cancellation only after a fresh queued/runner0/zero-step check; all ten old runs/jobs are independently confirmed completed/cancelled with no started work. The w1 record retains raw bindings, terminal receipts, earlier indexing/pending/HTTP502 observations and an independent raw-API audit. All modern obligations and all started work remain preserved. This adds no supplier approval, catalog adoption or hosted/task closure.

## Concurrent Go1.27 utility repair — 2026-10-30 23:14Z

The preserved old99f Go1.27 build completed, but the ARM64 real offline tool gate failed because `unzip` was absent; AMD64 verification was canceled. Concurrent w1 shipped `8af93065a`, adding only explicit `unzip` to the shared `--no-install-recommends` package list, with all other non-PM files identical to the reviewed83c profile. The real archive gate remains unchanged. The old Go index is not eligible recovery or supplier approval.

The fix's own automatic full push request41142599053 was canceled once before any native build started and is confirmed terminal. One Go-only normal rebuild is bound to8af/main/attempt1, run41142870861/job124361678277; build/runtime/scan/sign success is still pending. Existing modern requests and three legacy requests are preserved. The [w1 board](../../w1/README.md) links immutable failure, source, scheduling and exact historical-recipe compatibility evidence. Existing successfully gated images retain their exact old producer/verifier identity; this CI repair adds no supplier approval, production catalog adoption or hosted/task closure.

## Concurrent Elixir1.20 normal rebuild — 2026-10-30 23:33Z

Preserved old99f Elixir1.20 passed its build, both runtime gates and full scans, then failed during ARM64 registry Syft layer download with HTTP2 PROTOCOL_ERROR. Log order supports three completed signing/attestation commands; it is not a complete independent five-proof acceptance. The existing workflow fixes this transport path, but the explicit-unzip installer change makes the old index ineligible for same-recipe recovery.

Concurrent w1 retained the first exact800 preflight with zero POSTs, reviewed the PM-only9cb update against8af, checked all recent and known nonterminal requests, and submitted exactly one normal build. Actual native binding is9cb/main/attempt1, run41144656779/job124367871533, queued at first observation. No old-index authorization, cancellation, supplier approval or hosted/task closure is claimed. The [w1 board](../../w1/README.md) links frozen logs, dedupe, literal source predicate and actual run/jobs; independent Elixir supplier review remains w8/m91's work.

## 用户决定（2026-10-29，`/unblock w8` 主题 1）——单一 Bookworm 公共底优先

- **涉及 ID：** w8/m91、m93、m94、m95、m96、m97、m98、m99、m100、m101、m102、m103、m107（另见 w8/222）
- **结论来源：** 用户明确决定。用户先问「Render 是怎么做的」，得到解释（Render 只有一个 Debian 12 公共环境，工具预装，语言版本按环境变量装在上面）后，选择**单一 bex native 公共底**：一个 bookworm 底预装 Render 工具表，语言版本装在上面；没有选 design.md 的选项 C（按语言线构建镜像）。
- **冲突处理：** 明确用户决定覆盖较早 worker 按 parity 推断的选项 C。已开始的按语言线发布可收尾并保留真实证据，最终采用单一 Bookworm 公共底；无需把同一决定再交回用户。
- **剩余工作：** [t007](t007.md) 实现并验证共同底与精确运行时安装，复用已有包、工具、修补和发布证明。停止自动扩大 C 发布，不取消已开始的工作；共同底采用和原28个程序/数据/命令的真实验收仍由 worker 完成。

## Bounded dependency-transfer repair — 2026-10-31

[Actual Python3.11/Python3.13 download failures and repair](evidence/native-fetch-transport-20261031/README.md) retain the terminal job identities and private raw-log hashes. Wheel transfer retries are bounded to three fresh attempts and keep checksum/metadata checks final; PGDG's pinned key fetch now retries TLS transport failures within explicit time limits. Native matrix/transfer and OCI staging regressions pass. The old Rust installer failure remains recorded separately. This is recovery of already-started publication work; no new supplier approval, common-base design choice, adoption or hosted/task closure is claimed.

## Ninth supplier and JavaDB deadline repair — 2026-10-31

[Python3.14's exact published supplier](evidence/published-python314-20261031/README.md) is root-approved after the complete genuine V11 review:35 full blobs/1,197,437,263B, five strict proofs, two actual negative controls and both report bindings. Approvals are9/27; its Python3.14.10 does not close the exact3.14.8 default/222. All original hosted cases and catalog adoption remain open.

[Node26's actual JavaDB transfer](evidence/java-db-budget-20261031/README.md) reached86.08% before the21-minute deadline/10-second kill grace. Only the separate JavaDB-prefetch outer budget grows to30minutes; all complete image-scan gates stay unchanged. This fixes an existing publication budget; replacement execution remains unproved.

## Tenth published supplier — 2026-10-31

[Root-approved Ruby3.4](evidence/published-ruby34-20261031/README.md) binds source83c run41129277963/job124323020658, five strict signed proofs, two actual negative controls, both full report pairs and33 fresh complete anonymous blobs/1,192,208,302B. Root independently checked196 frozen payloads,22 Git recipe files/modes, all five signed-subject/canonical-predicate roundtrips and seven TUF roots. Both images report Ruby3.4.12; original generated/default and hosted cases remain separate. Published approvals are10/27, with catalog adoption and the common-base implementation still open. All findings remain retained.

## Eleventh published supplier — 2026-10-31

[Root-approved Python3.12](evidence/published-python312-20261031/README.md) binds source83c run41129082245/job124318545123. Five strict signed proofs, two actual negative controls, both full report pairs and all35 descriptors/1,188,703,761B are covered. Root independently checked194 frozen payloads,22 Git recipe files/modes and all five signed-subject/canonical-predicate roundtrips. Of the blob bytes,500,339,368B were26 new complete reads;688,364,393B were nine genuine prior Python3.14 reads, each matched against its approved receipt/journal and an actual fresh anonymous HEAD. No unselected cached entry counts. Published approvals are11/27; generated/default, hosted, common-base design and catalog adoption remain open.

## Twelfth published supplier — 2026-10-31

[Root-approved Node22](evidence/published-node22-20261031/README.md) binds source83c run41128855169/job124317237455. Five strict signed proofs, two actual negative controls, seven fresh TUF roots and both complete report pairs pass. Root checked all194 frozen payloads,22 Git recipe files/modes and all five signed-subject/canonical-predicate roundtrips. All35 descriptors/1,228,736,252B are covered:26 new complete reads/540,371,859B and nine genuine prior Python3.14 reads/688,364,393B, each matched to the approved receipt/journal and an actual fresh anonymous HEAD. Both images contain Node22.23.4. Published approvals are12/27; original generated/default and hosted acceptance, common-base implementation and catalog adoption remain open. All findings remain retained.

## Thirteenth published supplier — 2026-10-31

[Root-approved Go1.27](evidence/published-go127-20261031/README.md) binds exact8af source, run41142870861/job124361678277 and immutablef5c7b79a. All31 descriptors/1,216,838,007B were fresh complete anonymous reads with exact SHA/size. Root independently checked199 frozen artifacts,22 Git recipe files/modes, five strict signed proofs, two actual negative controls, seven fresh TUF roots and all five signed-subject/canonical-predicate roundtrips. Both images report Go1.27.3; all findings remain retained. Published approvals are13/27; original generated/default, hosted, user-selected common Bookworm implementation and catalog adoption remain open.

## Three fixed publication requests bound — 2026-10-31

[The complete plan and actual dispatch custody](evidence/targeted-native-rebuilds-20261031/README.md) records exactly one singleton normal build each for Python3.11, Python3.13 and Node26. Root independently checked all71 plan and133 execution files, direct HTTP200 returned run identities and all30 native Git inputs against the fixed2cc profile. The actual runs are41190561558/41190693730/41190806242, with successful matrices and queued build jobs at the frozen observation. No CI completion, new supplier approval or hosted acceptance is inferred from dispatch. Earlier actual failures and the stopped preflight remain retained.

## Common Bookworm first source slice — 2026-10-31

[The reviewed common foundation and resolver source](evidence/common-bookworm-source-20261031/README.md) follows the recorded user choice: one Bookworm tools layer, exact/default primary installation before companions, shared offline tooling and the existing complete publication gates. New automatic C matrices retire; historical singleton finish remains. Full operator tests and scoped lint pass; full workspace lint is capacity-incomplete with every stopped/failed attempt retained. Both production pins remain empty. Actual common publication, generated Node24/Python3.13/exactdefault3.14.8 images on both architectures, complete legacy/OTP compatibility and all original hosted DoDs remain open.

## Node20 immutable supplier evidence — 2026-10-31

[Root-approved Node20](evidence/published-node20-20261031/README.md) preserves the distinct BF producer and 6fd verifier and successful historical recovery run41113373387/job124263887574. Root checked all264 original payloads, both22-file Git recipe/mode sets, all37 fresh complete anonymous blobs/1,311,620,069B, five strict signed proofs, two actual negative controls, seven fresh TUF roots and all five full signed-bundle/predicate roundtrips. Both architectures report Node20.20.2; complete SPDX and Trivy findings remain retained without waivers. Published supplier approvals are14/27. This completes already-started C evidence only; the user-selected common Bookworm generated image, original functional/hosted acceptance and production adoption remain open.

## Complete original tool replay source — 2026-10-31

[Maintained replay source and controls](evidence/original-node-replay-source-20261031/README.md) now cover all13 original one-client programs (7Node24,6primaryPython3.13), preserving the seven prior specs and every original authored command/data. Seven offline controls and the full build-package suite pass; the original28 two-client hosted specs are unchanged. The generated-image gate remains skipped until a real immutable common candidate is selected. This is source preparation only: per-supplier omission controls, both-architecture compiled execution and original hosted acceptance remain open.

## Common runtime compatibility source — 2026-10-31

[Phase2 source, review findings and controls](evidence/common-bookworm-compat-source-20261031/README.md) preserve all27 reviewed selections above one common Bookworm final base, including exact OTP/Elixir/rebar, legacy distinct-SONAME libraries and exact EOL Node repair. Two source review defects were corrected and focused suites/lint/source controls pass. All19 source files/modes were independently checked. No common/runtime artifact is adopted: actual package/compiler/native-extension/ABI, both-architecture generated-image/SBOM and original28 hosted acceptance remain open. t007 stays in progress.

## Independent supplier omission controls — 2026-10-31

The common generated-image harness now prepares separate jq/rsync and Python metadata/pip/header omission subtests using test-only removal layers and exact original m98/m103 commands. Healthy independent prerequisites, a fresh actual BuildKit stdout probe and the later specific failure are required. Eight offline wrapper tests, five evidence-parser subcases and the full build-package suite pass; the Docker/image omission subtests remain unexecuted. [Source checkpoint](evidence/independent-supplier-controls-source-20261031/README.md) preserves this distinction. No original program/data/command, production pin or task completion changed.

## Legacy Python default Poetry bootstrap source — 2026-10-31

[Four-file source repair and regression evidence](evidence/legacy-python-poetry-bootstrap-source-20261031/README.md) use the common system interpreter only to bootstrap default Poetry2.1.3 beside Python3.7/3.8. App/tool versions, app ENV/PATH, explicit compatibility, uv and companions are preserved. Thirteen source controls pass; the pre-fix resolver fails exactly seven affected cases, and scoped suites/final live lint pass. Actual factory/tool/project/generated-image/architecture/hosted behavior remains unexecuted and the production common pin is empty. ADR060 records this source amendment without closing t007 or changing the original DoDs.

## Node18 immutable supplier evidence — 2026-10-31

[Root-approved Node18](evidence/published-node18-20261031/README.md) binds the same `dbc493ba` producer/verifier source and successful run41187749759/job124498652791. Root checked all190 original frozen payloads, four mirrored source files plus28 immutable Git source files/modes, all37 fresh complete anonymous blobs/1,313,080,570B, five strict signed proofs, two actual rejected source/subject controls, seven fresh TUF roots and all five full signed-subject/canonical-predicate roundtrips. Both architectures report Node18.20.8; complete SPDX and Trivy findings remain retained without waiver. Published supplier approvals are15/27. Common Bookworm generated-image qualification, exact Python3.14.8/default, original hosted acceptance and production adoption remain open.

## Common generated-image qualification wiring — 2026-10-31

[Reviewed CI/test source](evidence/common-generated-qualification-source-20261031/README.md) adds two actual-architecture generated-image jobs after the fully gated singleton common digest, with target-runtime prerequisite wheels, original13-program replays, separate omission checks and exact Python3.14.8 full Syft/SPDX binding. Independent review found and corrected the config-ID/manifest-root mismatch using real retained reports; both original formats and lossless custody remain. All six source files and43 initial/30 corrected frozen payloads were checked, and source/unit/Actions/lint controls pass. Actual new image execution remains unexecuted; t007, all27-selection compatibility, exact-default222 and original28 hosted/adoption acceptance remain open.

## Python3.11 immutable supplier evidence — 2026-10-31

[Root-approved Python3.11](evidence/published-python311-20261031/README.md) binds the same `d95dcd88` producer/verifier source and successful run41190561558/job124506595632. Root checked all186 frozen payloads, four mirrored files plus28 immutable Git source files/modes, all35 fresh complete anonymous blobs/1,187,301,419B, five strict signed proofs, two actual rejected source/subject controls, seven fresh TUF roots and all five full signed-subject/canonical-predicate roundtrips. Both architectures report Python3.11.18; full findings remain retained without waiver. Published supplier approvals are16/27. Common Bookworm qualification, exact Python3.14.8/default, original hosted and production adoption acceptance remain open.

## Qualification trigger freshness — 2026-10-31

[The three-file trigger repair](evidence/common-trigger-freshness-source-20261031/README.md) makes native generator/assembly/resolver and original-fixture changes trigger the singleton common publication and both-architecture generated qualification. Parsed actual-YAML regression checks fail six required selections on the previous workflow and pass after the narrow path additions; unrelated controller/backend/PM changes remain excluded. Source/unit/lint/Actions checks pass. Actual all-family/common/hosted/default/adoption gates remain open.

## Python3.13 immutable supplier evidence — 2026-10-31

[Root-approved Python3.13](evidence/published-python313-20261031/README.md) binds the same `d95dcd88` producer/verifier source and successful run41190693730/job124506987325. Root checked all186 frozen payloads, four mirrored files plus28 immutable Git source files/modes, all35 fresh complete anonymous blobs/1,193,025,253B, five strict signed proofs, two actual rejected source/subject controls, seven fresh TUF roots and all five full signed-subject/canonical-predicate roundtrips. Both architectures report Python3.13.18; full findings remain retained without waiver. Published supplier approvals are17/27. Common Bookworm qualification, exact Python3.14.8/default, original hosted and production adoption acceptance remain open.

## Legacy default Poetry dependency compatibility — 2026-10-31

[The narrow two-file source repair](evidence/legacy-poetry-dependency-source-20261031/README.md) caps the private virtualenv dependency only for common system-interpreter/default Poetry2.1.3 beside Python3.7/3.8, following the tagged upstream workaround without changing app/Poetry versions or explicit/modern/historical/uv routes. Fourteen recipe controls, seven expected old-source failures, scoped tests/lint and independent review pass; all24 frozen payloads and two live sources were checked. Actual old-project, seed, image/architecture and hosted compatibility remain unexecuted, and no adoption is implied.

## Node26 immutable supplier evidence — 2026-10-31

[Root-approved Node26](evidence/published-node26-20261031/README.md) binds the same `e48dd4a6` producer/verifier source and successful run41190806242/job124507306637. Root checked all182 frozen payloads, four mirrored files plus28 immutable Git source files/modes, all33 fresh complete anonymous blobs/1,238,687,654B, five strict signed proofs, two actual rejected source/subject controls, seven fresh TUF roots and all five full signed-subject/canonical-predicate roundtrips. Both architectures report Node26.12.1; full findings remain retained without waiver. Published supplier approvals are18/27. Common Bookworm qualification, exact Python3.14.8/default, original hosted and production adoption acceptance remain open.

## Node14 native-addon interpreter source — 2026-10-31

[Thirteen-file source repair and baseline runtime audit](evidence/node14-addon-source-20261031/README.md) supply a private compatible Python3.9 default for common Node14's bundled node-gyp after explicit user choices, preserving selected app/tool versions, PATH and loader settings. Root verified64 source/evidence plus9 audit payloads and all13 live source files/modes; full operator/codegen/vet/controller-envtest, scoped lint, ten common controls and four omission regressions pass, with independent source review clean. A separate unchanged-originals offline addon gate is prepared. Actual relocation/addon/default-source-fallback/common image, all27/default/exact/companion and original28 hosted acceptance remain unexecuted; t007 stays in progress and production pins stay empty.

## Elixir1.20 immutable supplier evidence — 2026-10-31

[Root-approved Elixir1.20](evidence/published-elixir120-20261031/README.md) binds the same `9cb21ded` producer/verifier source and successful run41144656779/job124367871533, with no recovery waiver. Root checked186 case plus46 intake payloads,25 complete Git source snapshots/modes (22 recipe plus three critical inputs), all35 fresh complete anonymous blobs/1,748,254,771B, five strict proofs, two actual negative rejects, seven fresh TUF roots and all five full signed-subject/canonical-predicate roundtrips. Both raw configs name Elixir v1.20.4/OTP29.1.2, with full SPDX/Trivy bindings and findings retained. Published supplier approvals are19/27. Common Bookworm/default/exact/companion/generated and original28 hosted/adoption acceptance remain open.

## Common scan download reliability source — 2026-10-31

[The reviewed five-file transport/trigger repair](evidence/native-mirror-transport-source-20261031/README.md) stages fully SHA/size-verified public bytes before Trivy reads the original index through an owned loopback registry, preserving raw reference/config/platform/report identities and every scan/signing/V11 gate. Fifty-eight controls and independent review pass after recorded cleanup/startup corrections; a real pinned Trivy0.70.0 tiny synthetic CLI smoke preserves all identities with exact owned cleanup and unchanged foreign cache. This smoke retains its unsupported-OS limitation and is not a production full-image qualification. Initial27, corrected29, final39 and smoke50 payloads are hash-bound. Actual qualified common publication, generated/family/default/companion and original28 hosted/adoption checks remain open; no pins or original programs changed.

## Further family qualification source — 2026-10-31

[Separate opt-in family profiles](evidence/common-family-qualification-source-20261031/README.md) prepare legacy Python3.7/3.8, all six Ruby lines plus exact3.4.4/default and a static companion, all four Elixir lines plus exactOTP28.0, and Go/Rust native/compiler/tree checks. Source/unit/oracle checks, final full build-package suite and scoped lint pass; independent review corrected the local Mix application relationship and retained both freezes. No actual expanded image, wheel/Poetry/NIF/native compiler, both-architecture, native-init/Kubernetes or hosted qualification is inferred. The original13/28 programs and first cohort are unchanged; t007 remains in progress and all production pins remain empty.

## Modern Python and native-init qualification source — 2026-10-31

[The 17-file source slice](evidence/modern-python-init-source-20261031/README.md) adds the five remaining modern Python table versions, requested-architecture native-init/drain preparation and read-only capacity observations. Source/unit checks, full build package, scoped lint and independent reviews pass after correcting PATH assumptions and two cleanup-order defects; all histories remain. Two genuine direct Linux cross-compiles verify ELF targets only, with owned cache cleanup. Actual profile/image/wheel/project/daemon/drain/hosted execution remains unrun; Node qualification source and the known pnpm14/16 issue remain separate pending work. No original13/28, first-cohort selector, runtime, production pin or task-completion change is claimed.

## Remaining Node qualification source — 2026-10-31

[The seven-file Node profile and corrected fixture evidence](evidence/node-family-qualification-source-20261031/README.md) prepare six remaining table lines, exact14.21.2 and a Node companion, so source profiles now represent all27 reviewed table selections. Review corrected an unsupported npm6 root-engine assumption and an overly broad negative result using a local incompatible dependency with manager-specific refusal proof. Nine positive projects remain byte-identical; seven host oracles, full build-package tests/scoped lint and final independent source review pass. Actual images, source fallback, addons and offline manager projects remain unexecuted, and known pnpm14/16 startup incompatibility remains a strict open failure. No standalone launcher, runtime change, original13/28 edit, production pin or task completion is claimed.

## First common publication and qualification transport — 2026-10-31 (loopx session)

Run 41222919592 (source `874bec431`) published the common Bookworm candidate through every build/runtime/scan/sign gate. Both qualification jobs failed on Docker Hub transport, not code: amd64 `node-compute` passed its generated network-off commands, then `read selected native primary environment … context deadline exceeded` and CloudFront blob timeouts failed the remaining cases (arm64 likewise, plus a buildkit image pull TLS timeout). `1a2e65a30` makes `DockerHubResolver.ImageMetadata` retry `registry.TransientError` reads (3 attempts, 2 s/4 s), which the production resolve path shares; `dockerhub.go` is outside the workflow's `native*.go` trigger, so the next common run must be dispatched or ride a native change. Separately, `de52dd6ca` (w8/226) resolves the pnpm 14/16 startup failure with pnpm's standalone build and updates the Node qualification harness to expect it. No supplier approval, adoption or task closure is claimed here.

## Actual common artifact review — 2026-11-01

[The complete common artifact review](evidence/common-artifact-review-20261101/README.md) verifies the published immutable Bookworm image on both architectures. All five signed payloads, strict source/subject negatives and41 fresh complete config/layer reads totaling1,522,239,095 bytes pass; an independent in-place audit agrees. The full signed scans retain all6,887 findings per architecture. The portable record authorizes this image for testing with no utility baseline yet. Complete generated-image utilities, every supported runtime/companion, production adoption and the original28 hosted cases remain open.

## Current image and complete utilities — 2026-11-01

[The current0941 artifact and utility baseline](evidence/common-current0941-20261101/README.md) binds source1feba, the successful actual publisher and both full utility jobs. Full artifact review and independent audit pass;41 fresh complete blob reads, all five signed reports, strict rejection controls and exact verifier cleanup are retained. Both original356-member utility artifacts pass source/program/HTTP/omission/default-SBOM checks on amd64 and arm64, and their complete raw reports and wheels are preserved.

The reviewed test-candidate record now has an actual complete300-file qualification-source baseline and both utility cohorts. The unchanged helper accepts it in a clean isolated source context; null baseline still refuses. Runtime-family/companion, native-init/pre-deploy, production adoption and the original28 hosted acceptance remain open. The exact14.21.2 prebuilt donor remains an actual execution dependency; no additional product decision is introduced. t007 stays in progress.

## Actual Node Yarn wrapper correction — 2026-11-01

[The actual failed donor Node14 case and exact correction](evidence/node-yarn-wrapper-oracle-20261101/README.md) establish an oracle path defect: canonical Yarn resolves to its reviewed shell wrapper, while the check demanded the JavaScript file. Exact wrapper equality and a regression pass all10 host controls and the full Go build-package suite; original programs, positive projects and other assertions remain unchanged. The failed result is retained, and all owned resources are proven absent after cleanup.

The amended qualification source needs a fresh full same-image utility CI cohort on both architectures before strict family reuse. The retained source1feba result remains historical; image0941 approval and empty production pins are unchanged. Full actual family, phase, adoption and original hosted acceptance remain open; t007 stays in progress.

## Actual Node14 donor pass and native-init correction — 2026-11-01

[The real ARM64 Node14 donor case](evidence/node14-local-diagnostic-20261101/README.md) now passes exact Node/header/ABI/addon identity, npm/Yarn/pnpm lifecycle/engine controls and network-disabled runtime. Independent replay and complete owned-resource/process/config cleanup pass. It is one source6883953 local case, not full-family, both-architecture or hosted acceptance.

[The native-init run and narrow Yarn exit correction](evidence/native-init-yarn-exit-oracle-20261101/README.md) preserve the separate failed attempt: npm passes; Yarn drains its held response correctly but its normal SIGTERM exit1 was refused. Upstream1.22.22 source confirms that status. The corrected Yarn-only predicate leaves all exact drain/HTTP/identity/cleanup checks intact; old-predicate regression fails and the corrected full Go build-package suite passes. No four-launch success is claimed; the remaining launches and current-source CI baseline require fresh execution. Both original source-bound histories remain retained and t007 stays in progress.

## Actual donor, native-init and child-build progress — 2026-11-01

Local [Node16](evidence/node16-local-diagnostic-20261101/README.md), [Node18](evidence/node18-local-diagnostic-20261101/README.md) and [AMD64 Node14 under emulation](evidence/node14-amd64-diagnostic-20261101/README.md) pass their complete existing generated build/offline-runtime, header/ABI/addon, package-manager lifecycle/engine and cleanup checks. Source688 andea937, ARM64 control machinery and AMD64 workload execution remain distinguished.

[The actual native-init profile](evidence/native-init-local-diagnostic-20261101/README.md) passes all four launches and exact held-request/SIGTERM/PID1/ELF/environment checks with complete cleanup. [The pre-deploy child build](evidence/predeploy-child-build-20261101/README.md) passes maintained/original build programs and exports a verified OCI child. Its wrapper failure is retained alongside the separate exact cleanup reconciliation. No Kubernetes Job ran; no runtime/phase/adoption claim is inferred from the build.

The current-source utility refresh later fails AMD64 on Docker Hub429 after its first positive case, matching the separate local Node20 supplier throttle. No failed result is accepted. [The verified immutable-metadata reuse fix](evidence/immutable-metadata-reuse-20261101/README.md) passes real request-count regression, full package, race, lint and independent checks. [Both actual quota failures](evidence/dockerhub-metadata-quota-20261101/README.md) remain unaccepted; new-source baseline, remaining families, actual Job, production adoption and original hosted DoDs remain open. t007 stays in progress.

## Source alignment and dependent queue — 2026-11-01

[The original28 Free-contract follow-on](evidence/original28-free-contract-20261102/README.md) retains the selected workspace, canonical instance-hours allowance, fresh successful observations and all once/cleanup/adoption gates. The optional-coverage capture regression is corrected; all70 host controls pass and the old-method failure remains. Actual approval/Ready/Free fields remain null.

ADR060 now records all selected shared utility versions and exercised feature/ABI/phase limits from the reviewed0941 image and locked source. [The per-task evidence map](evidence/original28-free-contract-20261102/capsule/EVIDENCE-TO-TASK-MAP.json) reuses the existing supplier, consumer audit, simplify history and functional tests. m93–m103, m107 and222 are parked behind this milestone's actual qualification/adoption and their unchanged hosted DoDs; no task or milestone is claimed complete.

Current-source run41368533416 completes the full AMD64 utility cohort in646.18 seconds with cleanup/hash/upload successful. ARM64 is running. The exact Node14.21.2 publication41360577241 completes all gates and exposes332869e644; its actual metadata/size check passes, while full strict artifact review remains pending. This milestone stays active for these checks and the remaining family/phase/adoption/original-hosted work.

## Current complete utility baseline accepted — 2026-11-01

[Both actuale6a2386utility cohorts and full artifact intake](evidence/common-current-e6-20261102/README.md) pass on the same reviewed0941image. All712original members, executing300-file source and full default-Python reports are retained. The test-candidate record changes only source/current utility evidence; the original image approval remains unchanged. Existing-helper image/full-baseline validation passes. Family/companion/exact/legacy profiles, actual phase execution, production adoption and original28hosted acceptance remain open.

## Exact14.21.2 supplier qualified — 2026-11-01

[The actual exact Node14.21.2 publication and V11 artifact review](evidence/exact-node14212-artifact-20261102/README.md) pass. All39fresh full blob reads, five complete signed payload/report roles, strict rejection controls, source/ancestry bindings and owned cleanup are retained losslessly; full findings are preserved. The same-version artifact gate is clear. Common generated exact-version/companion qualification and production adoption remain separate open work.

## Actual family failures and applied causal fixes — 2026-11-02

[The causal packet](evidence/common-family-causal-fixes-20261102/README.md) preserves the original 35 dispatches, six separate supplemental donor dispatches, complete raw job metadata, actual failed logs, local PID1 observations, source corrections, host controls, and independent reviews. The old-source ARM64 native-init artifact independently passes all four launches with exact drain/HTTP/identity/cleanup proofs; the AMD64 identity failure remains a failure and does not establish a transport defect. Seven Ruby zlib failures are byte-equality fixture errors; Go/Rust metadata admission and source-prerequisite DNS harness defects have narrow reviewed repairs. No failed result is upgraded to acceptance.

All ten source changes are applied. The exact final Go build package passes in 19.675 seconds and pinned scoped lint reports zero issues after Root's `hasNetwork` boolean refinement; the earlier combined package run and three simplify perspectives are retained separately. Full source/patch custody distinguishes actual CI, local observation, synthetic host controls, and the earlier observer-only failures. Original programs/data/commands and all original milestone DoDs remain intact.

The changed functional source still needs a fresh complete same-image utility CI baseline on both architectures and affected actual family replays. Actual Kubernetes phase replay, production adoption, and original 28 hosted acceptance remain open. This is implementation and diagnostic progress only: m91 and t007 stay in progress.

## OTP selector repairs and dated family evidence — 2026-11-02

[The selector-fix packet](evidence/common-otp-selector-fixes-20261102/README.md) retains the real exact-OTP page-11 HTTP403/page-1 deadline, the narrow equivalent query partition, old-RED/new-PASS HTTP controls, and one actual seven-request verified-TLS lookup with all HTTP200. The separate expectation correction retains authored ERLANG_VERSION28.0 while binding runtime/header/NIF checks to the exact verified resolved version under existing numeric prefix semantics. Original programs/data/commands and product version behavior are preserved. Three source files are applied, including Root's tagged-switch test cleanup. The exact final full build package passes in 27.088 seconds and pinned scoped lint reports zero issues; source and foreign go.work.sum are unchanged and owned process groups/TMP are gone.

Actual old-source Node22 ARM and its independently audited AMD counterpart pass, and Elixir1.17 passes both architectures with real C/NIF/rebar/Mix/crypto build/runtime proofs and cleanup. Their original source, artifacts, and exact limits remain dated; they are not new-source acceptance. Current 9039 utility run41389762980 fails both architectures on Docker Hub429 before the required tool checks, with cleanup/upload successful and artifact metadata retained. A JIT donor flag control is diagnostic only.

Actual pre-deploy positive execution passed, but a parser defect blocked whole positive-plus-negative acceptance. Fresh complete current-source utility CI, affected current families, whole phase acceptance, production adoption, and original 28 hosted acceptance remain open. m91 and t007 stay in progress; no original DoD or completed history is removed.

## Test-only Elixir emulator correction applied — 2026-11-02

[The emulation packet](evidence/common-elixir-emulation-20261102/README.md) retains the exact two applied qualification files, all 303 before-source inputs and the sealed host checks: full Go build package PASS in 24.703 seconds and pinned scoped lint with zero issues. Root reverified overlay-to-applied byte equality; no tests were repeated for publication. Production generator, helper, factory, original fixture programs and all 33 image-critical inputs are unchanged.

The genuine immutable OTP28.5.0.8 donor control preserves baseline exit1 plus its full crash dump and `+JMsingle true` exit0 with the exact version. Its four owned containers, credential scope and process groups are cleaned. Earlier noinput and locale failures remain separate failed histories. The applied qualifier selects this explicit build-ARG/runtime-env override only for observed ARM-daemon to AMD-target emulation, preserves authored bytes and refuses OCI image ENV leakage; actual common-prefix/full-family execution is not claimed.

Fresh complete current-source utility CI, affected families, whole phase acceptance, production adoption and original28 hosted acceptance remain open. The unrun common-prefix probe is useful additional evidence, not a new gate. m91 and t007 stay in progress; no original DoD or history is removed.

## Current-source utility baseline green on both architectures — 2026-11-02 (loopx session)

Common run 41397092125 at `8d725c9b9` passes publish (reused reviewed image) and both `qualify (amd64)` and `qualify (arm64)` utility jobs. `8d725c9b9` fixed the red matrix step from `b0caca89d`: the Elixir route control scraped the new `resolved-metadata` unit-test fixture as a route; it now reads only `commonElixirCases()`. Artifact intake/review of this run, the family replays, phase acceptance, adoption and the original 28 hosted cases remain open; no task is closed here.

## Actual bounded common Node predeploy and prior8d identity correction — 2026-11-02

[The actual phase packet](evidence/common-predeploy-command-proof-20261102/README.md) records one canonical same-common Node ARM64 Job completing in dev-8, followed by the same-image AlwaysPull wrong-auth Pod remaining unstarted. Independent offline audit passes 1,422 checks; all eight Kubernetes UIDs, four phase containers and two phase volumes are removed with baseline preservation. The separately retained registry guard and its implicit volume are also removed under their own exact receipt. This is a bounded denial proof, not a terminal/deadline proof. Eighteen HTTP responses retain exact bytes; three m91 responses retain decoded restoration results and reported hashes. Four raw Secret DELETE bodies stay private and only UID metadata projections are published.

The earlier run41397092125 at8d genuinely passed, but its publisher built998171c7 and both utility jobs used9981 with empty RECOVERED; it did not reuse0941 and predates the emulator correction. This corrects the earlier image-reuse wording while preserving its dated success. The later explicit0941 run41403110558 remains a separate qualification. This phase is sourcee6-equivalent Node ARM only; current full families, production adoption and original28 hosted acceptance stay open. No task or original DoD is closed here.

## Python-only addon wheel prerequisite fix — 2026-11-02

[The compact wheel-export packet](evidence/common-addon-python-wheel-fix-20261102/README.md) preserves the real common-Python3.13/Node16 ARM64 failure: verified downloads/configure succeeded, but the wheel-export prefix unnecessarily compiled Node16 and hit its30-minute context at1803.8seconds before any gyp/project work. The auxiliary canonical fixture omits only the Node16 version signal in its own file copy, requires identical full application-Python and Poetry choices, and uses the ordinary default Node24 without a legacy compiler dependency. The generic30-minute wheel helper, original main Node16 canonical/files/expected-addon, final45-minute source compilation, implicit manager installs and runtime checks remain unchanged.

The exact one-file implementation is applied. Focused/full host Go checks pass in0.452/7.517seconds, pinned scoped lint reports zero issues, all303 source inputs and the foreign sum remain unchanged during checks, and ownedTMP/process groups are gone. All five formerly running old jobs are now failed, preserving the original17success/53failure record; actual revised generated/compiler/gyp/runtime success remains open. The refined35+6case plan permits only proof-backed dated nonimpact coverage rather than inventing82mandatorynewjobs. Existing strict current-source fullutility/candidate admission remains unchanged. m91 and t007 stay in progress; affected actual families, production adoption and original28hosted acceptance remain open, with every original DoD/history preserved.

## 用户决定 — D3（架构范围），2026-11-01

- **来源：** 用户在 w6 会话中明确选择"生产只验 amd64"（推荐项）。核查依据：生产集群 13 个节点全部是 amd64（2026-11-01 `kubectl get nodes`）；arm64 只用于本地 Apple Silicon 开发集群；Render 的 native 环境也只提供 amd64。这些镜像是平台级通用镜像（operator 只接受 `ghcr.io/bex-co/bex-cnb-run@<digest>`，见 `lego/operator/internal/build/nativecommon.go:30`），成本按版本线计，与租户数量无关。
- **适用范围：** w1/m188 与 w8/m91（t007 公共底及各运行时资格）。旧版本线的同版本补丁、各运行时与 companion 资格、完整 utilities、Trivy 扫描、SBOM、签名和生产采用，只要求 **linux/amd64**。
- **arm64 的保留部分：** 只保留在公共 Bookworm 底这一个镜像上，供本地开发使用，并只做轻量检查（能启动、核心工具可用）。不做逐运行时的 arm64 资格，不做 arm64 全量扫描或签名，不以 arm64 结果阻塞生产。逐运行时的旧版补丁镜像不再发布 arm64。
- **对既有决定的影响：** D1=A（平台集中修补、同版本）与 D2=A（w1 协同 w8）不变。本决定只把原 DoD 和 D2 中"双架构验收"收窄为生产 amd64。用户已明确接受代价：在 Mac 本地运行旧版运行时，没有经过验证的 arm64 补丁版本。
- **worker 下一步：**
  1. 调整 `native-base-image.yml` / `native-common-image.yml`：每个镜像的 `platforms` 和逐架构的检查、扫描、签名都改为 amd64。公共底额外出一个 arm64 变体，只做轻量检查。同步更新相关 `test_native_oci_*` 测试与 README。
  2. 已经在跑的双架构 run 可以收尾，不必为了改架构而取消；新的精确版本触发（例如 node-14.21.2）应在第 1 步合入后再启动，避免白跑 arm64。
  3. 验收条件改为：同一固定公共底在 amd64 上完成完整 utilities 与实际运行时/companion 资格，arm64 公共底通过轻量检查。其余 DoD 不变。
- **相关：** D4（为镜像构建拆出 `bex-build` runner 池）在 [w6/m155](../../w6/m155/README.md) 跟进。不阻塞本项，但上线后这些构建不再与生产部署抢 runner。

## Complete utility evidence for 0b7 and D3 scope — 2026-11-02

[The retained packet](evidence/common-current-0b7-20261102/README.md) contains both successful full utility jobs from run 41407186362 and all 712 original artifact members. Independent content review and the unchanged validators passed on the exact isolated 0b7 source. The candidate record changes only source/utilities; the reviewed image and its approval are unchanged.

This evidence does not admit current main: the publisher runner label changed a recorded input. D3 workflow migration and a fresh AMD64 baseline are next. Production family qualification now follows the user's AMD64-only decision; the common ARM64 development variant receives a lightweight check. Required AMD64 families, production adoption and the original 28 hosted cases remain open. No task closes here.

## D3 production AMD64 source and dated coverage evidence — 2026-11-02

[The D3 packet](evidence/common-amd64-production-20261102/README.md) retains the eight exact tested/applied files, original proposal and Root review, closed 0941 policy proof, and every failed attempt. Final host checks pass 172 unique Python tests (147 OCI + 14 base + 11 common), configured three-pool actionlint, and Actions guard/selftests. The initial four failures/one error, unconfigured-label refusal and host-Node skip stay raw; corrected runs are separate. Root's current-tree Actions checks also pass after two peer changes, with source hashes and foreign sum unchanged. No runtime or CI success is inferred.

Production qualification targets AMD64; common ARM64 has lightweight local checks and no per-runtime gate. The minimal coverage plan and five dated Python metadata reviews preserve original/supplemental distinctions. Their 393 bodies are not duplicated; old 0b7 retains its already-published 712 members. A fresh complete AMD64 utility baseline on final shipped D3 source (301 tracked inputs) and the SAME 0941 is still required before family admission. Required families, production adoption and original 28 hosted gates remain open; no task closes.

## Actual D3 AMD64 complete utility baseline — 2026-11-02

[The complete AMD64 packet](evidence/common-d3-dc08-20261102/README.md) retains all 356 original members from successful run41428447253/source dc08 and the unchanged0941 approval. Full content review, Root independent hashes and the unchanged candidate validator pass on the exact301-input clean Git view at36fd. Only candidate source/utilities change; the current utilities map contains exactlyAMD64 under D3. Historical image ARM64 roles remain verified in their original scope. The connector failure chain and full raw/gzip reports are preserved.

This supplies the current full utility prerequisite for runtime-family qualification; production common/runtime pins remain empty. Required AMD64 families, actual integration/adoption and original28 hosted acceptance remain open, with no new ARM64 family gate. m91 and t007 stay in progress.

## Bounded Node donor policy and complete-packet checkout — 2026-11-02

[The donor-policy record](evidence/common-node-donor-policy-20261102/README.md) retains the exact14.21.2 route, bounded production-map behavior, dated primary admission and actual standalone Go checks. [The integrated checkout record](evidence/common-candidate-checkout-20261102/README.md) binds the final eight applied source files: populated-map misses retain their official exact runtime without a tenant compiler or a false repair claim; empty-map source qualification remains intact. Checkout now materializes complete literal evidence packets required by the candidate and its hash-bound custody, preserving all proof validation. Real Git materialization and all six new controls pass. The first full Python run had152 passes and one stale read-only workflow expectation; that method was corrected and passed separately, with every other method unchanged. Configured workflow lint and Actions checks pass. Three simplify reviews yielded one existing `slices.Clone` reuse. The final merged Go build-package suite passes in7.606seconds; an earlier empty-HOME module-cache setup failure remains raw.

[Actual native-init run87](evidence/common-native-init-run87-20261102/README.md) passes all four launchers on source30b1 and common0941. The complete79-member artifact, full job log, Root content admission and original V1-audit-failure/V2-pass history are retained once. All four held Unicode requests drain after SIGTERM and owned containers are removed. This is actual AMD64 guest execution under ARM/Rosetta, with the original Go time/HTTP predicates and explicit evidence limits. Later source reuse needs an exact nonimpact binding.

[Original28 readiness source preparation](evidence/original28-source-readiness-20261102/README.md) passes87 unique host controls and the same three expected-red counterfactuals, retaining the prior81-pass result and all four real harness failures. Per-case execution sources remain true; differing sources require an exact reviewed input-equality or branch-nonimpact descriptor. The actual contract remains null and all original28 programs/data/commands remain unchanged.

The separate `16c1a49d` CI fix records the local ARM smoke Dockerfile's digest-supplied placeholder in the exact pin-guard list; canonical positive/wrong-placeholder/wrong-path controls and ShellCheck pass. Production common/runtime pins remain empty. The changed301-input source requires a fresh complete AMD64 utility baseline on the same0941 before remaining family qualification. Actual generated companions, adoption and original28 hosted acceptance remain open; m91/t007 stay in progress with every original DoD preserved.

## Candidate evidence checkout budget — 2026-11-02

[The complete failed-run and source-fix record](evidence/common-checkout-hydration-budget-20261102/README.md) preserves run41449940993/sourceb067: initial source checkout8seconds, then complete-evidence Git materialization exceeded the introduced180second deadline with exit124. Strict selection and utility execution never ran; artifact count is zero. Underlying I/O or transport cause remains unknown. The workflow changes only three matching Git limits to600seconds and enclosing steps to12minutes, retaining the15minute planning limit and every proof, selector, runtime, auth and cleanup command. All six existing Git controls, the workflow guard, configured lint and Actions guard pass; source301/foreignsum and owned cleanup remain exact. A new actual current-source AMD64 utility run is still required; this failed result is never relabeled or rerun. m91/t007 remain in progress.
