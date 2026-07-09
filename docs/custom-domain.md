# Custom domains — CNAME onto the platform, SSL on our side

Every **`type: web`** App serves at **`<app>.<base-domain>`** — the platform hostname is auto-assigned and mandatory, like Render's `onrender.com` (mechanism: `bex.yml`'s `type: web` sets `App.spec.expose`; the operator's `BEX_BASE_DOMAIN` + wildcard DNS do the rest; `type: private` Apps have no ingress at all). A customer brings their own domain by pointing a CNAME at that hostname. They never touch certificates: **their one CNAME is also what authorizes us to issue TLS for their domain** (ACME HTTP-01 challenges for `www.theirdomain.com` land on our edge _because_ of the CNAME).

## What the customer does (their entire job)

```
CNAME  www.theirdomain.com  ->  <app>.<base-domain>
```

- **Apex domains (`theirdomain.com`) can't CNAME** per the DNS spec. Options: ALIAS/CNAME-flattening if their DNS provider supports it, or redirect apex → `www` at their registrar. Instruct `www` by default.
- Add the CNAME **first**, then tell us — issuance can only start once the record resolves (we retry, but the happy path is CNAME-first).

## What we do

```mermaid
sequenceDiagram
  participant C as customer DNS
  participant CF as Cloudflare edge<br/>(base-domain zone, proxied)
  participant O as operator and cert-manager<br/>(in-cluster)

  Note over C: ① CNAME www.theirdomain.com -> app.base-domain
  Note over CF: ② scripts/domain-add.sh — register the<br/>hostname (Cloudflare for SaaS). CF issues and<br/>renews the EDGE cert once the CNAME resolves
  Note over O: ③ add domain to the app's bex.yml domains: then<br/>scripts/app-apply.sh — operator adds an Ingress<br/>rule and its own per-host ORIGIN cert (HTTP-01)
  CF->>O: proxies Host: www.theirdomain.com to the origin
```

Two independent certs by design: Cloudflare's at the edge (browser-facing), cert-manager's at the origin (lets the zone run SSL mode Full (strict)).

- **Step ② exists because the base-domain zone is proxied** (orange-cloud): the CF edge only answers for hostnames it knows, so each customer domain must be registered as a _custom hostname_ (Cloudflare for SaaS — free ≤ 100 hostnames; one-time zone setup: enable it + set the fallback origin). Without it, customer traffic dies at the edge with an SSL mismatch.
- **Step ③ is ordinary bex mechanics**: `domains[1:]` in `bex.yml` becomes `App.spec.hosts`; the operator renders one Ingress rule and **one TLS secret per host**, so one customer's broken DNS can never block another domain's issuance or renewal (`<app>-tls` for the first host — never renamed — and `<app>-tls-<host>` for the rest).

## API surface — DNS instructions & verify (w5/m10)

Every custom domain the API returns carries a **`dnsRecord`** — the exact record the tenant creates at their registrar (Render's post-add DNS instructions; capture in [render-artifacts/custom-domain-dns-instructions.md](render-artifacts/custom-domain-dns-instructions.md)):

| Domain kind | `dnsRecord.type` | `dnsRecord.name` | `dnsRecord.value` |
| --- | --- | --- | --- |
| subdomain | `CNAME` | label prefix (`www`, `api.staging`) | `<app>.<base-domain>` |
| apex | `ALIAS` | `@` | `<app>.<base-domain>` |

The target is the App's **platform host** `<app>.<BEX_BASE_DOMAIN>` — bex-api reads `BEX_BASE_DOMAIN` (falling back to the App's status URL). bex points apex at the platform host via ALIAS/ANAME/CNAME-flattening rather than a bare `A`-record IP (the edge is Cloudflare-proxied), and the dashboard adds the "ALIAS/ANAME, or redirect apex → `www`" guidance.

A **verify** verb re-checks a domain's DNS/cert state now and returns its fresh status — `POST /v1/services/{id}/custom-domains/{name}/verify` (REST), `verifyCustomDomain(id, name)` (GraphQL), `verify_custom_domain` (MCP). bex verification is automatic (cert-manager reconciles the per-host TLS secret continuously), so this is an idempotent re-read, not a re-verification trigger — it gives the dashboard a "Verify / re-check" action that refreshes a pending row without a mutation. All three surfaces return the same `dnsRecord` fields and verify semantics.

## Lifecycle / operational notes

- **Pending window**: between "CNAME added" and "certs issued" (typically < 1 min after DNS resolves) the domain serves a mismatched cert. Every platform has this.
- **Renewal depends on the CNAME staying put.** If the customer deletes or repoints it, renewal fails and the domain goes dark within ~90 days — looking like _our_ outage. Per-domain cert health belongs in monitoring / the future control plane.
- **Ownership verification**: today `bex.yml` is trusted input (single tenant). Multi-tenant custom domains need a verify-before-route state machine (pending → verifying → active, owned-by-tenant) — that is control-plane work (see [control-plane.md](control-plane.md), the domains table), not operator work.
- Never probe a hostname before its DNS record exists — you'll poison resolver negative caches for up to an hour. Check with `dig @1.1.1.1` / `curl --resolve`.
