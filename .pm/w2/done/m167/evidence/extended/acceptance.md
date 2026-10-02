# m167 local-process acceptance — log text arrays

On 2026-10-02, 54 assertions passed against the modified composed backend API and an actual isolated Loki 3.6.1 instance. The installed Bex v0.2.1 launcher and unchanged Render v2.27.0 binary both returned `qa_text_A first marker` and `qa_text_B second marker` exactly once for either text-term order, excluding `qa_control unrelated marker`.

The Render binary matches the previously recorded SHA-256 `35950c3439f70fbe05323fbd04ddb7cad0448dd43215ac9b4125e986e08a2e90`; its upstream pin is `a764810a768202704e7206eb7b87a47211fcd98e`. Bex SHA-256 is `ea9e5e2c92a1094030e7537765092b633c7ec8793a076eeb55a136f6157cf8d0`. `versions.json` records the versions, image ID, and API harness hash. `source-manifest.json` records the six production source hashes, all unchanged after the final API build.

Commands used the owned resource ID and explicit start/end bounds, with the local endpoint and private test credential supplied through isolated environment/config settings:

```text
bex logs -r <owned-srv-id> --text qa_text_A,qa_text_B --type app --start <start> --end <end> --limit 100 -o json
bex logs -r <owned-srv-id> --text qa_text_B,qa_text_A --type app --start <start> --end <end> --limit 100 -o json
render logs -r <owned-srv-id> --text qa_text_A,qa_text_B --type app --start <start> --end <end> --limit 100 -o json
render logs -r <owned-srv-id> --text qa_text_B,qa_text_A --type app --start <start> --end <end> --limit 100 -o json
```

`evidence.jsonl` contains exact sanitized arguments, requests, response rows, and assertions. The passing controls cover:

- Both CLI binaries: both term orders, case-insensitive scalar search, and text OR combined with a level filter using AND.
- REST repeated `text` parameters and MCP `list_logs` arrays: both orders, empty entries, case duplicates, overlapping terms without duplicate rows, all-empty arrays, no-match results, literal regex metacharacters, preserved whitespace, level conjunction, and multiple resources.
- GraphQL's existing scalar `String` text input: case-insensitive search returns the expected single line.
- REST and MCP cursor continuation in both directions: two resources share boundary timestamps; limit 1 preserves each cross-resource timestamp group, all four matching rows appear once across the pages, and the final page is empty.
- An explicit time-bound conjunction, missing-token 401, and an unknown-resource 404.
- Label discovery retains its stream-index limitation: unmatched line text does not remove otherwise valid level values. Unsupported tail level filtering returns 400 before opening a stream.

An owned HTTP fixture process served `/health`, accepted `/emit`, wrote ten actual stdout records, and pushed those exact records to Loki with HTTP 204. An independent unfiltered Loki query and the captured stdout both matched the complete dataset. The API used the production `NewLokiSource` and composed REST/GraphQL/MCP handler; no line matching was stubbed.

This is local-process acceptance, with two seeded App metadata/status records in an isolated real envtest API server and a fixed local identity introspection fixture. Loki records were ingested directly from the fixture emitter rather than through a Kubernetes shipper. No operator deployment, customer OAuth ceremony, control-plane database, or OpenFGA integration was exercised. Pod fallback, live app/build/predeploy streams, managed datastore paths, and exhausting the broad-search time budget remain covered by the separate repository regression tests, not by this runtime evidence.

Two harness-only corrections occurred before the final run: the CLI emits concatenated JSON objects rather than one JSON document, and one restart raced the API rebuild. After correcting the parser and waiting for the completed build, the entire consumer matrix passed with the original owned Loki dataset. The original first CLI response already contained both requested lines.

Cleanup removed both owned App CRs, requested deletion of their private namespace, stopped the composed API, its envtest control plane, and the HTTP emitter, and removed only `bex-w2-m167-live-kfnuzajf-loki` after verifying its ownership label. Its data lived in container tmpfs. All owned ports were closed; the private token, kubeconfig, and both CLI config directories were removed. `cleanup.json` records the checks. No shared cluster, other container, production resource, or repository file was changed by this harness. `sanitization.json` verifies the exported files contain neither the private credential nor kubeconfig/private-key material.
