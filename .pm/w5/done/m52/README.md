# w5 · m52 — Settings IA alignment with Render's section layout

**Worker:** worker5 **Goal:** The settings page's section structure matches Render's — General (with a read-only Region row) › Build › Deploy (containing Auto-Deploy + Deploy Hook) › Custom Domains (with the platform-subdomain toggle nested inside) › Networking › Notifications › Health Checks › Maintenance Mode › bottom Suspend/Delete — while bex-only extensions stay in sensible places. **Status:** done (2026-07-27)

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | General card: rename + read-only Region row — **DONE** | 30m | —          |
| t002 | Split Build vs Deploy cards; move Deploy Hook into Deploy — **DONE** | 45m | t001       |
| t003 | Fold Platform Subdomain into Custom Domains; match Render's section order — **DONE** | 30m | t002       |
| t004 | Bottom danger area: Suspend + Delete as Render-style closing actions — **DONE** | 30m | t003       |
| t005 | Render parity — cross-surface consistency check — **DONE** | 30m | t004       |
| t006 | Simplify — run `/simplify` over the changed code — **DONE** | 20m | t005       |
| t007 | Test coverage — section structure + Region row tests — **DONE** | 30m | t005       |
| t008 | Closeout — verify DoD, mark done, move milestone — **DONE** | 15m | t007       |

## Definition of done

On dev-5, a web service's settings page renders sections in Render's order with Render's grouping: General (Name, Region read-only, Instance Type, plus bex's Idle timeout + Max shutdown delay), Build (Source, Branch, Root Directory, Build Command, Build Filters), Deploy (Pre-Deploy Command, Start Command, Auto-Deploy, Deploy Hook), Custom Domains (list + platform-subdomain toggle inside), Networking (bex IP allowlist), Notifications, Health Checks, Maintenance Mode, and a bottom Suspend/Delete danger area. Region shows the installation's `BEX_REGION`-derived value (hidden when unset). Type gating (cron/static/worker variants) still holds. Dashboard suite green.

## t005 parity walk (2026-07-27)

Live local walk (`yarn local-bex` + `yarn dev:local`). The **web service** `eden-cms-v2` settings page renders in Render's exact section order: **General** (Name, `Region: fsn1` read-only disabled textbox, Instance Type, Idle timeout, Max shutdown delay) › **Build** (Source, Branch, Root Directory, Dockerfile Path) › **Deploy** (Docker Command, Auto-Deploy, **Deploy Hook** embedded) › **Custom Domains** (with the **Platform Subdomain** toggle folded in) › **Networking** › **Notifications** › **Health Checks** › **Maintenance Mode** › **Suspend / Delete** danger area. The **cron** `nightly-report` keeps its own Deploy (Schedule/Command) section with no duplicate Deploy card, Auto-Deploy folds into Build for a git-cron, and its Deploy Hook stays a standalone card — type gating intact. **Outcome: clean**, matches `.playwright-mcp/render-settings-full.png`. Snapshots: `.playwright-mcp/m52-web.md`, `m52-cron.md`. The `region` field was spliced into `server.graphql` + the generated `ServerQuery`/`ServerDocument` per the codegen note (no backend change — the GraphQL `Service.region` already existed).

## Source + Goal linkage

- **Source:** user request 2026-07-26 — live section-by-section walk of both settings pages 2026-07-26/27 (Render heading order captured via DOM query: General, Build, Deploy, Custom Domains, PR Previews, Networking/Edge Caching, Notifications, Log Stream, Health Checks, Maintenance Mode, then "Suspend Web Service"/"Delete Web Service" buttons; capture `.playwright-mcp/render-settings-full.png`). bex today: merged Build & Deploy card, separate Platform Subdomain and Deploy Hook sections, Health Checks near the top, no Region row.
- **Goal linkage:** Render parity pillar (`docs/ADR018-render-parity.md`) — users coming from Render find things where Render puts them.
- **Expected outcome:** Structural muscle-memory parity; the page reads as the same product surface as Render's.
- **Why now:** The user asked for consistency with Render's page; doing IA after m50/m51 avoids re-moving rows that are being rewritten. PR Previews / Edge Caching / Log Stream / Disk / One-Off Jobs remain excluded per `.pm/DO_NOT_DO.md`.
- **Render parity task included:** yes — user-facing UI restructure; Region additionally touches the GraphQL read surface (REST already exposes `region` via `BEX_REGION`, w1/m53).
