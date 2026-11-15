# w22 · m312 — Docker and Compose in a sandbox: a measured spike on a throwaway gVisor node

**Worker:** worker22 **Goal:** ADR104 D6's unverified claims about Docker in a gVisor sandbox are measured, with a recommended option for the user to sign off, and no customer-facing runtime setting changed **Status:** blocked (t003 and t006 done and shipped; the rest needs a throwaway Kubernetes node with Cilium: t001/t002/t004 are half done on a local VM, t005 and t007 are not started)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Find or provision a throwaway gVisor node that is not the production sandbox pool | 45m | — |
| t002 | Configure the throwaway runtime for Docker: net-raw flags and a second RuntimeClass | 30m | t001 |
| t003 | Functional matrix: hello-world, build, and a two-service Compose file — **DONE** | 1h | t002 |
| t004 | Pod Security and sandbox pod-template check | 30m | t001 |
| t005 | Isolation tests with net-raw on: spoofing, raw packets and the DNS/SNI allow-list bypass | 1h30m | t002 |
| t006 | Make the checks repeatable: scripts/sandbox-docker-spike.sh — **DONE** | 45m | t003, t005 |
| t007 | Record findings, recommend one of D6's four options, tear down | 45m | t004, t006 |
| t008 | Simplify | 20m | t007 |
| t009 | Test coverage | 40m | t007 |
| t010 | Closeout | 10m | t008, t009 |

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
- **我的建议：** 继续停靠，等这个功能的优先级上来再做。已经完成的本地 VM 结果（ADR104 D6）保留。
