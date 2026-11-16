# w22 · m402 — Workspace admins can require 2FA, as on Render: a member without it is blocked, API keys and SSH included

**Worker:** worker22 **Goal:** a workspace admin can turn on "require two-factor authentication" in Workspace Settings. While it is on, a member who has not enrolled TOTP or a passkey cannot use the workspace's resources or settings from the dashboard, REST, GraphQL, MCP or SSH, and neither can an API key or SSH key that member created. They are sent to enroll, and regain access the moment they do. Turning the setting on and off is audited. **Status:** todo (t001–t005 and t007–t009 done and shipped; t006's live check is unblocked 2026-11-16 by the user's decision D4, see "解阻记录 2026-11-16"; t010 closes after it)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design record: where the setting lives, what "has 2FA" means, which principals and routes it covers, the admin lock-out guard and the check's cache; amend ADR012, ADR024 and ADR018 — **DONE** | 45m | — |
| t002 | The setting: a workspace column and migration, an admin-only GraphQL mutation and read field, and an audit event named as Render names it — **DONE** | 45m | t001 |
| t003 | Enforcement in bex-api: a member without 2FA gets a named 403 on every REST, GraphQL and MCP route of the workspace, except the exempt ones — **DONE** | 75m | t002 |
| t004 | Enforcement at the SSH gateway and the ticket mints: service SSH, the browser shell, agent-session attach and sandbox connect refuse a member without 2FA, and a live session is re-checked — **DONE** | 45m | t003 |
| t005 | Dashboard: the admin toggle in Workspace Settings, the blocked-member page that leads to enrollment, and the member row's required/not-enabled state — **DONE** | 75m | t002, t003 |
| t006 | Live check on `dev-N` with three kinds of principal: an enrolled member, a member without 2FA (session, API key, SSH) and an admin enabling it | 45m | t004, t005 |
| t007 | Render parity — **DONE** | 20m | t004, t005 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout | 10m | t006, t008, t009 |

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
- **Why now:** bex has paying workspaces (`.pm/w2/blocked/058.md` records 9 live subscriptions) whose members hold production credentials, and the second-factor foundation is shipped; the missing piece is only the enforcement layer. SSO/SAML/SCIM stay a non-goal (DO_NOT_DO): this is not SSO, and Render lists no plan gate for it, so it applies to every workspace.
- **Open recommendations, not user decisions** (this milestone's tasks implement them; the ADR in t001 records them as recommendations): "has 2FA" means an enrolled TOTP or passkey rather than an AAL2 session (Kratos already forces AAL2 on login for an enrolled user); the admin lock-out guard; exempting non-human platform principals. Nothing here diverges from Render's observable behavior, so no ADR018 divergence sign-off is needed. The sibling Render setting on the same page, allowed login methods (`ChangeTeamAllowedLoginMethodsEvent`), is not part of this milestone.
- **Render parity included:** the milestone adds a GraphQL field and mutation, a dashboard setting and an audit event, so t007 compares each surface with Render.

## Progress 2026-11-14 (/loopx w22)

- **Shipped (t001–t005, t007–t009):** the design in [ADR012 §9a](../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402), with the as-built differences recorded there (gateway verb, bounds, rollout order, the S1–S5 hardening below). Setting: `tenants.require_two_factor` (migration 0261), `Workspace.requireTwoFactor`, `setWorkspaceRequireTwoFactor(workspaceId: String!, enabled: Boolean!)`, audit event `ChangeTeam2FAEnforcementEvent`. Enforcement: one hook on `core.Base` (`internal/twofactor`, `core.TwoFactorGate`) behind every authorization decision, so REST, GraphQL and MCP share it, with `403 WORKSPACE_2FA_REQUIRED`. The SSH gateway asks bex-api over `POST /v1/workspace-two-factor` (HMAC) and runs the same hook. Dashboard: Workspace Settings card, one blocked-member page (enroll link, workspace switcher, leave), member-row state.
- **Hardening found in review and fixed with tests that fail on the old code:** an API key whose creator's Kratos identity was deleted is refused (it had been exempt), an `oauth2` identity with no client id fails closed, an agent-session object with no session reader fails closed, the default workspace cannot be given a requirement nothing enforces, and a mis-wired key-creator lookup now logs.
- **Found when merging with upstream's inbound IP rules (w9/m388):** the exempt workspaces list could have handed a blocked member the workspace's IP rules through `Workspace.ipAllowList` and `GET /v1/owners`. They are now withheld for a workspace that blocks the caller, with a test that fails without the fix.
- **A real defect t009 found:** two admins toggling at once audited twice. `SetTenantRequireTwoFactor` is now a conditional update that reports `changed`.
- **Checks:** the real-dependency backend suite (`BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...`) passed in all 78 packages; backend lint, `hack/deadcode.sh`, gofmt and `go vet` are clean; `cd dashboard && yarn lint && yarn test` pass (685 files, 7195 tests). More than 25 deliberate mutations of the gate, the funnel hook, the exempt operations, the lock-out guard, the caches and the gateway revalidation were each caught by a test, with no residue.
- **Parity (t007):** no verified drift from Render; [ADR018](../../../docs/ADR018-render-parity.md) has the row. REST and MCP are `—` by design: Render has no REST workspace-mutation API, and `owner.twoFactorAuthEnabled` is the user's own state, present only for `type=user`.
- **Not verified anywhere:** nothing ran against a live Kratos, Hydra or browser. The dashboard geometry and the enroll-then-return flow were checked in jsdom only.

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
