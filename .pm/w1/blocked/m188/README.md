# w1 · m188 — 旧版 native 运行时的安全补丁由平台集中维护

**Worker:** worker1 **Goal:** 用经过验证的平台补丁解除旧版官方镜像的上游等待，在用户选择的统一公共底架构中保持运行时版本，优先保护实际在用的受影响服务。 **Status:** blocked — t001/t002 done；补丁产物独立验收11/11、旧授权清理与交接完成；等待worker8的w8/m91/t007提供合格公共底及旧11条线同版本双架构资格，生产接入与线上修复尚未完成

## 用户决定 — D1=A，2026-10-29

- **来源：** 用户在 `/unblock w1` 中明确回复 `A`：允许平台集中修补，优先实际在用、确认受影响的旧版本，并接受由平台承担持续构建、安全扫描和兼容性验证。
- **边界：** 保留用户选定的语言版本；补丁在平台镜像发布时制作，租户构建直接使用固定镜像，不在每次租户 build 临时 apt 升级。
- **修复责任：** 同时核实系统库和运行时自带库，不能只升级系统 libbrotli1 就宣布所有运行时已修复。
- **当前用户动作：** 这项维护范围决定已完成，无需重复确认。实现、验证及后续具体执行条件由 worker 按既有规则处理。
- **状态：** 不再以“等官方镜像更新”为阻塞条件；用户随后执行 `$loopx w1`，已授权并开始实现和发布。当前逐版本验收进度见下文，全部完成前不标记 done。

## 已确认的公共底决定及后续分工 — 2026-10-31

- **依据：** 当前主线62a9c2500包含c5ee618ce公共底准备；[w8/m91记录的用户决定](../../../w8/m91/README.md)明确选择一个Bookworm公共底，覆盖较早worker推断的逐语言方案。已开始的逐语言发布允许收尾并保留证据，最终生产采用等待[公共底与生成镜像资格](../../../w8/m91/t007.md)。D1=A继续要求平台维护补丁、保持同版本；它没有授权临时绕过公共底。
- **当前处理：** 完成11个旧版本的发布与独立证据，供安全审阅和后续候选runtime donor复用；不在公共底为空时将逐语言C镜像设为租户基础镜像。较早记录中的直接11-pin接入计划已被本段取代，原始证明与失败历史保留。
- **尚需谁完成什么：** worker8在w8/m91/t007提供已审核的公共底固定digest，以及本项旧11条线的同版本运行时、库/头文件/扩展与companion的真实双架构兼容性证据。仅发布公共镜像、任务勾选或复制旧运行时目录不足以清除此条件。
- **worker1下一步：** 独立发布、清理和[具体交接](common-integration-handoff.md)已完成；仅验证产物的工具已全局移除旧逐语言接入写入能力。合格集成后才执行六项真实build/HTTP、最终operator Ready及新鲜全量服务清单/明确目标重建。
- **用户如何帮助：** 维护范围和公共底选型均已决定，无需再选择A/C或交付手动证书。需要调度实现时，先让`$loopx w8`推进m91/t007；上述证据可用后，`$loopx w1`继续生产接入与实际验收。当前门槛是尚未合格的公共底与旧版本集成，Node18 CI已完成。

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | 清点旧版运行时的实际使用与受影响库 — **DONE** | 45m | — |
| t002 | 建立平台统一补丁镜像的构建、验证与发布流程 — **DONE** | 2h | w1/m188/t001 |
| t003 | 接入 native pin 与持续维护记录 | 45m | w1/m188/t002, w8/m91/t007 |
| t004 | 验证发布、rollout 与受影响服务重建 | 1h | w1/m188/t003 |
| t005 | Render parity | 20m | w1/m188/t004 |
| t006 | Simplify | 20m | w1/m188/t005 |
| t007 | Test coverage | 45m | w1/m188/t005, w1/m188/t006 |
| t008 | Closeout | 15m | w1/m188/t007 |

## Definition of done

- 原 143 的 11 个版本均有最新、可复验的受影响状态和处置结论；缺 SBOM 不被当作安全，在用且受影响的版本优先，其他受支持版本不被静默忽略或移除。
- 需要平台修补的版本具有可重复的镜像输入、补丁来源、签名/SBOM、扫描与实际修复验证；覆盖需要的架构，运行时版本选择和兼容性合同保持一致。
- 已发布且验证的镜像进入 source pin 与 freshness inventory，持续构建、扫描、验证和失败保留原 pin 的流程可执行，维护责任及回归官方镜像条件有记录。
- 生产采用用户指定的单一合格Bookworm公共底及同版本补丁运行时闭包；逐语言发布数量与donor证明不能替代公共底生成镜像资格。
- 实际在用的受影响服务有发布、rollout、重建和运行验收证据；只更新源码或镜像标签不算完成。
- 无法在同版本下安全修补的情况必须有具体证据和明确剩余处置，不静默升级或禁用，也不因用户已授权而宣称验收通过。
- 存在仍受影响且未解决的版本时保持里程碑未完成；只有用户明确接受其他处置并更新范围后，才可重新评估完成条件。

## Source + Goal linkage

- **Source:** [w1/143 原始记录与用户决定](../../done/143.md)、[逐镜像核查证据](../../evidence/136-brotli-upstream-20261028.json)。2026-10-29 用户选择 D1=A；交接时 checkout 为 `6792b9cf2`，实施时复核最新上游和实际服务现状。
- **Goal linkage:** ADR008 的可靠、安全且 Render 兼容的托管能力；ADR060 的受审查构建来源与版本合同。
- **Expected outcome:** worker 可以自行修补并维护必要的旧版镜像，受影响服务用上已验证补丁，不继续被上游发布时间挡住。
- **Why now:** 上一轮可用更新已由 136 独立推进；剩余旧版标签缺少已验证修复，用户现在明确接受平台维护责任，原上游等待条件已解除。
- **Render parity:** 包含 t005，因为接入影响租户 native 构建的版本选择与部署行为，需验证所有入口和 workspace 语义一致；不新增 API/UI 功能。
- **Sizing:** 明显超过约 1 小时且跨现状核查、镜像发布、接入和线上验收，因此从 inbox 143 提升为里程碑。表内为初估，t001 根据实际库与回移工作校准，不把未知成本当成已验证结论。
- **Related work:** [w1/136](../../done/136.md) 的 19 个已更新 pin 与 CNB 后续发布/重建保持独立；本里程碑不重复实现它。

## loopx 交接

`/loopx w1`（Codex：`$loopx w1`）。先做 t001 的只读现状与库清点，再按表推进；沿用上述已确认决定，不重复询问是否允许集中修补。若实施发现新外部条件或同版本修复不可行，写明具体版本、证据、责任人与下一步，并完成其余可实施部分。

## 实施进展 — 2026-10-29

用户已执行 `$loopx w1`，开始实现。t001 核查完成，见 [assessment.md](assessment.md)。main 已有统一 native 基础镜像发布流程，t002 复用该流程增加系统及 Node 自带 Brotli 修复，不新增平行框架。尚未完成发布、接入和验收。

### 补丁验证进展

Bullseye 系统补丁已在 amd64/arm64 编译，Debian 的 14 项上游测试与实际 decoder 回归均通过；原 Python3.7 镜像在相同安全门槛下失败（错误 -9），修复后返回永久格式错误 -17，健康 round-trip 保持。见 [组件证据](evidence/system-brotli-component.json)。这只是本地组件验证，未冒充签名平台镜像发布。Node14/20 完整编译与 operator 全套验证仍在进行。

### 发布流程实现交接

统一配方、系统回移、同版本 Node 编译、双架构安全门槛、provenance、SBOM/扫描 attestation 和已验证 discovery tag 已实现。原官方 pin 保留为独立 upstream 输入，精确 patch 查询不误向平台 registry 请求，失败继续用已审核镜像；实际平台 digest 尚未接入。

本地 `GOWORK=off make test`（含 envtest）通过；构建包复测、matrix 行/递归/精确来源测试、Actions 和 image-pin 守卫通过；`/simplify` 三路复核已应用多行输入修正、重复 probe 清理和编译缓存改进。Bullseye 双架构组件测试通过。Node14 第一次完整编译成功且实测永久错误 -17，但旧版测试预期错误名称写错，已修正并开始干净重跑；版本身份与 streaming 验收不以那次失败记录冒充通过。Node20 的完整编译仍在运行。

发布配方需要先 ship 才能通过 main-only OIDC 工作流生成签名候选；发布、逐版本验证、source/inventory 接入与 live 验收仍未完成。t002–t008 保持开放，本次源码提交不代表里程碑 done。

### 后续发布效率与 CI 检查

已修正 adoption-only native pin 提交会取消其余镜像发布的问题：比较原始发布输入，相同输入不重建；并发取消仅限同一运行时的新发布。手动 refresh 和真实配方/上游变更仍发布。4 项 matrix 行为测试、Actions 校验和增量代码复核通过。Node 编译输出写入构建临时日志，只在失败时输出末尾，避免 BuildKit 截断有用结果。

