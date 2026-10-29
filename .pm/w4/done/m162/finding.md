# Tenant /healthz is intercepted by platform readiness

Two major findings from sweep 52, reproduced on owned fixtures on 2026-10-03. Reviewed against main `5fcaf3b49` after pulling the latest image-pin commit; the two implicated handlers did not change in that pull. The platform pin selects `8f6a69e9eaf5`. No fix for these handlers is present on main.

## Fixtures and observed sequence

| Fixture | ID | Configuration |
| --- | --- | --- |
| qa-20261003-healthpath-r52 | srv-db0afi2tm2ss7389qohg | Free native Go; public bex-co/bex main, examples/hello-go, build go build -o app ., start ./app, port 3000, auto-deploy off, Idle timeout 300s |
| qa-20261003-healthstatic-r52 | srv-db0ah8viujss73au0tgg | Free static, public bex-co/bex main, examples/static-site, no build inputs, publish ., auto-deploy off |

Both were created through the dashboard; mutation records below contain the exact inputs. Tenant namespace: `tea-d98210cbbpdc73dcrkvg`. The web App UID was `638e71c7-0322-49b1-94ce-7750cc2f3b88`. All lifecycle/rule mutations affected only these fixtures.

## 1. Activator returns a false successful health response before suspension or wake handling

**Severity: major.** A status-only monitor sees 200 while the application is suspended or asleep. Health-path requests cannot initiate wake. This is neither an authentication bypass nor disclosure of tenant content.

**Repro:** create the web fixture, verify its own `OK` at root and health paths, click Settings → Suspend, wait for Suspended, then GET/HEAD `/healthz`. Root and trailing-slash controls return the documented suspended 503, but exact, query and percent-encoded health paths return empty 200 without Retry-After or no-store. A fresh Settings reload and external curl reproduce it. Resume restores `/healthz → 200 OK`.

After natural auto-sleep, health-path-only requests at 07:10:30, 07:11:01 and 07:12:27 UTC all returned empty 200. API reads and the dashboard still reported Sleeping/Hibernated. Read-only Kubernetes observations showed Deployment replicas/readyReplicas 0 and unchanged `app.bex.co/last-active=2026-10-03T07:03:09Z`. At 07:12:43, the first request to `/healthz/` returned 503 with retryAfter 5; at 07:13:05 the original `/healthz` returned the app's `OK`, phase was Running, and last-active had advanced to 07:12:42Z. Thus the workload and ordinary wake path are functional.

**Expected:** every tenant path reaches the existing maintenance/suspension/wake decision. Explicit suspension never wakes; automatic sleep does. 200 is appropriate only once the actual application serves it.

**Root cause:** `lego/operator/cmd/activator/main.go:134–141` unconditionally answers 200 when decoded `r.URL.Path == "/healthz"`, before host resolution at 144–153, maintenance at 156–159, suspension at 169–172, or wake at 178–183. `wakeApp:190–216` is never called. The readiness probe uses the same public listener/path at `config/activator/deployment.yaml:38–43`. `main.go:117` has one production registration of this handler; there is no separate health listener.

**Static suspension is the same activator cause, not a static-server claim:** `internal/controller/app_controller.go:2133–2136` includes web and static in `suspendedRoutable`; the static suspended branch at 3500–3508 chooses the activator alias. The static fixture's Ingress was observed naming that alias on port 8888 while suspended, and its root correctly returned the same 503. Its `/healthz` was still empty 200 after a fresh page load.

**Fix:** remove the tenant-port special case and move the platform probe off HTTP paths. Prefer a TCP readiness probe on the existing activator port 8888: current healthz is only an unconditional process/listener check, and initial cache setup remains before ListenAndServe. Update the canonical Deployment in the same change and validate rendered manifests/local readiness. Do not reserve a different tenant path or use an untrusted Host/User-Agent/header as a health-probe bypass. If readiness must remain application-level HTTP, use a separate non-Service management listener and update its exposure/probes explicitly; do not leave either platform deployment unready.

## 2. Running static sites bypass content and saved rules at /healthz

**Severity: major.** The dashboard accepts a valid rewrite which is silently skipped on this path. Even the default extensionless fallback is intercepted.

**Repro:** create the no-build static fixture. Root and `/healthz/` return the site's index HTML; `/healthz`, its query variant and `/health%7a` return empty 200. Save two identical-target rewrites, `/healthz → /render.yaml` and `/qa-health-control → /render.yaml`. GraphQL and a fresh Redirects/Rewrites page retain both exact rows. Direct `/render.yaml` and the control return the complete same file (Content-Length 795); `/healthz` remains empty. Repeat after a fresh page load. This is not the URL-component bug m154 or cache-policy bug m160: the handler never enters the site-serving path.

**Root cause:** `lego/operator/cmd/staticserver/main.go:131–132` registers an unconditional host/method-agnostic `/healthz` handler on the same ServeMux used for the site handler at 166. The probe at `config/staticserver/deployment.yaml:94–99` depends on it. Go's mux chooses the literal health path before the root subtree. Consequently `internal/staticserver/staticserver.go:203–217` (method check and site resolution) and its object/rules/header pipeline are never reached. One production `staticserver.New` registration occurs in this executable.

**Fix:** remove that public HTTP override and use a TCP readiness probe on existing port 8080, preserving existing startup origin credential checks (main.go:152–159), server timeouts/shutdown and degraded unconfigured-origin behavior. On the tenant port, even `/healthz` must flow through the real static handler: existing objects win, otherwise ordered rules and documented SPA fallback apply; errors and custom headers keep their current semantics. Do not change the correct core matcher to compensate for a handler that bypasses it.

## Framework and consumer verification

