# Static edge rules confuse URL syntax with path data

Why: accepted rules must preserve a visitor's filename and serve the object identified by the configured local destination.

**Severity: major.** Two separately traced serving defects: wildcard redirects turn encoded filename characters into URL syntax, and rewrites treat URL syntax/encoding as literal object-key bytes. Neither is a data-loss or cross-tenant exploit claim. Scope: ~3h 5m including shared-caller verification and closing tasks.

## Fixture, reproduction and controls

Own fixture `qa-20261002-static-r16`, service `srv-davq6jede41s73canvdg`, workspace `tea-d98210cbbpdc73dcrkvg`; App `tea-d98210cbbpdc73dcrkvg-qa-20261002-static-r16`, UID `f7e06c68-bc88-4875-9ce0-d89d4fd8ea08`. Created through the Static Site wizard at 12:27:58Z from `https://github.com/bex-co/bex`, main, root `examples/static-site`, publish directory `.`, blank build command, auto-deploy Off. It reached Running/Live at rev-1 and served the 795-byte render.yaml. The interruption left an unsaved draft only. On resume the old session was revoked, a fresh muse.env session created, and the same owned fixture reused. No other resource was changed.

1. Open `https://dashboard.bex.co/static/srv-davq6jede41s73canvdg/redirects`. Add the nine rows in the first captured SetStaticRoutes mutation below and Save routes (21:38:11Z; HTTP 200).
2. Probe with external curl GET/HEAD, no redirect following. `/qa-base.yaml` returns the published YAML, while `/qa-rewrite.yaml` and `/qa-encoded.yaml` return 404. Direct requests to the configured destinations return the YAML.
3. `/qa-jump/a%3Fb.yaml` returns `Location: /docs/a?b.yaml`. After a fresh detail reload at 21:39:29Z, the same question case reproduces; hash, percent and space cases also produce unescaped ASCII characters in Location. Plain filenames work.
4. Browser navigation at 21:43:17Z from `/qa-jump/a%23b.yaml` lands at `https://qa-20261002-static-r16.onbex.co/docs/a#b.yaml` (HTTP 200, the existing implicit SPA fallback). The hash moved out of the path; the fallback's 200 is deliberate and is not itself a bug. Browser navigation to `/qa-rewrite.yaml` at 21:43:18Z shows HTTP 404 and `not found\n`.
5. Save a positive control at 21:44:03Z: change the query-bearing rewrite destination to plain `/render.yaml`, and the wildcard redirect destination to its `/docs/*` alias. The same previously failing rewrite now serves the exact 795-byte YAML/hash; the second redirect spelling still corrupts question/hash paths. Reloaded settings retain these changes.

The live probes also verify file-before-rule precedence, redirect-before-rewrite ordering, ordinary wildcard rewrite with an incoming query, and literal configured redirect query/fragment. Incoming-query forwarding on redirects is **not** asserted as a defect: this sweep did not establish that policy. Header wildcard grammar remains w4/m150 and is outside this filing.

## Root cause and precise target behavior

### 1. Captured redirect path data is emitted as URL syntax

- `lego/operator/internal/staticserver/staticserver.go:219` takes decoded `r.URL.Path`, normalized by `normalizePath:564–574`. `matchPattern:482–497` returns a decoded wildcard remainder.
- `matchRoutes:461–475` calls `expandDest:500–508`, which inserts that remainder directly into a destination string. Thus literal `?`, `#`, `%` and spaces captured from the request path become raw bytes in the target.
- `safeRedirectTarget:355–361` checks same-origin/backslash/control hazards; it is not a URI encoder. `ServeHTTP:260–267` passes the raw target to `http.Redirect`.
- Actual pinned builder metadata was resolved read-only from Docker Hub: `golang:1.26@sha256:9d2f36f06329b2a141b9db99ffa32765cf695ee57b813ca29e245e8670bcbfff` has `GOLANG_VERSION=1.26.8`. `lego/Dockerfile:11,18–36` builds the module without the root Go 1.27 workspace. Opened the installed **Go 1.26.8** sources: `src/net/url/url.go:660–674` decodes Path in setPath; `src/net/http/server.go:2367–2427` parses/cleans the target then sets Location through `hexEscapeNonASCII`; `src/net/http/http.go:139–153` leaves ASCII bytes unchanged. It does not escape reserved path characters on the application's behalf. This predicts the captured raw spaces and malformed percent escape as well as the question/hash failures.

