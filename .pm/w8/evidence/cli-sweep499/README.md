# CLI sweep 499 — static-site routes and headers round trip

**Verdict: no bug filed.** One static site was created, its redirect/rewrite rules and header rules round-tripped, both rule sets cleared, and the service deleted by ID with a HTTP `404` absence proof; the baseline diff reflects only the concurrent hunt's churn.

## Journey

| step | observed result |
| --- | --- |
| Create with `--plan free` | CLI parser refuses at exit `1`: "`--plan is not supported for static_site`" — static sites are planless; correct guard. |
| Create without `--plan` (public example repo, `--publish-directory .`) | Exit `0`, service ID minted. |
| `PUT /routes` (redirect + rewrite) | HTTP `200`; each rule gets a stable `rte-…` ID and ordered `priority` 0/1. |
| `GET /routes` | Same rules in Render's `{route, cursor}` envelope. |
| `PUT`/`GET /headers` (one `/*` rule) | HTTP `200`; `hdr-…` ID; identical envelope. |
| `PUT /routes` with `"type":"teleport"` | HTTP `400`: "`invalid request body at /0/type`" — closed enum with a precise JSON-pointer location. |
| `PUT []` on both, then delete | Both clear to `[]`; service GET `404` after deletion. |

Pairs with sweep 498, where the same surfaces on a web service refuse precisely (`is not a static_site`).

## Safety and unrun checks

- No plan mutation; only the exact owned ID was deleted; secrets unrecorded.
- The static site's build outcome was not awaited — rules attach independently of build state, which is the contract under test.
- Deploy lag means deployed revisions can precede local `main`.
