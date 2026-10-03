# Static browser caches retain a previous successful deployment

Severity: **major**. Returning visitors can keep old files after a successful republish; observed on a plain YAML filename. JS/CSS/configuration impact follows the same extension policy but those application effects were not executed live.

## Reproduction and controls

Fixture `qa-20261002-http-r43`, `srv-db090ogehcmc739j116g`, workspace `tea-d98210cbbpdc73dcrkvg`, App UID `1d2bef9f-859c-4160-a299-1fd80678b3cb`. No product source was changed. A Free no-build static deployment published existing public repository directories; changing the root also intentionally removed/restored index.html. The temporary root 404 under hello-go was expected fixture content, not a finding.

1. Create from https://github.com/bex-co/bex main, root `examples/static-site`, publish `.`, build blank, auto-deploy Off. Initial deploy `dep-db090ogehcmc739j1170` Live, rev-1, public HTML 200. Separate browser tab at the public root; `fetch('/render.yaml')` returned the static-site manifest with `Cache-Control: public, max-age=31536000, immutable`.
2. Settings → Edit Root Directory → `examples/hello-go` → Save changes → confirm. Wait for Running rev-2 and the new public bytes. At 05:23:06Z, a normal same-URL fetch still returned the old static-site manifest; `fetch('/render.yaml', {cache:'no-store'})` returned hello-go. Reloading the visitor page and repeating the normal fetch at 05:23:30Z still returned the old file.
3. Seed `/render.yaml?qa=r43-b` normally while hello-go is active. Restore `examples/static-site` through the same Settings flow. Running rev-3, deploy `dep-db092koehcmc739j1190`. After another fresh visitor-page load, normal fetch of that same query URL retained hello-go (614 bytes, Date 05:23:31Z); no-store returned the new static manifest (795 bytes). Root HTML used `public, max-age=0, must-revalidate` and returned the restored index.
4. Save `/render.yaml` / `Cache-Control` / `public, max-age=0, must-revalidate` in Headers, reload. A new query URL `/render.yaml?qa=r43-control` received that policy and the static manifest. The previously cached `?qa=r43-b` remained stale even though a no-store request now received the override. Change Root Directory back to hello-go; Running rev-5 (the header edit consumed a metadata generation). Normal fetch of the seeded control URL now returned hello-go at 05:27:40Z. This is an actual cross-publish control, not merely a changed header string.
5. REST, GraphQL and MCP service reads agreed on Running/revision/rootDir. Header reads and saved UI agreed on the explicit override. The control plane was not lying about publication; browser freshness hid the new bytes.

Browser: Chromium **154.0.8037.95**. No Playwright request interception, cache disabling, service worker, or product DOM replacement was used for the proof. The visitor's explicit fetches used default browser caching except the named no-store controls. The repo sample has no service-worker registration. Old response retention is observed over minutes; a full year was not elapsed. One year is the freshness lifetime advertised by the header, subject to browser eviction/user intervention.

Initial curl GET/HEAD `/`, `/render.yaml` succeeded; `/absent.js` correctly returned 404 and extensionless `/dashboard` served the documented SPA fallback. Range bytes=0-9 returned the whole file with 200, an optional range feature not filed here. HEAD capture duplicated each header because the probe used both `-I` and `-D -`; this is a probe formatting artifact, not duplicate wire headers. HEAD omitted Content-Length, which is permitted; not filed.

## Root cause and concrete target