**Target:** distinguish configured URL components from captured literal path data before expansion. Serialize the final local redirect URL with correct component escaping. Both supported aliases must preserve the captured filename, including nested separators, without turning its punctuation into query/fragment. Preserve the configured literal query/fragment control. Do not globally replace the decoded request path with EscapedPath: object lookup and header matching consume it too. Keep the final local-target guard and test its behavior after serialization; no external redirects/fetches are introduced.

### 2. Rewrite lookup uses the whole unparsed destination string

- The separate rewrite branch at `staticserver.go:278–290` calls `normalizePath(target)`, then `fetch`. That normalizer only uses path.Clean; it neither separates URL components nor percent-decodes.
- `fetch:365–369`, `lookupPath:380–386` and `Site.keyFor:128` therefore select an object named `render.yaml?from=rule` or `%72ender.yaml`, rather than the existing `render.yaml`. The 404 is consistent with the code, and switching the same row to the plain filename restores 200 without republishing.
- Direct requests pass through net/http URL parsing first, so their query is excluded and their encoded path is decoded. That is why the direct-destination controls succeed; it is not a browser cache workaround. The curl captures include response bytes and hashes.

**Target:** parse the configured local destination as URL components, use its decoded path exactly once for local object lookup, and exclude query/fragment from the object key. Expand decoded path captures into that logical path without reparsing the final expanded text as a URL (otherwise a literal captured question/hash would be lost again). Preserve existing S3-only, tenant/revision-scoped lookup, trailing-slash handling, genuine-miss behavior and error mapping. No schema widening is needed: `lego/types/v1alpha1/app_types.go:1105–1127` already has string Source/Destination fields and the two route types. Reject malformed accepted URL syntax coherently if validation needs tightening; test direct CR runtime input as well.

### Producer, stored shape and consumers

The editor `dashboard/src/features/services/components/static-site-section.tsx:178–209,229–313` sends the ordered rows. `lib/static-rule-validation.ts:18–40` requires rooted paths and rejects leading double slash. `hooks/use-static-site.ts:75–95` sends SetStaticRoutes and adopts the refetched rows; `api/static-site.graphql:6–14` selects the unchanged rule fields.

Backend `apps/service.go:4536–4567` accepts these rooted strings; `routesFromViews:4488–4501` only trims them; `SetRoutes:4694–4710` authorizes can_create, checks static type and patches App.spec.routes. The existing string input/output declarations at `apps/graphql.go:55–84,204–220` can represent every captured value. REST, GraphQL and MCP return the same saved rules, so changing read serialization cannot fix the public response. The resolver `lego/operator/internal/staticserver/resolver.go:65–98` copies App.spec.routes into the shared Site and periodically swaps snapshots. A few seconds of propagation are expected; the defect survived a fresh page and the later probes.

## Exhaustive caller and alias census

Non-test production searches over operator/backend/dashboard found:

