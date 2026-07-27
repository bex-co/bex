# w5 · m52 — Settings IA alignment with Render's section layout

**Worker:** worker5 **Goal:** The settings page's section structure matches Render's — General (with a read-only Region row) › Build › Deploy (containing Auto-Deploy + Deploy Hook) › Custom Domains (with the platform-subdomain toggle nested inside) › Networking › Notifications › Health Checks › Maintenance Mode › bottom Suspend/Delete — while bex-only extensions stay in sensible places. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | General card: rename + read-only Region row                                    | 30m | —          |
| t002 | Split Build vs Deploy cards; move Deploy Hook into Deploy                      | 45m | t001       |
| t003 | Fold Platform Subdomain into Custom Domains; match Render's section order      | 30m | t002       |
| t004 | Bottom danger area: Suspend + Delete as Render-style closing actions           | 30m | t003       |
| t005 | Render parity — cross-surface consistency check                                | 30m | t004       |
| t006 | Simplify — run `/simplify` over the changed code                               | 20m | t005       |
| t007 | Test coverage — section structure + Region row tests                           | 30m | t005       |
| t008 | Closeout — verify DoD, mark done, move milestone                               | 15m | t007       |

## Definition of done

On dev-5, a web service's settings page renders sections in Render's order with Render's grouping: General (Name, Region read-only, Instance Type, plus bex's Idle timeout + Max shutdown delay), Build (Source, Branch, Root Directory, Build Command, Build Filters), Deploy (Pre-Deploy Command, Start Command, Auto-Deploy, Deploy Hook), Custom Domains (list + platform-subdomain toggle inside), Networking (bex IP allowlist), Notifications, Health Checks, Maintenance Mode, and a bottom Suspend/Delete danger area. Region shows the installation's `BEX_REGION`-derived value (hidden when unset). Type gating (cron/static/worker variants) still holds. Dashboard suite green.

## Source + Goal linkage

- **Source:** user request 2026-07-26 — live section-by-section walk of both settings pages 2026-07-26/27 (Render heading order captured via DOM query: General, Build, Deploy, Custom Domains, PR Previews, Networking/Edge Caching, Notifications, Log Stream, Health Checks, Maintenance Mode, then "Suspend Web Service"/"Delete Web Service" buttons; capture `.playwright-mcp/render-settings-full.png`). bex today: merged Build & Deploy card, separate Platform Subdomain and Deploy Hook sections, Health Checks near the top, no Region row.
- **Goal linkage:** Render parity pillar (`docs/ADR018-render-parity.md`) — users coming from Render find things where Render puts them.
- **Expected outcome:** Structural muscle-memory parity; the page reads as the same product surface as Render's.
- **Why now:** The user asked for consistency with Render's page; doing IA after m50/m51 avoids re-moving rows that are being rewritten. PR Previews / Edge Caching / Log Stream / Disk / One-Off Jobs remain excluded per `.pm/DO_NOT_DO.md`.
- **Render parity task included:** yes — user-facing UI restructure; Region additionally touches the GraphQL read surface (REST already exposes `region` via `BEX_REGION`, w1/m53).
