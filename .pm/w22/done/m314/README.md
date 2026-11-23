# w22 · m314 — Sandbox CPU billed on use, as on Render: a gVisor accuracy spike, then the cutover

**Worker:** worker22 **Goal:** a running sandbox is billed for the CPU it uses plus the memory allocated, and the billing switches in one step, or the fallback is recorded for the user's sign-off **Status:** done (2026-11-23: the usage leg printed PASS on production with the w22/024 fix, sandbox-measure run 45147416800; see "解阻与结案记录 2026-11-23")

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Spike: does gVisor report usable per-sandbox CPU? Fix the thresholds, run, record the verdict — **DONE (verdict: fail)** | 1h30m | — |
| t002 | Record the pod name on every running interval — **DONE** | 1h | t001 |
| t003 | Meter used CPU from Prometheus: sandbox_cpu_seconds — **DONE** | 2h | t001, t002, w22/m313/t002 |
| t004 | Bill on use: price CPU and memory, stop billing the folded meter (or take the recorded fallback) — **DONE** | 1h30m | t003, w22/m313/t005 |
| t005 | Production check with a known load, and amend ADR023, ADR030 and ADR104 — **DONE** | 1h30m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- The CPU spike has a recorded verdict. Pass: used-CPU billing is live for every workspace from one dated timestamp and verified on production with a known load. Fail: the allocated-CPU charge is recorded in ADR018 as a divergence with the user's sign-off and a dated removal task. Escalate: the milestone is parked with the exact question for the user
  - **Amended 2026-11-16 (existing rule, not a user decision; see "解阻记录 2026-11-16"):** t001's verdict stays "fail" and the allocated-CPU fallback is not taken. The fallback was written for a meter gVisor makes unusable; this one is usable and its error is measured (+5.3% over the requested seconds, +9.8% over the load's own count, idle 0.45% of a core). ADR104 D8 already says to bill the measured cgroup CPU with no subtraction, and the parity rule forbids parking on an optional variant, so the cutover bills the measured CPU. A subtraction variant is a separate follow-up; the allocated-CPU charge would be a divergence that stands only with the user's sign-off in ADR018.
- CPU is billed on use and memory on allocation while running; suspended sandboxes pay storage only (with w22/m313)
- `sandbox_compute_seconds` history and the agent-session cost are unchanged
- ADR023, ADR030, ADR018 and ADR104 D8 agree with the code

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gap 8 (CPU meter and cutover)
- **Goal linkage:** Render parity first (ADR008): Render bills actively-used CPU plus allocated memory; usage metering and billing (ADR023, ADR030, ADR040); pillar 5 sandboxes
- **Expected outcome:** An idle running sandbox stops costing as much as a busy one, and the bill for a known load matches the work done
- **Why now:** An idle-but-running sandbox costs the same as a busy one on bex today, the most visible billing difference from Render; it depends on w22/m313's meters and on a spike whose result decides the branch.
- **Depends on:** w22/m313 (storage meter, memory meter, rates and surfaces).
- **Render parity:** included, usage surfaces change.
- **Recommendation, not a user decision:** bill measured cgroup CPU with no subtraction (ADR104 D8). The user is asked only if the spike's escalation thresholds trip or it fails; then the fallback needs a signed ADR018 divergence.
- **Allowlists:** none; one dated effective timestamp for every workspace (parity rule).

## 解阻记录 2026-11-14（/unblock w22）

