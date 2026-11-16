# w4 · m336 — In the zh dashboard, the deploy reasons w4/m329 does not code stay English

**Worker:** worker4 **Goal:** the health-check stall, the operator's pre-deploy verdicts, a build that never started, an abandoned deploy and every fixed cancel reason read in the dashboard's language on the deploy header, the Deploys list and Events, while REST and MCP keep Render's English byte for byte. **Status:** done — DoD replayed live on pin `7aaa8700befa` 2026-11-16 ([t005](done/t005.md))

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Types: 25 templates, a hint tail for the probe stall, token parameters — **DONE** | 45m | — |
| [t002](done/t002.md) | Writers: the operator and bex-api format through the templates — **DONE** | 45m | t001 |
| [t003](done/t003.md) | API and dashboard: `cancelReasonTemplate`, the zh and en catalogs, parameter wording — **DONE** | 40m | t002 |
| [t004](done/t004.md) | Docs — **DONE** | 5m | t003 |
| [t005](done/t005.md) | Live verification in zh on owned services — **DONE** | 20m | t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 5m | t003 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t004 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t004 |
| [t009](done/t009.md) | Closeout — **DONE** | 5m | t005–t008 |

Nine tasks, 215m (3h35m).

## Source + Goal linkage

- **Source:** promoted from inbox note w4/591 (the w4/m329 `/simplify` review, `/loopx w4` 2026-11-11) on 2026-11-11: five reason families across lego/types, the operator, bex-api's reconciler, GraphQL and the dashboard.
- **Goal linkage:** the dashboard speaks its user's language (w4/m329, w4/566); Render parity first keeps the API's English unchanged.
- **Render parity:** included. REST and MCP send the same English sentences as before, byte for byte; the codes are a GraphQL and dashboard extension, as in w4/m329.

## Finding (from w4/591)

Why: w4/m329 words 14 operator and bex-api reason sentences in zh through coded templates. A Chinese-language owner still reads the other families in English on the deploy header, the Deploys list and Events.

- **Source / severity / estimate:** w4/m329 `/simplify` review, `/loopx w4` 2026-11-11. **Minor** (i18n). ~1h; promote it if the kubelet family needs parsing beyond a template.
- **Uncoded families:**
  - The health-check and progress-deadline stall sentences (`ProbeStall*`, `lego/operator/internal/controller/app_controller.go` near `:3871`).
  - The operator's pre-deploy timeout, "the pre-deploy command did not finish within its %d-minute window…" (`lego/operator/internal/predeploy/predeploy.go:279`). bex-api's `pre_deploy_timed_out` template covers only its own watchdog sentence.
  - "the build never started: …" (`lego/backend/internal/store/reconciler.go` near `:2062`).
  - `abandonedDeployReason` (`reconciler.go` near `:2151`).
  - Kubelet image-pull messages copied into `failureReason`.
  - `cancelReason`, e.g. "Superseded by dep-…" (`reconciler.go` near `:1670` and `:1690`).
- **Fix:** add each sentence as a template in `lego/types/v1alpha1/deploy_reasons.go`, format it at its writer, and add the en and zh `deploys.reason.<code>` keys. Add `cancelReasonTemplate` on GraphQL `Deploy` and event `Details` for the cancel family. Kubelet text that is not ours stays English.
- **Tests:** `TestDashboardWordsEveryDeployReason` and the round-trip test cover each new template automatically. Add a writer test per family, and include a zh render of a cancel reason.
- **Dedupe:** w4/m329 (blocked, release gate) covers HTTP port detection, the image refusals and bex-api's three timeout sentences.

## Post-release probe (QA loop round 175, 2026-11-12): t005 passes

- **Pin:** `10acf5314735`, deployed 06:46 UTC. The dashboard was set to zh with `?lang=zh`.
- **Health-check stall:**
  - **Fixture:** Starter web service `qa-20261112-r175-web` (`srv-dbqm93s5ph4s73f9gpbg`), switched to `docker.io/library/nginx:1.27-alpine` with Health Check Path `/qa-nope`. Its deploy `dep-dbqmas45ph4s73f9gph0` was started with `POST …/deploys` at 07:06:24. The service was deleted afterwards (DELETE 204, then GET 404).
  - **While waiting (07:07:59 and 07:08:19):** the deploy header reads "容器正在运行，但其启动健康检查尚未成功，发布正在等待：端口 80 上的 GET /qa-nope。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。" The kind is 启动, and the sentence did not change between the two passes.
  - **After it failed (`update_failed` at 07:22:09):** the header, the Deploys list row and Events' `部署已完成` row all read "容器正在运行，但其启动健康检查在发布时限内始终未成功，部署因此失败：端口 80 上的 GET /qa-nope。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。"
  - REST `failureReason` keeps the English "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /qa-nope on port 80. Check that the path returns a 2xx or 3xx status on the service's own port."
