# w21 · m410 — Dashboard hardening from the round-25 audit: redirect validation, same-origin, content type, CSP, error headers, the return URL, and the consent screens

**Worker:** worker21 **Goal:** each gap the round-25 audit found in the dashboard is fixed with a test that fails first, or recorded as accepted with the reason, so a later change cannot turn one into an injection point. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Redirect validation: a `//host` that dot-segment collapse produces is refused | 30m | — |
| [t002](t002.md) | Same-origin and content type: stop trusting a client-sent X-Forwarded-Host; require JSON on the JSON endpoints | 45m | — |
| [t003](t003.md) | CSP and error headers: `base-uri`, `form-action`, a narrower `connect-src`, and route-rule headers on non-2xx responses | 60m | — |
| [t004](t004.md) | Kratos `allowed_return_urls`: no localhost in production | 20m | — |
| [t005](t005.md) | The device confirm page and the consent CSRF token | 60m | t001 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 20m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

1. `safeNext` and the service worker refuse a `//host` that dot segments produce.
2. `isSameOrigin` does not trust a client-sent `X-Forwarded-Host`, and the JSON routes require `Content-Type: application/json`.
3. The CSP names `base-uri` and `form-action` and a narrower `connect-src`, `unsafe-inline` is recorded as accepted with what removing it takes, and error responses keep the route-rule headers.
4. Production lists no localhost return URL.
5. The device page and the consent token are fixed, or the decision the owner has to make is stated and the task parked.
6. Each is a test that fails first, and `cd dashboard && yarn lint && yarn test` pass.

## Source + Goal linkage

- **Source:** inbox note w21/072 (promoted 2026-11-14), from the w21/m394 round-25 audit, hardening (Dashboard).
- **Goal linkage:** the security-review lineage (ADR028 to ADR110) and the dashboard's role as the credential-bearing browser surface.
- **Expected outcome:** none of the audit's seven items remains a gap a later change could turn into an open redirect, a CSRF, or a script injection.
- **Why now:** each is cheap while the audit's evidence is fresh, and the first one is a latent open redirect that only the router currently stops.
- **Render parity task:** omitted. Browser-surface hardening; no REST, GraphQL or MCP behavior and no customer-facing Render behavior is touched.

## Open decisions

t005: whether the human consent screen and device bridge should exist at all (`.pm/DO_NOT_DO.md` allows at most a headless consent acceptor). The owner's call; the task parks with the exact question if hardening them is not enough.
