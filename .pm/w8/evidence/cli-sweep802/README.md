# CLI sweep 802 — custom-domain add/verify/delete cycle (live, 2026-10-20)

Disposable free web service created, domain cycled, service deleted (204, then GET 404). No new bug filed.

- `POST /services/{id}/custom-domains {"name":"qa802.example.com"}` → 201 with `domainType: subdomain`, all three statuses `pending`, the traffic CNAME (`→ qa802-web.onbex.co`) and the ownership TXT challenge record.
- `POST …/custom-domains/qa802.example.com/verify` with no DNS configured → coded 409 `DOMAIN_OWNERSHIP_PENDING` naming the record host — fail-closed with instructions, not a bare error.
- Re-adding the same domain on the same service → 201 again (idempotent own-claim; the cross-service claim is the coded 409 `CUSTOM_DOMAIN_IN_USE` in `lego/backend/internal/apps/domains.go:910`). Render's same-service duplicate answer is unverified, so this is recorded, not filed.
- DELETE domain → 204; list → `[]`.
- Note: `origin/main` gained `a8c215fdf feat(domains): Render-identical custom-domain setup (w5/m137)` after the `ca30db3a3646` pin — production still runs the pin, so this sweep exercised the pinned behavior.
