# w8 · m118 — Redirect tenant HTTP to HTTPS

**Worker:** worker8 **Goal:** A plain-HTTP request to any tenant web host (platform subdomain or custom domain) is redirected permanently to the same `https://` URL, as on Render, so credentials and cookies never travel in cleartext and apps only ever see `x-forwarded-proto: https`. **Status:** blocked — t001–t004 done 2026-10-18; t005 live check on production after the release pipeline deploys the operator

## Tasks (in order)

| id   | title                                                        | est | depends_on   |
| ---- | ------------------------------------------------------------ | --- | ------------ |
| t001 — **DONE** | Redirect the App Ingress's HTTP router to HTTPS              | 90m | —            |
| t002 — **DONE** | Render parity: redirect contract across hosts and surfaces   | 20m | w8/m118/t001 |
| t003 — **DONE** | Simplify the changed code                                    | 15m | w8/m118/t002 |
| t004 — **DONE** | Test coverage: redirect, exclusions, attribution, ACME paths | 40m | w8/m118/t002 |
| t005 | Close out the milestone                                      | 15m | w8/m118/t004 |

Five tasks, 180m.

## Definition of done

- `curl -sI http://<slug>.onbex.co/x?y=1` on production returns a permanent 3xx whose `Location` is `https://<slug>.onbex.co/x?y=1`. The same holds for a verified custom domain and for a static site.
- HTTPS behavior is unchanged: the same response bodies, the `websecure-` router carries the bytes (w1/m51), and an IP allow-list still refuses with 403.
- cert-manager HTTP-01 issuance for a new custom domain still succeeds, because the solver's own Ingress is not redirected.
- `BEX_HTTPS_REDIRECT=false` keeps a cluster without certificates serving plain HTTP.

## Source + Goal linkage

- **Source:** inbox note `w8/121` (promoted 2026-10-18). `/qa-find-bugs-cli` sweep 114, 2026-10-18: live fixture `qa-20261018-38b5ed-echo` returned `curl http://<slug>.onbex.co/plain` → `HTTP/1.1 200` with `x-forwarded-proto: http` at the app, and no `Location` or HSTS. Traefik's `web` entrypoint deliberately has no global redirect, so HTTP-01 challenges work (`deploy/gitops/base/values/traefik.values.yaml:171`). No per-router `RedirectScheme` replaces it for tenant routes; `host_redirect` only redirects host to host.
- **Goal linkage:** ADR008 Render-alternative core hosting. Render: "Render automatically redirects all HTTP requests to HTTPS".
- **Expected outcome:** no tenant traffic is served over cleartext HTTP. Apps that build absolute URLs from `x-forwarded-proto` produce `https` links.
- **Why now:** this is a security-relevant parity gap found live. Session cookies sent to an `http://` link travel unencrypted.
- **Render parity included because** the redirect is tenant-facing behavior of every web service. No REST, GraphQL or MCP shapes change; t002 verifies the hosts, the dashboard's URLs, and the docs.
