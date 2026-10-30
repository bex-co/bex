# w1 · m188 — 旧版 native 运行时的安全补丁由平台集中维护

**Worker:** worker1 **Goal:** 用经过验证的平台补丁镜像解除旧版官方镜像的上游等待，保持用户选定的运行时版本，优先保护实际在用的受影响服务。 **Status:** in progress — t001 done；修正后的本地构建/启动验收完成，推进双架构签名发布与接入验收

## 用户决定 — D1=A，2026-10-29

- **来源：** 用户在 `/unblock w1` 中明确回复 `A`：允许平台集中修补，优先实际在用、确认受影响的旧版本，并接受由平台承担持续构建、安全扫描和兼容性验证。
- **边界：** 保留用户选定的语言版本；补丁在平台镜像发布时制作，租户构建直接使用固定镜像，不在每次租户 build 临时 apt 升级。
- **修复责任：** 同时核实系统库和运行时自带库，不能只升级系统 libbrotli1 就宣布所有运行时已修复。
- **当前用户动作：** 这项维护范围决定已完成，无需重复确认。实现、验证及后续具体执行条件由 worker 按既有规则处理。
- **状态：** 不再以“等官方镜像更新”为阻塞条件；修复尚未实施或验收，不标记 done。本次仅记录决定并准备交接，不自动启动实现或发布。

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

用户已执行 `$loopx w1`，开始实现；上文“本次仅记录决定”保留的是决策阶段历史。t001 核查完成，见 [assessment.md](assessment.md)。main 已有统一 native 基础镜像发布流程，t002 复用该流程增加系统及 Node 自带 Brotli 修复，不新增平行框架。尚未完成发布、接入和验收。

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
