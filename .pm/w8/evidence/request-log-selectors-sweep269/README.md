# Native request-log selectors — sweep269

2026-10-15 UTC, production Bex, isolated human QA in bex-canary. Source6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459; deployed revision unknown. Published Bex v0.3.2 and unchanged Render2.27.0/a764810a7682 both pass all32 filter cases. One initial unfiltered native control preserves all six emitted requests. No new bug or implementation change.

[Complete create/history/HTTP/native-query/delete and cleanup ledger](owned-native-HTTP-cleanup.json), [32 exact row-ID comparisons with full native streams](filter-matrix.json), [versions and scope](metadata.json), [redactions](redactions.json), [finite fixture driver](sweep269-request-filters.py.txt), [finite filter matrix](sweep269-filter-matrix.py.txt).

Bex natively creates one Free Node web service from public bex-co/bex/main/examples/runtime-version/node, auto-deploy off. A tiny stdlib server responds with a harmless marker, received method/URL and hostname. Initial queue/build convergence reaches Live and public200 with the exact marker. Six ordinary requests, each issued once, receive their intended response: GET literal,a.txt200, GET plain.txt200, POST created.txt201, GET missing.txt404, HEAD head.txt200 and GET caf%C3%A9.txt200. A unique parent path separates the test from health checks and other traffic. No request carries a cookie, authorization header or secret body.

The first native --type request --path <owned-prefix>/\* history read already contains all six records with exact paths, methods and status labels. The matrix derives expected row IDs from that independent unfiltered control; every query must contain exactly the expected IDs, once each. Both clients pass the same16 cases:

- All six own paths; one literal comma path using normal CSV flag quoting; a two-path OR; the encoded Unicode path.
- GET, POST and HEAD separately; GET/POST OR; exact404 and the2xx class.
- Path/method/status AND; a contradictory method yielding no rows.
- Exact host equality and a nonexistent host yielding no rows.
- One literal comma text term and two text terms OR.

All32 native commands exit0. This proves non-interactive query behavior, not interactive Apply/Copy preservation: existing w8/m57 and m60 remain independent upstream defects and are not marked fixed. No tail, browser, pagination/direction, label-value discovery or other resource-family claim is made. Request methods/statuses are present as native labels; request paths remain in the retained access-log message, consistent with the cardinality design.

The private harness initially attempted to parse the first native stdout as one JSON document. The client's normal output is a stream of JSON objects. That native success capture was preserved, the private decoder was corrected to consume all six objects, and no HTTP request was repeated. The native binary and server request were unchanged; this is not a product failure. Native mutation/query wire was not intercepted.

Exact-ID native deletion exits0 after enumerating the sole deploy child. Service detail, final list and former public URL confirm absence. All six immutable baseline IDs remain; final quota is services6/terminating0, PG/KV0/terminating0. Initial terminating samples are retained before convergence. The copied Render config is removed after each native read. No product Go test, fix, commit/push or external filing.
