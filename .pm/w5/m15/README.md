# w5 · m15 — New-service create wizard (source picker · repo picker · deploy)

**Worker:** worker5 **Goal:** the dashboard can create a service — bex's answer to Render's `dashboard.render.com/web/new`: pick a source (connected GitHub repo / public git URL / existing image), fill the settings, deploy, watch it come up. **Status:** todo (repo-picker path blocked on w2/m8; private-repo deploys on w2/m9)

## Tasks (in order)

| id   | title                                                                                       | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Route `/services/new` + source picker (GitHub repo · public Git URL · existing image)       | 40m | —          |
| t002 | Repo picker over `repos` — search, default branch, connect-prompt empty state               | 45m | t001       |
| t003 | Settings form — name, branch, root directory, instance type, env vars                       | 45m | t002       |
| t004 | Create via `createService` → redirect to the service page with deploy progress              | 40m | t003       |
| t005 | Live walkthrough vs Render's `/web/new`; capture reference in `docs/render-artifacts/`      | 30m | t004       |
| t006 | Render parity — wizard vs Render's new-service flow; flip the create-service UI cell        | 30m | t005       |
| t007 | Simplify — `/simplify` over the milestone's diff                                            | 30m | t006       |
| t008 | Test coverage — form validation, source-tab state, create error surfaces, connect prompt    | 40m | t006       |
| t009 | Closeout — DoD verified, move to `done/`                                                    | 15m | t008       |

## Definition of done

From the dashboard alone (no API calls by hand): a user creates a web service from each of the three sources — a connected GitHub repo picked from the searchable list, a public git URL, and a prebuilt image — lands on the service page, and watches it reach Running with a live URL. With no GitHub connection, the GitHub tab shows a working connect prompt instead of an empty list. Verified live.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-11 (the user's parity target was literally `dashboard.render.com/web/new`); flips the create-service **UI ✖** cell in [docs/render-parity.md](../../../docs/render-parity.md) and retires its "API-first, no dashboard create wizard" divergence note.
- **Goal linkage:** Render parity for the human surface; completes the deploy story w2/m8–m9 builds for agents so humans get the same power.
- **Expected outcome:** service creation stops being API/MCP-only; the dashboard covers Render's core "new web service" journey end to end.
- **Why now:** w2/m8 (repo list) and w2/m9 (private clones + push-to-deploy) make a wizard genuinely useful for the first time; sequencing it right behind them ships the whole arc while the APIs are fresh. Render parity task included: user-facing UI feature compared against Render's live flow.

## Notes

- Backend create surface already exists (`createService` — `POST /v1/services` equivalent; parity-verified w2/m4/t001); this milestone is UI-only unless small mutation-field gaps surface (file them, don't scope-creep).
- Public-URL and image tabs are **not** blocked on w2/m8 — only the GitHub tab is; t002's empty state degrades gracefully (connect prompt) if m8 lags.
