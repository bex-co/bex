# w5 · m11 — Type-aware service settings (cron jobs: hide Custom Domains + Idle timeout; show Schedule + Command)

**Worker:** worker5 **Goal:** Make the Settings page service-type-aware so a Cron Job never shows the Custom Domains section or the Idle timeout control (neither applies to a non-HTTP service), and instead shows a Deploy section with the cron's Schedule expression and Command — matching Render's cron settings page. **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est | depends_on   |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Gate Custom Domains + Idle timeout on `serviceType === 'web_service'` in the Settings page        | 30m | —            |
| t002 | Add cron-specific Deploy section: Schedule (cron expression, read-only) + Command display          | 45m | t001         |
| t003 | Render parity — verify cron settings against render.com/cron live page (REST/GraphQL/MCP/UI)      | 30m | t002         |
| t004 | Simplify — `/simplify` over the m11 diff                                                          | 20m | t003         |
| t005 | Test coverage — Settings renders correct sections by service type                                  | 30m | t003         |
| t006 | Closeout                                                                                           | 10m | t005         |

## Definition of done

With the local-bex stub serving a cron-type `nightly-report` fixture (w5/m10/t001 prerequisite): the Settings page for a Cron Job service shows no Custom Domains section and no Idle timeout control; it shows a Deploy section with the Schedule cron expression and the Command; the Settings page for a web service still shows Custom Domains and Idle timeout and no Deploy section. The four surfaces (REST, GraphQL, MCP, UI) agree on the fields that drive this branching. `yarn lint && yarn typecheck` green.

## Source + Goal linkage

- **Source:** Playwright comparison 2026-07-09 — `localhost:5173/services/nightly-report/settings` showed Custom Domains (with phantom `www.eden-cms.com` data from the hardcoded stub) and "Idle timeout"; Render's live cron settings page (`/cron/crn-cr0l4oa3esus73ajt990/settings`) shows Schedule + Command and no Custom Domains or Idle timeout.
- **Goal linkage:** pillar 1 (Render parity) — a cron job settings page that surfaces HTTP-only controls is actively confusing and diverges from Render's UX contract.
- **Expected outcome:** a developer navigating to a Cron Job service's Settings sees only the controls that apply to that service type; no phantom domain data bleeds into the view.
- **Why now:** the phantom-data bug (stub bleed) and the wrong-controls bug compound each other — fixing the stub (w5/m10/t001) without fixing the Settings page leaves the Cron Job settings displaying an empty-but-wrong Custom Domains table; both fixes are needed for the Settings page to be trustworthy. Sequenced after m10 starts (stub fix provides realistic fixture data for visual verification).
- **Render parity task included:** t002 touches a user-facing surface (Settings page) that must match Render's cron job UI; t003 verifies REST/GraphQL/MCP fields (`schedule`, `command`) and the dashboard rendering against Render's captured behavior.
