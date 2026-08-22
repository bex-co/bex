---
name: qa-find-bugs
description: >-
  Drive the live product at https://dashboard.bex.co as a signed-in QA user (QA_EMAIL / QA_PASSWORD from .env), hunt real bugs across the hosting features, research each fix down to file:line, file the non-duplicate findings to the w6 board through /pm, and /ship the scheduled milestone. Use when the user asks to QA the dashboard, try the product and find bugs, or run a live hosting bug hunt.
---

# Task: live QA hunt of bex hosting → researched fixes → `/pm` filing → `/ship`

Use the product the way a paying customer would, on **production** (`https://dashboard.bex.co`), find bugs in the **hosting features**, work out the actual fix in this repo, file only what is not already filed, and land the board entry.

Parse `$ARGUMENTS`:

- `wN` — target workstream for the filing. **Default `w6`.**
- surface names (`services`, `deploys`, `env`, `domains`, `scaling`, `logs`, `metrics`, `shell`, `static`, `cron`, `databases`, `keyvalue`, `blueprints`, `projects`) — restrict the sweep. Default: the whole hosting sweep in Phase 2.
- `DRY_RUN=1` — run Phases 0–5 and report; write nothing to `.pm/`, do not `/ship`.

## Phase 0 — Preflight

1. `git rev-parse --abbrev-ref HEAD` must be `main`; if not, STOP and ask (Phase 7 ships).
2. `git status --porcelain` — record what was already dirty. Phase 7 must **not** sweep it in.
3. Confirm the Playwright MCP browser tools are available. Screenshots go to `.playwright-mcp/` (configured `--output-dir`) — always pass **bare filenames**, named `qa-<surface>-<n>.png`.
4. **Never read `.env` yourself.** `Read(.env)` is denied by project policy, and the QA password must never reach the transcript, a scratch file, a screenshot, a `.pm` note, or a commit.

## Phase 1 — Sign in without ever seeing the password

`scripts/qa-login.sh` does the whole login. It reads `QA_EMAIL`/`QA_PASSWORD` from `.env` inside its own process, completes the Kratos password flow, and hands back only cookies — the password never reaches your context, a file, or a tool call. **Never** read `.env` yourself (`Read(.env)` is denied by project policy), and never type a password into `browser_type`.

1. Get a session, preferring the one-shot loopback form:

   ```bash
   bash scripts/qa-login.sh --serve   # prints: ok http://127.0.0.1:<port>/<token>.json
   ```

   It keeps the session state in memory and serves it **once**, on loopback, at an unguessable path, then exits. Nothing lands on disk.

2. Inject it into the browser with `browser_run_code_unsafe`, naming only the loopback URL:

   ```js
   async (page) => {
     const res = await page.request.get("http://127.0.0.1:<port>/<token>.json");
     if (!res.ok()) return "FETCH_FAILED " + res.status();
     const state = await res.json();
     await page.context().addCookies(state.cookies);
     await page.goto("https://dashboard.bex.co/");
     return page.url();
   };
   ```

   `browser_run_code_unsafe` **echoes its own code back in the tool result**, so nothing secret may appear in the snippet — that is exactly why the cookies arrive over a URL instead of as a literal. Its code also runs in a bare `vm` context whose only globals are `page` and `__end__`: there is no `require`, no `process`, no `import`, so a snippet cannot read `.env` itself. Do not try.

3. Alternative when the MCP server was started with `--caps=storage` (adds `browser_storage_state` / `browser_set_storage_state`): run `bash scripts/qa-login.sh` with no flag to write a 0600 state file under `.playwright-mcp/`, then `browser_set_storage_state` with its absolute path, and delete the file when the hunt ends — it holds a live session cookie. Check whether those tools exist before planning around them; they are opt-in and a `.mcp.json` change only takes effect in a new session.

4. Verify the session: the URL is no longer `/auth/login` and the workspace switcher renders. Script exit 2 (`QA_EMAIL/QA_PASSWORD` missing or empty) is the one case to hand back to the user — tell them to fill `.env`; never ask them for the password in chat.

5. Note which workspace and plan you landed in, and its pre-existing resources. Everything you create in Phase 2 lives in **this** workspace only.

## Phase 2 — Sweep the hosting features like a real user

Production rules — read them before you click anything destructive:

- Prefix everything you create with `qa-<yyyymmdd>-` and **delete it before you finish**. Anything you could not delete goes in the Phase 8 report, loudly.
- Never delete, suspend, rotate, downgrade, or reconfigure a resource you did not create, and never touch another workspace.
- Never buy anything: no paid plan upgrades, no paid add-ons, no card changes. Read those screens, don't submit them.
- Anything irreversible outside your own `qa-` resources: stop and ask first.
- Only attach custom domains you actually control; if you test the add-domain flow with a throwaway hostname, remove it in the same visit.

