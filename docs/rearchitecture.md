# 重构提案：把 Hetzner 生产基座一次做对（rearchitecture）

**状态：** proposed（2026-07-10） **范围：** Hetzner 生产环境的集群拓扑、网络、控制面、管理面、安全边界。不涉及产品 API 层（bex-api / dashboard / operator 的业务逻辑不变）。 **结论先行：** 今天生产上的所有基座级故障——CI 持续红、控制面不可管理、CSR 拒签循环、OpenBao 挂起 28 小时——都能追溯到同一个根因：**Hetzner 私有网络是绕过 CAPH 带外（out-of-band）挂上去的，而 CAPH 的网络字段不可变更（immutable），导致声明层与现实永久分叉。** 修补无解（字段改不了），正确做法是按目标架构一次性重建集群。

---

## 1. 现状拓扑（2026-07-10 实测）

```
┌─ Hetzner 项目 ──────────────────────────────────────────────────┐
│                                                                  │
│  bex-infra 网络 (Terraform 所有, 10.0.0.0/16, 子网 10.0.1.0/24)  │
│  ├── bex-infra 服务器：单节点 k3s = CAPI 管理集群（宠物、单点）   │
│  └── bex-control-plane-dw8vr：app 集群唯一控制面 ← 带外挂入！     │
│                                                                  │
│  app 集群 "bex"（CAPH 管理）                                     │
│  ├── bex-control-plane-dw8vr  (cx33, 未打污点, 平台组件全在此)    │
│  ├── bex-worker-0-p26nk-wjlj6 (cx33, m3 autoscaler 今日拉起,      │
│  │                             **未入私网**, 仅公网 IP)           │
│  ├── LB bex-kube-apiserver：公网 target                          │
│  └── LB bex-traefik：**私网 target (use_private_ip: true)**       │
│      —— 全部生产流量（dashboard、租户 app）走 LB→10.0.1.2 私网    │
└──────────────────────────────────────────────────────────────────┘
```

关键事实（均为当日 API/kubectl 实测，非推断）：

- 线上 `HetznerCluster.spec.hcloudNetwork` = `{"enabled": false, "cidrBlock": "10.0.0.0/16", "subnetCidrBlock": "10.0.0.0/24"}` —— **CAPH 从未被告知任何网络**。
- Hetzner 项目里唯一的网络是 Terraform 的 `bex-infra`（子网 `10.0.1.0/24`），挂着 2 台服务器：管理节点 `bex-infra` 和 app 控制面 `dw8vr`。overlay 里声明的 `10.0.0.0/24` 与现实的 `10.0.1.0/24` 甚至不一致。
- app 集群的 Hetzner CCM 配置为 `HCLOUD_NETWORK=bex` + `networking.enabled=true`——**名为 `bex` 的网络并不存在**。`bex` 恰好是 `hcloudNetwork.enabled: true` 时 CAPH 会自动创建的网络名，这暴露了 m7 的原始设计意图（CAPH 拥有网络），只是被 immutable 字段挡在半路，随后有人把控制面服务器手工挂到了 `bex-infra` 网络上作为变通。
- 控制面节点 InternalIP = `10.0.1.2`（私网地址）。

---

## 2. 诊断：一个根因，五组症状

### 根因：私网带外挂载 + CAPH 网络字段不可变

CAPH（v1.1.7）的模型是：**要么它从创建之日起拥有网络（创建、挂机器、校验地址、配 LB），要么完全没有网络**。`spec.hcloudNetwork` 有 webhook 强制 immutable，集群建成后永远改不了。m7 t005 想给 LB 上私网 target，正确路径（CAPH 拥有网络）被 immutable 挡住，于是采取了带外方案：手工把控制面服务器挂进 Terraform 的 `bex-infra` 网络，并把 CCM 切到 networking 模式。从那一刻起，**CAPH 眼中的世界和真实世界永久分叉**，以下全部症状由此派生。

### 症状 A：kubelet CSR 拒签循环（安全凭据在腐烂）

```
Warning  CSRValidationFailed  failed to validate kubelet csr:
         the IP address "10.0.1.2" is not allowed
```

**（2026-07-10 晚已兑现：CP 节点 kubelet serving 证书耗尽，`kubectl exec/logs/port-forward` 到该节点上的一切 pod 返回 `tls: internal error`——t001 的备份作业被迫改走集群内 Job + termination-log。）**

