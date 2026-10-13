# Unicode history searches omit stored case-insensitive matches

**Severity:** minor. **Attribution:** Loki dependency optimization surfaced by the backend history contract; the CLI request is valid. **Observed:** 2026-10-11 08:39–08:51 UTC. **State:** researched filing; no product implementation or ship.

Released Bex **0.3.2** and unmodified Render **2.27.0**, pin `a764810a768202704e7206eb7b87a47211fcd98e`, both receive HTTP 200 with `logs: []` when searching existing application lines for Greek iota. The full native history read returns the exact lines. Local Loki 3.6.7 reproduces the same omission independently. This affects an ordinary word such as `ελληνικά`, rather than malformed input or a permission probe.

## Fixture and native reproduction

Human QA OAuth, workspace **bex-canary / tea-daif693dqjvc73e7as3g**, production `https://api.bex.co/v1/`; noninteractive JSON. Owned Free Node web fixture **srv-db5kklsaqkos73b0skfg**, name `qa-20261011-b66611-textbytes`; one initial deploy **dep-db5kklsaqkos73b0skg0** reached Live and HTTP 200 with the exact unique marker before testing. It emitted these two lines among nine controlled messages:

```text
QA94_b66611 CAFÉ Ελληνικά
QA94_b66611 café ελληνικά
```

```sh
bex logs --resources srv-db5kklsaqkos73b0skfg --type app --text QA94_b66611 --limit 100 --confirm -o json
bex logs --resources srv-db5kklsaqkos73b0skfg --type app --text '"café ελληνικά"' --limit 100 --confirm -o json
bex logs --resources srv-db5kklsaqkos73b0skfg --type app --text ι --limit 100 --confirm -o json
```

Run equivalent `render logs` commands with a private config pointing to Bex and the same QA workspace. The ASCII quotes on the combined term are upstream CSV syntax for one literal value. Do not use Render production credentials.

- Full-marker controls: both native clients exit 0, Bex 3.150 seconds and Render 3.060 seconds, nine byte-exact messages including the two Greek lines. See [complete controls](evidence/native-byte-controls.json); Bex's recorded full read is 3.150 seconds.
- Combined-term captures: Bex exit 0 / 3.259 seconds; Render exit 0 / 2.570 seconds; both empty stdout/stderr and HTTP 200 / zero logs.
- Iota-only captures: Bex exit 0 / 2.605 seconds; Render exit 0 / 2.540 seconds; both empty stdout/stderr and HTTP 200 / zero logs.
- Bex `café` and `λ` each return the two expected original messages (2.836 and 2.415 seconds). Bex `ελληνικά` and `λληνικά` each return zero (2.620 and 3.184 seconds). This isolates iota, rather than all non-ASCII text.
- Both clients preserve trailing spaces, tabs, embedded quotes, a literal comma, regex metacharacters and a 14,357-byte Unicode line. The trailing-space query matches one line; the same query without that final space matches two.

## Exact wire evidence

[Eight native captures](evidence/native-wire.json) retain exact method, raw path, decoded query, safe native headers, status and complete response. A GET-only loopback capture changed only API authority through a temporary config copy and transparently forwarded the original path/query to fixed `api.bex.co`. No request payload was repaired. Authorization was forwarded privately and never journaled. Each child was reaped, capture server closed and copied config removed. Original config was untouched. Direct-production controls were not wire-intercepted.

Both native combined-term requests use this exact path:

```text
/v1/logs?ownerId=tea-daif693dqjvc73e7as3g&direction=backward&resource=srv-db5kklsaqkos73b0skfg&type=app&text=caf%C3%A9+%CE%B5%CE%BB%CE%BB%CE%B7%CE%BD%CE%B9%CE%BA%CE%AC&limit=100
```

Decoded `text` is exactly `café ελληνικά`. The captured Bex response is:

```json
{
  "hasMore": false,
  "nextStartTime": "2026-10-11T07:40:55Z",
  "nextEndTime": "2026-10-11T08:40:55Z",
  "logs": []
}
```

