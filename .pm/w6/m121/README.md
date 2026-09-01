# w6 · m121 — CORS preflight advertises a stale method + header list, so 53 mutating REST routes and two real request headers are unreachable from the origin bex itself allowlists

**Worker:** worker6 **Goal:** the preflight bex returns describes the router bex actually serves, so its Render-compatible REST surface is usable from a browser instead of read-only **Status:** todo (t001–t007 done; t008 awaits deployment + live production DoD)

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Derive `Access-Control-Allow-Methods` from the verbs the router registers — **DONE**                | 45m | —          |
| t002 | Fix `Access-Control-Allow-Headers`; add `Idempotency-Key`, keep `X-Api-Key` out, set Max-Age — **DONE** | 45m | t001       |
| t003 | Live-verify whether a cross-origin `EventSource` reconnect preflights `Last-Event-ID` — **DONE**    | 30m | —          |
| t004 | Drift guard: assert the preflight is a superset of the router, enumerated from the mux — **DONE**   | 40m | t001, t002 |
| t005 | Render parity — **DONE**                                                                            | 20m | t004       |
| t006 | Simplify — **DONE**                                                                                 | 20m | t005       |
| t007 | Test coverage — **DONE**                                                                            | 30m | t005       |
| t008 | Closeout                                                                                 | 10m | t006, t007 |

## Definition of done

- **The blocked verbs complete.** From `https://dashboard.bex.co` with a live session cookie, `fetch(url, {method:'DELETE', credentials:'include'})` against a REST route that serves DELETE **completes instead of throwing `Failed to fetch`** — verified on **at least two route families**, since both hunts that hit this used different ones (41st run: `DELETE /v1/services/{id}/custom-domains/{name}`; 55th run: `DELETE /v1/postgres/{id}`). Same check for a PATCH route and a PUT route.
- **The advertised list is the router's list, not a constant.** `OPTIONS` on a REST route returns an `Access-Control-Allow-Methods` that is a **superset of the verbs the router registers for that route**. Today it is the fixed string `GET, POST, OPTIONS` for every route: the 55th run sent `Access-Control-Request-Method: DELETE` and then `POST` to the same path and got the byte-identical method string back both times, which is what proves it is a constant.
- **`Idempotency-Key` survives the preflight.** A browser client can send it to the route that reads it at `lego/backend/internal/webhooks/rest.go:213` and the handler receives a non-empty value.
- **`Last-Event-ID` is decided by test, not by assumption.** Either it is advertised, or t003's live run shows a cross-origin `EventSource` reconnect does not need it advertised. The result is written down either way.
- **`X-Api-Key` is NOT in `Allow-Headers`,** and a comment at the list records why (it is outbound-only — `internal/sshgateway/modelproxy/modelproxy.go:380,384` — so the raw grep count that suggests otherwise does not mislead the next reader).
- **Drift fails the build.** Adding a route on a verb the preflight does not advertise makes `cd lego/backend && go test ./internal/api/...` fail, rather than silently 404-ing browser clients. Demonstrate by adding a throwaway route on a new verb and watching the test go red.
- **The origin allowlist is untouched.** A request carrying a non-allowlisted `Origin` still receives **no** CORS headers at all, and the `Vary: Origin` / `Vary: Accept-Encoding` behavior preserved by `e2394e52` still holds.

## Implementation evidence (2026-08-31)

- The working tree now derives each preflight answer from the composed root + REST muxes, advertises the requested routed method plus `OPTIONS`, adds `Idempotency-Key`, deliberately excludes `X-Api-Key` and browser-owned `Last-Event-ID`, and sets `Max-Age: 7200`.
- Live Chrome 152 cross-origin EventSource capture settled `Last-Event-ID`: first GET had no cursor; the reconnect GET carried `Last-Event-ID: cursor-1`; no OPTIONS request occurred. The signed-in production variant could not run because QA credentials are unset, and is not claimed.
- `go test ./...` passes in `lego/backend`; `make lint-backend` reports zero issues. The structural guard enumerates the real mux and has an executable stale-router/throwaway-verb failure case.
- **Closeout is intentionally still open.** Production was re-read after implementation and still returns `Access-Control-Allow-Methods: GET, POST, OPTIONS` and the old header list for DELETE, PATCH and PUT. Repository policy forbids commit/push without `$ship`, so t008's deployed browser DoD has not happened.

## Source + Goal linkage

- **Source:** `.pm/w6/060.md` — filed 2026-08-27 from the **41st** `/qa-find-bugs` run (journey 5, custom domains), found while trying to clean up throwaway domain claims over REST. Independently reproduced on a second route family by the **55th** run (2026-08-27T18:57Z, journey 11 / Postgres, workspace `tea-d98210cbbpdc73dcrkvg`), which also captured the raw preflight and measured the header list `060` had left explicitly unverified. `060` moves to `w6/done/060.md` as the originating note; its full repro, GraphQL control, 53-route table and Render note are preserved there.

