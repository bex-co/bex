# m50 acceptance — 2026-10-02

The Postgres capacity policy is enforced across shared create/update/preview/plan paths, Blueprint merged-state admission, and server-driven dashboard controls. Free storage stays fixed at 1 GB. Replicas require at least 0.5 CPU and 10 GB effective storage, with at most five. The existing basic-1gb tier supplies the paid control; HA remains a separate 1-CPU gate. No plan or price was added.

## Verification

- Full backend `GOWORK=off go test -p 1 ./...`: pass with dedicated real PostgreSQL, OpenFGA and OpenBao. Unchanged packages use the Go test cache. The apps package passed after updating older replica fixtures and pricing expectations; its staging-gate cleanup test also passes.
- Full operator `GOWORK=off GOFLAGS=-p=1 make test`: pass, including code generation and real envtest. Controller package: 88.865 seconds. Tests cover free storage preservation, eligible one/five readers, independent HA standby, legacy instance preservation, explicit reader removal, readiness, and aggregate storage quota during pending-reader and eligibility transitions.
- `GOFLAGS=-p=1 make lint`: all four Go modules and whole-program dead-code checks pass.
- Dashboard full suite: 457 files / 3,866 tests pass. Latest focused suite: 6 files / 47 tests pass, including two later boundary tests. Typecheck, ESLint, Knip and offline GraphQL codegen pass; generated outputs were reconciled with the backend schema.
- Reuse, quality and efficiency reviews ran on the full change and the reader extension. Existing GraphQL/Blueprint helpers replaced duplication, and the staged-sync test now cleans up its gate even on failure.

The first backend run exposed a shared success fixture using basic-256mb/5GB with a replica. One staging concurrency test waited indefinitely after that validation refusal; only its test process was interrupted for a stack trace. Updated eligible fixtures, correct 10GB pricing expectations and useful early-failure handling resolved the failures. The final full backend run is green. No check was waived.

## Authenticated API and CLI proof

[Final matrix](evidence/checks.jsonl): **59/59** unique admission controls pass on the fresh dev-8 API through REST, GraphQL, MCP, Blueprint, current Bex and unmodified same-pin Render 2.27.0. Refusals preserve database specs and inventory. Paid feature previews, capability metadata, disable controls and paid replica declaration/downgrade controls pass.

[All attempts](evidence/all-attempts.jsonl) retain 63 rows: four earlier harness/quota failures are explained in [the transient record](evidence/harness-transients.jsonl), followed by passing corrected controls. They were a too-long fixture name, the REST validation error envelope, a CLI response wrapper and the local one-Postgres quota. The free fixture was removed before the paid fixture was created.

API SHA256: `0db9744d37d1a43b63c8be54ced4208dcd05465ae407acd2b57155cb2d85e8d6`. [Build provenance](evidence/final-api-provenance.json) records the exact production Go and both CLI hashes. All recorded API source hashes match; a later tiers.yaml comment correction changes no catalog value.

## Reader mechanism proof

The initial API-created paid fixture exposed an existing gap: the old shared operator requested one primary and published reader aliases with no ready reader. [That observation is retained](evidence/existing-operator-runtime-observation.json). Task t009 fixes the projection, readiness and storage-quota calculation.

The isolated [production-generated CNPG projection](evidence/reader-runtime-projection.json), basic-1gb/10GB/one reader with HA off, reached **2/2 Ready**. One marker row inserted through the primary service was read through the `-ro` service: primary `f|off`, reader **`t|on|42`**. This proves a running standby, read-only mode and replicated data. [Runtime evidence](evidence/reader-runtime-evidence.jsonl) and [source provenance](evidence/reader-runtime-provenance.json) tie the object to the tested projection. Names retain the existing shared reader-pool model rather than isolated per-name clusters.

The new Bex operator was not rolled out to the shared development cluster. Envtest covers its reconcile/readiness/quota path; the live check applies the exact generated projection as a separately named CNPG Cluster without a Database owner. The existing local `hcloud-volumes` class uses local-path storage, so this does not verify Hetzner CSI. No large-storage or five-reader live write was needed.

## Cleanup and limits

All **9/9 tenant cleanup checks** pass. The standalone Cluster, both pods/PVCs, generated children and both owned PVs are verified absent. A narrowly scoped temporary tenant NetworkPolicy was needed for CNPG access to the current Kubernetes API and was removed with its namespace. [Cleanup evidence](evidence/cleanup-evidence.jsonl) and the runtime record identify exact resources.

Base dev-8 and three dedicated test services remain available; their status is [recorded separately](evidence/retained-infrastructure.json). No production or shared operator was changed. Dashboard behavior is covered by UI tests and live GraphQL metadata, without a browser session. GraphQL exposes no dedicated disk-size/pooler update mutation; this work does not introduce new replica/pooler authoring UI.

The [hash index](evidence/SHA256SUMS.json) covers every evidence payload. Agent sanitization scanned five known private values; root independently checked four recorded credentials, all 20 imported artifact hashes, API source hashes and final operator source hashes. No credential match was found. [Automated verification record](evidence/verification.json) preserves commands/log hashes and limitations.
