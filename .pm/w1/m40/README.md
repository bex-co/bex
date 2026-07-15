# w1 · m40 — Wake interstitial: the "sleeping, click to wake" page

**Worker:** worker1 **Goal:** A hibernated free-tier App's host serves Render's wake experience — an HTML interstitial that auto-refreshes to the live app once awake — instead of today's bare `503 {"error":"service hibernated"}` JSON, reusing the page-serving mechanism `w1/m37` just shipped. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's wake/spin-up page behavior live (status, copy, auto-refresh/retry cadence) → `docs/render-artifacts/` | 30m | —          |
| t002 | Activator serves a default wake interstitial (HTML + auto-refresh; API clients still get 503 + `Retry-After` via content negotiation), reusing m37's page path; wake still triggered | 45m | t001       |
| t003 | Update ADR007's "future work" note to the shipped behavior; maintenance + wake responders share the page-render seam | 20m | t002       |
| t004 | Render parity — wake surface vs Render's capture; content-negotiation consistency (browser HTML vs API 503) | 20m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                           | 20m | t004       |
| t006 | Test coverage — HTML for a browser Accept, 503+Retry-After for an API Accept, wake still fired          | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/`                                                         | 10m | t006       |

## Definition of done

Requesting a hibernated App's host with a browser `Accept: text/html` returns a wake interstitial (HTTP status per Render's capture) that auto-refreshes and lands on the live app once the Deployment wakes; an API client (`Accept: application/json`) still gets `503` + `Retry-After` with the wake triggered; ADR007 no longer calls the wake page future work; the maintenance and wake responders share one page-rendering code path.

## Source + Goal linkage

- **Source:** `docs/ADR007-restart-suspend-and-resume.md:36` records the wake page as future work ("a 'sleeping, click to wake' page is future work"); the activator today returns raw JSON (`lego/operator/cmd/activator/main.go:152`). `w1/m37` shipped the activator's custom-page-serving mechanism (default + bounded custom page). `/pm-brainstorm` round 11, 2026-07-15.
- **Goal linkage:** Render parity (pillar 1) — the user-facing wake UX; polishes the m4/m4.5 free-tier-sleep feature.
- **Expected outcome:** a slept app's first visitor sees a real wake page, not a JSON error; the app auto-loads when ready.
- **Why now:** sleep shipped as a headline feature with a raw-error wake UX; m37 just made the page-serving mechanism available, so the marginal cost is a template + content negotiation.
- **Render parity closing task: included** (t004) — a user-facing HTTP surface change.
