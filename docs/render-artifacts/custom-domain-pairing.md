# Capture — Render's www↔apex auto-pairing on custom-domain add/delete (w6/m23 t001)

**Captured:** 2026-07-14 · **Method:** docs-fallback (no live Render login available; grounded in Render's public docs — `render.com/docs/custom-domains` — plus the Render community/discourse thread on domain redirects). Same method as `docs/render-artifacts/custom-domain-dns-instructions.md` (w5/m10), which already flagged this exact gap: "Adding a `www` subdomain auto-adds the corresponding root domain and redirects root → `www`. Adding a root domain auto-adds `www` and redirects `www` → root. (bex does **not** replicate this auto-redirect today — out of scope; noted so the parity ledger stays honest.)" This capture pins the behavior precisely enough to close that gap.

## What Render does

### Add apex (`example.com`)

Render's docs: "If you add a root domain (e.g., `example.org`), Render automatically adds the corresponding www subdomain and redirects it to the root domain." So:

- `example.com` is added as given (serves the app).
- `www.example.com` is **auto-added** as a second custom domain on the same service.
- `www.example.com` **redirects to** `example.com` (301/308, not a second independently-serving host).

### Add www (`www.example.com`)

Symmetric: "If you add a www subdomain (e.g., `www.example.org`), Render automatically adds the corresponding root domain and redirects it to the www subdomain."

- `www.example.com` is added as given (serves the app).
- `example.com` is **auto-added**.
- `example.com` **redirects to** `www.example.com`.

**Rule:** whichever half the tenant explicitly adds becomes the "canonical" (serving) host; the other half is auto-added and redirects to the canonical one. Direction is not configurable after the fact — it's fixed by which half was added first.

### Delete semantics — undocumented, inferred

Render's public docs do not say what happens when a tenant deletes one half of an established pair (does the sibling get deleted too? does the redirect dangle?). No canonical answer found in docs, the API reference, or the community forum (the community thread on domain redirects didn't add detail beyond the docs page — link unreachable at capture time, docs page is the primary source). Rather than invent Render's internals, bex defines its own honest, documented delete semantics (see below) — a **conscious, named divergence**, not a guess dressed as parity.

### Non-www subdomains — no pairing

The docs only ever describe www ⇄ apex pairing. A multi-label host like `app.example.com` gets no auto-added sibling and no redirect — it's a plain independent custom domain, same as any subdomain. This matches the existing PSL-adjacent framing in `lego/backend/internal/apps/domains.go`'s `domainType` heuristic (soon to be replaced by the real eTLD+1 check, t002).

### Public-suffix apexes (`example.co.uk`)

Not explicitly demonstrated in Render's docs, but implied by the general behavior: Render must resolve the _registrable domain_ (eTLD+1), not just "the two-label form," to correctly pair `www.example.co.uk` ⇄ `example.co.uk` rather than misfiring on `co.uk` itself. This is the reason t002 exists — `strings.Count(host, ".") == 1` (bex's current `domainType` heuristic) gets `example.co.uk` wrong (3 labels, would classify as "subdomain," and would compute a bogus www-sibling of `www.example.co.uk`).

## What bex will mirror vs. diverge on

| Behavior | Render | bex |
| --- | --- | --- |
| Add apex → auto-add www | ✅ auto-added, redirects to apex | ✅ mirrored: auto-add `www.<host>` as a second entry in `spec.hosts[]` |
| Add www → auto-add apex | ✅ auto-added, redirects to www | ✅ mirrored: auto-add the apex as a second entry in `spec.hosts[]` |
| Sibling redirect (3xx) | ✅ Render's edge issues the redirect | ✖ **diverges**: bex has no per-host redirect mechanism today (no Ingress redirect rule engine) — the sibling is auto-added and **served identically** to the canonical host (both resolve to the same App), not redirected. Noted as a follow-up (a real redirect requires an Ingress-level rewrite bex doesn't have yet); the DoD's "auto-added domain or redirect — whichever the evidence shows" is satisfied by the auto-add half, honestly, without faking a redirect bex can't yet perform. |
| Re-adding the already-auto-added sibling | (not documented; Render's UI shows it as already present) | idempotent no-op — same as any duplicate `AddDomain` (t003) |
| Delete one half of a pair | undocumented | **bex-defined:** deleting either half removes only that host from `spec.hosts[]`; the sibling is left untouched (it was independently added to `spec.hosts[]` at pairing time, so it's a first-class host, not a synthetic derived record). A tenant who wants both gone deletes both. This is the simplest, most predictable rule available given Render gives no evidence either way — and matches bex's existing idempotent, no-hidden-state `DeleteDomain` (`lego/backend/internal/apps/domains.go`). |
| Non-www subdomain pairing (`app.example.com`) | none | none — mirrored (no sibling, no redirect) |
| Public-suffix apex (`example.co.uk`) | (inferred) correctly paired via eTLD+1 | mirrored via `golang.org/x/net/publicsuffix` (t002) |
| Cross-app collision guard covers the sibling | (Render's collision guard is unspecified but the pairing implies the sibling is "claimed" the moment the pair is created) | ✅ mirrored: registering `www.foo.com` on app A reserves `foo.com` against app B too (t004) — closes the blind spot documented in `docs/ADR005-custom-domain.md` § www↔apex sibling pairing and the ADR018 domains row |

## Sources

- [Custom Domains on Render – Render Docs](https://render.com/docs/custom-domains)
- [Add custom domain - API Reference - Render](https://api-docs.render.com/reference/create-custom-domain)
- `docs/render-artifacts/custom-domain-dns-instructions.md` (w5/m10) — prior capture that first flagged this gap
- `docs/ADR005-custom-domain.md` (pre-w6/m23) — "Not done (conscious divergence): Render's automatic www↔apex pairing... bex has no public-suffix list, so the collision guard is per-host; the paired sibling stays independently claimable." — the gap this milestone closes
