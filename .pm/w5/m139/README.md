# w5 · m139 — Postgres and Key Value logs stream live and filter by level

**Worker:** worker5 **Goal:** `render logs -r <dpg-…|red-…> --tail` streams a managed datastore's logs live, and `--level` narrows Key Value logs as it already narrows Postgres and service logs — on REST, the pinned Render CLI and the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                                                   | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Key Value log lines carry a level, and Key Value queries accept `level`                 | 60m  | —          |
| t002 | Live tail for `dpg-`/`red-` resources through the same authz, instance and text filters | 120m | t001       |
| t003 | Dashboard: the Postgres and Key Value Logs tabs go live                                 | 45m  | t002       |
| t004 | Pinned Render CLI walk on dev-5; checklist and artifacts updated                        | 45m  | t002, t003 |
| t005 | Render parity                                                                           | 30m  | t004       |
| t006 | Simplify                                                                                | 20m  | t005       |
| t007 | Test coverage                                                                           | 45m  | t005, t006 |
| t008 | Closeout                                                                                | 10m  | t007       |

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
