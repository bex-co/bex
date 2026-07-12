# Capture — Render's add-domain DNS-instructions flow (w5/m10 t003)

**Captured:** 2026-07-09 · **Method:** docs-fallback (no live Render login available; grounded in Render's public docs — `render.com/docs/custom-domains` and `render.com/docs/configure-other-dns` — not a Playwright dashboard capture). The design source for t004 (backend DNS-record fields + verify verb) and t005 (dashboard DNS-instructions panel).

## What Render shows after "Add custom domain"

Render's dashboard lists each custom domain with its **verification status** and, until the domain verifies, the **DNS record(s) the user must create at their registrar**. The record shape depends on whether the domain is a subdomain or an apex (root) domain.

### Subdomain — e.g. `www.example.com`

| Field  | Value                                    |
| ------ | ---------------------------------------- |
| Type   | `CNAME`                                  |
| Host   | `www` (the label(s) below the root zone) |
| Target | `<your-service>.onrender.com`            |

> "you should add a CNAME record for `www` and point it to `example.onrender.com`."

### Apex / root domain — e.g. `example.com`

Render gives two options, ALIAS/ANAME preferred:

| Option        | Type            | Host       | Target                        |
| ------------- | --------------- | ---------- | ----------------------------- |
| 1 (preferred) | `ALIAS`/`ANAME` | `@` (root) | `<your-service>.onrender.com` |
| 2 (fallback)  | `A`             | `@` (root) | `216.24.57.1` (LB IP)         |

> "you can use `216.24.57.1` to point your root domain to Render's load balancer IP." "Remove any `AAAA` records from your domain while configuring DNS." (Render is IPv4-only.)

### www ⇄ apex redirect

- Adding a `www` subdomain auto-adds the corresponding root domain and redirects root → `www`. Adding a root domain auto-adds `www` and redirects `www` → root. (bex does **not** replicate this auto-redirect today — out of scope; noted so the parity ledger stays honest.)

## Verification / re-check behavior

- There is a manual **"Verify"** button next to each domain in the dashboard.
- Clicking it re-checks DNS/cert state **now**: on failure Render hints "your DNS settings might not have propagated yet"; on success "Render issues a TLS certificate for your domain and updates the verification status."
- Render's REST exposes `POST …/custom-domains/{idOrName}/verify` as the same re-check (confirmed in the 2026-07-09 API re-audit; the ledger row this milestone closes).

## How bex maps this (the divergences t004/t005 encode)

bex serves every web App at the **platform host `<app>.<BEX_BASE_DOMAIN>`** (e.g. `<app>.onbex.co`) — the direct analogue of `<service>.onrender.com`. A tenant points their domain at that host. bex's DNS-record projection:

| Domain kind | Type | Host | Target |
| --- | --- | --- | --- |
| subdomain | `CNAME` | label prefix (`www`, `api.stage`) | `<app>.<base-domain>` |
| apex | `ALIAS` | `@` | `<app>.<base-domain>` |

Divergences from Render, deliberate:

1. **Apex → ALIAS to the platform host, not an A-record IP.** bex's edge is Cloudflare-proxied (`docs/ADR005-custom-domain.md`) with no stable tenant-facing anycast IP to surface, so bex emits an ALIAS/ANAME/CNAME-flattening target rather than a bare `A` record. The dashboard adds the apex guidance: use ALIAS/ANAME (or CNAME-flattening) if the provider supports it, otherwise redirect apex → `www` at the registrar. This matches `docs/ADR005-custom-domain.md`'s existing apex note.
2. **Verify is an idempotent re-read, not a state trigger.** bex verification is automatic — cert-manager continuously reconciles a per-host TLS secret (`docs/ADR005-custom-domain.md`), so there is no verification job to kick. bex's verify verb (`POST …/custom-domains/{name}/verify`, GraphQL `verifyCustomDomain`, MCP `verify_custom_domain`) **re-evaluates the TLS-secret/serving state now** and returns the fresh `verificationStatus`/`serverStatus`. Same "click Verify → status refreshes" UX; bex just never has stale state to unblock.
3. **No www⇄apex auto-redirect** (see above).

Field spelling for the wire shape (all three surfaces): a `dnsRecord { type, name, value }` object nested on the custom-domain — `type` ∈ {`CNAME`,`ALIAS`}, `name` the record host, `value` the platform-host target.
