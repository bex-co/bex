# Multi-value log text searches silently use only the first value

**Severity: major. Attribution: Bex server.** A successful CLI search silently omits matching log lines. Reversing the same two text values changes which line is returned. This can hide relevant diagnostics without an error or pagination hint.

## Context and expected contract

Observed 2026-10-02 around 09:31–09:35 UTC at `https://api.bex.co/v1/`, ordinary human device OAuth in workspace `bex` (`tea-d98210cbbpdc73dcrkvg`). Installed `/opt/homebrew/bin/bex` v0.2.1 imports Render v2.27.0 at `a764810a768202704e7206eb7b87a47211fcd98e`. An unmodified binary built from that exact pin reproduced against Bex in separate private CLI state.

Source diagnosis is at `ec1fe500fc9f84fe650b465f5479412bca13a9e2`; fetched origin `1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e` has no logs-package delta. The deployed API revision was not independently read. Latest successful deployment observed earlier was workflow 36978420030; later runs were still queued/pending. This is not inferred to be a deployment-lag fix: both faulty reductions remain in the fetched source.

The pinned `cmd/logs.go:75–78` defines `--text` as a string slice. `pkg/tui/views/logloader.go:162` forwards the entire array; `pkg/client/client_gen.go:12743–12749` uses exploded form serialization, yielding repeated `text` query parameters. Its generated API type is an array (`pkg/client/logs/logs_gen.go:275`). [Render's CLI reference](https://render.com/docs/cli-reference#logs) describes comma-separated text values, and [List logs](https://api-docs.render.com/reference/list-logs) declares a text array. Bex's own `docs/ADR010-observability.md:87` explicitly specifies OR within one filter and AND across filters. The two-value result must be independent of order and contain both matches. No Render production account was probed; the exact client, official documentation and Bex's established OR contract are the oracles.

## Live reproduction and controls

Owned free native-Go fixture: `qa-20261002-72bd71-health`, `srv-davnfbude41s73cantkg`, public repository `https://github.com/bex-co/bex`, branch `main`, root `examples/hello-go`, build `go build -o app .`, plan `free`, auto-deploy off. The tested service had a relative `healthz` path from a separate validation control; it was live and healthy. Recreating this log probe can use the ordinary `/healthz` path.

The CLI start-command update below scheduled live deploy `dep-davnivude41s73cantqg`. It printed three harmless JSON lines at 09:30:58Z and continued serving the example app:

```sh
bex services update "$SERVICE_ID" --start-command 'printf '"'"'%s\n'"'"' '"'"'{"level":"error","msg":"qa_level_72bd71_error"}'"'"' '"'"'{"level":"warn","msg":"qa_level_72bd71_warn"}'"'"' '"'"'{"level":"info","msg":"qa_level_72bd71_info"}'"'"'; exec ./app' --confirm -o json

bex logs -r "$SERVICE_ID" --type app --limit 20 --text qa_level_72bd71_error,qa_level_72bd71_info -o json
bex logs -r "$SERVICE_ID" --type app --limit 20 --text qa_level_72bd71_info,qa_level_72bd71_error -o json
```

| Probe | Actual | Exit / duration |
| --- | --- | --- |
| Single common prefix `qa_level_72bd71` | All three lines | 0 / 3.246s |
| `error,info` text markers | Only the error line | 0 / 3.103s |
| `info,error` text markers | Only the info line | 0 / 2.893s |
| Fresh Bex process, `error,info` repeat | Only error | 0 / 5.321s |
| Unmodified same-pin Render, `error,info` | Only error | 0 / 4.745s |
| REST repeated text parameters, both orders | HTTP 200, only the first term's line, `hasMore:false` | Complete in evidence.json |
| MCP `list_logs` text array, both orders | HTTP 200/tool success, only the first term's line, `hasMore:false` | Complete in evidence.json |
| Common text prefix + individual error/warning/info levels | Exactly the corresponding line | All exit 0 |
| Common text prefix + levels `error,warning` | Error and warning lines | 0 / 3.063s |
| Common text prefix + debug level | Honest empty | 0 / 2.705s |

The controls establish that both omitted/matched records exist, are in the same instance and time window, and are queryable. This is not a log-ingestion, permission, level-classification, page-size or CLI-rendering failure. Newly emitted `warn` is correctly labeled `warning`.

## Root cause and concrete repair

- **REST reduction:** `lego/backend/internal/logs/rest.go:496–505`, `parseLogParams`, sets `Search: v.Get("text")`. `url.Values.Get` returns only the first occurrence.
- **MCP reduction:** `logs/mcp.go:54,97–106` accepts `Text []string`, then sets `Search: firstNonEmpty(f.Text)`. The helper at `:200–209` explicitly documents this implementation limitation. That internal comment does not make an advertised public array or the ADR's no-ignored-filter promise truthful.
- **Shared scalar model:** `logs/service.go:257,276,310–327` stores one `Search string`, precomputes one lowercased term, and determines filtered reads from it. `keep` at `:448–450` performs one substring check.
- **Stored-log consumer:** `logs/loki.go:400–406` emits one escaped case-insensitive LogQL line filter. `:114` decides whether the expensive line-search time budget applies. Both must follow the complete normalized term set.

Carry a canonical set of text terms through the shared query model. Preserve all REST occurrences and MCP values; wrap scalar callers as a singleton. Match any supplied term within text, then retain existing AND composition with levels/resources/time and other filters. Loki needs one safely escaped OR predicate, not concatenating the terms into one literal string. Pod and progress paths must evaluate the same OR rule. Normalize empty and duplicate values deliberately, preserving no-filter behavior when no effective terms exist. Do not alter the imported CLI or introduce unrestricted regex processing while fixing arity.

## Counted blast radius

- **Three REST handlers** share `parseLogParams`: label values (`rest.go:67`), historical logs (`:99`), subscribe (`:134`). Two return log lines; label-value discovery intentionally does not evaluate text/path/host against its stream index, so retain that documented limitation.
- **Two MCP tools** share `logFilters.query`: `list_logs` (`mcp.go:145`) and `list_log_label_values` (`:179`). Apply the same label-discovery distinction.
- **Scalar consumers:** GraphQL `logs`/`logLabelValues` use `logQueryFromArgs` (`graphql.go:146–155`); the dashboard sends `$text: String` (`dashboard/src/features/logs/api/logs.graphql:20,33`). Preserve that public schema. `api/server.go:595–606` adapts the scalar datastore convenience query once for both Postgres and Key Value; retain its existing callers.
- **Three resource families** use the generic query: Apps, managed Postgres and Key Value. **Five Go filtering sites** call `q.keep`: `service.go:707,1017,1208` and `progress.go:396,482`, covering datastore/pod/progress collection and followers. Stored history additionally uses the Loki line filter.
- **Paging/budget seam:** `loki.go:114` and `service.go:326` must recognize multi-term searches. Preserve w4/m140's bounded search and continuation behavior, authorization/resource equality selectors, instance translation and the existing output envelopes.

Live observations cover stored App logs via CLI, REST and MCP. Live multi-term tails, build/pre-deploy logs, request logs, managed datastores, GraphQL scalar controls, label-value discovery and multi-resource pagination are **not separately exercised** for this finding; they are explicit regression work, not claimed live failures.

## Dedupe and boundaries

Searched open/done `.pm` for `firstNonEmpty`, `Get("text")`, first/multiple text values, comma-separated filters, and ignored text terms. No matching repair was found. `git log -S` attributes both reductions to `d45919d47` (w3/m8), so this is an original compatibility gap rather than a regression of later work. Adjacent items: w8/024 fixed multiple **resources** in a tail; w8/031 fixed level vocabulary; w9/071 fixed logfmt classification; w4/m140 fixed broad-search timeouts. The latter three contracts remain relevant controls. This run's structured-level tail returned the named documented pod-tail limitation, which is not included in the finding. Full regex support and expanding supported tail filters remain outside scope.

## Cleanup and evidence

All five deploy children were creation/mutation-proven and listed before deleting the owned parent; no custom domains existed. CLI deletion exited 0 in 2.227s. Detail returned 404, the CLI service list omitted the exact ID, and its former public URL returned 404. No resource from this sweep survives.

[evidence.json](evidence.json) contains complete sanitized CLI process records, diagnostic REST request paths/headers/responses and MCP request/SSE-response bodies. The REST captures replay the pinned serialization with the same human authority; they are diagnostic requests, not an intercepted CLI trace. They omit Authorization and contain only owned fixture metadata and harmless canaries. No application changes or tests were implemented by this filing.