The deployed binary's exact Go patch version was not queried. The repository platform Go line is 1.26; local installed standard-library source inspected is Go 1.26.5 (`go env GOROOT GOVERSION`). These public-response observations do not depend on asserting an exact deployed patch version:

- `net/http/request.go:1123` parses the request target through `url.ParseRequestURI`; `net/url/url.go:659–671` stores a decoded Path and optional RawPath. That explains the activator's observed `/health%7a` alias and why query does not affect the comparison.
- `net/http/server.go:2659–2685` dispatches via the ServeMux using the escaped request path; `routing_tree.go:154–176` prefers literal segments to subtree wildcard matches; `firstSegment:203–216` unescapes each segment. No method qualifier on the registered pattern means GET/HEAD both hit the health override. The observed trailing-slash controls take the normal handlers.
- Changing the two readiness consumers is required: deleting only their HTTP health handler would make the kubelet request the tenant/default handler without a tenant Host, receive its error, and mark the platform pods unready. TCP readiness avoids tenant-host resolution altogether and keeps service/alias ports unchanged. Validate this in the rendered deployments and an isolated local rollout; it was not changed or tested during this QA-only run.

## Blast radius and adjacent classes

An exhaustive production-code search of `lego/operator/cmd` found four components mentioning healthz: activator, staticserver, manager, and egress-meter. Only the first two are the tenant HTTP frontends in this finding. Do not rewrite the manager's dedicated health server or egress-meter's internal server. Within the affected executables there are exactly two production dispatch registrations in total (one per component), and exactly two canonical readiness consumers.

- Web (including legacy untyped web): automatic sleep and explicit suspend use the activator. Maintenance also passes through this shared handler; the same early return precedes it, but paid maintenance was **not** exercised live.
- Static: Running uses static-server; suspended uses activator in production. Without an activator configured, existing fallback is the static resolver's unknown-site 404. Keep that documented fallback.
- Worker/private/cron have no public App-host serving path here. Postgres and Key Value use their distinct TCP proxies. No change to those five type families.
- Platform and custom domains share host routing; only owned platform domains were tested. Custom domains, explicit `/healthz` object files, custom response headers, POST/other methods, and unconfigured-origin cases require regression tests, not claims of live coverage.
- Unknown hosts keep the normal respective response (activator generic unavailable 503, static-server unknown-site 404); method rejection stays as defined by the static handler. No authenticated resource lookup/error taxonomy changes. REST/GraphQL/MCP permissions and mutation semantics remain unchanged; these surfaces already reported honest state and saved rules.
- Preserve bounded cold-start 503/retry behavior, maintenance-before-suspension precedence, no-wake explicit suspension, hostname/TLS identity, and correct content/rules/cache/error handling. This work neither changes sleep policy nor addresses scanner traffic.

## Dedupe and precedent disposition

Searched all open and done PM records for healthz, health endpoint, activator/wake and suspended-200 terms; scanned 71 non-done README files across workstreams. No open item owns either public-health-path collision. Read DO_NOT_DO: the no-self-wake item is inapplicable (this probe demonstrates **failure to wake** on a specific inbound path); no paid feature was purchased. Recent 40 product commits and targeted `git log -S` checks show no landed fix.

The activator exception already existed in `6e2375774`; static-server's registration dates to its introduction `84e221650`. Treat this as a residual path-coverage gap, not a new regression caused by recent lifecycle or edge-rule fixes.

Precedents reviewed:

| Prior item and complete DoD disposition | This sweep |
| --- | --- |
| w2/done/m98 #1 suspend state | UI/GraphQL and zero-replica observation pass; REST suspend transport status was not re-run. |
| m98 #2 HTML suspension | Healthz query with HTML Accept fails (empty 200); ordinary-root HTML negotiation not re-run. |
| m98 #3 JSON suspension | Root passes; healthz exact/query/encoded fails. |
| m98 #4 plain text | Static ordinary root returns the documented text503; healthz fails; web plain-text root not re-run. |
| m98 #5 certificate unchanged | HTTPS certificate validation succeeds; issuer/serial equality was not measured. |
| m98 #6 no wake while suspended | Web Deployment remains zero and annotation unchanged; full Events audit not repeated. |
| m98 #7 resume | Web healthz returns its original OK body; static content/rules work again after resume. |
| m98 #8 static/sleep controls | Static's old 404 expectation was superseded by w4/done/160; current root503 passes. Sleep healthz fails; ordinary slash wake control passes. |
| m98 #9 delete | UI deletion and by-id REST/public404 pass; REST DELETE204 transport was not used. |
| w6/done/m94 DoD1 app-or-wake response | Healthz produces neither actual app bytes nor wake503; control wakes normally. The transition-race timing trial itself was not repeated. |
| m94 DoD2 resume-race disposition | Existing readiness-gated implementation is retained; this finding is outside that race. |
| m94 DoD3 suspend/maintenance controls | Suspension root control passes but healthz fails; maintenance unverified live. |
| w3/done/m46 DoD1 suspended cert/copy + resume | Updated by w4/160; current root503 and resumed content pass. Exact certificate identity unmeasured. |
| m46 DoD2 deletion list | By-id/public/cluster removal verified; Overview lingering-row timing not measured. |
| m46 DoD3 clear-cache menu/MCP | Not exercised or changed. |
| m46 DoD4 documented SPA fallback | Root and trailing-slash fallback pass; exact healthz is intercepted before that correct behavior. |
| w4/done/160 target suspension + resume + unknown-host | Both fixture roots now emit correct503 and resume restores content; healthz is the residual; unknown-host regression is task work. |
| w7/done/m72/t002 cache + bounded request handling | The note explicitly describes non-healthz routing; no evidence its host-cache fix regressed. Keep cache/concurrency behavior. |

m150 (header pattern grammar), m154 (URL destination semantics), m160 (browser cache lifetime), and w9/m89/m92 hunt precedents do not own this pre-dispatch collision. No existing item was closed based on this partial re-probe.

