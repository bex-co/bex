# w22 · m402 — Workspace admins can require 2FA, as on Render: a member without it is blocked, API keys and SSH included

**Worker:** worker22 **Goal:** a workspace admin can turn on "require two-factor authentication" in Workspace Settings. While it is on, a member who has not enrolled TOTP or a passkey cannot use the workspace's resources or settings from the dashboard, REST, GraphQL, MCP or SSH, and neither can an API key or SSH key that member created. They are sent to enroll, and regain access the moment they do. Turning the setting on and off is audited. **Status:** done (2026-11-16: the live check ran on `dev-10` against real Kratos, Hydra, OpenFGA and the SSH gateway, see "Live check 2026-11-16"; it found and fixed three defects)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design record: where the setting lives, what "has 2FA" means, which principals and routes it covers, the admin lock-out guard and the check's cache; amend ADR012, ADR024 and ADR018 — **DONE** | 45m | — |
| t002 | The setting: a workspace column and migration, an admin-only GraphQL mutation and read field, and an audit event named as Render names it — **DONE** | 45m | t001 |
| t003 | Enforcement in bex-api: a member without 2FA gets a named 403 on every REST, GraphQL and MCP route of the workspace, except the exempt ones — **DONE** | 75m | t002 |
| t004 | Enforcement at the SSH gateway and the ticket mints: service SSH, the browser shell, agent-session attach and sandbox connect refuse a member without 2FA, and a live session is re-checked — **DONE** | 45m | t003 |
| t005 | Dashboard: the admin toggle in Workspace Settings, the blocked-member page that leads to enrollment, and the member row's required/not-enabled state — **DONE** | 75m | t002, t003 |
| t006 | Live check on `dev-N` with three kinds of principal: an enrolled member, a member without 2FA (session, API key, SSH) and an admin enabling it — **DONE** | 45m | t004, t005 |
| t007 | Render parity — **DONE** | 20m | t004, t005 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout — **DONE** | 10m | t006, t008, t009 |

## Definition of done

