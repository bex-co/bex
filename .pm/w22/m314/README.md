# w22 · m314 — Sandbox CPU billed on use, as on Render: a gVisor accuracy spike, then the cutover

**Worker:** worker22 **Goal:** a running sandbox is billed for the CPU it uses plus the memory allocated, and the billing switches in one step, or the fallback is recorded for the user's sign-off **Status:** todo (t001 done: verdict **fail** on the ±5% line, recorded unchanged; unblocked 2026-11-16 by `/unblock w22` under the standing parity rule: bill the measured CPU, see "解阻记录 2026-11-16")

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Spike: does gVisor report usable per-sandbox CPU? Fix the thresholds, run, record the verdict — **DONE (verdict: fail)** | 1h30m | — |
| t002 | Record the pod name on every running interval | 1h | t001 |
| t003 | Meter used CPU from Prometheus: sandbox_cpu_seconds | 2h | t001, t002, w22/m313/t002 |
| t004 | Bill on use: price CPU and memory, stop billing the folded meter (or take the recorded fallback) | 1h30m | t003, w22/m313/t005 |
| t005 | Production check with a known load, and amend ADR023, ADR030 and ADR104 | 1h30m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- The CPU spike has a recorded verdict. Pass: used-CPU billing is live for every workspace from one dated timestamp and verified on production with a known load. Fail: the allocated-CPU charge is recorded in ADR018 as a divergence with the user's sign-off and a dated removal task. Escalate: the milestone is parked with the exact question for the user
  - **Amended 2026-11-16 (existing rule, not a user decision; see "解阻记录 2026-11-16"):** t001's verdict stays "fail" and the allocated-CPU fallback is not taken. The fallback was written for a meter gVisor makes unusable; this one is usable and its error is measured (+5.3% over the requested seconds, +9.8% over the load's own count, idle 0.45% of a core). ADR104 D8 already says to bill the measured cgroup CPU with no subtraction, and the parity rule forbids parking on an optional variant, so the cutover bills the measured CPU. A subtraction variant is a separate follow-up; the allocated-CPU charge would be a divergence that stands only with the user's sign-off in ADR018.
- CPU is billed on use and memory on allocation while running; suspended sandboxes pay storage only (with w22/m313)
- `sandbox_compute_seconds` history and the agent-session cost are unchanged
- ADR023, ADR030, ADR018 and ADR104 D8 agree with the code

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gap 8 (CPU meter and cutover)
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

