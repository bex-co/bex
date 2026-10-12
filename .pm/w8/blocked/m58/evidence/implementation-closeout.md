# w8/m58 implementation closeout

Date: 2026-10-12

## What changed

Backend Loki history searches no longer delegate tenant text matching to Loki's case-insensitive regexp line filter. `NewLokiSource` now asks Loki for bounded raw scan pages for any text search and applies the existing Go `LogQuery.matchesText` literal matcher locally for raw searches; display searches continue through the display scan path. `lokiQueryFor` still pushes indexed stream selectors and supported path/host request filters, but text terms stay out of LogQL.

This preserves the API contract used by REST, GraphQL, MCP and the imported CLI: normalized literal `strings.ToLower` substring matching over original stored messages. It avoids Loki 3.6.7's regexp-simplifier canonical-literal loss for Greek iota while retaining bounded scans and `ScanIncompleteError` partial pages when coverage runs out.

## Tests added/updated

- `TestLokiSourceAppliesUnicodeTextSearchLocally` proves an iota search returns both `CAFÉ Ελληνικά` and `café ελληνικά` from raw Loki lines and does not include a `|~` text predicate in LogQL.
- `TestLokiSourceLocalTextSearchReachesMatchBehindRawPage` proves a match behind a full 1000-line raw scan page remains reachable.
- Query-shape tests now assert text terms stay out of LogQL while stream selectors and datastore/request scoping remain intact.
- Literal/metacharacter tests now verify local matching remains literal and regex-safe.

Validation run:

```sh
cd lego/backend && go test ./internal/logs
cd lego/backend && go test ./...
```

Both passed on 2026-10-12.

## Remaining gate

The code and regression coverage are complete locally, but the milestone's definition of done requires released-production native Bex and unmodified Render CLI receipts against App, Postgres and Key Value fixtures, plus cleanup evidence. That needs the pushed backend change to deploy first, then a live QA replay. Until that exists, t003/t006 and the milestone remain blocked on production deployment/live acceptance evidence.
