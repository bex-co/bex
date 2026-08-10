# w1 · m67 — Security-scan design fixes: admission, token class, connect binding, webhook bounds

**Worker:** worker1 **Goal:** the four Tier-2 findings that need real design — not one-line patches — are closed: unauthenticated identity-provider amplification, the blanket audience-less token exception, the portable GitHub connect state, and the two unbounded webhook resource paths. **Status:** done

## Tasks (in order)

| id   | title                                                                           | est | depends_on                             |
| ---- | ------------------------------------------------------------------------------- | --- | -------------------------------------- |
| t001 | Token-class discipline: exact issuer + audience for human tokens (F6)            | 60m | — — **DONE**                           |
| t002 | Pre-auth admission: IP-keyed limiter + credential bound before Hydra/Kratos (F1) | 75m | — — **DONE**                           |
| t003 | Subject-bound one-time GitHub connect transaction (F5)                           | 90m | — — **DONE**                           |
| t004 | Per-workspace webhook endpoint quota + bounded fanout (F2)                       | 60m | — — **DONE**                           |
| t005 | Webhook delivery retention sweep (F3)                                            | 60m | t004 — **DONE**                        |
| t006 | Render parity                                                                    | 30m | t001, t003, t004, t005 — **DONE**      |
| t007 | Simplify                                                                        | 20m | t006 — **DONE**                        |
| t008 | Test coverage                                                                   | 60m | t006 — **DONE**                        |
| t009 | Closeout                                                                        | 10m | t008 — **DONE**                        |

## Definition of done

- An active Hydra token with an empty `aud` no longer authorizes a **human** subject when the resource is configured; the documented exception survives only for positively identified client classes. ✅ (env-gated activation — below)
- A flood of unique invalid bearers/session tokens from one source is shed **before** any Hydra introspection or Kratos whoami call; oversized credentials are refused before allocation; valid-credential behavior and the inner identity-keyed limiter are unchanged. ✅
- The GitHub connect flow consumes a server-side, single-use transaction bound to the initiating bex subject; a callback presented by a different principal is refused, and a consumed nonce cannot be replayed. ✅
- Webhook endpoint creation refuses past a per-workspace cap enforced at the persistence boundary, and one dispatch pass cannot allocate an unbounded event×endpoint batch. ✅
- Terminal webhook deliveries are purged on an age/count policy by a bounded sweep. ✅

## Outcome (2026-08-10)

