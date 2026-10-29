# w8 · m127 — Free services cannot send outbound SMTP (25/465/587), as on Render

**Worker:** worker8 **Goal:** A free web service, private service, worker or cron job is refused outbound TCP 25, 465 and 587, as Render's free instances are. Paid services keep 587 and 2525. 25 and 465 stay blocked for every plan by the provider, which the user decided on 2026-10-20 to keep. Tenants see one documented rule. **Status:** todo（2026-10-29 解除阻塞，待线上验收；原记录：t001–t006 done 2026-10-20; t007 live check on production after GitOps syncs the platform policy and the release pipeline deploys the operator and the dashboard

## Tasks (in order)

| id   | title                                                                      | est | depends_on                 |
| ---- | -------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Operator: free-plan egress policy denying TCP 25/465/587                    | 60m | —                          |
| t002 — **DONE** | Plan changes: upgrade lifts the deny, downgrade restores it                 | 30m | w8/m127/t001               |
| t003 — **DONE** | Tenant docs and dashboard copy: the SMTP rule per plan                      | 30m | w8/m127/t001               |
| t004 — **DONE** | Render parity: egress ports per plan in ADR018                              | 15m | w8/m127/t003               |
| t005 — **DONE** | Simplify the changed code                                                   | 15m | w8/m127/t004               |
| t006 — **DONE** | Test coverage: free vs paid policies, plan changes, other ports untouched   | 30m | w8/m127/t004               |
| t007 | Close out the milestone                                                     | 30m | w8/m127/t005, w8/m127/t006 |

## Definition of done

Live on production after the release pipeline deploys the operator, with owned fixtures deleted by ID with 404 proofs:

- From a free service, a TCP connect to `smtp.gmail.com:587` fails fast (refused or dropped by policy), and `:443` still connects.
- From a paid service, `:587` connects and greets `220`. `:25` and `:465` still fail (provider block, unchanged).
- Upgrading the free service to a paid plan lets `:587` connect after the rollout, and downgrading blocks it again.
- ADR018's "Outbound SMTP ports" row, the tenant docs and the dashboard copy all describe the same rule.

## Source + Goal linkage

- **Source:** inbox note `w8/149` (`/qa-find-bugs-cli` sweep 1256, 2026-10-20), [kept in done/149.md](../done/149.md). User decision 2026-10-20: do **not** request Hetzner's 25/465 unblock; refuse free services 587 too, as on Render.
- **Goal linkage:** [ADR018](../../../docs/ADR018-render-parity.md) "Outbound SMTP ports" row (render.com/docs/free: "Free web services can't send outbound network traffic on ports 25, 465, or 587"); free-tier anti-abuse; [ADR022](../../../docs/ADR022-tenant-isolation.md)/[ADR043](../../../docs/ADR043-tenant-namespace-isolation.md) per-App network policy.
- **Expected outcome:** free-plan behavior matches Render's, free instances cannot be used to send spam over submission ports, and tenants get one documented rule instead of silent timeouts.
- **Why now:** the user decided it on 2026-10-20, and the parity row already records the gap.
- **Render parity included:** tenant-visible network behavior and dashboard copy change.

## Notes

- Build on the operator's per-App policy (`reconcileNetworkPolicy`, `app_controller.go`; `environment_network_policy.go`; the platform CiliumNetworkPolicy egressDeny pattern in ADR022 §Node and cloud-metadata egress). A deny must not widen anything else. DNS and every other port behave as today.
- 25/465 for every plan stay provider-blocked by decision: do not add bex config that implies they could open.

## Progress (2026-10-20)

- t001: the operator labels every pod of a free-plan App (`isFreeApp`: `free` or an empty tier) `app.bex.co/free-plan=true` — Deployment (`appPolicyLabels`), cron runs (`cronPodSpec`), pre-deploy Jobs (`predeploy.Options.FreePlan`). The platform `CiliumClusterwideNetworkPolicy` `deny-free-plan-smtp-egress` (`deploy/gitops/base/tenant-node-egress.yaml`) egress-denies TCP 25/465/587 to `world` for that label, with `enableDefaultDeny.egress: false`; `scripts/gitops-validate.sh` pins its exact shape. A Cilium-native policy because a portable NetworkPolicy cannot subtract from the namespace's internet allow.
- t002: a plan change converges the label in both directions, including a held release's served template (`syncPolicyLabels`); envtest `App free-plan SMTP label` covers free → paid → free. The pods roll with the new template, as a tier change already does. The operator rollout rolls existing free (and untiered) Apps' pods once; paid templates are unchanged.
- t003: the dashboard plan picker states the rule under its Free and Paid groups (en + zh), and its loading skeleton draws the same lines. No in-repo tenant docs exist; the picker is the tenant-facing statement.
- t004: ADR018 "Outbound SMTP ports" row and ADR022 §Free-plan SMTP egress.
- t005: `/simplify` applied (one `freePlanLabelValue` helper on `isFreeApp` instead of a second free constant, `world` instead of `all` so in-cluster relays are unaffected). Declined: extracting a shared envtest manager helper.
- t006: `free_plan_smtp_test.go` (service, cron and pre-deploy labels per tier; paid labels untouched; label sync both ways) and `free_plan_smtp_envtest_test.go`. They fail on the pre-milestone code (no label existed).



## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m127
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。