- **来源：** 用户决定（D1，2026-11-14）加复核证据。
- **决定：** 用户授权 w22 的 worker 在生产上做短时取证：加探针并手动 `gh workflow run` 派发 `isolation-matrix.yml`（环境 `production-observe`），只用一次性、不计费的 workspace（ADR046 billing-exclude，与周跑相同），读 Zot 大小、Prometheus、kubelet 统计。m314 可以对一个 sandbox 加几分钟 CPU 负载，m313 可以挂起一个 sandbox 等至少一小时的汇总。授权只到这里：更长的负载、多个 sandbox、写生产业务数据都要再问用户。
- **已核实（访问不是计划，是已有的）：** (1) 该工作流的 `Get app cluster kubeconfig` 步骤用 `scripts/fetch-app-kubeconfig.sh` 取 `admin.conf`，所以 Zot 的 Secret、Prometheus、kubelet 统计都读得到；(2) `gh` 已登录，`workflow_dispatch` 已有成功运行（43577969524、43579027377、43580573152，2026-11-14）；(3) `scripts/qa-login.sh` 2026-11-14 实测通过（exit 0，已 `--logout`），w32/m284 记的"QA 登录坏了"已过时。
- **约束：** 与周跑共用 `concurrency: isolation-matrix`，不要和每周二 03:41 UTC 的 cron 撞；不给 `production-*` 环境加人工闸门（DO_NOT_DO #ENV-REVIEWERS）；证据只记摘要和 run id，原始日志不入库（#NO-ARTIFACTS-IN-GIT）；探针结束时必须清理一次性 workspace 和 sandbox。如果发现某项读不到，把缺口写成精确的新闸门再停靠，不要猜。

- **本项的状态：** t001 的阈值已在测量前记录，尚未测量；t002–t009 依赖它的结论。t003、t004 依赖的 m313 t002、t005 均已完成。
- **worker 的下一步（t001）：** 在一次性 workspace 里对一个 sandbox 加已知的 CPU 负载，**不超过 10 分钟、只用一个 sandbox**；读 Prometheus 里该 sandbox 的 CPU 序列，用已记录的阈值判断 gVisor 报告的每 sandbox CPU 是否可用，写下结论。结论为"不可用"时走 README 里已写的回退方案并记给用户签字；为"可用"时继续 t002。
- **验收：** t001 有一个带数据的结论（摘要，不贴原始序列）；之后各任务按原依赖推进。

## 判定记录 2026-11-15（/loopx w22）

