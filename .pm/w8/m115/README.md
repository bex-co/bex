# w8 · m115 — Surface edge-blocked requests in request logs

**Worker:** worker8 **Goal:** a request refused by the service's own inbound IP rules (edge 403) appears in that service's request logs, as on Render. **Status:** todo (t001–t002, t004 done; hosted replay pending)

## Tasks (in order)

| id   | title                                                                 | est | depends_on    |
| ---- | --------------------------------------------------------------------- | --- | ------------- |
| t001 | Attribute edge-rejected request lines to their App in the log shipper — **DONE** | 50m | —             |
| t002 | Prove attribution safety across line shapes and tenants — **DONE** | 30m | t001          |
| t003 | Verify Render parity across log surfaces                              | 25m | t002          |
| t004 | Simplify the request-log pipeline change — **DONE** | 15m | t003          |
| t005 | Test real blocked-request visibility end to end                       | 40m | t003          |
| t006 | Accept the hosted replay and close milestone                          | 20m | t004, t005    |

**Total:** 6 tasks / 180m.

## Definition of done

With an inbound IP allowlist that excludes the client set on an owned free web service: `curl` receives 403, and within two minutes `bex logs -r <id> --type request --status-code 403` and `--path <blocked-path>` each return the refusal line(s) with truthful method/status/host; unfiltered request history includes them; status discovery (`/v1/logs/values?label=statusCode`) offers `403`. Forwarded 200s, app-generated 4xx/5xx, static-site lines, and platform edge hosts keep their current attribution; no line is ever attributed to another tenant's App. Pipeline fixture tests fail before / pass after; the hosted two-reader replay (maintained Bex build + raw REST) passes; owned fixtures are deleted and absence-verified.

## Source + Goal linkage

- **Source:** functional native CLI sweep458 (2026-10-18, `bex-canary`, human device login, HEAD build): service `srv-dba461r4pcls73bt1ro0` created with `--ip-allow-list cidr=203.0.113.7/32` — client curls answer 403 (allowlist enforced), yet `--status-code 403`, `--path`, and unfiltered `--type request` queries return only the later 200s; two fresh 403s at 03:59:00Z still invisible 90 s later; status discovery offers only `["200"]`. Sanitized receipts in the sweep458 evidence record.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) dependable usable hosting; [ADR010](../../../docs/ADR010-observability.md) (`request` = Traefik's access log for that App); [ADR018](../../../docs/ADR018-render-parity.md). Render's [inbound IP rules](https://render.com/docs/inbound-ip-rules) state the oracle verbatim: "For web services, blocked requests do still appear in HTTP request logs."
- **Expected outcome:** a user debugging "my client gets 403" sees the refusals — with source-visible line content — in the service's request logs instead of an apparently empty history.
- **Why now:** the allowlist is a shipping bex feature whose refusals are invisible in bex's own primary debugging surface; the gap is in the last-mile attribution pipeline, not in a missing feature.
- **Render parity:** included — the fix changes tenant-visible log reads (REST/GraphQL/MCP share one Core query; the dashboard renders the same store). Preserve existing fields, labels, filters, and the cardinality budget; no new API surface.