The Render response has the same shape and zero logs, with its request's 07:41:09Z–08:41:09Z window. Both windows contain the proven startup lines. The native User-Agent is `render-cli/2.27.0 (macOS - 26.5.1)`; Bex additionally sends `X-Bex-Cli-Version: 0.3.2`. CLI JSON output is a stream of log objects, so empty stdout is zero rows rather than a malformed envelope.

## API controls and limits

[API evidence](evidence/api-controls.json): corrected raw GraphQL `logs(resource: OWN_ID, type: "app", text: "ι", limit: 100) { hasMore logs { message timestamp } }` returns zero with no errors. The first diagnostic query incorrectly placed entry fields on `LogList`; its valid schema refusal is retained and discarded as bug evidence. REST with the native iota path plus `textMode=display` returns both original messages. This extension is not exposed as a pinned CLI flag. No dashboard UI failure or display-search regression is claimed.

Source diagnosis uses checkout **ac96995b3bd8b823b9ece55d0870d2b4ee8df774**. Production API/Loki revisions were not observed; matching local Loki behavior is not proof of the deployed image version. Installed Bex 0.2.1 was not the tested binary.

## Native live-tail control (sweep 95)

A fresh owned Free Node fixture `srv-db5kv4db3aac73f8m6ag`, name `qa-20261011-b47051-livetext`, reached Live/HTTP 200 and emitted no Greek lines until the controlled `/emit` request. Both native clients ran `logs --resources OWN_ID --type app --text '"QA95_b47051 café ελληνικά"' --limit 100 --tail --confirm -o json`. Their stdout was empty before the trigger. The owned HTTP request returned the exact marker and printed `QA95_b47051 CAFÉ Ελληνικά` plus `QA95_b47051 café ελληνικά`.

Both clients received both original messages exactly once. The [complete native tail receipt](evidence/live-tail-control.json) records the trigger, output, timestamps and process cleanup. The harness sent SIGINT and reaped both children: Bex POSIX signal status **-2** / 6.758 seconds, Render **-2** / 6.800 seconds, no stderr or forced kill. Its first postcondition incorrectly assumed exit 0; that harness assumption was corrected while retaining the actual receipts. This is positive live-delivery evidence, not a product interruption defect. Native SSE wire was not intercepted.

After-tail native historical reads returned empty (Bex exit 0 / 3.166 seconds; Render exit 0 / 2.934 seconds). No independent ASCII history read proved those new HTTP-emitted lines had reached Loki before deletion, so these reads alone are not an additional history reproduction. Sweep 94 independently proves its matching messages exist in history.

[Second-fixture cleanup](evidence/live-tail-fixture-cleanup.json): native delete exit 0 / 1.234 seconds; detail/list/runtime absent, all six baseline services retained, quota six used / zero terminating, no datastores. Both tails were reaped and the copied comparison config removed. The native App live-tail Unicode control now passes; datastore, MCP and dashboard UI coverage remain unverified.

## Managed Postgres reproduction (sweep 100)

Fresh owned Free PostgreSQL 18 **dpg-db5m5m09n5bc73aqiib0**, `qa-20261011-998562-pglocal`, reached available with its requested database/user. Three native Bex SQL commands completed with exit 0 and no stderr, each emitting one harmless server-log marker:

```sh
bex psql OWN_PG_ID --command "DO \$qa100\$ BEGIN RAISE LOG '%', 'QA100_998562 Ι'; END \$qa100\$;" --confirm -o json -- -X -At --set ON_ERROR_STOP=1
```

The other two commands emit lower-case `ι` and the positive control `λ`. [Exact SQL commands and results](evidence/pg-sql-markers.json) retain the literal arguments without relying on shell reconstruction. Query the supported datastore subset with `logs --resources OWN_PG_ID --text TERM --limit 100 --confirm -o json`; omit the app-specific `--type` argument.

