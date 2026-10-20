# CLI sweep 628 — group secret rotation fans out to both linked services

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixtures `qa628-a`/`qa628-b` (free busybox, serving a truncated sha256 of the group key) and group `qa628-group` with a `generateValue` key; all deleted at close (group 404, services 404, zero `qa628` names). The secret itself was never printed — only hashes. Completes sweep 627's Unrun item.

- **Shared start:** both services converged on the same initial hash after linking.
- **Rotation converges everywhere:** one per-key PUT with `generateValue: true` rolled both services; within ~30 seconds each was serving the identical new hash (`916c82c9…`, distinct from the initial `7f5e9ce1…`). No service kept the old value, and the two never disagreed at any poll where both had rolled.
- A harness note for future sweeps: the first rotation attempt addressed an empty group id (a renamed scratch file) and got a plain `404` — the probe loop then correctly never observed a new hash, and the real rotation was re-run against the recorded id. The 404 caused no state change.

## Unrun

- Rotation while one linked service is suspended (its share waits for the next deploy, per sweep 590's held semantics).
