# w3 · m368 — A dead alert pipeline or a public outage reaches a maintainer within minutes, not the 5–9 h GitHub's scheduler leaves

**Worker:** worker3 **Goal:** an external dead-man's switch hears from Alertmanager's `Watchdog` every minute and alerts when it stops, and external HTTP checks watch the public endpoints every few minutes. The documented detection time becomes a measured one. The schedule-drift check stops reporting a false "never" for push-heavy workflows. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Decision and design: the external monitor (recommended: a hosted dead-man's switch plus HTTP checks), its account and where its URLs live | 20m | — |
| [t002](t002.md) | Alertmanager routes `Watchdog` to a webhook receiver that pings the dead-man's switch at least every minute; URL from an out-of-band Secret | 60m | t001 |
| [t003](t003.md) | External HTTP checks of `api.bex.co`, `dashboard.bex.co` and a tenant `onbex.co` host at ≤ 5-minute intervals, as code or documented configuration | 45m | t001 |
| [t004](t004.md) | `ci-schedule-drift.sh` reads enough history for push-heavy workflows; a fixture reproduces #112's false "never" | 45m | — |
| [t005](t005.md) | Drill: stop the feed and record time to notification; ADR010 and ADR088 state the measured bound | 40m | t002, t003 |
| [t006](t006.md) | Simplify | 15m | t004, t005 |
| [t007](t007.md) | Test coverage | 30m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

- Over 24 hours, the dead-man's switch records production `Watchdog` pings no more than 2 minutes apart.
- A drill that stops the pings (a temporary check, or a paused dev route; never by stopping production Alertmanager) produces a notification to the maintainer inside the configured grace period, recorded with times.
- HTTP checks of the three public endpoints run at ≤ 5-minute intervals and alert on failure.
- `scripts/ci-schedule-drift.sh` no longer reports `deploy.yml` as "never"; its test fixture with one scheduled run among 100 pushes passes; #112 is closed or narrowed to real lateness.
- ADR010 § Alerting heartbeat and ADR088 §1 state the measured detection bound instead of "hourly".

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-12, item 4, filed by `/pmx them all to w3`. Re-checked 2026-11-12 against `origin/main` `24bea6c5c`.
- **Evidence:**
  - ADR088 §1 (`docs/ADR088-platform-observability-ui.md:32,229`) names `.github/workflows/alerting-heartbeat.yml` "hourly" and the whole-cluster-outage detector. ADR010 § Alerting heartbeat (`docs/ADR010-observability.md:533-540`) says the same.
  - Scheduled runs of that workflow (`gh run list --workflow alerting-heartbeat.yml --event schedule`) on 2026-11-11 to 11-13 UTC: 15:24, 22:47, 03:54, 12:54, 20:56, 01:56, i.e. 5–9 hours apart. GitHub runs schedules best-effort, and the 2026-11-09 move to hosted runners (DO_NOT_DO #CI-RUNNERS) left no other external probe.
  - `scripts/ci-schedule-drift.sh` gives the heartbeat a 12 h window, so a dead Prometheus/Alertmanager or an edge outage can go unseen for about 12 hours.
  - `Watchdog` is routed to the `null` receiver, "read off-cluster by alerting-heartbeat.sh" (`deploy/gitops/base/prometheus.yaml:221-226`).
  - Issue #112 (open) reports `deploy.yml` scheduled runs as "never": `runs_for` reads one page of 100 runs (`ci-schedule-drift.sh:161-176`), and `deploy.yml` runs on every push, so 100 runs hold about one scheduled run.
  - The 2026-08-16 outage (`beancount-cms-v2.onbex.co` and the proxy in front of it served 404 for hours; DO_NOT_DO #PSL) shows silent failures do last hours here.
- **Goal linkage:** ADR008 roadmap item 2 and "Ops hardening" in ADR008 What's next; ADR010's rule that the pipeline must never be its own only witness.
- **Expected outcome:** the maintainer learns of a whole-cluster or alert-pipeline outage in minutes. The heartbeat workflow stays as a second, slower layer.
- **Why now:** detection got slower on 2026-11-09 and nothing replaced the cadence. w7/055 deferred an in-cluster trigger with "worsening drift can reopen that decision"; #112 is that drift. This is independent of w3/m174's blocked drill.
- **Open decision, recommendation only (not a user decision):** t001 recommends (A) a hosted dead-man's switch with HTTP checks, e.g. healthchecks.io (open source, so it can later be self-hosted outside the cluster), which is the kube-prometheus documented use of `Watchdog`. Alternatives: (B) a Cloudflare Worker cron that dispatches the heartbeat workflow every 10 minutes (no new vendor; needs a Workers token and a fine-grained PAT); (C) documentation only, accepting about 12 hours (shrinks the DoD and needs the user's sign-off). The account and its URLs need the user (t001); t004 can proceed without them.
- **Render parity omitted:** platform-internal operations. No REST, GraphQL, MCP or dashboard surface changes.