机制：kubelet 开启了 `rotate-server-certificates: "true"`，serving 证书通过 CSR 轮换；CAPH 内置 CSR 审批器只放行它**自己知道的**机器地址。`10.0.1.2` 是带外挂上去的，CAPH 不认识，于是每一张含私网 IP 的 serving-cert CSR 都被拒。更糟的是本 overlay 把 `cluster-signing-duration` 设为 **6h0m0s**——serving 证书每 6 小时就要续一次，续不上。事发时 `kubectl logs/exec` 尚可用（最后一张被批准的证书还覆盖公网地址），但这是在到期倒计时里运行：一旦彻底过期，针对该节点的 logs/exec/metrics（TLS 到 kubelet `:10250`）全部失效。

### 症状 B：KCP 无法管理控制面（生产没有升级/自愈/换机能力）

```
EtcdClusterHealthy=Unknown  Failed to connect to etcd:
                            failed to get etcd status: context deadline exceeded
RollingOut=True             Rolling out 1 not up-to-date replicas
```

机制：Cluster API 的 KubeadmControlPlane 控制器在做任何控制面变更前有 preflight 检查（etcd 成员健康）。检查需要从**管理集群**触达工作集群控制面的 etcd；控制面节点的地址如今是 `10.0.1.2`，虽然管理节点恰好也挂在同一张网（同为带外操作），但主机层路由/回程未配通（热挂载的网卡不会自动配置），结果是 `context deadline exceeded`。preflight 永远不过 → **一切 KCP 操作被无限期排队**：w1/m3 的调度器配置 rollout 停在 `RollingOut=True`、机器故障后的 remediation 不会执行、未来的 Kubernetes 版本升级无从谈起。生产集群目前处于"能跑但不可运维"状态。

### 症状 C：CI（app-cluster.yml）自 2026-07-09 起持续红

```
The HetznerCluster "bex" is invalid: spec.hcloudNetwork: Invalid value:
{"enabled":true, ...}: field is immutable
```

机制：m7 把 `hcloudNetwork.enabled: true` 写进了 overlay，但线上对象是 `enabled: false` 且不可改。`kubectl apply -f cluster.yaml` 应用到 HetznerCluster 这一篇时被 webhook 拒绝，step 失败，**其后的所有步骤（等控制面、装 autoscaler、app 集群 addons）全部不执行**。2026-07-10 m3 上线时，正是这个失败导致 autoscaler 安装步骤被跳过、只能手工执行。只要 overlay 和线上不一致，这条产线就永远是红的——任何后续 infra 变更都无法通过 CI 送达生产。

### 症状 D：保险栓效应——"修好管理面"会直接引发断流事故

这是当前状态最阴险的性质。全部生产流量走 `bex-traefik` LB 的**唯一私网 target**（`use_private_ip: true` → `10.0.1.2`）。假设有人"修好"了症状 B（比如配通管理节点到 `10.0.1.0/24` 的路由）：

1. KCP preflight 通过，排队中的 rollout 立即执行——**替换控制面机器**；
2. CAPH 创建的新机器**不会**挂进任何网络（CAPH：`enabled: false`）；
3. CCM 无法为一台不在网络里的机器配置私网 LB target；
4. `bex-traefik` 失去唯一 target → **生产断流**。

也就是说：KCP 的阻塞目前反而是防止事故的保险栓；"恢复可管理"与"保住生产流量路径"在现有架构内互斥。这就是为什么修补路线（无论先修哪个症状）都是死路——**每个症状的"修复"都会引爆另一个症状**。

### 症状 E：控制面兼职工作节点（容量与故障域合一）

独立于网络问题、但同样是"临时决定固化成架构"的产物：

- overlay 里 `taints: []` 摘掉了 control-plane 污点（"single-node cluster: … Scale up later"），全部平台组件（Traefik、Kratos/Hydra/OpenFGA、OpenBao、CNPG×3、Prometheus、Loki、bex-api、dashboard、Argo）挤在这台 cx33（4 vCPU/8G）上；
- 实测 CPU requests 达 **96%**——`openbao-1`、`openbao-2`（HA 副本）和一个 `cilium-operator` 副本**挂起了约 28 小时**无处调度（2026-07-10 m3 的 autoscaler 上线后才落地，顺带证明了容量枯竭早已发生）；
- etcd 单成员，无 quorum；数据安全仅靠每日快照（docs/etcd-backup-restore.md）。控制面机器一坏，恢复是"从快照重建"级别的事故，不是"failover"。

### 附带发现（独立问题，一并记录）

