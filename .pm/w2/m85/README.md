# w2 · m85 — Valid TLS for unknown `*.onbex.co` hosts

**Worker:** worker2 **Goal:** make every first-level `onbex.co` hostname complete a browser-trusted TLS handshake, including hosts that have no App route and should return 404 **Status:** todo

## Tasks (in order)

| id   | title                                                        | est | depends_on      |
| ---- | ------------------------------------------------------------ | --- | --------------- |
| t001 | Add the least-privilege DNS-01 credential supply path        | 45m | —               |
| t002 | Issue the `*.onbex.co` fallback certificate                  | 45m | t001            |
| t003 | Make Traefik use the wildcard only as its default TLSStore   | 45m | t002            |
| t004 | Deploy and verify unknown, active, and suspended hosts live  | 30m | t003            |
| t005 | Simplify the GitOps and secret-provisioning changes          | 20m | t004            |
| t006 | Test coverage for wildcard/default-certificate invariants    | 45m | t003            |
| t007 | Closeout                                                     | 15m | t005, t006      |

## Definition of done

- `openssl s_client -servername notfound.onbex.co` returns a publicly trusted certificate whose SAN covers `*.onbex.co`, never `CN=TRAEFIK DEFAULT CERT`.
- `https://notfound.onbex.co/` still returns the platform's intentional 404; the fix changes TLS trust, not routing unknown hosts to a tenant or dashboard.
- Active App hosts continue serving their own cert/routes, and the managed-cert behavior for suspended static sites in `w3/m46` is not regressed.
- The wildcard is issued through ACME DNS-01 with a zone-scoped token supplied out of Git; no Cloudflare credential or certificate private key is committed or logged.
- Renewal, missing-secret, and not-Ready states are observable through existing cert-manager alerts and structural validation.

## Live evidence and implementation seam

Re-verified 2026-08-29:

```text
GET https://notfound.onbex.co/ -> HTTP/2 404
subject=CN=TRAEFIK DEFAULT CERT
issuer=CN=TRAEFIK DEFAULT CERT
SAN=DNS:<random>.traefik.default
```

Current GitOps has only HTTP-01 `letsencrypt-{staging,prod}` ClusterIssuers. HTTP-01 cannot issue a wildcard; create a production-only namespaced DNS-01 Issuer/Certificate in `traefik`, then a Traefik `TLSStore/default` referencing its Secret. Scope Traefik's default TLS-resource namespace explicitly so it cannot resolve a tenant namespace's same-named store. Supply the Cloudflare token from the protected deploy environment using the repository's `.env.example` → `scripts/gh-secrets.sh` → workflow Secret pattern.

## Source + Goal linkage

- **Source:** user report and live verification, 2026-08-29: unknown `onbex.co` hostnames negotiate Traefik's self-signed default cert before returning 404.
- **Goal linkage:** ADR004 app deployment, ADR005 custom domains, and ADR012's public-edge trust boundary. A hosting platform's default domain must not show a browser certificate warning for a non-existent service.
- **Expected outcome:** unknown first-level hosts fail safely at HTTP with a normal 404 over trusted TLS; existing routed hosts remain unchanged.
- **Why now:** the defect is live, user-visible before any HTTP response can be trusted, and the certificate warning makes an intentional 404 look like a platform compromise.
- **Render parity:** **omitted** — this is platform-edge TLS plumbing with no REST, GraphQL, MCP, or dashboard contract change. `w3/m46` remains the owner of suspended static-site routing semantics; this milestone covers the no-route TLS fallback.
