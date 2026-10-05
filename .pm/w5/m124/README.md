# w5 · m124 — Service settings: one section list drives the page and its skeleton, and image services reuse the Deploy card

**Worker:** worker5 **Goal:** The settings page and its pending skeleton can't drift (the AGENTS.md skeleton rule), and image services get the same Deploy card (Docker Command, Pre-Deploy Command, Deploy Hook) as repo services. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Drive the settings page from one section list | 45m | — |
| t002 | Make the skeleton map the same section list | 1h | t001 |
| t003 | Reuse the Deploy card for image services | 1h | t001 |
| t004 | Render parity | 20m | t002, t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 30m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- For each service type × source, a test asserts the skeleton's regions equal the page's sections, and pending vs ready is verified side by side at desktop and narrow-mobile widths.
- Image services' Deploy card shows Docker Command, Pre-Deploy Command and Deploy Hook. The duplicated row, confirm step and gate in `image-command-section.tsx` are gone, and so is the standalone `!service?.repo` deploy-hook branch.
- Pre-Deploy for an image service works end to end on dev-5.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `739c4b217` (w4/m166) and `174fca341` (w4/m165): m166 added a skeleton region by hand, and the skeleton already disagrees with the page for image services.
- **Goal linkage:** The root AGENTS.md skeleton rule; ADR018 dashboard parity for image services.
- **Expected outcome:** No layout jump on settings navigation, and image services are configurable like Render's.
- **Why now:** Every new settings section repeats the drift, and m166 just added one.
- **Render parity included:** image-service settings UI changes.
