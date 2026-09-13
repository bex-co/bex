# w4 · m100 — Stop bex-api shedding a legitimate dashboard page load, and record the running commit on config-change deploys

**Worker:** worker4 **Goal:** one signed-in user opening one dashboard page never receives a `429 RATE_LIMITED`, and a deploy row opened by a configuration save names the commit that is actually running. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------ | --- | ------------------ |
| t001 | Give valid-credential concurrency its own budget, separate from the amplification bound | 60m | —                  |
| t002 | Collapse the Metrics page's per-chart GraphQL fan-out into few round trips             | 60m | —                  |
| t003 | Surface `RATE_LIMITED` instead of silently rendering a chart with missing series       | 40m | w4/m100/t001       |
| t004 | Carry the running release's commit into `config_change` deploy rows                    | 50m | —                  |
| t005 | Blast-radius + control-case regression tests for both budgets and all rollout callers  | 50m | w4/m100/t001, w4/m100/t004 |
| t006 | Render parity sweep over the changed surfaces                                          | 30m | w4/m100/t002, w4/m100/t003, w4/m100/t005 |
| t007 | Simplify pass over this milestone's changes                                            | 30m | w4/m100/t006       |
| t008 | Test coverage for the shipped behavior                                                 | 40m | w4/m100/t006       |
| t009 | Closeout                                                                               | 15m | w4/m100/t008       |

## Definition of done

Each bullet is a command or a click the next person can repeat against production and watch succeed.

- **Concurrency by a valid credential is no longer the scarce resource.** This is the load-bearing bullet — verify it first. From one signed-in page, with the caller's rate bucket verified healthy (90 sequential `GET https://api.bex.co/v1/services?limit=1` → 90 × 200), `Promise.all` of 25 identical `Metrics` GraphQL POSTs returns **25 × 200**, and 40 returns **40 × 200**. Measured twice on two different services and at two different times: 2026-09-13 pass 1 on `srv-daj6sj0gsm7s73f63nmg` → ≥1 × 429 of 25; pass 8 on `srv-d9ndt8hmcglc739fkp50` → **5 × 429 of 25** and **17 × 429 of 40**, each body `{"data":null,"errors":[{"extensions":{"code":"RATE_LIMITED"},"message":"rate limit exceeded"}]}` with `Retry-After: 1`.
- **A dashboard page load produces zero 429s — on a service with more than one instance.** With the bucket verified healthy as above, loading `https://dashboard.bex.co/services/<srv-id>/metrics` once yields **0** `RATE_LIMITED` responses in `browser_network_requests`.

  ⚠️ **This bullet is not discriminating on a single-instance service — do not accept the milestone on it alone.** Pass 8 re-probed it and a 1-instance idle service (`eden-dash-v3`) already yields **0 × 429 out of 23 POSTs today, with no fix**, while pass 1's sheds (2–8 × 429 out of 25–34 POSTs) happened on a service that had **2 instances** mid-rollout. Every pre-existing service in the workspace is 1-instance (checked: `eden-dash-v3`, `beancount-forum`, `agentmarketcap-1`, `beancount-cms-v2`, `eden-cms-v2` — all `replicas: 1`), which is why the symptom looked intermittent. The per-instance metric fan-out appears to scale with instance count and to cross the bound at 2+; t002 should confirm that relationship while it measures the fan-out, and whoever verifies this bullet must use a ≥2-instance service (or a service mid-rollout) or rely on the concurrency bullet above instead.
