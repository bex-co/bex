# w7 · m426 — A bex API key works as a bearer token, as a Render API key does

**Worker:** worker7 **Goal:** one bex API key string, sent as `Authorization: Bearer`, authenticates REST, GraphQL and MCP as a Render API key does, so Render's clients work with only the host and the key changed; the `client_credentials` exchange keeps working. **Status:** blocked — release gate, no user action (t001–t005, t008 and t009 done and shipped: `8227312bc` server, CLI and docs, `956e920d0` dashboard; t006, t007 and t010 wait for the production pin to carry `956e920d0`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | ADR012 §8 defines the bearer key: its string, the server-side exchange, failure limits; the ADR018 row follows — **DONE** | 45m | — |
| t002 | bex-api accepts an API key as `Authorization: Bearer` on REST, GraphQL and MCP — **DONE** | 120m | t001 |
| t003 | Create answers one key string, and the dashboard shows it with a curl example — **DONE** | 75m | t002 |
| t004 | Failed key attempts are rate-limited, counted and never logged — **DONE** | 45m | t002 |
| t005 | The CLI and the skills pack use an API key directly — **DONE** | 60m | t002 |
| t006 | Live check on production: curl, the `render` CLI, `/mcp` and the Terraform provider with one key | 45m | t003, t004, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

On production, as the QA user in `bex-canary`, with an owned key that is revoked afterwards:

1. One API key string sent as `Authorization: Bearer <key>` answers 200 on `GET /v1/services`, on a GraphQL query and on an MCP `tools/call`, acting as the key's workspace machine identity (the audit rows name the key's id).
2. The unmodified `render` CLI at the pinned version, with `RENDER_HOST` and `RENDER_API_KEY=<key>`, lists services. Render's Terraform provider, with `RENDER_HOST`, `RENDER_API_KEY` and `RENDER_OWNER_ID`, reads one existing service in a `terraform plan`.
3. After the key is revoked, every surface answers 401 within the window ADR012 §8d states.
4. A wrong secret, a malformed key and random text all get the same 401 body. Repeated failures are rate-limited (429) without saying which part was wrong.
5. A `client_credentials` exchange with the same key's id and secret still works.
6. ADR012 §8 and ADR018's API-key row describe the bearer key, and no dashboard locale says a key is "not a bearer token".
7. `cd lego/backend && go test ./...`, `make lint`, `cd lego/cli && go test ./...`, and dashboard `yarn lint` + `yarn test` pass.

## Record

**2026-11-16, implementation (t001–t005, with t008 and t009 run ahead of the production check).** What shipped is in each task file under `done/`. In short:

- **The key.** `bex_<id>_<secret>`, defined in ADR012 §8e. bex-api exchanges it at Hydra's token endpoint and remembers the token per replica; nothing new is stored. Create answers it once as `key` on REST, GraphQL and MCP.
- **Checked against real components**, on a throwaway kind cluster with a Postgres-backed bex-api, a real Hydra and Kratos, and the `bex` CLI built from the tree (`scripts/auth-oauth21-e2e.sh`, leg `extra/w7-m426`, with `BEX_BIN`):
  - a signed-in person mints a key, and that one string answers 200 on REST, GraphQL and an MCP `tools/call`;
  - `BEX_API_KEY=<key> bex services -o json` lists the key's workspace with no login and no exchange;
  - a wrong secret, an unknown id, a malformed key and random text get one 401;
  - the key's id and secret still exchange for a working token;
  - the revoked key answers 401 on the three surfaces 0.1 s after the revocation (5.2 s on a bex-api without the store, where the bound is the introspection window).
- **Against Hydra v26.2.0**, the version production runs, with a DB-free bex-api: the three surfaces answer 200, the failures answer one 401, and the 58th wrong attempt from one source answers 429 `auth_overloaded` with `Retry-After`.
- **Not yet checked:** anything on production. Definition-of-done items 1 to 5 are t006. Render's Terraform provider and the unmodified `render` CLI have not been run against the change anywhere.

**Merged on the way in.** w21/091 step 1 (`e3a57b1c3`) landed in `auth.go` and `authadmission.go` while this was shipping. The two are merged: a key answered 429 is counted in its `bex_api_auth_admission_sheds_total` like every other, under `source` or a new cause `key_id` for the per-key-id bound, and its test and ADR088 §6 list that cause.

