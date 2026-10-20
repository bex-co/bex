# CLI sweep 626 — `generateValue` in env groups: shared minted value, rotation on re-PUT

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture: env group `qa626-group` (`evg-dbbapcbjauss73bn3k3g`, `ownerId` explicit per the sweep-585 lesson), deleted at close (204, then 404). No service was linked; generated values are recorded only by shape, never verbatim.

- **Group create honors `generateValue`:** creating the group with `{key: G626, generateValue: true}` minted a 44-character base64 value, revealable via the per-key GET — same minting shape as service vars (sweep 625). Bulk list responses show only `{key}` entries; the value is per-key reveal.
- **Re-PUT rotates:** a bulk PUT repeating `generateValue: true` for the same key produced a *different* 44-character value. Regeneration is not idempotent — repeating the request rotates the shared secret for every linked service on its next roll (matching the service-level rotation in sweep 625).
- Responses stay value-free on every bulk surface; only the deliberate per-key reveal returns the secret.

## Unrun

- Two services linked to one group reading the generated key: do both receive the identical shared value in-process (expected) — needs two live fixtures.
