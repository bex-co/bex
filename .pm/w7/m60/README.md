# w7 · m60 — Outside-gate public route hardening: webhook intake metering + completeness guard

**Worker:** worker7 **Goal:** the last two unmetered unauthenticated POST routes (`/v1/webhooks/git`, `/v1/webhooks/stripe`) shed floods before any signature work, and a CI completeness guard forces every future directly-mounted route to be classified and protected **Status:** todo

## Tasks (in order)

| id   | title                                                                                                 | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | IP-keyed token bucket on `/v1/webhooks/git` + `/v1/webhooks/stripe` (`BEX_WEBHOOK_RATE_LIMIT`/`_BURST`) | 45m | —          |
| t002 | Completeness guard over the composed `Handler()` mux (the `012` fix)                                    | 45m | t001       |
| t003 | Env-table / `.env.example` / `internal/api/CLAUDE.md` always-public inventory sync                      | 15m | t002       |
| t004 | Simplify pass over the changed code                                                                     | 20m | t003       |
| t005 | Test coverage: shed-before-HMAC, legit-delivery pass-through, guard turns red on unclassified mounts    | 45m | t003       |
| t006 | Closeout                                                                                                | 10m | t005       |

## Definition of done

- An unauthenticated flood against either webhook route sheds with 429 **before** HMAC/signature verification, while a signed legitimate delivery inside the budget always passes; `0` disables (byte-identical default off or generous documented default — decided in t001).
- A new directly-mounted route added to `server.go`'s `Handler()` without a documented always-public classification (including its limiter story) turns CI red.
- `.env.example`, the root `CLAUDE.md` env table, and `internal/api/CLAUDE.md`'s always-public inventory are in sync with the new knobs.
- Backend suite + lint green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w7` round 2, 2026-07-30 code sweep — `Handler()` (`lego/backend/internal/api/server.go:837`) mounts both webhook intakes outside the auth gate where neither the identity-keyed `BEX_RATE_LIMIT` limiter nor the device-flow `BEX_DEVICE_RATE_LIMIT` limiter reaches them; each request costs a full body read (≤ 2 MiB) + HMAC computation. Absorbs inbox `w7/012` (ADR045 Finding 8 — no completeness guard over directly-mounted routes).
- **Goal linkage:** API abuse hardening — the w7/m3 rate-limits + w4/m31 device-flow-limiter lineage applied to the last unmetered public surface.
- **Expected outcome:** no unauthenticated route on bex-api can be flooded for free; the outside-gate surface is CI-inventoried so it can't silently grow.
- **Why now:** these are the only unauthenticated, unmetered POST surfaces left after the device-flow limiter closed the same class of gap; `012` is already-filed audit debt that the same milestone retires.
- **Render parity:** omitted — both routes are bex-internal/extension surfaces (the git webhook is bex's ADR017 push-to-deploy extension; the Stripe intake is internal billing machinery) with no Render-comparable contract. The 429 shape gets documented in t001.
