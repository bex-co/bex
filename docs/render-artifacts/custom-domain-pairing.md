# Capture — Render's www↔apex auto-pairing on custom-domain add/delete (w6/m23 t001)

**Captured:** 2026-07-14 (pairing semantics from Render's public docs), extended 2026-07-15 with live HTTPS probes against public Render-routed custom domains. The live probes pin the previously unresolved status/path/query/TLS behavior in both directions without requiring access to a tenant's Render account.

## Live redirect evidence (2026-07-15)

Both probes used a deliberately nonexistent path and a percent-encoded query so the edge redirect could not be confused with application routing. `curl` performed normal certificate verification (no `-k`).

### Apex canonical: `www` redirects to apex

`www.autocentriromanord.com` is a live CNAME to `base44.onrender.com`; the apex is the serving canonical host.

```console
$ dig +short CNAME www.autocentriromanord.com
base44.onrender.com.

$ curl -sS -o /dev/null -D - 'https://www.autocentriromanord.com/m30-probe/path?alpha=one%20two&beta=3'
HTTP/2 301
location: https://autocentriromanord.com/m30-probe/path?alpha=one%20two&beta=3
server: cloudflare
```

The redirecting host's certificate was valid and hostname-specific: subject/SAN `www.autocentriromanord.com`, Google Trust Services `WE1`, valid 2026-06-18 through 2026-09-16.

### `www` canonical: apex redirects to `www`

`3sigma.it` and `www.3sigma.it` both resolve to Render's documented custom-domain load-balancer address `216.24.57.1`; the canonical response carries Render's `rndr-id` header.

```console
$ curl -sS -o /dev/null -D - 'https://3sigma.it/m30-probe/path?alpha=one%20two&beta=3'
HTTP/2 301
location: https://www.3sigma.it/m30-probe/path?alpha=one%20two&beta=3
server: cloudflare

$ curl -sS -o /dev/null -D - 'https://www.3sigma.it/m30-probe/path?alpha=one%20two&beta=3'
HTTP/2 404
rndr-id: a98c968b-b2e7-4535
server: cloudflare
```

The canonical returned 404 only because the probe path was intentionally nonexistent. The redirecting apex's certificate was valid and hostname-specific: subject/SAN `3sigma.it`, Google Trust Services `WE1`, valid 2026-06-17 through 2026-09-15.

### Pinned redirect contract

- Status is **301 Moved Permanently** in both directions.
- `Location` switches only the hostname, forces `https`, and preserves the path and query byte-for-byte (including percent encoding and parameter order).
- The redirecting sibling terminates valid TLS before returning the 301; it therefore needs its own certificate just like the canonical host.

## What Render does

### Add apex (`example.com`)

Render's docs: "If you add a root domain (e.g., `example.org`), Render automatically adds the corresponding www subdomain and redirects it to the root domain." So:

- `example.com` is added as given (serves the app).
- `www.example.com` is **auto-added** as a second custom domain on the same service.
- `www.example.com` **redirects to** `example.com` with 301, preserving path and query; it is not a second independently-serving host.

### Add www (`www.example.com`)

Symmetric: "If you add a www subdomain (e.g., `www.example.org`), Render automatically adds the corresponding root domain and redirects it to the www subdomain."

- `www.example.com` is added as given (serves the app).
- `example.com` is **auto-added**.
- `example.com` **redirects to** `www.example.com`.

**Rule:** whichever half the tenant explicitly adds becomes the "canonical" (serving) host; the other half is auto-added and redirects to the canonical one. Direction is not configurable after the fact — it's fixed by which half was added first.

### Delete semantics — undocumented, inferred

Render's public docs do not say what happens when a tenant deletes one half of an established pair (does the sibling get deleted too? does the redirect dangle?). No canonical answer found in docs, the API reference, or the community forum (the community thread on domain redirects didn't add detail beyond the docs page — link unreachable at capture time, docs page is the primary source). Rather than invent Render's internals, bex defines its own honest, documented delete semantics (see below) — a **conscious, named divergence**, not a guess dressed as parity.

### Non-www subdomains — no pairing

The docs only ever describe www ⇄ apex pairing. A multi-label host like `app.example.com` gets no auto-added sibling and no redirect — it's a plain independent custom domain, same as any subdomain. bex mirrors this using its public-suffix-based registrable-domain check.

### Public-suffix apexes (`example.co.uk`)

Not explicitly demonstrated in Render's docs, but implied by the general behavior: Render must resolve the _registrable domain_ (eTLD+1), not just "the two-label form," to correctly pair `www.example.co.uk` ⇄ `example.co.uk` rather than misfiring on `co.uk` itself. bex's w6/m23 implementation uses `golang.org/x/net/publicsuffix` for the same reason.

## What bex will mirror vs. diverge on

| Behavior | Render | bex |
| --- | --- | --- |
| Add apex → auto-add www | ✅ auto-added, redirects to apex | ✅ mirrored: auto-add `www.<host>` as a second entry in `spec.hosts[]` |
| Add www → auto-add apex | ✅ auto-added, redirects to www | ✅ mirrored: auto-add the apex as a second entry in `spec.hosts[]` |
| Sibling redirect (3xx) | ✅ 301; HTTPS; path/query preserved | ✅ mirrored by w6/m30: a per-sibling Traefik `RedirectRegex` middleware + dedicated TLS-bearing Ingress redirects to the explicitly added canonical host |
| Re-adding the already-auto-added sibling | (not documented; Render's UI shows it as already present) | makes the sibling explicit: clears its redirect so both hosts serve directly, without adding a duplicate |
| Delete one half of a pair | undocumented | **bex-defined:** deletes only the named host; the sibling remains. If the deleted host was the redirect target, the surviving sibling becomes directly served so no redirect can dangle. |
| Non-www subdomain pairing (`app.example.com`) | none | none — mirrored (no sibling, no redirect) |
| Public-suffix apex (`example.co.uk`) | (inferred) correctly paired via eTLD+1 | mirrored via `golang.org/x/net/publicsuffix` (t002) |
| Cross-app collision guard covers the sibling | (Render's collision guard is unspecified but the pairing implies the sibling is "claimed" the moment the pair is created) | ✅ mirrored: registering `www.foo.com` on app A reserves `foo.com` against app B too (t004) — closes the blind spot documented in `docs/ADR005-custom-domain.md` § www↔apex sibling pairing and the ADR018 domains row |

## Sources

- [Custom Domains on Render – Render Docs](https://render.com/docs/custom-domains)
- [Add custom domain - API Reference - Render](https://api-docs.render.com/reference/create-custom-domain)
- `docs/render-artifacts/custom-domain-dns-instructions.md` (w5/m10) — prior capture that first flagged this gap
- `docs/ADR005-custom-domain.md` (pre-w6/m23) — "Not done (conscious divergence): Render's automatic www↔apex pairing... bex has no public-suffix list, so the collision guard is per-host; the paired sibling stays independently claimable." — the gap this milestone closes
