# CLI sweep 544 — events of a deleted service (REST diagnosis)

**Verdict: no bug filed.** Closes the unrun leg left by sweep 543. Same isolated session, 2026-10-19T11:00Z. The pinned CLI has no events command, so these are same-credential REST reads of the boundary the CLI sits on — not CLI journey coverage. Nothing created or mutated (the deleted ID is sweep 543's already-deleted fixture; the live ID is a read-only baseline selector).

| probe | observed result |
| --- | --- |
| `GET /v1/services/<deleted-id>/events` | `404` with the plain `NOT_FOUND` body. |
| `GET /v1/services/<synthetic-id>/events` | Byte-identical `404` body — deleted vs never-existed stays indistinguishable. |
| `GET /v1/services/<live baseline>/events` (control) | `200` with cursor-paged events. |

Board movement since the last sweep: this loop's w8/131 note and evidence through sweep 542 were committed by a concurrent ship (`f5bef4285`), and a worker's w8/131 implementation (predeploy classifier + tests) is in progress uncommitted in this tree — untouched by this loop. The deployed image pin has not advanced past `8e57d4eaa121`, so w8/129 and w8/130 live verifications remain queued behind it; at this point the uncovered-surface backlog at the current pin is effectively exhausted, and the loop's remaining high-value work is those live flips plus genuinely new variants as they appear.
