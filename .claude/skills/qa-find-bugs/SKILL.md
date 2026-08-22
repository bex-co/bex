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

1. `browser_navigate` → `https://dashboard.bex.co/auth/login`, then `browser_snapshot` to read the real field labels — the form is Ory Elements and its label text varies by locale (`Email`, `E-Mail`).
2. Probe the MCP server's working directory (no secret in, no secret out): `browser_run_code_unsafe` with `async () => process.cwd()`. If it is not the repo root, use the absolute path to `.env` in step 3.
3. Fill and submit **inside the Playwright process**, so the credentials never enter your context. Adapt the selectors to the snapshot from step 1, and return only the landing URL:

   ```js
   async (page) => {
     let fs;
     try {
       fs = await import("node:fs");
     } catch {
       fs = require("node:fs");
     }
     const read = (k) => {
       const m = fs
         .readFileSync("/ABSOLUTE/PATH/TO/REPO/.env", "utf8")
         .match(new RegExp("^\\s*" + k + "=(.*)$", "m"));
       return m ? m[1].trim().replace(/^["']|["']$/g, "") : "";
     };
     const email = read("QA_EMAIL"),
       password = read("QA_PASSWORD");
     if (!email || !password) return "MISSING_CREDENTIALS";
     await page.getByLabel(/e-?mail/i).fill(email);
     await page.getByLabel(/password/i).fill(password);
     await page.getByRole("button", { name: /sign in|log in/i }).click();
     await page.waitForLoadState("networkidle");
     return page.url(); // never return the values themselves
   };
   ```

4. Verify the session: the URL is no longer `/auth/login` and the workspace switcher renders. `MISSING_CREDENTIALS` → STOP and tell the user to add `QA_EMAIL` / `QA_PASSWORD` to `.env`.
5. If `browser_run_code_unsafe` is unavailable or refused: **STOP and ask** the user to sign the MCP browser in themselves. Do not ask for the password in chat, and do not type a password you were handed in conversation — that defeats the whole point of step 3.
6. Note which workspace and plan you landed in, and its pre-existing resources. Everything you create in Phase 2 lives in **this** workspace only.

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

Reproduce every candidate at least once from a fresh page load before believing it. For each: exact steps, expected vs actual, evidence paths, and severity — **blocker** (a core hosting journey cannot be completed), **major** (completes but the product misleads or loses data), **minor** (cosmetic / copy / polish). Drop what you cannot reproduce; note it as unreproduced rather than filing it.

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
