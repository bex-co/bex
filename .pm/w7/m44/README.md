# w9 · m44 — Sample lifecycle verification on prod: deploy → up → suspend → down → resume → up → delete → gone, for every `examples/` sample, via the official Render CLI

**Worker:** worker9 **Goal:** every sample in `examples/` provably completes the full Render lifecycle on production through the official Render CLI — deploy reaches `live` and serves at `*.onbex.co`, suspend takes it down, resume brings it back, delete removes it without residue — and every problem the sweep exposes is fixed or filed **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Unblock the deploy pipeline: fix the red dashboard test, land the m43 digest, re-enable Argo | 45m | —                |
| t002 | Lifecycle harness: `scripts/samples-lifecycle.sh` (CLI-first, REST where the CLI lacks verbs) | 45m | t001             |
| t003 | Web-service samples on prod: hello-go, hello-node, hello-python, whoami (image)              | 45m | t002             |
| t004 | Special-type samples on prod: static-site, cron-demo, stack-demo                             | 60m | t002             |
| t005 | Fix problems the sweep found (small: in-milestone; large: file follow-ups)                   | 60m | t003, t004       |
| t006 | Render parity: any fixes must land consistently on REST/GraphQL/MCP/dashboard                | 30m | t005             |
| t007 | Simplify: `/simplify` over the code this milestone changed                                   | 20m | t006             |
| t008 | Test coverage: harness wired as a rerunnable guard + tests for shipped fixes                 | 30m | t006             |
| t009 | Closeout                                                                                     | 10m | t008             |

## Definition of done

One documented green sweep against production: for each of `examples/hello-go`, `hello-node`, `hello-python`, `whoami-app.yaml`, `static-site`, `cron-demo`, and `stack-demo`, the lifecycle runs hands-free via the official Render CLI (raw Render-compatible REST only for verbs the CLI lacks, noted per leg) — deploy reaches `live`/scheduled, the URL-bearing samples serve their expected body at `https://<name>.onbex.co`, suspend stops serving (and stops cron runs), resume restores it, delete removes the service with no leftover cluster resources (pods, Secrets, htpasswd users) — with the deploy pipeline green first (dashboard test fixed, CI-pinned m43 operator digest live, Argo automation re-enabled). Problems found are fixed in-milestone or filed as concrete follow-ups; none silently skipped.

## Source + Goal linkage

- **Source:** user request 2026-07-17 (`/pm for w9`) immediately after w9/m43 shipped: "after deployment use render cli to deploy, verify up with \*.onbex.co, then suspend/verify down, resume/verify up, delete/verify down for all samples in examples dir. fix problems if there are." Session context: the m43 prod verification only exercised hello-go create+delete; the deploy pipeline is currently red on a pre-existing dashboard test failure (5 consecutive `deploy.yml` failures since 2026-07-17T19:35Z, `build-and-deploy` skipped), and Argo automation on `bex-operator`/`bex-platform-prod` is still suspended from m43's manual rollout (`w9/done/m43/done/t004.md`).
- **Goal linkage:** Render parity (ADR018) and the core product promise (ADR008): the samples are the first-run funnel — if any lifecycle leg fails on prod, real users hit it first. ADR007 (restart/suspend/resume) and ADR029 (static sites) get their first systematic prod exercise.
- **Expected outcome:** a rerunnable lifecycle guard (`scripts/samples-lifecycle.sh`) plus a green prod sweep across all seven samples; the deploy pipeline green again; the m43 drift (suspended Argo automation) resolved.
- **Why now:** m43 just proved first-deploys work but nothing exercises suspend/resume/delete on prod; the pipeline being red blocks every future ship, so t001 is urgent independent of this milestone; buildpack-era samples (hello-node/python) and the special types (static, cron, stack) have plausibly never run the full lifecycle on the Hetzner cluster.
- **Render parity task included:** t005's fixes may touch tenant-facing suspend/resume/delete surfaces; t006 checks any such fix across REST/GraphQL/MCP/dashboard against render.com behavior.
