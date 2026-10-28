# w8 · m140 — Password-free internal Key Value connections

**Worker:** worker1 **Goal:** Render's host/port-only Key Value clients work in every workspace without restarting Valkey. **Status:** todo (t001–t006 done; full ship gate and hosted acceptance pending)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Implement transparent authenticated internal transport | 40m | — |
| t002 — **DONE** | Integrate a readiness-gated stable Service cutover | 50m | t001 |
| t003 — **DONE** | Preserve workspace and environment isolation on both hops | 30m | t002 |
| t004 — **DONE** | Publish Render-shaped internal connection URLs | 25m | t003 |
| t005 — **DONE** | Render parity | 20m | t004 |
| t006 — **DONE** | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t006 |
| t008 | Hosted acceptance | 40m | t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- Render's webdis guide works with REDIS_HOST/REDIS_PORT alone: PING is PONG and SET/GET round-trip.
- Internal URL is redis://<id>:6379 after readiness-gated migration; old authenticated URLs still work.
- External TLS continues requiring valid authentication; same-environment access works and cross-workspace/environment traffic remains denied.
- CONFIG SET remains denied through the internal front; migration changes no Valkey pod template, preserving Off-cache data.
- Hosted fixtures are deleted by ID with 404 proofs; all relevant suites and simplify pass.

## Source + Goal linkage

- **Source:** w8/204, CLI QA sweep 9098, 2026-10-26; promoted 2026-10-28.
- **Goal linkage:** Render parity first (ADR008); native host/port clients must work against managed Key Value.
- **Expected outcome:** webdis and host/port-only workers reach their private store without a password while external AUTH and default ACL restrictions remain.
- **Why now:** an official Render deployment guide currently fails every command with NOAUTH, and ADR018's unsigned safe-superset claim is false.
- **Design:** a per-store proxy Deployment authenticates upstream as default over a dedicated backend Service. The stable Service switches only after authenticated readiness. External TLS is passed through unchanged. Valkey's template is untouched, avoiding data loss on Persistence Off caches. Internal-auth enforcement toggle is separate follow-up work.
- **Render parity task:** included because shared connection-info and Blueprint URL projections change on every surface.

## Verification checkpoint — 2026-10-28

Protocol race/socket tests, types and Key Value API suites, full operator `make test`, and real-Valkey smoke checks pass. The real Valkey check exercised unauthenticated PING/binary SET/GET, CONFIG denial under the default ACL, old credentialed AUTH, backend auth enforcement and authenticated readiness; its Docker fixture was removed. Simplify completed all three reviews and applied reuse, admission bookkeeping and proxy reservation fixes. Full fresh-dependency backend ship gate and workspace lint must pass before push; hosted acceptance remains pending deployment.
