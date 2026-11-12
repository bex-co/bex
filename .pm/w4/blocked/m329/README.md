# w4 · m329 — In the zh dashboard, the service header's "Slug" label, the CPU unit "cores" and the operator's deploy reason sentences stay English

**Worker:** worker4 **Goal:** a Chinese-language owner reads the service header, the Metrics CPU limit and a deploy's in-progress or failure reason in Chinese, while every API keeps Render's English sentences. **Status:** blocked — t001–t004 and t006–t008 done (shipped); t005 live replay waits for the next production deploy, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Dashboard: the zh "Slug" label and the CPU unit through i18n — **DONE** | 15m | — |
| [t002](done/t002.md) | Types and operator: the deploy reason sentences as coded templates — **DONE** | 40m | — |
| [t003](done/t003.md) | Backend: GraphQL `failureReasonTemplate` and `stallReasonTemplate` — **DONE** | 30m | t002 |
| [t004](done/t004.md) | Dashboard: known templates worded in en and zh on the deploy header, the Deploys list and Events — **DONE** | 40m | t003 |
| [t005](t005.md) | Live verification on an owned image service | 20m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 5m | t004 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t004 |
| [t008](done/t008.md) | Test coverage — **DONE** | 25m | t004 |
| [t009](t009.md) | Closeout | 10m | t005–t008 |

Nine tasks, 200m (3h20m).

## Source + Goal linkage

- **Source:** promoted from inbox note w4/566 (live `/qa-find-bugs w4` loop, rounds 76 and 77, 2026-11-11 UTC) on 2026-11-11. Wording the operator's reasons in zh needs coded templates in `lego/types`, the operator and bex-api writers, a GraphQL field and the dashboard, more than one sub-hour task.
- **Goal linkage:** the zh locale is bex's own promise (w5/m3).
- **Render parity:** n/a for the zh text; Render's dashboard is English only. REST and MCP keep the English sentences, as Render's API does; the GraphQL template fields are a bex extension.

## Residuals (filed)

- w4/591: the reason families this milestone does not code yet — health-check and progress-deadline stalls, the operator's pre-deploy timeout, "the build never started", the abandoned-deploy reason, kubelet image-pull messages and `cancelReason`.
- w4/589: `lego/types` tests run in no CI job, which is why this milestone's template tests live in `lego/backend/internal/gqlutil`.

## Finding (from w4/566)

Why: a Chinese-language owner reads their service header, Metrics limits and a deploy in progress in Chinese, except for a header label, a unit, and the one sentence that explains what the deploy is doing. w4/552 fixed the same class of problem for paused-plan reasons only.

- **Source / severity / estimate:** live `/qa-find-bugs w4` loop, round 76, 2026-11-11 05:23–05:31 UTC (`muse.env`, workspace `bex-canary`, pin `2f1c62c93f49`, whose code is the same as `a0d5802594e9`). **Minor** (i18n). ~40m for the three strings, plus ~30m if the `failureReason` families join (round 77).
- **Fixture:** Free image service `qa-20261110-zh76` (`srv-dbpvnosdb7tc73b8lb4g`, `traefik/whoami:v1.10.1`). Deleted afterwards (204, then 404).
- **Probe:** with `?lang=zh` (`<html lang="zh">`), I scanned `/services/<id>`, `/deploys`, `/events`, `/logs`, `/metrics`, `/env`, `/settings`, `/scaling` and `/plan`. I looked for text nodes and `aria-label`/`placeholder`/`title` values that had no CJK characters and contained English words outside a technical allowlist. Apart from user content (names, ids, images) and the known Pro Ultra reason (w4/552), the remaining English was:
  1. **"Slug"**: the service header's `<dt>`, on every tab ("Slug | qa-20261110-zh76"). The neighbouring terms are translated (实例数, 修订, 创建时间).
  2. **"0.100 cores"**: Metrics → CPU → "限额 0.100 cores | 0.2%".
  3. **"Detected HTTP on port 80; serving it"**: while the first deploy was in progress, both on the Deploys list row (`进行中 | dep-dbpvnosdb7tc73b8lb50 | Detected HTTP on port 80; serving it | 6 分钟前创建`) and on Events → 部署已开始. It disappeared once the deploy was `live`; GraphQL `Deploy` has `stallReason`, `failureReason` and `cancelReason`.