- **The 55th run's evidence.** One page, one session, back to back:

  ```
  GET    https://api.bex.co/v1/postgres/dpg-da88eqfm2e9c73ft5vmg  ->  ok 200
  DELETE https://api.bex.co/v1/postgres/dpg-da88eqfm2e9c73ft5vmg  ->  THREW "Failed to fetch"
  ```

  The GET is the control: identical origin, cookie and page, so the session and the origin allowlist are provably fine and only the method list refuses. The preflight read directly — a stronger artifact than `060`'s console message, which was the browser's rendering of this:

  ```
  OPTIONS /v1/postgres/{id}
    Origin: https://dashboard.bex.co
    Access-Control-Request-Method: DELETE
  -> 204
     access-control-allow-origin:       https://dashboard.bex.co
     access-control-allow-methods:      GET, POST, OPTIONS
     access-control-allow-credentials:  true
  ```

  The server **accepts** the preflight (204, origin correctly echoed) and then omits the method, so the browser never sends the real request. Re-running with `Access-Control-Request-Method: POST` returned the identical method string — a static constant, not a per-route computation.

- **Root cause:** `lego/backend/internal/api/auth.go:724-725`, two literals inside `withCORS` (`:708-734`):

  ```go
  w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Session-Token")
  w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
  ```

  The function's own doc comment explains the origin allowlist and why `Allow-Credentials` is needed for the Kratos cookie; it says nothing about restricting methods, so this reads as a stale default rather than a decision. The router disagrees with it on **53 distinct route patterns**: DELETE 26, PATCH 17, PUT 10 (`grep -rho '"<VERB> /[^"]*"' internal/ --include='*.go' | grep -v _test | sort -u | wc -l`), across services, custom domains, env vars, env groups, environments, registry credentials, ssh keys, Postgres and Key Value.

- **Header list, measured (settles `060`'s Unverified line).** Advertised: `Authorization, Content-Type, X-Session-Token`. Also measured: **no** `Access-Control-Max-Age` (every preflight costs a round trip) and no `Access-Control-Expose-Headers`. Against the 37 distinct headers handlers actually read (`grep -rhoE '\.Header\.Get\("[^"]+"\)' internal/ --include='*.go' | grep -v _test`), filtered to ones an inbound browser would send:
  - `Idempotency-Key` — **confirmed gap**, read at `internal/webhooks/rest.go:213`. Its route's verb (POST) is allowed but the header is not, so a browser client silently loses idempotency.
  - `Last-Event-ID` — read at `internal/logs/rest.go:159` (SSE log-stream resume), absent from the list. This was untested in the source run; t003 resolved it above with a live Chrome reconnect capture: browser-owned `Last-Event-ID` does not preflight.
  - `X-Api-Key` — **ruled out; do not "fix" it.** Only `internal/sshgateway/modelproxy/modelproxy.go:380,384`, where the gateway sets a credential on an **outbound** upstream request. It is not an inbound auth header.
  - Deliberately excluded: webhook-signature headers (`webhook-id`/`webhook-timestamp`/`webhook-signature`, `Stripe-Signature`, `X-Hub-Signature-256`, `X-GitHub-*`) are server-to-server and never browser-sent; `X-Vault-Token`, `X-Goog-Api-Key`, `OPEN-SANDBOX-API-KEY` are outbound; `X-Forwarded-*`, `Cookie`, `Origin`, `Accept-Encoding`, `Content-Length` are infrastructure- or browser-managed.

- **Goal linkage:** [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md) — bex advertises a Render-compatible REST surface. Today that surface is **read-only** to any browser client on the one origin bex itself allowlists, while `POST /graphql` on that same origin mutates everything.

- **Expected outcome:** a browser client of bex's REST API can call the mutating routes the API documents, and the preflight stops being a second hand-maintained description of the router that nothing keeps honest.

- **Why now:** the security rationale is already dead — `060`'s control deleted the same three resources from the same browser, origin and cookie via `POST /graphql`, so the method list blocks nothing an attacker on that origin could not already do; it only removes REST parity. Two independent hunts have now hit it while doing ordinary cleanup, which is evidence it obstructs real API use rather than only tests.

- **Render parity:** included (t005) — this changes an advertised REST surface. Note when writing it that Render documents its REST API as server-to-server and does not invite browser clients, so bex is **not** restoring a Render behavior here; it is making its own allowlisted origin coherent with its own router. Worth a ledger line in `docs/ADR018-render-parity.md` only if the advertised surface changes shape.

- **Blast radius:** `withCORS` is a single function fronting the entire REST router, so the change is global to every route it serves — 53 currently unreachable by verb. The GET/POST routes that work today must get regression coverage too, not only the broken verbs: they work *because* of the current string, so they are exactly what a careless "derive it" refactor would break.

- **Adjacent classes:** a non-allowlisted `Origin` must keep receiving **no** CORS headers at all (the `len(allowed) > 0` / `allowed[origin]` guard is unchanged); same-origin and server-to-server callers never preflight and are unaffected. Widening the **method** list must not widen the **origin** list — they are separate decisions that live two lines apart.

- **Consumer, checked:** the dashboard does not exercise the broken path, which is why this survived. `grep -rn "method: *['\"]DELETE" dashboard/src` returns **zero** hits; a database delete goes through `dashboard/src/features/databases/hooks/use-delete-database.ts` → `databases.graphql`. The 55th run deleted its own fixture with `mutation { deleteDatabase(id:) }` → `{"data":{"deleteDatabase":true}}`, and the REST GET 404'd within 1 second.

- **Still unverified:** whether a third-party browser client of the REST API exists in production today, and whether `BEX_API_CORS_ORIGIN` is set to anything beyond the dashboard in production (never read — the hunts observed only the dashboard origin being echoed). The `EventSource` reconnect question is resolved by t003 above.
