# w4 · m202 — Honor image commands after switching from native Git

**Worker:** worker4 **Goal:** a prebuilt image executes its saved Docker Command after a native Git source switch **Status:** blocked (release pipeline deploys the operator; then QA replays the DoD for t006). t001–t005 done.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Apply prebuilt command overrides independently of retained build metadata — **DONE** | 45m | — |
| t002 | Audit shared command consumers and preserve selected native artifacts — **DONE** | 35m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 35m | t003 |
| t006 | Closeout | 20m | t004, t005 |

## Definition of done

- Repeat the [recorded Node web-service journey](finding.md): create an owned Free public-Git Node service, cancel its queued initial build, use Settings → Source → Existing Image to save BusyBox 1.37, and save the exact recorded Docker Command. The carrying release becomes Live and the public URL returns HTTP 200 with `qa-a311-recovered\n`; the saved command is executed.
- Repeat from a fresh 390px deploy page using Cancel and Manual Deploy → Deploy latest image. The manual release also becomes Live and serves the exact marker. Preserve the first canceled build and the intervening canceled release in history; do not label a release Live before readiness.
- Repeat the image-created control with the same image, command, port and environment value. It remains Live and serves the same marker. Source saves continue to stage without a deploy, and command saves retain their existing explicit rollout behavior.
- Remove the exact owned fixtures, verify detail/public 404s and all seven inventory ID sets, revoke only the hunt session, and record runtime observations beside the exact requests and complete responses.
- The source-only cron/buildpack/private/worker and native-artifact boundaries are verified through t002/t005; their current live behavior is not asserted by this hunt.

## Source + Goal linkage

- **Source:** user-requested continuous functional `qa-find-bugs`, w4, 2026-10-12 Pacific / 2026-10-13 UTC pass a311. [Finding and complete wire proof](finding.md).
- **Goal linkage:** ADR008 core hosting; ADR004 application deployment; ADR026 saved source transitions; ADR018 Docker Command parity.
- **Expected outcome:** switching a service to a prebuilt image no longer silently discards an accepted command because its former source was native or buildpack.
- **Why now:** a command-save release and a separate fresh manual retry both remain in a startup loop with HTTP 503, while a separately created Free image control with the identical command reaches Live/HTTP 200.
- **Sizing:** 170m across command projection, selected launch-mode transport, shared consumers, parity, simplification, behavioral coverage and live closeout.
- **Standing closing tasks:** Render parity, Simplify, Test coverage and Closeout are included because the operator's command behavior is visible through the dashboard and API-configured hosting journeys.
- **Scope:** filing only. Preserve saved native settings, source-save semantics, existing runtime-read work in w8/088 and command clearing in w4/m194. No paid changes or product implementation in this hunt.
