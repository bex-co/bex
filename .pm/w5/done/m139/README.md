# w5 · m139 — Postgres and Key Value logs stream live and filter by level

**Worker:** worker5 **Goal:** `render logs -r <dpg-…|red-…> --tail` streams a managed datastore's logs live, and `--level` narrows Key Value logs as it already narrows Postgres and service logs — on REST, the pinned Render CLI and the dashboard. **Status:** done

## Tasks (in order)

| id   | title                                                                                   | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Key Value log lines carry a level, and Key Value queries accept `level` — **DONE**                 | 60m  | —          |
| t002 | Live tail for `dpg-`/`red-` resources through the same authz, instance and text filters — **DONE** | 120m | t001       |
| t003 | Dashboard: the Postgres and Key Value Logs tabs go live — **DONE**                                 | 45m  | t002       |
| t004 | Pinned Render CLI walk on dev-5; checklist and artifacts updated — **DONE**                        | 45m  | t002, t003 |
| t005 | Render parity — **DONE**                                                                           | 30m  | t004       |
| t006 | Simplify — **DONE**                                                                                | 20m  | t005       |
| t007 | Test coverage — **DONE**                                                                           | 45m  | t005, t006 |
| t008 | Closeout — **DONE**                                                                                | 10m  | t007       |

Total: **8 tasks, about 6h 15m**.

## Definition of done

- On dev-5, the pinned Render CLI's `render logs -r <dpg-id> --tail` and `render logs -r <red-id> --tail` stream new lines as the datastore writes them (WebSocket), with `--instance` and `--text` honored, and exit cleanly on interrupt.
- `GET /v1/logs?resource=<red-id>&level=warning` (and the CLI's `--level`) returns only Valkey warning lines, no longer a 400; level values match the vocabulary Postgres logs already use.
- A live subscription to a datastore the caller cannot read is refused exactly as the historical query refuses it (no existence leak), before any streaming header or 101.
- The dashboard's Postgres and Key Value Logs tabs offer live mode with the same controls as the service log viewer.
- `docs/cli-compatibility-checklist.md` no longer lists datastore tails as unsupported; the two render-artifacts pages describe the shipped behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` (2026-10-19), item 5, chosen by the user the same day. Render's `/logs` `resource` parameter is "the id of a server, cronjob, job, postgres, redis, workflow, or sandbox group", and `level` filters every resource. bex refuses datastore tails in `unsupportedLiveResource` (`lego/backend/internal/logs/service.go:754-765`, called from `logs/rest.go:209-212`) and refuses `level` for Key Value in `validateKeyValue` (`logs/service.go:779-781`); `datastorelogs.ParseLine` extracts a level only for Postgres (`lego/backend/internal/datastorelogs/logs.go:179-189`). The CLI checklist marks `--tail` `[~]` with "datastore tails remain unsupported" (`docs/cli-compatibility-checklist.md:121`) and no board item owns it. Pinned divergences: `docs/render-artifacts/postgres-logs.md`, `docs/render-artifacts/keyvalue-logs.md`.
- **Goal linkage:** ADR018 Render parity; the Render CLI as bex's fifth surface (DO_NOT_DO "first-party CLI" item — gaps become bex-api work); ADR009/ADR021 managed data.
- **Expected outcome:** debugging a database live works the way it does for a service, from the CLI and the dashboard.
- **Why now:** w8/m48 just gave Postgres level filtering, leaving Key Value the outlier; the datastore read path is already direct-from-pod (`datastorelogs`), and the backend already streams followed pod logs for apps (`PodLogsFollow`, `logs/service.go:136-179`), so the tail is incremental. The `/qa-find-bugs-cli` loop exercises `logs` every sweep.
- **Render parity included:** this changes REST (subscribe + query), the CLI-visible contract, GraphQL/MCP log queries (level) and the dashboard, so t005 re-checks every surface.

## Evidence (2026-10-20)

- **DoD 1 (CLI live tails, dev-5).** Pinned `render-oss/cli` via the `bex` launcher against dev-5's bex-api: `bex logs -r dpg-dbbfrn1jg4r30sm5h680 --tail` streamed a forced `select 1/0` as `ERROR:  division by zero` with no CNPG instance-manager JSON; `bex logs -r red-dbbhh61jg4r11240u0l0 --tail` streamed a `BGSAVE` run after it opened (`* Background saving started`, `* DB saved on disk`). Both stop promptly on SIGINT (the pinned CLI's default signal exit, as for a service tail). Re-run on the final post-simplify binary.
- **DoD 2 (Key Value level).** `--tail --level info` carried the BGSAVE lines; `--tail --level warning` carried only Valkey's own `#` lines. REST/GraphQL/MCP `level` and the Loki selector are pinned by `TestKeyValueLevelFilter` and `TestKeyValueLevelOnEveryAdapter`; dev-5's own history read is empty (its Loki receives no tenant streams from the local shipper), so the history half was proven by tests, the shipper half by the Alloy pipeline test on the shared table.
- **DoD 3 (refusals before streaming).** `TestSubscribeRefusesManagedTailsBeforeStreaming`: a denied caller gets 403 and an HTTP filter or `level` beside an App gets the named 400, all before any `text/event-stream` header or WebSocket 101.
- **DoD 4 (dashboard).** Key Value Logs tab, Live on, `BGSAVE` lines streamed in (`.playwright-mcp/m139-kv-live-1280.png`); at 390px the lines read correctly after the short-instance fix (`m139-kv-live-390.png` — before it, a full instance id squeezed the message to zero width, a pre-existing bug the walk exposed).
- **DoD 5 (docs).** CLI checklist `--tail` row, `postgres-logs.md`, `keyvalue-logs.md`, ADR010 subscribe section, ADR018 Key Value log row.
- **Suites.** Backend logs/datastorelogs/postgres/keyvalue/api green; `make lint` 0 issues (all four modules); shipper `TenantDatastoreLogsTest` green on the local Alloy 1.3.1 image (the pinned digest could not be pulled without registry credentials; CI runs it); dashboard logs/routes/databases/keyvalue suites green apart from `deploys-history-filters` "Back from a deploy", which times out only under full-suite load and passes alone.
- **Environment.** A dev-5 Key Value store `red-dbbhh61jg4r11240u0l0` (free) was created for the walk beside m138's `dpg-dbbfrn1jg4r30sm5h680`; both stay in the walk tenant for reuse.
