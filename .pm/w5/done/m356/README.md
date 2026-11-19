# w5 · m356 — In the zh dashboard, a deploy's port hints and a build's queued cause stay English, and the hint comes and goes between passes

**Worker:** worker5 **Goal:** the health-check stall's port hints, a queued build's cause and the deferred restart's wait read in the dashboard's language, and the stall text holds still while the deploy waits; REST and MCP keep Render's English byte for byte. **Status:** done — the live DoD replayed on production pin `4b2cebd0bd03` on 2026-11-19

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Types: templates for the port hints, the fallback-HTTP hint, the build's queued causes and the gRPC stall; a parser that reads an appended tail sentence by sentence — **DONE** | 60m | — |
| [t002](done/t002.md) | Operator: hints and queued causes format through the templates; the stall keeps the last port-scan verdict, so its text holds between passes — **DONE** | 45m | t001 |
| [t003](done/t003.md) | bex-api: GraphQL `DeployReason.parts` for a hint or cause that parses — **DONE** | 30m | t001 |
| [t004](done/t004.md) | Dashboard: render the parts, en and zh keys, `{wait}` as a localized duration — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Live verification in zh on owned services — **DONE** | 20m | t002, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 10m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 5m | t007, t008 |

Nine tasks, 260m (4h20m).

## Definition of done