- **1 matchRoutes caller** (`ServeHTTP:259`), **1 expandDest caller** (`matchRoutes:467`), **1 safeRedirectTarget caller** (`:261`). The runtime fix is global to static sites, not an allowlist for these fixture paths.
- **3 matchPattern callers:** route matching `:463`, success/redirect header matching `:514`, error header matching `:537`. **2 normalizePath callers:** request path `:219` and rewrite target `:279`. Keep header matching on the original request path and avoid changing its grammar (m150).
- **3 SetRoutes callers:** REST `apps/rest.go:1215`, GraphQL `graphql.go:1748`, MCP `mcp.go:1047`. REST GET/PUT registrations are `rest.go:1244–1245`; GraphQL setter is setStaticRoutes; MCP names are list_static_routes/update_static_routes.
- **2 routesFromViews callers** (`service.go:2779` create and `:4709` setter), **2 validateRoutes callers** (`:2665` create and `:4702` setter). **2 gqlRouteInputs callers** (create `graphql.go:1398`, setter `:1748`) and **2 routeArgViews callers** (MCP create `mcp.go:580`, setter `:1047`). Blueprint updates also project routes at `blueprint_plan.go:235`; direct CR input converges on the same runtime consumer.
- **1 RoutesEditor JSX mount** (`routes/services.$serviceId.redirects.tsx:23`), shared by canonical `/static/$serviceId/redirects` and the `/services/$serviceId/redirects` page/canonicalization path. `static.$serviceId.redirects.tsx:15` mounts that shared page. The mutation hook has **2 callers**: edge-rule page (routes/headers) and publish-directory settings; do not change header/publish semantics to fix runtime URL handling.
- Destination aliases: `:splat` and trailing `/*`, both exercised. Runtime hostname aliases from effectiveHosts include platform, explicit host and additional hosts; only the platform host was probed here.
- Resource family: **static** uses this handler; **web, cron, worker, private, Postgres, Key Value** do not. The resolver explicitly filters TypeStaticSite (`resolver.go:76`), and SetRoutes rejects the other service types. Shared API authorization and non-static serving remain outside the change.

## Adjacent states and limits

Preserve authenticated 401/forbidden/not-found handling and can_create authorization; no resource-existence oracle or new read is needed. Known-site invalid local redirects remain 400, genuine object misses 404, origin permission/timeout errors 502, oversized objects 413, capacity sheds 503; unknown-host 404 and unsupported-method 405 retain their existing separation. A malformed URL configuration should be a bounded input refusal, not a crash or cross-origin fetch. Existing content remains rendered during ordinary polling and resolver refresh; acceptance of a setting does not imply instantaneous serving propagation.

Unverified live: malformed escapes, encoded slashes/dots/backslashes, double encoding, captured query/fragment placeholders, Unicode, actual destination files containing reserved characters, custom domains, other tenants, Blueprint/direct CR writes, REST/MCP writes, built assets, deliberate origin failures and cold-cache edge behavior. These belong to t003/t006. Do not describe them as passed.

## Render and dedupe