| Term           | Expected rows | Native Bex | Native Render |
| -------------- | ------------- | ---------- | ------------- |
| `QA100_998562` | 3             | 3          | 3             |
| `ι`            | 2             | 0          | 0             |
| `λ`            | 1             | 1          | 1             |

Every case exits 0 with no stderr and captured HTTP 200. Independent whole-marker controls prove all three PostgreSQL log records are stored; expected substring matches derive from those exact original messages. All six native output streams match their captured wire counts and message bytes. [Full native requests/responses](evidence/pg-native-wire.json) and [expected/actual rows](evidence/pg-native-cases.json) confirm the existing history defect reaches managed Postgres through the generic CLI route. This extends this issue's verified resource families; it is not a new root cause or a duplicate filing.

The first app-oriented harness included `--type app`, outside the managed Postgres filter subset, and correctly received 400. That refusal is retained and explicitly qualified as expected; all six rows above use supported filters. The transparent capture forwards each actual native GET unchanged to Bex, never journals auth and removes its temporary configuration/server/process.

[Cleanup](evidence/pg-fixture-cleanup.json): a cached own SQL connection first returned `1`; native delete exited 0 / 0.624s; detail 404, native Postgres list empty, six baseline services retained, former cached SQL connection exit 2, Postgres quota used/terminating 0/0. Only this fixture's temporary CA was removed. Current source HEAD **775e23c181b7** includes the static-publication image-pin update, not a Unicode-history fix; platform runtime revision remains unobserved. Managed Postgres native raw history is now live-confirmed; Key Value markers, managed-data display/live/convenience routes, MCP and dashboard UI remain unverified.

## Managed Key Value reproduction (sweep 101)

Fresh owned Free Key Value **red-db5m9k09n5bc73aqiidg**, `qa-20261011-c081f6-kvlog`, reached available with noeviction / journal_snapshot. Normal connection setup allowed the own runner's address. Three native Bex `kv-cli` commands emitted harmless process-log markers and returned each exact marker, all exit 0 without a harness deadline:

```sh
bex kv-cli OWN_KV_ID -- --raw EVAL 'redis.log(redis.LOG_NOTICE, ARGV[1]); return ARGV[1]' 0 'QA101_c081f6 Ι'
```

The other two commands use lower-case `ι` and lambda `λ`. [Native generator receipts](evidence/kv-native-markers.json) contain exact arguments, output and completed terminal results. Use the supported native datastore history query `logs --resources OWN_KV_ID --text TERM --limit 100 --confirm -o json`, with no app-only type filter.

| Term           | Expected rows | Native Bex | Native Render |
| -------------- | ------------- | ---------- | ------------- |
| `QA101_c081f6` | 3             | 3          | 3             |
| `ι`            | 2             | 0          | 0             |
| `λ`            | 1             | 1          | 1             |

All six native queries exit 0 with empty stderr and captured HTTP 200. Independent full-marker controls prove all three exact lines are ingested; expected normalized substring matches derive from those messages. Native stdout counts and message bytes match the actual responses. [Complete native wire evidence](evidence/kv-native-wire.json) and [case expectations/results](evidence/kv-native-cases.json) show the existing root cause also affects managed Key Value history. Together with App and Postgres, both managed-data resource families are now live-confirmed through both unchanged clients. Temporary capture processes/configurations were removed.

[Owned cleanup](evidence/kv-fixture-cleanup.json): cached Redis first returned PONG; native delete exited 0 / 0.720s; detail 404 and native list empty; all six baseline services remained. The former own endpoint then returned exit 1 / no PONG, and Key Value quota finished at used/terminating 0/0. Its recorded `connection_refused:false` is retained: the probe establishes loss of Redis service, not a particular transport error string. Postgres remained empty. Managed-data display/convenience routes, MCP and dashboard UI still require acceptance verification; managed live subscriptions are deliberately unsupported (see [083](../083.md)); no new issue, fix or deployment was created by this extension.

## Root cause and consumer trace