- **autoscaler 版本偏斜的静默失败**（已修，教训保留）：chart 默认镜像 CA 1.35 对 v1.31 apiserver 启动 DRA informer（`resource.k8s.io/v1`，v1.31 不提供），informer cache 永远同步不完成，**主循环静默地从不启动**——无报错、无扩容评估，只有 reflector 重试日志。已通过 `CA_TAG=v1.31.5`（安装脚本参数 + CI 固定）修复；规则：CA 镜像跟随工作集群的 minor 版本。
- **平台 CNPG 集群没有持续备份**：m17（2026-07-09 完成）给**租户** `Database` 接上了 barmanObjectStore（每日基础备份 + WAL，保留 30 天，见 `database_controller.go`），但平台侧的 `bex-db`/`kratos-db`/`hydra-db`（deploy/gitops 定义）仍无对象存储备份。重建前必须对全部库手工 `pg_dump`（租户库虽有 barman，dump 仍作兜底）；给平台集群补持续备份是本次重建后的直接跟进项。
- **OpenBao 生产从未初始化**（t001 执行时发现，2026-07-10）：三个 pod 全部 `sys/health=501 (uninitialized)`，PVC 仅 41 小时——m10 的"prod activation（首次 `bao-init.sh` + live PUT）"runbook 从未执行，**租户 env-vars API 在 prod 从未工作过**。重建流程随之简化：无 OpenBao 快照可恢复，t006 直接在新集群上首跑 `bao-init.sh`（终于完成 m10 的激活）。
- **备份链已静默断裂 ≥3 天**（同日发现）：`etcd-backup-s3`/`openbao-backup-s3` secret 从未在本集群创建（runbook 带外步骤被遗漏），etcd 备份连续失败 3 天无人知晓。已按 runbook 补建 secret，etcd 快照恢复产出。教训并入第 6 节：带外步骤必然被遗忘，重建后的备份凭据应进 SealedSecret 走 git。
- 与本提案无关但当日观察到：新 ReplicaSet 的 bex-api 因 `duplicate migration file: 0004_usage.down.sql` 崩溃循环（迁移编号冲突，属另一工作流，已在后续提交修复）；两个租户 app `ImagePullBackOff` 9 小时。

---

## 3. 目标架构：四个决定

### 决定一：网络归 CAPH 所有，全声明式

`hcloudNetwork.enabled: true` 从第一天写进 HetznerCluster——CAPH 创建并拥有名为 `bex` 的网络（CIDR 取 **`10.10.0.0/16`**，与 `bex-infra` 的 `10.0.0.0/16` 明确区分，杜绝再次混用）。此后：

- 每台机器（控制面、worker）创建即入网，地址是 CAPH 已知的 → CSR 校验天然通过；
- CCM 的 `HCLOUD_NETWORK=bex` 指向真实网络（现有配置终于自洽）；
- 两个 LB 都由声明生成私网 target：kube-apiserver LB（CAPH 管）与 Traefik LB（CCM 依 Service 注解 `load-balancer.hetzner.cloud/use-private-ip: "true"` 管）；
- 带外操作清零：`kubectl apply` overlay 永远与现实一致，CI 恢复为唯一变更通道。

### 决定二：控制面/工作负载彻底分离

- **KCP `replicas: 3`**（etcd quorum），**恢复 control-plane NoSchedule 污点**；m3 的 `MostAllocated` 调度器配置原样保留在 KCP 声明里，新集群第一天生效。
- 两个 worker 池：
  - **`bex-platform`**（min 2，污点+标签 `bex.co/platform`）：平台组件（Traefik、Ory、OpenBao、CNPG、观测栈、bex 自身）经 toleration/nodeSelector 落在这里。OpenBao 三副本从此真正 HA。
  - **`bex-worker-0`**（租户池）：保留 m3 的 autoscaler min/max 注解与 scale-from-zero 容量提示，min 0——租户负载弹性伸缩，租户代码永不与平台凭据存储共享内核。

### 决定三：pivot 成自管理集群，销毁宠物管理节点

CAPI 官方 bootstrap-and-pivot 模式：新集群建成后在其上 `clusterctl init --infrastructure hetzner`，再 `clusterctl move` 把 Cluster/Machine 对象搬入——**app 集群自己管理自己的机器**。收益：

- 管理面与工作面同网，"跨集群 L3 不通导致 KCP 失明"这一类问题（w1/014）在结构上不再存在；
- cluster-autoscaler 切到 `incluster-incluster` 模式（不再需要 kubeconfig secret），并且可以回归 Argo Application 管理——`deploy/gitops/base/` 里 cluster-api 与 autoscaler 占位文件当初写的正是这个终态（此前"autoscaler 不进 Argo"的决定记录以"外部管理集群"为前提，pivot 后作废）；
- `bex-infra` k3s 单点在 pivot 后 `terraform destroy` 销毁。Terraform 定义保留；灾难恢复 runbook = "重建 bootstrap 节点 → 从 etcd 快照恢复"，而非供养一台常驻宠物。

### 决定四：安全边界一次收紧到位

东西向流量全部进入私网后：