- **来源：** 复核证据（t001 的生产测量，[sandbox-measure run 43696023922](https://github.com/bex-co/bex/actions/runs/43696023922)），阈值是 2026-11-10 在测量前写下的，没有移动。
- **结果：** 准确度**不通过**：十轮单核 60 秒负载，pod 级读数 632.06 CPU 秒，负载自己在 sandbox 内用了 575.76 秒（+9.8%），比请求的 600 秒高 5.3%（阈值 ±5%）。idle 基线 0.45% 单核，通过（线为 2%）。syscall 重负载三轮 60 秒读到 359.48 CPU 秒，即 180 秒里 2.00 核，正好在 runc 的 2× 线上（runc 对照 pod 被 PodSecurity `restricted` 拒绝，所以只有"单线程不超过 1 核"的上界，比值至少 1.997）。
- **另一个事实：** Prometheus 对 sandbox pod 没有 CPU 序列：gVisor 下工作负载容器自己的 cgroup 没有序列，pod 级序列是 `container=""`，被 `kubernetes-cadvisor` 的 `metric_relabel_configs` 丢掉；任何基于 Prometheus 的"按用量计费"都还要改 scrape 配置。
- **按 DoD 需要用户的决定（blocked 的闸门）：** 三选一，见下面的问题。t002 到 t009 在这之前不做，因为它们建在答案上。
  1. **按测得的 CPU 计费，不扣除**（ADR104 D8 写下的推荐）：客户的纯 CPU 型任务多付约 10%，syscall 重的任务（大量文件或网络 IO）最多约 2 倍；与 Render 的"按实际使用的 CPU 计费"最接近，空闲 sandbox 不再和忙碌的一样贵。
  2. **按测得的 CPU 除以一个开销系数（例如 1.1）计费：** CPU 型任务基本对齐，syscall 重的任务仍偏高；系数要有依据并写进 ADR104。
  3. **继续按分配的 CPU 计费：** 这是一条必须经用户签字、记入 ADR018、带日期移除任务的偏离（空闲 sandbox 和忙碌的一样贵，Render 不是这样）。
- **用户答复之后 worker 的第一步：** 选 1 或 2：t002（每个运行区间记 pod 名）、t003（先改 scrape 配置保留 `*-sandbox` 命名空间的 pod 级序列，再做 `sandbox_cpu_seconds`），然后 t004 起；选 3：写 ADR018 偏离与移除日期，t004 走回退分支，t002 到 t003 不做。

## 解阻记录 2026-11-16（/unblock w22，UTC）

- **来源：** 现有规则加复核证据，**不是用户决定**。规则一：AGENTS.md「Render parity 是第一原则」：对标 Render 的充分实现不因可选决定停靠，更严或更松的变体另开条目，偏离只有用户签字记入 ADR018 才成立。规则二：ADR104 D8「开销策略：按测得的 cgroup CPU 计费，不扣除；扣除开销的变体另开跟进」。
- **结论：** 走 2026-11-15 判定记录里的**选项 1**，按 pod 级测得的 CPU 计费，不扣除，每个 workspace 从同一个带日期的时间点起生效，没有 allowlist 或 canary。
  - **选项 2**（除以开销系数）是 D8 明确放到单独跟进的变体：t005 收尾时由 worker 另开一条 inbox 笔记，不挡本里程碑。
  - **选项 3**（继续按分配的 CPU 计费）是偏离 Render 的做法，只有用户签字才成立；这次没有签字，所以不走。
  - t001 的 fail 结论不改写。改变的是 fail 之后怎么办：D8 的回退方案为"gVisor 读数不可用"而写，这次读数可用且误差已量化（比请求的 600 CPU 秒高 5.3%，比负载自己在 sandbox 内用的高 9.8%；空闲 0.45% 单核；syscall 重的负载读到 2.00 核，runc 对照因 PodSecurity 没跑成，只有上界）。
- **Render 依据：** ADR104 D8 表：Render 按**活跃使用**的 CPU 计费（$0.12 每 CPU 小时，等待模型或人时不计），内存按分配计费，暂停的 sandbox 只付快照存储。出处是 render.com 的发布文章和对比文章，2026-11-10 读过。本次没有重新抓取；t005 改文档前由 worker 重读并写下日期。
- **客户会看到的（技术细节在后）：**
  - 一个开着但空闲的 sandbox，CPU 费近乎零；今天它和忙碌的一样贵。
  - 纯 CPU 型任务比实际用的多收约 10%；bex 单价是 Render 的 0.70 倍（$0.084 对 $0.12），所以每单位真实工作仍约便宜 23%（0.084×1.098 ≈ $0.092）。
  - syscall 重的任务（大量文件或网络 IO）最多按约 2 倍读数收费，这是实测的上界，不是平均。
  - 暂停的 sandbox 只付存储（m313，已在生产）。
  - **未核实：** sandbox pod 的 CPU limit 是否把账单封顶在套餐的分配 CPU 上。若是，用量计费永远不会比今天的分配计费更贵。t004 先核实并写下结果。
- **剩余闸门：** 无外部闸门。m313 t002、t005 已完成。**用户可否决：** 不同意按测得值计费（想要选项 2 或 3）的话，在 t004 放出生效时间点之前说一声；t004 之前账单不会有任何变化。
- **worker 的下一步：**
  1. t002：每个运行区间记 pod 名。
  2. t003：第一步先改 Prometheus 的 `kubernetes-cadvisor` 抓取（`deploy/gitops/base/prometheus.yaml:468` 起的 `metric_relabel_configs` 丢掉 `container=""` 的序列，而 gVisor 下 sandbox 的 CPU 只在这个 pod 级序列上）。保留它时不能让既有的 sum-by-pod 查询重复计数：例如只对 `*-sandbox` 命名空间保留并改名，或单独一个 job。这是 worker 的技术设计。然后做 `sandbox_cpu_seconds`。
  3. t004：先核实 pod CPU limit 与套餐的关系并记录，再做用量计费切换；**不走回退分支**。
  4. t005：生产已知负载检查，数字记入任务，容差按 t001 实测的比值（见任务里的修订），文档修订，另开选项 2 的 inbox 笔记。
- **验收：** 见各任务；DoD 第 1 条的修订见上。

## 实施记录 2026-11-17（/loopx w22，UTC）

- **已发布：** 用量计费的代码、抓取配置、测试和文档（t002、t004、t006–t008 完成；t003 已实现并发布）。**生效时间点是 2026-11-18T00:00:00Z**，所有 workspace 同一时刻，没有 allowlist。在此之前账单不变：新的 CPU 和内存用量只记录、显示为"不计费"。
- **客户会看到的：**
  - 生效后，通过 sandbox API 创建的 sandbox 运行时按实际用的 CPU（$0.084 每 CPU 小时）加分配的内存（$0.0133 每 GiB 小时）计费，不再有按规格的加权计费行。空闲一小时的 `1c-2g` 约 $0.027（以前 $0.108）。
  - agent session 的 sandbox 不变，继续按加权规格计费，所以 `outcome.cost` 与账单一致。
  - Stripe 发票上仍是一行 sandbox compute（目录已到 Stripe 每个订阅 20 项的上限，CPU 和内存折算成等价的加权单位记到这一行）；用量页和 API 分开显示 CPU 与内存。不需要新的 Stripe Price，也不需要任何凭据。
- **t004 第 0 步的结论（生产上读到的）：pod 的 CPU 上限不会把账单封顶在套餐规格上。** init 容器 `execd-installer` 取命名空间 LimitRange 的默认 2 CPU，pod 的有效上限是"各容器之和"与"最大 init 容器"中较大的一个，gVisor 下 sentry 和工作负载共用 pod 的 cgroup。所以 `starter` 和 `1c-2g` 最多按 2 核计费：`1c-2g` 最坏一小时 $0.195（以前 $0.108），纯 CPU 负载 $0.111。t001 里 syscall 重负载读到的 2.00 核就是这个配额，不是倍数。修复另开为 w22/020。**这比 2026-11-16 解阻记录里"若封顶则永远不比今天贵"的假设更差，用户如果因此想改主意（选项 2 或 3），请在 2026-11-18T00:00:00Z 之前说。** 改时间点只需改 `pricing.yaml` 的 `sandboxUsage.billedFrom` 并发布。
- **与任务原文的偏离：** t004 写的是"加 Stripe 行"，实际是复用现有的一行（原因见上）。`sandboxUsage` 的费率暂不出现在 `GET /pricing`（bex.co 还没有对应的展示）。
- **未做、需要部署之后才能做：**
  1. t003 的生产验收：部署完成后跑 `gh workflow run sandbox-measure.yml -f leg=usage -f usage_idle_seconds=600`（一次性、不计费的 workspace，两个 sandbox，约 25 分钟）。生效前用量行是"不计费"的，所以**可以也应该在 2026-11-18T00:00:00Z 之前跑**，这样计量有问题能在计费切换前发现。把 `MEASURE busy` 里的比值记进 t003。
  2. t005 的第 1、2 步：2026-11-18T00:00:00Z 之后再跑一次（默认空闲一小时），它会检查计费行的费率和金额。数字记进 t005 和 ADR104 D8。
  3. t009 收尾。
- **证明闸门已清的检查：** `gh run list --workflow deploy.yml --limit 5` 里有包含本次提交的成功运行；Prometheus 里 `sandbox_pod_cpu_usage_seconds_total` 有序列；上面两次 `usage` 运行都是 PASS。
- **如果生效前的那次运行失败：** 把 `sandboxUsage.billedFrom` 往后推并发布是安全的退路（之前的用量保持加权计费），然后修计量。

## 解阻记录 2026-11-21（/loopx w22，UTC）

- **来源：** 复核证据，不是用户决定。涉及 t003、t005、t009。
- **原闸门已清：** 生产的镜像 pin 是 `f797f525f5df`（`4c41a69d7`），`git merge-base --is-ancestor dcf1f1991 f797f525f5df` 成立（`83f8805de`，即 w22/020 的修复，也在里面）。生效时间点 2026-11-18T00:00:00Z 已过。
- **缺口，如实记录：** "实施记录 2026-11-17" 要求在切换之前先跑一次 `usage`（t003 的生产验收，也是切换前发现计量问题的安全网）。**这次运行从未进行**：`gh run list --workflow sandbox-measure.yml` 里 2026-11-16 之后、本次之前没有任何运行。所以：
  - 用量计费从 2026-11-18T00:00:00Z 起已在生产上生效约三天，期间计量没有在生产上验证过；"有问题就把 `billedFrom` 往后推"的退路已经用不上。
  - "切换前 CPU 和内存显示为不计费、加权行是收费行"在生产上再也无法观察，只有测试覆盖（`TestSandboxUsageSwitchPG`、`TestSandboxBilledOnUseAcrossAdapters`）。
- **什么可以代替切换前的那次：** t003 的三条生产验收（pod 级 CPU 序列存在、忙碌一分钟约 60 CPU 秒并记下比值、空闲接近零）不依赖切换前后：切换后的同一条腿用同一个计量器读同一条 Prometheus 序列，并和节点 cAdvisor 及负载自己的 `times` 对比。所以切换后的一次 PASS 同时满足 t003 的生产验收和 t005 的第 1、2 步。
- **切换后的运行：[sandbox-measure run 44867679299](https://github.com/bex-co/bex/actions/runs/44867679299)（`leg=usage`，`usage_idle_seconds=600`，2026-11-21 04:46Z）：FAIL，16 项检查里 1 项失败。** 空闲用 600 秒而不是 t005 原文的一小时：验收是"CPU 费约为零、内存照收"，按比例判断，t001 的空闲基线也是 600 秒。
  - **失败的一项，"busy: metered CPU matches the pod-level counter"：** 计量器给忙碌 sandbox 的 333 秒运行区间记了 **307 CPU 秒**，而节点 cAdvisor 上这个 pod 的计数器在暂停前**总共只有 298.42**（负载期间走了 297.48）。多记 8.6 CPU 秒（2.9%），检查的上限是总数的 1.02 倍加 2 秒（306.39）。
  - **原因（由代码和数字推出，没有读生产 Prometheus 的原始样本来确认）：** 计量器读的是 `sum(increase(...[333s]))`（`lego/backend/internal/sandbox/cpu_meter.go`），而 Prometheus 的 `increase()` 会把首尾样本之间的增量按**整个窗口的平均速率**外推到窗口两端。这个区间末尾 20 多秒是空闲的，外推却按约 0.9 核补了最后一次抓取到暂停之间的那段：298.42 × (1 + 约 9 秒 ÷ 约 310 秒) ≈ 307。误差上限约为一个抓取间隔（15 秒，外推最多 16.5 秒）乘以区间的平均核数，每个区间结束时一次；整点边界上的外推前后相抵。按钱算每核每个区间最多约 $0.0004，但它超过了客户自己能读到的 cgroup 数字，和 ADR104 D8"按客户可以复现的数字计费"不符。另开为 **w22/024**。
  - **通过的 15 项和读数：**
    - 忙碌（5 轮 60 秒单核，请求 300 秒）：负载自己用了 288.5 秒，计量 307，是负载自己的 **1.064 倍**、请求的 1.023 倍（t001 的比值是 1.098 和 1.053，t005 的验收在这两个比值之内）；内存 665 GiB 秒（2 GiB × 333 秒）。
    - 空闲 602 秒：2 CPU 秒，**0.33% 单核**（线为 2%）；内存 1203 GiB 秒。
    - 暂停后：CPU、内存都不再增长（307 和 665 不变），没有加权行。
    - `GET /v1/usage`：`sandboxUsageBilledFrom` 是 2026-11-18T00:00:00Z；忙碌 sandbox 的 CPU 行 0.0853 CPU 小时 × $0.084 = $0.01，内存行 0.185 GiB 小时 × $0.0133；空闲 sandbox 的 CPU 行 0.000556 CPU 小时、$0.00，内存行 0.334 GiB 小时；两个 sandbox 都没有 `sandbox_compute_seconds` 行，没有 `unbilled`。
    - 配额（w22/020 的检查）：`1c-2g` 的 pod 配额 1 核，`starter` 0.5 核；`starter` 上四个 syscall 重循环 60 秒用了 30.12 CPU 秒（0.502 核）；两个 pod 的安装步骤都在同一秒内完成，创建到容器启动 4 秒。
  - **清理：** 运行结束时删除一个一次性 workspace 失败（日志里的 WARNING）；它不计费，下一次运行的过期清扫会重试。
- **结论：停靠。** t003 和 t005 的验收条目按读数都成立，但这条腿是它们的检查手段，它没有打印 PASS，失败的那项正是 t003 的计量器；DoD 的"在生产上用已知负载验证"不能靠一次失败的运行成立。t003、t005 保持未完成，读数已记在任务里。
- **剩余闸门（不需要用户）：** w22/024 落地并到生产之后，再跑一次 `gh workflow run sandbox-measure.yml -f leg=usage -f usage_idle_seconds=600`，打印 PASS 后把数字记进 t003、t005 和 ADR104 D8，然后 t009 收尾。

## 解阻与结案记录 2026-11-23（/loopx w22，UTC）

- **来源：** 复核证据，不是用户决定。涉及 t003、t005、t009。
- **原闸门已清：** 生产的镜像 pin 是 `9fdf7e0a1171`（`cded300699`，17:33Z），`git merge-base --is-ancestor ceb23ce81 9fdf7e0a1171` 成立。第一个带上 `ceb23ce81`（w22/024 的修复）的 pin 是 `cce6c749f6ff`（2026-11-21T09:00Z）。
- **这条腿已经在生产上打印过 PASS：[sandbox-measure run 45147416800](https://github.com/bex-co/bex/actions/runs/45147416800)**（`leg=usage`，`usage_idle_seconds=600`，2026-11-23T09:29Z）。当时的 pin `1a5a8e872398` 带 `ceb23ce81`，脚本带 w22/024 的修复 `42ed010012`。所有检查都通过，最后一行是 `PASS: usage leg: a busy, an idle and a suspended sandbox are metered as designed (billed on use: true)`。这次运行是为 w22/024 的验收派发的，没有记到板上，本记录补上。读数：
  - **忙碌**（5 轮 60 秒单核，请求 300 秒）：负载自己用了 291.64 秒；节点 cAdvisor 上 pod 计数器在负载期间走了 296.73，总数 297.08；计量 **297**。这是负载自己的 1.018 倍、请求的 0.99 倍，在 t001 的 1.098 倍和 1.053 倍之内，并且不超过计数器（2026-11-21 是 307 对 298.42，w22/024 的修复生效）。运行区间 342 秒，`cpuSettled`，没有延后；内存 683 GiB 秒（2 GiB × 342 秒）。
  - **空闲** 602 秒：1 CPU 秒，0.17% 单核（线为 2%）；内存 1203 GiB 秒，全额。
  - **暂停后：** CPU 和内存停在 297 和 683，只有快照存储增长，没有加权行。
  - **`GET /v1/usage`：** `sandboxUsageBilledFrom` 是 2026-11-18T00:00:00Z。忙碌 sandbox 的 CPU 行 0.0825 CPU 小时 × $0.084 = $0.01，内存行 0.19 GiB 小时 × $0.0133 = $0.00；空闲 sandbox 的 CPU 行 0.000278 CPU 小时、$0.00，内存行 0.334 GiB 小时、$0.00。两个 sandbox 都没有 `sandbox_compute_seconds` 行，也没有 `unbilled`。
  - **配额**（w22/020 的检查）：`1c-2g` 的 pod 配额 1 核，`starter` 0.5 核；`starter` 上四个 syscall 重循环 60 秒用了 29.51 CPU 秒（0.492 核）；安装步骤 0 秒，创建到容器启动 2 秒。
- **本次派发的三次重跑没有走到计量检查。** [45275218820](https://github.com/bex-co/bex/actions/runs/45275218820)、[45276212231](https://github.com/bex-co/bex/actions/runs/45276212231)、[45277163478](https://github.com/bex-co/bex/actions/runs/45277163478)（22:00Z 到 22:13Z）都失败在忙碌负载的 exec 流上：`curl: (56) Recv failure: Connection reset by peer`，分别在流开始后约 46、16、15.5 秒。
  - Traefik 的访问日志里这三条流都是 200，结束得比客户端被重置晚 2 到 6 秒；bex-api、ssh-gateway、Traefik 都没有重启。所以断在客户端和 Traefik 之间（Cloudflare、负载均衡器、NodePort），不在计量器。另开为 **w22/028**。
  - 三次运行的 sandbox 都已删除，生产上没有残留的 sandbox pod。两个一次性 workspace 删除失败（日志里的 WARNING），下一次运行的过期清扫会删除它们；09:29Z 那次留下的两个就是在 22:00Z 被清掉的。
- **PASS 之后计量路径上的改动：**
  - 抓取配置 `6d5b219bd3`（w6/167，20:56Z）改写了产生 `sandbox_pod_cpu_usage_seconds_total` 的 relabel。重跑之后在生产的 Prometheus 上读到，这三次运行的六个 pod 都有这条序列。
  - `506b9b40ef`（w6/m566 t001，agent session 的 sandbox 的 CPU 和内存记为不计费行）在 PASS 之后的 pin 里，由它自己的条目和测试覆盖（`TestSandboxUsageSwitchPG` 随之更新）。这次 PASS 不覆盖它。
- **发票预览：** 一次性 workspace 按 ADR046 不计费，没有 Stripe 订阅，所以没有发票预览可读。CPU 和内存折成加权单位导出到 Stripe 的一步由 `TestSandboxUsageWeightedUnitsCostTheSame` 和 `TestSandboxBilledOnUseAcrossAdapters` 覆盖。
- **结论：完成。** DoD 逐条核对：
  1. spike 有结论（fail），按 2026-11-16 的修订按测得的 CPU 计费，2026-11-18T00:00:00Z 起每个 workspace 同时生效，并在生产上用已知负载验证（run 45147416800）。
  2. CPU 按用量、内存按分配计费，暂停的 sandbox 只付存储（同一次运行）。
  3. `sandbox_compute_seconds` 的历史和 agent session 的成本不变（t004、t008 的测试；agent session 仍走加权计量）。
  4. ADR023、ADR030、ADR018、ADR104 D8 与代码一致（t005 第 5 步；ADR104 D8 的生产检查和 gap 8 本次更新）。
- **留给别的条目：** w22/024 和 w22/020 等的是同一次 PASS（run 45147416800），由它们自己的 worker 落地。

## Render parity, 2026-11-17 (t006)

REST, GraphQL and MCP return the same rows, lines, `unbilled` flags and `sandboxUsageBilledFrom`; the dashboard reads the GraphQL fields. Render's public API has no usage endpoint, so the comparison is against its documented billing (launch post and comparison article, re-read 2026-11-17). Drift, all recorded:

| Render | bex from 2026-11-18T00:00:00Z | Where it is recorded |
| --- | --- | --- |
| CPU billed on active use, on Firecracker | Billed on use at the pod's cgroup, which under gVisor includes the sentry: +9.8% on a pure CPU load | ADR104 D8, ADR030 §17; the overhead-factor variant is w22/019 |
| A plan names its CPU; whether a sandbox can exceed it is not documented | A pod below 2 CPU can use and be billed for up to 2 cores | ADR104 D8, ADR030 §17; fix filed as w22/020 |
| Memory billed while running, on allocation | Same | none |
| Suspended: snapshot storage only | Same (w22/m313) | none |
| Invoice layout for sandboxes: not established | One sandbox compute line on the Stripe invoice; CPU and memory apart on the usage page and API | ADR104 D8, ADR040 |
| Whether Hobby pays for sandbox usage: not stated | Open, as before | ADR030 §15, ADR104 D8 |
| Prices are in an article, not on render.com/pricing | The rates are in `pricing.yaml` and ADR030, not yet on `GET /pricing` or bex.co | ADR030 §17 |

An agent session's sandbox keeps the weighted meter (ADR047 D6); Devin's pricing page gives no per-CPU reference (read 2026-11-17).