Run **whole journeys**, not page loads — do the thing, wait for the async state, then check that the UI's promise is true:

| # | Journey | The promise to verify |
| --- | --- | --- |
| 1 | Project / environment / workspace create + switch | resources land in the right project+env everywhere they are listed |
| 2 | Create a web service from a public repo (e.g. `examples/hello-go`) | build log streams, deploy reaches Live, the `.onbex.co` URL actually serves — `curl -sSI` it outside the browser too |
| 3 | Deploys tab: manual deploy, redeploy, cancel, rollback, deploy hook | each action's terminal state matches what the list, the detail page and Events all say |
| 4 | Env vars, secret files, env groups | a saved value survives reload, triggers a redeploy, and reaches the running process |
| 5 | Custom domain add + verification + cert status | instructions are correct and the state machine never sticks |
| 6 | Scaling: instance count, plan view, autoscaling | the applied number is the number that runs; zero-downtime claim holds |
| 7 | Logs (live tail, filters, search, time range) and Metrics | filters narrow, ranges shift, empty states are honest |
| 8 | Shell / SSH into a running instance | attaches, or fails with a real reason |
| 9 | Static site: create, redirects, header rules | rules take effect on the served response |
| 10 | Cron job, background worker, private service | schedule/run/logs behave; private service is not publicly reachable |
| 11 | Postgres: create, connection info reveal + copy, backups, delete | copied URL connects; backup list is real |
| 12 | Key Value: create, connection info, delete | same |
| 13 | Blueprints (`bex.yml`) sync | plan diff matches what apply does |
| 14 | Deliberately break a build (bad start command) | failure UX, events, notifications/webhooks tell the truth |
| 15 | Free-tier sleep → wake on first request | the activator wakes it inside the advertised window |
| 16 | Delete every `qa-` resource | it disappears from every list, project page, and usage view |

Capture as you go: `browser_console_messages` and `browser_network_requests` after each journey (note every 4xx/5xx and every call that hangs), plus a screenshot at each surprising state.

What counts as a bug: the UI lying (status disagreeing between list, detail and events), a promise not kept (Live badge over a 404 URL), states that never resolve, mutations that report success and did nothing (or the reverse), lost form input, dishonest empty states, search/pagination that does not narrow, uncaught console errors, failed background requests, wrong English, i18n gaps, missing accessible names on primary controls, and Render-parity divergences. When a field looks wrong in the UI, check whether REST/GraphQL/MCP agree — a disagreement is the more valuable finding.

What is **not** a bug: upstream Ory Elements cosmetics, anything in `.pm/DO_NOT_DO.md`, the deliberate `—` non-goals in `docs/ADR018-render-parity.md`, plan-gated features, your own bad input, and anything already fixed on `main` but not yet deployed (Phase 5 catches these).

## Phase 3 — Triage

Reproduce every candidate at least once from a fresh page load before believing it. Four traps this hunt actually fell into — check each before you write a finding down:

- **Your own sweep rate is not a user's.** Navigating dozens of pages back to back drains the `BEX_RATE_LIMIT` bucket (500/min, keyed on the caller identity) and every later page starts returning `429 RATE_LIMITED`. Before blaming the product for throttling, idle 30–60s and redo the journey at human pace with pauses. Only a 429 that survives that is real.
- **Accessibility heuristics lie.** A DOM scan for "button with no innerText and no aria-label" flags every control labelled by a sibling `<label for>`. Re-check with the real accessibility tree — `await page.locator('main').ariaSnapshot()` — and keep only controls that come back genuinely unnamed.
- **A tab that redirects is not a broken tab.** `/services/<id>/headers` and `/redirects` land on `/settings` because those are static-site surfaces; identical page sizes across URLs usually means a deliberate redirect, so check `location.pathname` before calling it a rendering bug.
- **Non-browser clients hit different infrastructure.** `api.bex.co` sits behind Cloudflare, which answers a `Python-urllib/*` User-Agent with `403 error code 1010`. Probe the API from inside the page (`page.evaluate` + `fetch(..., {credentials:'include'})`), not from a bare script, or you will file a bot-protection response as an API bug.

When the UI looks wrong, query the API directly from the page before concluding where the bug lives — this hunt's main finding only became clear from the raw GraphQL response, which showed the backend returning an all-empty object where the UI merely looked confused. For each: exact steps, expected vs actual, evidence paths, and severity — **blocker** (a core hosting journey cannot be completed), **major** (completes but the product misleads or loses data), **minor** (cosmetic / copy / polish). Drop what you cannot reproduce; note it as unreproduced rather than filing it.

## Phase 4 — Research the fix

For every surviving bug, find the root cause in this repo and cite `file:line`. Research map:

