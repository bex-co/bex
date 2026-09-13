# w4 · m105 — A dashboard-minted API key cannot be used from the dashboard alone: no `client_id`, and the advertised scopes are refused

**Worker:** worker4 **Goal:** a user who mints an API key in Settings can authenticate with it without leaving the dashboard — closing `w4/m8`'s own stated goal ("handing an agent a credential no longer requires `curl`") and the round trip `w4/m8` recorded as unverified. **Status:** todo

## Tasks (in order)

| id   | title                                                                      | est | depends_on   |
| ---- | -------------------------------------------------------------------------- | --- | ------------ |
| t001 | Surface the `client_id` and what to do with the credential                    | 45m | —            |
| t002 | Settle the scope story: discovery advertises scopes the key cannot request     | 45m | —            |
| t003 | Tests: the mint → exchange → authenticate → revoke round trip                 | 40m | w4/m105/t001, w4/m105/t002 |
| t004 | Render parity sweep over the changed surfaces                                 | 25m | w4/m105/t003 |
| t005 | Simplify pass over this milestone's changes                                   | 20m | w4/m105/t004 |
| t006 | Test coverage for the shipped behavior                                        | 30m | w4/m105/t004 |
| t007 | Closeout                                                                     | 15m | w4/m105/t006 |

## Definition of done

- **Mint → authenticate is completable from the dashboard alone.** A user creates a key in Settings → Access credentials and, using only what the dashboard shows them, obtains a token and makes an authenticated API call. Today the one-time dialog shows **only** the 26-character secret — verified live, the dialog contains exactly one `<code>` element and the text "Copy this key now — you won't be able to see it again." — and the keys table has columns Name / Created / Created by / Last used / Actions with **no** id column, so the `client_id` the credential requires appears nowhere in the UI.
- **The `client_id` is visible and copyable** for every key, not only at mint time, because it is an identifier rather than a secret (Hydra omits secrets from list reads — `apikeys/service.go:539`).
- **The credential's usage is stated where it is minted.** The dialog or its neighbourhood says this is an OAuth2 `client_credentials` client, names the token endpoint, and gives the exact exchange — because "API key" invites the reasonable assumption that it is a bearer token, and using it as one returns a bare `401 unauthorized` (verified).
- **Requesting the advertised scopes succeeds, or discovery stops advertising them to this client class.** `GET /.well-known/oauth-protected-resource` returns `scopes_supported: ["bex.read","bex.write","bex.sensitive"]`, yet `scope=bex.read` on the exchange is refused: `invalid_scope — The OAuth 2.0 Client is not allowed to request scope 'bex.read'` (verified). Whichever way t002 resolves it, a caller following the discovery document must not be rejected.
- **The round trip has a test.** `w4/m8` shipped with "no live-cluster browser check of the mint→authenticate round trip" and `w4/m13` with "the TTL's live-introspection check needs a running Hydra"; both gaps are why this went unnoticed for two months. A test covers mint → exchange → authenticated call → revoke → both refusals.

## Verified live this run (2026-09-13 pass 13)

Key `qa-0913k-key`, minted through the dashboard and revoked at the end of the run. The secret is deliberately **not** recorded here.

```text
# the key used directly as a bearer — the natural reading of "API key"
GET /v1/services?limit=1   Authorization: Bearer <secret>
→ 401 {"error":"unauthorized","id":"unauthorized","message":"unauthorized"}

# with the advertised scope, using the client_id obtained from GraphQL (not the UI)
POST https://oauth.bex.co/oauth2/token
  grant_type=client_credentials&client_id=<id>&client_secret=<secret>&scope=bex.read
→ {"error":"invalid_scope","error_description":"… not allowed to request scope 'bex.read'."}

# omitting scope entirely — works
POST https://oauth.bex.co/oauth2/token
  grant_type=client_credentials&client_id=<id>&client_secret=<secret>
→ access_token (94 chars)

GET /v1/services?limit=2                              → 200
GET /v1/owners/tea-d98210cbbpdc73dcrkvg/limits         → 200
  {"services":{"used":6,"terminating":1,"limit":100},"postgres":{…},"keyValues":{…}}
```

So the credential is fully functional; the **only** missing ingredient is the `client_id`, which the UI never shows. `apikeys/service.go:482` confirms `APIKey.ID` *is* the Hydra client id (`APIKey{ID: c.ClientID, …}`), and GraphQL `apiKeys { id }` returns it — the data exists, it simply is not surfaced. The minted client is created with `GrantTypes: ["client_credentials"]`, `AuthMethod: "client_secret_post"` and **no `Scope` field** (`apikeys/service.go:500-505`), which is why the advertised scopes are refused.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-13 pass 13, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`. One API key minted through the UI and revoked in the same run.
- **This closes a gap two milestones recorded and neither took.** `w4/m8` (done) built this surface with the goal that "handing an agent a credential no longer requires `curl`", and its own status line records "**one gap not closed this session: no live-cluster browser check of the mint→authenticate round trip**". Doing that round trip is what exposes this: it cannot be completed without `curl` *and* a GraphQL query. `w4/m13` (done) added the `created-by`/`last-used` columns and left "the TTL's live-introspection check needs a running Hydra" open. Neither is re-filed; this is the unverified round trip finally exercised.
- **Goal linkage:** `docs/ADR012-auth.md` §7/§8 owns the API-key credential model (Hydra `client_credentials`, 15m access-token TTL); ADR008 pillar 4 (deploy-from-chat) depends on an agent being able to hold a working credential.
- **Expected outcome:** a machine credential minted in the product is usable from the product.
- **Why now:** the surface has existed since 2026-07-08 and the round trip has never been exercised end to end; the fix is small, and the scope inconsistency is a trap for exactly the careful caller who reads the discovery document first.
- **Render parity task included:** yes — Render's API keys are plain bearer tokens (`rnd_…`), so bex's OAuth2 client-credentials model is a deliberate ADR012 divergence whose user-facing presentation is a parity question worth recording.

## Verified working, and not filed

- **Revocation is immediate and complete**, exactly as its confirm dialog promises ("Anything authenticating with this key will stop working immediately. This can't be undone."): the **already-issued** access token began returning `401` within seconds — so the introspection cache honours revocation, which live-exercises `w4/m31`'s `revocationEpoch` work — and a fresh exchange fails `invalid_client`. The key also disappeared from `apiKeys` immediately.
- **`last-used` is real.** `lastUsedAt` was `""` at mint and `2026-09-13T19:08:38Z` after the authenticated calls — `w4/m13`'s throttled `TouchAPIKey` working.
- **The mint dialog's one-time-secret discipline is correct**: shown once, unmasked, with a Copy button and an explicit warning; the list never returns it afterwards.
- **`m100`'s bearer control case is settled here** — see that milestone's re-probe log. Bearer: 25 parallel → 25 × 200, 40 parallel → 40 × 200, 90 sequential → 0 × 429. Session cookie under the identical probe (pass 8): 5 × 429 of 25 and 17 × 429 of 40. The shed is specific to the session credential class, as `auth.go:572-575` predicts.

## Not verified

- Whether an API-key token can reach every surface a session can (only `/v1/services` and `/v1/owners/{id}/limits` were called), and whether the scopeless token is correctly *refused* anywhere it should be — if scopes are meant to gate `bex.sensitive` reads, a scopeless key may be over- or under-privileged. t002 owns this; it is the part with real security weight.
- SSH key create/delete was **not** exercised this run.
