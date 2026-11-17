# w22 · m312 — Docker and Compose in a sandbox: a measured spike on a throwaway gVisor node

**Worker:** worker22 **Goal:** ADR104 D6's unverified claims about Docker in a gVisor sandbox are measured, with a recommended option for the user to sign off, and no customer-facing runtime setting changed **Status:** done (2026-11-16: measured on a free local Kubernetes node with Cilium, never a bex node; every isolation attempt was blocked; option 1 recommended and the user asked to sign off; the node is deleted)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Find or provision a throwaway gVisor node that is not the production sandbox pool — **DONE** | 45m | — |
| t002 | Configure the throwaway runtime for Docker: net-raw flags and a second RuntimeClass — **DONE** | 30m | t001 |
| t003 | Functional matrix: hello-world, build, and a two-service Compose file — **DONE** | 1h | t002 |
| t004 | Pod Security and sandbox pod-template check — **DONE** | 30m | t001 |
| t005 | Isolation tests with net-raw on: spoofing, raw packets and the DNS/SNI allow-list bypass — **DONE** | 1h30m | t002 |
| t006 | Make the checks repeatable: scripts/sandbox-docker-spike.sh — **DONE** | 45m | t003, t005 |
| t007 | Record findings, recommend one of D6's four options, tear down — **DONE** | 45m | t004, t006 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 40m | t007 |
| t010 | Closeout — **DONE** | 10m | t008, t009 |

## Definition of done

- ADR104 D6 carries measured results for hello-world, `docker build`, two-service Compose with an internal call and a published port, the Pod Security check, and every isolation attempt (spoofing, raw packets, DNS/SNI bypass, platform floor), with runsc version, kernel and node image
- A recommendation among D6's options (1)-(4) and the user's sign-off request are recorded; no production sandbox-pool runtime setting was changed
- `scripts/sandbox-docker-spike.sh` re-runs the checks and refuses the production pool
- The throwaway node is torn down

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gap 7
- **Goal linkage:** Render parity first (Render documents Docker and Docker Compose in a sandbox); pillar 5 sandboxes (`.pm/DO_NOT_DO.md` #18 re-opened 2026-07-27)
- **Expected outcome:** ADR104 D6 stops saying "unverified": the user can choose among the four options on measured evidence, including whether the platform floor holds with `--net-raw` on
- **Why now:** Render states Docker support in plain words and bex's difference is user-visible; the choice also decides node-pool design (a second RuntimeClass, a Kata pool), so it needs evidence before anyone builds.
- **Render parity:** omitted. The spike adds no REST, GraphQL, MCP or UI surface and changes no behavior; it produces evidence and a recommendation.
- **Open decision, not made here:** which of D6's options 1-4 to build, and any divergence sign-off in ADR018. t007 records a recommendation; the user decides.
- **Safety:** the production sandbox pool's runsc configuration is never changed by this milestone.

## 解阻记录 2026-11-14（/unblock w22）

- **来源：** 复核证据；本次没有新的用户决定。
- **仍然卡着：** t005 的隔离测试和 t007 的最终方案建议需要一台一次性的、带 gVisor 和 Cilium 的 Kubernetes 节点。推断（未验证）：在生产 sandbox 节点上试需要放宽被测 sandbox 的权限（嵌套容器），会削弱你想保住的隔离边界，所以不建议借用生产节点。
- **你能怎么帮：** 决定是否值得为"sandbox 里跑 Docker/Compose"付一台按小时计费的一次性节点，并指定一个负责人和拆除日期。
- **我的建议：** 继续停靠，等这个功能的优先级上来再做。已经完成的本地 VM 结果（ADR104 D6）保留。（2026-11-16 被 D4 部分取代：见下。）

## 解阻记录 2026-11-16（/unblock w22）

- **来源：** 用户决定 **D4**（2026-11-16）：对"本机搭建 + 生产探针"的推荐项回复 "ok, approve"。提问里这一项写的是"让 m312 先试免费的本地 gVisor+Cilium 节点"；完整的本机范围和复核证据记在 [m413 的同名记录](../../m413/README.md)。
- **批准的范围：**
  1. 先在本机试一台一次性的 Kubernetes 节点：一个单独的、用完即删的 colima VM profile（像 2026-11-10 的 `gvisor-spike`：Apple VZ、Ubuntu、runsc），在里面起单节点 Kubernetes（k3s 或 kind，worker 选）加 Cilium 和 runsc，不碰 dev-N 栈用的默认 profile。
  2. 两个 VM 不同时跑：这台 Mac 32 GiB，dev-N 栈的 VM 占 16 GiB。先做 m402、m403、m413 的实机检查，停掉默认 profile，再做本项。
  3. 数据卷剩余磁盘低于 25 GiB 就停下（worker 的护栏）；结束后删掉这个 VM profile。
  4. `--net-raw` 和 `--allow-packet-socket-write` 只在这台一次性节点上开，**永远不碰生产 sandbox 池**（本里程碑的 Safety 条款不变）。
- **没有批准：** 任何付费资源。**这一点不变：** 一台 Hetzner `cpx32` 一次性节点（t001 的 2026-11-10 记录）需要用户另外指定负责人和拆除日期。
- **未验证（诚实说明）：** 本地的 Kubernetes 节点能不能跑 runsc 并复现 Cilium 的 DNS/SNI 出口策略，没有任何人试过。t005 的"DNS/SNI allow-list 绕过"这一项依赖 Cilium FQDN 策略，是最可能在本地做不成的一项。
- **再次停靠的条件：** 本地节点做不成（runsc 起不来，或 Cilium 策略无法复现），把精确的失败写下来，把本里程碑移回 `blocked/`，闸门写成"一台带 Cilium 的一次性付费节点，用户指定负责人和拆除日期"。已经做到的部分（t001、t002、t004 的本地结果）保留。
- **仍然是用户的决定：** t007 记录推荐的 D6 方案（1 到 4），用户之后选；这不挡本里程碑收尾。
- **worker 的下一步：** t001 步骤 1 的"本地"分支；然后 t002、t004、t005、t007 照原步骤。
- **验收：** t001 的验收标准：一台 `kubectl run` 用 gVisor RuntimeClass 能跑、且没有租户负载的节点，runsc 版本、内核、节点镜像写下来。

## Outcome 2026-11-16 (/loopx w22)

- **Node:** the local branch of decision D4 worked. One throwaway colima VM ran single-node k3s 1.34 with Cilium 1.19.5 and runsc as containerd runtime handlers, with bex's sandbox Cilium policies applied from the repository. It is deleted, with its data disk; the default colima profile was stopped meanwhile and is running again.
- **Results:** [ADR104 D6](../../../../docs/ADR104-sandbox-render-parity.md#d6--isolation-runtime-and-docker-in-a-sandbox), "D6 isolation results". Docker and Compose work in a pod; the platform floor and the allow-list held with `--net-raw` and with `--allow-packet-socket-write`; production's runsc pin does not run Docker 29; Pod Security and the pod admission policy each refuse a Docker pod.
- **Limits, stated in the ADR:** one node (no traffic between nodes), aarch64 and k3s, Cilium values taken from ADR023 with `socketLB.hostNamespaceOnly=true` added so a gVisor pod can reach a Service.
- **Waiting on the user, and not a gate for this milestone:** choose option 1 (recommended) or a signed divergence. Building option 1 is not filed.
