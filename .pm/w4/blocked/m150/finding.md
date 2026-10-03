# Static header wildcard patterns save but never match

Why: a customer can save a file-type or nested-path header policy, see it preserved across every read surface, and still serve responses without that policy.

**Severity:** major — silent loss of configured response-header behavior. This run used harmless, unique `X-QA-*` markers; no security exploit or security-header mutation is claimed. **Source:** continuous `$qa-find-bugs`, sweep 11, 2026-10-02 UTC, using the user-selected `muse.env` and w4 destination. **Research HEAD:** `8ddc8a746`.

## Reproduction and target

1. In workspace `bex` (`tea-d98210cbbpdc73dcrkvg`), create a free Static Site through Public Git URL: `https://github.com/bex-co/bex`, branch `main`, Root Directory `examples/static-site`, Publish Directory `.`, no build command, auto-deploy off. This run's name was `qa-20261002-static-r11`, id `srv-davojns5o9vs73dt7r6g`, first deploy `dep-davojns5o9vs73dt7r70`, commit `8ddc8a7`. It reached Live and served the sample.
2. Open `https://dashboard.bex.co/static/srv-davojns5o9vs73dt7r6g/headers`. Save an exact `/render.yaml` header and a `/*.yaml` header with different names. Reload. Both remain saved; only the exact header reaches GET and HEAD of the existing file.
3. Change the **same** failing rule's path to `/render.yaml`, retaining name/value. After the resolver refresh it appears. Change it back to `/*.yaml`; add `/**/*.yaml`, `/**/*`, and a `/nested/*` control; also change the `/*` marker to `r11-glob-v2`. Reload and wait until that new marker reaches the origin.
4. Probe an existing root file, a missing root file, a missing nested file and a nested directory. The wildcard selectors fail while exact, catch-all and subtree controls work. The service stays at `rev-1`.

| Saved path | Request | Target | Observed after refresh |
| --- | --- | --- | --- |
| `/*.yaml` | `/render.yaml`, 200 | `X-QA-Pattern: r11-root-yaml` | Missing; exact and catch-all headers present |
| `/*.yaml` | `/missing-r11.yaml`, 404 | Same pattern header | Missing; catch-all present |
| `/**/*.yaml` | `/nested/missing-r11.yaml`, 404 | `X-QA-Nested: r11-nested-yaml` | Missing; prefix and catch-all present |
| `/**/*` | `/nested/`, 200; nested missing file, 404 | `X-QA-Nested-All: r11-nested-all` | Missing on both |
| `/render.yaml` (same formerly failing rule) | `/render.yaml`, 200 | `X-QA-Pattern: r11-root-yaml` | Present after refresh |
| `/nested/*` | Nested directory and missing file | `X-QA-Prefix: r11-prefix` | Present on 200 and 404 |

