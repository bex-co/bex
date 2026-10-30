# w3 · m79 — Run the full agent-session stack locally on `dev-N`

**Worker:** worker3 **Goal:** a developer can bring the entire [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md) agent-session stack up end to end on the shared local kind/CAPD `bex` cluster — live sandbox, streamed turn, draft PR — via one opt-in path on `scripts/dev-env.sh N`. **Status:** todo — t001–t010 done; 2026-10-29 unblock: developer GitHub App `bex-dev3-agents-puncsky` + disposable repo `puncsky/bex-dev3-agent-sandbox` staged in `dev-3/.agent/`; t012 installs it via the bex connect flow and proves the draft PR, then t011 closes

## Tasks (in order)

| id   | title                                                                            | est | depends_on            |
| ---- | -------------------------------------------------------------------------------- | --- | --------------------- |
| t001 | Dev auth/secret services: OpenFGA + OpenBao for `dev-N` — **DONE**                 | 45m | —                     |
| t002 | Cilium CRD + OpenSandbox controller/CRD + relaxed local template into CAPD — **DONE** | 45m | —                     |
| t003 | In-cluster OpenSandbox k8s-runtime server + multi-tenant `<ws>-sandbox` resolution — **DONE** | 45m | t002                  |
| t004 | Agent image: build + import into CAPD nodes — **DONE**                             | 30m | —                     |
| t005 | In-cluster ssh-gateway: image + Deployment/SA/RBAC/host-key/DB-role/Service — **DONE** | 60m | t001, t002            |
| t006 | Reverse hop (gateway → host bex-api `:8091`) + `:8081`/`:8083` port-forwards — **DONE** | 30m | t005                  |
| t007 | `dev-env.sh N agents` overlay: orchestrate the above + inject bex-api env — **DONE** | 60m | t001, t003, t004, t006 |
| t008 | `status`/verify agent stack + end-to-end repo-less session smoke — **DONE**        | 45m | t007                  |
| t009 | Simplify — **DONE**                                                               | 20m | t008                  |
| t010 | Test coverage: `dev-env.test.sh` derivation for the new ports/env — **DONE**       | 30m | t008                  |
| t012 | Real GitHub App delivery smoke: local session opens a draft PR                    | 45m | t008                  |
| t011 | Closeout                                                                          | 10m | t009, t010, t012      |

## Definition of done

`bash scripts/dev-env.sh N up --agents` (or the equivalent subcommand) brings up, on top of the base env, OpenFGA + OpenBao + the Cilium CRD + the OpenSandbox controller/CRD + an in-cluster OpenSandbox k8s-runtime server + an in-cluster ssh-gateway, and injects the full agent-session env into the locally-run bex-api so all three gates flip. `dev-env.sh N status` asserts `capabilities.enabled == true` and gateway/OpenSandbox/OpenFGA health. With a developer-supplied model key (in OpenBao at `tenants/default/agent-sessions/<ws>/model-key`) and a real GitHub App installation, creating a session lands a sandbox pod in `<ws>-sandbox`, the attach stream emits `data-acp` parts, one turn runs against the model proxy, and delivery opens a draft PR. The runbook `docs/runbooks/agent-sessions-local-dev.md` matches the shipped behavior.

## Live verification (dev-3, 2026-08-29)

- `agent-up` completed twice after the fixes; the second idempotent run kept the reverse-hop proxy pod stable and restarted bex-api without a listener collision.
- Authenticated `status` reported `verification inventory: ALL GREEN`, including OpenFGA store `bex`, unsealed OpenBao, OpenSandbox, both CRDs, reverse-hop reachability, gateway attach/exec, and `capabilities.enabled=true`.
- A real OpenAI key stored through OpenBao ran Codex session `ags-da9rm6hjg4r6vgri6avg` to `completed`; sandbox `dbb2739d-99c5-4e7b-839e-f4a38e29b0b1` ran in the correct `<ws>-sandbox` namespace on the control-plane node. Terminal attach replay returned HTTP 200, the v1 marker, 15 parts including five `data-acp-*` parts, the exact real-model response, and `[DONE]`.
- The remaining capability state is exactly `enabled=true`, `modelKeyReady=true`, `github.connected=false`, `ready=false`. `.pm/w3/dev-3/.agent/github-app.env` and `github-app.pem` are absent. A normal `gh` user token cannot substitute for the App OAuth + installation-token paths, so no draft PR was fabricated and the milestone remains open. A fresh local connection needs App ID, slug, OAuth client ID/secret, private key, and an installation on the disposable repo.