1. Pinned `cmd/logs.go:79–83` accepts text as a StringSlice; `pkg/tui/views/logloader.go:161` forwards it. Generated `pkg/client/client_gen.go:12743–12749` emits the exploded text query. The actual request retains every Unicode byte.
2. Backend `internal/logs/rest.go:506` retains decoded text values. `service.go:332–341` lowercases/deduplicates nonempty terms without trimming; `matchesText` at 472–480 defines the local lowercase/literal matcher.
3. `internal/logs/loki.go:58–60` normalizes text and constructs the history query. `lokiQueryFor` at 497–511 escapes each term, creates `(?i)` alternatives, and delegates raw filtering to Loki. Raw history does not use the display-mode local matching path at 169 and 269–271.
4. Loki 3.6.7's [RegexSimplifier](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/log/filter.go#L705) turns a case-insensitive literal AST into a contains filter using the parser's canonical `reg.Rune`. [newContainsFilter](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/log/filter.go#L556) lowercases that representation; [containsLower](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/log/filter.go#L448) then compares ordinary lowercased line runes. These operations do not preserve the parser's complete fold class.
5. The actual pinned regexp dependency `github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc` represents `(?i)ι` as literal **U+0345**, a combining mark. Lowercasing leaves that mark; it differs from ordinary **U+03B9** iota in either fixture line. `(?i)ελληνικά` has the same substituted mark inside the canonical literal. The full regexp engine matches both messages; the simplified literal does not.

The [actual local Loki receipt](evidence/local-loki.json) pushes two UTF-8 messages and queries unmodified `grafana/loki:3.6.7`: unfiltered 2; `(?i)café` 2; `(?i)ελληνικά`, `(?i)λληνικά`, combined term all 0; case-sensitive `|= "ελληνικά"` 1. It uses only its own temporary Docker Desktop container and filesystem; exact-ID/label deletion and absence are recorded. The [canonical-literal probe](evidence/canonical-literal-probe.json), [source](evidence/canonical-literal-probe.go.txt), [module](evidence/canonical-literal-probe-go.mod.txt) and [sum](evidence/canonical-literal-probe-go.sum.txt) run the real regexp parser and engine, compare Go's existing lowercase/literal semantics, and expose the canonical literal mismatch. The probe does not instantiate the whole Loki simplifier; the separate real-runtime receipt verifies it.

## Neighboring native failures (sweep 96)

Fresh owned Free Node fixture **srv-db5l3ijcgaac73d05ft0**, name `qa-20261011-394cfd-casefold`, reached Live / exact HTTP 200. Both clients' full-marker capture returned all twelve original lines before their word searches:

```text
QA96_394cfd РУССКИЙ
QA96_394cfd русский
QA96_394cfd ΡΟΔΟ
QA96_394cfd ροδο
QA96_394cfd KELVIN
QA96_394cfd KELVIN
QA96_394cfd kelvin
QA96_394cfd İSTANBUL
QA96_394cfd ISTANBUL
QA96_394cfd istanbul
QA96_394cfd Ι
QA96_394cfd ι
```

Use `logs --resources OWN_ID --type app --text TERM --limit 100 --confirm -o json` in either native client. Quote an ASCII-prefixed combined term as one CSV value, for example `--text '"QA96_394cfd istanbul"'`.

| Text term              | Expected rows | Bex rows | Render rows |
| ---------------------- | ------------- | -------- | ----------- |
| `QA96_394cfd`          | 12            | 12       | 12          |
| `русский`              | 2             | 1        | 1           |
| `ροδο`                 | 2             | 1        | 1           |
| `kelvin`               | 3             | 2        | 2           |
| `istanbul`             | 3             | 2        | 2           |
| `ι`                    | 2             | 0        | 0           |
| `QA96_394cfd русский`  | 2             | 2        | 2           |
| `QA96_394cfd istanbul` | 3             | 3        | 3           |
| `QA96_394cfd kelvin`   | 3             | 3        | 3           |

All eighteen native processes exit 0 with no stderr; every captured response is HTTP 200. [Complete requests/responses and native receipts](evidence/neighbor-native-wire.json) and [expected/actual original messages](evidence/neighbor-native-cases.json) preserve the missing capital-letter rows. Both full-marker controls pass, independently proving ingestion. A stricter prefixed literal returns three Istanbul/Kelvin lines while the broader word returns two: adding a required literal prefix cannot legitimately increase substring matches. This evidence does not depend solely on a choice between Unicode folding conventions.

The [display-mode API controls](evidence/neighbor-display-api.json) return both Greek rho lines, but return only two for `QA96_394cfd istanbul`, while native raw history for that exact term returns three. All fixture messages are plain text, so display projection leaves them unchanged. This is a confirmed display-mode REST mismatch; the dashboard UI was not driven. Its source opts into display mode, so its consumer belongs in blast-radius verification. The earlier Greek-iota display control still passes.

[Real local Loki 3.6.7](evidence/neighbor-local-loki.json) reproduces the same word and prefix counts. [Source-model results](evidence/neighbor-model.json), [source](evidence/neighbor-model.go.txt), [module](evidence/neighbor-model-go.mod.txt) and [sum](evidence/neighbor-model-go.sum.txt) distinguish canonical-literal loss, the first-byte check and full-regexp behavior. Its 3 canonical and 250 leading-byte candidate case pairs are model counts under the recorded compiler, not 253 individually confirmed production failures.

In `containsLower` at [Loki filter.go:456–471](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/log/filter.go#L456), the initial byte check lowercases ASCII bytes only. Uppercase Cyrillic Р starts with UTF-8 byte D0 while lowercase р starts D1; uppercase Greek Ρ starts CE while lowercase ρ starts CF. Kelvin K and dotted capital İ also cannot pass the ASCII first-byte test for `k` / `i`. With an ASCII prefix, the inner rune loop at 498–527 instead applies `unicode.ToLower` and matches them. A full-regexp-only correction would still differ for dotted İ: the actual regexp engine does not match it with `(?i)i`, while the platform's local lowercase matcher and raw prefixed native control do. Preserve the established normalized literal semantics rather than assuming all ASCII query terms are safe.

[Cleanup](evidence/neighbor-fixture-cleanup.json): native delete exit 0 / 1.604 seconds; detail/list/runtime absent; all six baseline services retained; quota six used / zero terminating; no datastores. The local reproduction and proposed-prefilter containers were each deleted by exact recorded ID/label and verified absent. The checkout used for this extension is **dd01a90cd2cd023d3ec02249874cbab5b6ec30ca**; deployed API/Loki revisions remain unobserved.

## Proposed fix and blast radius

Make Loki filtering a conservative superset, then apply the shared normalized literal matcher before limiting results. Generalize the existing bounded display-search scan for affected raw searches and correct display-mode prefiltering too. Checking only whether the query contains non-ASCII is insufficient: ASCII `kelvin` and `istanbul` fail against Unicode input. Forcing the full regexp engine alone also fails the existing dotted-İ lowercase semantics.

A concrete prefilter is a safely escaped, scoped case-insensitive term union plus a **case-sensitive non-ASCII fallback**: logical regexp `(?i:escapedTerms)|[^\x00-\x7F]`; display mode also retains its ESC/CR candidates. Construct the actual LogQL string using the existing quoted serializer. The [actual local superset probe](evidence/neighbor-superset-probe.json) verifies every expected Russian, Greek-rho, Kelvin, Istanbul and iota message survives that candidate selection. Extra rows are intentional and must be rejected by `q.matchesText` in the requested raw/display mode. Scope `(?i:...)` to the terms; applying it globally to the negated ASCII class also folds Kelvin into the excluded class. The proposal is verified locally, not implemented or deployed.

Fetch bounded raw pages and advance coverage using raw lines, not only matches. Preserve the 18-second budget, 1000-line scan pages, inclusive timestamp handling, timestamp-group re-read, direction, original messages and honest incomplete-coverage cursors. A single unfiltered/raw page cannot certify the remaining history as empty. Retain the no-text fast path and selective ASCII filtering where the superset condition is proven; measure dense-log queries rather than treating ASCII-only terms as a correctness guard. Keep structured/resource/time filters and unsupported-source refusals. Do not alter the CLI or simply bump a dependency without real-runtime evidence.

`cmd/api/main.go:564` wires one production `NewLokiSource`. Generic REST, GraphQL (`logs/graphql.go:149–160`) and MCP (`logs/mcp.go:98–108`) share the query. Production Postgres and Key Value convenience adapters delegate to the same generic core (`internal/api/server.go:762,765`; `postgres/logs.go:45–48`, `keyvalue/logs.go:46–48`). App/request/build history and managed-data history share this boundary; managed Postgres and Key Value native raw history now reproduce above. Pod fallback and live followers use local matching and must remain consistent; the native App live-tail Unicode control above passes. Platform history shares the source but was not probed through admin access. Dashboard service Logs opts into display mode (`dashboard/src/features/logs/components/log-viewer.tsx:154`), whose Greek diagnostic REST controls pass but the plain-text dotted-İ diagnostic fails. The dashboard UI and other consumers were not exercised; the separate native managed Postgres reproduction is recorded above.

Regression coverage must include real Loki iota/word and leading-byte failures, ASCII-query Kelvin/dotted-İ cases, stricter-prefix monotonicity, lambda/café positives, original bytes, raw/display parity, mixed ASCII/Unicode OR terms, empty/retained-whitespace behavior, literals, page saturation before a matching Unicode line, both directions, inclusive nanosecond endpoints and honest partial coverage. Add composed REST/GraphQL/MCP and datastore coverage rather than parser-only or request-string-only tests.

## Dedupe, setup qualifications and cleanup

Open/blocked/done board searches for Greek, iota, Unicode log search and case-insensitive Unicode found no equivalent filing. [w2/m167 consumer matrix](../../w2/done/m167/consumer-matrix.md) explicitly qualifies unverified broad fold equivalence, but does not contain this live false-empty bug or an open fix. This differs from w8/082 fractional time loss, m57 comma form loss and w4/m189 display-text matching. No upstream issue or external message was submitted; web index searches were inconclusive, not exhaustive dedupe.

The first fixture create embedded the long message directly and was correctly refused by the 4096-byte start-command limit. Its exact nonce was absent from inventory; no resource was created. The revised unique fixture generated that line at runtime. Initial 404/503 responses occurred before Live and are not findings.

[Fixture cleanup](evidence/fixture-cleanup.json): native delete exit 0 / 1.389 seconds; API detail 404, exact ID absent, former runtime 404, all six baseline services retained, quota six used / zero terminating, no Postgres/Key Value resources. No existing resource was mutated. Local container and capture processes are also gone. Fix estimate: 2h35m across six tasks; no code change, commit, push or rollout was performed by QA.

## Adjacent scope boundaries

Sweep 102 confirms managed-data live subscriptions are deliberately unsupported. [w8/083](../083.md) files their silent WebSocket refusal separately; do not require managed live Unicode delivery or implement followers in this history fix. Preserve supported App/build follower matching. [w4/256](../../w4/done/256.md) independently owns the historical/live message-size boundary. If its cap lands, exact raw matching must preserve the original stored-byte predicate before returned-message capping, while display matching follows the declared capped display projection. Keep both contracts in composed checks; do not silently let an unrelated cap drop an otherwise valid raw match. The native messages in this finding remain within that size boundary.

## Sweep179 regression — 2026-10-12

[Exact native captures and realistic current-source diagnostic](evidence/live-app-sweep179/README.md) reveal false completed empty raw history behind 1,000 newer nonmatches. Existing m58 already requires this boundary; t007 schedules the 45m repair and caller/closing tasks reopen. Earlier Unicode and managed-data controls remain valid. Root cause is filtering raw candidates before scan coverage, rather than the original Unicode regexp loss. No duplicate issue, product fix or ship by QA.
