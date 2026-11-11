# Render's wildcard custom domains — captured flow

Captured 2026-11-11 (w4/m326 t001) from [Render's custom-domain guide](https://render.com/docs/custom-domains), sections "Wildcard domains" and "CAA records". No wildcard domain was added to a Render workspace for this capture.

## What Render documents

- **Scope.** "You can apply a wildcard domain (e.g., `*.example.org`) to a Render service to point all matching subdomains to it (e.g., `docs.example.org`, `blog.example.org`, etc.)". Custom domains are for web services and static sites. TLS certificates are created and renewed "for all custom domains, including wildcard domains".
- **Records.** "This configuration requires setting **all** of the following CNAME DNS records":

  | Name | Value | Purpose |
  | --- | --- | --- |
  | `*` | the service's `onrender.com` subdomain | traffic |
  | `_acme-challenge` | `[your-service-subdomain].verify.renderdns.com` | "Enables Render to manage certificate issuance and renewal for your wildcard domain via Let's Encrypt." |
  | `_cf-custom-hostname` | `[your-service-subdomain].hostname.renderdns.com` | "Enables Cloudflare (Render's DDoS protection provider) to validate ownership of your wildcard domain." |

- **The root domain.** "To use a wildcard domain, your root domain must _also_ point to Render. If your root domain points to a non-Render destination, your wildcard configuration will fail." Under Cloudflare's orange-cloud proxying, a wildcard added without its root sends the root's traffic to the same destination, so Render says to grey-cloud the root.
- **CAA.** A domain with CAA records needs `issuewild` entries for Let's Encrypt and Google Trust Services.
- **Verification** is the ordinary Verify step and background check, with no wildcard-specific wording.

## bex (w4/m326)

- **Add** takes `*.<name>`: one leading `*.` label over a claimable host. `*.co.uk`, `*.*.example.com` and `foo.*.example.com` are refused as invalid, and `*.onbex.co` and anything under the platform's zones as reserved. A platform with no ACME delegation zone refuses every wildcard (`CUSTOM_DOMAIN_WILDCARD_UNAVAILABLE`).
- **Records.**
  - The domain's `dnsRecord` is the CNAME at `*` (relative to the zone) to `<slug>.onbex.co`.
  - Its `challengeRecord` is `_acme-challenge` CNAME `<slug>.<delegation zone>`, the analogue of Render's `verify.renderdns.com`. The delegation zone hosts nothing else, as Render's `renderdns.com` is separate from `onrender.com`.
  - The certificate is issued by DNS-01 through that delegation.
- **No `_cf-custom-hostname` record.** It serves Cloudflare-for-SaaS ownership validation inside Render's edge, which bex doesn't run.
- **No root-domain requirement.** Render's comes from its Cloudflare edge; bex verifies a wildcard on its own two records.
- **Verification** passes when a fresh name one label below the base resolves to the service, and `_acme-challenge.<base>` is a CNAME to the service's delegation target.
- **Collisions.** Within one workspace, a concrete host under a wildcard routes to its own service. Across workspaces, a wildcard and any name below it are refused, at any depth, at add and again at verification.
