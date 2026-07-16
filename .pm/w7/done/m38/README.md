# w7 · m38 — Custom-domains list: honor cursor/limit + verificationStatus/domainType filters

**Worker:** worker7 **Goal:** `GET /v1/services/{id}/custom-domains` stops ignoring its documented request parameters: `cursor`/`limit` paginate (today it emits per-item cursors it never honors — the projects-style loop hazard) and Render's `verificationStatus`/`domainType` filters filter. **Status:** done

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Confirm Render's custom-domains list params in the pinned OpenAPI                    | 15m | —          | — **DONE** |
| t002 | Implement `cursor`/`limit` + `verificationStatus`/`domainType`; check GraphQL/MCP    | 50m | t001       | — **DONE** |
| t003 | Update the ADR018 custom-domains row's divergence list                               | 15m | t002       | — **DONE** |
| t004 | Render parity                                                                         | 25m | t003       | — **DONE** |
| t005 | Simplify                                                                              | 20m | t004       | — **DONE** |
| t006 | Test coverage                                                                         | 30m | t004       | — **DONE** |
| t007 | Closeout                                                                              | 15m | t006       | — **DONE** |

## Definition of done

Paging the custom-domains list by echoed cursor terminates with full coverage; `limit` honored with the w1/m42 documented default; `verificationStatus=verified|unverified` and `domainType=apex|subdomain` return only matching domains (test-asserted); ADR018's custom-domains row records the fix.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — mechanical-consistency mining, verified in code: `lego/backend/internal/apps/rest.go:949-956` (`listDomains` never reads `r.URL.Query()`) + `lego/backend/internal/apps/domains.go:455-482` (`customDomainWithCursor` envelope, cursor = name, emitted but never honored). The ADR018 custom-domains row documents other divergences but not this one.
- **Goal linkage:** Render parity (custom domains, docs/ADR005-custom-domain.md + ADR018 row). Assigned to w7 on the w7/m34 precedent (API-surface parity chores alongside hardening).
- **Expected outcome:** the domains list stops silently ignoring documented Render request parameters; compliant paging clients terminate.
- **Why now:** same contract-bug class as w1/m42/w6/m32 — cheapest fixed in the same wave, inheriting the same convention + `StablePage` pattern.
- **Render parity:** included — REST/GraphQL/MCP list surface change.
