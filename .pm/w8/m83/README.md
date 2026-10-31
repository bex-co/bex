# w8 · m83 — Supply automatic compression for web responses

**Worker:** worker8 **Goal:** eligible ordinary web responses transfer compressed with exact decoded content and correct neighboring HTTP behavior. **Status:** in progress — t001–t005 done 2026-10-31 (gzip-only Traefik `compress`, ADR018 signed differences); t006 live closeout waits for the operator deploy and a Free web-service window in bex-canary (Free hours reset 2026-11-01 00:00 UTC)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Wire compatible automatic web compression — **DONE** | 50m | — |
| [t002](done/t002.md) | Audit shared route callers and HTTP neighbors — **DONE** | 25m | t001 |
| [t003](done/t003.md) | Render parity — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 10m | t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 35m | t003 |
| [t006](t006.md) | Closeout | 20m | t004, t005 |

Six tasks,160m total.

## Definition of done

- Fresh permitted Free web services through both native clients reach first-deployment Live with exact source/artifacts, then eligible responses negotiate smaller Brotli/gzip over HTTP1/2 and decode exactly without tenant changes.
- Current identity, authored gzip, no-transform and separate finite SSE events keep working; meaningful negotiation/framing/validator/range and other shared routing neighbors are verified on the actual reviewed proxy.
- Maintained projection and actual-proxy tests fail before/pass after; shared routing/ownership/cost bounds and existing public-surface settings are audited. Required checks and explicit live/source limits are recorded.
- Owned services/children/metadata are absent, original baselines remain and quotas terminate at0. Tasks/milestone move to done through /pm with synchronized statuses and no stubs; static m79 retains its independently accepted scope.

## Source + Goal linkage

- **Source:** [functional CLI sweep352](evidence/cli-sweep352/README.md), [edge diagnosis](finding.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 web-service compatibility and the current shared ingress architecture.
- **Expected outcome:** compressible web responses transfer fewer bytes while exact content and streaming behavior are preserved.
- **Why now:** both native services are healthy, all16 payload checks pass, but10 normal encoding offers stay at full size; authored gzip proves the transfer saving and source projection lacks a compressor. No matching repair is on fresh main.
- **Render parity included:** Render documents load-balancer Brotli/gzip for app responses. Identity is legal HTTP; this is a minor hosting/performance gap, distinct from completed static-handler m79.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m83。
- **结论来源：** 当前固定 Traefik 3.7.5 配置与既有真实代理测量复核。裸 compress 违反 no-transform / ETag，Brotli 大 Range 也有错误；现有 guard 的 Yaegi wrapper 又会缓冲 streaming。
- **处理：** 不能为了移出 blocked 删除这些未达成的验收，或让用户替 worker 选择破坏 HTTP 邻居。优先在保持原 DoD 的前提下修复/升级供应方；worker 先核实最新官方修复是否可用，再准备具体最小变更。本批 D2 的 19 份 render-oss/cli 补丁不包括本项尚未准备的 Traefik/Yaegi PR。
- **剩余门槛：** 若仍需对外提交，具体补丁可评审后另取授权；若仅已有官方修复可用，按真实代理回归自行恢复。未证实可用修复，本项继续 blocked。


## 解除阻塞记录（2026-10-31，`/unblock w8`）

- **涉及 ID：** w8/m83
- **结论来源：** 用户明确决定（`/unblock w8` B，2026-10-31）。
- **结论：** 选项 1：现在就上线 Traefik 自带的 `compress`（流式安全）。用户签字接受两处与 Render 的差异，由 worker 记入 ADR018：带 `Cache-Control: no-transform` 的响应也会被压缩；强 ETag 压缩后保持为强 ETag。
- **约束：** 不能引入**错误**行为。如果启用 `br` 会让大的 Range（206）响应编码错误，就只用 gzip，或把 Range 响应排除在压缩之外，直到证明正确。保留 allowlist，去掉 `compressguard`。
- **后续：** 上游修复（traefik/yaegi 的 Flusher 包装，或 compress 增加 no-transform/ETag 选项）出来后可以消除这两处差异，worker 另立 follow-up。
- **用户门槛：** 无。
- **worker 下一步：** 从 t001 起实施（基于 [attempt](evidence/edge-compression-attempt/README.md) 的补丁），覆盖 t002 的共享路由和 HTTP 邻居审计；验收：可压缩响应解码后内容与原文逐字节一致，流式响应不被缓冲，Range 响应正确；记录 ADR018 的签字差异。