## Render comparison

[Render free-service docs](https://render.com/docs/free), checked 2026-10-03, describe waking on an incoming HTTP request and explicitly document a robots.txt exception; they do not document a healthz exception. [Render health-check docs](https://render.com/docs/health-checks) distinguish instance probes from public traffic and let an application own its configured path. This is a bex hosting-contract defect; no live Render healthz/suspended-static wire response was tested. Keep the deliberate bex differences in ADR007/ADR018 instead of inventing exact Render parity.

## Durable evidence

Exact request/response records below are from the owned probes. Public bodies and headers are complete. The API records include create, idle configuration, suspend/resume, state/rule reads and deletion. Credentials/cookies are not included. The UI's first Save routes response was missed by an incorrectly named response predicate; the persisted rows are independently evidenced by fresh-page readback, the exact GraphQL response and working control route. No fabricated mutation response is substituted.

```json
{
  "baseline": [
    {
      "at": "2026-10-03T07:03:25.097Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "2",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:03:25 GMT"
      },
      "body": "OK"
    },
    {
      "at": "2026-10-03T07:03:25.269Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "2",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:03:25 GMT"
      },
      "body": "OK"
    },
    {
      "at": "2026-10-03T07:03:25.445Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz?qa=r52",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "2",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:03:25 GMT"
      },
      "body": "OK"
    },
    {
      "at": "2026-10-03T07:03:25.617Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz/",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "2",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:03:25 GMT"
      },
      "body": "OK"
    }
  ],
  "suspendedFirst": [
    {
      "at": "2026-10-03T07:03:54.783Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "29",
        "content-type": "application/json",
        "date": "Sat, 03 Oct 2026 07:03:54 GMT",
        "retry-after": "3600"
      },
      "body": "{\"error\":\"service suspended\"}"
    },
    {
      "at": "2026-10-03T07:03:54.961Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:03:54 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:03:55.171Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz?qa=r52",
        "headers": {
          "Accept": "text/html"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:03:55 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:03:55.356Z",
      "request": {
        "method": "HEAD",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "date": "Sat, 03 Oct 2026 07:03:55 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:03:55.911Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/health%7a",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:03:55 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:03:56.088Z",
      "request": {
        "method": "GET",
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz/",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "29",
        "content-type": "application/json",
        "date": "Sat, 03 Oct 2026 07:03:56 GMT",
        "retry-after": "3600"
      },
      "body": "{\"error\":\"service suspended\"}"
    }
  ],
  "suspendedReload": [
    {
      "at": "2026-10-03T07:04:25.911Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:04:25 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:04:26.096Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "29",
        "content-type": "application/json",
        "date": "Sat, 03 Oct 2026 07:04:26 GMT",
        "retry-after": "3600"
      },
      "body": "{\"error\":\"service suspended\"}"
    }
  ],
  "resumed": {
    "at": "2026-10-03T07:05:38.536Z",
    "request": {
      "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
      "method": "GET"
    },
    "status": 200,
    "headers": {
      "content-length": "2",
      "content-type": "text/plain; charset=utf-8",
      "date": "Sat, 03 Oct 2026 07:05:38 GMT"
    },
    "body": "OK"
  },
  "sleepHealthOnly": [
    {
      "at": "2026-10-03T07:10:30.099Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:10:30 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:10:30.278Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz?qa=sleep",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:10:30 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:10:30.454Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/health%7a",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:10:30 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:11:01.455Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:11:01 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:12:27.414Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:12:27 GMT"
      },
      "body": ""
    }
  ],
  "wakeControl": [
    {
      "at": "2026-10-03T07:12:43.032Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz/",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "45",
        "content-type": "application/json",
        "date": "Sat, 03 Oct 2026 07:12:43 GMT",
        "retry-after": "5"
      },
      "body": "{\"error\":\"service hibernated\",\"retryAfter\":5}"
    },
    {
      "at": "2026-10-03T07:13:05.862Z",
      "request": {
        "url": "https://qa-20261003-healthpath-r52.onbex.co/healthz",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "2",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:13:05 GMT"
      },
      "body": "OK"
    }
  ],
  "staticBaseline": [
    {
      "at": "2026-10-03T07:05:20.582Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=0, must-revalidate",
        "content-length": "1548",
        "content-type": "text/html",
        "date": "Sat, 03 Oct 2026 07:05:20 GMT"
      },
      "body": "<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"utf-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n    <title>bex static site</title>\n    <style>\n      *,\n      *::before,\n      *::after {\n        box-sizing: border-box;\n        margin: 0;\n        padding: 0;\n      }\n      body {\n        font-family: system-ui, sans-serif;\n        background: #0f172a;\n        color: #e2e8f0;\n        display: flex;\n        align-items: center;\n        justify-content: center;\n        min-height: 100vh;\n      }\n      .card {\n        text-align: center;\n        padding: 3rem 4rem;\n        border: 1px solid #1e293b;\n        border-radius: 1rem;\n        background: #1e293b;\n        max-width: 480px;\n      }\n      h1 {\n        font-size: 2rem;\n        font-weight: 700;\n        letter-spacing: -0.03em;\n        color: #f8fafc;\n      }\n      p {\n        margin-top: 0.75rem;\n        color: #94a3b8;\n        line-height: 1.6;\n      }\n      .badge {\n        display: inline-block;\n        margin-top: 1.5rem;\n        padding: 0.35rem 0.85rem;\n        border-radius: 9999px;\n        font-size: 0.8rem;\n        font-weight: 600;\n        background: #6366f1;\n        color: #fff;\n      }\n    </style>\n  </head>\n  <body>\n    <div class=\"card\">\n      <h1>Hello from bex</h1>\n      <p>\n        This static site is served by bex — no server, no Dockerfile. Edit\n        <code>index.html</code>, push to git, and bex redeploys instantly.\n      </p>\n      <span class=\"badge\">static_site</span>\n    </div>\n  </body>\n</html>\n"
    },
    {
      "at": "2026-10-03T07:05:20.755Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:05:20 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:05:20.944Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz/",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=0, must-revalidate",
        "content-length": "1548",
        "content-type": "text/html",
        "date": "Sat, 03 Oct 2026 07:05:20 GMT"
      },
      "body": "<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"utf-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n    <title>bex static site</title>\n    <style>\n      *,\n      *::before,\n      *::after {\n        box-sizing: border-box;\n        margin: 0;\n        padding: 0;\n      }\n      body {\n        font-family: system-ui, sans-serif;\n        background: #0f172a;\n        color: #e2e8f0;\n        display: flex;\n        align-items: center;\n        justify-content: center;\n        min-height: 100vh;\n      }\n      .card {\n        text-align: center;\n        padding: 3rem 4rem;\n        border: 1px solid #1e293b;\n        border-radius: 1rem;\n        background: #1e293b;\n        max-width: 480px;\n      }\n      h1 {\n        font-size: 2rem;\n        font-weight: 700;\n        letter-spacing: -0.03em;\n        color: #f8fafc;\n      }\n      p {\n        margin-top: 0.75rem;\n        color: #94a3b8;\n        line-height: 1.6;\n      }\n      .badge {\n        display: inline-block;\n        margin-top: 1.5rem;\n        padding: 0.35rem 0.85rem;\n        border-radius: 9999px;\n        font-size: 0.8rem;\n        font-weight: 600;\n        background: #6366f1;\n        color: #fff;\n      }\n    </style>\n  </head>\n  <body>\n    <div class=\"card\">\n      <h1>Hello from bex</h1>\n      <p>\n        This static site is served by bex — no server, no Dockerfile. Edit\n        <code>index.html</code>, push to git, and bex redeploys instantly.\n      </p>\n      <span class=\"badge\">static_site</span>\n    </div>\n  </body>\n</html>\n"
    },
    {
      "at": "2026-10-03T07:05:21.115Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz?qa=r52",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:05:21 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:05:21.286Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/health%7a",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:05:21 GMT"
      },
      "body": ""
    }
  ],
  "staticSuspended": [
    {
      "at": "2026-10-03T07:06:05.055Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/",
        "method": "GET"
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "27",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:06:05 GMT",
        "retry-after": "3600"
      },
      "body": "This service is suspended.\n"
    },
    {
      "at": "2026-10-03T07:06:05.234Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:06:05 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:06:05.406Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz",
        "method": "HEAD"
      },
      "status": 200,
      "headers": {
        "date": "Sat, 03 Oct 2026 07:06:05 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:06:05.923Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/health%7a",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:06:05 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:06:06.120Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz/",
        "method": "GET"
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "27",
        "content-type": "text/plain; charset=utf-8",
        "date": "Sat, 03 Oct 2026 07:06:06 GMT",
        "retry-after": "3600"
      },
      "body": "This service is suspended.\n"
    }
  ],
  "staticSuspendedReload": [
    {
      "at": "2026-10-03T07:06:54.703Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:06:54 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:06:54.881Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/",
        "method": "GET",
        "headers": {
          "Accept": "application/json"
        }
      },
      "status": 503,
      "headers": {
        "cache-control": "no-store",
        "content-length": "29",
        "content-type": "application/json",
        "date": "Sat, 03 Oct 2026 07:06:54 GMT",
        "retry-after": "3600"
      },
      "body": "{\"error\":\"service suspended\"}"
    }
  ],
  "staticRulesFirst": [
    {
      "at": "2026-10-03T07:08:28.261Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/render.yaml",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=31536000, immutable",
        "content-length": "795",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 07:08:28 GMT"
      },
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    },
    {
      "at": "2026-10-03T07:08:28.440Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:08:28 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:08:28.646Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/qa-health-control",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=31536000, immutable",
        "content-length": "795",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 07:08:28 GMT"
      },
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    }
  ],
  "staticRulesReload": [
    {
      "at": "2026-10-03T07:09:06.181Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/healthz",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "content-length": "0",
        "date": "Sat, 03 Oct 2026 07:09:06 GMT"
      },
      "body": ""
    },
    {
      "at": "2026-10-03T07:09:06.388Z",
      "request": {
        "url": "https://qa-20261003-healthstatic-r52.onbex.co/qa-health-control",
        "method": "GET"
      },
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=31536000, immutable",
        "content-length": "795",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 07:09:06 GMT"
      },
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    }
  ],
  "captures": [
    {
      "at": "2026-10-03T06:59:21.335Z",
      "request": [
        {
          "operationName": "CreateService",
          "variables": {
            "repo": "https://github.com/bex-co/bex",
            "branch": "main",
            "name": "qa-20261003-healthpath-r52",
            "type": "web_service",
            "rootDir": "examples/hello-go",
            "runtime": "go",
            "buildCommand": "go build -o app .",
            "startCommand": "./app",
            "plan": "free",
            "autoDeploy": false,
            "port": 3000,
            "ownerId": "tea-d98210cbbpdc73dcrkvg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation CreateService($name: String!, $ownerId: String, $environmentId: String, $type: String, $repo: String, $image: String, $registryCredentialId: String, $branch: String, $rootDir: String, $runtime: String, $buildCommand: String, $startCommand: String, $dockerfilePath: String, $buildFilter: BuildFilterInput, $plan: String, $autoDeploy: Boolean, $schedule: String, $command: String, $publishPath: String, $port: Int, $envVars: [EnvVarInput], $secretFiles: [SecretFileInput]) {\n  createService(\n    name: $name\n    ownerId: $ownerId\n    environmentId: $environmentId\n    type: $type\n    repo: $repo\n    image: $image\n    registryCredentialId: $registryCredentialId\n    branch: $branch\n    rootDir: $rootDir\n    runtime: $runtime\n    buildCommand: $buildCommand\n    startCommand: $startCommand\n    dockerfilePath: $dockerfilePath\n    buildFilter: $buildFilter\n    plan: $plan\n    autoDeploy: $autoDeploy\n    schedule: $schedule\n    command: $command\n    publishPath: $publishPath\n    port: $port\n    envVars: $envVars\n    secretFiles: $secretFiles\n  ) {\n    id\n    name\n    type\n    phase\n    projectId\n    environmentId\n    registryCredentialId\n    latestDeployId\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"createService\":{\"__typename\":\"Service\",\"environmentId\":null,\"id\":\"srv-db0afi2tm2ss7389qohg\",\"latestDeployId\":\"dep-db0afi2tm2ss7389qoi0\",\"name\":\"qa-20261003-healthpath-r52\",\"phase\":\"\",\"projectId\":null,\"registryCredentialId\":null,\"type\":\"web_service\"}}}]\n"
    },
    {
      "request": [
        {
          "operationName": "SetIdleTimeout",
          "variables": {
            "id": "srv-db0afi2tm2ss7389qohg",
            "idleTTLSeconds": 300
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetIdleTimeout($id: String!, $idleTTLSeconds: Int!) {\n  setIdleTimeout(id: $id, idleTTLSeconds: $idleTTLSeconds) {\n    id\n    idleTTLSeconds\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"setIdleTimeout\":{\"__typename\":\"Service\",\"id\":\"srv-db0afi2tm2ss7389qohg\",\"idleTTLSeconds\":300,\"phase\":\"Building\"}}}]\n"
    },
    {
      "at": "2026-10-03T07:03:00.285Z",
      "request": [
        {
          "operationName": "CreateService",
          "variables": {
            "repo": "https://github.com/bex-co/bex",
            "branch": "main",
            "name": "qa-20261003-healthstatic-r52",
            "type": "static_site",
            "rootDir": "examples/static-site",
            "autoDeploy": false,
            "publishPath": ".",
            "ownerId": "tea-d98210cbbpdc73dcrkvg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation CreateService($name: String!, $ownerId: String, $environmentId: String, $type: String, $repo: String, $image: String, $registryCredentialId: String, $branch: String, $rootDir: String, $runtime: String, $buildCommand: String, $startCommand: String, $dockerfilePath: String, $buildFilter: BuildFilterInput, $plan: String, $autoDeploy: Boolean, $schedule: String, $command: String, $publishPath: String, $port: Int, $envVars: [EnvVarInput], $secretFiles: [SecretFileInput]) {\n  createService(\n    name: $name\n    ownerId: $ownerId\n    environmentId: $environmentId\n    type: $type\n    repo: $repo\n    image: $image\n    registryCredentialId: $registryCredentialId\n    branch: $branch\n    rootDir: $rootDir\n    runtime: $runtime\n    buildCommand: $buildCommand\n    startCommand: $startCommand\n    dockerfilePath: $dockerfilePath\n    buildFilter: $buildFilter\n    plan: $plan\n    autoDeploy: $autoDeploy\n    schedule: $schedule\n    command: $command\n    publishPath: $publishPath\n    port: $port\n    envVars: $envVars\n    secretFiles: $secretFiles\n  ) {\n    id\n    name\n    type\n    phase\n    projectId\n    environmentId\n    registryCredentialId\n    latestDeployId\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"createService\":{\"__typename\":\"Service\",\"environmentId\":null,\"id\":\"srv-db0ah8viujss73au0tgg\",\"latestDeployId\":\"dep-db0ah8viujss73au0th0\",\"name\":\"qa-20261003-healthstatic-r52\",\"phase\":\"\",\"projectId\":null,\"registryCredentialId\":null,\"type\":\"static_site\"}}}]\n"
    },
    {
      "at": "2026-10-03T07:03:37.954Z",
      "request": [
        {
          "operationName": "SuspendService",
          "variables": {
            "id": "srv-db0afi2tm2ss7389qohg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SuspendService($id: String!, $confirm: String) {\n  suspendService(id: $id, confirm: $confirm) {\n    id\n    suspended\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"suspendService\":{\"__typename\":\"Service\",\"id\":\"srv-db0afi2tm2ss7389qohg\",\"phase\":\"Running\",\"suspended\":\"suspended\"}}}]\n"
    },
    {
      "request": {
        "query": "query {service(id:\"srv-db0afi2tm2ss7389qohg\"){id phase suspended revision}}"
      },
      "status": 200,
      "body": "{\"data\":{\"service\":{\"id\":\"srv-db0afi2tm2ss7389qohg\",\"phase\":\"Hibernated\",\"revision\":\"rev-1\",\"suspended\":\"suspended\"}}}\n"
    },
    {
      "at": "2026-10-03T07:05:06.735Z",
      "request": [
        {
          "operationName": "ResumeService",
          "variables": {
            "id": "srv-db0afi2tm2ss7389qohg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation ResumeService($id: String!) {\n  resumeService(id: $id) {\n    id\n    suspended\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"resumeService\":{\"__typename\":\"Service\",\"id\":\"srv-db0afi2tm2ss7389qohg\",\"phase\":\"Hibernated\",\"suspended\":\"not_suspended\"}}}]\n"
    },
    {
      "at": "2026-10-03T07:05:39.373Z",
      "request": [
        {
          "operationName": "SuspendService",
          "variables": {
            "id": "srv-db0ah8viujss73au0tgg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SuspendService($id: String!, $confirm: String) {\n  suspendService(id: $id, confirm: $confirm) {\n    id\n    suspended\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"suspendService\":{\"__typename\":\"Service\",\"id\":\"srv-db0ah8viujss73au0tgg\",\"phase\":\"Running\",\"suspended\":\"suspended\"}}}]\n"
    },
    {
      "at": "2026-10-03T07:06:55.591Z",
      "request": [
        {
          "operationName": "ResumeService",
          "variables": {
            "id": "srv-db0ah8viujss73au0tgg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation ResumeService($id: String!) {\n  resumeService(id: $id) {\n    id\n    suspended\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"resumeService\":{\"__typename\":\"Service\",\"id\":\"srv-db0ah8viujss73au0tgg\",\"phase\":\"Hibernated\",\"suspended\":\"not_suspended\"}}}]\n"
    },
    {
      "request": {
        "query": "query {service(id:\"srv-db0ah8viujss73au0tgg\"){id phase suspended revision routes{type source destination}}}"
      },
      "status": 200,
      "body": "{\"data\":{\"service\":{\"id\":\"srv-db0ah8viujss73au0tgg\",\"phase\":\"Running\",\"revision\":\"rev-1\",\"routes\":[{\"destination\":\"/render.yaml\",\"source\":\"/healthz\",\"type\":\"rewrite\"},{\"destination\":\"/render.yaml\",\"source\":\"/qa-health-control\",\"type\":\"rewrite\"}],\"suspended\":\"not_suspended\"}}}\n"
    },
    {
      "at": "2026-10-03T07:11:23.367Z",
      "request": {
        "method": "GET",
        "url": "https://api.bex.co/v1/services/srv-db0afi2tm2ss7389qohg"
      },
      "status": 200,
      "body": "{\"id\":\"srv-db0afi2tm2ss7389qohg\",\"name\":\"qa-20261003-healthpath-r52\",\"immutableName\":\"qa-20261003-healthpath-r52\",\"slug\":\"qa-20261003-healthpath-r52\",\"displayName\":\"\",\"type\":\"web_service\",\"suspended\":\"not_suspended\",\"dashboardUrl\":\"https://dashboard.bex.co/web/srv-db0afi2tm2ss7389qohg\",\"createdAt\":\"2026-10-03T06:59:21Z\",\"updatedAt\":\"2026-10-03T07:08:20Z\",\"owner\":{\"id\":\"tea-d98210cbbpdc73dcrkvg\",\"name\":\"bex\",\"email\":\"puncsky@gmail.com\",\"type\":\"team\"},\"serviceDetails\":{\"env\":\"go\",\"envSpecificDetails\":{\"buildCommand\":\"go build -o app .\",\"preDeployCommand\":\"\",\"startCommand\":\"./app\"},\"internalAddress\":\"qa-20261003-healthpath-r52:3000\",\"maintenanceMode\":{\"enabled\":false,\"uri\":\"\"},\"maxShutdownDelaySeconds\":30,\"numInstances\":1,\"plan\":\"free\",\"port\":3000,\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"runtime\":\"go\",\"url\":\"https://qa-20261003-healthpath-r52.onbex.co\"},\"suspenders\":[],\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Hibernated\",\"replicas\":1,\"revision\":\"rev-1\",\"urls\":[\"https://qa-20261003-healthpath-r52.onbex.co\"],\"idleTTLSeconds\":300,\"rootDir\":\"examples/hello-go\",\"repo\":\"https://github.com/bex-co/bex\",\"branch\":\"main\",\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"pushDeliveryMethod\":\"github_app\",\"notifyOnFail\":\"default\",\"notificationsToSend\":\"default\"}\n"
    },
    {
      "at": "2026-10-03T07:11:23.571Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/mcp",
        "body": {
          "jsonrpc": "2.0",
          "id": "qa-r52-srv-db0afi2tm2ss7389qohg",
          "method": "tools/call",
          "params": {
            "name": "get_service",
            "arguments": {
              "serviceId": "srv-db0afi2tm2ss7389qohg"
            }
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"qa-r52-srv-db0afi2tm2ss7389qohg\",\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"autoDeploy\\\":\\\"no\\\",\\\"autoDeployTrigger\\\":\\\"off\\\",\\\"branch\\\":\\\"main\\\",\\\"createdAt\\\":\\\"2026-10-03T06:59:21Z\\\",\\\"dashboardUrl\\\":\\\"https://dashboard.bex.co/web/srv-db0afi2tm2ss7389qohg\\\",\\\"displayName\\\":\\\"\\\",\\\"id\\\":\\\"srv-db0afi2tm2ss7389qohg\\\",\\\"idleTTLSeconds\\\":300,\\\"immutableName\\\":\\\"qa-20261003-healthpath-r52\\\",\\\"name\\\":\\\"qa-20261003-healthpath-r52\\\",\\\"notificationsToSend\\\":\\\"default\\\",\\\"notifyOnFail\\\":\\\"default\\\",\\\"ownerId\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"phase\\\":\\\"Hibernated\\\",\\\"pushDeliveryMethod\\\":\\\"github_app\\\",\\\"replicas\\\":1,\\\"repo\\\":\\\"https://github.com/bex-co/bex\\\",\\\"revision\\\":\\\"rev-1\\\",\\\"rootDir\\\":\\\"examples/hello-go\\\",\\\"serviceDetails\\\":{\\\"env\\\":\\\"go\\\",\\\"envSpecificDetails\\\":{\\\"buildCommand\\\":\\\"go build -o app .\\\",\\\"preDeployCommand\\\":\\\"\\\",\\\"startCommand\\\":\\\"./app\\\"},\\\"internalAddress\\\":\\\"qa-20261003-healthpath-r52:3000\\\",\\\"maintenanceMode\\\":{\\\"enabled\\\":false,\\\"uri\\\":\\\"\\\"},\\\"maxShutdownDelaySeconds\\\":30,\\\"numInstances\\\":1,\\\"plan\\\":\\\"free\\\",\\\"port\\\":3000,\\\"region\\\":\\\"fsn1\\\",\\\"renderSubdomainPolicy\\\":\\\"enabled\\\",\\\"runtime\\\":\\\"go\\\",\\\"url\\\":\\\"https://qa-20261003-healthpath-r52.onbex.co\\\"},\\\"slug\\\":\\\"qa-20261003-healthpath-r52\\\",\\\"suspended\\\":\\\"not_suspended\\\",\\\"suspenders\\\":[],\\\"type\\\":\\\"web_service\\\",\\\"updatedAt\\\":\\\"2026-10-03T07:08:20Z\\\",\\\"urls\\\":[\\\"https://qa-20261003-healthpath-r52.onbex.co\\\"]}\"}],\"structuredContent\":{\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"branch\":\"main\",\"createdAt\":\"2026-10-03T06:59:21Z\",\"dashboardUrl\":\"https://dashboard.bex.co/web/srv-db0afi2tm2ss7389qohg\",\"displayName\":\"\",\"id\":\"srv-db0afi2tm2ss7389qohg\",\"idleTTLSeconds\":300,\"immutableName\":\"qa-20261003-healthpath-r52\",\"name\":\"qa-20261003-healthpath-r52\",\"notificationsToSend\":\"default\",\"notifyOnFail\":\"default\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Hibernated\",\"pushDeliveryMethod\":\"github_app\",\"replicas\":1,\"repo\":\"https://github.com/bex-co/bex\",\"revision\":\"rev-1\",\"rootDir\":\"examples/hello-go\",\"serviceDetails\":{\"env\":\"go\",\"envSpecificDetails\":{\"buildCommand\":\"go build -o app .\",\"preDeployCommand\":\"\",\"startCommand\":\"./app\"},\"internalAddress\":\"qa-20261003-healthpath-r52:3000\",\"maintenanceMode\":{\"enabled\":false,\"uri\":\"\"},\"maxShutdownDelaySeconds\":30,\"numInstances\":1,\"plan\":\"free\",\"port\":3000,\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"runtime\":\"go\",\"url\":\"https://qa-20261003-healthpath-r52.onbex.co\"},\"slug\":\"qa-20261003-healthpath-r52\",\"suspended\":\"not_suspended\",\"suspenders\":[],\"type\":\"web_service\",\"updatedAt\":\"2026-10-03T07:08:20Z\",\"urls\":[\"https://qa-20261003-healthpath-r52.onbex.co\"]}}}\n\n"
    },
    {
      "at": "2026-10-03T07:11:23.790Z",
      "request": {
        "method": "GET",
        "url": "https://api.bex.co/v1/services/srv-db0ah8viujss73au0tgg"
      },
      "status": 200,
      "body": "{\"id\":\"srv-db0ah8viujss73au0tgg\",\"name\":\"qa-20261003-healthstatic-r52\",\"immutableName\":\"qa-20261003-healthstatic-r52\",\"slug\":\"qa-20261003-healthstatic-r52\",\"displayName\":\"\",\"type\":\"static_site\",\"suspended\":\"not_suspended\",\"dashboardUrl\":\"https://dashboard.bex.co/static/srv-db0ah8viujss73au0tgg\",\"createdAt\":\"2026-10-03T07:03:00Z\",\"updatedAt\":\"2026-10-03T07:07:39.310148796Z\",\"owner\":{\"id\":\"tea-d98210cbbpdc73dcrkvg\",\"name\":\"bex\",\"email\":\"puncsky@gmail.com\",\"type\":\"team\"},\"serviceDetails\":{\"buildCommand\":\"\",\"numInstances\":1,\"plan\":\"free\",\"publishPath\":\".\",\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"url\":\"https://qa-20261003-healthstatic-r52.onbex.co\"},\"suspenders\":[],\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Running\",\"replicas\":1,\"revision\":\"rev-1\",\"urls\":[\"https://qa-20261003-healthstatic-r52.onbex.co\"],\"idleTTLSeconds\":0,\"rootDir\":\"examples/static-site\",\"repo\":\"https://github.com/bex-co/bex\",\"branch\":\"main\",\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"pushDeliveryMethod\":\"github_app\",\"notifyOnFail\":\"default\",\"notificationsToSend\":\"default\"}\n"
    },
    {
      "at": "2026-10-03T07:11:23.996Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/mcp",
        "body": {
          "jsonrpc": "2.0",
          "id": "qa-r52-srv-db0ah8viujss73au0tgg",
          "method": "tools/call",
          "params": {
            "name": "get_service",
            "arguments": {
              "serviceId": "srv-db0ah8viujss73au0tgg"
            }
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"qa-r52-srv-db0ah8viujss73au0tgg\",\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"autoDeploy\\\":\\\"no\\\",\\\"autoDeployTrigger\\\":\\\"off\\\",\\\"branch\\\":\\\"main\\\",\\\"createdAt\\\":\\\"2026-10-03T07:03:00Z\\\",\\\"dashboardUrl\\\":\\\"https://dashboard.bex.co/static/srv-db0ah8viujss73au0tgg\\\",\\\"displayName\\\":\\\"\\\",\\\"id\\\":\\\"srv-db0ah8viujss73au0tgg\\\",\\\"idleTTLSeconds\\\":0,\\\"immutableName\\\":\\\"qa-20261003-healthstatic-r52\\\",\\\"name\\\":\\\"qa-20261003-healthstatic-r52\\\",\\\"notificationsToSend\\\":\\\"default\\\",\\\"notifyOnFail\\\":\\\"default\\\",\\\"ownerId\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"phase\\\":\\\"Running\\\",\\\"pushDeliveryMethod\\\":\\\"github_app\\\",\\\"replicas\\\":1,\\\"repo\\\":\\\"https://github.com/bex-co/bex\\\",\\\"revision\\\":\\\"rev-1\\\",\\\"rootDir\\\":\\\"examples/static-site\\\",\\\"serviceDetails\\\":{\\\"buildCommand\\\":\\\"\\\",\\\"numInstances\\\":1,\\\"plan\\\":\\\"free\\\",\\\"publishPath\\\":\\\".\\\",\\\"region\\\":\\\"fsn1\\\",\\\"renderSubdomainPolicy\\\":\\\"enabled\\\",\\\"url\\\":\\\"https://qa-20261003-healthstatic-r52.onbex.co\\\"},\\\"slug\\\":\\\"qa-20261003-healthstatic-r52\\\",\\\"suspended\\\":\\\"not_suspended\\\",\\\"suspenders\\\":[],\\\"type\\\":\\\"static_site\\\",\\\"updatedAt\\\":\\\"2026-10-03T07:07:39.310148796Z\\\",\\\"urls\\\":[\\\"https://qa-20261003-healthstatic-r52.onbex.co\\\"]}\"}],\"structuredContent\":{\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"branch\":\"main\",\"createdAt\":\"2026-10-03T07:03:00Z\",\"dashboardUrl\":\"https://dashboard.bex.co/static/srv-db0ah8viujss73au0tgg\",\"displayName\":\"\",\"id\":\"srv-db0ah8viujss73au0tgg\",\"idleTTLSeconds\":0,\"immutableName\":\"qa-20261003-healthstatic-r52\",\"name\":\"qa-20261003-healthstatic-r52\",\"notificationsToSend\":\"default\",\"notifyOnFail\":\"default\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Running\",\"pushDeliveryMethod\":\"github_app\",\"replicas\":1,\"repo\":\"https://github.com/bex-co/bex\",\"revision\":\"rev-1\",\"rootDir\":\"examples/static-site\",\"serviceDetails\":{\"buildCommand\":\"\",\"numInstances\":1,\"plan\":\"free\",\"publishPath\":\".\",\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"url\":\"https://qa-20261003-healthstatic-r52.onbex.co\"},\"slug\":\"qa-20261003-healthstatic-r52\",\"suspended\":\"not_suspended\",\"suspenders\":[],\"type\":\"static_site\",\"updatedAt\":\"2026-10-03T07:07:39.310148796Z\",\"urls\":[\"https://qa-20261003-healthstatic-r52.onbex.co\"]}}}\n\n"
    },
    {
      "at": "2026-10-03T07:12:27.694Z",
      "request": {
        "query": "query {service(id:\"srv-db0afi2tm2ss7389qohg\"){id phase suspended revision}}"
      },
      "response": {
        "status": 200,
        "body": "{\"data\":{\"service\":{\"id\":\"srv-db0afi2tm2ss7389qohg\",\"phase\":\"Hibernated\",\"revision\":\"rev-1\",\"suspended\":\"not_suspended\"}}}\n"
      }
    },
    {
      "at": "2026-10-03T07:13:29.459Z",
      "request": [
        {
          "operationName": "DeleteService",
          "variables": {
            "id": "srv-db0afi2tm2ss7389qohg"
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
      "body": "[{\"data\":{\"deleteService\":true}}]\n"
    },
    {
      "at": "2026-10-03T07:13:32.443Z",
      "request": [
        {
          "operationName": "DeleteService",
          "variables": {
            "id": "srv-db0ah8viujss73au0tgg"
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
      "body": "[{\"data\":{\"deleteService\":true}}]\n"
    }
  ],
  "cleanup": [
    {
      "id": "srv-db0afi2tm2ss7389qohg",
      "api": {
        "status": 404,
        "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      },
      "publicHealthz": {
        "status": 404,
        "body": "404 page not found\n"
      }
    },
    {
      "id": "srv-db0ah8viujss73au0tgg",
      "api": {
        "status": 404,
        "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      },
      "publicHealthz": {
        "status": 404,
        "body": "404 page not found\n"
      }
    }
  ]
}
```

Read-only cluster observations, projected from the owned App/Deployment (not a claim that the entire CR was printed): 07:04:45 web `suspended=true, phase=Hibernated, last-active=07:03:09Z`, Deployment replicas0; 07:10:33 replicas0/ready0; 07:12:29 still Hibernated/replicas0/ready0 with the same annotation; 07:13:05 after the slash request Running, last-active07:12:42Z. The stored App `spec.replicas=1` is desired awake capacity; it does not contradict actual Deployment replicas0.

Local artifacts (existence checked; screenshots visually inspected): `.playwright-mcp/qa-r52-api-captures.json`, `qa-r52-web-suspended.png`, `qa-r52-static-suspended.png`, `qa-r52-static-health-rules.png`, `qa-r52-suspended-healthz-headers.txt`, `qa-r52-suspended-healthz-body.txt` (external curl, HTTP/2 200, zero bytes), `qa-r52-console.txt`, `qa-r52-network.txt`, and `qa-muse-r52-ledger.json`. These gitignored artifacts are supplemental; the inline records carry the handoff.

## Cleanup and limits

Both UI deletes returned true; both REST by-id reads and public healthz URLs returned404. Exact metadata-filtered inventories in the tenant and bex-build namespaces found no owned App/Deployment/ReplicaSet/pod/Service/Ingress/Job/Secret residue. The static auxiliary tab was closed, the QA Kratos session was revoked successfully, cookies and local session handles removed. No foreign resources were mutated.

No paid maintenance, controlled custom domain, platform probe change, local fix, product regression suite, or production rollout was performed. Implementation tasks must verify these adjacent cases where specified. This filing ships research and scheduled work only.