**F6 — token-class discipline (`internal/api/auth.go`).** The empty-`aud` exception existed for `client_credentials` API keys but was written as "any active token with an empty aud", so a **human** token minted for a self-registered (DCR) client that never requested this resource also passed — carrying the user's full workspace rights. The rule is now class-aware and uses **Hydra's own client record** as the authority rather than inferring from the token: machine tokens keep the exception unconditionally; an audience-less human token is admitted only when the client carries the `bex.co/platform-client` metadata marker that `scripts/auth-bootstrap-client.sh` already stamps on the clients bex provisions (the official Render CLI's device-flow client, bex-mobile). The lookup is cached per client and consulted **only** on the one path where it changes a decision, so ordinary traffic pays no extra Hydra call; a failed lookup fails closed (503).

⚠️ **Deliberately env-gated (`BEX_OAUTH_REQUIRE_AUDIENCE`, ships `"0"`).** Enforcing before an operator re-runs the bootstrap script would refuse the official Render CLI's logins, which legitimately request no audience — an availability-first exception to the m65 F13/F16 secure-by-default direction, and a temporary one. Activation order is documented in `docs/ADR012-auth.md` §7 and in the deployment manifest comment.

**F1 — pre-auth admission (`internal/api/authadmission.go`, new).** The gate is `auth(rl(handler))` and `rl` keys on the **resolved identity**, so it cannot bound the work of resolving one; there is no negative cache (deliberately — it would mask revocations) and singleflight only coalesces *identical* tokens, so every unique invalid credential bought exactly one Hydra/Kratos round trip from an anonymous caller.

The design decision worth recording: **it meters failures, not traffic.** A per-request IP budget in front of the gate — the obvious fix — would have been wrong here, because the dashboard's SSR calls all arrive from one pod IP carrying each user's forwarded session, and Kratos sessions are not positively cached; that budget would throttle the whole dashboard under load, trading a hypothetical outage for a real one. So a successful authentication costs nothing however many users share a source, while each invalid credential spends one token and an exhausted source is refused before the upstream call. A process-wide in-flight cap bounds the other axis, and credentials over 4 KiB are refused before any allocation. On by default (60 failures/min/IP, 64 in flight), trusted-proxy aware, `0`/`0` restores prior behavior exactly.

**F5 — GitHub connect binding (`internal/github/`, migration 0071).** The signed state carried `{workspace, expiry}` and the callback is anonymous, so the flow's two proofs belonged to **different principals** and were individually portable: an attacker's install URL, completed by a victim GitHub org admin, bound the victim's repositories to the attacker's workspace — and m65's unique installation binding then locked the rightful workspace out. Now `StartConnect` records a server-side transaction (`github_connect_transactions`: nonce, workspace, **initiating subject**, expiry) and the state carries only the opaque nonce, so possessing a state authorizes nothing. The callback consumes the row atomically (`DELETE … RETURNING` ⇒ single-use across replicas) and requires the presenting bex subject to equal the initiator; the Kratos cookie is `SameSite=Lax`, so it does ride GitHub's top-level GET redirect, and an anonymous callback is refused outright. The nonce is consumed even when a later step refuses, so a probed attempt cannot be retried against a different installation. State version bumped to 2 so an in-flight old-format state is refused rather than misparsed.

**F2 — webhook endpoint quota (`internal/store/webhooks.go`).** Endpoint creation had no ceiling anywhere — not service, not store, not schema — while the dispatch worker expands every event across every enabled endpoint of the workspace. Now capped at 25 per workspace, enforced in the **same transaction** as the insert with `FOR UPDATE` over the workspace's rows, so two concurrent creates at the boundary cannot both take the last slot. Refused with a typed `WEBHOOK_ENDPOINT_LIMIT` coded error carrying the limit, identical on REST, GraphQL, and MCP.

**F3 — delivery retention (`internal/store/webhooks.go`, `internal/webhooks/worker.go`).** `webhook_deliveries` is both the durable queue and the dashboard's history view, and the only `DELETE` in the tree was migration `0055`'s one-shot dedup — so terminal rows accumulated forever. A bounded sweep now rides the existing worker tick (hourly at most): terminal rows older than 90 days (`BEX_WEBHOOK_RETENTION_DAYS`) or beyond 1000 per endpoint (`BEX_WEBHOOK_RETENTION_KEEP`) are deleted in batches with `FOR UPDATE SKIP LOCKED` so two replicas cooperate. A pending or retryable delivery is never eligible, and the sweep runs even for workspaces with no enabled endpoints — deleting the last endpoint previously stranded its whole history.

**Verification.** Full backend suite green (`go test ./...`, all packages, including the reworked `internal/api`, `internal/github`, `internal/webhooks`, `internal/store`). New regression tests: 5 token-class cases (incl. the fail-closed lookup and the off-by-default proof), 7 admission cases (flood bounded before upstream with an asserted upstream-call count, valid credentials never charged, per-source isolation, forwarded-client keying behind a trusted proxy, oversized-credential refusal, unchanged-when-off), 7 connect-binding cases written **as the attack** (victim completing the attacker's link, replay, unknown nonce, consumption-on-refusal, anonymous callback, plus the same lure over the real HTTP route), and 5 webhook-bounds cases. The pre-existing full-stack callback test was updated to the new contract and now also asserts an anonymous callback records nothing.

**Render parity (t006).** All four changes reach their surfaces through one core verb each, so REST/GraphQL/MCP stay identical by construction: the endpoint cap is one typed `core.CodedError` (`NewBadRequestError`) that the shared adapters render as REST `params`, GraphQL `extensions`, and MCP error text; token acceptance and the connect callback are gate/route-level and surface-independent. Render has no public counterpart for webhook-endpoint quotas or delivery retention (its own product has undocumented internal limits), so these are bex-side bounds, not parity divergences. No ledger row changed.

**Simplify (t007).** Reused rather than reimplemented: `core.KeyedRateLimiter` + `core.TrustedProxies` for the admission budget (the same primitives behind the four existing IP-keyed limiters), `writeTooManyRequests` for the 429 dialect (so a client cannot tell which limiter shed it), the audit/push retention shape for the sweep, and the existing `classify`/`CodedError` error mapping. The one genuinely new mechanism is the connect transaction, which the existing single-use nonce table could not serve (it stores no subject or workspace).

**Systemic note for the lineage.** F5 and F6 land in exactly the neighborhoods `w1/m65` hardened three days earlier (F2 installation attach, F7 role tuples). Both recurrences shared one missing primitive — a durable server-side record binding two independent proofs — which this milestone now builds for the connect flow. The remaining instance of that shape is the DB→OpenFGA projection (scan F17, tracked in `.pm/w1/046.md`); an outbox there would close it.

**Lint note.** `make lint` reports 18 pre-existing operator-module issues in files this milestone never touched; the backend module lints clean.

**Uncommitted pending `/ship`.**

## Source + Goal linkage

- **Source:** the 2026-08-10 codex-security repository scan (`~/.codex/state/plugins/codex-security/scans/bex/codex-security-bex-qmBeaW/report.md`, revision `855b0ce7`) — Findings 1, 2, 3, 5, 6 (all medium) — and the same-day triage that verified each against source and separated them from the m66 fast path.
- **Goal linkage:** `docs/ADR012-auth.md` (token classes, OAuth 2.1 discovery), `docs/ADR006-bex-api.md` §Rate limits and §Outbound event webhooks, `docs/ADR026-github-integration.md` (installation binding), `docs/ADR003-control-plane.md` (durable state, multi-replica).
- **Expected outcome:** the shared identity services stop being cheaply amplifiable by anonymous traffic; a consented third-party client can no longer be replayed against bex without explicit resource binding; an installation can no longer be bound to a workspace the installing human never chose; one tenant can no longer grow shared worker memory and database storage without bound.
- **Why now:** F5/F6 recur in the neighborhoods m65 hardened, for one shared reason (no durable record tying two proofs together); F1/F2/F3 are the availability half — all three unauthenticated-or-cheap paths into shared multi-tenant resources.
- **Render parity task included** — F2/F3 add tenant-visible refusals and history semantics, F6 changes token acceptance, F5 changes the connect flow's callback contract.
