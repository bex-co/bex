# CLI sweep 523 — cross-service deploy-detail non-enumeration

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:27Z. One owned free fixture created for a real deploy ID and deleted after; the second service named in probes is a pre-existing baseline service that was only ever read.

| probe (`GET /v1/services/{svc}/deploys/{dep}`) | observed result |
| --- | --- |
| Own service + its own deploy (control) | `200` with the deploy record. |
| Baseline service + fixture's deploy ID | `404 {"code":"NOT_FOUND",…,"message":"deploy not found"}`. |
| Fixture service + baseline's deploy ID | Byte-identical `404` body. |
| Baseline service + synthetic `dep-000…` | Byte-identical `404` body — a cross-service ID is indistinguishable from a nonexistent one; no existence oracle. |
| After deleting the fixture: its own pair | `404` (service-level not-found). |

The pinned CLI has no deploy-detail command (`deploys list` only), so the cross-service legs ran over REST with the same QA credential as diagnosis of the same server boundary the CLI's list/cancel verbs sit on — not counted as CLI journey coverage.

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbau4jaaps2c7393hep0` deleted by ID; its deploy read then 404s. No cancel/rollback mutations were attempted against any pre-existing deploy.
- Unrun: cross-*workspace* deploy IDs (hunt rules keep probes inside the selected workspace; the same store scoping produces this 404), GraphQL/MCP deploy-by-ID equivalents.
- Deploy lag means deployed revisions can precede local `main`.