- **The anti-amplification guarantee of `w1/m67` t002 still holds.** A flood of unique *invalid* bearers from one source is still shed with the upstream introspection/whoami call count asserted bounded, and oversized credentials are still refused before any upstream call — the existing `w1/m67` tests stay green unchanged.
- **The Metrics page costs few round trips, and the cost does not grow with instance count.** One load of `/services/<srv-id>/metrics` issues a single-digit number of GraphQL POSTs, and that number is the same for a 1-instance and a 3-instance service. Today: **23** POSTs for a 1-instance service (pass 8, `eden-dash-v3`) and **25–34** for a service with 2 instances mid-rollout (pass 1), with duplicate `Metrics(INSTANCES)` and `MetricsFilters(STATUS_CODE)` operations issued more than once per load.
- **A rate-limited metrics read is visible.** With a `RATE_LIMITED` response forced for one chart's query and no cached series to fall back on, that chart renders an explicit degraded state naming throttling — not an empty or zero-valued chart, and not a generic error card that reads as "metrics unavailable" (which would misattribute a transient shed to an unwired backend).
- **A config-change deploy names the code it runs.** Save an environment-variable change on a repo-backed service, then `GET /v1/services/<srv-id>/deploys?limit=2`: the new `trigger: "config_change"` row carries a `commit` object whose `id` equals the `commit.id` of the deploy it superseded, and `/services/<srv-id>/deploys` renders the short SHA on that row. Today the row has no `commit` key at all while the superseded `trigger: "create"` row has `commit.id 5ef5e18…`.
- **Image-backed services stay honest.** The same save on an image-backed service still produces a `config_change` row with **no** `commit` key — the fix carries a commit forward, it never synthesizes one.
- **All four rollout callers are covered.** A regression test exists per `rollout.Tracker` caller (`secrets/service.go:95`, `secrets/batch.go:283`, `envgroups/service.go:1115`, `apps/service.go:4006`), including the callers that behave correctly today.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-13 UTC, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`, against production `api.bex.co`. Throwaway service `qa-20260913-web` (`srv-daj6sj0gsm7s73f63nmg`, repo `github.com/bex-co/bex`, root `examples/hello-go`, docker runtime) created and deleted within the run. Evidence is the probe transcripts pasted into t001 and t004 — re-runnable requests and complete responses — plus gitignored console captures under `.playwright-mcp/` (`console-2026-09-13T09-40-07-500Z.log`, `console-2026-09-13T09-43-13-269Z.log`, `console-2026-09-13T09-52-48-439Z.log`).
- **This overturns a recorded non-defect judgment — see `w6/028`.** That note (2026-08-22, closed 2026-08-27) measured the same Metrics fan-out (17–20 GraphQL POSTs per load) and classified it *"efficiency, not correctness"* on the premise that _"steady-state polling stays well under the 500/min `BEX_RATE_LIMIT`"_, and separately recorded that _"the 429s seen mid-hunt were self-inflicted by the sweep's own navigation rate, not reachable by a real user's pace."_ Both halves of that premise are disproved by this run: the shed is **not** rate-driven (90 sequential requests after a 90s idle → 0 × 429) and **is** concurrency-driven (25 parallel requests → 429), and the budget that sheds is not `BEX_RATE_LIMIT` (500 rpm, burst 500 — `api/ratelimit.go:68-77`, `core/ratelimit.go:77-90`) but the auth-admission per-credential in-flight bound `credentialMaxInflight = BEX_AUTH_MAX_INFLIGHT/8 = 8` (`api/authadmission.go:108-113`, `:175-184`), which answers with the byte-identical envelope via `writeAuthOverloaded` → `writeTooManyRequests` (`authadmission.go:212-215`). `w6/028` is not re-filed and not reopened; this milestone supersedes its disposition with the measurement it lacked.
- **Goal linkage:** ADR008 pillar 1 (a hosting product a paying customer can actually use) and `docs/ADR006-bex-api.md` §Rate limits — a budget meant to stop anonymous amplification of Ory must not shed the first-party dashboard's own reads. `docs/ADR004-app-deployment.md` + `docs/ADR018-render-parity.md` for the deploy record: Render's deploy object carries `commit { id, message, createdAt }` for every Git-backed deploy (`docs/render-artifacts/deploy-detail-page.md:10,17`), so a bex deploy row with no commit is a parity gap as well as a product one.
- **Expected outcome:** the Metrics tab stops dropping reads for every signed-in user, the failure that remains is visible rather than silent, and deploy history can answer "what code is live?" after a configuration save — which is exactly the question a rollback decision turns on.
- **Why now:** the throttle is on the default path of an ordinary page load by an ordinary user (no flood, no automation), it degrades silently, and its non-defect disposition is on record — so it will keep being re-observed and re-dismissed until the real budget is named. The deploy-commit gap rides along because it was found in the same journey, touches the same deploy surface the Render-parity closing task already has to sweep, and is a small bounded change at one line.
- **Render parity task included:** yes — REST, GraphQL and MCP all read the same `deploys` row, and the dashboard renders the metrics degraded state and the deploy commit.

## Re-probe log

- **2026-09-13 pass 8 (independent re-confirmation).** The concurrency mechanism reproduced on a different service at a different time: 25 parallel identical `Metrics` queries → 5 × 429; 40 parallel → 17 × 429, after 90 sequential requests returned 90 × 200. The **page-load** symptom did **not** reproduce: one metrics load on the 1-instance `eden-dash-v3` issued 23 POSTs and shed none. The DoD above was corrected accordingly — the concurrency bullet is now the load-bearing one and the page-load bullet carries an explicit warning that it passes today on a single-instance service. `w6/028`'s disposition remains overturned: it attributed the sheds to `BEX_RATE_LIMIT` and to sweep pace, and both remain disproved (rate-driven shedding is absent at 90 sequential requests; concurrency-driven shedding is present twice).

### Bearer control case — settled 2026-09-13 pass 13

`t001` step 1 asked for this and it comes out as the code predicts, which strengthens the filing rather than correcting it. An API key was minted, exchanged for an access token, and used against the identical endpoint:

| caller | 90 sequential | 25 parallel | 40 parallel |
| --- | --- | --- | --- |
| **bearer** (API-key token) | 0 × 429 | **0 × 429** | **0 × 429** |
| **session cookie** (pass 8) | 0 × 429 | 5 × 429 | 17 × 429 |

Same endpoint, same workspace, same machine. So the shed is specific to the **session** credential class — exactly what `auth.go:572-575` predicts, since `introspect` positively caches while `whoami` deliberately does not. t001's fix should therefore target the session path or the caching asymmetry, not the in-flight bound globally. The key was revoked at the end of that run; full journey in `w4/m105`.

## Unverified this run

Carried from the findings so nothing inferred arrives as something observed:

- The **control case** for the in-flight bound — that a bearer/API-key caller is unaffected because `introspect` positively caches while `whoami` deliberately does not (`api/auth.go:572-575`, "Kratos sessions are not positively cached here, so every session request is an upstream call") — is read from code only. No API key was minted and no bearer-authenticated concurrency probe was run. t001 must probe it.
- The exact concurrency threshold was not bisected. 25 parallel requests produced 1 × 429 and 30 parallel REST requests produced 0, which is consistent with a bound of 8 plus browser/edge serialization, but the observed simultaneity at the server was never measured. The mechanism (only concurrency-keyed budget in the gate, identical response envelope, `Retry-After: 1`) is what pins the cause, not the count.
- The **source of the carried-forward commit** in t004 is not settled. `a.Spec.BuildCommit` was considered and **rejected**: `lego/types/v1alpha1/app_types.go:421-433` states the subsequent deploy always resets it to empty, so it is not a record of the running release. t004 must pick between a store read and inheritance inside `PGStore.CreateDeploy`, and must verify whichever it picks against a real repo-backed service.
- Other hosting journeys were not exercised this run and are not asserted anywhere above: Shell/SSH, static-site redirects and header rules, cron jobs, background and private services, Postgres and Key Value lifecycles, blueprint sync apply, free-tier sleep/wake, and rollback execution (the Rollback action was observed as offered on the deactivated row, never clicked).
