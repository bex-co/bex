# w4 · m38 — Render-CLI token-refresh idempotency + 7d lifespan

**Worker:** worker4 **Goal:** the official Render CLI's concurrent token refreshes stop racing to 401 — a durable, replica-shared idempotency layer makes `POST /v1/token/refresh/` return one token pair no matter how many refreshes land at once, and the CLI-token lifespan moves from the 25h band-aid to Render's 7d posture **Status:** done

## Tasks (in order)

| id   | title                                                              | est | depends_on   |
| ---- | ------------------------------------------------------------------ | --- | ------------ |
| t001 | Control-plane idempotency store for CLI token refresh (migration)   | 60m | — — **DONE**                   |
| t002 | Idempotent, advisory-locked `refreshToken` proxy (replica-shared)   | 75m | w4/m38/t001 — **DONE**         |
| t003 | Bump CLI-token lifespan to 7d + document the sticky-race mitigation | 20m | w4/m38/t002 — **DONE**         |
| t004 | Simplify pass                                                       | 20m | w4/m38/t003 — **DONE**         |
| t005 | Test coverage                                                       | 40m | w4/m38/t003 — **DONE**         |
| t006 | Closeout                                                            | 10m | w4/m38/t005 — **DONE**         |

## Closeout (2026-08-17)

- **t001** — migration 0082 adds the SHA-256-keyed, byte-preserving response cache with expiry/creation timestamps and an expiry index. The store treats expired rows as misses and reclaims them in bounded `FOR UPDATE SKIP LOCKED` batches.
- **t002** — `refreshToken` hashes the inbound credential before crossing the store boundary; one Postgres transaction takes the digest-derived advisory lock, checks the cache, calls Hydra once on a miss, and writes the successful response before unlocking. OAuth errors remain verbatim and uncached. DB-free local mode retains direct proxying.
- **t003** — the permanent Render CLI Hydra client now uses `168h` for both device and refresh grants. The bootstrap reads the stored client back and checks both fields; ADR012 records the seven-day posture and why replica-shared idempotency removes the sticky lost-race failure.
- **t004** — `/simplify` ran with the required reuse/quality/efficiency reviews. Applied findings: expiry sweeps use `SKIP LOCKED`, expiry is calculated in the cache write from the database clock, the lock invariant is no longer bypassable through exported test-only store methods, repeated grant strings share a constant, and concurrency-test waits are bounded/failure-safe. The migration's required `created_at` was retained.
- **t005** — real-Postgres tests cover migration up/down, store round-trip/logical expiry/physical sweep, two replica-shaped services blocked on the exact advisory key, restart durability, concurrent distinct-token independence, raw-input hash-only persistence, one live access token after one upstream mint, and verbatim error-not-cached behavior.
- **Render parity task omitted** — the OAuth wire shape is unchanged and this milestone adds no REST/GraphQL/MCP resource field or dashboard surface; the concurrent-refresh test is the official-CLI compatibility proof.

**Verified:** `go build ./...`; a clean PostgreSQL 17 run of `BEX_TEST_DB_URI=… go test -count=1 ./...`; `make lint-backend` (`0 issues`); `bash -n scripts/auth-bootstrap-client.sh`; `git diff --check`; and an echoing fake-Hydra execution of `auth-bootstrap-client.sh` proving both `168h` fields survive its PUT→GET round-trip guard.

**Deferred operator step:** the bootstrap was not re-run against production Hydra in this implementation session. Production picks up the seven-day client setting only when an operator runs `scripts/auth-bootstrap-client.sh`; this was explicitly out of implementation scope.

## Definition of done

Two concurrent `POST /v1/token/refresh/` calls for the **same** refresh token — even when Traefik round-robins them onto **different bex-api replicas** — return an **identical** `{access_token, refresh_token}` pair and leave exactly one live access token (neither refresh revokes the other's). The dedupe is **durable** (backed by the control-plane Postgres, survives a bex-api restart), keyed on a **hash** of the refresh token (the raw token is never stored), and self-expiring. `scripts/auth-bootstrap-client.sh` mints **7d** CLI-token lifespans for both grants and its round-trip assertion still passes. A test drives the concurrent-refresh path against a real control-plane Postgres and asserts single-mint idempotency (one upstream Hydra call, one token pair returned to both callers).

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md:33` (filed 2026-07-16, proxy-captured evidence 2026-07-15) via `/pm-brainstorm more work for w4`, 2026-08-17. Verified in code at proposal time: `lego/backend/internal/cliauth/service.go:211`'s `refreshToken` has no idempotency/advisory-lock, and `scripts/auth-bootstrap-client.sh:159` still carries the interim `CLI_TOKEN_LIFESPAN=25h` with an explicit "bump toward 7d if it proves out" note.
- **Goal linkage:** Render-CLI compatibility (the fifth surface, `docs/cli-compatibility-checklist.md` lineage) + auth robustness — continues w4's CLI-auth thread (m27 device login, m31 auth-gate hardening, m35/m37 CLI distribution + self-update).
- **Expected outcome:** interactive official-CLI sessions (`render logs`, the TUI) stop intermittently 401-ing on token refresh; the fix holds at any token TTL and the moment bex-api scales past one replica.
- **Why now:** the shipped 25h lifespan is an admitted band-aid (its own comment says so); the idempotency layer is the acknowledged *correct* fix and removes a latent multi-replica correctness race (an in-memory singleflight can't dedupe refreshes Traefik splits across replicas). Proactive: the filed trigger ("the 401 recurs under 25h, or a second CLI-auth incident") has not provably fired — it is being addressed as a latent-correctness fix rather than waiting for a user-facing recurrence.
- **Render-parity task omitted:** no REST/GraphQL/MCP field or dashboard change — the work is internal auth-proxy behavior (`internal/cliauth`) + a Hydra client-config value, and it makes bex's refresh **more** Render-consistent (7d posture, OAuth token shape unchanged). The **Test-coverage** task instead carries the CLI-compat assertion (an unmodified-CLI-style concurrent refresh returns one pair, no 401).