- An admin of any workspace, on any plan, can turn the setting on and off in Workspace Settings. A non-admin cannot (named 403 on the mutation, no control in the UI).
- With it on, for a member who has no TOTP and no passkey enrolled: a dashboard session, a REST/GraphQL/MCP call carrying their session, an API key they created, and an SSH or shell ticket mint for a service in the workspace all get the named 403 `WORKSPACE_2FA_REQUIRED` (or the gateway's refusal), with the workspace id in its details. The dashboard shows a page that explains why and leads to Kratos enrollment in account settings. The member's other workspaces are unaffected.
- After the member enrolls, access returns within the check's cache lifetime (recorded in the ADR). A member who removes their last second factor while it is on is blocked again within the same bound, and a live SSH or shell session is closed at the gateway's next revalidation.
- The admin who enables it must have 2FA themselves, so the setting cannot lock out its only actor (a bex safety guard, recorded in the ADR as a recommendation of this milestone, not a Render fact).
- Enabling and disabling write an audit-log entry that REST `GET /v1/owners/{ownerId}/audit-logs` and GraphQL return under Render's event type `ChangeTeam2FAEnforcementEvent`.
- Every path above has an automated test, and the live check on `dev-N` is recorded in this README with the observed codes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w22` (2026-11-14), filed by `/pmx all to w22`.
  - **Render** ([render.com/docs/login-settings](https://render.com/docs/login-settings), read 2026-11-14): "Enforce two-factor authentication (2FA) from your **Workspace Settings** page"; "Only workspace admins can configure these enforcements"; "your team members can't access the workspace's resources or settings until they enable 2FA for their Render account"; "Team members with SSH or API keys can't use these keys to access workspace resources until they enable 2FA". The page lists no plan requirement for 2FA enforcement (the plan requirement it states is for SAML SSO).
  - **Pinned Render OpenAPI** (`lego/backend/internal/api/openapi/render-public-api-1.json`): the audit-log event type enum contains `ChangeOrg2FAEnforcementEvent` and `ChangeTeam2FAEnforcementEvent`.
  - **bex today:** Kratos has TOTP, WebAuthn and recovery codes with a `highest_available` AAL policy (ADR018 MFA row, w4/m11; ADR012 §9). `lego/backend/internal/workspaces/kratos.go` (~lines 191–199) derives `MFAEnabled` from the identity's `totp`/`webauthn` credentials, and `members/service.go` and `dashboard/src/features/team/components/member-row.tsx` only display it as a badge. There is no workspace setting, no refusal and no audit event. `grep -rniE "2fa.?enforc|WORKSPACE_2FA|Team2FA|Org2FA|secure login" .pm docs lego dashboard/src` finds only the OpenAPI enum.
- **Goal linkage:** ADR008 § Render parity first (Identity, workspaces and account); pillar 1, Render-compatible REST and GraphQL.
- **Expected outcome:** a workspace holding production credentials can require every member, and every key and SSH session a member holds, to be second-factor protected, as on Render.
- **Why now:** bex has paying workspaces (`.pm/w2/done/058.md` records 9 live subscriptions) whose members hold production credentials, and the second-factor foundation is shipped; the missing piece is only the enforcement layer. SSO/SAML/SCIM stay a non-goal (DO_NOT_DO): this is not SSO, and Render lists no plan gate for it, so it applies to every workspace.
- **Open recommendations, not user decisions** (this milestone's tasks implement them; the ADR in t001 records them as recommendations): "has 2FA" means an enrolled TOTP or passkey rather than an AAL2 session (Kratos already forces AAL2 on login for an enrolled user); the admin lock-out guard; exempting non-human platform principals. Nothing here diverges from Render's observable behavior, so no ADR018 divergence sign-off is needed. The sibling Render setting on the same page, allowed login methods (`ChangeTeamAllowedLoginMethodsEvent`), is not part of this milestone.
- **Render parity included:** the milestone adds a GraphQL field and mutation, a dashboard setting and an audit event, so t007 compares each surface with Render.

## Progress 2026-11-14 (/loopx w22)

- **Shipped (t001–t005, t007–t009):** the design in [ADR012 §9a](../../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402), with the as-built differences recorded there (gateway verb, bounds, rollout order, the S1–S5 hardening below). Setting: `tenants.require_two_factor` (migration 0261), `Workspace.requireTwoFactor`, `setWorkspaceRequireTwoFactor(workspaceId: String!, enabled: Boolean!)`, audit event `ChangeTeam2FAEnforcementEvent`. Enforcement: one hook on `core.Base` (`internal/twofactor`, `core.TwoFactorGate`) behind every authorization decision, so REST, GraphQL and MCP share it, with `403 WORKSPACE_2FA_REQUIRED`. The SSH gateway asks bex-api over `POST /v1/workspace-two-factor` (HMAC) and runs the same hook. Dashboard: Workspace Settings card, one blocked-member page (enroll link, workspace switcher, leave), member-row state.
- **Hardening found in review and fixed with tests that fail on the old code:** an API key whose creator's Kratos identity was deleted is refused (it had been exempt), an `oauth2` identity with no client id fails closed, an agent-session object with no session reader fails closed, the default workspace cannot be given a requirement nothing enforces, and a mis-wired key-creator lookup now logs.
- **Found when merging with upstream's inbound IP rules (w9/m388):** the exempt workspaces list could have handed a blocked member the workspace's IP rules through `Workspace.ipAllowList` and `GET /v1/owners`. They are now withheld for a workspace that blocks the caller, with a test that fails without the fix.
- **A real defect t009 found:** two admins toggling at once audited twice. `SetTenantRequireTwoFactor` is now a conditional update that reports `changed`.
- **Checks:** the real-dependency backend suite (`BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...`) passed in all 78 packages; backend lint, `hack/deadcode.sh`, gofmt and `go vet` are clean; `cd dashboard && yarn lint && yarn test` pass (685 files, 7195 tests). More than 25 deliberate mutations of the gate, the funnel hook, the exempt operations, the lock-out guard, the caches and the gateway revalidation were each caught by a test, with no residue.
- **Parity (t007):** no verified drift from Render; [ADR018](../../../../docs/ADR018-render-parity.md) has the row. REST and MCP are `—` by design: Render has no REST workspace-mutation API, and `owner.twoFactorAuthEnabled` is the user's own state, present only for `type=user`.
- **Not verified anywhere:** nothing ran against a live Kratos, Hydra or browser. The dashboard geometry and the enroll-then-return flow were checked in jsdom only.

## Live check 2026-11-16 (t006, /loopx w22)

Run on `dev-10` (w22 has no slot) with the agent leg, because only that leg has a real OpenFGA and the in-cluster SSH gateway: a real Kratos (TOTP and WebAuthn), Hydra, OpenFGA, bex-api and `ssh-gateway` built from this tree, the dashboard dev server and a real `ssh` client. Workspace `m402-live` on Pro (Hobby allows only the admin role), admin A, developers B and C, one `starter` image service, an API key and an SSH key each for B and C. Nothing ran against production. The environment is torn down.

**Steps 1 to 7, as observed:**

1. Before the setting: B's and C's sessions, API keys, shell-ticket mints and native SSH all work. B and C turning the setting on get GraphQL `FORBIDDEN` (audited as `ChangeTeam2FAEnforcementEvent`, status `error`, `outcome: denied`).
2. A without a second factor gets `WORKSPACE_2FA_ADMIN_NOT_ENROLLED`, and no audit row. A enrolls TOTP and turns it on: one `ChangeTeam2FAEnforcementEvent`, status `success`, `to: "true"`, from REST `GET /v1/owners/{id}/audit-logs` and GraphQL `auditLogs`. Turning it on again writes no row.
3. C (no second factor), first probe 34 s after enabling: session REST `403 WORKSPACE_2FA_REQUIRED` with `params.workspaceId`; session GraphQL the same code with `extensions.workspaceId`; C's API key the same on REST and GraphQL; MCP `list_services` with C's key answers `WORKSPACE_2FA_REQUIRED: …`, and `list_workspaces` leaves `ipAllowList` out for C while B gets it; shell-ticket mint 403 with the code; native `ssh` prints the banner "bex: this workspace requires two-factor authentication. Enable it in your account settings, then connect again." and then `Permission denied (publickey)`. C's own workspace answers 200, and the workspaces list still shows `requireTwoFactor: true` for this one. The dashboard shows the blocked page for C (heading, "Set up two-factor authentication" to `/settings#security`, recheck, workspace switcher, leave).
4. B (TOTP): all of the above succeed, including B's API key, a shell ticket redeemed at the gateway (WebSocket 101) and native SSH. So Hydra's `bex.co/created-by` stamp reads back for a fresh key in both directions.
5. C enrolls TOTP in the dashboard through the blocked page's link. Session and API key work at the first probe, 29 s later; native SSH 52 s later. Kratos raises the enrolling session to AAL2 itself; C's other, older session answers Kratos 403 until it steps up, which is Kratos's `highest_available` policy and not the gate.
6. C removes TOTP while holding an open native SSH session and an open web shell. Session and API key are refused at the first probe, a new SSH login 17 s later, and both open sessions are closed 46 s and 47 s later (the web shell with `{"type":"error","message":"session closed"}`).
7. A turns it off: C works at once on the API and 31 s later over SSH; a second `success` row, `to: "false"`. Turning it off again writes no row.

**Passkey, which the unit tests could not show:** with the setting on again and C holding no TOTP, C registered a WebAuthn key in the dashboard (Chromium's virtual authenticator). API access returned 47 s later and SSH 70 s later. Kratos lists a `webauthn` credential for every identity, with or without a key, and the gate told the two apart: C was refused before the key and admitted after it.

**Dashboard as the admin:** A signs in through the TOTP challenge, sees the switch on, all three member rows with the 2FA badge, turns it off with the switch ("Two-factor authentication is no longer required in m402-live") and the audit row follows. The member row's "required, not enabled" state was not looked at in the browser: by then every member had a factor.

**Bounds, all inside ADR012 §9a:** bex-api 60 s (observed 0 to 47 s), a new SSH login about 90 s (17 to 70 s), an open session about 150 s (46 and 47 s).

**Defects the check found, each fixed with a test that fails without the fix:**

- **Nobody could enroll an authenticator app in the dashboard.** `dedupeUiNodes` (`dashboard/src/common/lib/ory/sanitize-flow-nodes.ts`, since 2026-08-25) keyed nodes by group, name, type and value. The settings flow's QR image and secret text have none of the last three, so the secret was dropped as a duplicate of the QR, and Ory Elements renders the enrollment form only when it finds both. The page said "scan the QR code" and showed neither a code nor an input. The blocked page's link therefore led to a form nobody could complete. The key now includes the node type and id.
- **A non-admin saw a live switch.** The settings card decided "admin" from `workspace.role`, which the workspace list reports as `admin` for every membership, so developer C got a switch that could only answer 403. It now uses the capability check (`can_manage`), like the cards beside it, and shows the placeholder while that check is in flight.
- **Native SSH never worked on a `dev-N` stack.** The dev gateway asked the in-cluster bex-api Service, which does not exist there, whether a key's owner has a verified email, so every login was refused with "verify your bex account email first". `scripts/dev-env/agent/ssh-gateway.yaml` now sets `BEX_IDENTITY_VERIFICATION_API_URL` to the host bex-api, and `scripts/dev-env.test.sh` checks it.

**What it took to run, for the next live check (m403 t011, m413 t008):**

- `kind` 0.33.0 and `clusterctl` 1.14.4 from Homebrew; colima at 8 CPU / 16 GiB.
- colima's VM has `fs.inotify.max_user_instances=128`. With that, containerd in every CAPD worker fails to start its CRI plugin ("failed to create fsnotify watcher: too many open files") and the workers never join. `colima ssh -- sudo sysctl -w fs.inotify.max_user_instances=8192` fixes it until the VM restarts; `mock-cluster.sh` does not check for it.
- This Mac's `docker` has no buildx plugin, and the legacy builder cannot parse the Dockerfiles' `$BUILDPLATFORM`. The VM's own docker has buildx and mounts the repo at the same path, so a `docker` shim on `PATH` that runs `build` through `colima ssh -- docker` was enough. Nothing else was installed.
- The browser shell needs `BEX_SHELL_WS_URL` exported before `agent-up` (bex-api inherits it) and a port-forward of the gateway's `:8080`; native SSH needs one of `:2222`. `dev-env.sh` sets up neither.
- The kind/CAPD cluster and colima are left running for the two checks that follow; `dev-10`, its agent leg and its tenant namespaces are removed.

**To repeat:** `bash scripts/mock-cluster.sh`, `bash scripts/mock-cluster.sh operator`, `bash scripts/dev-env.sh <N> up`, `agent-up`; register three Kratos identities over the API flows as `scripts/auth-mfa-e2e.sh` does, mark their addresses verified through the Kratos admin API, and walk t006's steps with `X-Session-Token`, `Authorization: Bearer bex_<id>_<secret>` and `ssh -p <forward> srv-…@127.0.0.1`.

**Seen and left alone (not part of this milestone):** a blocked member's account settings show "couldn't load API keys" with the generic message, not the 2FA reason; the dev gateway template also leaves the share-file, result and spawn callbacks at the in-cluster default.

## Gate: t006 (the live check) — cleared 2026-11-16, see "解阻记录 2026-11-16"

(The gate as it stood on 2026-11-14, kept for history.) **BLOCKED — needs, from the user or infra, a machine that can run the shared local cluster.** `scripts/dev-env.sh <N> up` requires the kind/CAPD cluster `bex` from `bash scripts/mock-cluster.sh` (kind, clusterctl, CAPD, CNPG, the operator, charts pulled from ghcr). The machine this session ran on has neither `kind` nor `clusterctl` and an 8 GiB Docker VM, and a ghcr pull returned 504 in the same session, so the stack could not be brought up. Everything t006 asks is unchanged: with real Kratos TOTP, an enrolled member (B), a member without 2FA (C), and an admin (A) on one workspace: A is refused enabling without 2FA, then enrolls and enables (one `ChangeTeam2FAEnforcementEvent`); C's dashboard session, REST/GraphQL, own API key, shell ticket and native SSH are all refused and C's other workspace still works; B's everything works; C enrolls and regains access within the documented bound (about 60 s API, about 90 s SSH); C removes TOTP and is blocked again, and an open shell is closed within about 150 s; A turns it off. Things only a live stack can show: that `MFAEnrolled` reads a real TOTP and a real passkey enrollment correctly (the webauthn `include_credential` assumption is the one the member badge already makes), that Hydra's `bex.co/created-by` stamp reads back for a fresh key, and the SSH banner in a real `ssh` client. Record the observed codes and timings under a "Live check" heading here, then run t010.

**Rollout note for whoever deploys:** bex-api must be running the new verb before the new SSH gateway, otherwise native SSH and ticket redemption are refused until it is (ADR012 §9a "Rollout order").

## 解阻记录 2026-11-14（/unblock w22）

- **来源：** 复核证据；本次没有新的用户决定，等用户选路径。
- **卡点：** t006 的实机检查要真实的 Kratos 加 TOTP，本机跑不起共享 kind/CAPD 栈：Docker VM 只有 8 GiB，`kind`、`clusterctl` 没装，ghcr 拉取返回 504。
- **你能怎么帮（二选一，都不是必须）：**
  - **路径 1（默认）：** 把 Docker Desktop 的内存调到至少 16 GiB，并确认本机能拉 ghcr.io。之后 worker 自己把 `kind`、`clusterctl` 装进 scratchpad（带 sha256 校验）并跑 `bash scripts/mock-cluster.sh`（该动作已预先批准），再做 t006。
  - **路径 2：** 用户另行授权 worker 把这项检查做成生产探针（估计需要两个 QA 账号加 TOTP，且不能给真实 workspace 开启强制 2FA，这一点未验证）。目前没有授权，所以不走。
- **不做的后果：** 代码已发布且有真实 Postgres 测试覆盖，里程碑停在最后一步实机验收。

## 解阻记录 2026-11-16（/unblock w22）

- **来源：** 用户决定 **D4**（2026-11-16）：对"本机搭建 + 生产探针"的推荐项回复 "ok, approve"。完整范围和复核证据记在 [m413 的同名记录](../m413/README.md)；本项只用其中的**本机部分**，**没有生产探针**：不要在生产的 workspace 上开 2FA 强制（原来的路径 2 里"这一点未验证"仍然成立，用户没有批准）。
- **本机部分（已批准）：**
  1. 装 `kind`、`clusterctl`（brew，或带 sha256 校验的二进制放进 scratchpad，worker 选）。
  2. colima 默认 profile 改成 8 CPU / 16 GiB 并启动（现在是停止的，4 CPU / 8 GiB；`bex-w21` 共用这个 Docker，启动前看一眼 `docker ps`）。
  3. 跑 `bash scripts/mock-cluster.sh` 和 `scripts/dev-env.sh <N> up`。N 只能是 1 到 10，w22 没有自己的槽：先 `status` 选一个没起来的，用完 `down`。
  4. 本项不需要 agent 腿，也不需要模型。TOTP 用 `scripts/auth-mfa-e2e.sh` 的做法在 dev 栈的 Kratos 上登记。
  5. 护栏（worker 的）：数据卷剩余磁盘低于 25 GiB 就停下，把读数写进新闸门。
  6. 结束：`dev-N down`；`docker ps` 没有别的容器时 `colima stop`。
- **闸门已清除：** 授权已给，机器经核实够用（Apple M6、12 核、32 GiB，数据卷剩 67 GiB，ghcr 可达）。bring-up 失败时，把精确原因写成新闸门再停靠。
- **worker 的下一步：** 与 m413、m403 共用同一次环境搭建，先做 t006（它不依赖 agent 腿，最简单）；验证 2FA 的 TOTP、API key 和 SSH 各路径，按 t006 的步骤 1 到 8 记录。部署顺序提醒（Rollout note）不变。
- **验收：** t006 的验收标准；然后 t010 收尾。