[Render's primary documentation](https://render.com/docs/redirects-rewrites) describes destination URLs, serving destination content for rewrites, wildcard capture substitution, and existing-file precedence. It supports full external destinations too; bex deliberately restricts these to local paths. No authenticated Render mutation was run, and its exact query/escaping edge cases were not observed live. The expected outcome here follows the local destination and wildcard contract plus the direct-request controls, not an invented Render capture. Keep external destinations, named placeholder expansion, cache purge and implicit-SPA policy out of scope.

Analysis HEAD: `00c5aeba9`. Production static-server image was read as `ghcr.io/bex-co/bex-operator@sha256:7bc6535cde1da49bcdc406781e2b2cfcd9e8a99a8bc8ba212d8418b84ae0951c`, the e97ca42273a9 pin. `git diff e97ca42273a9 HEAD -- lego/operator/internal/staticserver` is empty; the actual consumer is not a pending-main fix. Latest 40 dashboard/lego commits and targeted histories for expandDest/RawQuery found no repair (expandDest originates in 84e221650).

Whole-board open/done searches for static URL encoding, wildcard escaping, query-bearing rewrites and redirect fragments found no matching issue. **42 open/blocked milestone READMEs** were enumerated across workstreams. DO_NOT_DO was reread: cache purge and unrelated hosting exclusions do not apply. Existing m150 is a pattern-matching gap for response headers and is not extended with this different URL-serialization mechanism.

This is an **uncovered runtime correctness gap from w1/done/m21**, not a claimed recurrence of a previously tested encoded-destination fix. Its entire DoD disposition:

| Prior criterion | This sweep's disposition |
| --- | --- |
| Build/publish a static repo with no compute workload serving it | No-build example published at rev-1; build-command/Vite pipeline not repeated. Existing shared serving design is retained. |
| Routes and headers affect served responses | Ordinary redirects/rewrites pass; URL-boundary cases fail as recorded. Headers were not retested this sweep; preserve prior m94/m101 controls in t003/t006. |
| REST/GraphQL/MCP create/read/update plus dashboard controls | Dashboard create/update and three-surface route reads observed. Other write adapters are source-traced, assigned explicit follow-up verification. |
| ADR018/ADR029 parity entries backed by evidence | Existing entries describe paths/captures but omit this boundary; t004 documents the corrected behavior and any deliberate restrictions. |

Related w4/m94's file-precedence redirect control passes here. Its catch-all rewrite toggle/request-path-header controls passed in sweep 11, not rerun as a new claim here; project/environment placement and failure-class matrix were not exercised. w4/m101's error headers and other autoscaling/accessibility fixes are not claimed regressed. Their shared-handler tests remain required controls. No duplicate filing for those milestones or w4/m150.

## Durable API evidence

Each capture below includes the exact credential-free request and complete response. Replay through an authenticated browser; replace the deleted fixture ID. HTTP 200 configuration reads do not establish serving correctness.

### Dashboard mutations and selected query

```json
[
  {
    "at": "2026-10-02T21:38:12.180Z",
    "request": [
      {
        "operationName": "SetStaticRoutes",
        "variables": {
          "id": "srv-davq6jede41s73canvdg",
          "routes": [
            {
              "type": "redirect",
              "source": "/qa-redirect",
              "destination": "/render.yaml?from=rule#frag"
            },
            {
              "type": "rewrite",
              "source": "/qa-rewrite.yaml",
              "destination": "/render.yaml?from=rule"
            },
            {
              "type": "rewrite",
              "source": "/qa-copy/*",
              "destination": "/:splat"
            },
            {
              "type": "redirect",
              "source": "/qa-jump/*",
              "destination": "/docs/:splat"
            },
            {
              "type": "redirect",
              "source": "/qa-order",
              "destination": "/render.yaml"
            },
            {
              "type": "rewrite",
              "source": "/qa-order",
              "destination": "/index.html"
            },
            {
              "type": "rewrite",
              "source": "/qa-encoded.yaml",
              "destination": "/%72ender.yaml"
            },
            {
              "type": "rewrite",
              "source": "/qa-base.yaml",
              "destination": "/render.yaml"
            },
            {
              "type": "redirect",
              "source": "/render.yaml",
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
            "id": "srv-davq6jede41s73canvdg",
            "routes": [
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml?from=rule#frag",
                "source": "/qa-redirect",
                "type": "redirect"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml?from=rule",
                "source": "/qa-rewrite.yaml",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/:splat",
                "source": "/qa-copy/*",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/docs/:splat",
                "source": "/qa-jump/*",
                "type": "redirect"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml",
                "source": "/qa-order",
                "type": "redirect"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/index.html",
                "source": "/qa-order",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/%72ender.yaml",
                "source": "/qa-encoded.yaml",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml",
                "source": "/qa-base.yaml",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/index.html",
                "source": "/render.yaml",
                "type": "redirect"
              }
            ]
          }
        }
      }
    ]
  },
  {
    "at": "2026-10-02T21:40:55.849Z",
    "request": {
      "query": "query QAStatic($id:String!){service(id:$id){id name type phase routes{type source destination}}}",
      "variables": {
        "id": "srv-davq6jede41s73canvdg"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "service": {
          "id": "srv-davq6jede41s73canvdg",
          "name": "qa-20261002-static-r16",
          "phase": "Running",
          "routes": [
            {
              "destination": "/render.yaml?from=rule#frag",
              "source": "/qa-redirect",
              "type": "redirect"
            },
            {
              "destination": "/render.yaml?from=rule",
              "source": "/qa-rewrite.yaml",
              "type": "rewrite"
            },
            {
              "destination": "/:splat",
              "source": "/qa-copy/*",
              "type": "rewrite"
            },
            {
              "destination": "/docs/:splat",
              "source": "/qa-jump/*",
              "type": "redirect"
            },
            {
              "destination": "/render.yaml",
              "source": "/qa-order",
              "type": "redirect"
            },
            {
              "destination": "/index.html",
              "source": "/qa-order",
              "type": "rewrite"
            },
            {
              "destination": "/%72ender.yaml",
              "source": "/qa-encoded.yaml",
              "type": "rewrite"
            },
            {
              "destination": "/render.yaml",
              "source": "/qa-base.yaml",
              "type": "rewrite"
            },
            {
              "destination": "/index.html",
              "source": "/render.yaml",
              "type": "redirect"
            }
          ],
          "type": "static_site"
        }
      }
    }
  },
  {
    "at": "2026-10-02T21:44:04.042Z",
    "request": [
      {
        "operationName": "SetStaticRoutes",
        "variables": {
          "id": "srv-davq6jede41s73canvdg",
          "routes": [
            {
              "type": "redirect",
              "source": "/qa-redirect",
              "destination": "/render.yaml?from=rule#frag"
            },
            {
              "type": "rewrite",
              "source": "/qa-rewrite.yaml",
              "destination": "/render.yaml"
            },
            {
              "type": "rewrite",
              "source": "/qa-copy/*",
              "destination": "/:splat"
            },
            {
              "type": "redirect",
              "source": "/qa-jump/*",
              "destination": "/docs/*"
            },
            {
              "type": "redirect",
              "source": "/qa-order",
              "destination": "/render.yaml"
            },
            {
              "type": "rewrite",
              "source": "/qa-order",
              "destination": "/index.html"
            },
            {
              "type": "rewrite",
              "source": "/qa-encoded.yaml",
              "destination": "/%72ender.yaml"
            },
            {
              "type": "rewrite",
              "source": "/qa-base.yaml",
              "destination": "/render.yaml"
            },
            {
              "type": "redirect",
              "source": "/render.yaml",
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
            "id": "srv-davq6jede41s73canvdg",
            "routes": [
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml?from=rule#frag",
                "source": "/qa-redirect",
                "type": "redirect"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml",
                "source": "/qa-rewrite.yaml",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/:splat",
                "source": "/qa-copy/*",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/docs/*",
                "source": "/qa-jump/*",
                "type": "redirect"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml",
                "source": "/qa-order",
                "type": "redirect"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/index.html",
                "source": "/qa-order",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/%72ender.yaml",
                "source": "/qa-encoded.yaml",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/render.yaml",
                "source": "/qa-base.yaml",
                "type": "rewrite"
              },
              {
                "__typename": "StaticRoute",
                "destination": "/index.html",
                "source": "/render.yaml",
                "type": "redirect"
              }
            ]
          }
        }
      }
    ]
  },
  {
    "at": "2026-10-02T21:46:05.363Z",
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-davq6jede41s73canvdg"
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
    ]
  }
]
```

### REST / GraphQL / MCP readback

```json
[
  {
    "at": "2026-10-02T21:40:54.907Z",
    "request": {
      "surface": "REST",
      "url": "https://api.bex.co/v1/services/srv-davq6jede41s73canvdg/routes",
      "method": "GET"
    },
    "status": 200,
    "response": "[{\"type\":\"redirect\",\"source\":\"/qa-redirect\",\"destination\":\"/render.yaml?from=rule#frag\"},{\"type\":\"rewrite\",\"source\":\"/qa-rewrite.yaml\",\"destination\":\"/render.yaml?from=rule\"},{\"type\":\"rewrite\",\"source\":\"/qa-copy/*\",\"destination\":\"/:splat\"},{\"type\":\"redirect\",\"source\":\"/qa-jump/*\",\"destination\":\"/docs/:splat\"},{\"type\":\"redirect\",\"source\":\"/qa-order\",\"destination\":\"/render.yaml\"},{\"type\":\"rewrite\",\"source\":\"/qa-order\",\"destination\":\"/index.html\"},{\"type\":\"rewrite\",\"source\":\"/qa-encoded.yaml\",\"destination\":\"/%72ender.yaml\"},{\"type\":\"rewrite\",\"source\":\"/qa-base.yaml\",\"destination\":\"/render.yaml\"},{\"type\":\"redirect\",\"source\":\"/render.yaml\",\"destination\":\"/index.html\"}]\n"
  },
  {
    "at": "2026-10-02T21:40:55.819Z",
    "request": {
      "surface": "GraphQL",
      "url": "https://api.bex.co/graphql",
      "method": "POST",
      "body": {
        "query": "query QAStatic($id:String!){service(id:$id){id name type phase routes{type source destination}}}",
        "variables": {
          "id": "srv-davq6jede41s73canvdg"
        }
      }
    },
    "status": 200,
    "response": "{\"data\":{\"service\":{\"id\":\"srv-davq6jede41s73canvdg\",\"name\":\"qa-20261002-static-r16\",\"phase\":\"Running\",\"routes\":[{\"destination\":\"/render.yaml?from=rule#frag\",\"source\":\"/qa-redirect\",\"type\":\"redirect\"},{\"destination\":\"/render.yaml?from=rule\",\"source\":\"/qa-rewrite.yaml\",\"type\":\"rewrite\"},{\"destination\":\"/:splat\",\"source\":\"/qa-copy/*\",\"type\":\"rewrite\"},{\"destination\":\"/docs/:splat\",\"source\":\"/qa-jump/*\",\"type\":\"redirect\"},{\"destination\":\"/render.yaml\",\"source\":\"/qa-order\",\"type\":\"redirect\"},{\"destination\":\"/index.html\",\"source\":\"/qa-order\",\"type\":\"rewrite\"},{\"destination\":\"/%72ender.yaml\",\"source\":\"/qa-encoded.yaml\",\"type\":\"rewrite\"},{\"destination\":\"/render.yaml\",\"source\":\"/qa-base.yaml\",\"type\":\"rewrite\"},{\"destination\":\"/index.html\",\"source\":\"/render.yaml\",\"type\":\"redirect\"}],\"type\":\"static_site\"}}}\n"
  },
  {
    "at": "2026-10-02T21:40:56.431Z",
    "request": {
      "surface": "MCP",
      "url": "https://api.bex.co/mcp",
      "method": "POST",
      "body": {
        "jsonrpc": "2.0",
        "id": 161,
        "method": "tools/call",
        "params": {
          "name": "list_static_routes",
          "arguments": {
            "serviceId": "srv-davq6jede41s73canvdg"
          }
        }
      }
    },
    "status": 200,
    "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":161,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"routes\\\":[{\\\"destination\\\":\\\"/render.yaml?from=rule#frag\\\",\\\"source\\\":\\\"/qa-redirect\\\",\\\"type\\\":\\\"redirect\\\"},{\\\"destination\\\":\\\"/render.yaml?from=rule\\\",\\\"source\\\":\\\"/qa-rewrite.yaml\\\",\\\"type\\\":\\\"rewrite\\\"},{\\\"destination\\\":\\\"/:splat\\\",\\\"source\\\":\\\"/qa-copy/*\\\",\\\"type\\\":\\\"rewrite\\\"},{\\\"destination\\\":\\\"/docs/:splat\\\",\\\"source\\\":\\\"/qa-jump/*\\\",\\\"type\\\":\\\"redirect\\\"},{\\\"destination\\\":\\\"/render.yaml\\\",\\\"source\\\":\\\"/qa-order\\\",\\\"type\\\":\\\"redirect\\\"},{\\\"destination\\\":\\\"/index.html\\\",\\\"source\\\":\\\"/qa-order\\\",\\\"type\\\":\\\"rewrite\\\"},{\\\"destination\\\":\\\"/%72ender.yaml\\\",\\\"source\\\":\\\"/qa-encoded.yaml\\\",\\\"type\\\":\\\"rewrite\\\"},{\\\"destination\\\":\\\"/render.yaml\\\",\\\"source\\\":\\\"/qa-base.yaml\\\",\\\"type\\\":\\\"rewrite\\\"},{\\\"destination\\\":\\\"/index.html\\\",\\\"source\\\":\\\"/render.yaml\\\",\\\"type\\\":\\\"redirect\\\"}]}\"}],\"structuredContent\":{\"routes\":[{\"destination\":\"/render.yaml?from=rule#frag\",\"source\":\"/qa-redirect\",\"type\":\"redirect\"},{\"destination\":\"/render.yaml?from=rule\",\"source\":\"/qa-rewrite.yaml\",\"type\":\"rewrite\"},{\"destination\":\"/:splat\",\"source\":\"/qa-copy/*\",\"type\":\"rewrite\"},{\"destination\":\"/docs/:splat\",\"source\":\"/qa-jump/*\",\"type\":\"redirect\"},{\"destination\":\"/render.yaml\",\"source\":\"/qa-order\",\"type\":\"redirect\"},{\"destination\":\"/index.html\",\"source\":\"/qa-order\",\"type\":\"rewrite\"},{\"destination\":\"/%72ender.yaml\",\"source\":\"/qa-encoded.yaml\",\"type\":\"rewrite\"},{\"destination\":\"/render.yaml\",\"source\":\"/qa-base.yaml\",\"type\":\"rewrite\"},{\"destination\":\"/index.html\",\"source\":\"/render.yaml\",\"type\":\"redirect\"}]}}}\n\n"
  }
]
```

## Complete external HTTP probes

Commands were curl with bounded connect/total timeouts, no -L. Every record includes the full response headers/body and stderr. HEAD controls are retained in the fresh-page set. The baseline YAML GET hash was `8c76de576c5a2331f5dc4aa2ab3df80d4b90386868bd05ce44375b6b635598dd`.

### Initial saved rules

```json
[
  {
    "time": "2026-10-02T21:38:29.022020+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 404 \ncontent-type: text/plain; charset=utf-8\ndate: Fri, 02 Oct 2026 21:38:28 GMT\nx-content-type-options: nosniff\ncontent-length: 10",
    "body": "not found\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:38:30.172519+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-encoded.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-encoded.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 404 \ncontent-type: text/plain; charset=utf-8\ndate: Fri, 02 Oct 2026 21:38:30 GMT\nx-content-type-options: nosniff\ncontent-length: 10",
    "body": "not found\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:38:31.292205+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-redirect?incoming=one",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-redirect?incoming=one"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:38:31 GMT\nlocation: /render.yaml?from=rule#frag\ncontent-length: 62",
    "body": "<a href=\"/render.yaml?from=rule#frag\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:38:32.538740+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-copy/render.yaml?incoming=one",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-copy/render.yaml?incoming=one"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:38:32 GMT\ncontent-length: 795",
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:38:35.149541+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml?incoming=one",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml?incoming=one"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:38:35 GMT\nlocation: /docs/a?b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a?b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:38:36.511131+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/render.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/render.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:38:36 GMT\ncontent-length: 795",
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n",
    "stderr": ""
  }
]
```

### Fresh detail-page reload

```json
[
  {
    "time": "2026-10-02T21:39:30.701553+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-base.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-base.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:39:30 GMT\ncontent-length: 795",
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:31.377315+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-base.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-base.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:39:31 GMT",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:32.036094+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 404 \ncontent-type: text/plain; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:31 GMT\nx-content-type-options: nosniff\ncontent-length: 10",
    "body": "not found\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:32.610304+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 404 \ncontent-type: text/plain; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:32 GMT\nx-content-type-options: nosniff\ncontent-length: 10",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:33.222661+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-encoded.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-encoded.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 404 \ncontent-type: text/plain; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:33 GMT\nx-content-type-options: nosniff\ncontent-length: 10",
    "body": "not found\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:33.869349+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-encoded.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-encoded.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 404 \ncontent-type: text/plain; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:33 GMT\nx-content-type-options: nosniff\ncontent-length: 10",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:34.519005+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:34 GMT\nlocation: /docs/a?b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a?b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:35.151644+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:35 GMT\nlocation: /docs/a?b.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:36.000084+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:35 GMT\nlocation: /docs/a#b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a#b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:36.661244+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:36 GMT\nlocation: /docs/a#b.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:37.238864+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%25b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%25b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:37 GMT\nlocation: /docs/a%b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a%b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:37.840906+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%25b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%25b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:37 GMT\nlocation: /docs/a%b.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:38.445008+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/plain.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/plain.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:38 GMT\nlocation: /docs/plain.yaml\ncontent-length: 51",
    "body": "<a href=\"/docs/plain.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:39.077429+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/plain.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/plain.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:39 GMT\nlocation: /docs/plain.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:39.671657+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%20b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%20b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:39 GMT\nlocation: /docs/a b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:40.289632+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%20b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%20b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:39:40 GMT\nlocation: /docs/a b.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:40.863000+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/%72ender.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/%72ender.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:39:40 GMT\ncontent-length: 795",
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:41.433348+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/%72ender.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/%72ender.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:39:41 GMT",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:42.014134+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/render.yaml?from=rule",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/render.yaml?from=rule"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:39:41 GMT\ncontent-length: 795",
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:39:42.599421+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/render.yaml?from=rule",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/render.yaml?from=rule"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:39:42 GMT",
    "body": "",
    "stderr": ""
  }
]
```

### Same-row rewrite control and trailing-wildcard alias

```json
[
  {
    "time": "2026-10-02T21:44:39.035322+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:44:38 GMT\ncontent-length: 795",
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:39.723196+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-rewrite.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 200 \ncache-control: public, max-age=31536000, immutable\ncontent-type: binary/octet-stream\ndate: Fri, 02 Oct 2026 21:44:39 GMT",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:40.314866+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:44:40 GMT\nlocation: /docs/a?b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a?b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:40.953392+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%3Fb.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:44:40 GMT\nlocation: /docs/a?b.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:41.550700+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:44:41 GMT\nlocation: /docs/a#b.yaml\ncontent-length: 49",
    "body": "<a href=\"/docs/a#b.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:42.202588+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-jump/a%23b.yaml"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:44:42 GMT\nlocation: /docs/a#b.yaml",
    "body": "",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:42.822451+00:00",
    "method": "GET",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-order",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "https://qa-20261002-static-r16.onbex.co/qa-order"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:44:42 GMT\nlocation: /render.yaml\ncontent-length: 47",
    "body": "<a href=\"/render.yaml\">Moved Permanently</a>.\n\n",
    "stderr": ""
  },
  {
    "time": "2026-10-02T21:44:43.512632+00:00",
    "method": "HEAD",
    "url": "https://qa-20261002-static-r16.onbex.co/qa-order",
    "command": [
      "curl",
      "--connect-timeout",
      "10",
      "--max-time",
      "20",
      "-sS",
      "-i",
      "-I",
      "https://qa-20261002-static-r16.onbex.co/qa-order"
    ],
    "exit": 0,
    "responseHeaders": "HTTP/2 301 \ncontent-type: text/html; charset=utf-8\ndate: Fri, 02 Oct 2026 21:44:43 GMT\nlocation: /render.yaml",
    "body": "",
    "stderr": ""
  }
]
```

## Cleanup and artifact checks

Dashboard deletion at 21:46:04Z used `sudo delete static site qa-20261002-static-r16`. The overview no longer listed it. Complete API read after deletion:

```json
{
  "at": "2026-10-02T21:46:31.245Z",
  "request": {
    "method": "GET",
    "url": "https://api.bex.co/v1/services/srv-davq6jede41s73canvdg"
  },
  "status": 404,
  "response": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
}
```

Public HEAD returned 404 at 21:46:33Z. Final exact inventory:

```json
{
  "time": "2026-10-02T21:47:16.023767+00:00",
  "items": [],
  "secretsMetadataOnly": []
}
```

The original interrupted session and the resumed session were revoked through their own logout handle. Credential files were removed and browser cookies cleared. The fixture required no manual cluster mutation. Local artifact existence was checked before filing: `.playwright-mcp/qa-static-r16-rewrite-404.png` (inspected browser 404); `qa-r16-api-captures.json`, `qa-r16-surface-reads.json`, `qa-r16-http-{rules,fresh,exact-control}.json`, and `qa-r16-inventory-{before-delete,after-delete-settled}.json`. These are supplementary ignored files; complete serving/API probes are inline.

Dashboard console warnings/errors after the fresh rules reload were zero. Network reads/writes returned 200; the pre-login 401 and navigation-aborted request preceded the authenticated probes. Browser public 404 diagnostics are the reproduced response, not an additional finding. No paid resource, external domain, fleet setting or product fix was changed.
