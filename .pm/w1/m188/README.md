# w1 · m188 — 旧版 native 运行时的安全补丁由平台集中维护

**Worker:** worker1 **Goal:** 用经过验证的平台补丁镜像解除旧版官方镜像的上游等待，保持用户选定的运行时版本，优先保护实际在用的受影响服务。 **Status:** in progress — t001 done；Node14、Node16、Python3.7 签名发布独立验收通过（3/11），继续其余候选与接入验收

## 用户决定 — D1=A，2026-10-29

- **来源：** 用户在 `/unblock w1` 中明确回复 `A`：允许平台集中修补，优先实际在用、确认受影响的旧版本，并接受由平台承担持续构建、安全扫描和兼容性验证。
- **边界：** 保留用户选定的语言版本；补丁在平台镜像发布时制作，租户构建直接使用固定镜像，不在每次租户 build 临时 apt 升级。
- **修复责任：** 同时核实系统库和运行时自带库，不能只升级系统 libbrotli1 就宣布所有运行时已修复。
- **当前用户动作：** 这项维护范围决定已完成，无需重复确认。实现、验证及后续具体执行条件由 worker 按既有规则处理。
- **状态：** 不再以“等官方镜像更新”为阻塞条件；用户随后执行 `$loopx w1`，已授权并开始实现和发布。当前逐版本验收进度见下文，全部完成前不标记 done。

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | 清点旧版运行时的实际使用与受影响库 — **DONE** | 45m | — |
| t002 | 建立平台统一补丁镜像的构建、验证与发布流程 | 2h | w1/m188/t001 |
| t003 | 接入 native pin 与持续维护记录 | 45m | w1/m188/t002 |
| t004 | 验证发布、rollout 与受影响服务重建 | 1h | w1/m188/t003 |
| t005 | Render parity | 20m | w1/m188/t004 |
| t006 | Simplify | 20m | w1/m188/t005 |
| t007 | Test coverage | 45m | w1/m188/t005, w1/m188/t006 |
| t008 | Closeout | 15m | w1/m188/t007 |

## Definition of done

- 原 143 的 11 个版本均有最新、可复验的受影响状态和处置结论；缺 SBOM 不被当作安全，在用且受影响的版本优先，其他受支持版本不被静默忽略或移除。
- 需要平台修补的版本具有可重复的镜像输入、补丁来源、签名/SBOM、扫描与实际修复验证；覆盖需要的架构，运行时版本选择和兼容性合同保持一致。
- 已发布且验证的镜像进入 source pin 与 freshness inventory，持续构建、扫描、验证和失败保留原 pin 的流程可执行，维护责任及回归官方镜像条件有记录。
- 实际在用的受影响服务有发布、rollout、重建和运行验收证据；只更新源码或镜像标签不算完成。
- 无法在同版本下安全修补的情况必须有具体证据和明确剩余处置，不静默升级或禁用，也不因用户已授权而宣称验收通过。
- 存在仍受影响且未解决的版本时保持里程碑未完成；只有用户明确接受其他处置并更新范围后，才可重新评估完成条件。

## Source + Goal linkage

- **Source:** [w1/143 原始记录与用户决定](../done/143.md)、[逐镜像核查证据](../evidence/136-brotli-upstream-20261028.json)。2026-10-29 用户选择 D1=A；交接时 checkout 为 `6792b9cf2`，实施时复核最新上游和实际服务现状。
- **Goal linkage:** ADR008 的可靠、安全且 Render 兼容的托管能力；ADR060 的受审查构建来源与版本合同。
- **Expected outcome:** worker 可以自行修补并维护必要的旧版镜像，受影响服务用上已验证补丁，不继续被上游发布时间挡住。
- **Why now:** 上一轮可用更新已由 136 独立推进；剩余旧版标签缺少已验证修复，用户现在明确接受平台维护责任，原上游等待条件已解除。
- **Render parity:** 包含 t005，因为接入影响租户 native 构建的版本选择与部署行为，需验证所有入口和 workspace 语义一致；不新增 API/UI 功能。
- **Sizing:** 明显超过约 1 小时且跨现状核查、镜像发布、接入和线上验收，因此从 inbox 143 提升为里程碑。表内为初估，t001 根据实际库与回移工作校准，不把未知成本当成已验证结论。
- **Related work:** [w1/136](../done/136.md) 的 19 个已更新 pin 与 CNB 后续发布/重建保持独立；本里程碑不重复实现它。

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
