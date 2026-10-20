# CLI sweep 842 — custom-domain edge shapes (live, 2026-10-20)

Disposable free web service created, domains cycled, service deleted (204, then GET 404). No new bug filed.

- Apex `example.com` → 201 with `domainType: apex`, an `ALIAS @` traffic record, and a `www.example.com` companion appearing in the list alongside it.
- Underscore hostname → coded 400 `CUSTOM_DOMAIN_INVALID` naming the hostname in `params`.
- Claiming the service's **own** platform host (`qa842-web.onbex.co`) as a custom domain → 201. Recorded without filing: the claim names the service's own slug host (it routes to itself, and a *different* service's host is the coded 409 `CUSTOM_DOMAIN_IN_USE` family from sweep 802), so this reads as a benign self-claim; Render's answer for the same input is unverified, and there is no cross-tenant effect to hang a filing on.