## Source + Goal linkage

- **Source:** user request 2026-08-21 ("how to ensure we can run the entire agents session stack locally with dev-5") + the full-stack trace recorded in `docs/runbooks/agent-sessions-local-dev.md`.
- **Goal linkage:** pillar 5 (ADR047 cloud coding-agent sessions). Today the feature is only exercisable against prod (`scripts/agent-session-verify.sh`); this gives every workstream a local inner loop for agent-session/dashboard/transcript work.
- **Expected outcome:** the agent-session gates (`enabled`/`modelProxy`/`ticket`) flip on `dev-N`, and a session runs to a draft PR against a local sandbox — provably, via `status` + smoke.
- **Why now:** the dashboard agent-session surface (routes `agents`, `agents_.$agentSessionId`) and the transcript/hibernation/archive work (ADR051/059/065) have no local test path, so every change is validated blind or against prod. This closes that gap.
- **Render parity task omitted:** this milestone changes only local dev tooling (scripts, dev manifests, a runbook) — no REST/GraphQL/MCP/UI surface changes — so the standing Render-parity task does not apply.

## Notes / risks

- The ssh-gateway **must** run in-cluster locally (attach dials `podIP:8787`; git/model proxies are pod-initiated over cluster DNS — neither routable from the host on CAPD). See the runbook's placement table.
- The one novel local hop is in-cluster gateway → **host** bex-api `:8091` (credential/model mint), the mirror of dev-env's existing host→cluster port-forwards.
- Real credentials are developer-supplied and never committed: a model provider key (OpenBao) and a GitHub App installation.
- Local divergences from prod are intentional and documented: no Cilium enforcement (Calico), no gVisor (runc). A local run is **not** isolation validation.

## Unblock 记录（2026-10-29，/unblock w3，origin/main `e4d9c65d6`）

- **涉及：** t012、t011。
- **用户明确决定 D6（2026-10-29）：** 由 agent 用有头浏览器自行完成 GitHub 侧操作，用户只负责登录 GitHub（puncsky）。
- **已完成（`/unblock` 本次执行）：**
  - 新建一次性私有仓库 `puncsky/bex-dev3-agent-sandbox`（只有 README，可随时删除）。
  - 新建 GitHub App `bex-dev3-agents-puncsky`（App ID `5534636`，Client ID `Iv23liIxB5ivjlBUZpyR`，只能安装在本账号）。权限：contents write、pull_requests write、checks read、metadata read（`gh api /apps/bex-dev3-agents-puncsky` 已核对）。Redirect URI 为 `http://localhost:54030/v1/git/callback`（dev-3 的 `BEX_API_PORT`），已开启「Request user authorization (OAuth) during installation」，webhook 关闭（本地收不到），不设 Setup URL（符合 ADR026 §2）。
  - 私钥 → `dev-3/.agent/github-app.pem`；`BEX_GITHUB_APP_ID/_SLUG/_CLIENT_ID/_CLIENT_SECRET` → `dev-3/.agent/github-app.env`。两个文件都是 0600、已 gitignore，值没有出现在对话或日志里。client secret 已验证有效：用假 code 调 OAuth 返回 `bad_verification_code`，而不是 `incorrect_client_credentials`。
- **有意未做：** 没有安装 App。t012 应走 bex 的 connect 流程（`POST /v1/git/connect` → 首次安装，GitHub 会带回签名 state）来安装，并且**只选** `puncsky/bex-dev3-agent-sandbox`，这样首次安装路径也能被验证。如果已经误装，就走 ADR078 §3a 的 claim 流程。
- **剩余阻塞：** 无。worker 下一步：重建 mock cluster（预先批准）→ `bash scripts/dev-env.sh 3 up --agents`（或 `agent-up`）→ 在 dev-3 dashboard 上连接 GitHub。Playwright 浏览器已登录 GitHub；会话失效时请用户重新登录。之后用 sandbox 仓库跑一次 session，直到打开 draft PR，记录 PR URL、session id 和 `status` 输出 → t011 closeout。完成后这个 App 和仓库可以保留给本地复用，也可以删除，由 worker 在 closeout 里记录。
