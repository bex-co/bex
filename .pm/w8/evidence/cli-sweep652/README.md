# CLI sweep 652 — custom-domain validation and lifecycle (live, 2026-10-19)

Disposable free web service `qa652-web` (`srv-dbbbceavfpec73dm0jc0`). Deleted and verified absent (REST 404).

`POST /services/{id}/custom-domains`:

| Name | Result |
| --- | --- |
| `not a domain` | 400 `CUSTOM_DOMAIN_INVALID: invalid hostname "not a domain"` (with `params.hostname`) |
| `-bad.com` | 400 same coded refusal |
| `bad_domain!.com` | 400 same coded refusal |
| `qa652.example.com` | 201, statuses `pending`, CNAME `qa652 → qa652-web.onbex.co` plus `_bex-challenge` TXT ownership record |
| `UPPER.example.COM` | 201, normalized to `upper.example.com` |
| duplicate `qa652.example.com` | 201 again, same id (idempotent upsert) |

`DELETE /services/{id}/custom-domains/{name}` → 204, and 204 again on repeat (idempotent). No bug filed.
