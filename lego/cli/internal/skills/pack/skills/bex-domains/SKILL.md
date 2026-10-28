---
name: bex-domains
description: >-
  Configures custom domains and TLS certificates on bex—DNS setup, CNAME
  records, apex domains, wildcard domains, and certificate troubleshooting.
  Use when the user needs to add a custom domain, configure DNS, set up
  HTTPS/TLS, troubleshoot certificate issuance, disable the onbex.co
  subdomain, or add a wildcard domain.
  Trigger terms: custom domain, DNS, CNAME, TLS, SSL, HTTPS, certificate,
  apex domain, wildcard domain, onbex.co, domain verification.
license: MIT
compatibility: bex web services and static sites
metadata:
  author: bex
  version: "1.0.0"
  category: networking
---

# bex Custom Domains

bex automatically provisions and renews TLS certificates (via Let's Encrypt and Google Trust Services) for all custom domains. All HTTP traffic is redirected to HTTPS. Custom domains work on **web services** and **static sites** only.

## When to Use

- Adding a **custom domain** to a web service or static site
- Configuring **DNS records** (CNAME, A, or ALIAS) with a provider
- Troubleshooting **certificate issuance** or **domain verification** failures
- Choosing between **apex** (`example.com`) and **www** (`www.example.com`)
- **Disabling the `onbex.co` subdomain** after adding a custom domain

## Domain Limits and Cost

Workspace plans include a limited number of custom domains. Adding domains beyond the included allowance incurs an additional cost. Confirm current allowances and charges at [bex pricing](https://bex.co/pricing) or in the Dashboard.

## Setup Steps

### 1. Add domain in Dashboard

1. Go to your service's **Settings > Custom Domains**
2. Click **+ Add Custom Domain**
3. Enter your domain (e.g. `app.example.com`)
4. Click **Save**

Adding a `www` subdomain automatically adds the root domain (and vice versa) with a redirect between them.

### 2. Configure DNS

Add a DNS record with your provider pointing to your bex service:

| Domain type | Record type | Name | Value |
|-------------|-------------|------|-------|
| **Subdomain** (`app.example.com`) | CNAME | `app` | `<service>.onbex.co` |
| **Apex** (`example.com`) on Cloudflare | CNAME (flattened) | `@` | `<service>.onbex.co` |
| **Apex** on other providers | A | `@` | Use bex-provided IP (see Dashboard) |

**Important:** Remove any `AAAA` (IPv6) records for your domain. bex uses IPv4, and stale `AAAA` records cause unexpected behavior.

Provider-specific guides:
- [Cloudflare](https://render.com/docs/configure-cloudflare-dns)
- [Namecheap](https://render.com/docs/configure-namecheap-dns)
- [Other providers](https://render.com/docs/configure-other-dns)

### 3. Verify domain

Click **Verify** in the Dashboard. If verification fails, DNS may not have propagated yet—wait a few minutes and retry.

Speed up verification by flushing DNS caches:
- [Google Public DNS](https://developers.google.com/speed/public-dns/cache)
- [Cloudflare DNS](https://1.1.1.1/purge-cache/)
- [OpenDNS](https://cachecheck.opendns.com/)

After verification, bex issues a TLS certificate automatically.

## Wildcard Domains

bex does not support wildcard custom domains (`*.example.com`). Add each subdomain as its own custom domain.

## CAA Records

If your domain has `CAA` records, add entries for bex's certificate authorities:

```
example.com IN CAA 0 issue "letsencrypt.org"
example.com IN CAA 0 issuewild "letsencrypt.org"
```

Without these, TLS certificate issuance fails silently.

## Disabling the `onbex.co` Subdomain

After adding at least one custom domain, you can disable the default `onbex.co` subdomain:

1. Settings > Custom Domains > **bex Subdomain** > toggle to **Disabled**
2. All requests to the `onbex.co` URL receive a 404
3. Can be re-enabled at any time

## Blueprint Configuration

Custom domains are specified in the `domains` field:

```yaml
services:
  - type: web
    name: api
    runtime: node
    plan: starter
    domains:
      - app.example.com
      - www.example.com
```

Blueprint `domains` only **declare** the domain association. You still need to configure DNS with your provider manually.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| AAAA records present | Remove all IPv6 AAAA records for the domain |
| CAA records blocking issuance | Add `letsencrypt.org` and `pki.goog` entries |
| Verifying too quickly | Wait 2-5 minutes for DNS propagation, then flush caches |
| Trying to add domain to a private service | Custom domains only work on web services and static sites |
| 502 after verification | Routing rules are updating — wait a few minutes |

## References

| Document | Contents |
|----------|----------|
| `references/dns-configuration.md` | Provider-specific DNS setup, apex domain options, TTL recommendations |

## Related Skills

- **bex-web-services** — Web service configuration, TLS, port binding
- **bex-static-sites** — Static site domains, CDN, headers
- **bex-blueprints** — `domains` field in `render.yaml`
