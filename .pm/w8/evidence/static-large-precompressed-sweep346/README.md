# Large authored gzip and automatic Brotli — sweep346

[Results](results.json) · [complete captured native create, HTTP/API/native readbacks and cleanup](native-HTTP-API-cleanup.json) · [source tests](source-events.jsonl) · [local exact-build execution](local-build-preflight.json).

Source c84c348b0e4a2248ee0f80a09cc15165275e14d7 follows a scoped main refresh with no newer changes. Serving revision is unobserved. One fresh published-Bex0.3.2/a6350da0 native Free static fixture uses public Express/serve-static, explicit master, Node24, autoDeploy off and a dependency-free authored build. Its actual unchanged POST is captured through an owned loopback relay: HTTP201/native0 and exact name/ID/workspace/Free prove ownership. The relay and copied config are removed. Native inventory reads through Bex and unchanged Render2.27.0/a764810a7682 both find the exact own fixture. No authenticated Render account is used.

The exact JavaScript is syntax/execution-checked in a private local directory before create; Python independently generates the expected payload. The hosted first deployment reaches Live/HTTP200 with the exact nonce, without recovery or redeploy. Its build produces168874-byte plain text, a79687-byte gzip copy, a no-transform copy and an artifact hash/size manifest. The saved gzip is substantially above the automatic1KiB floor and below8MiB, closing the explicit large-precompressed live gap in [m79 sweep340](../../done/m79/evidence/cli-sweep340/README.md), whose gzip body was559bytes.

## Thirteen actual HTTP comparisons

- Artifact manifest matches independently generated identity bytes/hash and proves the saved compressed size is above the threshold. Identity GET is exact; a plain-text br GET actually returns Brotli, independently decoded to the same bytes.
- Three full saved-asset GETs offering br+gzip, gzip, and disabled br+enabled gzip all return the original gzip bytes/hash/length and decode exactly. There is no second compression layer. The authored strong ETag is retained.
- HEAD is bodyless and retains encoding/ETag. Content-Length, when supplied, is checked against the saved compressed size; its presence is recorded in results, not assumed as a mandatory HEAD field. Range returns the first32 compressed bytes with the compressed total. If-None-Match returns bodyless304; If-Match succeeds with full original gzip; If-Range returns the same encoded byte range.
- The large no-transform asset stays identity/byte-exact despite offering br. A malformed br weight receives exact identity for the plain asset.

Header creation is API-only: three ordinary rules author gzip, no-transform and a strong ETag. Exact PUT200/GET200 comparison passes; the CLI exposes no header setter. The first HEAD while the rules propagate does not yet contain them; the next bounded readiness read does. Only then are the13 representation checks run. Raw public bodies are hashed and independently decoded; they are reproducible from the exact authored build and nonce. All relevant response headers and HTTP statuses are retained. No browser or native HTTP-client coverage is invented.

The existing TestBrotliIneligibleResponsesStayExact, TestBrotliPreconditionsAndHead and TestBrotliFramingOverHTTP execute unchanged and pass3/3, no skips/failures. The precompressed maintained fixture is smaller; this hosted case supplies the larger boundary evidence. Source staticserver.go:366–407 applies authored headers before choosing compression; encoding.go:77–82 rejects an authored Content-Encoding, and the fixed gzip therefore retains its validator and byte representation. No product implementation/test change or full operator suite is claimed.

## Cleanup and limits

Native Bex delete exits0. Own detail/former public URL and recorded first deployment return404; native inventory contains exactly the immutable original6 services, services used6/terminating0 and PostgreSQL/KeyValue usage0. No Blueprint or other dependent resource is created. No paid fixture, purge/CDN work or external source edit occurs. The isolated QA login remains for the authorized continuing loop. Baseline configurations are projected out and credentials masked before filing.

This is a passing focused ordinary hosting journey, not a new defect or a rewrite of m79's full acceptance. Paid services, other precompression formats, concurrent cache pressure, objects above the8MiB ceiling, cross-revision changes and physical cluster artifact teardown remain unexercised here. [Finite native journey](finite-native-journey.py.txt), [capture/client helpers](isolated-native-capture-helpers.py.txt), [HTTP driver](HTTP-representation-driver.py.txt). Uncommitted.
