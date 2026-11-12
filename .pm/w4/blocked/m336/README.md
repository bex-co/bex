# w4 · m336 — In the zh dashboard, the deploy reasons w4/m329 does not code stay English

**Worker:** worker4 **Goal:** the health-check stall, the operator's pre-deploy verdicts, a build that never started, an abandoned deploy and every fixed cancel reason read in the dashboard's language on the deploy header, the Deploys list and Events, while REST and MCP keep Render's English byte for byte. **Status:** blocked — t001–t004 and t006–t008 done (shipped); t005 live replay waits for the next production deploy, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Types: 25 templates, a hint tail for the probe stall, token parameters — **DONE** | 45m | — |
| [t002](done/t002.md) | Writers: the operator and bex-api format through the templates — **DONE** | 45m | t001 |
| [t003](done/t003.md) | API and dashboard: `cancelReasonTemplate`, the zh and en catalogs, parameter wording — **DONE** | 40m | t002 |
| [t004](done/t004.md) | Docs — **DONE** | 5m | t003 |
| [t005](t005.md) | Live verification in zh on owned services | 20m | t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 5m | t003 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t004 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t004 |
| [t009](t009.md) | Closeout | 5m | t005–t008 |

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
