---
name: loop-to-meet-webby-award-bar
metadata:
  internal: "true"
description: >-
  Loop one or more dashboard pages through review → redesign or polish → re-review until they meet the Webby Awards bar: capture each page at seven viewport widths (plus dark mode, zh and keyboard passes), have a fresh-context judge score it against the six Webby website criteria and a UX checklist, fix the findings in the working tree, and repeat until every criterion scores 8 or more at every width. Use when the user asks to bring a page up to Webby or award-level design quality, to review and polish a page's responsiveness and user experience in a loop, or invokes `/loop-to-meet-webby-award-bar <page or pages>`. Leaves changes uncommitted.
---

# Task: loop page(s) through review → redesign → re-review until they meet the Webby bar

Take the pages in `$ARGUMENTS`, judge each one the way a Webby Awards juror and a demanding user would at every screen size, fix what holds it back, and judge again. Stop when the bar in [rubric.md](rubric.md) is met, or when the loop honestly cannot get there.

Parse `$ARGUMENTS`:

- one or more pages — full URLs (`https://dashboard.bex.co/services/<id>/deploys`) or dashboard paths (`/services/<id>/deploys`). No page given: ask which pages, then continue.
- `--max-rounds N` — cap on redesign rounds per run. **Default 6.**

## Phase 0 — Preflight

1. `git status --porcelain` — record what was already dirty. This skill never commits, and its report must separate its own changes from what was there.
2. Map each page to its route file in `dashboard/src/routes/` (flat dot-named: `/services/<id>/deploys` → `services.$serviceId.deploys.index.tsx`) and its feature code in `dashboard/src/features/`. Read the route, its `pendingComponent` skeleton, and [dashboard/AGENTS.md](../../../dashboard/AGENTS.md) before judging anything.
3. Out of scope, say so and skip: the bex.co marketing and docs site (separate repo `bex-co/eden-cms-v2`), the mobile app (no web target), HTML email.
4. Confirm the Playwright MCP browser tools are available. Screenshots go to `.playwright-mcp/` — always pass **bare filenames**.
5. Sign in exactly as [qa-find-bugs Phase 1](../qa-find-bugs/SKILL.md) does: `bash scripts/qa-login.sh --serve`, then inject the cookies with `browser_run_code_unsafe` from the loopback URL. **Never read `.env` or `muse.env`, never type a password.** Keep the `logout <jar>` path for Phase 6.
6. Open each page on production. If the QA user cannot see it (404, wrong workspace, redirect to `/`), **stop for that page and report it** — do not review a different resource in its place.
7. Start the iteration target, which renders the working tree against the same production data so the same resource ids work. Pick a free port `<P>` (5173 is often taken by another session; never stop a server this run did not start) and name it in every URL:

   ```bash
   cd dashboard && VITE_KRATOS_PUBLIC_URL=http://localhost:<P>/kratos \
     VITE_API_URL=http://localhost:<P>/graphql \
     VITE_KRATOS_SSR_URL=https://auth.bex.co \
     VITE_SSR_API_URL=https://api.bex.co/graphql \
     yarn dev --port <P> --strictPort
   ```

   The two `SSR` variables are required: `dashboard/.env` points the server-side calls at the local stub, and without them every page redirects to login. Then copy the browser context's `bex.co` cookies onto `localhost` (`context.cookies()` → `addCookies` with `domain: "localhost"`, `secure: false`) and open the page. If production data cannot be reached from the dev server, fall back to `yarn local-bex` + `yarn dev:local` (synthetic data, no auth) and say in the report that the page was judged on stub data.

## Phase 1 — Capture

For each page, with `browser_resize` then `browser_take_screenshot` (full page), named `webby-<page-slug>-r<round>-<variant>.png`:

- **Widths** (ready state): 360×800, 390×844, 768×1024, 1024×768, 1280×800, 1440×900, 1920×1080.
- **States** at 390 and 1440: the pending skeleton (hold the route pending), empty, error, and every dialog, menu, sheet or popover the page owns, opened.
- **Variants** at 390 and 1440: dark mode, `zh` locale.
- **Keyboard pass** at 1440: tab through the page; capture any control with no visible focus ring, an illogical order, or a trap.
- **Measured facts**, with `browser_evaluate`, at every width: `document.documentElement.scrollWidth > innerWidth` (horizontal overflow), interactive elements under 44×44 px at widths below 768, console errors, layout shift between pending and ready.

Capture hygiene, or the judge scores artefacts instead of the page:

