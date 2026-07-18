# w9 · m44 — Sample lifecycle verification on prod: deploy → up → suspend → down → resume → up → delete → gone, for every `examples/` sample, via the official Render CLI

**Worker:** worker9 **Goal:** every sample in `examples/` provably completes the full Render lifecycle on production through the official Render CLI — deploy reaches `live` and serves at `*.onbex.co`, suspend takes it down, resume brings it back, delete removes it without residue — and every problem the sweep exposes is fixed or filed **Status:** in progress — sweep complete, 5/7 GREEN on prod; hello-python + static-site fixed & validated locally, awaiting `/ship` (their Dockerfile fixes must reach `origin/main` before the prod re-run); pipeline-green + Argo re-enable are the user's `/ship` + prod-cluster steps

## Tasks (in order)

| id   | title                                                                                        | est | depends_on       | status |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------- | ------ |
| t001 | Unblock the deploy pipeline: fix the red dashboard test, land the m43 digest, re-enable Argo | 45m | —                | ◐ dashboard test fixed (1339 green); ship + Argo = user's `/ship` |
| t002 | Lifecycle harness: `scripts/samples-lifecycle.sh` (CLI-first, REST where the CLI lacks verbs) | 45m | t001             | — **DONE** (hello-go green on prod) |
| t003 | Web-service samples on prod: hello-go, hello-node, hello-python, whoami (image)              | 45m | t002             | ◐ hello-go/node/whoami GREEN; hello-python fixed, needs push + re-run |
| t004 | Special-type samples on prod: static-site, cron-demo, stack-demo                             | 60m | t002             | ◐ cron-demo/stack-demo GREEN; static-site fixed, needs push + re-run |
| t005 | Fix problems the sweep found (small: in-milestone; large: file follow-ups)                   | 60m | t003, t004       | — **DONE** (4 fixes; follow-ups `.pm/w9/010`, `011` filed) |
| t006 | Render parity: any fixes must land consistently on REST/GraphQL/MCP/dashboard                | 30m | t005             | — **DONE** (no bex-api/dashboard surface changed; product gaps filed w/ Render comparison) |
| t007 | Simplify: `/simplify` over the code this milestone changed                                   | 20m | t006             | — **DONE** (`json_find_id` consolidation; micro-fetch/reuse findings skipped w/ reason) |
| t008 | Test coverage: harness wired as a rerunnable guard + tests for shipped fixes                 | 30m | t006             | — **DONE** (`all` mode + cli-compat checklist section; dashboard fixes carry tests) |
| t009 | Closeout                                                                                     | 10m | t008             | pending — gated on DoD (7/7 green + pipeline + Argo) |

## Progress (2026-07-17)

Prod sweep executed against `api.bex.co` via `scripts/samples-lifecycle.sh` (user-authorized). Each leg is CLI-first; service suspend/resume use raw REST (the CLI lacks them). verify-down requires the URL to stop serving for 3 consecutive checks (a transient 502 during the suspend reconcile must not count as "down"); verify-up asserts a unique per-run token.

**GREEN on prod (5/7):** hello-go, hello-node, whoami, cron-demo, stack-demo. Prod verified clean after (no residue; only pre-existing user resources remain).

**Problems found → fixed (t005):**

- **hello-python** — Dockerfile ran `flask run` with no `FLASK_APP` → container crash-loop (`Could not locate a Flask application`), deploy `update_failed`. Fixed → `CMD ["python", "main.py"]`; docker build+run validated locally. Needs push + prod re-run.
- **whoami** — bound `:80` → `bind: permission denied` (tenant pods drop ALL caps incl. NET_BIND_SERVICE, w7/m2; and whoami ignores `$PORT` while bex routes an image service to 3000). Fixed harness spec + `examples/whoami-app.yaml` (`WHOAMI_PORT_NUMBER`, high port) → GREEN on prod. Broader "no listening-port auto-detection" gap filed `.pm/w9/011`.
- **static-site** — `--type static_site` went down the Docker build path and failed (`no Dockerfile`); bex's static build is Dockerfile-only (ADR029) but the sample had none and its `bex.yml` over-promised "no Dockerfile needed". Stopgap: added a minimal Dockerfile + honest comment; extract-simulation validated locally. Real no-Dockerfile-publish fix filed `.pm/w9/010`. Needs push + prod re-run.
- **cron-demo** — Render CLI requires `--cron-command` even for an image-entrypoint cron. Fixed harness → GREEN (run → no-run-while-suspended → run-again, proven via logs).

**Blocked on the user (`/ship` + Argo):** the two remaining prod re-runs (hello-python, static-site) need their Dockerfile fixes on `origin/main`; the deploy-pipeline-green + m43-digest-pin come from that same `/ship`; Argo re-enable on `bex-operator` + `bex-platform-prod` needs the Hetzner prod kubeconfig (runbook `w9/done/m43/done/t004.md`). After `/ship`: re-run the two samples → 7/7, confirm `deploy.yml` green + operator on the CI digest + Argo held, then t009 closeout.

## Definition of done

One documented green sweep against production: for each of `examples/hello-go`, `hello-node`, `hello-python`, `whoami-app.yaml`, `static-site`, `cron-demo`, and `stack-demo`, the lifecycle runs hands-free via the official Render CLI (raw Render-compatible REST only for verbs the CLI lacks, noted per leg) — deploy reaches `live`/scheduled, the URL-bearing samples serve their expected body at `https://<name>.onbex.co`, suspend stops serving (and stops cron runs), resume restores it, delete removes the service with no leftover cluster resources (pods, Secrets, htpasswd users) — with the deploy pipeline green first (dashboard test fixed, CI-pinned m43 operator digest live, Argo automation re-enabled). Problems found are fixed in-milestone or filed as concrete follow-ups; none silently skipped.

## Source + Goal linkage

- **Source:** user request 2026-07-17 (`/pm for w9`) immediately after w9/m43 shipped: "after deployment use render cli to deploy, verify up with \*.onbex.co, then suspend/verify down, resume/verify up, delete/verify down for all samples in examples dir. fix problems if there are." Session context: the m43 prod verification only exercised hello-go create+delete; the deploy pipeline is currently red on a pre-existing dashboard test failure (5 consecutive `deploy.yml` failures since 2026-07-17T19:35Z, `build-and-deploy` skipped), and Argo automation on `bex-operator`/`bex-platform-prod` is still suspended from m43's manual rollout (`w9/done/m43/done/t004.md`).
- **Goal linkage:** Render parity (ADR018) and the core product promise (ADR008): the samples are the first-run funnel — if any lifecycle leg fails on prod, real users hit it first. ADR007 (restart/suspend/resume) and ADR029 (static sites) get their first systematic prod exercise.
- **Expected outcome:** a rerunnable lifecycle guard (`scripts/samples-lifecycle.sh`) plus a green prod sweep across all seven samples; the deploy pipeline green again; the m43 drift (suspended Argo automation) resolved.
- **Why now:** m43 just proved first-deploys work but nothing exercises suspend/resume/delete on prod; the pipeline being red blocks every future ship, so t001 is urgent independent of this milestone; buildpack-era samples (hello-node/python) and the special types (static, cron, stack) have plausibly never run the full lifecycle on the Hetzner cluster.
- **Render parity task included:** t005's fixes may touch tenant-facing suspend/resume/delete surfaces; t006 checks any such fix across REST/GraphQL/MCP/dashboard against render.com behavior.
