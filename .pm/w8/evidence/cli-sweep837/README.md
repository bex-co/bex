# CLI sweep 837 — registry-credential GraphQL control re-run for w8/135 (live, 2026-10-20)

Disposable credential created via GraphQL, read/deleted via REST, verified absent in its home workspace (GET 404, list `[]`). No new bug filed; [w8/135](../../135.md) remains the open REST-create/PATCH defect.

- GraphQL `createRegistryCredential(name, host: "ghcr.io", username, authToken)` → `rgc-…` with no secret echoed; REST GET by id → 200 with the same fields and no secret; REST DELETE → 204.
- Workspace note (documented, sweep 743): the GraphQL mutation landed the credential in the caller's **default** workspace (`tea-d982…`, "bex"), not canary — the harness's `X-Bex-Workspace` header does not rescope GraphQL, and owner resolution follows the caller's default workspace. The canary-scoped REST list therefore did not contain it; deletion was verified against the home workspace, not just canary.
