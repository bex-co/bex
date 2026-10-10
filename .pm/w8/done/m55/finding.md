# Accepted comma names cannot be resolved by CLI update and clone

- **Severity:** minor; named update and clone are blocked, while ID selection and the running site remain available. No data loss or outage claim.
- **Attribution:** Bex server compatibility. Both published Bex v0.3.2 and actual unmodified Render v2.27.0 / `a764810a768202704e7206eb7b87a47211fcd98e` reproduce the failure against Bex.
- **Context:** 2026-10-09 UTC, functional CLI QA sweep 51, `https://api.bex.co/v1/`, `bex-canary` / `tea-daif693dqjvc73e7as3g`, isolated human device OAuth, non-TTY JSON. Checkout `c5e413f38324fdff0a36f4c3edc0063e889ce639`; actual deployed revision unobserved.
- **Source and goal:** ongoing user-requested w8 bug hunt; ADR008 reliable hosting, ADR006 API compatibility, ADR018 pinned CLI contract and completed w8/034 display-name admission.
- **Estimate:** 3h45m across seven tasks; filing only, no product implementation, external submission, commit or push.

## Durable evidence

[Full commands, outputs, timings, own-resource API diagnostics, controls and cleanup](evidence/repro.json); [actual pinned request-builder probe](evidence/probe.go.txt), [module](evidence/go.mod.txt), [checksums](evidence/go.sum.txt), and [name-parameter schema census](evidence/name-contracts.json).

The fixture was a new owned Free static site `srv-db4et2r4am4s73f096fg` / `qa-20261009-933205-xstatic`, public `bex-co/bex`, root `examples/static-site`, publish `.`, no build command, auto-deploy off. Static creation at this pin omits `--plan`, which is forbidden for this type; backend normalization chooses Free. Its initial deploy `dep-db4et2r4am4s73f096g0` became Live, and the original immutable public URL served the expected HTML with HTTP 200. All fixtures are now deleted.

## Reproduction

Rename that exact owned ID:

```sh
bex services update srv-db4et2r4am4s73f096fg --name 'qa-20261009-933205-csv,雪%+&=' --confirm -o json
```

Exit 0 in 1.705s, empty stderr. API detail and the native unfiltered services list both return the exact accepted display name on the same ID. The service continues serving its original URL.

Now update the accepted name without changing its existing auto-deploy setting:

```sh
bex services update 'qa-20261009-933205-csv,雪%+&=' --auto-deploy=false --confirm -o json
```

Bex exits 1 in 0.329s; unmodified Render exits 1 in 0.467s. Both stdout captures are empty. Complete Bex stderr:

```text
Error: failed to resolve service "qa-20261009-933205-csv,雪%+&=": No service named 'qa-20261009-933205-csv,雪%+&=' in workspace tea-daif693dqjvc73e7as3g. To search another workspace, run `render workspace set <name|ID>`, or pass the service ID instead.
```

Ordinary cloning by this same accepted name also fails:

```sh
bex services create --from 'qa-20261009-933205-csv,雪%+&=' --name '<FRESH_QA_NAME>' --confirm -o json
```

Bex exits 1 in 0.369s and unmodified Render exits 1 in 0.571s; stdout is empty. Both errors wrap the same `No service named` refusal as `failed to clone configuration from source service ...: failed to resolve source service ...`. The artifact contains the complete errors and exact distinct fresh names. Both create intents remain absent; no blind retry or deletion of an ambiguous object occurred.

Expected: a valid, accepted display name identifies the service for these supported named commands. The failed no-op update leaves its saved name and settings in place. The message's embedded Render wording is an existing upstream residual, not a second finding here.

## Isolating controls

| Journey | Observed result |
| --- | --- |
| Deploy list by comma-containing name | Both clients exit 0 |
| Deploy list by opaque ID | Both clients exit 0 |
| Native unfiltered service list | Exact accepted comma name and owned ID present |
| Bex clone by opaque ID while the comma name is saved | Exit 0 in 2.215s; `srv-db4ev334am4s73f096hg` becomes Live, HTTP 200 expected HTML |
| Rename only removes comma, retaining `雪%+&=` | Exit 0; exact API readback |
| Named no-op update with the no-comma control | Bex exit 0 in 1.464s; Render exit 0 in 1.430s |
| Render named clone with the no-comma control | Exit 0 in 1.524s; `srv-db4evn393q6c73at0340` becomes Live, HTTP 200 expected HTML |

Thus direct name-path handling works; the failure is specific to commands that resolve through the filtered inventory. Unicode, percent, plus, ampersand and equals are preserved in the no-comma control. This is not the known clone-region or one-sided build-filter issue.

## Request and response

An **API-only diagnostic**, with the same isolated QA authority, reproduced the generated request shape exactly:

```text
GET /v1/services?name=qa-20261009-933205-csv%2C%E9%9B%AA%25%2B%26%3D
HTTP 200
[]
```

The complete method/path/status/body and relevant headers are in the artifact. No native live wire interceptor was enabled. This API-only result and the actual offline pinned request builder establish the request/parser boundary; they are not a claim of captured native HTTP traffic.

The offline program calls the actual `client.NewListServicesRequest`, without network. It produces:

| Input name array | Raw query |
| --- | --- |
| One name `qa-literal,雪%+&=` | `name=qa-literal%2C%E9%9B%AA%25%2B%26%3D` |
| Two names `qa-first`, `qa-second` | `name=qa-first,qa-second` |
| Ordinary name plus a literal-comma name | `name=qa-first,qa-literal%2C%E9%9B%AA%25%2B%26%3D` |
| One literal-percent name `qa-literal%2C` | `name=qa-literal%252C` |

An encoded comma inside an element is distinguishable from a raw comma separating elements before query decoding. The parser discards that distinction.

