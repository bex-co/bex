# w3 · m12 — Metrics `host`/`path` filter honesty fix

**Worker:** worker3 **Goal:** stop returning silently-unfiltered metrics data for `host`/`path` query filters that look accepted but do nothing **Status:** done

## Tasks (in order)

| id   | title                                                                                                    | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Add explicit rejection (400) when `host`/`path` filters are passed to `http-requests`/`http-latency`/`bandwidth` metrics queries, across REST/GraphQL/MCP — **DONE** | 1.5h | —          |
| t002 | Remove the now-dead silent parsing of `host`/`path` into the metrics query path                            — **DONE** | 45m  | t001       |
| t003 | Update `docs/ADR006-bex-api.md:322` to drop the "accepted but not yet applied" language                     — **DONE** | 30m  | t001       |
| t004 | Dashboard: audit any metrics filter UI for host/path controls that would now error, adjust or remove        — **DONE** | 45m  | t001       |
| t005 | Render parity: verify the 400 behavior is consistent across REST/GraphQL/MCP + dashboard                    — **DONE** | 30m  | t004       |
| t006 | Simplify                                                                                                     — **DONE** | 30m  | t005       |
| t007 | Test coverage                                                                                                — **DONE** | 1h   | t005       |
| t008 | Closeout                                                                                                     — **DONE** | 15m  | t007       |

## Definition of done

Passing `host`/`path` to a metrics query returns a clear 400 across REST, GraphQL, and MCP instead of a silently-unfiltered series; `docs/ADR006-bex-api.md` no longer describes the old silent-drop behavior; no dashboard control offers a filter that would now error.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 (fourth pass) — `docs/ADR006-bex-api.md:322` ("`host`/`path` filters are accepted but not yet applied — Traefik service counters lack those labels"), confirmed live in `lego/backend/internal/metrics/source.go:207-209` and `rest.go:201-202`. Scoped as an honest-refusal fix rather than adding Prometheus labels, mirroring the cardinality-safety precedent the logs pipeline already established (`TestPathAndHostNeverBecomeLabels`) rather than reopening that same cardinality risk for metrics.
- **Goal linkage:** API honesty/correctness — a filter that appears accepted but silently does nothing can mislead a caller (human or agent) into trusting path-scoped numbers that are actually whole-service.
- **Expected outcome:** no caller can be misled into thinking a metrics query is host/path-scoped when it isn't; the failure mode becomes a loud, immediate 400 instead of a silently wrong answer.
- **Why now:** small, self-contained, closes a real correctness gap the project has already fixed once for logs (the store-only-filter-503 precedent, `docs/ADR010-observability.md`/root `CLAUDE.md`'s `BEX_LOKI_URL` entry). Render parity included — the rejection behavior must be consistent across REST/GraphQL/MCP.
