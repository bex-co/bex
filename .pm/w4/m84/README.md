# w4 · m84 — Durable pending custom-domain claims and DNS-TXT ownership

**Worker:** worker4 **Goal:** turn the existing DNS-TXT pre-proof into a durable claim workflow: adding a custom hostname reserves a unique, app-bound pending claim and returns actionable proof instructions, but cannot project an Ingress or Certificate until a fresh DNS check atomically promotes the claim to verified. **Status:** todo

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Persist pending/verified domain claims and backfill existing domains     | 60m | —          |
| t002 | Separate claim lifecycle from serving projection on every write path     | 90m | t001       |
| t003 | Verify TXT ownership and promote claims atomically                       | 60m | t002       |
| t004 | Expose pending proof instructions across API surfaces and dashboard      | 45m | t003       |
| t005 | Render parity                                                            | 30m | t004       |
| t006 | Simplify                                                                 | 20m | t005       |
| t007 | Test coverage                                                            | 60m | t005       |
| t008 | Closeout                                                                 | 10m | t007       |

## Definition of done

- Adding a new custom domain creates or returns one globally unique pending claim with a cryptographically random challenge bound to workspace, app, and canonical hostname. It does not add the hostname to `App.spec.hosts`, Ingress, or Certificate.
- Missing, wrong, stale, or resolver-failed TXT proof leaves the claim pending. Correct proof atomically promotes the same claim to verified; only verified rows enter serving projection.
- Existing domain rows migrate/backfill as verified without outage. Re-add is idempotent for the owning App; another App receives the existing non-enumerating conflict behavior for pending and verified claims alike.
- Direct add, service create, Blueprint sync, redirect-sibling pairing, reprojection, and deletion all use the same lifecycle; none can bypass verification or strand a pending/verified claim.
- REST, GraphQL, MCP, and dashboard expose pending/verified state and copy-ready TXT name/value instructions without putting challenges in URLs, logs, metrics, or secrets storage.
- Storeless deployments remain fail-closed: they may retain the existing verify-before-add path, but can never serve an unverified hostname.
- Migration, real-Postgres concurrency, fake DNS, projector, cross-surface, dashboard, and end-to-end lifecycle tests are green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w4` on 2026-08-17, promoting `.pm/w1/045.md` after its `w1/m66` + `w1/m67` sequence gate completed. Current main already has deterministic app-bound TXT pre-proof; this milestone completes it into the durable pending lifecycle the note originally called for.
- **Goal linkage:** `.pm/GOAL.md` goals 2, 5, and 7 — tenant isolation, truthful state, and Render-compatible self-service; ADR008's public-hosting control-plane boundary.
- **Expected outcome:** tenants can add a domain before DNS is ready, receive stable instructions, verify later, and serve only after ownership is proven; dangling DNS and first-claim squatting no longer become serving authorization.
- **Why now:** reserved-host checks, global uniqueness, deterministic TXT verification, domain CRUD surfaces, and the w1/m66/m67 security prerequisites already exist. Persistence and projection state are the remaining coherent slice.
- **Render parity:** included because custom-domain create/verify/status and dashboard instructions are directly user-facing.