- **Root cause:**
  - **The zh "Slug" string is English.** `dashboard/src/features/services/locales/zh.ts:256` sets `services.colSlug` to `"Slug"`, rendered at `components/service-detail-header.tsx:652`.
  - **The CPU unit is hard-coded.** `dashboard/src/features/metrics/lib/format.ts:8` returns `` `${…} cores` ``, outside i18n. `formatMetricValue` has 4 callers in components.
  - **The port-detection reason is English free text.** The operator writes it in `lego/operator/internal/controller/web_image_http.go:162`, alongside its siblings at `:152` and `:167-172`: "Port %d accepted a connection but has not answered HTTP yet…", "Port detection could not reach the instance…", "No port answers HTTP yet…", "The detected HTTP port exceeds the service's 75-port limit…".
    - It reaches the dashboard as GraphQL `Deploy.stallReason` (`lego/backend/internal/deploys/graphql.go:70`).
    - The dashboard renders it verbatim at `features/deploys/components/deploys-list-page.tsx:264` and `deploy-header.tsx:126`, through `DeployFailureReason`, and in the Events row.
    - The same text is also written to the deploy log (`lego/backend/internal/logs/progress.go:303`). That copy is log content and stays English.
- **Fix (target behavior):**
  - **"Slug" is translated in zh.** Use whatever Chinese term the team prefers for the platform-subdomain segment, for example "标识（Slug）".
  - **The CPU unit is localized.** It reads "0.100 核" in zh and "0.100 cores" in en, through an i18n key; the four callers pass the active language.
  - **Port-detection reasons render in the active language.** The backend adds a machine-readable reason code with parameters next to the English `stallReason`, for example `stallReasonCode: "http_port_detected"` with `port: 80`, plus codes for the four siblings. The dashboard maps known codes to en and zh strings and falls back to the server's English text for an unknown code.
    - REST and MCP keep their English text, as Render's API does.
    - Expose the code on GraphQL first; it is a bex extension.
- **Tests:**
  - A zh render test for the header, the Metrics CPU limit and a deploy row with `stallReasonCode`.
  - An unknown code falls back to the English text.
- **`failureReason` is the same class (round 77, 2026-11-11 05:38–05:41 UTC):**
  - Fixture: Free image service `qa-20261110-fr77` (`srv-dbpvufik10fs73dvo700`) created with the nonexistent tag `docker.io/traefik/whoami:v0.0.0-qa-does-not-exist`. Deleted afterwards (204, then 404).
  - The deploy became `update_failed` with `failureReason` "the image docker.io/traefik/whoami:v0.0.0-qa-does-not-exist was not found in its registry: the repository or tag does not exist. Check the image URL and tag, or deploy a digest that exists". That text is composed at `lego/operator/internal/controller/image_network.go:549`.
  - With `?lang=zh` the sentence shows verbatim on the service overview, the Deploys row and the Events row.
  - So the reason-code approach should cover `failureReason` too. At least the operator-composed image-pull, health-check-timeout and port-detection families need codes, keeping the English text as the fallback.
  - `cancelReason` ("Superseded by dep-…") was not scanned. Decide whether the remaining families belong here or in a follow-up.
- **Unverified / scope note:**
  - Static site, Postgres and Key Value pages were not scanned this round.
- **Render:** n/a. Render's dashboard is English-only; bex's zh locale is its own promise (w5/m3).
- **Dedupe:**
  - w4/552 (open) covers paused-plan reasons only, from `pausedTierMessage`/`pausedPlanMessage`. Its scan of the deploys page found only commit messages, because no image deploy was mid-detection then.
  - `grep -rn "Detected HTTP on port\|colSlug\|cores" .pm` finds no open item.
  - Not deploy lag: `git log a0d5802594e9..origin/main -- dashboard/src lego/operator/internal/controller/web_image_http.go` is empty.
- **DoD (live):**
  - With `?lang=zh`, an owned Free image service's header shows the Chinese slug label.
  - Metrics shows "限额 0.100 核".
  - During the first deploy, the Deploys row and the Events "部署已开始" row show the port-detection message in Chinese.
  - With `?lang=en` the same places read exactly as today.

- **Addendum (QA loop round 146, 2026-11-11, pin `8330744bd643`):** one more member of the same port-detection family stays English. `/services/<id>/deploys?lang=zh` on a fresh Free image service (`docker.io/traefik/whoami:v1.10.1`) showed `paragraph: Waiting for the container to start before detecting its HTTP port` during its first deploy. It is written at `lego/operator/internal/controller/web_image_http.go:132`. Give it a reason code with the siblings above.
