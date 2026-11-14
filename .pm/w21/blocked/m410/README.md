# w21 · m410 — Dashboard hardening from the round-25 audit: redirect validation, same-origin, content type, CSP, error headers, the return URL, and the consent screens

**Worker:** worker21 **Goal:** each gap the round-25 audit found in the dashboard is fixed with a test that fails first, or recorded as accepted with the reason, so a later change cannot turn one into an injection point. **Status:** BLOCKED (t004 needs the user's decision on the "Local dev against prod" localhost return URL; t001–t003 and t005–t007 are done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Redirect validation: a `//host` that dot-segment collapse produces is refused — **DONE** | 30m | — |
| [t002](done/t002.md) | Same-origin and content type: stop trusting a client-sent X-Forwarded-Host; require JSON on the JSON endpoints — **DONE** | 45m | — |
| [t003](done/t003.md) | CSP and error headers: `base-uri`, `form-action`, a narrower `connect-src`, and route-rule headers on non-2xx responses — **DONE** | 60m | — |
| [t004](t004.md) | Kratos `allowed_return_urls`: no localhost in production — **BLOCKED (user decision)** | 20m | — |
| [t005](done/t005.md) | The device confirm page and the consent CSRF token — **DONE** | 60m | t001 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 20m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

1. `safeNext` and the service worker refuse a `//host` that dot segments produce.
2. `isSameOrigin` does not trust a client-sent `X-Forwarded-Host`, and the JSON routes require `Content-Type: application/json`.
3. The CSP names `base-uri`, `unsafe-inline` is recorded as accepted with what removing it takes, and error responses keep the route-rule headers. **`form-action` and a narrower `connect-src` did not ship** (2026-11-14): the first breaks the OAuth callback redirects, the second needs a browser sweep ([w21/081](../../081.md)); both reasons are in `dashboard/AGENTS.md`.
4. Production lists no localhost return URL.
5. The consent token is fixed (keyed by `CONSENT_CSRF_SECRET`; creating the Secret is an operator step) and the device page's enrichment was closed as not possible ([w21/080](../../done/080.md): Hydra exposes no device request before the accept). The premise that the screens might not exist was wrong: they are the Render CLI's device login.
6. Each is a test that fails first, and `cd dashboard && yarn lint && yarn test` pass.

## Source + Goal linkage

- **Source:** inbox note w21/072 (promoted 2026-11-14), from the w21/m394 round-25 audit, hardening (Dashboard).
- **Goal linkage:** the security-review lineage (ADR028 to ADR110) and the dashboard's role as the credential-bearing browser surface.
- **Expected outcome:** none of the audit's seven items remains a gap a later change could turn into an open redirect, a CSRF, or a script injection.
- **Why now:** each is cheap while the audit's evidence is fresh, and the first one is a latent open redirect that only the router currently stops.
- **Render parity task:** omitted. Browser-surface hardening; no REST, GraphQL or MCP behavior and no customer-facing Render behavior is touched.

## Open decisions

t004 (the gate): keep or remove the localhost return URL that the documented "Local dev against prod" workflow needs. See [t004](t004.md).