**2026-11-16, parked on the release gate (t006 step 1).** The implementation is on main as `8227312bc` (server, CLI, docs, alert) and `956e920d0` (dashboard), the second containing the first. The newest `chore(deploy): pin platform images to …` commit on main at the check pinned `5be555762b06`, which does not contain it, so production does not serve the change yet and t006 cannot run.

- **The gate clears by itself**: the deploy workflow builds main and commits the next pin. Nothing is needed from anyone.
- **The check that proves it cleared:**

  ```bash
  git fetch origin main
  pin=$(git log origin/main --grep='pin platform images to' -1 --format=%s | sed -E 's/.*pin platform images to ([0-9a-f]+).*/\1/')
  git merge-base --is-ancestor 956e920d0 "$pin" && echo "the pin carries 956e920d0"
  ```

- **Then:** t006 (the production walk of definition-of-done items 1 to 5 as the QA user in `bex-canary`, with the pinned `render` CLI and Render's Terraform provider), t007 (Render parity, with the note below), t010 (closeout).

**Order.** t008 (Simplify) and t009 (Test coverage) list t007 as a dependency and were done first, because the review and the tests had to precede the ship. If t006 or t007 changes code, both get another pass.

**Found on the way, filed:** `w7/064` (a DB-free bex-api crashes at start once Hydra holds an API key) and `w7/065` (`scripts/auth-oauth21-e2e.sh` fails at its older leg 8 and leaves its servers running).

**For t007.** One thing to weigh there, seen locally: the 401 is one REST-shaped body on all three surfaces, while the 429 is REST-shaped on REST and MCP and a GraphQL `errors` envelope on `/graphql`. That is how `auth_overloaded` has answered since w4/m100 for every credential, not something this milestone introduced.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 1; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/api, read 2026-11-16: "All Render API requests require authentication via API key." The first request is a curl with the key in place of `{{render_api_token_goes_here}}`; "Revoking an API key takes effect immediately." Render's Terraform provider sends `req.Header.Set("Authorization", "Bearer "+p.APIKey)` and reads its host from `RENDER_HOST` (`render-oss/terraform-provider-render`, `internal/provider/provider.go`, main, read 2026-11-16).
- **bex today:** a key is a Hydra `client_credentials` client (`lego/backend/internal/apikeys/service.go`). ADR018 § Identity, workspaces & account, API-key row: "A Render-shaped client that pastes the secret as a bearer gets `401 unauthorized`", recorded as "deliberate, ADR012" with no user sign-off. ADR012 §8 says the same. The dashboard says "This is an OAuth2 client_credentials client, not a bearer token" (`dashboard/src/features/api-keys/locales/en.ts:124`). The skills pack rewrites Render's one-line `RENDER_API_KEY` CI step into a curl + jq exchange (`lego/cli/internal/skills/packgen/packgen.go:72-75`).
- **Goal linkage:** ADR008 Render parity first; ADR006 (bex-api is Render-compatible, and authentication is every Render client's first step).
- **Expected outcome:** Render's SDKs, its Terraform provider and CI scripts written for Render run against bex with the host and the key changed, with no token exchange and no 15-minute expiry.
- **Why now:** the divergence has no recorded sign-off, so by the AGENTS.md parity rule it is amended, not kept. `w32/blocked/m284` (Render SDK compatibility) waits on a short-lived `BEX_ACCESS_TOKEN` from the user; a bearer key removes that kind of gate for QA and for CI.
- **Decision state:** accepting the key as a bearer on every surface is this proposal's recommendation, resting on ADR008 Render parity first and the AGENTS.md rule to amend an ADR that conflicts with parity. It is not a recorded user decision. Keeping the exchange as the only way in would be a divergence that needs the user's sign-off in ADR018.
- **Not changed here:** a Render API key acts for its user across that user's workspaces; a bex key is a workspace machine identity. t001 records that difference in ADR018; this milestone does not change it.
- **Adjacent work:** `w21/090` and `w21/091` also edit `lego/backend/internal/api/auth.go` (revocation fan-out, the introspection window). `w1/m404` is about device-login token refresh, not API keys.
- **Render parity:** included: authentication changes on REST, GraphQL and MCP, and the dashboard's key dialog changes.
