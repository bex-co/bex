# w4 · m84 — Durable pending custom-domain claims and DNS-TXT ownership

**Worker:** worker4 **Goal:** turn the existing DNS-TXT pre-proof into a durable claim workflow: adding a custom hostname reserves a unique, app-bound pending claim and returns actionable proof instructions, but cannot project an Ingress or Certificate until a fresh DNS check atomically promotes the claim to verified. **Status:** done

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Persist pending/verified domain claims and backfill existing domains — **DONE** | 60m | —          |
| t002 | Separate claim lifecycle from serving projection on every write path — **DONE** | 90m | t001       |
| t003 | Verify TXT ownership and promote claims atomically — **DONE**                   | 60m | t002       |
| t004 | Expose pending proof instructions across API surfaces and dashboard — **DONE**  | 45m | t003       |
| t005 | Render parity — **DONE**                                                        | 30m | t004       |
| t006 | Simplify — **DONE**                                                             | 20m | t005       |
| t007 | Test coverage — **DONE**                                                        | 60m | t005       |
| t008 | Closeout — **DONE**                                                             | 10m | t007       |

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

## Closeout

- **Shipped lifecycle:** migration `0086_domain_claim_state` backfills every live
  row verified, adds the closed claim state plus random challenge/attempt
  metadata, and refuses a downgrade while pending claims exist. Its verified
  default keeps pre-m84 synchronous-proof writers safe during rolling rollout;
  claim-aware writers explicitly insert pending with a 192-bit random value.
  Exact-host uniqueness covers both states, same-App retries preserve the row
  and challenge, and conditional promotion binds app + claim id + expected
  challenge so a deleted/recreated row cannot inherit stale proof.
- **No-unverified-serving proof:** direct add, service create, Blueprint sync,
  www/apex sibling pairing, deletion, and store reprojection share the durable
  claim lifecycle. Only verified rows enter `App.spec.host`, `spec.hosts`, and
  redirects; a redirect is eligible only when both endpoints are verified. The
  operator remains mechanism-only and gained no DNS resolver or ownership
  transition. Storeless deployments retain their synchronous pre-proof gate.
- **Surfaces and parity:** REST, GraphQL, MCP, and the dashboard expose explicit
  ownership state and recoverable TXT instructions separately from traffic DNS,
  TLS verification, and active serving. Render's current guide, OpenAPI, Verify
  endpoint, Blueprint reference, and repository-pinned official CLI client were
  rechecked on 2026-08-17. Add → DNS → Verify, apex/www pairing, conflicts, and
  `unverified | verified` filters align; Bex's random TXT, hard non-serving
  pending invariant, synchronous fresh-view Verify result, and additive fields
  are documented security extensions. The pinned CLI has generated API support
  but no user-facing custom-domain command.
- **Simplify:** the required reuse/quality/efficiency reviews consolidated the
  dashboard GraphQL shape into a generated fragment, reused DNS-record mapping
  and Store scanning abstractions, centralized ownership-record naming, switched
  pending verification to structured GraphQL codes, added a real three-state
  ownership/certificate/serving presentation, removed pending-only projector
  kicks and idempotent writes, removed an unused pending index, and avoided
  discarded row scans. Managed and storeless view builders remain separate
  deliberately because combining them would hide which persistence boundary
  supplies ownership truth; declaration writes remain transactional per host so
  conflict attribution and challenge creation stay explicit.
- **Verification:** `cd lego/backend && go test ./...`; full
  `BEX_TEST_DB_URI` `go test ./internal/store -count=1` against disposable
  PostgreSQL 17 (including migration/backfill, rolling writer, concurrent claim,
  stale promotion, and verified-only projection); `cd lego/operator && make
  test`; `make lint` (`0 issues` across all Go modules); dashboard `yarn test`
  (330 files / 2264 tests) and `yarn lint`; offline GraphQL codegen from the
  backend schema; `git diff --check`.
