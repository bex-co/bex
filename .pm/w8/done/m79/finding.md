# Native static sites omit Render's automatic Brotli compression

Why: ordinary static text, JavaScript and CSS transfer at their full size after a successful native deployment even when the visitor offers Brotli, increasing transfer bytes compared with Render's documented static-site behavior.

- **Severity:** minor; functional Render compatibility/performance gap. All content is correct; no outage or data loss is claimed.
- **Versions/context:** published Bex0.3.2/a6350da0, unchanged Render2.27.0/a764810a7682 (module v1.1.3-0.20260909214233-a764810a7682); source 82ed0717b60ff156c77604050fed2188f8f10df0; serving SHA unobserved.2026-10-15 UTC, api.bex.co/v1, bex-canary/tea-daif693dqjvc73e7as3g, isolated human QA credential, noninteractive JSON. Two separate explicit Free static fixtures; no baseline mutation or paid resource.
- **Contract:** [Render static sites](https://render.com/docs/static-sites#brotli-compression) document automatic Brotli. This is a parity finding, not a claim that [HTTP Accept-Encoding](https://www.rfc-editor.org/rfc/rfc9110.html#section-12.5.3) requires compression: an identity response is protocol-valid for these requests. No Render production fixture was created.

## Reproduction and live results

Run the exact native command and generated build text in [native receipts](evidence/cli-sweep326/native-http-cleanup.json) or the [guarded driver](evidence/cli-sweep326/native-driver.py.txt), with an isolated QA config:

```sh
bex services create --name qa-<date>-<nonce>-bex-enc --type static_site \
  --repo https://github.com/expressjs/serve-static --branch master \
  --build-command '<retained Node asset generator>' --publish-directory public \
  --env-var NODE_VERSION=24 --env-var SKIP_INSTALL_DEPS=true \
  --auto-deploy=false --confirm -o json
bex deploys list <owned-id> --confirm -o json
curl --disable -sS -D headers.txt -o body.bin -H 'Accept-Encoding: br' \
  https://<owned-site>.onbex.co/app.js
```

Repeat the equivalent create with unmodified same-pin Render against Bex and a fresh name. Both actual unchanged POST /v1/services requests return201/native exit0; both first deployments reach Live and return the exact index marker. Each site serves three compressible generated files with identity, br, gzip and weighted br/gzip offers. All24 content comparisons pass. All24 omit Content-Encoding and send the full identity representation:

| Asset | Content size per response | br offered | Observed coding |
| --- | --: | --- | --- |
| payload.txt | 122880 bytes | both clients, bare and weighted | identity |
| app.js | 221184 bytes | both clients, bare and weighted | identity |
| app.css | 241664 bytes | both clients, bare and weighted | identity |

Expected for eligible assets: correctly negotiated Brotli, smaller encoded body that decodes exactly, accurate framing/variant semantics, and identity when requested. Full redacted native wire/response records and HTTP headers/hashes are durable. The large repeated bodies are deterministic from the retained generator; they are not saved verbatim24 times. Requests are plain curl GETs against the newly returned static URLs; they are runtime HTTP evidence, not a native CLI compression flag.

## Root cause and blast radius

Attribution: runtime/static-server. Both clients reproduce after successful builds; source confirms the omission rather than a native request mismatch.

- [staticserver.go:344](../../../../lego/operator/internal/staticserver/staticserver.go) serveObject writes Content-Type, Cache-Control and authored headers, then passes the original obj.Body to http.ServeContent at372 without negotiating Accept-Encoding. Its **three** callers at262/311/321 cover direct files/directory index, rewritten targets and SPA fallback. There is no automatic encoder in this package.
- [s3origin.go:91](../../../../lego/operator/internal/staticserver/s3origin.go) Get reads the published object into Body/ContentType; [cache.go:53](../../../../lego/operator/internal/staticserver/cache.go) counts the immutable object's body in its shared byte budget. Encoding variants must retain bounded cache/in-flight memory accounting and revision identity.
- [cmd/staticserver/main.go:73](../../../../lego/operator/cmd/staticserver/main.go) newServer mounts the site directly. [app_controller.go:2904](../../../../lego/operator/internal/controller/app_controller.go) assembles allow-list/websocket-meter middleware, with no automatic compression. Prod/base Traefik values contain no compression configuration in this checkout; serving deployment configuration was not inspected through admin access.
- Backend [api/gzip.go:36](../../../../lego/backend/internal/api/gzip.go) is for bex-api reads, not this static-server. The operator must not import backend; respect operator→types←backend. No CLI fork or public API field is needed.
- Static headers intentionally permit authored Content-Encoding for precompressed assets ([w4/204](../../../w4/done/204.md)); preserve that body exactly rather than compressing it twice. Existing reserved framing-header protections remain.

The actual Handler overlay has **three failing Brotli subcases, four passing controls and an expected failing parent**. Six maintained neighboring top-level tests and all named subcases pass. [Events and source](evidence/cli-sweep326/README.md) preserve the exact limits. No implementation/fixed candidate or full module run is claimed.

## Proposed fix and acceptance

Implement bounded static response negotiation for compressible eligible objects, supplying Brotli when accepted and useful. Preserve identity and q=0, MIME eligibility, existing Vary fields, Cache-Control:no-transform, authored Content-Encoding, no-body statuses, HEAD and byte-range/precondition correctness. Use encoding-aware validators when needed; never label identity bytes as br or gzip. Bound memory and cache variants within the existing shared budgets; avoid per-request unbounded CPU/work. Preserve publication-prefix cache isolation and atomic deployment/rollback behavior. Gzip fallback may be considered within the same negotiation design, but missing gzip alone is not this finding.

Regression checks must exercise actual encoding/decoding and mixed identity/br requests through the Handler, not only Dockerfile/config text. Retain direct/index/rewrite/SPA, existing header rules, unknown-host/missing/error paths, conditional304/412,206/416, HEAD and cross-revision/cache-neighbor behavior. Re-run both published/current Bex and unchanged pinned Render create→Live→HTTP→delete journeys on fresh owned Free static fixtures. Record installed/check-out labels accurately.

REST/GraphQL/MCP/dashboard all configure the same static publication; no new schema is needed. Audit all exposed header/rule writers and built/no-build publication routes, and verify the same served behavior from each entry point. Any live paths not exercised must be explicit acceptance limits, rather than inferred parity.

## Dedupe, exclusions and cleanup

Searches across open/blocked/done .pm use Brotli, compression, Accept-Encoding, Content-Encoding, static serving and cache symbols. [w9/m61](../../../w9/done/m61/README.md) and [w9/044](../../../w9/done/044.md) cover API gzip only. [w4/204](../../../w4/done/204.md) covers authored framing headers, not automatic encoding. [sweep317](../../evidence/static-ranges-sweep317/README.md) accepts byte ranges and explicitly excludes compression. No matching automatic-static-Brotli implementation is found in targeted source/history; this is not a duplicate rollout gate. DO_NOT_DO's static CDN purge remains excluded; no CDN deployment, purge API, sandbox, security work, paid plan or platform configuration mutation is proposed.

Both native deletes exit0 after exact ownership and child enumeration. Later detail/list/runtime/child absence, original six services present and quota terminating0 prove cleanup. Initial terminating2 receipts remain. Source probe/debug helpers and setup-only local script errors are harness evidence, not extra product findings. The hunt remains active with only its isolated auth retained.

Unverified: real Render requests, browser load times, production transfer costs, concurrent/load behavior, direct GraphQL/MCP/UI create, no-build static publication, rollback and serving SHA. **Estimate:**170m across one implementation task and standing parity/simplify/test/closeout tasks. This filing schedules work; it does not implement or ship a fix.