- In `bex-canary`, with the dashboard in zh, a web image service whose server binds only 127.0.0.1 shows its stall on the deploy header entirely in Chinese, the port hint included, and three reads 5 s apart show the same text.
- A deploy whose build never started shows its queued cause in Chinese on the deploy header and the Deploys list (`build_never_started`'s cause). Amended 2026-11-12 after review: a deploy still queued carries no in-flight reason, because the backend's `stallDiagnosis` (`lego/backend/internal/store/reconciler.go`) does not surface `BuildQueued`; showing it while waiting would be a new surface, not this milestone's wording.
- A restart that waited for a deploy and timed out shows its wait as a Chinese duration, not "3h0m0s".
- REST `GET /v1/services/{id}/deploys` and MCP return the same English sentences as before; the writers' tests pin them byte for byte.
- An appended sentence no template words still shows, in English, after the worded ones.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for 残留问题 for w5` (2026-11-12), item 1, from the residuals w4/m336 recorded (its t007 Simplify and t008 Test coverage records, and the review of `b46178706`).
- **Evidence:**
  - m336 keeps the stall's code with a `{hint}` tail, but the tail stays English. Every tail sentence is bex's own:
    - `imagePortStallHint`'s three variants (`lego/operator/internal/controller/app_controller.go:6124-6133`) and the fallback-HTTP hint;
    - `detectBoundPort`'s four variants and `noOpenPortMessage`, with its "it listened on port N before" clause (`port_detect.go:138-210`);
    - `detectPrivatePrimary`'s two (`private_ports.go:233`, `:250`).
  - A queued build's cause is English as well: "`%s has %d/%d concurrent builds active; waiting for a slot`" (`app_controller.go:1563`), "a concurrent dispatch took the last build slot; waiting for a slot", "waiting for build capacity: …", and the datastore waits from `prepareSourceBuildEnv`.
  - `restart_wait_timed_out` prints Go's "3h0m0s" inside the zh sentence.
  - The port scan runs once per 15 s (`dueUnreadyScan`), while an in-flight rollout requeues every 5 s, so the hint appears on about one pass in three, and the stall text changes between reads in both languages.
- **Goal linkage:** the dashboard speaks its user's language (w4/m329, w4/m336). Render parity first keeps the API's English unchanged.
- **Expected outcome:** the stall a zh owner meets most (an image that listens nowhere the platform can reach) reads entirely in Chinese, and the text stops flickering.
- **Why now:** m336 shipped the mechanism and waits for its live replay (w4/m336 t005), which meets exactly this mixed, flickering text. w4/615, still open, changes how `PORT` is chosen and may reword these hints. If it lands first, t001's templates follow its sentences; if this lands first, 615 changes the templates with them.
- **Render parity:** included (t006). The change touches GraphQL and the dashboard. REST and MCP keep Render's English, and the parts are a GraphQL extension, as m329's codes are.
- **Dedupe:** w4/m336 (blocked on its release gate) coded the main sentences; w4/600 (zh absolute timestamps) is a different string family. Kubelet text that is not bex's stays English by design.

## Unblock (2026-11-19)

- **Affected:** w5/m356 t005, t009. **Source:** verified evidence (release gate).
- **Finding:** the milestone's code commit `01b35169c` has been in every production pin since `1c9c55598` (2026-11-13, pin `e0ba016b5c52`). The current pin is `4b2cebd0bd03` (`19faf35f3`, the latest `chore(deploy): pin platform images` on `origin/main`), and `git merge-base --is-ancestor 01b35169c 19faf35f3` holds. The production app cluster runs that pin: `bex-controller-manager` and `bex-api` run `bex-operator@sha256:4407341b…` and `dashboard` runs `bex-dashboard@sha256:f34e8f5f…`, the digests `deploy/gitops/base/bex.yaml` and `dashboard.yaml` pin at `19faf35f3`. No gate remains.
- **Next:** replay t005 live in zh, then close (t009).

## Live verification on pin `4b2cebd0bd03` (2026-11-19, t005)

QA user via `scripts/qa-login.sh` (logged out afterwards), workspace `bex-canary`, dashboard in zh via `?lang=zh`. Playwright MCP did not connect, so a headless Chrome (playwright-core 1.56) read the pages. MCP went through `scripts/qa-mcp.py` with a run-minted API key, revoked afterwards. Fixtures, all deleted afterwards (DELETE 204, then GET 404):

- `qa-20261119-m356-lo`, a Free image service on `python:3.12-alpine` running `python -m http.server 10000 --bind 127.0.0.1`, created 09:33:39Z.
- `qa-20261119-m356-q`, a Free Docker web service from `examples/hello-go`, and the Key Value `qa-20261119-m356-kv` it reads through `fromService` (`property: connectionString`), applied as one Blueprint at 09:41:59Z. The Key Value is Starter because the workspace's one free Key Value is w6/m469's probe. It was deleted at once (DELETE 204), and it and its Secret were gone at 09:43:57Z. The first build was canceled, and a new deploy at 09:44:11Z waited for the datastore's connection details from then on, holding no build slot (`status.buildHold.cause: datastore`).

Results:

- **Stall on a server bound to 127.0.0.1 (DoD 1).** From 09:35:41Z REST `stallReason` read "the container is running but its startup health check has not succeeded, so the rollout is waiting: the configured health check. Check that the path returns a 2xx or 3xx status on the service's own port. Nothing accepted a TCP connection on port 10000, the default because the image declares no TCP ports. The server must bind host 0.0.0.0 on the port in PORT (10000). If the service does not need to receive traffic on any port, create a background worker instead. No port answered HTTP during startup discovery." It did not change in 18 reads 5 s apart (to 09:37:10Z) and held until the deploy failed. GraphQL reads it as `probe_other_waiting` with three parts (`hint_image_default_port` with port 10000, `hint_background_worker`, `hint_no_http_discovered`) and no remainder.
  - The zh deploy header, read at 09:37:46, 09:37:57 and 09:38:07Z (a 5 s pause between reloads), showed the same text each time, entirely in Chinese: "容器正在运行，但其启动健康检查尚未成功，发布正在等待：已配置的健康检查。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。端口 10000 上没有任何进程接受 TCP 连接；镜像未声明任何 TCP 端口，因此使用的是默认端口。服务器必须绑定到 0.0.0.0，并监听 PORT 环境变量指定的端口（10000）。如果服务不需要在任何端口上接收流量，请改为创建后台工作进程。启动探测期间没有端口响应 HTTP。"
  - After it failed (`update_failed` at 09:48:42Z), the header, the Deploys list row and the Events row read "容器正在运行，但其启动健康检查在发布时限内始终未成功，部署因此失败：已配置的健康检查。……", with the same three hints in Chinese. REST `failureReason` keeps the English "… never succeeded within the rollout window, so the deploy failed: the configured health check. … No port answered HTTP during startup discovery."
- **Queued cause (DoD 2).** The held deploy stayed `queued` with no in-flight reason, its row's `updatedAt` fixed at 09:44:11Z, and the App's Ready condition read `BuildQueued`: "Waiting for a linked datastore's connection details before building" (while the cluster's four build slots were full it read "the cluster's build capacity is full; waiting for a slot"). The 125-minute build gate closed it `build_failed` at 11:49:20Z with no `startedAt`. REST `failureReason` reads "the build never started: Waiting for a linked datastore's connection details before building". GraphQL reads it as `build_never_started` with one part, `queued_datastore`, and no remainder. The zh deploy header, the Deploys list row and the Events row all read "构建一直未开始：正在等待关联数据存储的连接信息，然后再开始构建".
- **Restart wait (DoD 3).** Not staged: it needs a restart behind a deploy that runs for 3 h (t005's out of scope). The dashboard test `a restart that waited too long` words `3h0m0s` as 3 小时. The production dashboard, given that reason in its GraphQL answer (patched in the browser; nothing written), rendered "这次重启等待的部署未在 3 小时内完成".
- **REST and MCP (DoD 4).** MCP `get_deploy`'s `stallReason` was byte-identical to REST's (`cmp`). So was MCP's `failureReason` for the never-started build. Both are the English sentences the operator tests pin (`TestStallHintsAndQueuedCausesAreTemplated`).
- **Remainder (DoD 5).** No deploy in `bex-canary` carries a remainder (a GraphQL read of all five services' deploys), and none can be staged: every hint and cause bex writes now has a template. With the stall's GraphQL answer patched in the browser to carry one more sentence as its remainder, the production zh header rendered the Chinese sentence and hints, then "The probe saw an unexpected reply." in English, last. The dashboard test `keeps an unworded hint and the remainder in English` and gqlutil's remainder test cover it in CI.
- **t005 step 2's premise.** Production runs no per-workspace build cap (`BEX_MAX_CONCURRENT_BUILDS=0`; the cluster ceiling `BEX_MAX_ACTIVE_BUILDS=4` is the only one), and the 125-minute build gate outlasts a build's 120-minute deadline. So a deploy cannot be held behind a build limit until it closes without holding every tenant's builds. A deleted linked datastore holds one deploy's build without taking a slot, so it stood in.