## Root cause and concrete fix

`lego/backend/internal/apps/rest.go:674–684` calls `r.URL.Query()` and then `core.QueryList(q, "name")`. `core/pagination.go:33–44` splits each already-decoded value on commas. The literal `%2C` from the actual pinned serializer becomes a comma before splitting, turning the one complete display name into two unrelated filter values. `apps/rest.go:711` then compares those fragments with the immutable name and displayed name, producing the empty list.

The client is a faithful consumer: `pkg/service/repo.go:196–213` constructs `NameParam{query}` and keeps exact returned-name matches; no match becomes the observed named refusal. `cmd/serviceupdate.go:124` uses that resolver before updating; `cmd/servicecreate.go:166` uses it before reading clone configuration. The pinned generated `pkg/client/client_gen.go` `NewListServicesRequest` uses `StyleParamWithOptions("form", false, "name", ..., Type:"array")` and retains the resulting pre-encoded raw fragments. The offline probe proves those generated bytes. Changing the launcher or request construction would mask a valid upstream request.

Introduce one request-aware name-array parser that retains raw query value boundaries. Split **unescaped raw commas** before decoding individual elements, and decode each element exactly once; `%2C` is a literal comma, while `%252C` is a literal `%2C` substring. Preserve repeated-key OR behavior, existing trimming/deduplication and error handling. Feed correctly parsed names into the existing matching/filter/pagination path. Start with the confirmed `/services` field, and apply the same helper only to audited, declared string-array name filters. Keep the existing value-only helper for unaffected fields rather than blindly replacing all calls.

The compatibility review must distinguish canonical raw delimiters from legacy callers that percent-encode a whole comma-separated list. The latter is ambiguous with a single literal-comma name; do not hide this by guessing from inventory matches. Specify the field-local interpretation from the pinned serializer/schema, audit repository callers/tests and document any required caller correction. Do not reject valid display names, truncate them, or broaden results to include both fragments and the complete name. Preserve current filter conjunction and exact matching.

No name storage, routing, deploy-history or display-name validation change is needed. The source's detail read, unfiltered list, direct named deploy path and running URL prove those producers preserve the name. REST list parsing is the observed fault; GraphQL/MCP scalar name inputs and dashboard rename behavior were not live-tested.

## Shared consumers and bounded verification

There are **37 production `core.QueryList` invocations in nine files**. Nine are name-filter sites, in seven files:

- `apps/rest.go:684` services; `:1122` jobs; `:1235` static headers.
- `apps/disks.go:404` disks.
- `postgres/rest.go:97`, `keyvalue/rest.go:245`.
- `projects/rest.go:136`, `environments/rest.go:43`, `envgroups/rest.go:29`.

These are a source census, not nine live reproductions. The contract artifact records 13 GET operations with a `name` parameter: thirteen arrays: twelve explicitly declare form/explode-false, while owners omits those declarations. Some declared operations use other parsers, aliases, stricter resource-name grammars or unsupported product surfaces. Classify them explicitly; do not apply an explode-false parser to the owners field without its separate declared-style audit, widen name admission, or provision paid fixtures to test the audit. The jobs name filter is not in the pinned name-parameter census and must retain its existing extension contract unless deliberately included.

The shared service resolver also serves native delete; its failure here is predicted by source, **not live tested**. Native log lookup has a separate list-based name resolver, also inspected only. Confirm update and clone with the original commands; verify other callers with meaningful contract tests and appropriate safe fixtures. Generic path/value comma filters are adjacent audit leads, not certified live findings.

Regression cases must use the actual pinned generated builder and composed server, then exercise the real service resolver. A stubbed resolver or parser-only string test cannot certify CLI compatibility. Required cases include one literal-comma name, two separate names, a mixed array, repeated keys, duplicates, whitespace behavior, literal percent/plus/ampersand/equals, `%252C`, omitted/no-match and pagination/other-filter composition. Preserve closed enum/ID filters and other parameter styles. Malformed query decoding must keep the existing named refusal behavior rather than silently losing a value.

## Dedupe, source status and cleanup

Searched open/blocked/done board items for comma names, encoded commas, `%2C`, name filters, decoding before splitting and `QueryList`. Existing w8/034 covers name admission, w1/m168 owner arrays, w1/110 suspension arrays, and w6/075 region arrays; none covers an encoded literal comma within a name. w8/m54 concerns build-filter body serialization, while w8/075 concerns cross-type clone defaults. Both successful static controls here exclude those symptoms. The CLI compatibility ledger and anti-goals were checked; this is a server fix, with no CLI fork or new admission rule.

Targeted history for `core/pagination.go` shows the helper's shared filter lineage (`75e0018c8`, then `a0bd18186`); current main still decodes before splitting. No implementation fixing this was found on main, so this is not a new filing for known deploy lag. Actual production revision remains unobserved. Live Render API behavior was not exercised; the pinned schema and actual unmodified CLI are the compatibility oracle. A web fetch of the immutable upstream raw page was unavailable; the locally installed exact module and actual generated-builder execution provide the pinned source evidence.

Both successful clones were deleted before the source by exact ledger-owned IDs after child enumeration. All three creation proofs and initial baseline exclusions are preserved. Detail/list/former-public-URL checks show absence/404, and both rejected fresh-name intents remain absent. Quota first showed **9 used / 3 terminating**, then read-only reconciliation reached **6 used / 0 terminating**, PG 0 and KV 0, with all six baseline service IDs present. No second delete was sent. Kubernetes artifact UIDs were not independently inspected.

The copied Render config was removed without logging out the copied grant; the original isolated human Bex grant remains active for the requested loop. This milestone schedules researched work and does not claim implementation or post-fix acceptance.