- **Superseded:**
  - **Fixture:** Free web service `qa-20261112-r175-sup` (`srv-dbqmas8rcnqs73dmd0jg`, whoami), deleted afterwards.
  - Two `POST …/deploys` 4 s apart: the first, `dep-dbqmcmc5ph4s73f9gpm0`, closed `canceled` at 07:10:41 with no `startedAt`.
  - REST `cancelReason` reads "Superseded by dep-dbqmcngrcnqs73dmd0r0".
  - The zh Deploys list, deploy header and Events all read "已被 dep-dbqmcngrcnqs73dmd0r0 取代". Events adds "较新的部署取代了此部署。"
- **Suspended:**
  - `POST …/suspend` at 07:10:55, while `dep-dbqmcngrcnqs73dmd0r0` was `update_in_progress`. That deploy closed `canceled` at 07:11:10.
  - REST `cancelReason` reads "Canceled: the service was suspended".
  - The zh list, header and Events read "已取消：服务已暂停".
- **Seen and filed separately:**
  - The same pages show absolute dates in English (w4/600).
  - Events shows the failed deploy's duration as `930s` against the list's `15m 29s` (w4/601).

## Unblock (2026-11-16)

- **Affected:** w4/m336 t005, t009. **Source:** verified evidence (release gate).
- **Finding:** production runs platform pin `7aaa8700befa` (`546945798`, the latest `chore(deploy): pin platform images` on `origin/main`). `git merge-base --is-ancestor` puts the gate line's `4630a48a7`, the fix commit `88d0df3b2` (w4/m336) and the stall follow-up `01b35169c` (w5/m356) inside it, so the release gate is clear and no gate remains. Next step: replay t005 live, then close (t009).

## Live verification on pin `7aaa8700befa` (2026-11-16, t005)

QA user via `scripts/qa-login.sh`, workspace `bex-canary`, dashboard in zh via `?lang=zh`. Fixtures were two image services, both deleted afterwards (DELETE 204, then GET 404):

- `qa-20261116-r336-web` (`srv-dbtm7k8ddohc73aivf5g`, Starter, `docker.io/library/nginx:1.27-alpine`, Health Check Path `/qa-nope`), first deploy `dep-dbtm7k8ddohc73aivf60` at 20:13 UTC;
- `qa-20261116-r336-sup` (`srv-dbtm7lfmufrc73b9vkj0`, Free, `traefik/whoami:v1.10.1`).

Results:

- **Health-check stall while waiting (20:16 and 20:17:17).** The deploy header read "容器正在运行，但其启动健康检查尚未成功，发布正在等待：端口 80 上的 GET /qa-nope。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。" both times, kind 启动, no flip between passes; the port hint is now in zh too (w5/m356).
- **After it failed (`update_failed` at 20:28:07).** The header, the Deploys list row and Events' `部署已完成` row read "容器正在运行，但其启动健康检查在发布时限内始终未成功，部署因此失败：端口 80 上的 GET /qa-nope。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。" REST `failureReason` keeps the English "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /qa-nope on port 80. Check that the path returns a 2xx or 3xx status on the service's own port."
- **Superseded.** Three `POST …/deploys` about 2 s apart at 20:15:03: the queued middle one, `dep-dbtm8i8ddohc73aivf90`, closed `canceled` with no `startedAt`. REST `cancelReason` reads "Superseded by dep-dbtm8ioddohc73aivfa0"; the zh Deploys list, the deploy header and Events read "已被 dep-dbtm8ioddohc73aivfa0 取代", and Events adds "较新的部署取代了此部署。" (Two deploys alone no longer supersede: the first starts immediately and the second queues behind it.)
- **Suspended.** `POST …/suspend` at 20:14:29 while `dep-dbtm82fmufrc73b9vkm0` was `update_in_progress`; it closed `canceled` at 20:14:29. REST `cancelReason` reads "Canceled: the service was suspended"; the zh list, header and Events read "已取消：服务已暂停".
