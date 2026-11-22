# w32 · m567 — A web service's deploy log follows `==> Your service is live 🎉` with Render's primary URL block

**Worker:** worker32 **Goal:** a web service deploy that goes live closes its log (`bex logs --type build`, the live tail, the deploy page, GraphQL and MCP) with the block Render writes after its live line, naming the URL the deploy went live at. The URL is the one recorded on the deploy's own row, so an older deploy's log does not change when the service later gets a custom domain. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record the URLs a deploy went live at on its row](t001.md) | 1h | — |
| t002 | [Narrate Render's primary URL block after a web service's live line](t002.md) | 1h | t001 |
| t003 | [Record the block in the progress-line contract and ADR018](t003.md) | 20m | t002 |
| t004 | [Ship and verify on production](t004.md) | 45m | t002, t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Render's block

Render writes six lines after `==> Your service is live 🎉`. A blank marker line is `==> ` with its trailing space, and the rule is `==> ` followed by 59 slashes:

```text
==> Your service is live 🎉
==> 
==> ///////////////////////////////////////////////////////////
==> 
==> Available at your primary URL https://<service>.onrender.com
==> 
==> ///////////////////////////////////////////////////////////
```

What 18 public GitHub issue logs show, read with `gh issue view` on 2026-11-22:

- **The shape.** These 12 logs carry the whole block, and every rule is 59 slashes:
  - lss233/kirara-ai#1498 (2025-09-08)
  - muppetbrown/mana_meeples_singles_market#180 (2025-10-19)
  - rkendel1/freq#99 (2026-01-12)
  - lelewithheart/Pixel-Breach#10 (2026-01-27)
  - bleuvent/tsf-weather#1 (2026-03-09)
  - Collective-Intelligence-Labs/omnichain-payments-evm#46 (2026-03-30, an nginx image)
  - ianchiu111/Pipecat#1 (2026-04-08)
  - falcon-Z/z0-auth#12 (2026-04-11)
  - m03chv13h/blunzinger-chess#469 (2026-05-29)
  - joseph-m-pasia/kg-house-price-prediction#13 (2026-08-16)
  - unknownsideofme/LATCH#42 (2026-10-21)
  - Jeanajsja/SGSDev#1 (2026-11-17)

  HKUDS/nanobot#6607 (a Docker command) and human-flourishing-frameworks/human-flourishing-frameworks#12 quote only the URL line.
- **The timing.** Each block line is a separate log line. The first follows the live line by 30 to 240 ms, and the lines after it are 3 to 110 ms apart (falcon-Z: `17.712` live, then `17.947`, `17.951`, `17.955`, `17.958`, `17.961`, `17.965`). The app's own request lines can fall between them (Jeanajsja/SGSDev#1, joseph-m-pasia#13, Collective-Intelligence-Labs#46).
- **No trailing slash in the message.** Logs read through Render's API or a log export (RFC 3339 nanosecond stamps: kirara-ai, mana_meeples#180, freq, Pipecat, z0-auth) end the URL at the host: `https://z0-auth.onrender.com`. So does Jeanajsja/SGSDev#1. Logs copied from the dashboard's viewer show `https://pixel-breach.onrender.com/`. The dashboard renders the URL as a link and its text gains the slash. The message bex writes has none.
- **A custom domain is the primary URL.** Its log line counts the service's other domains:
  - barouu78-wq/teranga-ai#417 (2026-10-10, quoting Render's log): `Available at your primary URL https://teranga-ai.fr + 2 more domains`
  - mvryhan-web/video-variator-android#233 (2026-10-17): `https://deepsyq.app + 2 more domains`
  - bookorbit/bookorbit#31 (2026-05-18, its author redacted the domain): `https://<domain>/ + 1 more domain`

  Render adds a root domain's `www` counterpart and redirects it to the root, and a service keeps its `onrender.com` subdomain unless it is disabled ([render.com/docs/custom-domains](https://render.com/docs/custom-domains)). So a root domain counts its `www` redirect and the `onrender.com` subdomain as 2 more: the count covers every other domain the service answers at. That is an inference from three logs; w32/085's Render capture can confirm it.
- **Which types get it.** A web service does, native or Docker. A static site does not: its log ends at `==> Your site is live 🎉` (muppetbrown/mana_meeples_singles_market#226, a static site of the same account on the same day as #180's web service with the block; notwo/youknowFrontend_graveyard#84; YodelingCowboys/First-Yak-Project#1). A private service and a background worker have no public URL, and no public log shows either with the block. A cron job has no live line.

## Definition of done

- On production, once the pin carries this milestone's commits, the released `bex` reads these logs in `bex-canary`:
  - A free web service that goes live. `bex logs -r <srv> --type build -o text` and a live `--tail` show `==> Your service is live 🎉`, then the six lines above with `==> Available at your primary URL https://<slug>.onbex.co`. There is no trailing slash.
  - A free static site ends at `==> Your site is live 🎉` with no block.
  - Every fixture is deleted by ID, and each delete is confirmed by a 404.
- The URL comes from the deploy row's own record, written when the row went live. A test shows that a domain added after a deploy went live leaves that deploy's block unchanged. A row that went live before the record existed has no block.
- Unit tests cover:
  - the block by type: web native, web Docker, prebuilt image and rollback on a web service have it; a static site, a private service, a background worker and a cron job do not;
  - the block by outcome: `live` and `deactivated` have it; failures and cancels do not;
  - `+ 1 more domain` and `+ N more domains`;
  - the order: the block follows the live line and comes before the initial deploy hook's lines;
  - the live tail: it emits each repeated `==> ` and rule line once, and never before the build's last line.
- `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract and ADR018 § Logs, Build logs record the block with the Render logs above.

## Source + Goal linkage

- **Source:** inbox note [w32/083](../done/083.md), filed 2026-11-22 as a follow-up from w32/m560 t009, and promoted the same day by `/loopx w32`. The note's ~45m covered the narration only. Its own scope check requires the URL to be recorded on the deploy row, which adds a migration, the reconciler's live close and the store read.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) in the deploy log. Render's log says where the deploy went live, so the user reads the URL without leaving the log.
- **Expected outcome:** a bex web service's deploy log ends as Render's does, with its URL, including the primary custom domain once one is verified.
- **Why now:** w32/081 and w32/m560 brought the rest of the log's close and rollout to Render's wording, and this block is the last Render line after a successful web deploy that bex does not write. w32/084 (the instance's own output in the deploy log) is separate, and this milestone leaves it alone.
- **Render parity task included:** the block appears on tenant-facing surfaces: REST `/v1/logs`, the subscribe stream, GraphQL `logs`, MCP `list_logs`, the CLI and the dashboard's deploy log pane.