- UI: `dashboard/src/routes/<route>.tsx`, `dashboard/src/features/<area>/` (`services`, `deploys`, `env-groups`, `databases`, `keyvalue`, `logs`, `metrics`, `projects`, `blueprints`, `usage`)
- API: `lego/backend/internal/<area>/` — REST + GraphQL + MCP live together, so a fix in one is a fix in three ([docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md))
- Runtime/reconcile: `lego/operator/` (Deployment/Service/Ingress, build pipeline, activator)
- Read the governing ADR before proposing anything — catalog in [docs/CLAUDE.md](../../../docs/CLAUDE.md); most relevant here are ADR004 (deploys), ADR005 (custom domains), ADR009 (Postgres), ADR021 (Key Value), ADR029 (static sites), ADR049 (`render.yaml` parity), ADR018 (parity ledger).
- Compare against render.com's behavior for the same surface; record deliberate divergence as divergence, not as a bug.

Write one record per bug:

```
### <n>. <one-line symptom>
- Severity: blocker | major | minor
- Repro: 1… 2… 3… (on <url>)
- Expected / Actual:
- Evidence: .playwright-mcp/qa-<surface>-<n>.png · <console/network excerpt>
- Root cause: <path/file.tsx:120> — <why>
- Fix: <what changes; which of REST/GraphQL/MCP/UI must move together>
- Render: <what render.com does, or n/a>
- Estimate: <tens of minutes>
```

## Phase 5 — Dedupe before filing anything

Do this for every finding, and record the outcome in its record:

1. `grep -ril "<distinctive term>" .pm --include="*.md"` — search **open and `done/`** items. An already-fixed bug that is live again is a **regression**: file it as one, citing the old `wN/mN`. An open item that covers it: do not file a duplicate — extend it with `/pm add-task <wN/mN> <title>`.
2. Re-read `.pm/DO_NOT_DO.md`. A finding that matches an anti-goal is not filed; say so in the report with the item it matches.
3. Scan open milestones everywhere, not just the target workstream: `find .pm -path '*/done' -prune -o -name README.md -print`.
4. Check whether the fix already landed but is not deployed: `git log --oneline -40 -- dashboard lego` plus a targeted `git log -S"<symbol>"`. If it is on `main`, it is a deploy-lag note in the report, not a bug to file.
5. Prior live hunts and their residuals are precedent — `w9/m89`, `w9/m92`, `.pm/w9/051.md`. Match their shape, don't re-file their contents.

## Phase 6 — Hand over to `/pm` (default `w6`)

`/pm` is the **only** skill that writes to `.pm/`. Invoke it; do not hand-edit the board.

- **> ~1h across more than one task** → `/pm new milestone w6 <title>`. Supply: the title, one task per bug with estimate and `depends_on`, a Definition of done written as observable live-verifiable states (one bullet per bug, in the shape `w9/m92`'s DoD uses), and Source + Goal linkage naming this hunt's date, evidence paths, the ADR the surface belongs to, expected outcome, why now, and whether the Render-parity closing task applies (it does whenever a REST/GraphQL/MCP/UI surface changes).
- **≤ ~1h** → `/pm add w6 <note>` as an inbox note. Do not inflate small findings into a milestone.
- Let `/pm` own numbering and the standing closing tasks (Render parity → Simplify → Test coverage → Closeout). Never hand-roll them.
- Confirm afterwards that `.pm/w6/README.md`, the milestone `README.md`, and each task's frontmatter agree, and that `npx prettier@3.4.2 --write "**/*.md"` has run.

## Phase 7 — `/ship` the scheduled milestone

Invoke `/ship` so the newly scheduled work lands on `main` and is visible to whoever picks it up.

- Scope the commit to the `.pm/` files this run created. Anything that was already dirty in Phase 0 stays out — surface it to the user instead of sweeping it in.
- `.playwright-mcp/` is gitignored: evidence stays local and is referenced by path from the milestone, exactly as `w9/m89` does.
- Never commit `.env` or `*.kubeconfig`.
- This ships the **filing**, not the fixes. Implementing the tasks is `/loop-worker w6` or ordinary work afterwards — say so in the report.
- `DRY_RUN=1` skips this phase entirely.

## Phase 8 — Report

- Journeys exercised, and which were skipped and why (plan-gated, unsafe on prod, out of scope).
- Findings by severity, each with root cause `file:line` and the proposed fix.
- What was filed and where (`w6/mNN` or `w6/NNN.md`), what was deduped away and against what, and what was rejected as an anti-goal / non-goal / deploy lag.
- Cleanup status: every `qa-` resource deleted, or exactly what is still live.
- The shipped HEAD.

## Non-negotiables

- The QA password never enters the transcript, a file, a screenshot, a `.pm` note, or a commit.
- Production is real: create only in the QA workspace, prefixed, and clean up.
- Only `/pm` writes `.pm/`; only `/ship` commits.
- Bare filenames for screenshots (they land in `.playwright-mcp/`).
- Prettier every touched markdown file before finishing.
