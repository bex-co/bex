# CLI sweep 497 — custom-domain lifecycle

**Verdict: no bug filed.** Two free web services with one custom domain each were created, probed, and deleted by ID with HTTP `404` absence proofs; the baseline was restored exactly (one new row belongs to the concurrent hunt).

## Journey

| step | observed result | assessment |
| --- | --- | --- |
| `bex domains --help` | `unknown command` at exit `1` | Custom domains are API/dashboard-only in the pinned CLI; consistent upstream gap, not a defect. |
| `POST /v1/services/{id}/custom-domains` (`qa497.invalid.example.com`) | HTTP `201`: `domainType: subdomain`, all statuses `pending`, CNAME record targeting the service's `.onbex.co` host, TXT `_bex-challenge` ownership record | Render-shaped pending flow; verification correctly waits on DNS that will never exist for an `.invalid` name. |
| Re-add the same domain to the same service | HTTP `201` with the same object | Deliberate idempotence: [`domains.go:902`](../../../../lego/backend/internal/apps/domains.go) reserves the `409 CUSTOM_DOMAIN_IN_USE` for cross-service conflicts, and the add path treats a same-service duplicate as a no-op (covered by `domain_codes_test.go`). Not a bug. |
| Malformed name (`"not a domain!!"`) | HTTP `400`, `CUSTOM_DOMAIN_INVALID` with the hostname echoed in `params` | Precise validation dialect ([`domains.go:259`](../../../../lego/backend/internal/apps/domains.go)). |
| `DELETE …/custom-domains/{name}`, list again | `204`-style empty success; list returns `[]` | Clean removal before service deletion. |

## Safety and unrun checks

- No plan mutation; domains pointed at `.invalid` names that can never resolve or be claimed; only exact owned IDs deleted; secrets unrecorded (the TXT challenge token is per-domain and died with the fixture).
- Cross-service `CUSTOM_DOMAIN_IN_USE` was read from code but not exercised live, to avoid creating a second long-lived fixture pair.
- Deploy lag means deployed revisions can precede local `main`.