- `lego/operator/internal/staticserver/staticserver.go:333-360` writes successful object responses. At **351** it sets the cache policy, then **353** applies explicit request-path headers. `cacheControl` at **662-673** treats only `.html`, `.htm` and extensionless served paths as revalidating; every other extension gets a year plus immutable. The comment assumes build tooling hashes assets, yet the test fixture `/assets/app.js` is not hashed (`staticserver_test.go:116-130`). Plain no-build files are supported product inputs.
- The default's immutability reasoning confuses two identities. `Site.keyFor` at **131-140** and `get` at **422 onward** key server objects by revision prefix. `resolver.go:78-94,135-141` selects the active prefix. The public URL remains `/render.yaml` across those revisions. A browser keys its response by the public URL, not the private S3 prefix. No-store/new-query controls prove the server already had the new bytes; purging its origin cache cannot repair this.
- `s3origin.go:92-117` returns body and content type, not a public freshness policy. `serveObject` supplies the problematic header itself. The configuration schema can express the safe override: `types/v1alpha1/app_types.go:1140-1154` has string Name/Value; backend `apps/service.go:4740-4770,4884` validates and saves it. No serializer or non-null restriction prevents the target value.
- Consumer path: browser freshness accepts the emitted Cache-Control. [RFC 8246 section 2](https://www.rfc-editor.org/rfc/rfc8246) defines immutable as unchanged during freshness; [RFC 9111](https://www.rfc-editor.org/rfc/rfc9111) defines freshness/revalidation. Removing only immutable leaves the year-long max-age and does not fix ordinary fetches. The actual Chromium behavior was observed, not inferred from a generic browser model.
- Standard library inspected locally on Go **1.26.5** (platform Dockerfile pins the Go 1.26 builder family): `net/http/header.go:39-41` delegates Set to `net/textproto/header.go:21-23`, replacing with one canonical value; `net/http/server.go:1532-1535` writes the selected header map. There is no framework cache transform to fix. The source claim is about these inspected paths, not an assertion that production's exact Go patch version was queried.

**Target:** successful static responses with no matching custom Cache-Control rule use `public, max-age=0, must-revalidate` for every filename, including hash-looking names, GET and HEAD, directly served files, rewrite targets and fallback. Keep the immutable revision-keyed server cache intact. Preserve matching explicit tenant Cache-Control overrides and last-matching-rule precedence. Owners who know their filenames are content-addressed can already opt into long freshness through Headers; do not guess immutability from extensions or filename regexes. ETags/304 optimization are separate optional work, not required to make revalidation return correct bytes.

**Rollout limit:** new default headers cannot recall legacy responses that browsers do not request. The live override confirmed this. Document forced reload/cache eviction or changing the affected asset URL as recovery for already cached clients. Do not promise retroactive refresh, issue global Clear-Site-Data, add a purge API, or rewrite tenants' HTML. A fix verified against an old legacy cache entry would falsely fail; begin fixed-policy acceptance with a fresh cache, then reuse it across the two publishes.

## Blast radius, aliases and neighboring behavior

Repository search found **one production call** to `cacheControl`, **three calls** to `serveObject` (staticserver.go:251 direct hit, :300 rewrite hit, :310 SPA fallback) and **one production registration** of this handler (`cmd/staticserver/main.go:166`). `resolver.go:78` restricts it to nonsuspended published static_site Apps. Family census: **static affected**; web/private/worker/cron use their own workload serving, Postgres and Key Value their database protocols, so all six are outside this change. No claim of live testing those six in this sweep.

Host aliases from `effectiveHosts` (`resolver.go:115-130`, delegating to CRD EffectiveHosts) share the handler: platform hostname, spec.host and configured hosts/custom domains. Only the platform hostname was exercised. Public GET/HEAD arbitrary paths, directory index, rewrite and fallback are aliases of successful delivery. Custom Cache-Control applies after the default via `applyHeaders:577-583` and must continue to win. The error helper `applyErrorHeaders` is separate: 400 invalid route, 404 misses, 413 oversize, 502 origin failure, 503 overload/Retry-After and 405 unknown-method behavior must not acquire a success policy or lose their own headers. Auth/authorization and unknown-host isolation remain unchanged; no new existence signal.

Existing control-plane aliases need **no schema change**: REST GET/PUT `/v1/services/{id}/headers` (`apps/rest.go:1221-1247`); GraphQL service.headers and setStaticHeaders (`graphql.go:505,1751-1758`); MCP list_static_headers/update_static_headers (`mcp.go:1053-1073`). Dashboard `features/services/api/static-site.graphql:17-27` and `hooks/use-static-site.ts:98-115` save/refetch string rules. The three read forms and UI save were exercised; REST/MCP writes were not. Service read aliases GET service, GraphQL service and MCP get_service agreed, exact responses below.

Pre-settle behavior: continue serving the previous successful revision until atomic publication and resolver refresh; do not treat in-flight old bytes as this failure. Both failing probes ran after new bytes were visible through no-store. No UI pending-state change is proposed.

## Dedupe, history and prior guarantees

Searched all open/blocked/done PM text for browser caching, stale static content, cacheControl, immutable and 31536000; scanned **46 open/blocked milestone README titles** (the broader inventory contained 68 README files including workstreams/dev stacks). Read DO_NOT_DO, recent 40 dashboard/lego commits, targeted file history and `git log -S 31536000`. No open owner or pending-main fix found. Policy appears unchanged in original **84e221650** (w1/m21); **41e6f1c29** (w4/m94) added a test asserting the existing asset policy, not a repair of browser refresh. This is an original uncovered guarantee, not a claimed recurrence of a fixed browser-cache bug.

DO_NOT_DO's static CDN cache-purge exclusion still applies and is preserved: immutable per-revision origin objects do not need purging. This filing concerns mutable public URL response headers only. Existing w4/m150 wildcard grammar, m154 URL-component expansion and m155 failed-publish phase are distinct; their full hosted acceptance is not closed here. w9/m89 and m92 prior QA filing shape reviewed; auth/agent findings unrelated.

Full original DoD disposition to avoid overstating prior milestones:

- **w1/done/m21:** (1) no-build publication and public serving passed; a compiled static build/no workload inventory at creation was not separately tested here. (2) explicit cache header save/live delivery passed; redirect/rewrite behavior not rerun. (3) UI create/update plus REST/GraphQL/MCP reads passed; all API writes not rerun. (4) documentation rows exist; this new browser freshness gap narrows the broad success claim, no blanket parity closure.
- **w4/done/m94:** (1) existing-file/SPA behavior without a configured wildcard passed; toggling a configured rule not rerun. (2) direct GET/HEAD existing file passed; explicit redirect configuration not rerun. (3) request-path rewrite header pair not rerun; exact cache header control passed. (4) saved cache rule survived UI reload/API reads; no project membership fixture in this run; deletion tracked below. (5) error/resource-family tests not rerun in filing-only work and are required regression boundaries.

[Render static-site documentation](https://render.com/docs/static-sites) describes atomic publication and latest working content after CDN invalidation; [custom headers](https://render.com/docs/static-site-headers) documents owner Cache-Control configuration. Checked 2026-10-03. No authenticated Render site or exact default header policy was probed; the selected revalidating default is the bounded bex fix, not a claimed verbatim Render wire contract.

## Evidence and unverified work

Local `.playwright-mcp/qa-r43-running-rev3.png` was visually inspected: it supports Running/Live rev-3 only, not the browser-cache comparison. Durable complete requests/responses and byte bodies below are the cache evidence. Local files: qa-r43-api-captures.json, qa-r43-http-initial.json, qa-r43-console.txt, qa-r43-network.txt, qa-r43-cache-history.txt, qa-r43-dedupe.txt and qa-muse-r43-ledger.json. The files are gitignored, so this record embeds the replay data.

No product fix or product test suite ran for this filing. Browser-specific cache eviction, compiled JS/CSS execution, custom domains, multiple matching overrides, explicit immutable opt-in across deployment, origin-error classes, HTTP validators and hashed assets are unverified live; use local regression coverage and bounded hosted acceptance in the tasks. No need to add features for range/ETag to close this bug.

## Durable exact probe data

Browser probes used a separate tab at the owned site's origin. Default probe: `const r = await fetch('/render.yaml'); ({status:r.status, body:await r.text()})`. Named controls only used `{cache:'no-store'}` or the recorded stable query URL. Saved mutations and API reads include their entire request payloads and responses below; cookie/session material is excluded.

```json
{
  "browser": "154.0.8037.95",
  "captures": [
    {
      "at": "2026-10-03T05:19:31.299Z",
      "request": [
        {
          "operationName": "CreateService",
          "variables": {
            "repo": "https://github.com/bex-co/bex",
            "branch": "main",
            "name": "qa-20261002-http-r43",
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
      "responseRaw": "[{\"data\":{\"createService\":{\"__typename\":\"Service\",\"environmentId\":null,\"id\":\"srv-db090ogehcmc739j116g\",\"latestDeployId\":\"dep-db090ogehcmc739j1170\",\"name\":\"qa-20261002-http-r43\",\"phase\":\"\",\"projectId\":null,\"registryCredentialId\":null,\"type\":\"static_site\"}}}]\n"
    },
    {
      "at": "2026-10-03T05:22:13.199Z",
      "request": [
        {
          "operationName": "SetRootDir",
          "variables": {
            "id": "srv-db090ogehcmc739j116g",
            "rootDir": "examples/hello-go"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetRootDir($id: String!, $rootDir: String!, $confirm: String) {\n  setRootDir(id: $id, rootDir: $rootDir, confirm: $confirm) {\n    id\n    repo\n    branch\n    rootDir\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"setRootDir\":{\"__typename\":\"Service\",\"branch\":\"main\",\"id\":\"srv-db090ogehcmc739j116g\",\"phase\":\"Running\",\"repo\":\"https://github.com/bex-co/bex\",\"rootDir\":\"examples/hello-go\"}}}]\n"
    },
    {
      "at": "2026-10-03T05:23:31.858Z",
      "request": [
        {
          "operationName": "SetRootDir",
          "variables": {
            "id": "srv-db090ogehcmc739j116g",
            "rootDir": "examples/static-site"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetRootDir($id: String!, $rootDir: String!, $confirm: String) {\n  setRootDir(id: $id, rootDir: $rootDir, confirm: $confirm) {\n    id\n    repo\n    branch\n    rootDir\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"setRootDir\":{\"__typename\":\"Service\",\"branch\":\"main\",\"id\":\"srv-db090ogehcmc739j116g\",\"phase\":\"Running\",\"repo\":\"https://github.com/bex-co/bex\",\"rootDir\":\"examples/static-site\"}}}]\n"
    },
    {
      "at": "2026-10-03T05:26:19.641Z",
      "request": [
        {
          "operationName": "SetStaticHeaders",
          "variables": {
            "id": "srv-db090ogehcmc739j116g",
            "headers": [
              {
                "path": "/render.yaml",
                "name": "Cache-Control",
                "value": "public, max-age=0, must-revalidate"
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
      "responseRaw": "[{\"data\":{\"setStaticHeaders\":{\"__typename\":\"Service\",\"headers\":[{\"__typename\":\"StaticHeader\",\"name\":\"Cache-Control\",\"path\":\"/render.yaml\",\"value\":\"public, max-age=0, must-revalidate\"}],\"id\":\"srv-db090ogehcmc739j116g\"}}}]\n"
    },
    {
      "at": "2026-10-03T05:27:12.546Z",
      "request": [
        {
          "operationName": "SetRootDir",
          "variables": {
            "id": "srv-db090ogehcmc739j116g",
            "rootDir": "examples/hello-go"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetRootDir($id: String!, $rootDir: String!, $confirm: String) {\n  setRootDir(id: $id, rootDir: $rootDir, confirm: $confirm) {\n    id\n    repo\n    branch\n    rootDir\n    phase\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"setRootDir\":{\"__typename\":\"Service\",\"branch\":\"main\",\"id\":\"srv-db090ogehcmc739j116g\",\"phase\":\"Running\",\"repo\":\"https://github.com/bex-co/bex\",\"rootDir\":\"examples/hello-go\"}}}]\n"
    },
    {
      "at": "2026-10-03T05:28:15.563Z",
      "request": [
        {
          "operationName": "DeleteService",
          "variables": {
            "id": "srv-db090ogehcmc739j116g"
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
      "responseRaw": "[{\"data\":{\"deleteService\":true}}]\n"
    }
  ],
  "reads": [
    {
      "path": "/v1/services/srv-db090ogehcmc739j116g",
      "request": null,
      "status": 200,
      "body": "{\"id\":\"srv-db090ogehcmc739j116g\",\"name\":\"qa-20261002-http-r43\",\"immutableName\":\"qa-20261002-http-r43\",\"slug\":\"qa-20261002-http-r43\",\"displayName\":\"\",\"type\":\"static_site\",\"suspended\":\"not_suspended\",\"dashboardUrl\":\"https://dashboard.bex.co/static/srv-db090ogehcmc739j116g\",\"createdAt\":\"2026-10-03T05:19:31Z\",\"updatedAt\":\"2026-10-03T05:23:47Z\",\"owner\":{\"id\":\"tea-d98210cbbpdc73dcrkvg\",\"name\":\"bex\",\"email\":\"puncsky@gmail.com\",\"type\":\"team\"},\"serviceDetails\":{\"buildCommand\":\"\",\"numInstances\":1,\"plan\":\"free\",\"publishPath\":\".\",\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"url\":\"https://qa-20261002-http-r43.onbex.co\"},\"suspenders\":[],\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Running\",\"replicas\":1,\"revision\":\"rev-3\",\"urls\":[\"https://qa-20261002-http-r43.onbex.co\"],\"idleTTLSeconds\":0,\"rootDir\":\"examples/static-site\",\"repo\":\"https://github.com/bex-co/bex\",\"branch\":\"main\",\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"pushDeliveryMethod\":\"github_app\",\"notifyOnFail\":\"default\",\"notificationsToSend\":\"default\"}\n"
    },
    {
      "path": "/graphql",
      "request": {
        "query": "query { service(id:\"srv-db090ogehcmc739j116g\") { id type phase revision rootDir } }"
      },
      "status": 200,
      "body": "{\"data\":{\"service\":{\"id\":\"srv-db090ogehcmc739j116g\",\"phase\":\"Running\",\"revision\":\"rev-3\",\"rootDir\":\"examples/static-site\",\"type\":\"static_site\"}}}\n"
    },
    {
      "path": "/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 43,
        "method": "tools/call",
        "params": {
          "name": "get_service",
          "arguments": {
            "serviceId": "srv-db090ogehcmc739j116g"
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":43,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"autoDeploy\\\":\\\"no\\\",\\\"autoDeployTrigger\\\":\\\"off\\\",\\\"branch\\\":\\\"main\\\",\\\"createdAt\\\":\\\"2026-10-03T05:19:31Z\\\",\\\"dashboardUrl\\\":\\\"https://dashboard.bex.co/static/srv-db090ogehcmc739j116g\\\",\\\"displayName\\\":\\\"\\\",\\\"id\\\":\\\"srv-db090ogehcmc739j116g\\\",\\\"idleTTLSeconds\\\":0,\\\"immutableName\\\":\\\"qa-20261002-http-r43\\\",\\\"name\\\":\\\"qa-20261002-http-r43\\\",\\\"notificationsToSend\\\":\\\"default\\\",\\\"notifyOnFail\\\":\\\"default\\\",\\\"ownerId\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"phase\\\":\\\"Running\\\",\\\"pushDeliveryMethod\\\":\\\"github_app\\\",\\\"replicas\\\":1,\\\"repo\\\":\\\"https://github.com/bex-co/bex\\\",\\\"revision\\\":\\\"rev-3\\\",\\\"rootDir\\\":\\\"examples/static-site\\\",\\\"serviceDetails\\\":{\\\"buildCommand\\\":\\\"\\\",\\\"numInstances\\\":1,\\\"plan\\\":\\\"free\\\",\\\"publishPath\\\":\\\".\\\",\\\"region\\\":\\\"fsn1\\\",\\\"renderSubdomainPolicy\\\":\\\"enabled\\\",\\\"url\\\":\\\"https://qa-20261002-http-r43.onbex.co\\\"},\\\"slug\\\":\\\"qa-20261002-http-r43\\\",\\\"suspended\\\":\\\"not_suspended\\\",\\\"suspenders\\\":[],\\\"type\\\":\\\"static_site\\\",\\\"updatedAt\\\":\\\"2026-10-03T05:23:47Z\\\",\\\"urls\\\":[\\\"https://qa-20261002-http-r43.onbex.co\\\"]}\"}],\"structuredContent\":{\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"branch\":\"main\",\"createdAt\":\"2026-10-03T05:19:31Z\",\"dashboardUrl\":\"https://dashboard.bex.co/static/srv-db090ogehcmc739j116g\",\"displayName\":\"\",\"id\":\"srv-db090ogehcmc739j116g\",\"idleTTLSeconds\":0,\"immutableName\":\"qa-20261002-http-r43\",\"name\":\"qa-20261002-http-r43\",\"notificationsToSend\":\"default\",\"notifyOnFail\":\"default\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Running\",\"pushDeliveryMethod\":\"github_app\",\"replicas\":1,\"repo\":\"https://github.com/bex-co/bex\",\"revision\":\"rev-3\",\"rootDir\":\"examples/static-site\",\"serviceDetails\":{\"buildCommand\":\"\",\"numInstances\":1,\"plan\":\"free\",\"publishPath\":\".\",\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"url\":\"https://qa-20261002-http-r43.onbex.co\"},\"slug\":\"qa-20261002-http-r43\",\"suspended\":\"not_suspended\",\"suspenders\":[],\"type\":\"static_site\",\"updatedAt\":\"2026-10-03T05:23:47Z\",\"urls\":[\"https://qa-20261002-http-r43.onbex.co\"]}}}\n\n"
    }
  ],
  "headerReads": [
    {
      "path": "/v1/services/srv-db090ogehcmc739j116g/headers",
      "request": null,
      "status": 200,
      "body": "[{\"path\":\"/render.yaml\",\"name\":\"Cache-Control\",\"value\":\"public, max-age=0, must-revalidate\"}]\n"
    },
    {
      "path": "/graphql",
      "request": {
        "query": "query { service(id:\"srv-db090ogehcmc739j116g\") { id type phase revision rootDir headers { path name value } } }"
      },
      "status": 200,
      "body": "{\"data\":{\"service\":{\"headers\":[{\"name\":\"Cache-Control\",\"path\":\"/render.yaml\",\"value\":\"public, max-age=0, must-revalidate\"}],\"id\":\"srv-db090ogehcmc739j116g\",\"phase\":\"Running\",\"revision\":\"rev-5\",\"rootDir\":\"examples/hello-go\",\"type\":\"static_site\"}}}\n"
    },
    {
      "path": "/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 44,
        "method": "tools/call",
        "params": {
          "name": "list_static_headers",
          "arguments": {
            "serviceId": "srv-db090ogehcmc739j116g"
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":44,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"headers\\\":[{\\\"name\\\":\\\"Cache-Control\\\",\\\"path\\\":\\\"/render.yaml\\\",\\\"value\\\":\\\"public, max-age=0, must-revalidate\\\"}]}\"}],\"structuredContent\":{\"headers\":[{\"name\":\"Cache-Control\",\"path\":\"/render.yaml\",\"value\":\"public, max-age=0, must-revalidate\"}]}}}\n\n"
    }
  ],
  "firstProof": {
    "at": "2026-10-03T05:23:06.835Z",
    "before": {
      "status": 200,
      "cacheControl": "public, max-age=31536000, immutable",
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    },
    "cached": {
      "status": 200,
      "cacheControl": "public, max-age=31536000, immutable",
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    },
    "fresh": {
      "status": 200,
      "cacheControl": "public, max-age=31536000, immutable",
      "body": "# render.yaml — the manifest for deploy-from-chat (pillar 4).\n# Deploy in one call: give an agent the repo + this file via the MCP `deploy`\n# tool (or POST it to /v1/services). The operator builds from git (Dockerfile\n# here), runs it, and returns a live https URL. A signed git push then\n# redeploys (POST /v1/webhooks/git). See docs/ADR006-bex-api.md.\nservices:\n  - name: hello-go\n    type: web\n    runtime: docker\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    branch: main\n    plan: free\n    healthCheckPath: /\n    envVars:\n      - key: MESSAGE\n        value: \"hello from bex\"\n"
    }
  },
  "freshPageRepeat": {
    "at": "2026-10-03T05:23:30.958Z",
    "status": 200,
    "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
  },
  "secondSeed": {
    "status": 200,
    "cacheControl": "public, max-age=31536000, immutable",
    "body": "# render.yaml — the manifest for deploy-from-chat (pillar 4).\n# Deploy in one call: give an agent the repo + this file via the MCP `deploy`\n# tool (or POST it to /v1/services). The operator builds from git (Dockerfile\n# here), runs it, and returns a live https URL. A signed git push then\n# redeploys (POST /v1/webhooks/git). See docs/ADR006-bex-api.md.\nservices:\n  - name: hello-go\n    type: web\n    runtime: docker\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    branch: main\n    plan: free\n    healthCheckPath: /\n    envVars:\n      - key: MESSAGE\n        value: \"hello from bex\"\n"
  },
  "secondProof": [
    {
      "at": "2026-10-03T05:24:30.961Z",
      "url": "/render.yaml?qa=r43-b",
      "cache": "default",
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=31536000, immutable",
        "content-length": "614",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 05:23:31 GMT"
      },
      "body": "# render.yaml — the manifest for deploy-from-chat (pillar 4).\n# Deploy in one call: give an agent the repo + this file via the MCP `deploy`\n# tool (or POST it to /v1/services). The operator builds from git (Dockerfile\n# here), runs it, and returns a live https URL. A signed git push then\n# redeploys (POST /v1/webhooks/git). See docs/ADR006-bex-api.md.\nservices:\n  - name: hello-go\n    type: web\n    runtime: docker\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    branch: main\n    plan: free\n    healthCheckPath: /\n    envVars:\n      - key: MESSAGE\n        value: \"hello from bex\"\n"
    },
    {
      "at": "2026-10-03T05:24:31.130Z",
      "url": "/render.yaml?qa=r43-b",
      "cache": "no-store",
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=31536000, immutable",
        "content-length": "795",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 05:24:31 GMT"
      },
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    },
    {
      "at": "2026-10-03T05:24:31.300Z",
      "url": "/",
      "cache": "default",
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=0, must-revalidate",
        "content-length": "1548",
        "content-type": "text/html",
        "date": "Sat, 03 Oct 2026 05:24:31 GMT"
      },
      "body": "<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"utf-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n    <title>bex static site</title>\n    <style>\n      *,\n      *::before,\n      *::after {\n        box-sizing: border-box;\n        margin: 0;\n        padding: 0;\n      }\n      body {\n        font-family: system-ui, sans-serif;\n        background: #0f172a;\n        color: #e2e8f0;\n        display: flex;\n        align-items: center;\n        justify-content: center;\n        min-height: 100vh;\n      }\n      .card {\n        text-align: center;\n        padding: 3rem 4rem;\n        border: 1px solid #1e293b;\n        border-radius: 1rem;\n        background: #1e293b;\n        max-width: 480px;\n      }\n      h1 {\n        font-size: 2rem;\n        font-weight: 700;\n        letter-spacing: -0.03em;\n        color: #f8fafc;\n      }\n      p {\n        margin-top: 0.75rem;\n        color: #94a3b8;\n        line-height: 1.6;\n      }\n      .badge {\n        display: inline-block;\n        margin-top: 1.5rem;\n        padding: 0.35rem 0.85rem;\n        border-radius: 9999px;\n        font-size: 0.8rem;\n        font-weight: 600;\n        background: #6366f1;\n        color: #fff;\n      }\n    </style>\n  </head>\n  <body>\n    <div class=\"card\">\n      <h1>Hello from bex</h1>\n      <p>\n        This static site is served by bex — no server, no Dockerfile. Edit\n        <code>index.html</code>, push to git, and bex redeploys instantly.\n      </p>\n      <span class=\"badge\">static_site</span>\n    </div>\n  </body>\n</html>\n"
    }
  ],
  "headerControl": [
    {
      "at": "2026-10-03T05:26:44.546Z",
      "url": "/render.yaml?qa=r43-control",
      "cache": "default",
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=0, must-revalidate",
        "content-length": "795",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 05:26:44 GMT"
      },
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    },
    {
      "at": "2026-10-03T05:26:44.547Z",
      "url": "/render.yaml?qa=r43-b",
      "cache": "default",
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=31536000, immutable",
        "content-length": "614",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 05:23:31 GMT"
      },
      "body": "# render.yaml — the manifest for deploy-from-chat (pillar 4).\n# Deploy in one call: give an agent the repo + this file via the MCP `deploy`\n# tool (or POST it to /v1/services). The operator builds from git (Dockerfile\n# here), runs it, and returns a live https URL. A signed git push then\n# redeploys (POST /v1/webhooks/git). See docs/ADR006-bex-api.md.\nservices:\n  - name: hello-go\n    type: web\n    runtime: docker\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    branch: main\n    plan: free\n    healthCheckPath: /\n    envVars:\n      - key: MESSAGE\n        value: \"hello from bex\"\n"
    },
    {
      "at": "2026-10-03T05:26:44.717Z",
      "url": "/render.yaml",
      "cache": "no-store",
      "status": 200,
      "headers": {
        "cache-control": "public, max-age=0, must-revalidate",
        "content-length": "795",
        "content-type": "binary/octet-stream",
        "date": "Sat, 03 Oct 2026 05:26:44 GMT"
      },
      "body": "# render.yaml — static_site example: no Dockerfile and no build command needed —\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
    }
  ],
  "controlAfter": {
    "at": "2026-10-03T05:27:40.925Z",
    "url": "/render.yaml?qa=r43-control",
    "cache": "default",
    "status": 200,
    "headers": {
      "cache-control": "public, max-age=0, must-revalidate",
      "content-length": "614",
      "content-type": "binary/octet-stream",
      "date": "Sat, 03 Oct 2026 05:27:40 GMT"
    },
    "body": "# render.yaml — the manifest for deploy-from-chat (pillar 4).\n# Deploy in one call: give an agent the repo + this file via the MCP `deploy`\n# tool (or POST it to /v1/services). The operator builds from git (Dockerfile\n# here), runs it, and returns a live https URL. A signed git push then\n# redeploys (POST /v1/webhooks/git). See docs/ADR006-bex-api.md.\nservices:\n  - name: hello-go\n    type: web\n    runtime: docker\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    branch: main\n    plan: free\n    healthCheckPath: /\n    envVars:\n      - key: MESSAGE\n        value: \"hello from bex\"\n"
  },
  "deletedRead": {
    "status": 404,
    "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
  }
}
```

Initial independent curl observations (complete bodies):

```json
[
  {
    "method": "GET",
    "path": "/",
    "requestHeaders": [],
    "exit": 0,
    "responseRaw": "HTTP/2 200 \r\ncache-control: public, max-age=0, must-revalidate\r\ncontent-type: text/html\r\ndate: Sat, 03 Oct 2026 05:20:40 GMT\r\ncontent-length: 1548\r\n\r\n<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"utf-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n    <title>bex static site</title>\n    <style>\n      *,\n      *::before,\n      *::after {\n        box-sizing: border-box;\n        margin: 0;\n        padding: 0;\n      }\n      body {\n        font-family: system-ui, sans-serif;\n        background: #0f172a;\n        color: #e2e8f0;\n        display: flex;\n        align-items: center;\n        justify-content: center;\n        min-height: 100vh;\n      }\n      .card {\n        text-align: center;\n        padding: 3rem 4rem;\n        border: 1px solid #1e293b;\n        border-radius: 1rem;\n        background: #1e293b;\n        max-width: 480px;\n      }\n      h1 {\n        font-size: 2rem;\n        font-weight: 700;\n        letter-spacing: -0.03em;\n        color: #f8fafc;\n      }\n      p {\n        margin-top: 0.75rem;\n        color: #94a3b8;\n        line-height: 1.6;\n      }\n      .badge {\n        display: inline-block;\n        margin-top: 1.5rem;\n        padding: 0.35rem 0.85rem;\n        border-radius: 9999px;\n        font-size: 0.8rem;\n        font-weight: 600;\n        background: #6366f1;\n        color: #fff;\n      }\n    </style>\n  </head>\n  <body>\n    <div class=\"card\">\n      <h1>Hello from bex</h1>\n      <p>\n        This static site is served by bex \u2014 no server, no Dockerfile. Edit\n        <code>index.html</code>, push to git, and bex redeploys instantly.\n      </p>\n      <span class=\"badge\">static_site</span>\n    </div>\n  </body>\n</html>\n"
  },
  {
    "method": "HEAD",
    "path": "/",
    "requestHeaders": [],
    "exit": 0,
    "responseRaw": "HTTP/2 200 \r\nHTTP/2 200 \r\ncache-control: public, max-age=0, must-revalidate\r\ncache-control: public, max-age=0, must-revalidate\r\ncontent-type: text/html\r\ncontent-type: text/html\r\ndate: Sat, 03 Oct 2026 05:20:41 GMT\r\ndate: Sat, 03 Oct 2026 05:20:41 GMT\r\n\r\n\r\n"
  },
  {
    "method": "GET",
    "path": "/render.yaml",
    "requestHeaders": [],
    "exit": 0,
    "responseRaw": "HTTP/2 200 \r\ncache-control: public, max-age=31536000, immutable\r\ncontent-type: binary/octet-stream\r\ndate: Sat, 03 Oct 2026 05:20:41 GMT\r\ncontent-length: 795\r\n\r\n# render.yaml \u2014 static_site example: no Dockerfile and no build command needed \u2014\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
  },
  {
    "method": "HEAD",
    "path": "/render.yaml",
    "requestHeaders": [],
    "exit": 0,
    "responseRaw": "HTTP/2 200 \r\nHTTP/2 200 \r\ncache-control: public, max-age=31536000, immutable\r\ncache-control: public, max-age=31536000, immutable\r\ncontent-type: binary/octet-stream\r\ncontent-type: binary/octet-stream\r\ndate: Sat, 03 Oct 2026 05:20:42 GMT\r\ndate: Sat, 03 Oct 2026 05:20:42 GMT\r\n\r\n\r\n"
  },
  {
    "method": "GET",
    "path": "/render.yaml",
    "requestHeaders": ["Range: bytes=0-9"],
    "exit": 0,
    "responseRaw": "HTTP/2 200 \r\ncache-control: public, max-age=31536000, immutable\r\ncontent-type: binary/octet-stream\r\ndate: Sat, 03 Oct 2026 05:20:43 GMT\r\ncontent-length: 795\r\n\r\n# render.yaml \u2014 static_site example: no Dockerfile and no build command needed \u2014\n# bex clones the repo and publishes rootDir/staticPublishPath as-is to object storage\n# (w9/010, Render parity), serving it through the shared static-server at a\n# https URL (docs/ADR029-static-sites.md). Declaring a dockerfilePath, a\n# buildCommand, or a runtime instead opts into the build path: the image's\n# staticPublishPath directory then holds the built site.\n# A git push redeploys automatically via the push webhook.\nservices:\n  - name: static-site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex # replace with your fork/checkout\n    rootDir: examples/static-site\n    branch: main\n    staticPublishPath: . # publish this directory as-is; set to \"dist\" for a real build step\n"
  },
  {
    "method": "GET",
    "path": "/absent.js",
    "requestHeaders": [],
    "exit": 0,
    "responseRaw": "HTTP/2 404 \r\ncontent-type: text/plain; charset=utf-8\r\ndate: Sat, 03 Oct 2026 05:20:43 GMT\r\nx-content-type-options: nosniff\r\ncontent-length: 10\r\n\r\nnot found\n"
  },
  {
    "method": "GET",
    "path": "/dashboard",
    "requestHeaders": [],
    "exit": 0,
    "responseRaw": "HTTP/2 200 \r\ncache-control: public, max-age=0, must-revalidate\r\ncontent-type: text/html\r\ndate: Sat, 03 Oct 2026 05:20:44 GMT\r\ncontent-length: 1548\r\n\r\n<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"utf-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n    <title>bex static site</title>\n    <style>\n      *,\n      *::before,\n      *::after {\n        box-sizing: border-box;\n        margin: 0;\n        padding: 0;\n      }\n      body {\n        font-family: system-ui, sans-serif;\n        background: #0f172a;\n        color: #e2e8f0;\n        display: flex;\n        align-items: center;\n        justify-content: center;\n        min-height: 100vh;\n      }\n      .card {\n        text-align: center;\n        padding: 3rem 4rem;\n        border: 1px solid #1e293b;\n        border-radius: 1rem;\n        background: #1e293b;\n        max-width: 480px;\n      }\n      h1 {\n        font-size: 2rem;\n        font-weight: 700;\n        letter-spacing: -0.03em;\n        color: #f8fafc;\n      }\n      p {\n        margin-top: 0.75rem;\n        color: #94a3b8;\n        line-height: 1.6;\n      }\n      .badge {\n        display: inline-block;\n        margin-top: 1.5rem;\n        padding: 0.35rem 0.85rem;\n        border-radius: 9999px;\n        font-size: 0.8rem;\n        font-weight: 600;\n        background: #6366f1;\n        color: #fff;\n      }\n    </style>\n  </head>\n  <body>\n    <div class=\"card\">\n      <h1>Hello from bex</h1>\n      <p>\n        This static site is served by bex \u2014 no server, no Dockerfile. Edit\n        <code>index.html</code>, push to git, and bex redeploys instantly.\n      </p>\n      <span class=\"badge\">static_site</span>\n    </div>\n  </body>\n</html>\n"
  }
]
```

## Cleanup and verification status

UI deletion succeeded; REST GET returned 404 with the full body in the captures. Independent curl public root returned 404. Final exact-name/owner-UID tenant App, Deployment, ReplicaSet, Job, Pod, Service, Ingress and Secret inventory was empty; bex-build exact-name Job/Pod/Secret inventory was empty. The App finalizer completed; no manual finalizer removal or shared resource deletion. QA logout returned `ok logged-out`, visitor tab closed and browser cookies cleared. Console captured only the deliberate final API 404; no unexpected 4xx/5xx from the six saved mutations or six direct API read probes. Network history includes prior sweeps; it is not claimed as a per-sweep event count.