[Render's primary header documentation](https://render.com/docs/static-site-headers), checked 2026-10-02, distinguishes root extension patterns (`/*.css`) from nested extension patterns (`/**/*.css`), and defines `/**/*` for paths with at least two slashes. This sweep exercised that extension grammar with YAML. Authenticated Render execution was **not** performed.

**Required behavior:** implement documented header-path matching, including the root/nested boundary, in normal and resolved-error responses. Preserve exact, global and subtree patterns. Keep route matching and `:splat` expansion compatible. Rejecting these valid Render header patterns or merely changing the save message does not satisfy the finding.

## Root cause and implementable fix

- **Producer:** `lego/backend/internal/apps/service.go:4560–4588` validates a bounded, slash-rooted path and valid name/value; `:4704–4720` authorizes, validates, gates to static sites and patches `spec.headers`. `:4475–4485` only trims Path/Name and preserves Value. The three adapters share this path.
- **Type:** `lego/types/v1alpha1/app_types.go:1128–1145` stores Path as a string bounded to 2048 characters; the headers slice has a 100-item cap. `apps/graphql.go:82–88` uses a non-null String path and `:224–240` copies it into the neutral view. No schema or serializer needs to interpret a glob; the actual replies prove the patterns survive.
- **Consumer:** `lego/operator/internal/staticserver/resolver.go:65–98` copies `app.Spec.Headers` into the Site snapshot. Polling at `:101–119` explains normal propagation delay. The updated catch-all marker and exact control rule rule out a stale snapshot.
- **Failure:** `staticserver.go:482–497` handles `/*`, a literal prefix with trailing `/*`, or exact equality. Thus `/*.yaml` and `/**/*.yaml` are literal comparisons; `/**/*` becomes a literal `/**` prefix. No third-party glob library participates: the actual matcher uses Go string operations/equality. Both header consumers call it, at `:514` and `:537`; emission is behind the false predicate.
- **Control explanation:** exact, catch-all and subtree probes take those three implemented branches. They use the same consumer, without a caller workaround. `applyErrorHeaders` excludes body metadata names, not these markers. `http.Header.Set` ordering cannot explain the result because every marker name is unique.
- **Fix:** add bounded header-pattern matching and use it in the two header consumers. Leave the route matcher/capture expansion compatible. Do not adopt a glob package's zero-directory `**/` behavior without checking Render's root/nested examples. Preserve normalized **request** path matching, existing-file precedence, header ordering, error exclusions, refresh timing and rule/path budgets. Avoid unbounded or backtracking work per public request.
- **Pre-settle state:** the dashboard accurately shows persisted configuration before the next resolver refresh; tests must wait for the changed catch-all marker before judging served behavior. No origin-effect claim should be inferred from mutation completion alone.

## Counted blast radius and aliases

Repository-wide production searches, excluding tests:

- **3 calls to `matchPattern`:** `matchRoutes` at `staticserver.go:463`, `applyHeaders` at `:514`, `applyErrorHeaders` at `:537`. Allowlist the change to the **2 header calls**; route matching and `expandDest` stay compatible.
- **2 applyHeaders callers:** redirect at `:265`, successful object at `:343`. **2 applyErrorHeaders callers:** `serveSiteError` at `:549`, `serveSiteBusy` at `:557`. Test normal, rewrite, redirect and resolved-error classes, GET and HEAD.
- **3 SetHeaders adapter callers:** `apps/rest.go:1228`, `apps/graphql.go:1748`, `apps/mcp.go:1069`. Writes are `PUT /v1/services/{id}/headers`, `setStaticHeaders`, `update_static_headers`. Read aliases are the corresponding REST GET, GraphQL `service`/`server` headers, and `list_static_headers`. Individual-rule APIs remain the ADR018 non-goal.
- **2 headersFromViews callers:** create spec at `service.go:2776` and SetHeaders at `:4719`; create validates at `:2664`. Blueprint accepts headers at `blueprint_compiler.go:485` and projects them at `blueprint_plan.go:234`; it shares the served consumer without being another SetHeaders caller. Create-time/Blueprint round trips are verification work, not live coverage claimed here.
- **1 editor mount:** `HeadersEditor` (`static-site-section.tsx:323`) is mounted by `routes/services.$serviceId.headers.tsx:23`; `routes/static.$serviceId.headers.tsx:13` reuses the same page. Both URL families use `use-static-site.ts:98–115` and the GraphQL mutation.
- **All 7 resource families accounted for:** static sites use this server; web, private, cron and worker are excluded by `requireStaticSite` and the resolver's type gate; Postgres and Key Value have separate controllers/protocols.
- **Adjacent classes:** preserve API authentication/authorization and not-found/forbidden handling. Public unknown-host 404 and method 405 stay headerless. Resolved 400/404/413/502/503 keep their current policy, error-body metadata protections and platform Retry-After precedence. No fallback-to-success or existence disclosure change.

## Dedupe and deployment

Searched open, blocked and done Markdown for `matchPattern`, static/header glob/wildcard/pattern combinations and route/header parity; scanned **217 active Markdown files / 61 active READMEs** across workstreams. No open item covers this grammar. w4/m146, w4/m110→w5/m105 and old editor/layout notes concern different behavior. w9/m89/m92 do not cover this selector gap.

This is a **residual grammar gap from w1/m21**, not a recurrence of the distinct w4/m94 or w4/m101 fixes. Full original DoD disposition follows below. DO_NOT_DO excludes CDN cache purge and individual-rule APIs; existing header rules are explicitly in scope. ADR029's trailing-wildcard paragraph specifies **route Source/capture** semantics. Its accepted implicit SPA fallback divergence is also separate. Neither records an exception for Render's header wildcard table.

Checked the latest 40 dashboard/lego commits and targeted history. `git log -S 'func matchPattern'` traces to original `84e221650`; no pending main fix. Production static-server uses `ghcr.io/bex-co/bex-operator@sha256:7bc6535cde1da49bcdc406781e2b2cfcd9e8a99a8bc8ba212d8418b84ae0951c` (the `e97ca42273a9` platform train). `git diff e97ca42273a9..HEAD -- lego/operator/internal/staticserver/staticserver.go lego/operator/internal/staticserver/resolver.go` is empty. This is not deploy lag.

## Prior milestones: full DoD disposition

| Prior DoD | This sweep / remaining work |
| --- | --- |
| w1/m21 #1: build/publish a repository without a serving Deployment | No-build sample published and served Live; no per-site Deployment. Its ExternalName Service alias is routing infrastructure. Vite/CRA builds were not exercised. |
| w1/m21 #2: redirects, rewrites, response headers | Basic rules and exact/catch-all/subtree headers pass; glob grammar is the residual failure. |
| w1/m21 #3: API shapes and dashboard controls | Dashboard create/edit and three read surfaces verified. REST/MCP writes, create-time/Blueprint headers and publish-directory edits remain unprobed. |
| w1/m21 #4: parity ledger closed with evidence | ADR018 currently says complete; t003 must reconcile the wildcard gap with measured behavior. |
| w4/m94 #1: file before catch-all, remove/restore | GET/HEAD preserve today's 795-byte YAML through toggles; old fixture was 810 bytes. |
| w4/m94 #2: file before redirect, missing path redirects | Pass: YAML 200/no Location; `/qa-old` 301 `/index.html`. |
| w4/m94 #3: request-path headers with/without rewrite | Pass: `/qa-route` carries path + catch-all markers, not index marker. Direct index gets its own marker. Remove/restore preserves this. |
| w4/m94 #4: persistence, grouping, deletion | Rules persist and deletion completes. Grouping not exercised; fixture deliberately ungrouped. |
| w4/m94 #5: shared-handler and adjacent classes | Source traced. Origin/security/custom-domain/other-tenant/built-asset probes remain unverified and are carried into t002/t005. |
| w4/m101 #1–4: autoscaling types/writers/UI/worker decision | Separate, untouched subsystem; not re-probed or claimed regressed. |
| w4/m101 #5: catch-all header on 404 | Pass. Glob markers fail selection while catch-all/prefix markers reach those same 404s. |
| w4/m101 #6: resolved errors and headerless unknown host/405 | 404 observed; other classes source-traced only. Preserve policy in t002/t005. |
| w4/m101 #7: error body metadata protection | Unmodified, not live-probed; preserve in tests. |
| w4/m101 #8–12: switches/copy/env-row/SSH/spelling | Unrelated UI checks; not re-probed or claimed regressed. |

## Limits, evidence and cleanup

**Unverified live:** authenticated Render execution; literal CSS files (YAML exercised the extension selector); security headers; custom domains/other tenants; origin failures/overload; duplicate header names; non-static mutation refusal; REST/MCP writes; Blueprint/create-time headers; Unicode/encoded paths and invalid-pattern grammar. These are tasks/test obligations, not measured DoD assertions.

Verified local artifacts: `.playwright-mcp/qa-static-header-glob-r11.png` (saved glob beside exact control), `qa-r11-api-captures.json`, `qa-r11-http-{baseline,headers,glob-reload,exact-control,exact-control-settled,glob-v2,routes,rewrite-off,rewrite-restored}.json`, and `qa-r11-inventory-{before-delete,after-delete-first,after-delete-settled}.json`. They are ignored scratch evidence; the complete primary probes below carry the handoff.

Console warning/error capture was empty during rule/header testing. Captured API mutations and reads were HTTP 200 with no GraphQL errors. Post-delete REST 404s were intentional. Navigation-interrupted polling requests were not established as hangs and are not filed.

UI delete succeeded at 10:46:27Z; service/header REST 404, empty filtered list, GraphQL null/not found and dashboard absence followed at 10:46:50–51Z. At 10:47:23Z the exact-identity inventory had **no App, routing alias, Ingress, Certificate, publish/purge Job/Pod, build credentials, TLS Secret or other selected child artifact**. App finalization completed; object-store emptiness is inferred from that fail-closed completion, not an independent S3 listing. The session was revoked and local handle/browser cookies cleared.

## Complete replayable probes

Authenticated API requests used the dashboard session cookie, which is omitted. Substitute a newly created fixture id to replay after this run's cleanup. Request payloads and response bodies below are complete captures.

### Exact-path control for the same rule

POST `https://api.bex.co/graphql`:

```json
{
  "time": "2026-10-02T10:41:41.299Z",
  "request": [
    {
      "operationName": "SetStaticHeaders",
      "variables": {
        "id": "srv-davojns5o9vs73dt7r6g",
        "headers": [
          {
            "path": "/*",
            "name": "X-QA-All",
            "value": "r11-all"
          },
          {
            "path": "/qa-route",
            "name": "X-QA-Path",
            "value": "r11-request"
          },
          {
            "path": "/index.html",
            "name": "X-QA-Index",
            "value": "r11-index"
          },
          {
            "path": "/render.yaml",
            "name": "X-QA-Pattern",
            "value": "r11-root-yaml"
          },
          {
            "path": "/render.yaml",
            "name": "X-QA-Exact",
            "value": "r11-exact-yaml"
          }
        ]
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetStaticHeaders($id: String!, $headers: [StaticHeaderInput]) {\n  setStaticHeaders(id: $id, headers: $headers) {\n    id\n    headers {\n      path\n      name\n      value\n      __typename\n    }\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "setStaticHeaders": {
          "__typename": "Service",
          "headers": [
            {
              "__typename": "StaticHeader",
              "name": "X-QA-All",
              "path": "/*",
              "value": "r11-all"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Path",
              "path": "/qa-route",
              "value": "r11-request"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Index",
              "path": "/index.html",
              "value": "r11-index"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Pattern",
              "path": "/render.yaml",
              "value": "r11-root-yaml"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Exact",
              "path": "/render.yaml",
              "value": "r11-exact-yaml"
            }
          ],
          "id": "srv-davojns5o9vs73dt7r6g"
        }
      }
    }
  ]
}
```

A probe at 10:41:43Z was only two seconds after saving and saw the old snapshot. That is normal propagation, not another bug. Settled control:

At 2026-10-02T10:41:59.208369+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/render.yaml` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=31536000, immutable
content-type: binary/octet-stream
date: Fri, 02 Oct 2026 10:41:59 GMT
x-qa-all: r11-all
x-qa-exact: r11-exact-yaml
x-qa-pattern: r11-root-yaml

```

### Wildcard restored, fresh page and settled snapshot

POST `https://api.bex.co/graphql`:

```json
{
  "time": "2026-10-02T10:42:19.169Z",
  "request": [
    {
      "operationName": "SetStaticHeaders",
      "variables": {
        "id": "srv-davojns5o9vs73dt7r6g",
        "headers": [
          {
            "path": "/*",
            "name": "X-QA-All",
            "value": "r11-glob-v2"
          },
          {
            "path": "/qa-route",
            "name": "X-QA-Path",
            "value": "r11-request"
          },
          {
            "path": "/index.html",
            "name": "X-QA-Index",
            "value": "r11-index"
          },
          {
            "path": "/*.yaml",
            "name": "X-QA-Pattern",
            "value": "r11-root-yaml"
          },
          {
            "path": "/render.yaml",
            "name": "X-QA-Exact",
            "value": "r11-exact-yaml"
          },
          {
            "path": "/**/*.yaml",
            "name": "X-QA-Nested",
            "value": "r11-nested-yaml"
          },
          {
            "path": "/**/*",
            "name": "X-QA-Nested-All",
            "value": "r11-nested-all"
          },
          {
            "path": "/nested/*",
            "name": "X-QA-Prefix",
            "value": "r11-prefix"
          }
        ]
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetStaticHeaders($id: String!, $headers: [StaticHeaderInput]) {\n  setStaticHeaders(id: $id, headers: $headers) {\n    id\n    headers {\n      path\n      name\n      value\n      __typename\n    }\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "setStaticHeaders": {
          "__typename": "Service",
          "headers": [
            {
              "__typename": "StaticHeader",
              "name": "X-QA-All",
              "path": "/*",
              "value": "r11-glob-v2"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Path",
              "path": "/qa-route",
              "value": "r11-request"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Index",
              "path": "/index.html",
              "value": "r11-index"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Pattern",
              "path": "/*.yaml",
              "value": "r11-root-yaml"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Exact",
              "path": "/render.yaml",
              "value": "r11-exact-yaml"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Nested",
              "path": "/**/*.yaml",
              "value": "r11-nested-yaml"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Nested-All",
              "path": "/**/*",
              "value": "r11-nested-all"
            },
            {
              "__typename": "StaticHeader",
              "name": "X-QA-Prefix",
              "path": "/nested/*",
              "value": "r11-prefix"
            }
          ],
          "id": "srv-davojns5o9vs73dt7r6g"
        }
      }
    }
  ]
}
```

The new catch-all marker proves the updated snapshot was serving, at least 21 seconds after save:

At 2026-10-02T10:42:41.696142+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/render.yaml` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=31536000, immutable
content-type: binary/octet-stream
date: Fri, 02 Oct 2026 10:42:41 GMT
x-qa-all: r11-glob-v2
x-qa-exact: r11-exact-yaml

```

At 2026-10-02T10:42:43.038273+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/missing-r11.yaml` (exit 0):

```http
HTTP/2 404
content-type: text/plain; charset=utf-8
date: Fri, 02 Oct 2026 10:42:43 GMT
x-content-type-options: nosniff
x-qa-all: r11-glob-v2
content-length: 10

```

At 2026-10-02T10:42:44.222775+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/nested/missing-r11.yaml` (exit 0):

```http
HTTP/2 404
content-type: text/plain; charset=utf-8
date: Fri, 02 Oct 2026 10:42:44 GMT
x-content-type-options: nosniff
x-qa-all: r11-glob-v2
x-qa-prefix: r11-prefix
content-length: 10

```

At 2026-10-02T10:42:45.431493+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/nested/` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=0, must-revalidate
content-type: text/html
date: Fri, 02 Oct 2026 10:42:45 GMT
x-qa-all: r11-glob-v2
x-qa-prefix: r11-prefix

```

Independent first reproduction after an earlier full-page reload:

At 2026-10-02T10:41:20.050066+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/render.yaml` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=31536000, immutable
content-type: binary/octet-stream
date: Fri, 02 Oct 2026 10:41:20 GMT
x-qa-all: r11-all
x-qa-exact: r11-exact-yaml

```

### Read surfaces retain the accepted values

REST and MCP:

```json
{
  "time": "2026-10-02T10:42:39.305Z",
  "request": {
    "label": "rest-headers",
    "url": "https://api.bex.co/v1/services/srv-davojns5o9vs73dt7r6g/headers",
    "method": "GET"
  },
  "response": {
    "status": 200,
    "body": "[{\"path\":\"/*\",\"name\":\"X-QA-All\",\"value\":\"r11-glob-v2\"},{\"path\":\"/qa-route\",\"name\":\"X-QA-Path\",\"value\":\"r11-request\"},{\"path\":\"/index.html\",\"name\":\"X-QA-Index\",\"value\":\"r11-index\"},{\"path\":\"/*.yaml\",\"name\":\"X-QA-Pattern\",\"value\":\"r11-root-yaml\"},{\"path\":\"/render.yaml\",\"name\":\"X-QA-Exact\",\"value\":\"r11-exact-yaml\"},{\"path\":\"/**/*.yaml\",\"name\":\"X-QA-Nested\",\"value\":\"r11-nested-yaml\"},{\"path\":\"/**/*\",\"name\":\"X-QA-Nested-All\",\"value\":\"r11-nested-all\"},{\"path\":\"/nested/*\",\"name\":\"X-QA-Prefix\",\"value\":\"r11-prefix\"}]\n"
  }
}
```

```json
{
  "time": "2026-10-02T10:42:39.531Z",
  "request": {
    "label": "mcp-headers",
    "url": "https://api.bex.co/mcp",
    "method": "POST",
    "body": {
      "jsonrpc": "2.0",
      "id": 111,
      "method": "tools/call",
      "params": {
        "name": "list_static_headers",
        "arguments": {
          "serviceId": "srv-davojns5o9vs73dt7r6g"
        }
      }
    }
  },
  "response": {
    "status": 200,
    "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":111,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"headers\\\":[{\\\"name\\\":\\\"X-QA-All\\\",\\\"path\\\":\\\"/*\\\",\\\"value\\\":\\\"r11-glob-v2\\\"},{\\\"name\\\":\\\"X-QA-Path\\\",\\\"path\\\":\\\"/qa-route\\\",\\\"value\\\":\\\"r11-request\\\"},{\\\"name\\\":\\\"X-QA-Index\\\",\\\"path\\\":\\\"/index.html\\\",\\\"value\\\":\\\"r11-index\\\"},{\\\"name\\\":\\\"X-QA-Pattern\\\",\\\"path\\\":\\\"/*.yaml\\\",\\\"value\\\":\\\"r11-root-yaml\\\"},{\\\"name\\\":\\\"X-QA-Exact\\\",\\\"path\\\":\\\"/render.yaml\\\",\\\"value\\\":\\\"r11-exact-yaml\\\"},{\\\"name\\\":\\\"X-QA-Nested\\\",\\\"path\\\":\\\"/**/*.yaml\\\",\\\"value\\\":\\\"r11-nested-yaml\\\"},{\\\"name\\\":\\\"X-QA-Nested-All\\\",\\\"path\\\":\\\"/**/*\\\",\\\"value\\\":\\\"r11-nested-all\\\"},{\\\"name\\\":\\\"X-QA-Prefix\\\",\\\"path\\\":\\\"/nested/*\\\",\\\"value\\\":\\\"r11-prefix\\\"}]}\"}],\"structuredContent\":{\"headers\":[{\"name\":\"X-QA-All\",\"path\":\"/*\",\"value\":\"r11-glob-v2\"},{\"name\":\"X-QA-Path\",\"path\":\"/qa-route\",\"value\":\"r11-request\"},{\"name\":\"X-QA-Index\",\"path\":\"/index.html\",\"value\":\"r11-index\"},{\"name\":\"X-QA-Pattern\",\"path\":\"/*.yaml\",\"value\":\"r11-root-yaml\"},{\"name\":\"X-QA-Exact\",\"path\":\"/render.yaml\",\"value\":\"r11-exact-yaml\"},{\"name\":\"X-QA-Nested\",\"path\":\"/**/*.yaml\",\"value\":\"r11-nested-yaml\"},{\"name\":\"X-QA-Nested-All\",\"path\":\"/**/*\",\"value\":\"r11-nested-all\"},{\"name\":\"X-QA-Prefix\",\"path\":\"/nested/*\",\"value\":\"r11-prefix\"}]}}}\n\n"
  }
}
```

A later fresh-page GraphQL read includes the redirect/rewrite controls and unchanged revision:

```json
{
  "time": "2026-10-02T10:43:55.882Z",
  "request": {
    "query": "query QaStaticR11($id:String!){service(id:$id){id name type phase revision routes{type source destination} headers{path name value}}}",
    "variables": {
      "id": "srv-davojns5o9vs73dt7r6g"
    }
  },
  "response": {
    "status": 200,
    "body": {
      "data": {
        "service": {
          "headers": [
            {
              "name": "X-QA-All",
              "path": "/*",
              "value": "r11-glob-v2"
            },
            {
              "name": "X-QA-Path",
              "path": "/qa-route",
              "value": "r11-request"
            },
            {
              "name": "X-QA-Index",
              "path": "/index.html",
              "value": "r11-index"
            },
            {
              "name": "X-QA-Pattern",
              "path": "/*.yaml",
              "value": "r11-root-yaml"
            },
            {
              "name": "X-QA-Exact",
              "path": "/render.yaml",
              "value": "r11-exact-yaml"
            },
            {
              "name": "X-QA-Nested",
              "path": "/**/*.yaml",
              "value": "r11-nested-yaml"
            },
            {
              "name": "X-QA-Nested-All",
              "path": "/**/*",
              "value": "r11-nested-all"
            },
            {
              "name": "X-QA-Prefix",
              "path": "/nested/*",
              "value": "r11-prefix"
            }
          ],
          "id": "srv-davojns5o9vs73dt7r6g",
          "name": "qa-20261002-static-r11",
          "phase": "Running",
          "revision": "rev-1",
          "routes": [
            {
              "destination": "/index.html",
              "source": "/render.yaml",
              "type": "redirect"
            },
            {
              "destination": "/index.html",
              "source": "/qa-old",
              "type": "redirect"
            },
            {
              "destination": "/index.html",
              "source": "/*",
              "type": "rewrite"
            }
          ],
          "type": "static_site"
        }
      }
    }
  },
  "sourceValues": ["/render.yaml", "/qa-old", "/*"],
  "saveDisabled": true
}
```

### Passing route controls

Actual POST `https://api.bex.co/graphql` save:

```json
{
  "time": "2026-10-02T10:43:21.957Z",
  "request": [
    {
      "operationName": "SetStaticRoutes",
      "variables": {
        "id": "srv-davojns5o9vs73dt7r6g",
        "routes": [
          {
            "type": "redirect",
            "source": "/render.yaml",
            "destination": "/index.html"
          },
          {
            "type": "redirect",
            "source": "/qa-old",
            "destination": "/index.html"
          },
          {
            "type": "rewrite",
            "source": "/*",
            "destination": "/index.html"
          }
        ]
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetStaticRoutes($id: String!, $routes: [StaticRouteInput]) {\n  setStaticRoutes(id: $id, routes: $routes) {\n    id\n    routes {\n      type\n      source\n      destination\n      __typename\n    }\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "setStaticRoutes": {
          "__typename": "Service",
          "id": "srv-davojns5o9vs73dt7r6g",
          "routes": [
            {
              "__typename": "StaticRoute",
              "destination": "/index.html",
              "source": "/render.yaml",
              "type": "redirect"
            },
            {
              "__typename": "StaticRoute",
              "destination": "/index.html",
              "source": "/qa-old",
              "type": "redirect"
            },
            {
              "__typename": "StaticRoute",
              "destination": "/index.html",
              "source": "/*",
              "type": "rewrite"
            }
          ]
        }
      }
    }
  ]
}
```

After removing/restoring the catch-all rewrite through the UI, fresh reload, and resolver refresh:

At 2026-10-02T10:45:39.424869+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/render.yaml` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=31536000, immutable
content-type: binary/octet-stream
date: Fri, 02 Oct 2026 10:45:39 GMT
x-qa-all: r11-glob-v2
x-qa-exact: r11-exact-yaml

```

At 2026-10-02T10:45:40.804003+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/qa-old` (exit 0):

```http
HTTP/2 301
content-type: text/html; charset=utf-8
date: Fri, 02 Oct 2026 10:45:40 GMT
location: /index.html
x-qa-all: r11-glob-v2

```

At 2026-10-02T10:45:42.092514+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/qa-route` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=0, must-revalidate
content-type: text/html
date: Fri, 02 Oct 2026 10:45:42 GMT
x-qa-all: r11-glob-v2
x-qa-path: r11-request

```

At 2026-10-02T10:45:43.194917+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i -I https://qa-20261002-static-r11.onbex.co/nested/missing-r11.yaml` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=0, must-revalidate
content-type: text/html
date: Fri, 02 Oct 2026 10:45:43 GMT
x-qa-all: r11-glob-v2
x-qa-prefix: r11-prefix

```

Complete GET of the existing-file control (the file is 795 bytes, SHA-256 `8c76de576c5a2331f5dc4aa2ab3df80d4b90386868bd05ce44375b6b635598dd`):

At 2026-10-02T10:45:38.794455+00:00, `curl --connect-timeout 10 --max-time 20 -sS -i https://qa-20261002-static-r11.onbex.co/render.yaml` (exit 0):

```http
HTTP/2 200
cache-control: public, max-age=31536000, immutable
content-type: binary/octet-stream
date: Fri, 02 Oct 2026 10:45:38 GMT
x-qa-all: r11-glob-v2
x-qa-exact: r11-exact-yaml
content-length: 795

# render.yaml — static_site example: no Dockerfile and no build command needed —
# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage
# (w9/010, Render parity), serving it through the shared static-server at a
# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a
# buildCommand, or a runtime instead opts into the build path: the image's
# staticPublishPath directory then holds the built site.
# A git push redeploys automatically via the push webhook.
services:
  - name: static-site
    type: web
    runtime: static
    repo: https://github.com/bex-co/bex # replace with your fork/checkout
    rootDir: examples/static-site
    branch: main
    staticPublishPath: . # publish this directory as-is; set to "dist" for a real build step
```

### Deletion and API absence

POST `https://api.bex.co/graphql`:

```json
{
  "time": "2026-10-02T10:46:27.237Z",
  "request": [
    {
      "operationName": "DeleteService",
      "variables": {
        "id": "srv-davojns5o9vs73dt7r6g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "deleteService": true
      }
    }
  ],
  "url": "https://dashboard.bex.co/"
}
```

```json
[
  {
    "time": "2026-10-02T10:46:50.321Z",
    "request": {
      "label": "deleted-rest",
      "url": "https://api.bex.co/v1/services/srv-davojns5o9vs73dt7r6g",
      "method": "GET"
    },
    "response": {
      "status": 404,
      "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
    }
  },
  {
    "time": "2026-10-02T10:46:50.543Z",
    "request": {
      "label": "deleted-headers",
      "url": "https://api.bex.co/v1/services/srv-davojns5o9vs73dt7r6g/headers",
      "method": "GET"
    },
    "response": {
      "status": 404,
      "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
    }
  },
  {
    "time": "2026-10-02T10:46:50.959Z",
    "request": {
      "label": "deleted-list",
      "url": "https://api.bex.co/v1/services?ownerId=tea-d98210cbbpdc73dcrkvg&name=qa-20261002-static-r11",
      "method": "GET"
    },
    "response": {
      "status": 200,
      "body": "[]\n"
    }
  },
  {
    "time": "2026-10-02T10:46:51.195Z",
    "request": {
      "label": "deleted-gql",
      "url": "https://api.bex.co/graphql",
      "method": "POST",
      "body": {
        "query": "query QaR11Deleted($id:String!){service(id:$id){id name phase}}",
        "variables": {
          "id": "srv-davojns5o9vs73dt7r6g"
        }
      }
    },
    "response": {
      "status": 200,
      "body": "{\"data\":{\"service\":null},\"errors\":[{\"message\":\"not found\",\"locations\":[{\"line\":1,\"column\":33}],\"path\":[\"service\"]}]}\n"
    }
  }
]
```
