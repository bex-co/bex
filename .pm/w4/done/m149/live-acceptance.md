# m149 live acceptance — 2026-10-02 (UTC 2026-10-03T06:17–06:22Z)

**Deployed image:** `ghcr.io/bex-co/bex-operator@sha256:e396cf3585ac250a25f2d947e7b2b8f309def52e7bb15cb7826d92078a4f27d9` on `bex-controller-manager` and `bex-api` (GitOps pin `20fb64867` → `1263d12ae`, which contains the fix commits `93ee9b66e` and `81056258f` and the requested `18958459c`). **Workspace/namespace:** `tea-d98210cbbpdc73dcrkvg`. **Session:** fresh `scripts/qa-login.sh` session in an isolated temp directory, revoked at the end.

**Transport note:** the Playwright browser was in use by a parallel agent, so creates and deletes were sent as the exact GraphQL mutations the dashboard issues (`createKeyValue`; `deleteKeyValue(id)`, the `useDeleteKeyValue` hook's mutation) from curl with the signed-in session, not by clicking. The dashboard delete components are unchanged by this milestone and their suites pass (verification.md).

## Public Free fixture — `qa-20261003-kv-tls-pub` / `red-db09rrqtm2ss7389qmk0`

- Created 06:17:18Z (`plan: free, public: true`); GraphQL `status: available` at 06:18:36Z.
- Before delete (06:18:10Z): KeyValue finalizers `["app.bex.co/kv-tls-cleanup"]`, phase Ready; Certificate `red-db09rrqtm2ss7389qmk0-kv-tls` Ready=True; CertificateRequest `-kv-tls-1`; StatefulSet, pod, Service, PVC `data-…-0`; Secrets `red-db09rrqtm2ss7389qmk0` (owner KeyValue), `-auth` (owner KeyValue), and **`-kv-tls` UID `d33030da-44c1-41b1-9d1a-e608119cdf1e`** (`kubernetes.io/tls`, no owner reference).
- `deleteKeyValue` at 06:18:36Z → `{"data":{"deleteKeyValue":true}}`. In the same second, Certificate, CertificateRequest and the TLS Secret were already gone. By 06:19:33Z the full inventory was empty: no KeyValue/Certificate/CertificateRequest/StatefulSet/pod/Service/PVC, no Secret matching the ID, `secrets "red-db09rrqtm2ss7389qmk0-kv-tls" not found`, and **0** Secrets in the namespace carry UID `d33030da-…`. No manual Secret deletion.

## Private Free control — `qa-20261003-kv-tls-priv` / `red-db09rroehcmc739j11v0`

- Created 06:17:19Z (`public: false`); available; carried the same `kv-tls-cleanup` finalizer but never issued a Certificate/TLS Secret.
- `deleteKeyValue` at 06:20:18Z → `true`. At 06:20:59Z the inventory was empty (CR, workload, PVC, both owned Secrets). The TLS obligation did not strand it.

## Read shapes after each delete (unchanged)

| Surface | Public | Private |
| --- | --- | --- |
| `GET /v1/key-value/{id}` | HTTP 404 `{"error":"not found","id":"not_found","message":"not found"}` | same |
| GraphQL `keyValue(id)` | `{"data":{"keyValue":null},"errors":[{"message":"not found",…,"path":["keyValue"]}]}` | same |
| MCP `tools/call get_key_value` | `{"result":{"content":[{"type":"text","text":"not found"}],"isError":true}}` | same |
| GraphQL `keyValues` (Overview list) | omits fixture | omits fixture |

## Corrected audit

`scripts/delete-audit.sh --kv <id>` run with the `.env` loader removed, kubectl bound to `hetzner-prod`/20s, `APPS_NS=tea-d98210cbbpdc73dcrkvg`, backup endpoint/destination unset: **14 passed, 0 failed, exit 0** for both IDs, including `KeyValue TLS certificates.cert-manager.io …-kv-tls is absent`, `KeyValue TLS secret …-kv-tls is absent`, and `no CertificateRequests`; the S3 check is reported `SKIP … prerequisite not configured`. The failing side stays proven by `bash scripts/delete-audit.test.sh` (re-run today, all cases PASS, including Secret-only, Certificate-only and request-only residue returning non-zero).

## Cleanup

Both fixtures were deleted by the probes above and converged to zero. The verification session was revoked with `qa-login.sh --logout` on its own jar. Not exercised live (covered by local tests only, per verification.md): paid/backup-enabled, public→private before delete, interrupted issuance, manager restart, row-action/REST-delete/purge entrypoints.