Ship gate 指出的 [main 失败 run 40968544989](https://github.com/bex-co/bex/actions/runs/40968544989) 是 Alpine CDN 的一次 TLS 下载故障；上一绿 run 40966856467 使用相同 Dockerfile 成功。附近三次失败原因均不同，暂不添加无依据的源码修改；已仅重跑失败任务并立即继续，未等待 CI。见 [诊断证据](evidence/red-main-40968544989.json)。

### 本地组合与真实 native 启动验收 — 2026-10-29（UTC 02:03Z）

- Node14.21.3 / Brotli1.0.10 和 Node20.20.2 / Brotli1.1.1 的 arm64 干净编译、原版本/ABI/关键库/npm 保留、同步/一字节流式畸形输入和健康 round-trip 全部通过。Google 原始 C 源码与 Node 自带树的对比没有发现会被覆盖的 Node 专有修复。
- 使用已编译组件组合当前完整系统/工具配方，未修改其安装和安全步骤；两者均通过原版 check.Dockerfile（真实 PG18 archive、工具转换、系统和自带 Brotli，network=none）。
- 新增 `nativebackport_test.go`，复用已有 resolver/Dockerfile 和 `runServing`，实际执行用户构建、保留构建产物、限时启动 HTTP 服务并核查同版本与安全行为。本地通过临时 Go overlay 仅替换两个候选镜像验证 Node14/20，生产 pin 未改变；最新测试在构建及启动阶段都运行探针，异常容器有清理。
- 证据：[组合与 native 启动](evidence/local-native-start.json)、[Node14](evidence/node14-final-runtime.json)、[Node20](evidence/node20-final-runtime.json)、[源码对比](evidence/vendored-source-comparison.json)。签名双架构候选仍由正式发布任务生成，不能将本地成功算作 t002/t003/t004 完成。

### 同步最新供应流程与编译排队 — 2026-10-29（UTC 02:28Z）

- main 的 `2de693f18` 增加精确 primary 在共享系统基座上的兼容路径与 Python 工具供应；native 发布改用已有公开包 `ghcr.io/bex-co/bex-cnb-run:native-<upstream-tag>`，不会移动 CNB 的 `latest`。验收器已更新标签、workflow 身份和匿名拉取核查，未沿用旧命名误判。
- 旧发布 `40986883137` 的 Node14/16 在约 40 分钟后被真实的新配方取代，安全/签名门槛未执行，因此没有算作已发布候选。替代发布 `40989646534`（源码 `2de693f18`，包含全部 Brotli 修复）继续执行。
- 实际取消显示首次编译尚未导出缓存时会丢失长时间工作。改为同运行时正在执行的任务先完成、新配方替换 pending 任务并复用完成缓存；源码仍正常 ship，不等待 main CI，也没有削弱发布或采用验证。
- 新 m91 路径明确兼容 m188 的 `upstream` 原始来源字段（`TestSuppliedBasePreservesPlatformBackportProvenance` 等回归组通过）。m188 不用不完整的全局供应映射冒充全部 27 个运行时接入，后续只采用本项已验证的 11 个 line 镜像并保留原来源。
- 各入口/版本规则准备核查见 [parity 证据](evidence/parity-prepared.json)；正式采用和 rollout 仍未完成，任务不提前关闭。

### Python / Ruby 完整配方与真实 native 启动 — 2026-10-29（UTC 03:20Z）

- Python3.7.17、Ruby3.0.7 的 arm64 候选使用当前原版 Dockerfile 和 check.Dockerfile，没有替换编译步骤；真实 PG18 archive、工具转换、系统/私有 Node 的 Brotli 探针均通过。Python 的离线 wheel/C-extension、metadata/pkg_resources/distutils 和 Ruby JSON/OpenSSL/Zlib 健康操作也通过。
- 新增两个集成测试，复用已有 native resolver、Dockerfile、构建和 HTTP helper，在 build/start 两阶段执行安全探针，HTTP 返回必须与构建产物及所选语言版本一致。临时 overlay 使用上述本地候选，实际 Python3.7.17 和 Ruby3.0.7 均通过；生产 pin 未改变。
- Python3.7 显式使用兼容的 Poetry1.5.1，fixture 无 uv.lock；不声称默认 Poetry2.1.3 能运行在 Python3.7。旧版本需匹配 companion tool 是既有要求（w8/done/173 与 [Render Poetry 版本文档](https://render.com/docs/poetry-version)），本项不修改用户所选版本。
- 见 [Python 完整配方](evidence/python37-combined-proof.json)、[Ruby 完整配方](evidence/ruby30-combined-proof.json)、[真实 native 启动](evidence/local-python-ruby-native-start.json)。以上是本地兼容性证据，签名双架构发布、采用与线上验收仍待完成。

### 验收发现并修复 companion 可移植性缺口 — 2026-10-29（UTC 03:50Z）

- 真实对照复现：官方 Node20.20.2 需要 GLIBC2.28 / GLIBCXX3.4.21，在 Python3.7/Ruby3.0 Bullseye 均启动；Bookworm 重编版本需要 GLIBC2.34 / GLIBCXX3.4.30，在两者均失败。此前本地 Node20 主镜像通过不覆盖 companion 合同，该候选不能采用；见 [ABI 与实际启动证据](evidence/companion-node-abi-regression.json)。Node14 的 Bullseye 编译在两者启动正常。
- 四个旧 Node 补丁改用固定 Bullseye 编译基座，最终镜像保留原官方主运行时和系统版本。Python/Ruby 集成测试显式选择 Node20 并核查 build/start 的真实双运行时版本；不改变用户版本来规避失败。
- check.Dockerfile 增加实际 Bullseye companion gate，最终阶段消费检查产物，双架构发布都会执行。已用旧 Node20 候选证明 gate 失败，用 Node14 候选证明 gate 通过；见 [红绿对照](evidence/node-portability-gate-controls.json)。修正后 Node20 的完整编译及真实组合仍需通过，未预先宣称成功。
- 当前有证据的最旧 reviewed primary 为 Bullseye；最旧可选 Elixir1.17/OTP27 的官方历史配方已是 Bookworm，见 [来源核查](evidence/oldest-primary-metadata.json)。此结论不建立永久发行版 allowlist。旧配方发布停止采用，正式发布/接入/rollout 仍保持未完成。

### 可移植修复本地验收通过 — 2026-10-29

- 新 Node20.20.2/Brotli1.1.1 已在 Bullseye 完整编译，npm/运行时身份检查通过；ELF 需求降至 GLIBC2.29 / GLIBCXX3.4.26，在原始 Python3.7/Ruby3.0 Bullseye 均通过 canonical 畸形/健康 decoder 探针。新二进制配置与官方除已解释的 prefix/release URL metadata 外一致；见 [编译与基座证明](evidence/portable-node20-compiler.json)。
- 更新后的真实 native fixture 均通过（47.459s）：Python3.7.17/Poetry1.5.1 + Node20.20.2，Ruby3.0.7 + Node20.20.2，build/start 两阶段系统、私有和 companion Node 探针及 HTTP 双运行时版本均通过；见 [组合绿对照](evidence/companion-native-green.json)。
- Python/Ruby 的当前完整配方与原版 check.Dockerfile 通过；Node20 组合当前最终配方，仅复用刚验证的编译产物，保留原 Bookworm 主镜像/ENV/npm，完整工具、Bullseye gate、安全检查及真实主运行时构建/HTTP（6.680s）均通过。见 [Python/Ruby](evidence/portable-python-ruby-suppliers.json)、[Node20 主镜像](evidence/portable-node20-final-native.json)。
- [ABI/config 补充核查](evidence/node-config-compatibility.json) 未发现 Node14/20 的配置或实际 release/module paths/features/FIPS/ICU 合同变化。所有本地 overlay 已明确范围，不代替签名发布验收。
- 修正配方已通过 `bfdeb6221` ship。正式发布 41000570846 正在构建首批 Node14/16；11 个不可变签名候选、接入和 live 验收仍未完成，t002–t008 保持开放。

### 正式发布失败处置：扫描预算、登录重试与受限恢复 — 2026-10-30

- Node14/16 的正式双架构构建及离线/Bullseye 检查通过，但 Trivy 默认时限在分析 GCC10/12 `lto1` 时耗尽；没有产生完整扫描报告或签名，仍不可采用。已核实的原索引分别为 `4fec31...` / `280207...`，出处与完整 digest 见 [14](evidence/node14-publisher-scan-timeout.json)、[16](evidence/node16-publisher-scan-timeout.json)。Node18 在编译前因 GHCR 等待响应超时失败，见 [登录证据](evidence/node18-publisher-login-timeout.json)。原 Node20/Python 等任务保留。
- 逐架构扫描预算增加到有界 20m，不跳过扫描或签名；登录采用最多三次、单次 60s、5/10s 退避，并始终清除 job 的 GHCR 凭据。
- 实测导入 Node14 inline cache 成功，但中间编译阶段未命中，已立即终止本地测试、未继续冷编译；见 [缓存证据](evidence/node14-inline-cache-miss.json)。因此不以“导入缓存”冒充省掉编译。
- `.github/native-base-recovery.json` 仅授权已人工核实的同源 Node14/16 不可变索引：base/package/非 Markdown 配方必须保持，重命名也算变更；恢复会重做全部双架构检查、扫描、SBOM 与签名。原镜像 producer SHA 与新 verification SHA 分开记录。实际 source pin 仍未切换。验证后的 discovery tag 发布即删除对应授权；数据文件清理不会再触发全部发布。
- 受限选择器 8 项实际隔离 Git 测试（含非 MD→MD 重命名）、登录脚本三种成功/失败路径、runtime 矩阵四种选择/注入数据路径通过，见 [恢复选择](evidence/recovery-selection-tests.json)、[登录](evidence/registry-login-retry-tests.json)、[矩阵](evidence/runtime-filter-tests.json)。Actions/image-pin/lint 守卫通过；修正还需 ship 后实际发布验收。
- 接入的完整 build 包模拟 11 个平台 ref/55 项 inventory 通过，见 [仅单元数据预检](evidence/adoption-unit-dryrun.json)。扩大线上 census 核实三个旧 auto 配置实际为 Alpine/Node22，结合派发代码按客户 Dockerfile 路径排除；历史构建记录缺失被明确保留，不作 Brotli 安全结论，见 [来源分类](evidence/auto-runtime-classification.json)。当前仍未发现运行中的 11 个旧版平台运行时。

### 签名传输失败与定向恢复 — 2026-10-30

- Python3.7 的双架构构建、运行检查和扫描通过，但第一份 amd64 SPDX attestation 在向 Rekor POST 时耗尽客户端两次尝试；没有完成全部签名证明，不可采用。见 [诊断证据](evidence/python37-publisher-signing-failure.json)。为每次原有 cosign sign/attest 增加最多三次、每次 5m（另加 10s 强制终止宽限）、5/10s 退避，不跳过透明日志或任一架构证明。
- 将该已核实索引加入现有 recovery 授权；保持 producer `bfdeb6221`，新 verification SHA 单独记录，重做全部检查、扫描及签名。验证后的 discovery tag 发布即移除授权。
- 已在逐项确认全部构建任务未分配 runner、步骤为 0 后，将全 27 项恢复队列换为 Node14/16 定向恢复，并单独安排原登录失败的 Node18。原 Node20 与其他实际构建保留，避免重复长时间编译。
- 实际完整签名 shell 的五种成功/失败路径通过，见 [重试行为](evidence/signing-retry-tests.json)；恢复选择器新增 Python3.7 同源路径，九项隔离 Git 行为检查通过，见 [恢复选择](evidence/recovery-selection-tests.json)。
- 11 个版本尚未完成正式验收，source pin、接入与 live 验收仍未完成；t002–t008 保持开放。

### 大型 SBOM 的现代透明日志路径 — 2026-10-30

- Node14 定向恢复同样已通过双架构检查与扫描，但第一份 amd64 SPDX attestation 向 Rekor POST 失败；初始镜像签名已经完成，不能从整个 shell 步骤名误认失败命令。见 [Node14 证据](evidence/node14-recovery-spdx-failure.json)。
- 同期部署构建用相同 cosign3.0.6/Syft1.51.1 完成五份 SPDX attestation，见 [成功对照](evidence/deploy-signing-comparator.json)。因此没有证据支持普遍服务中断。上游 [2808](https://github.com/sigstore/rekor/issues/2808)/[2813](https://github.com/sigstore/rekor/pull/2813) 说明过大请求也可能只返回同样的通用错误；当前日志未暴露 HTTP 状态或请求上限，不编造具体数值。
- 发布改为固定 cosign3.1.4，生成 TUF 默认的 Rekor v2/Fulcio/TSA 配置，并用于所有原有 sign/attest；[Rekor v2.3.0](https://github.com/sigstore/rekor-tiles/releases/tag/v2.3.0) 与现代 SDK 对 DSSE PAE 提交摘要和签名，完整 SBOM/扫描谓词仍在签名 OCI bundle 中。保持透明日志、证书与 workflow 身份校验，验证端使用兼容的新客户端，不使用跳过 tlog 或自定义压缩谓词。
- 配置生成及五次签名/证明均有界重试；实际 shell 的五种成功/失败路径验证配置、签名或最后一份证明失败时不会移动标签。真实新版签名和双架构证明仍需 ship 后执行，不以本地 stub 结果替代。

- 固定 cosign3.1.4 的实际 CLI 预检通过：TUF signing-config 连续生成/覆盖两次 exit0、内容 hash 一致，sign/attest 参数帮助与固定源码吻合；见 [实际 CLI 预检](evidence/modern-signing-config-preflight.json)。没有在本地冒充 GitHub workflow 签名或向公开 Rekor 提交。

- 用同一 Syft1.51.1 对同一 Python3.7 digest/amd64 重建 SPDX 成功：24,452,589 bytes，谓词 base64 本身为 32,603,452 bytes，见 [大小证据](evidence/python37-sbom-size.json)；独立 [BuildKit descriptor](evidence/buildkit-sbom-descriptor-sizes.json) 记录两架构约 22 MB 的原始 SPDX。重建不是取回原 runner 的逐字节文件，原 HTTP 状态/请求上限仍未知；只作为大型请求推断的佐证，不替代新版实际签名验收。扫描临时文件已自行清理。

- UTC 09:11 再次只读清点 [29 个 App](evidence/pre-adoption-census-20261030-0911.json)：在运行的平台 native 仍为 Node22/24 与 Go，三个旧 auto 服务镜像与既有 Dockerfile 分类证据一致；未发现本项 11 个旧版在用。Failed/Canceled 无 pod 项仍不冒充运行验收。此项用于发布优先级，采用后仍需新一轮 census 与 rollout 验收。

- Python3.8 的原始双架构构建/运行检查/扫描均通过，同样在第一份 amd64 SPDX 的 Rekor v1 POST 失败；同源 base、两架构版本、不可变 index 与旧 gate 日志已绑定，见 [恢复证据](evidence/python38-publisher-signing-failure.json)。将精确 index 加入临时授权，由现代工作流定向重做全部门槛；十项实际恢复选择检查通过，未改变生产 pin。

- Node20 的正式双架构编译完成，原版本20.20.2、Bullseye companion、系统/自带 Brotli、离线工具及两架构扫描均通过；第一份 SPDX 仍由旧 Rekor v1 流程上传失败。已核实精确 `2c69d3...` index 与 producer/base/gate 日志，加入同一受限恢复，见 [证据](evidence/node20-publisher-signing-failure.json)。生产 pin 继续等待完整验收。
- Ship gate 指出的 main 失败41027077470 已以 `c8ca76473` 单独修复测试 fixture 的相等截止时间竞争：测试服务端保持到完整 scan 预算之后。十次重复、race 包检查和 operator/codegen/envtest 全套通过，见 [诊断与实际验证](evidence/red-main-41027077470.json)。修复已推送后恢复发布工作，未等待远程绿灯。

### 发布后独立核对发现 SBOM 子镜像绑定差异 — 2026-10-30

- Node14 新版发布工作流成功，五份完整 bundle 的证书、精确 workflow SHA、透明日志和可信时间戳均通过；独立内容复核发现 amd64 SPDX root `99158...` 与实际 registry 子 manifest `933c...` 不一致，见 [独立检查](evidence/node14-independent-semantic-review.json)。Trivy 的两架构 config 与原 index 子镜像均吻合；不能仅以共同 index subject 或版本相同放过 SPDX 差异。当时继续 held；Node14 后于 UTC 11:27 以实际层内容等价证明解除，见下节独立验收。
- Syft 自动选源支持 Docker-save 合成 manifest；现有 metadata 尚未证明该临时摘要与目标子镜像等价，不添加放宽规则。发布改为明确 `syft registry:$REF`，同一已编译 index 重新走全部发布门槛。
- [已有真实 registry 来源 Node14 预检](evidence/node14-registry-source-preflight.json) 的 amd64 root 与实际 `933c...` 一致，语言与补丁包版本正确；这份 Node14 记录使用 Syft1.55.0，不能冒充1.51.1 实测。另有同一 prefix 的真实 Syft1.51.1 Python3.7 扫描记录。正式新签名仍需实际重放后独立核对。

### 首个签名候选独立验收 — 2026-10-30（UTC 11:27Z）

- **1/11：Node14.21.3 通过。** [验收收据](evidence/node14-published-acceptance/acceptance.json) 记录原 producer、独立 verification SHA、双架构运行/Bullseye gate、五份严格签名证明与匿名 discovery digest。完整 SPDX 与 Trivy 仍以公开 OCI digest 保留，不把报告存在当作零漏洞。
- 旧自动来源 SPDX 的 amd64 synthetic manifest 已由实际压缩层 digest、解压 DiffID/大小、原配置及顺序逐项重建，精确得到签名中的 digest；[内容等价证据](evidence/node14-layer-equivalence/MANIFEST.json) 与独立复核/正负守卫通过。不是添加任意 digest 别名。
- 每个架构仍报告三条 CVE-2026-111497（brotli/libbrotli-dev/libbrotli1）；处置依据是明确的 Debian 源码回移、补丁版本和实际 decoder 探针，不隐藏扫描发现，也不声称旧运行时所有 CVE 已清零。
- 验证 discovery 后已删除 Node14 的一次性恢复授权；其余授权保留。当前 source pin 尚未采用，剩余十项、正式接入、rollout 与新 census 继续执行，t002–t008 保持开放。

### Python3.9 定向恢复 — 2026-10-30

原始双架构索引的 Python3.9.25、producer revision、实际离线工具/运行检查与两份扫描均已核实；原任务仅在首份 SPDX 向 Rekor v1 提交时失败，见 [原始证据](evidence/python39-recovery-intake/receipt.json)。已将该精确 digest 加入同源恢复授权；恢复需重做当前双架构门槛、扫描、registry 来源 SBOM 与现代签名，不提前计算为通过。Node14 授权删除及其余五个精确恢复路径的选择器检查共十二项通过。

### 其余旧 SPDX 绑定与 registry 重放 — 2026-10-30

Node16、Python3.7、Python3.8 的旧 amd64 SPDX 也未证明与实际子镜像内容绑定，见 [三项 held 收据](evidence/sbom-binding-holds-20261030.json)。这不是不安全内容的结论，也不添加任意 digest 别名；均保留既有精确索引授权，以明确 registry 来源重新运行完整门槛、报告及签名。Node16 由 41043547006 推进，Python3.7/3.8 分别定向重放 41051768218/41052230534，Python3.9 恢复为 41050319125。实际 [d947](evidence/publisher-source-reviews/d947.json) / [e47](evidence/publisher-source-reviews/e47.json) 源码逐一检查了发布输入范围，未把 main 的新提交默认当作已批准；当前仍为 1/11，source pin 未切换。

### 已编译产物转交当前完整恢复 — 2026-10-30（UTC 12:26–12:29Z）

- Python3.10.22 与 Node18.20.8 的实际双架构构建及全部运行/Bullseye 门槛通过；逐次重新查询确认没有尚未导出的编译后，停止旧 BF 批次及 Node18 单项旧流程。原 amd64 扫描被中断，arm64 扫描、SBOM 与签名未完成，不能据此算作验收，也不记录为 Rekor 失败。
- 原 producer 分别为 BF 与仅适用于 Node18 的49ee；匿名 index/child/config 哈希、原版本、实际 gate 日志与未变配方均已绑定。见 [Python3.10](evidence/python310-recovery-intake/receipt.json)、[Node18](evidence/node18-recovery-intake/receipt.json)。新增精确恢复授权要求重新完成当前双架构门槛、两份全扫描、registry 来源 SBOM 与完整现代签名。
- 旧21ed批次的两个剩余任务均 pending、0步骤，已提交取消；保留已在进行的 canonical5e89 全27项批次，避免重新冷编译刚完成的 Node18。见 [逐次安全检查与请求](evidence/obsolete-batch-transfer/21ed-obsolete-queued-cancel-review.json)。所有取消均保留已导出的不可变产物及历史证据，不以取消请求冒充发布或采用成功。

同步 main 的 `59e9d38fa` 已清理 Node16/Python3.7 的临时恢复授权；保留该清理，不恢复已过期条目。本项独立完整采用守卫继续核验两者的已有签名与真实层证据，未仅凭其他 worker 的结论增加计数。当前五个授权路径与三个已清理路径、配方/包/base 等负向路径的十四项实际选择检查通过；新增授权与逐次取消证据另经 [独立复核](evidence/new-recovery-auth-cancel-review.json)。Node20 旧 attempt2 重跑只剩零步骤 pending native 任务，已停止该旧重跑，保留 `41053647412` 的当前 registry 来源恢复。

### 独立正式验收推进至 3/11 — 2026-10-30（UTC 12:45Z）

- Node14、[Python3.7](evidence/python37-published-acceptance/acceptance.json)、[Node16](evidence/node16-published-acceptance/acceptance.json) 均已完成完整候选守卫与新鲜匿名 discovery body/header digest 检查，逐项计数为1、2、3；原有通过记录保持哈希不变。来源是已固定的实际双架构发布，不是 overlay 或模拟镜像。
- 两项新候选的 amd64 Docker-save 合成摘要都由实际压缩层 digest、解压 DiffID/大小、顺序及原配置精确重建；arm64 均直接绑定 registry 子镜像，不使用未被实际签名的另一派生摘要。两份独立复核、保留的实际读取/完成记录、五份严格签名和全扫描语义均通过。新守卫的2个正向与26个有意篡改反例通过；保留发现与回移/探针处置，不将旧运行时宣称为零漏洞。
- Python3.7 的重复定向恢复在重新确认 queued/runner0/steps0 后提交取消；Node16/Python3.7 临时授权的既有清理保留。当前源 pin 尚未采用；剩余Node18/20、Python3.8/3.9/3.10、Ruby3.0/3.1/3.2均有当前发布路径。t002–t008继续开放。
- 已推送 `5803d2030` 并针对其实际 SHA [检查发布输入范围](evidence/publisher-source-reviews/5803.json)。Node18与Python3.10以新授权进入完整恢复；三项Ruby在相同当前配方下新建候选，均不因为已排队就计入通过。

### 重复 registry HTTP/2 扫描传输故障的有界恢复 — 2026-10-30

Node16旧arm64扫描、Node20旧amd64扫描及当前canonical Node14 amd64扫描，均在读取镜像层时收到peer的HTTP/2 `PROTOCOL_ERROR`；后两次均未耗尽20m预算，运行门槛已通过。同一不可变索引此前扫描成功，因此按已观察到的间歇传输失败处理，不推断具体上游故障来源。见 [逐任务日志诊断](evidence/scan-layer-transport-diagnosis/receipt.json)。

现有固定Trivy action的 `TRIVY_CMD` 入口增加最多3次完整扫描、5/10s退避；每次前后清理失败报告，扫描自身仍为20m，另有21m/10s的进程终止界限。原镜像参数、架构、JSON、全部未修复发现与exit-code0报告语义保持；任一架构耗尽尝试仍中止签名和discovery。没有引入新action或更换扫描器。

[六项实际入口行为检查](evidence/trivy-retry-behavior/receipt.json) 通过，覆盖双架构、含空格输出、失败恢复、三次失败、缺少输出路径及外部deadline；这是受控命令stub的配置/退出行为检查，不冒充真实安全扫描或21分钟计时。实际发布采用需在修复ship后继续逐项验证；当前3/11不因此增加。

同步main的 `b83bf5035` 已为两架构扫描配置 `GODEBUG=http2client=0`，本次合并保留该协议设置并补充有界完整重试。合并后的六项官方入口行为检查再次通过，逐次调用均核对协议环境、架构和原扫描选项；扫描器版本、CVE报告范围与签名门槛保持。测试不宣称已在真实registry重现或证明peer根因，后续仍以实际发布结果验收。

### Python3.8 完整正式验收通过，推进至 4/11 — 2026-10-30（UTC 13:30Z）

[Python3.8验收收据](evidence/python38-published-acceptance/acceptance.json) 已绑定原BF producer、C8 verification、五份独立严格签名、双架构原运行/全扫描和新鲜匿名 discovery body/header。amd64 的17层真实测量精确重建签名摘要；arm64直接对应registry子镜像。31份V8有界临时文件测量、1份此前已成功的V7测量、1份已有Node14空层测量均有实际执行、哈希和清理证据；原argv缺失的边界保留，不编造执行记录，也不重复读取约1.14GB已测量数据。

两份独立内容/执行复核、完整采用守卫、1个正向与27个篡改反例均通过；[fd358实际来源复核](evidence/publisher-source-reviews/fd358.json) 与旧三项完整回归均通过。原三项收据保持不变，Py3.8临时恢复授权已移除。旧E47重复重扫在再次查询时已失败结束，因此未实际取消它，亦不将该次失败视为已验收旧C8产物不安全。

剩余7项为Node18/20、Python3.9/3.10、Ruby3.0/3.1/3.2，已排入使用HTTP/1.1与有界完整扫描重试的新来源队列。当前source pin、inventory采用和生产rollout仍待全部11项及实际集成验证，t002–t008继续开放。

归档另保留 [批准状态元数据对应记录](evidence/python38-published-acceptance/metadata-transition/acceptance-metadata-transition-hash-recovery.json)：完整守卫验证的候选为 `ace338ee…`，计入完成后为 `4540a299…`。仅五项批准状态元数据变化；已在事后按已记录哈希精确恢复原7405字节用于复验，并如实标注非原始时间点捕获。签名证明、验证记录、验收收据和最终候选保留原始字节。

### Python3.9/3.10 正式验收推进至 6/11 — 2026-10-30（UTC 15:15Z）

[Python3.10 收据](evidence/python310-published-acceptance/acceptance.json) 与 [Python3.9 收据](evidence/python39-published-acceptance/acceptance.json) 分别计为第5、6项。两者保留原BF producer和新FD verification的区别；五份严格签名、两架构直接registry子镜像/配置绑定、实际运行与完整扫描、固定Python版本和系统补丁均通过，完整采用守卫及新鲜匿名discovery body/header一致。其他旧运行时漏洞仍保留在完整报告内，不宣称零漏洞。

本次在批准前保存了实际候选原文，并保留冻结守卫、验证记录、批准状态转换和各自计数时的aggregate快照；原四项收据与证据保持原字节。Python3.10原batch的人为取消不冒充原扫描成功，新FD已重做全部门槛。两项临时恢复授权已清理，十四项实际隔离Git选择器检查通过；见 [清理后行为验证](evidence/recovery-selection-python39-310-retired.json)。

剩余Node18/20、Ruby3.0/3.1/3.2继续发布与逐项验收。source pin、inventory尚未采用；全部11项、真实应用测试和生产rollout完成前，t002–t008继续开放。

### Ruby3.0 正式验收推进至 7/11 — 2026-10-30（UTC 15:36Z）

[Ruby3.0验收收据](evidence/ruby30-published-acceptance/acceptance.json) 对应FD源码的正常双架构构建，producer与verifier相同，没有recovery授权。五份严格证明、两架构直接registry子镜像/配置绑定、Ruby3.0.7身份、实际运行门槛、完整扫描与新鲜匿名discovery body/header全部通过；原六项证据保持原字节。

Bullseye的三条目标Brotli版本命中在两架构完整报告中均保留；其 `1.0.9-2+deb12u1+bex11u1` 已回移固定Debian安全源码，结合签名包身份、实际畸形输入永久错误与健康round-trip证据逐项处置，不使用ignore。省略发现或删除处置说明的反例均被完整守卫拒绝。

最终validation只在全部检查完成后一次冻结发布；批准执行显式传入并校验人工复核的确切哈希，随后重做冻结守卫与匿名拉取确认。[错误批准哈希控制](evidence/promotion-approved-hash-control.json) 证明差异会在写入前拒绝。[归档核查](evidence/ruby30-acceptance-archive-audit.json) 确认全部链接与原始字节保留。剩余Node18/20、Ruby3.1/3.2继续；实际pin采用、应用测试与rollout仍待完成。

### Syft registry传输失败的有界恢复 — 2026-10-30（UTC 17:21Z）

Ruby3.2的FD正常构建、双架构实际运行门槛与完整扫描均通过，签名步骤在ARM64 Syft读取镜像层时收到HTTP/2 `PROTOCOL_ERROR`。此前镜像签名和AMD SPDX/Trivy证明已完成；不将整个步骤名称误认为cosign/Rekor失败，也不将不完整证明算作验收。见 [精确失败日志](evidence/ruby32-syft-transport-failure/receipt.json) 与 [不可变产物核验](evidence/ruby32-recovery-intake/receipt.json)。同次AMD Trivy的两次EOF已由现有完整重试恢复，最终两架构扫描均成功。

Syft保持相同registry来源、平台和完整spdx-json，只在该命令设置 `GODEBUG=http2client=0`，最多3次完整尝试，每次30m与10s终止宽限，5/10s退避。每次前及失败后清理对应架构的输出；耗尽后不执行该架构证明或discovery。原cosign5m重试、TUF/证书/透明日志/时间戳验证及所有扫描范围保持不变。[八项实际完整shell控制](evidence/syft-transport-retry/validation.json) 验证双架构、部分输出恢复、三次失败、deadline、原签名失败阻止标签及参数/环境/完整报告保留；受控stub不冒充真实网络发布或30分钟计时。

受限恢复仅增加已核实的Ruby3.2索引 `e7e887fb…`，保留原FD producer与固定base；新verification将重做全部运行门槛、双架构全扫描、完整SBOM及五份签名证明，成功discovery后清理授权。[十五项实际选择器检查](evidence/recovery-selection-ruby32-added.json) 通过。Ruby3.1独立证明核验继续；当前正式计数仍为7/11，source pin与生产接入尚未改变。

### Ruby3.1 正式验收推进至 8/11 — 2026-10-30（UTC 17:53Z）

[Ruby3.1收据](evidence/ruby31-published-acceptance/acceptance.json) 已完成五份严格证明、两架构直接registry子镜像/配置绑定、Ruby3.1.7与固定系统补丁核验、完整扫描和新鲜匿名discovery body/header确认。目标Brotli发现为零，其他旧版本漏洞仍保留在报告中。真实候选原文、批准状态转换、最终冻结守卫与计数时快照已归档，原七项证据保持原字节。

传输修复的 [3990来源审核](evidence/publisher-source-reviews/3990-review.json) 与 [后续明确准入](evidence/publisher-source-reviews/3990-admission.json) 分开保留，历史审核状态不回填；新守卫保留FD等全部旧角色，[七项回归](evidence/3990-seven-candidate-regressions.json) 通过。Ruby3.1本身的producer、verification与证书仍是FD，不改写为新来源。

后续4462仅增加PM记录，与3990所有非PM字节完全相同；[精确来源准入](evidence/publisher-source-reviews/4462-admission.json) 和 [仅来源策略追加](evidence/publisher-source-reviews/4462-activation.json) 均保留原八项。[八项回归及未满11项的实际采用拒绝](evidence/4462-eight-candidate-regressions.json) 通过，原守卫快照与验收记录不改。

3990自动全量重复发布在原生任务尚未产生时提交一次取消，最终已确认cancelled，未损失编译；见 [逐次请求和实际终态](evidence/3990-duplicate-batch-cancellation/completion-final.json)。4462已有的Ruby3.2定向恢复请求 `41101339222` 经实际run/jobs确认，直接复用，没有重复派发；见 [请求核对](evidence/ruby32-4462-recovery-request/inspection.json)。剩余Node18/20、Ruby3.2继续；source pin、inventory、真实应用测试与生产rollout尚未完成。

### Node18/20 的修复流程交接 — 2026-10-30（UTC 19:22Z）

后续main的6fd4仅改变五处自动生成的部署镜像digest；发布workflow、配方、共享action、版本matrix与恢复目录均与3990逐字节相同。[精确来源准入](evidence/publisher-source-reviews/6fd4-admission.json)、[来源策略追加](evidence/publisher-source-reviews/6fd4-activation.json) 和 [八项完整回归](evidence/6fd4-eight-candidate-regressions.json) 已记录；只增加该实际SHA，不改变八项验收证据，未满11项时仍拒绝生成采用副本。

已核实Node18的新run `41113367877` / job `124263227234` 与Node20的新run `41113373387` / job `124263887574`，两者均实际使用6fd4，包含Syft传输修复。原已编译索引分别保留49ee与BF producer身份。两次旧FD取消均在新原生任务存在、旧任务仍queued/runner0且没有任何已开始步骤时各提交一次；Node20已确认cancelled，Node18收据保留19:19Z时的pending观察，后续终态另行追加，不回写历史。见 [交接收据](evidence/fixed-node-verification-handoff/handoff-receipt.json) 与 [原文目录](evidence/fixed-node-verification-handoff/MANIFEST.json)。Ruby3.2继续复用既有4462请求；本次只解决排队交接，正式验收仍为8/11。

### 增量质量复核与交接终态 — 2026-10-30（UTC 19:34Z）

Node18旧FD请求随后也已确认cancelled，原生job仍为runner0、零步骤；[最终确认](evidence/fixed-node18-terminal/terminal-confirmation.json) 单独归档，保留此前pending收据不变。三个修复任务继续排队，没有新增派发或基础设施变更。

最近Syft重试及Ruby3.2恢复授权的完整51行增量已并行完成代码复用、质量、效率三路 `/simplify` 复核，无需进一步修改；见 [复核记录](evidence/incremental-simplify-3990/review.json)。正式11项采用数据仍需最终复核，t006未提前关闭。

六项真实应用验收的日志、精确源码、主/companion镜像、同次attempt与拒绝skip规则已做源码审阅；仅收紧临时收集器的三个参数值检查，防止误把10当作1等前缀匹配。新v2 helper与旧原文分开保留，当前workflow通过、本地三项错误后缀对照均拒绝；见 [未来收集器启用记录](evidence/native-fixture-collector-review/root-activation.json)。这不是Docker/HTTP执行结果；正式发布计数仍为8/11，接入、真实应用与线上验收继续保持开放。

### 恢复授权清理校验准备 — 2026-10-30

原临时选择器检查脚本在最终空授权目录上会因取首项而退出；新 [v2检查器](evidence/recovery-selector-empty-support/check-v2.py) 支持真实空目录，覆盖全部11项退休授权，包括Ruby3.0/3.1/3.2，且所有不复用路径必须产生零输出字节。旧脚本与历史记录保持原字节，新检查器仍提取实际workflow内的选择逻辑，在隔离Git fixture中执行，不修改真实目录。

当前真实三项授权的17项检查通过，明确报告尚未全部清理；显式临时空目录的13项对照通过，明确报告不是实际仓库清理。见 [启用与结果说明](evidence/recovery-selector-empty-support/root-activation.json)。每项正式验收通过后才删除对应授权；最终t008仍需用实际仓库目录重跑，确认真实输入为空、11项均不复用。本次没有删除授权、接入pin或运行应用验收，正式计数仍为8/11。

### 同步完整镜像层扫描流程 — 2026-10-30（UTC 20:32Z）

并行w8/m91发布的846a修复在扫描前用固定Trivy远程路径完整下载、校验镜像层，先完成两类数据库准备，再执行原有每架构20分钟、最多三次全扫描；保留Java、漏洞和secret覆盖、原始REF/platform、完整报告与签名门槛。新增并行度1、8GiB镜像/单文件限制、28GiB可用空间检查和仅清理自身临时目录。实际runner容量、完整报告与扫描成功仍需生产者重放，空间检查不是预留，也不据此宣称已修复历史EOF原因。

根复核与独立复核确认配方、运行时、版本matrix和其他门槛未变；113项同伴归档的存储及原文哈希均核对通过。先前 [待准入审核](evidence/846a-source-policy/pending-review.json) 保留false，随后 [明确来源准入](evidence/846a-source-policy/root-admission.json) 与 [独立语义复核](evidence/846a-source-policy/semantic-review.json) 分开记录。守卫只改三个常量行，明确保留旧3990、FD、4462、6fd4等来源及49ee仅Node18限制；[策略切换](evidence/846a-source-policy/policy-activation.json) 只向来源集合加入实际846a SHA，八项验收原文与59项受保护哈希保持不变。[八项完整重放](evidence/846a-source-policy/eight-candidate-regressions.json) 通过，未满11项时仍实际拒绝采用；正式进度仍为8/11。

共享恢复目录新增的Elixir1.18/1.19属于w8/m91，本次 [真实目录19项选择检查](evidence/846a-source-policy/current5-selector-controls.json) 保留全部五项授权。上节空目录对照是当时的收尾准备；现在关闭w1的条件明确为其11个旧版本授权均已移除，其他工作流未完成的授权继续保留，不要求整个共享目录为空。既有Node18/20、Ruby3.2定向请求继续，镜像接入、真实应用及线上验收保持开放。

### 旧排队任务切换到已修复流程 — 2026-10-30（UTC 21:13Z）

旧99f现代运行时发布已出现三次实际失败，83c已包含相同配方、相同27项版本matrix的扫描及Syft传输修复。为避免继续执行旧流程，核对全部68个请求与33个源码指纹后，仅选择十个queued/runner0/零步骤且没有修复流程重复请求的现代任务；三个已开始任务和本项三个旧版本请求均保留。[只读计划](evidence/modern-ci-rollforward-requests/plan-final.json) 与 [随后明确执行授权](evidence/modern-ci-rollforward-requests/root-execution-approval.json) 分别保留原文。

十个替代请求均实际使用83c/main/attempt1，每项只POST一次；[21:13Z请求收据](evidence/modern-ci-rollforward-requests/request-heads.json) 如实记录当时尚未取消旧任务。后续每项须先确认正确替代原生job已存在，再新鲜核对旧job仍queued/runner0/零步骤，才请求取消旧项；任何已开始工作继续保留。原生绑定、取消请求及真实终态另行追加，不回写这份请求阶段快照。不保证GitHub队列优先顺序。

这次CI交接保留w8/m91全部现代运行时义务，其独立镜像验收仍归该工作流；w1正式验收仍为8/11。较大API原文仅做无损压缩，[目录清单](evidence/modern-ci-rollforward-requests/MANIFEST.json) 同时记录存储与原文字节的SHA/大小；发布请求不计作镜像接入或应用验收。

### CI排队交接完成 — 2026-10-30（UTC 21:32Z）

[完成收据](evidence/modern-ci-rollforward-completed/final-receipt.json) 逐项关联十个实际83c原生job、十次新鲜旧job零步骤检查、十次HTTP202和十个最终completed/cancelled且runner0/零步骤的确认；没有重复派发、强制取消或中断已开始工作。先前的Node22索引延迟、Python3.11终态查询HTTP502及pending观察均保留原文。

[根复核](evidence/modern-ci-rollforward-completed/root-audit.json) 再次读取十组原始run/jobs，核对源码、attempt、runtime、旧任务检查及最终零步骤状态。请求阶段三个执行helper已在原归档中保持原哈希；之后新增的终态读取helper随完成阶段单独归档。所有365项请求记录与156项完成记录保持各自观察时间，不回写21:13时的零取消快照。

w8/m91的十项现代运行时工作全部保留并继续在修复流程上执行，镜像独立验收仍归该工作流。本项Node18/20、Ruby3.2三个请求保持原样；正式验收仍为8/11，继续等待其产物、接入及真实应用/线上验证。

### Go运行门槛缺包修复与旧产物精确复核 — 2026-10-30（UTC 23:14Z）

旧99f的Go1.27双架构构建完成，但ARM64实际工具验证因缺少`unzip`退出，AMD64兄弟检查随即取消；不把它归为模拟器故障或完整验收。`8af93065a`已单独修复统一安装列表的显式依赖，保留真实zip/unzip转换及其余全部门槛；语法、五项matrix测试和ship gates通过，新Go双架构运行/扫描/签名尚待实际完成。见[原始失败与修复/派发收据](evidence/go-unzip-ci-fix/MANIFEST.json)。

本次push自动全量请求`41142599053`在新鲜确认尚无原生构建后只取消一次，最终completed/cancelled，没有中断已开始工作。随后只派发一次Go1.27正常新建，实际绑定8af/main/attempt1的run`41142870861`、job`124361678277`；旧不完整Go索引没有恢复授权。Node18、Ruby3.2已开始各自原有完整恢复，Node20原队列保留。现代Go镜像的独立采用仍归w8/m91，不增加本项计数。

当前树只增加了这一个安装token；旧八项镜像实际已通过同一真实解压门槛，签名/配方原文仍保留。采用守卫只批准完整旧、新`install.sh`哈希之间的精确兼容，限定原18个已准入历史producer，其他配方、文件集合与producer到verifier的一致性仍严格；没有向本项来源集合加入83c或8af。见[明确批准、策略与八项完整重放](evidence/unzip-exact-recipe-compatibility/MANIFEST.json)。隔离正例及四项拒绝控制、实际未满11项的采用拒绝均通过，60个原文件哈希经根复核不变；context保持8/11。最初[批准消息捕获](evidence/unzip-initial-approval-capture.json)与随后根批准的追加关联均保留，不回写先前记录。实际pin接入、六项应用验收与生产rollout继续开放。

### 旧Elixir签名失败交接与接入准备回归 — 2026-10-30（UTC 23:33Z）

旧99f的Elixir1.20完成构建、双架构运行验证及完整扫描，但ARM64 Syft读取层时收到HTTP/2 `PROTOCOL_ERROR`。日志只支持镜像签名、AMD SPDX/Trivy三个命令完成，不冒充五份密码学证明；其余两项与discovery未执行。当前流程已包含对应传输修复，但统一安装配方已增加显式unzip，因此没有为旧索引添加恢复授权。初次精确800派发在发现main更新时停止，实际POST为零；复核9cb仅增加PM记录、全部非PM字节与8af相同后，全来源排重并只派发一次正常新建，实际run`41144656779` / job`124367871533`绑定9cb/main/attempt1。见[原日志、完整排重、来源与原生绑定](evidence/elixir120-current-normal-build/MANIFEST.json)。现代Elixir验收仍归w8/m91，不增加本项计数；原始404观察与零POST预检均保留。

当前采用准备工具的完整生成路径还存在两个未定义的局部变量；未满11项时的既有拒绝发生在该路径之前，因此先前没有执行到。仅在同一候选完成验证后显式读取其digest和jobURL，验证函数及全部安全/来源规则保持原字节。[明确标注的合成writer回归](evidence/SYNTHETIC-writer-locals-regression/MANIFEST.json)用同一fixture实际复现旧版NameError、新版11行/55项及各行独立digest/来源正确；只mock候选验证函数，不构成真实镜像或签名验收。真实八项收据仍拒绝生成，61项原文件哈希经根复核不变；guard为3fe1b2bd，context仍8/11且未增加现代来源角色。

Ruby3.2已于23:30Z完成双架构运行验证并进入AMD全扫描；Node18继续运行验证，Node20保持队列。共享池空出的第三个runner已于23:21:44Z执行更早的生产部署；原始API观察随本节归档，不把排队当作任务失联，也不取消已开始工作。实际11项pin接入、六项应用测试和线上rollout仍未完成。

### 刷新实际使用优先级与零副本对象识别 — 2026-10-31（UTC 00:21–00:34Z）

采用前只读刷新仍为29个App、8项候选、6个运行Pod；候选身份、实际服务镜像和版本与原09:11快照相同，两次Pod替换仍使用相同镜像。未观察到新增的运行中旧11条线。初始严格报告仍保留5项unresolved，未回填为安全或零目标；见[初始快照、分开的补充结论与全部工具记录](evidence/priority-census-owner-uid-refresh/MANIFEST.json)。

三项legacy auto的Alpine/Node22实际观察未变，客户Dockerfile来源沿用旧记录中的高置信推断，不能改写为已保留的构建Job证明。market-size为nativeNode失败状态，没有活动发布、镜像或工作负载。cloneix为nativeNode、取消且尚无活动发布，仍有当前App UID归属的零副本Deployment；通过该App现有专属拉取凭据在内存中读取固定gen3 manifest/config，验证digest与大小后，仅保留Node26.10.0/Linuxamd64版本声明。没有读取文件系统层、保存凭据或完整Env/config，也没有执行该镜像或声称安全；专属loopback连接已清理。cloneix未来重建的有效版本仍因源文件不可用及未读取的环境Secret引用而未知，不能假设未来版本或未来安全。

原清单工具只按workload标签关联，漏记该无标签的零副本Deployment；新独立v2优先核对namespace和当前App UID/name/API group，拒绝冲突归属，不增加仅名称匹配的回退，并保留原无owner标签路径。13项隔离控制复现旧漏记、新识别及错误UID等拒绝；无Pod、未知运行时或保留镜像仍保持unresolved。旧helper、原计划与初始5项快照保持原字节。后续t004改用02231095的v2及追加计划，必须在真实operator Ready后用实际部署源码重新采集并结合明确范围证据；本节不是最终rollout、零目标、安全或任务完成证明。正式发布验收仍为8/11，Node18与Ruby3.2继续签名，Node20排队。

### Ruby3.2 正式验收推进至 9/11 — 2026-10-31（UTC 00:57Z）

[Ruby3.2验收收据](evidence/ruby32-published-acceptance/acceptance.json)保留FD producer与4462 verification/证书的区别；原不可变e7e887索引完成五份严格证明、双架构直接registry子镜像/配置绑定、Ruby3.2.11身份、固定系统包、实际运行门槛及完整扫描。目标Brotli发现为零；每架构其他11,472项发现（含88项critical）保留，不作全漏洞清零结论。

根复核实际候选原文、FINAL与控制后，明确批准精确6872fd51验证哈希；966执行器再次读取新鲜匿名discovery并重放冻结3fe守卫后才计数。实际7340字节原文、root批准、元数据转换及第九项aggregate快照均保留，原八项proof/candidate的16个哈希不变。[74项归档核查](evidence/ruby32-acceptance-archive-audit.json)确认原始字节、大小、路径唯一性及全部批准/转换锚点。

已删除仅此Ruby3.2的临时恢复授权；Node18/20与其他工作流的三项Elixir授权保持原样。[退休记录](evidence/ruby32-recovery-retirement.json)与[20项实际隔离Git选择检查](evidence/recovery-selection-ruby32-retired.json)已保存。剩余Node18继续签名、Node20已经进入双架构运行验证；当前只读采用计划为9项、仍不完整且官方matrix不变。source pin接入、六项真实应用测试与生产rollout尚未执行，t002–t008继续开放。

### Node18 重复 EOF 的完整 OCI 暂存修复 — 2026-10-31（UTC 02:13Z）

Node18 的6fd4恢复任务 `41113367877` / `124263227234` 已失败：运行门槛及两份完整扫描通过，ARM64 Syft三次读取中两次记录不同层的unexpected EOF，中间一次约30分钟后失败。中间一次只能按耗时推断deadline，日志没有exit124。镜像签名及AMD两份证明仅有命令完成证据，不能算五份严格验收；见[原始日志与终态](evidence/node18-syft-eof-failure/ARCHIVE-MANIFEST.json)。

修复提交 `da0e56e80` 先按选定平台逐个下载完整OCI元数据、配置与压缩层，核验大小和SHA256，失败时最多三次有界续传；只有校验完成的布局才交给固定Syft1.55.0本地读取。旧固定action默认Syft1.51.1的OCI目录路径缺少架构信息，未增加空架构例外；[1.55.0的小型真实工具格式对照](evidence/native-oci-staging-fix/syft155-format/compatibility-review.json)保留原始child/config并拒绝错误平台。该对照不算真实运行时镜像验收。原全部扫描、现代签名和标签发布门槛保留。[32项组件/实际shell隔离控制](evidence/native-oci-staging-fix/controls/receipt.json)及Actions、actionlint、image-pin和ship门槛通过；三路simplify另修正了完整EOF边界和每次Syft重试的私有缓存清理，已验证的布局保留。

当前安装配方已增加unzip，与49ee原产物不同，因此Node18下一次须正常新建，不把旧7dd索引伪装为当前配方。Node20原6fd4请求继续；已有九项验收、官方版本matrix与未采用的source pin保持原样。定向构建和最终接入验收继续，t002–t008仍开放。

根与独立复核完成实际da0e的完整发布输入闭包，含新增helper及测试、未变27项官方matrix和44项inventory；只将其加入精确producer/verifier来源。守卫只改三个常量行，全部函数字节不变，显式保留旧846和全部历史来源、49ee仅Node18与既有安装配方兼容边界。九项真实完整守卫重放通过，未齐11项仍实际拒绝生成接入副本，85项原证据哈希未变；见[来源、批准、切换及重放原文](evidence/da0e-source-policy/MANIFEST.json)。本次自动全量请求 `41158989325` 只在新鲜确认原生任务尚未分配runner或开始步骤后取消一次，最终cancelled；[全部请求及终态](evidence/da0e-own-batch-cancellation/ARCHIVE-MANIFEST.json)保留，Node20与其他工作流不受影响。

全来源排重与两次精确main检查后，只提交一次Node18正常新建请求；实际[run41159591513](https://github.com/bex-co/bex/actions/runs/41159591513)与原生job `124414471403` 已确认使用da0e/main/attempt1、只选择Node18。截至02:24Z，matrix成功，原生job排队且尚无已开始步骤；见[请求、实际选择器、排重与绑定原文](evidence/node18-oci-normal-request/ARCHIVE-MANIFEST.json)。旧7dd授权保留但实际选择器输出为零，不复用已变配方。请求和排队均不增加验收计数；正式进度仍为9/11，后续继续核对完整产物。

### Node20 正式验收推进至 10/11 — 2026-10-31（UTC 03:04Z）

[Node20验收收据](evidence/node20-published-acceptance/acceptance.json)绑定原BF producer、6fd verifier/证书及不可变2c69d3索引。02:57Z任务成功后，五份原始严格密码学证明、三项真实CLI负向对照、双架构直接registry子镜像/配置绑定、Node20.20.2/Brotli1.1.1、固定系统包及实际Bullseye companion门槛全部通过。目标Brotli发现为零；每架构其他11,171项发现（含72项critical）完整保留，不作全漏洞清零结论。

独立完整2a守卫与四项结构拒绝通过后，根复核精确FINAL `2169a6fe…`，连同独立追加的 `50e2e266…` 说明更正一并批准。更正只说明冻结文件中沿用旧模板的一句来源文字；结构化BF/6fd/da0e、原始证明和实际检查均正确，冻结原文未重写。执行器随后重新匿名核对stable标签并重放冻结守卫，才原子计入第十项。实际8537字节原候选、root批准、说明更正、转换和第十项快照均保存；原九项proof/candidate的18个哈希未变。[92项归档核查](evidence/node20-acceptance-archive-audit.json)确认全部字节、路径及批准锚点。

只删除已验收Node20的临时恢复授权；Node18旧7dd条目及其他工作流的三项Elixir条目保留，见[退休记录](evidence/node20-recovery-retirement.json)及[20项实际选择器隔离Git检查](evidence/recovery-selection-node20-retired.json)。[当前只读接入计划](evidence/adoption-plan-count10.json)仍不完整、27项官方matrix不变，尚未生成接入副本。Node18的da0e正常新建请求继续排队；source pin、六项真实应用测试与生产rollout尚未完成，t002–t008继续开放。

### 补齐最终线上身份与收尾检查 — 2026-10-31（UTC 04:12Z）

源码核对确认native resolver从manager的BackupHelperImage取得镜像，后者可由环境覆盖或按POD_NAME/POD_NAMESPACE读取指定Pod；旧Ready工具只看新manager镜像与Ready，未核对这条路由。新增[明确批准的v2检查及30项隔离控制](evidence/final-ready-selfimage-v2/MANIFEST.json)同时检查模板和实际当前Pod的唯一Downward API身份及无override/envFrom，拒绝16种旧误通过；原镜像、GitOps、owner/imageID、新Pod Ready since及外部签名/源码条件全部保留。没有生产调用，未将合成控制算作实际rollout，最终仍需实际部署源码与真实Ready后的报告。

另准备[只读迁移检查器及04:04Z快照](evidence/closeout-dryrun-preparation/MANIFEST.json)：当时17处人工Markdown链接的虚拟移动目标均存在，2450项原证据建立字节哈希清单。历史静态工具和计划保留原文；证据内的Markdown、JSON、原命令和SHA均不可改写。实际t008前须对最新任务位置及新增live-acceptance重新运行，核对原证据后才移动整个目录，不留旧stub。此准备未改变任何任务完成状态，正式发布验收仍为10/11，Node18继续排队。

### 主线新增外部证书续期门槛 — 2026-10-31

同步主线后核实deploy `41124028938` 的失败来自fallback证书不足30天；集群当前证书的公开元数据显示2026-11-30到期。只读取证书元数据，未读取私钥；现有本机配置、标准续期目录和连接工具未提供可用续期入口，见[核对记录](evidence/main-ci-cert-renewal-intake.json)。上游已记录在[w10/010](../../../w10/blocked/010.md)；已异步向用户索取现有续期入口/安全凭据位置或新证书与私钥文件路径，不要求在聊天中粘贴凭据。原30天/SAN/信任/keypair门槛保持不变。

按主线红CI规则继续推送已完成工作；Node18、11项接入和独立应用验收仍继续，实际生产rollout须待证书门槛清除。不因该新增门槛提前结束仍可实施的工作，也不将当前状态计入完成。

### 自动续期修复已进入主线 — 2026-10-31（UTC 07:46Z）

同步至25a222016后核实，其他工作流已按用户接受的[ADR093](../../../../docs/ADR093-onbex-wildcard-tls.md)在e12166f37改用cert-manager/Cloudflare DNS-01自动续期，并替换旧手动PEM的30天部署门槛。production-deploy已存在新的token名称（只读名称与更新时间，未读值）；当前公开edge通过新规则下的可信证书、14天有效期及404检查，但仍服务旧证书。见[当前门槛核对](evidence/main-ci-cert-gate-superseded.json)。

之前向用户索取手动证书与私钥文件的请求已被新方案替代，无需再回答；原核对记录保留。e121部署41178940668仍排队，未声称自动首次签发或实际新operator Ready。继续Node18、全部11项接入、六项真实应用验收及最终生产验证；不把源代码修复当成线上验收完成。

### 当前主线传输修复的精确兼容性复核 — 2026-10-31（UTC 08:44Z）

94165237d增加发布时Python wheel与PGDG key的有界传输重试，2cc8723df将JavaDB预取延长至30分钟；完整镜像扫描的20分钟预算、全部重试、完整报告和签名门槛保留。wheel输入与校验、PGDG固定key、版本和补丁未变。fetch helper会留在镜像，且部分HTTP失败行为不同，因此只认可已验证成功产物的合同兼容，不声称镜像、apt状态或失败行为逐字节相同。

根与独立复核后启用精确4cfe9fb8检查：只允许已审阅的941配方与2cc工作流差异，核对精确文件哈希、两个新增测试路径及941→2cc→当前HEAD祖先关系；原DA0产物来源权限、producer/verifier配方严格相等和全部密码学检查保留，941/2cc未获产物来源权限。候选a349因缺少祖先约束被退回，修正后才批准，旧版从未启用。10项真实完整守卫重放通过，17项隔离控制（1正、16负）通过，未齐11项仍实际拒绝生成接入副本；95份其他受保护原文与aggregate保持不变，唯一预期替换是活动检查2a→4cfe。

[全部提案、原始测试、独立复核、根批准与启用记录](evidence/current-tree-compatibility-2cc/MANIFEST.json)已归档，[87文件根核查](evidence/current-tree-compatibility-archive-audit.json)验证存储及原始字节、37项当前源码输入和旧/新活动路径按内容哈希定位。三份原始diff无损gzip，收尾定位器须使用original_sha256/original_bytes还原，不改写历史原文。验收仍为10/11，Node18的DA0正常请求继续排队；未生成接入副本，六项真实应用验收与最终生产验证仍开放。

### Node18 取消后的正常请求恢复与来源授权 — 2026-10-31（UTC 09:35Z）

DA0请求41159591513的原生job124414471403于08:51:55Z被取消；runner为0、步骤为空，没有丢失编译成果。实际check annotation记录取消来自@puncsky账号，不能据此推断具体人工、agent会话或原因。完整85项工作流核对确认没有其他排队/运行Node18或未展开matrix风险；2cc自动批次的27项原生任务也已取消且没有执行原生步骤。原始记录见[完整恢复与来源证据](evidence/node18-dbc-recovery-source/MANIFEST.json)。

根复核精确37项发布输入与正常构建选择器后，只提交一次Node18定向正常请求，09:12:40Z获204。已确认实际[run41187749759](https://github.com/bex-co/bex/actions/runs/41187749759)、源码dbc493ba、main/workflow_dispatch/attempt1，matrix通过且只选择原生job124498652791。旧7dd配方不同，实际选择器输出为空，正常重新构建；不把旧索引算作已验收。主线移动期间产生的9fe草案虽通过审查但从未启用，实际run绑定后才批准真正DBC来源。

活动检查更新为精确1690cdaa，只新增DBC的Node18 producer与verification角色；两者各自受Node18限制，原49ee verifier语义、DA0锚点、941/2cc当前源码兼容性、配方严格配对及全部原密码学/运行门槛保留。10项真实完整重放、8项来源边界反例及未齐11项的真实拒绝通过。aggregate仅添加实际DBC的accepted_sources，10份候选及其余字段不变；108份其他受保护原文保留。

[151文件根归档核对](evidence/node18-dbc-recovery-archive-audit.json)验证原始及存储字节、两份无损压缩diff和113项跨归档内容引用；包含完整47份最终调度阶段文件及其清单、原取消/去重、从未启用的9fe历史、实际DBC批准/启用与观察者交接。活动G/Q路径须按预期SHA定位历史冻结版本，原始pending/false记录保留其时间语义。源授权与排队不增加验收计数；正式仍10/11，Node18待实际构建/全部验收，接入副本、六项应用与生产验证继续开放。


### Node18 正式验收与公共底交接 — 2026-10-31（UTC 13:40Z）

[Node18实际验收归档](evidence/node18-published-acceptance/MANIFEST.json)绑定正常DBC producer/verifier、run41187749759/job124498652791与不可变6c40aaf4索引。双架构构建、真实离线工具门槛、两份完整扫描与签名于13:06:58Z完成；五份严格原始证明、三个实际CLI反例、直接child/config绑定与Bullseye companion均经完整守卫验证。根审核并批准精确FINAL4e393661，执行器在一次未写入清单的TLS失败后，成功新鲜匿名核对stable标签并重放冻结65检查，才计入第11项。Node18.20.8、内置Brotli1.1.1及固定系统包的身份保留；目标Brotli发现为零，其他每架构13,347项发现（含228项critical）完整保留。

[94文件根核对](evidence/node18-acceptance-archive-audit.json)确认原始/存储字节、实际6408字节候选与仅五项元数据转换、root批准和失败/成功调度日志；约69MB完整报告压缩缓存未重复写入Git，其公共不可变OCI引用与原始/压缩哈希保留，根另外流式核对了368,188,556字节解压内容。旧7dd没有被接纳，仅因不同的新6c40已验证而退役；三项其他工作流Elixir授权保持原样。实际20项选择器检查、当前021e目录下全部11项完整重放，以及真实11项receipt的旧写入拒绝均通过。source pin与45项inventory未变。

[公共底决定与仅产物验证策略](evidence/common-proof-only-policy/MANIFEST.json)由[127文件根核对](evidence/common-proof-only-policy-archive-audit.json)保留：49项精确C5源码、原始用户决定、从未启用的C2草案、全局删除旧逐语言写入的65检查、52项来源控制、10项既有证明与当时待接纳Node18的回归、17项隔离控制及三种写入拒绝。C5只获当前代码比较资格，未增加产物producer/verifier角色；旧source、certificate、recipe严格配对不变。激活时count10与当时proposed/pending字段保持原文，后续11项实际完成由独立验收记录说明。

[应用验收前置检查与三路复核](evidence/common-acceptance-precondition/MANIFEST.json)保留完整build包通过、当前空公共底的三个实际integration拒绝及Actions通过。原程序、数据、build/start、decoder和HTTP断言不变；没有把拒绝或默认跳过计作真实公共底构建。t002现已完成；t003–t008的公共底接入、六项真实应用与生产验收仍开放。

下一步与明确责任见[中文交接](common-integration-handoff.md)和[逐版本机器可读记录](evidence/common-integration-handoff/common-integration-handoff.json)。需要worker8先提供合格公共底和同版本旧运行时闭包；现有Debian suite保护不能被删除来绕过兼容性。用户已完成两项决策，无需再选型；发布11/11只表示产物证据完成，不能当作公共底或线上修复。


### 本轮停放条件 — 2026-10-31

[最新远端源码核对](evidence/common-integration-gate-at-parking.json)确认公共底pin仍为空、发行版一致性保护保留，w8/m91/t007的实际公共底/旧运行时资格仍未完成。发布、完整产物审阅、旧授权清理、验收前置保护与[交接](common-integration-handoff.md)均已落实，t001/t002完成；t003为外部依赖阻塞，其余任务等待实际集成后继续。按loopx将整个里程碑停放到blocked，不将未满足的公共底或线上DoD标为完成，也不把已成功的Node18任务当作阻塞。

解除条件是worker8提供真实合格产物与生成镜像证据；届时`$loopx w1`继续实际接入和线上验收。`/unblock w1`可读取本记录解释下一步，无需重复询问已经明确的维护/公共底选择。


### 同步主线后的门槛复查 — 2026-10-31

[同步后记录](evidence/common-integration-gate-after-sync.json)确认e36b1462c新增的是原始Node工具重放源码与控制；上游明确保留真实公共底/旧版本双架构资格，两个生产pin仍为空。该准备没有解除本项阻塞，也没有给新源码增加产物角色。继续沿用已确认决定和交接，不把源码测试或11份逐语言证明当作公共底生产验收。