- **Hetzner Cloud Firewall**（按 label selector 应用到 CAPH 机器，新机器自动继承；限制的是端口而非来源 IP，不触碰 DO_NOT_DO 对静态源地址白名单的否决）：公网入站只留 **80/443**（Traefik LB）、**6443**（kube-api LB 前端，kube TLS/RBAC 把关）、**22**（key-only，既有基线）。**无认证的 VXLAN `:8472/UDP`、kubelet `:10250`、etcd `:2379`、NodePort 段从公网消失**——这是今天最实际的暴露面（VXLAN 报文注入可绕过 NetworkPolicy 直达 pod 网络）。
- **Cilium WireGuard 加密**（addons 一个 values 开关）：跨节点 pod 流量加密、隧道端点互认。私网解决"走哪"，WireGuard 解决"看得见也读不懂"。
- CSR 审批链恢复健康后，`cluster-signing-duration: 6h` 从雷变回合理的短周期轮换。

---

## 4. 一次性重建顺序（接受停机）

1. **备份先行**：etcd 快照、OpenBao Raft 快照（两份 runbook 已有）、`pg_dump --no-owner` 全部 CNPG 库（bex-db、kratos-db、hydra-db、租户库）→ 对象存储。**另加两样容易漏的**（2026-07-10 拉取 review 时发现）：`kubectl get apps,databases,keyvalues.app.bex.co -A -o yaml` 的 CR 快照——控制面 store 只投影 apps，**Database/KeyValue 的意图只存在于 etcd**，不备则重建后全部租户数据存储静默消失；以及每个 Valkey 实例的 RDB（`BGSAVE` + 拷出 `dump.rdb`）。做对事情包含不弄丢数据。
2. **改声明**（一个 PR）：重写 `infra/clusterapi/overlays/hetzner-caph/cluster.yaml`（决定一、二）；`app-cluster.yml` addons 加 Cilium WireGuard 开关；`infra/clusterapi/autoscaler-values.yaml` 切 `incluster-incluster`；Terraform 增加防火墙资源。Kubernetes 版本本次**保持 v1.31**（少一个变量；版本升级恰恰是重建后 KCP 恢复可用的第一个受益项，作为后续独立变更）。
3. **拆**：`kubectl delete cluster bex`（CAPH 回收机器与 kube-api LB）；CCM 创建的 `bex-traefik` LB 用 hcloud API 清理。
4. **建**：跑 `app-cluster.yml`（此后它必须全绿）→ CAPH 建网/建 LB/3×CP/平台池 → addons → `deploy.yml` 装 Argo → **平台整体从 git 收敛**（GitOps 投资的兑现时刻）→ 恢复 OpenBao 快照与 CNPG dump。
5. **Pivot**（决定三）：`clusterctl init` + `clusterctl move` → 装 in-cluster autoscaler（Argo App，镜像版本跟集群 minor）→ `terraform destroy` 销毁 `bex-infra` 节点。
6. **外部指针**：DNS（`dashboard.bex.co`、`*.onbex.co`）指向新 Traefik LB IP；CI secrets 中的 kubeconfig 更新。
7. **验收**：`scripts/verify-elastic.sh`（此时 `MGMT_CTX` 即集群自身）三段全过；`app-cluster.yml` 端到端绿；prod kube-scheduler 带 `--config=/etc/kubernetes/scheduler-config.yaml`。

## 5. 本次重建关闭与解锁的事项

| 类别 | 事项 |
| --- | --- |
| 关闭 | `w1/014`（KCP 失明 + CSR 循环 + immutable 僵局）——根因消除，非症状修补 |
| 关闭 | `w1/m3` t008（调度器配置随新 KCP 落地，DoD 三条全部在 prod 成立） |
| 关闭 | m7 t005 的原始意图（LB 私网 target 成为受支持形态，而非带外变通） |
| 关闭 | OpenBao HA 副本挂起（平台池有容量）、CSR/6h 证书腐烂、控制面单 etcd |
| 解锁 | `w1/008` 每服务自动扩缩（m3 之上的 Render parity 缺口） |
| 解锁 | `w1/013` Postgres HA（需要多节点）、k8s 版本升级（需要可用的 KCP） |
| 触发 | `w1/m17` 数据保护提级——本次的手工 `pg_dump` 就是它的 "why now" |

## 6. 一条教训（写给未来的变更）

m7 的失败模式不是"目标错了"（LB 私网 target 是对的），而是**在不可变字段面前选择了带外变通**：声明层与现实一分叉，每一个后续症状的"修复"都会引爆另一个症状，最终整个系统停在"不可管理但不敢修"的死锁里。规则化表述：**基础设施只接受经由声明（git → CI → controller）的变更；当声明层拒绝你（immutable、validation）时，答案是重建出正确的声明，绝不是绕过它去改现实。**