- The dashboard scrolls inside an inner container, so a full-page screenshot shows only the fold. Take the fold at the stated height, then a second `-full` shot with the viewport made as tall as that container's `scrollHeight`.
- Set the viewport before navigating, move the mouse to `0,0` before each shot (stray hover opens tooltips), and hide the dev build's TanStack devtools button.
- `browser_run_code_unsafe` has no `setTimeout`; use `page.waitForTimeout`. Hold a route pending by delaying `**/graphql` with `page.route`, force the error state by aborting it, and call `page.unrouteAll()` afterwards.
- Look at every screenshot before handing it to the judge. Re-shoot one that does not show the state its name claims.

Round 0 captures **production** as the baseline. Later rounds capture the local build.

## Phase 2 — Judge

Spawn a **fresh-context** subagent for each round. Give it only: [rubric.md](rubric.md), the round's screenshot paths, the measured facts, and one line on what the page is for. Do not give it the diff, the earlier scores, or your own opinion — the loop must not grade its own work.

It returns, per the rubric: a 1–10 score for each of the six Webby criteria at each width, and a findings list where every finding has a severity (blocker / major / minor), the width, the screenshot it was seen in, and what a better version looks like.

Read the screenshots yourself too. Where you disagree with the judge on a fact (it missed an overflow, or reported one that is not there), check the page and correct the record; do not overrule its taste to raise a score.

## Phase 3 — Bar check

A page meets the bar when **every criterion scores 8 or more at every width** and **no blocker or major finding is open**. All pages meet it → Phase 6.

## Phase 4 — Redesign or polish

Fix findings in severity order; when a structural problem and a cosmetic one overlap, fix the structure first. Decide per round whether the page needs a redesign (hierarchy, layout or flow is wrong) or polish (the structure is right, the execution is not). Load the `frontend-design` skill for visual direction.

Constraints:

- **Render parity wins** ([AGENTS.md](../../../AGENTS.md) top rule). Behavior, information architecture, labels and flows keep matching render.com. This loop raises execution quality: layout, hierarchy, spacing, type, motion, states, responsiveness, accessibility. A finding that can only be fixed by diverging from Render is recorded as **needs user decision** and left alone; it does not count against the bar.
- Reuse the tokens in `dashboard/src/style.css` and the primitives in `dashboard/src/common/components/ui/`. Follow the layout rules in dashboard/AGENTS.md (section spacing, stat-card shape, one left rail, no oversized hero type in-app).
- **Skeleton shape parity.** Any layout change updates the route's pending skeleton to match, verified pending vs ready at desktop and narrow mobile.
- New or changed copy goes through `t()` in all eight locales. A colour change updates `src/test/__tests__/theme-contrast.test.ts`.
- Fix shared components where the flaw lives, but re-capture every page in this run that uses them, and name in the report any other page a shared change touches.

## Phase 5 — Verify, then loop

From `dashboard/`, on the files this round touched: `yarn vitest related --run <files>`, `yarn eslint <files>`, `yarn typecheck`. Fix failures properly; never weaken a test to pass.

Then return to Phase 1 for the next round.

**Stop conditions** — whichever comes first:

- the bar is met (Phase 3);
- `--max-rounds` is reached;
- two consecutive rounds raise no score and close no blocker or major finding.

The last two end in a **bar not met** report. Never lower the threshold, drop a width, or skip the judge to finish.

## Phase 6 — Final gates and report

1. From `dashboard/`: full `yarn lint` and `yarn test`. These block a push; report failures with their output.
2. `npx prettier@3.4.2 --write` on any touched markdown.
3. Stop the dev server. Revoke this run's session and remove its jar: `bash scripts/qa-login.sh --logout <jar>` then `rm -f <jar>`.
4. Report:
   - verdict per page: bar met, or bar not met and why the loop stopped;
   - the score table — criteria × widths, round 0 next to the final round;
   - what changed, by file, and any other page a shared change touches;
   - findings still open, and those parked as **needs user decision** (parity);
   - before and after screenshot names;
   - gate results, and that the changes are uncommitted and waiting for `/ship`.

## Non-negotiables

- Only `/ship` commits. Invoking this skill does not authorize a commit or push.
- Screenshots stay in `.playwright-mcp/` under bare filenames and never enter git.
- The QA password never enters the transcript, a file, a screenshot, or a commit.
- Production is read-only for this skill: look, do not create, change or delete anything there.
- The scores are a judge model's reading of the Webby criteria, not a prediction of an award. Say so in the report.
