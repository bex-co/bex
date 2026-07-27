# w5 · m50 — Settings edit-in-place parity: always-rendered disabled inputs (Render pattern)

**Worker:** worker5 **Goal:** Every editable field on the service Settings page renders as a real input box (textbox or select) that is always visible but disabled until its pencil Edit button is clicked — Render's exact interaction — replacing bex's plain-text-plus-Edit-button rows. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Shared editable-field row component (disabled input + pencil → Cancel/Save changes)        | 45m | —          |
| t002 | Migrate General + Maintenance rows (Service Name, Max Shutdown Delay, custom page URL)     | 30m | t001       |
| t003 | Migrate Build & Deploy text rows (Branch, Root Dir, commands, Dockerfile Path, publish)    | 45m | t001       |
| t004 | Migrate remaining rows (Health Check Path, cron Schedule/Command, Notifications select)    | 30m | t002, t003 |
| t005 | Render parity — cross-surface consistency check                                            | 30m | t004       |
| t006 | Simplify — run `/simplify` over the changed code                                           | 30m | t005       |
| t007 | Test coverage — behavior tests for the shared row + migrated fields                        | 45m | t005       |
| t008 | Closeout — verify DoD, mark done, move milestone                                           | 15m | t007       |

## Definition of done

On the service Settings page (live dev-5 browser check, web + static + cron services): every editable field shows its current value inside a visibly disabled input (or select) with a pencil button; clicking the pencil enables and focuses that same input and swaps the pencil for Cancel + "Save changes" with Save disabled until the value actually differs; Cancel restores the original value and disabled state; fields that trigger a rebuild still show their confirm dialog before saving. No settings row renders its value as plain text with a separate "Edit …" button anymore. Dashboard test suite green.

## Source + Goal linkage

- **Source:** user request 2026-07-26 — live authenticated walk of Render's settings page (`dashboard.render.com/web/srv-cr1aprdds78s739qrbg0/settings`, Node web service) against bex's (`dashboard.bex.co/services/srv-d9bkcspg9s7c73d0n8ug/settings`), 2026-07-26/27; Render full-page capture saved as `.playwright-mcp/render-settings-full.png` (session artifact). Verified interaction on Render: the Name textbox stays in the DOM, becomes `[active]` on pencil click, and shows `Cancel` + `Save changes [disabled]` until dirty.
- **Goal linkage:** Render parity pillar (`docs/ADR018-render-parity.md`) — the dashboard is bex's UI surface of the parity ledger; Settings is the highest-traffic management page.
- **Expected outcome:** The settings page reads and behaves like Render's — users see at a glance which values exist and which are editable, and edit affordances are discoverable without layout shift.
- **Why now:** User-reported inconsistency on the live production dashboard; m51–m53 build on the same rows, so the shared row component must land first (m53's select-shaped Auto-Deploy row consumes t001's select variant).
- **Render parity task included:** yes — this is user-facing dashboard UI; the check compares the final page against Render's live settings page interaction-for-interaction.
